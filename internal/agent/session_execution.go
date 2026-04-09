package agentruntime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"spire2mind/internal/game"
)

func (s *Session) executeDirectAction(ctx context.Context, expectedState *game.StateSnapshot, request game.ActionRequest) error {
	beforeState, err := s.runtime.Client.GetState(ctx)
	if err != nil {
		return err
	}
	beforeState, err = s.freshActionableStateForExecution(ctx, beforeState)
	if err != nil {
		return err
	}
	if expectedState != nil && digestState(beforeState) != digestState(expectedState) {
		beforeState, _ = s.stabilizeLiveStateForExecution(ctx, expectedState, beforeState)
	}
	resolved, err := s.actionResolver().ResolveRequest(expectedState, beforeState, request)
	if err != nil {
		s.todo.RecordFailure(request.Action, err)
		return err
	}
	beforeState = resolved.LiveState
	request = resolved.Request
	if expectedState != nil && resolved.RecoveryKind != "" {
		driftKind := classifyStateDrift(expectedState, beforeState)
		if !shouldQuietDecisionReuse(driftKind, resolved.RecoveryKind, resolved.OriginalRequest, request) {
			s.emit(SessionEvent{
				Time:    time.Now(),
				Kind:    SessionEventStatus,
				Cycle:   s.cycle,
				Attempt: s.currentAttemptForState(beforeState),
				Message: s.say("live state changed before direct action execution; remapping the action onto the latest legal state", "live state changed before direct action execution; remapping the action onto the latest legal state"),
				Screen:  beforeState.Screen,
				RunID:   beforeState.RunID,
				State:   beforeState,
				Data: map[string]interface{}{
					"expected_state_summary":     decisionStateSummary(expectedState),
					"live_state_summary":         decisionStateSummary(beforeState),
					"expected_state_fingerprint": digestState(expectedState),
					"live_state_fingerprint":     digestState(beforeState),
					"drift_kind":                 driftKind,
					"recovery_kind":              resolved.RecoveryKind,
					"decision_reused":            resolved.DecisionReused,
				},
			})
		}
	}

	beforeState, request, err = s.rebindRequestForImmediateExecution(ctx, expectedState, beforeState, request)
	if err != nil {
		s.todo.RecordFailure(request.Action, err)
		return err
	}

	result, err := s.runtime.Client.Act(ctx, request)
	if err != nil && request.Action == "play_card" && isInvalidTargetActionError(err) {
		latestState, stateErr := s.runtime.Client.GetState(ctx)
		if stateErr == nil {
			if recovered, ok := s.actionResolver().RecoverPlayCardTarget(beforeState, latestState, request); ok {
				result, err = s.runtime.Client.Act(ctx, recovered.Request)
				if err == nil {
					beforeState = recovered.LiveState
					request = recovered.Request
				}
			}
		}
	}
	if err != nil && isInvalidActionError(err) {
		if recoveredResult, recoveredState, recoveredRequest, ok := s.recoverStaleInvalidAction(ctx, expectedState, beforeState, request, err); ok {
			result = recoveredResult
			beforeState = recoveredState
			request = recoveredRequest
			err = nil
		}
	}
	if err != nil {
		if recoveredResult, ok := s.recoverTransientActionTransport(ctx, beforeState, request, err); ok {
			result = recoveredResult
			err = nil
		}
	}
	if err != nil {
		s.todo.RecordFailure(request.Action, err)
		return err
	}
	if !result.Stable {
		result = s.settlePendingActionResult(ctx, beforeState, request, result)
	}

	beforeDigest := digestState(beforeState)
	afterDigest := digestState(&result.State)
	if !result.Stable && beforeDigest != "" && beforeDigest == afterDigest {
		if s.failures != nil {
			s.failures.Record(beforeDigest, request)
		}
	}

	s.todo.RecordAction(request.Action, beforeState, &result.State)
	s.compact.RecordAction(request.Action, &result.State, result.Message)

	s.emit(SessionEvent{
		Time:    time.Now(),
		Kind:    SessionEventAction,
		Cycle:   s.cycle,
		Attempt: s.currentAttemptForState(&result.State),
		Message: result.Message,
		Screen:  result.State.Screen,
		RunID:   result.State.RunID,
		Action:  request.Action,
		State:   &result.State,
		Data: map[string]interface{}{
			"stable": result.Stable,
			"status": result.Status,
		},
	})

	return nil
}

