package agentruntime

import (
	"context"
	"fmt"
	"strings"
	"time"

	openagenttypes "github.com/hunknownz/open-agent-sdk-go/types"

	"spire2mind/internal/config"
	"spire2mind/internal/game"
)

type CycleSummary struct {
	LastAssistantText string                 `json:"last_assistant_text,omitempty"`
	LastAction        string                 `json:"last_action,omitempty"`
	Decision          *ActionDecision        `json:"decision,omitempty"`
	ActCalls          int                    `json:"act_calls"`
	MadeProgress      bool                   `json:"made_progress"`
	HadInvalidAction  bool                   `json:"had_invalid_action"`
	Turns             int                    `json:"turns"`
	Cost              float64                `json:"cost"`
	Metadata          map[string]interface{} `json:"metadata,omitempty"`
}

func (s *Session) runAgentCycle(ctx context.Context, prompt string, state *game.StateSnapshot) (*CycleSummary, error) {
	if s.usesStructuredDecisionMode() {
		return s.runStructuredDecisionCycle(ctx, prompt, state)
	}

	result := &CycleSummary{}
	eventCh, errCh := s.runtime.Agent.Query(ctx, prompt)

	for event := range eventCh {
		switch event.Type {
		case openagenttypes.MessageTypeAssistant:
			s.handleAssistantEvent(result, event)
		case "tool_result":
			s.handleToolResultEvent(result, event)
		case openagenttypes.MessageTypeResult:
			result.Cost = event.Cost
			result.Turns = event.NumTurns
			if result.Metadata == nil {
				result.Metadata = make(map[string]interface{})
			}
			result.Metadata["provider"] = "api"
			result.Metadata["cycle_duration_ms"] = event.Duration
			if event.Usage != nil {
				result.Metadata["input_tokens"] = event.Usage.InputTokens
				result.Metadata["output_tokens"] = event.Usage.OutputTokens
			}
			s.emit(SessionEvent{
				Time:    time.Now(),
				Kind:    SessionEventStatus,
				Cycle:   s.cycle,
				Attempt: s.attemptCount,
				Message: s.say("agent cycle completed", "Agent cycle completed"),
				Cost:    event.Cost,
				Turns:   event.NumTurns,
				Data: map[string]interface{}{
					"provider":          "api",
					"cycle_duration_ms": event.Duration,
					"input_tokens":      usageInputTokens(event.Usage),
					"output_tokens":     usageOutputTokens(event.Usage),
				},
			})
		}
	}

	if err := <-errCh; err != nil {
		return nil, err
	}

	return result, nil
}

