package agentruntime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"spire2mind/internal/game"
)

func (s *Session) handleProviderFailure(ctx context.Context, err error, state *game.StateSnapshot) error {
	if s.runtime.Agent == nil || s.forceDeterministic {
		return err
	}

	s.providerFailures++
	s.providerState = "recovering"
	s.todo.RecordFailure("agent_cycle", err)
	if s.providerFailures >= maxProviderFailures {
		s.forceDeterministic = true
		s.runtime.Agent.Close()
		s.runtime.Agent = nil
		s.providerState = "fallback"
		s.emit(SessionEvent{
			Time:    time.Now(),
			Kind:    SessionEventStatus,
			Cycle:   s.cycle,
			Attempt: s.currentAttemptForState(state),
			Message: s.sayf("agent provider failed %d times; falling back to deterministic mode", "agent provider failed %d times; falling back to deterministic mode", s.providerFailures),
			Screen:  state.Screen,
			RunID:   state.RunID,
			Data: map[string]interface{}{
				"mode":              "deterministic_fallback",
				"provider_state":    s.providerState,
				"provider_recovery": "fallback",
			},
		})
		return nil
	}

	backoff := time.Duration(s.providerFailures) * 2 * time.Second
	s.emit(SessionEvent{
		Time:    time.Now(),
		Kind:    SessionEventStatus,
		Cycle:   s.cycle,
		Attempt: s.currentAttemptForState(state),
		Message: s.sayf("agent cycle failed (%v); retrying in %s", "agent cycle failed (%v); retrying in %s", err, backoff),
		Screen:  state.Screen,
		RunID:   state.RunID,
		Data: map[string]interface{}{
			"provider_failures": s.providerFailures,
			"mode":              s.cfg.ModeLabel(),
			"provider_state":    s.providerState,
			"provider_recovery": classifyProviderRecovery(err),
		},
	})

	timer := time.NewTimer(backoff)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (s *Session) handleDirectActionFailure(ctx context.Context, err error, state *game.StateSnapshot, request game.ActionRequest) bool {
	if err == nil || !isRecoverableActionError(err) {
		return false
	}

	recoveryKind := recoverableActionKind(err)
	if isSoftReplanDriftKind(recoveryKind) {
		if recoveryAlreadyReported(err) {
			return true
		}
		s.emit(SessionEvent{
			Time:    time.Now(),
			Kind:    SessionEventStatus,
			Cycle:   s.cycle,
			Attempt: s.currentAttemptForState(state),
			Message: s.sayf("soft seam recovery for %s; replanning on the latest state", "soft seam recovery for %s; replanning on the latest state", formatActionDebug(request)),
			Screen:  state.Screen,
			RunID:   state.RunID,
			Data: map[string]interface{}{
				"recovery_kind":  "soft_replan",
				"drift_kind":     recoveryKind,
				"provider_state": s.providerState,
			},
		})
		return true
	}

	if liveState, driftKind, ok := s.recoverPhaseAdvanceInvalidAction(ctx, err, state, request); ok {
		s.emit(SessionEvent{
			Time:    time.Now(),
			Kind:    SessionEventStatus,
			Cycle:   s.cycle,
			Attempt: s.currentAttemptForState(liveState),
			Message: s.sayf("soft seam recovery for %s; latest state already advanced to the next legal phase (%s)", "soft seam recovery for %s; latest state already advanced to the next legal phase (%s)", formatActionDebug(request), driftKind),
			Screen:  liveState.Screen,
			RunID:   liveState.RunID,
			State:   liveState,
			Data: map[string]interface{}{
				"recovery_kind":              "soft_replan",
				"drift_kind":                 driftKind,
				"provider_state":             s.providerState,
				"expected_state_summary":     decisionStateSummary(state),
				"live_state_summary":         decisionStateSummary(liveState),
				"expected_state_fingerprint": digestState(state),
				"live_state_fingerprint":     digestState(liveState),
			},
		})
		return true
	}

	s.repeatedInvalidActions++
	if s.failures != nil {
		s.failures.Record(digestState(state), request)
	}
	s.emit(SessionEvent{
		Time:    time.Now(),
		Kind:    SessionEventToolError,
		Cycle:   s.cycle,
		Attempt: s.currentAttemptForState(state),
		Message: fmt.Sprintf("recoverable action failure for %s: %v", formatActionDebug(request), err),
		Screen:  state.Screen,
		RunID:   state.RunID,
		Data: map[string]interface{}{
			"recovery_kind":  recoveryKind,
			"provider_state": s.providerState,
		},
	})

	if s.repeatedInvalidActions >= maxRepeatedInvalidActions {
		s.emit(SessionEvent{
			Time:    time.Now(),
			Kind:    SessionEventStatus,
			Cycle:   s.cycle,
			Attempt: s.currentAttemptForState(state),
			Message: s.sayf("repeated action failures (%d) on %s", "repeated action failures (%d) on %s", s.repeatedInvalidActions, state.Screen),
			Screen:  state.Screen,
			RunID:   state.RunID,
			Data: map[string]interface{}{
				"provider_state": s.providerState,
			},
		})
	}

	return true
}

func (s *Session) recoverPhaseAdvanceInvalidAction(ctx context.Context, err error, expectedState *game.StateSnapshot, request game.ActionRequest) (*game.StateSnapshot, string, bool) {
	if !isInvalidActionError(err) || !shouldTreatInvalidActionAsPhaseAdvance(request.Action) {
		return nil, "", false
	}
	if s == nil || s.runtime == nil || s.runtime.Client == nil {
		return nil, "", false
	}

	liveState, getErr := s.runtime.Client.GetState(ctx)
	if getErr != nil || liveState == nil {
		return nil, "", false
	}

	liveState, getErr = s.freshActionableStateForExecution(ctx, liveState)
	if getErr != nil || liveState == nil {
		return nil, "", false
	}

	driftKind := classifyStateDrift(expectedState, liveState)
	if isSoftReplanDriftKind(driftKind) {
		return liveState, driftKind, true
	}

	return nil, "", false
}

func shouldTreatInvalidActionAsPhaseAdvance(action string) bool {
	switch strings.TrimSpace(strings.ToLower(action)) {
	case "claim_reward", "proceed", "continue_after_game_over", "return_to_main_menu":
		return true
	default:
		return false
	}
}

func isRecoverableActionError(err error) bool {
	if err == nil {
		return false
	}

	text := strings.ToLower(err.Error())
	return strings.Contains(text, "invalid_action") ||
		strings.Contains(text, "invalid_target") ||
		strings.Contains(text, "state_unavailable") ||
		isTransientActionTransportError(err)
}

func isTransientActionTransportError(err error) bool {
	if err == nil {
		return false
	}

	text := strings.ToLower(err.Error())
	return strings.Contains(text, "wsarecv") ||
		strings.Contains(text, "forcibly closed by the remote host") ||
		strings.Contains(text, "connection reset") ||
		strings.Contains(text, "broken pipe") ||
		strings.Contains(text, " eof") ||
		strings.HasSuffix(text, ":eof") ||
		strings.HasSuffix(text, "eof") ||
		strings.Contains(text, "unexpected eof") ||
		strings.Contains(text, "connection aborted")
}