func (s *Session) rebindRequestForImmediateExecution(ctx context.Context, expectedState *game.StateSnapshot, liveState *game.StateSnapshot, request game.ActionRequest) (*game.StateSnapshot, game.ActionRequest, error) {
	if s == nil || s.runtime == nil || s.runtime.Client == nil {
		return liveState, request, nil
	}

	latestState, err := s.runtime.Client.GetState(ctx)
	if err != nil || latestState == nil {
		return liveState, request, nil
	}
	latestState, err = s.freshActionableStateForExecution(ctx, latestState)
	if err != nil {
		return nil, request, err
	}

	needsResolve := digestState(latestState) != digestState(liveState) || ValidateActionRequest(latestState, request) != nil
	if !needsResolve {
		return latestState, request, nil
	}

	if expectedState != nil {
		latestState, _ = s.stabilizeLiveStateForExecution(ctx, expectedState, latestState)
	}

	resolved, err := s.actionResolver().ResolveRequest(expectedState, latestState, request)
	if err != nil {
		return nil, request, err
	}
	return resolved.LiveState, resolved.Request, nil
}

func (s *Session) recoverStaleInvalidAction(ctx context.Context, expectedState *game.StateSnapshot, beforeState *game.StateSnapshot, request game.ActionRequest, actErr error) (*game.ActionResult, *game.StateSnapshot, game.ActionRequest, bool) {
	if s == nil || s.runtime == nil || s.runtime.Client == nil {
		return nil, beforeState, request, false
	}

	recoveryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	liveState, err := s.runtime.Client.GetState(recoveryCtx)
	if err != nil || liveState == nil {
		return nil, beforeState, request, false
	}
	if refreshed, refreshErr := s.freshActionableStateForExecution(recoveryCtx, liveState); refreshErr == nil && refreshed != nil {
		liveState = refreshed
	}

	resolved, err := s.actionResolver().ResolveRequest(expectedState, liveState, request)
	if err != nil || sameActionRequest(request, resolved.Request) && digestState(beforeState) == digestState(resolved.LiveState) {
		return nil, beforeState, request, false
	}

	s.emit(SessionEvent{
		Time:    time.Now(),
		Kind:    SessionEventStatus,
		Cycle:   s.cycle,
		Attempt: s.currentAttemptForState(resolved.LiveState),
		Message: s.say("action became stale before execution; retrying on the latest stable state", "action became stale before execution; retrying on the latest stable state"),
		Screen:  resolved.LiveState.Screen,
		RunID:   resolved.LiveState.RunID,
		State:   resolved.LiveState,
		Data: map[string]interface{}{
			"recovery_kind":              "invalid_action_rebind",
			"expected_state_summary":     decisionStateSummary(expectedState),
			"live_state_summary":         decisionStateSummary(resolved.LiveState),
			"expected_state_fingerprint": digestState(expectedState),
			"live_state_fingerprint":     digestState(resolved.LiveState),
			"original_error":             actErr.Error(),
		},
	})

	result, err := s.runtime.Client.Act(recoveryCtx, resolved.Request)
	if err != nil {
		return nil, beforeState, request, false
	}
	return result, resolved.LiveState, resolved.Request, true
}

func (s *Session) recoverTransientActionTransport(ctx context.Context, beforeState *game.StateSnapshot, request game.ActionRequest, actErr error) (*game.ActionResult, bool) {
	if !isTransientActionTransportError(actErr) || s == nil || s.runtime == nil || s.runtime.Client == nil {
		return nil, false
	}

	recoveryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	liveState, err := s.runtime.Client.GetState(recoveryCtx)
	if err != nil || liveState == nil {
		return nil, false
	}
	if refreshed, refreshErr := s.freshActionableStateForExecution(recoveryCtx, liveState); refreshErr == nil && refreshed != nil {
		liveState = refreshed
	}

	if beforeState != nil && digestState(beforeState) == digestState(liveState) && ValidateActionRequest(liveState, request) == nil {
		return nil, false
	}

	s.emit(SessionEvent{
		Time:    time.Now(),
		Kind:    SessionEventStatus,
		Cycle:   s.cycle,
		Attempt: s.currentAttemptForState(liveState),
		Message: s.say("action response stream dropped; continuing from the latest live state", "action response stream dropped; continuing from the latest live state"),
		Screen:  liveState.Screen,
		RunID:   liveState.RunID,
		State:   liveState,
		Data: map[string]interface{}{
			"provider_state":             s.providerState,
			"recovery_kind":              "transport_recover",
			"expected_state_summary":     decisionStateSummary(beforeState),
			"live_state_summary":         decisionStateSummary(liveState),
			"expected_state_fingerprint": digestState(beforeState),
			"live_state_fingerprint":     digestState(liveState),
		},
	})

	return &game.ActionResult{
		Action:  request.Action,
		Status:  "completed_after_transport_recover",
		Stable:  game.IsActionableState(liveState),
		Message: s.say("Action recovered from a dropped bridge response.", "Action recovered from a dropped bridge response."),
		State:   *liveState,
	}, true
}

