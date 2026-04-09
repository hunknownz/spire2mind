package agentruntime

import "spire2mind/internal/game"

type ActionResolutionPipeline struct{}

type ActionResolutionResult struct {
	ExpectedState   *game.StateSnapshot
	LiveState       *game.StateSnapshot
	Decision        *ActionDecision
	OriginalRequest game.ActionRequest
	Request         game.ActionRequest
	RecoveryKind    string
	DecisionReused  bool
}

func NewActionResolutionPipeline() *ActionResolutionPipeline {
	return &ActionResolutionPipeline{}
}

func (p *ActionResolutionPipeline) ResolveRequest(expectedState *game.StateSnapshot, liveState *game.StateSnapshot, request game.ActionRequest) (*ActionResolutionResult, error) {
	return p.resolve(expectedState, liveState, actionRequestToDecision(request))
}

func (p *ActionResolutionPipeline) ResolveDecision(expectedState *game.StateSnapshot, liveState *game.StateSnapshot, decision *ActionDecision) (*ActionResolutionResult, error) {
	return p.resolve(expectedState, liveState, decision)
}

func (p *ActionResolutionPipeline) resolve(expectedState *game.StateSnapshot, liveState *game.StateSnapshot, decision *ActionDecision) (*ActionResolutionResult, error) {
	if decision == nil {
		return nil, ValidateActionDecision(liveState, decision)
	}

	canonical := cloneActionDecision(decision).normalize()
	originalRequest := DecisionToActionRequest(canonical)
	resolved := &ActionResolutionResult{
		ExpectedState:   expectedState,
		LiveState:       liveState,
		Decision:        canonical,
		OriginalRequest: originalRequest,
		Request:         originalRequest,
	}
	if liveState == nil {
		return resolved, ValidateActionRequest(liveState, resolved.Request)
	}

	if expectedState == nil || decisionStateDigest(liveState) == decisionStateDigest(expectedState) {
		resolved.Request = NormalizeActionRequestForState(liveState, originalRequest)
		if err := ValidateActionRequest(liveState, resolved.Request); err != nil {
			return nil, err
		}
		return resolved, nil
	}

	if request, recoveryKind, ok := reuseDecisionOnLiveState(expectedState, liveState, canonical); ok {
		resolved.Request = request
		resolved.RecoveryKind = recoveryKind
		resolved.DecisionReused = true
		return resolved, nil
	}

	normalized := NormalizeActionRequestForState(liveState, originalRequest)
	if ValidateActionRequest(liveState, normalized) == nil &&
		canNormalizeActionRequest(expectedState, liveState, originalRequest, normalized) &&
		(!sameActionRequest(originalRequest, normalized) || decisionStateDigest(liveState) != decisionStateDigest(expectedState)) {
		resolved.Request = normalized
		resolved.RecoveryKind = "request_normalize"
		resolved.DecisionReused = true
		return resolved, nil
	}

	return nil, stateUnavailableError(expectedState, liveState)
}

func canNormalizeActionRequest(expectedState *game.StateSnapshot, liveState *game.StateSnapshot, original game.ActionRequest, normalized game.ActionRequest) bool {
	if expectedState == nil || liveState == nil {
		return true
	}

	switch original.Action {
	case "play_card":
		if original.CardIndex == nil || normalized.CardIndex == nil {
			return false
		}
		expectedCard, ok := combatHandCard(expectedState, *original.CardIndex)
		if !ok {
			return false
		}
		liveCard, ok := combatHandCard(liveState, *normalized.CardIndex)
		if !ok {
			return false
		}
		if matchCardOption(expectedCard, liveCard) {
			return true
		}
		return stableIdentity(expectedCard, "cardId", "id") == "" &&
			stableIdentity(liveCard, "cardId", "id") == "" &&
			*original.CardIndex == *normalized.CardIndex
	default:
		return true
	}
}

func (p *ActionResolutionPipeline) RecoverPlayCardTarget(expectedState *game.StateSnapshot, liveState *game.StateSnapshot, request game.ActionRequest) (*ActionResolutionResult, bool) {
	if expectedState == nil || liveState == nil || request.Action != "play_card" {
		return nil, false
	}

	if resolved, err := p.ResolveRequest(expectedState, liveState, request); err == nil {
		if !sameActionRequest(request, resolved.Request) || decisionStateDigest(liveState) != decisionStateDigest(expectedState) {
			return resolved, true
		}
	}

	if request.CardIndex == nil {
		return nil, false
	}

	card, ok := combatHandCard(liveState, *request.CardIndex)
	if !ok || !cardRequiresTarget(liveState, card) {
		return nil, false
	}

	normalized := NormalizeActionRequestForState(liveState, request)
	if ValidateActionRequest(liveState, normalized) == nil &&
		(!sameActionRequest(request, normalized) || decisionStateDigest(liveState) != decisionStateDigest(expectedState)) {
		return &ActionResolutionResult{
			ExpectedState:   expectedState,
			LiveState:       liveState,
			Decision:        actionRequestToDecision(normalized),
			OriginalRequest: request,
			Request:         normalized,
			RecoveryKind:    "request_normalize",
			DecisionReused:  true,
		}, true
	}

	validTargets := combatTargetIndicesForCard(liveState, card)
	if len(validTargets) != 1 {
		return nil, false
	}

	targetIndex := validTargets[0]
	normalized.TargetIndex = &targetIndex
	if ValidateActionRequest(liveState, normalized) != nil {
		return nil, false
	}

	return &ActionResolutionResult{
		ExpectedState:   expectedState,
		LiveState:       liveState,
		Decision:        actionRequestToDecision(normalized),
		OriginalRequest: request,
		Request:         normalized,
		RecoveryKind:    "request_normalize",
		DecisionReused:  true,
	}, true
}