func (s *Session) runStructuredDecisionCycle(ctx context.Context, prompt string, state *game.StateSnapshot) (*CycleSummary, error) {
	result, err := s.runtime.Agent.Prompt(ctx, prompt)
	if err != nil {
		return nil, err
	}

	summary := &CycleSummary{
		LastAssistantText: sanitizeLiveReasoning(result.Text),
		Turns:             result.NumTurns,
		Cost:              result.Cost,
		Metadata: map[string]interface{}{
			"provider":                  s.cfg.ProviderLabel(),
			"decision_mode":             "structured",
			"structured_output_present": result.StructuredOutput != nil,
		},
	}
	if result.Usage.InputTokens > 0 || result.Usage.OutputTokens > 0 {
		summary.Metadata["input_tokens"] = result.Usage.InputTokens
		summary.Metadata["output_tokens"] = result.Usage.OutputTokens
	}
	if result.Duration > 0 {
		summary.Metadata["cycle_duration_ms"] = result.Duration.Milliseconds()
	}

	if summary.LastAssistantText != "" {
		s.emit(SessionEvent{
			Time:    time.Now(),
			Kind:    SessionEventAssistant,
			Cycle:   s.cycle,
			Attempt: s.currentAttemptForState(state),
			Message: summary.LastAssistantText,
			Screen:  state.Screen,
			RunID:   state.RunID,
		})
	}

	decision, err := ParseActionDecision(result.StructuredOutput, result.Text)
	if err != nil {
		failureKind := classifyStructuredDecisionFailure(result.StructuredOutput, result.Text, err)
		summary.Metadata["decision_contract_status"] = "fallback"
		summary.Metadata["decision_failure_kind"] = failureKind
		summary.Metadata["decision_parse_error"] = err.Error()

		if fallbackDecision, fallbackReason, ok := s.chooseStructuredFallbackDecision(state); ok {
			summary.Decision = fallbackDecision
			summary.LastAction = fallbackDecision.Action
			summary.Metadata["decision_fallback"] = "local_policy"
			summary.Metadata["decision_fallback_reason"] = fallbackReason
			if fallbackDecision.Reason != "" {
				summary.Metadata["decision_reason"] = fallbackDecision.Reason
			}

			s.emit(SessionEvent{
				Time:    time.Now(),
				Kind:    SessionEventStatus,
				Cycle:   s.cycle,
				Attempt: s.currentAttemptForState(state),
				Message: s.say("structured decision missing or invalid; falling back to local policy", "structured decision missing or invalid; falling back to local policy"),
				Screen:  state.Screen,
				RunID:   state.RunID,
				Cost:    result.Cost,
				Turns:   result.NumTurns,
				Data: map[string]interface{}{
					"provider":                  s.cfg.ProviderLabel(),
					"decision_mode":             "structured",
					"decision_contract_status":  "fallback",
					"decision_failure_kind":     failureKind,
					"structured_output_present": result.StructuredOutput != nil,
					"decision_fallback":         "local_policy",
					"decision_fallback_reason":  fallbackReason,
					"decision_parse_error":      err.Error(),
					"action":                    fallbackDecision.Action,
					"reason":                    fallbackDecision.Reason,
					"cycle_duration_ms":         result.Duration.Milliseconds(),
					"input_tokens":              result.Usage.InputTokens,
					"output_tokens":             result.Usage.OutputTokens,
				},
			})
			return summary, nil
		}

		return nil, fmt.Errorf("structured decision contract failure (%s): %w", failureKind, err)
	}

	summary.Decision = decision
	summary.LastAction = decision.Action
	summary.Metadata["decision_contract_status"] = "success"
	if decision.Reason != "" {
		summary.Metadata["decision_reason"] = decision.Reason
	}

	s.emit(SessionEvent{
		Time:    time.Now(),
		Kind:    SessionEventStatus,
		Cycle:   s.cycle,
		Attempt: s.currentAttemptForState(state),
		Message: s.say("structured decision received", "structured decision received"),
		Screen:  state.Screen,
		RunID:   state.RunID,
		Cost:    result.Cost,
		Turns:   result.NumTurns,
		Data: map[string]interface{}{
			"action":                    decision.Action,
			"reason":                    decision.Reason,
			"provider":                  s.cfg.ProviderLabel(),
			"decision_mode":             "structured",
			"decision_contract_status":  "success",
			"structured_output_present": result.StructuredOutput != nil,
			"cycle_duration_ms":         result.Duration.Milliseconds(),
			"input_tokens":              result.Usage.InputTokens,
			"output_tokens":             result.Usage.OutputTokens,
		},
	})

	return summary, nil
}

func classifyStructuredDecisionFailure(raw interface{}, fallbackText string, err error) string {
	if err == nil {
		return ""
	}
	lower := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case raw == nil && strings.TrimSpace(fallbackText) == "":
		return "missing_structured_output"
	case strings.Contains(lower, "missing structured action decision"):
		return "missing_structured_output"
	case strings.Contains(lower, "structured output missing action"):
		return "empty_action_in_structured_output"
	case strings.Contains(lower, "structured output is"):
		return "invalid_structured_shape"
	case strings.Contains(lower, "parse fallback action decision"):
		return "fallback_text_parse_failed"
	default:
		return "invalid_structured_shape"
	}
}

func (s *Session) chooseStructuredFallbackDecision(state *game.StateSnapshot) (*ActionDecision, string, bool) {
	if state == nil {
		return nil, "", false
	}

	if request, reason, ok := ChooseRuleBasedAction(state, s.maxAttempts, s.currentAttemptForState(state), s.failures); ok {
		decision := actionRequestToDecision(request)
		if decision != nil {
			decision.Reason = reason
		}
		return decision, "rule_based_action", true
	}

	if request, reason, ok := s.choosePlannerFallbackAction(state); ok {
		decision := actionRequestToDecision(request)
		if decision != nil {
			decision.Reason = reason
		}
		return decision, "planner_fallback_action", true
	}

	if request, reason, ok := ChooseDeterministicAction(state, s.maxAttempts, s.currentAttemptForState(state), s.failures); ok {
		decision := actionRequestToDecision(request)
		if decision != nil {
			decision.Reason = reason
		}
		return decision, "deterministic_action", true
	}

	return nil, "", false
}