func (s *Session) settlePendingActionResult(ctx context.Context, beforeState *game.StateSnapshot, request game.ActionRequest, result *game.ActionResult) *game.ActionResult {
	if s == nil || s.runtime == nil || s.runtime.Client == nil || result == nil || result.Stable {
		return result
	}

	waitCtx, cancel := context.WithTimeout(ctx, executionDriftWaitTimeout)
	defer cancel()

	settled, err := s.runtime.Client.WaitUntilActionable(waitCtx, executionDriftWaitTimeout)
	if err != nil || settled == nil {
		return result
	}

	beforeDigest := digestState(beforeState)
	settledDigest := digestState(settled)
	if beforeDigest == "" || settledDigest == "" {
		return result
	}
	if beforeDigest == settledDigest && strings.EqualFold(strings.TrimSpace(request.Action), "claim_reward") {
		return result
	}

	settledResult := *result
	settledResult.State = *settled
	settledResult.Stable = true
	settledResult.Status = "completed_after_wait"
	settledResult.Message = s.say(
		"Action completed after transition settled.",
		"Action completed after transition settled.",
	)
	return &settledResult
}

func (s *Session) actionResolver() *ActionResolutionPipeline {
	if s == nil || s.resolver == nil {
		return NewActionResolutionPipeline()
	}
	return s.resolver
}

func (s *Session) stateGate() *StableStateGate {
	if s == nil {
		return nil
	}
	if s.gate != nil {
		return s.gate
	}
	if s.runtime == nil {
		return nil
	}
	return NewStableStateGate(s.runtime.Client)
}

func (s *Session) freshActionableStateForExecution(ctx context.Context, candidate *game.StateSnapshot) (*game.StateSnapshot, error) {
	gate := s.stateGate()
	if gate == nil {
		return candidate, nil
	}
	return gate.FreshActionableStateForExecution(ctx, candidate, executionDriftWaitTimeout)
}

func remapActionRequestForLiveState(expectedState *game.StateSnapshot, liveState *game.StateSnapshot, request game.ActionRequest) (game.ActionRequest, string, bool) {
	resolved, err := NewActionResolutionPipeline().ResolveRequest(expectedState, liveState, request)
	if err != nil || resolved == nil || resolved.RecoveryKind == "" {
		return game.ActionRequest{}, "", false
	}
	return resolved.Request, resolved.RecoveryKind, true
}

func (s *Session) stabilizeLiveStateForExecution(ctx context.Context, expectedState *game.StateSnapshot, liveState *game.StateSnapshot) (*game.StateSnapshot, string) {
	gate := s.stateGate()
	if gate == nil {
		return liveState, classifyStateDrift(expectedState, liveState)
	}
	return gate.StabilizeExecutionDrift(ctx, expectedState, liveState, executionDriftWaitTimeout)
}

func shouldForceExecutionReread(driftKind string) bool {
	switch driftKind {
	case driftKindSameScreenIndexDrift, driftKindActionWindowChanged:
		return true
	default:
		return false
	}
}

func shouldStabilizeExecutionDrift(driftKind string) bool {
	switch {
	case driftKind == driftKindRewardTransition,
		driftKind == driftKindSelectionSeam,
		driftKind == driftKindSameScreenIndexDrift,
		driftKind == driftKindActionWindowChanged,
		driftKind == driftKindSameScreenStateDrift:
		return true
	case strings.HasPrefix(driftKind, driftKindScreenTransition+":reward"),
		strings.HasPrefix(driftKind, driftKindScreenTransition+":card_selection"),
		strings.HasPrefix(driftKind, driftKindScreenTransition+":game_over"):
		return true
	default:
		return false
	}
}

func remapPlayCardRequestOnFailure(expectedState *game.StateSnapshot, liveState *game.StateSnapshot, request game.ActionRequest) (game.ActionRequest, *game.StateSnapshot, bool) {
	resolved, ok := NewActionResolutionPipeline().RecoverPlayCardTarget(expectedState, liveState, request)
	if !ok || resolved == nil {
		return game.ActionRequest{}, nil, false
	}
	return resolved.Request, resolved.LiveState, true
}

func sameActionRequest(left game.ActionRequest, right game.ActionRequest) bool {
	if left.Action != right.Action {
		return false
	}
	if !sameOptionalInt(left.CardIndex, right.CardIndex) {
		return false
	}
	if !sameOptionalInt(left.TargetIndex, right.TargetIndex) {
		return false
	}
	if !sameOptionalInt(left.OptionIndex, right.OptionIndex) {
		return false
	}
	return true
}

func sameOptionalInt(left *int, right *int) bool {
	switch {
	case left == nil && right == nil:
		return true
	case left == nil || right == nil:
		return false
	default:
		return *left == *right
	}
}