func (s *Session) handleAssistantEvent(result *CycleSummary, event openagenttypes.SDKMessage) {
	if event.Message == nil {
		return
	}

	text := strings.TrimSpace(openagenttypes.ExtractText(event.Message))
	if sanitized := sanitizeLiveReasoning(text); sanitized != "" {
		result.LastAssistantText = sanitized
		s.emit(SessionEvent{
			Time:    time.Now(),
			Kind:    SessionEventAssistant,
			Cycle:   s.cycle,
			Attempt: s.attemptCount,
			Message: sanitized,
		})
	}

	for _, block := range event.Message.Content {
		if block.Type != openagenttypes.ContentBlockToolUse {
			continue
		}

		payload := cloneMap(block.Input)
		toolName := block.Name
		actionName, _ := payload["action"].(string)
		if toolName == "act" {
			result.ActCalls++
			result.LastAction = actionName
			result.MadeProgress = true
		}

		s.emit(SessionEvent{
			Time:    time.Now(),
			Kind:    SessionEventTool,
			Cycle:   s.cycle,
			Attempt: s.attemptCount,
			Message: fmt.Sprintf("tool call: %s", toolName),
			Tool:    toolName,
			Action:  actionName,
			Data:    payload,
		})
	}
}

func (s *Session) handleToolResultEvent(result *CycleSummary, event openagenttypes.SDKMessage) {
	text := strings.TrimSpace(event.Text)
	if text == "" {
		return
	}

	kind := SessionEventStatus
	if strings.Contains(text, "invalid_action") || strings.Contains(text, "invalid_target") {
		kind = SessionEventToolError
		result.HadInvalidAction = true
	}

	s.emit(SessionEvent{
		Time:    time.Now(),
		Kind:    kind,
		Cycle:   s.cycle,
		Attempt: s.attemptCount,
		Message: text,
	})
}

func (s *Session) usesStructuredDecisionMode() bool {
	if s == nil || s.runtime == nil || s.runtime.Agent == nil {
		return false
	}
	if s.runtime.Agent.ProviderMode() == openagenttypes.ProviderClaudeCLI {
		return true
	}
	return s.cfg.UsesStructuredAPIDecisions()
}

func (s *Session) choosePlannerFallbackAction(state *game.StateSnapshot) (game.ActionRequest, string, bool) {
	if !shouldUsePlannerFallback(s, state) {
		return game.ActionRequest{}, "", false
	}

	plan := s.currentCombatPlan(state)
	if plan == nil || len(plan.Candidates) == 0 {
		return game.ActionRequest{}, "", false
	}

	request, ok := plan.BestActionRequest()
	if !ok {
		return game.ActionRequest{}, "", false
	}
	request = NormalizeActionRequestForState(state, request)
	if err := ValidateActionRequest(state, request); err != nil {
		return game.ActionRequest{}, "", false
	}

	label := valueOrDash(plan.Candidates[0].Label)
	reason := s.sayf(
		"planner-backed deterministic combat action (%s): %s",
		"planner-backed deterministic combat action (%s): %s",
		valueOrDash(plan.Mode),
		label,
	)
	return request, reason, true
}

func (s *Session) choosePlannerDirectAction(state *game.StateSnapshot) (game.ActionRequest, string, bool) {
	if !shouldUsePlannerDirectAction(s, state) {
		return game.ActionRequest{}, "", false
	}

	plan := s.currentCombatPlan(state)
	if !plannerDirectActionAllowed(state, plan) {
		return game.ActionRequest{}, "", false
	}

	request, ok := plan.BestActionRequest()
	if !ok {
		return game.ActionRequest{}, "", false
	}
	request = NormalizeActionRequestForState(state, request)
	if err := ValidateActionRequest(state, request); err != nil {
		return game.ActionRequest{}, "", false
	}

	label := valueOrDash(plan.Candidates[0].Label)
	reason := s.sayf(
		"planner-selected combat action (%s): %s",
		"planner-selected combat action (%s): %s",
		valueOrDash(plan.Mode),
		label,
	)
	return request, reason, true
}