func isInvalidTargetActionError(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "invalid_target")
}

func isInvalidActionError(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "invalid_action")
}

func (s *Session) executeModelDecision(ctx context.Context, state *game.StateSnapshot, decision *ActionDecision) error {
	liveState, err := s.runtime.Client.GetState(ctx)
	if err != nil {
		return err
	}
	liveState, err = s.freshActionableStateForExecution(ctx, liveState)
	if err != nil {
		return err
	}
	if digestState(liveState) != digestState(state) {
		liveState, _ = s.stabilizeLiveStateForExecution(ctx, state, liveState)
	}

	resolved, err := s.actionResolver().ResolveDecision(state, liveState, decision)
	if err != nil {
		driftKind := classifyStateDrift(state, liveState)
		softReplan := isSoftReplanDriftKind(driftKind)
		if !softReplan {
			s.todo.RecordFailure("model_decision", err)
		}
		recoveryKind := "hard_replan"
		if softReplan {
			recoveryKind = "soft_replan"
		}
		message := s.sayf("live state changed before model action execution; replanning (%s)", "live state changed before model action execution; replanning (%s)", driftKind)
		if softReplan {
			message = s.sayf("live state shifted along a known seam; replanning (%s)", "live state shifted along a known seam; replanning (%s)", driftKind)
		}
		s.emit(SessionEvent{
			Time:    time.Now(),
			Kind:    SessionEventStatus,
			Cycle:   s.cycle,
			Attempt: s.currentAttemptForState(liveState),
			Message: message,
			Screen:  liveState.Screen,
			RunID:   liveState.RunID,
			State:   liveState,
			Data: map[string]interface{}{
				"expected_state_summary":     decisionStateSummary(state),
				"live_state_summary":         decisionStateSummary(liveState),
				"expected_state_fingerprint": digestState(state),
				"live_state_fingerprint":     digestState(liveState),
				"drift_kind":                 driftKind,
				"recovery_kind":              recoveryKind,
				"decision_reused":            false,
			},
		})
		if softReplan {
			err = markSoftReplanReported(err)
		}
		return err
	}

	request := resolved.Request
	if resolved.RecoveryKind != "" {
		driftKind := classifyStateDrift(state, liveState)
		if !shouldQuietDecisionReuse(driftKind, resolved.RecoveryKind, resolved.OriginalRequest, request) {
			s.emit(SessionEvent{
				Time:    time.Now(),
				Kind:    SessionEventStatus,
				Cycle:   s.cycle,
				Attempt: s.currentAttemptForState(liveState),
				Message: s.say("live state changed before model action execution; reusing the decision on the latest legal state", "live state changed before model action execution; reusing the decision on the latest legal state"),
				Screen:  liveState.Screen,
				RunID:   liveState.RunID,
				State:   liveState,
				Data: map[string]interface{}{
					"expected_state_summary":     decisionStateSummary(state),
					"live_state_summary":         decisionStateSummary(liveState),
					"expected_state_fingerprint": digestState(state),
					"live_state_fingerprint":     digestState(liveState),
					"drift_kind":                 driftKind,
					"recovery_kind":              resolved.RecoveryKind,
					"decision_reused":            true,
				},
			})
		}
	}
	if decision != nil && strings.TrimSpace(decision.Reason) != "" {
		s.emit(SessionEvent{
			Time:    time.Now(),
			Kind:    SessionEventAction,
			Cycle:   s.cycle,
			Attempt: s.currentAttemptForState(state),
			Message: decision.Reason,
			Screen:  state.Screen,
			RunID:   state.RunID,
			Action:  decision.Action,
		})
	}

	return s.executeDirectAction(ctx, resolved.LiveState, request)
}

func (s *Session) readActionableState(ctx context.Context) (*game.StateSnapshot, error) {
	state, err := s.runtime.Client.GetState(ctx)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(state.Screen, "UNKNOWN") {
		if err := s.waitForBridgeReady(ctx, bridgeBootstrapWaitTimeout); err != nil {
			return nil, err
		}
	}
	gate := s.stateGate()
	if gate == nil {
		return nil, fmt.Errorf("state gate is unavailable")
	}
	stableState, err := gate.ReadStableActionableState(ctx, state, stateWaitTimeout)
	if err != nil {
		return nil, err
	}
	if stableState != nil {
		return stableState, nil
	}
	return state, nil
}

func (s *Session) waitForBridgeReady(ctx context.Context, timeout time.Duration) error {
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		health, err := s.runtime.Client.GetHealth(waitCtx)
		if err == nil && health.Ready {
			return nil
		}

		select {
		case <-waitCtx.Done():
			return waitCtx.Err()
		case <-ticker.C:
		}
	}
}