func shouldUsePlannerFallback(s *Session, state *game.StateSnapshot) bool {
	if s == nil || state == nil || !strings.EqualFold(strings.TrimSpace(state.Screen), "COMBAT") {
		return false
	}
	if s.forceDeterministic {
		return true
	}
	if s.runtime == nil || s.runtime.Agent == nil {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(s.providerState)) {
	case "fallback", "recovering":
		return true
	default:
		return false
	}
}

func shouldUsePlannerDirectAction(s *Session, state *game.StateSnapshot) bool {
	if s == nil || state == nil || !strings.EqualFold(strings.TrimSpace(state.Screen), "COMBAT") {
		return false
	}
	if s.cfg.ForceModelEval {
		return false
	}
	return s.canUseModelAgent()
}

func plannerDirectActionAllowed(state *game.StateSnapshot, plan *CombatPlan) bool {
	if state == nil || plan == nil || len(plan.Candidates) == 0 {
		return false
	}

	best := plan.Candidates[0]
	if strings.TrimSpace(best.Action) == "" {
		return false
	}
	if strings.EqualFold(best.Action, "end_turn") {
		return len(plan.Candidates) == 1
	}
	if !strings.EqualFold(best.Action, "play_card") {
		return false
	}

	snapshot := buildCombatSnapshot(state, nil)
	if hasSingleLiveEnemy(snapshot) {
		return true
	}

	if len(plan.Candidates) == 1 {
		return true
	}

	scoreGap := plan.Candidates[0].Score - plan.Candidates[1].Score
	if scoreGap >= plannerDirectStrongGap {
		return true
	}

	if isSimpleCombatSnapshot(snapshot) && scoreGap >= plannerDirectSimpleGap {
		return true
	}

	return false
}

func hasSingleLiveEnemy(snapshot CombatSnapshot) bool {
	livingEnemies := 0
	for _, enemy := range snapshot.Enemies {
		if enemy.Hittable && enemy.CurrentHP > 0 {
			livingEnemies++
		}
	}
	return livingEnemies == 1
}

func isSimpleCombatSnapshot(snapshot CombatSnapshot) bool {
	livingEnemies := 0
	for _, enemy := range snapshot.Enemies {
		if enemy.Hittable && enemy.CurrentHP > 0 {
			livingEnemies++
		}
	}
	playableCards := countPlayableCards(snapshot)
	return livingEnemies <= 1 || playableCards <= 2
}

func (s *Session) canUseModelAgent() bool {
	if s == nil || s.runtime == nil || s.runtime.Agent == nil || s.forceDeterministic {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(s.providerState)) {
	case "fallback", "recovering", "deterministic":
		return false
	default:
		return true
	}
}

func usageInputTokens(usage *openagenttypes.Usage) int {
	if usage == nil {
		return 0
	}
	return usage.InputTokens
}

func usageOutputTokens(usage *openagenttypes.Usage) int {
	if usage == nil {
		return 0
	}
	return usage.OutputTokens
}

func initialProviderState(cfg config.Config) string {
	switch {
	case cfg.UsesClaudeCLI():
		return "healthy"
	case cfg.ModelProvider == config.ModelProviderAPI && cfg.HasModelConfig():
		return "healthy"
	default:
		return "deterministic"
	}
}

func initialSessionProviderState(cfg config.Config, runtime *Runtime) string {
	if runtime == nil || runtime.Agent == nil {
		return "deterministic"
	}
	return initialProviderState(cfg)
}

func classifyProviderRecovery(err error) string {
	if err == nil {
		return ""
	}

	lower := strings.ToLower(err.Error())
	switch {
	case strings.Contains(lower, "deadline exceeded"),
		strings.Contains(lower, "context deadline exceeded"),
		strings.Contains(lower, "timeout"),
		strings.Contains(lower, "timed out"):
		return "provider_timeout"
	case strings.Contains(lower, "structured decision contract failure"):
		return "decision_contract_failure"
	case strings.Contains(lower, "claude cli transport unavailable"),
		strings.Contains(lower, "pipe has been ended"),
		strings.Contains(lower, "broken pipe"),
		strings.Contains(lower, "stream closed before result"),
		strings.Contains(lower, "eof"):
		return "transport_restart"
	default:
		return "provider_retry"
	}
}
