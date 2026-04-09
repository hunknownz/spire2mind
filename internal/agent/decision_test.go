package agentruntime

import (
	"encoding/json"
	"strings"
	"testing"

	"spire2mind/internal/game"
)

func TestValidateActionDecisionPlayCardValid(t *testing.T) {
	state := &game.StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"hand": []any{
				map[string]any{
					"index":              1,
					"playable":           true,
					"requiresTarget":     true,
					"validTargetIndices": []any{0, 2},
				},
			},
			"enemies": []any{
				map[string]any{"index": 0, "isHittable": true},
				map[string]any{"index": 2, "isHittable": true},
			},
		},
	}

	cardIndex := 1
	targetIndex := 2
	decision := &ActionDecision{
		Action:      "play_card",
		CardIndex:   &cardIndex,
		TargetIndex: &targetIndex,
	}

	if err := ValidateActionDecision(state, decision); err != nil {
		t.Fatalf("ValidateActionDecision returned unexpected error: %v", err)
	}
}

func TestValidateActionDecisionPlayCardInvalidTarget(t *testing.T) {
	state := &game.StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"hand": []any{
				map[string]any{
					"index":              1,
					"playable":           true,
					"requiresTarget":     true,
					"validTargetIndices": []any{0},
				},
			},
			"enemies": []any{
				map[string]any{"index": 0, "isHittable": true},
			},
		},
	}

	cardIndex := 1
	targetIndex := 2
	decision := &ActionDecision{
		Action:      "play_card",
		CardIndex:   &cardIndex,
		TargetIndex: &targetIndex,
	}

	err := ValidateActionDecision(state, decision)
	if err == nil {
		t.Fatal("ValidateActionDecision returned nil, want invalid_target error")
	}
	if !strings.Contains(err.Error(), "invalid_target") {
		t.Fatalf("ValidateActionDecision returned %q, want invalid_target", err)
	}
}

func TestValidateActionDecisionPlayCardRequiresTargetFromTargetMetadata(t *testing.T) {
	state := &game.StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"hand": []any{
				map[string]any{
					"index":              1,
					"playable":           true,
					"requiresTarget":     false,
					"targetType":         "AnyEnemy",
					"validTargetIndices": []any{0},
				},
			},
			"enemies": []any{
				map[string]any{"index": 0, "isHittable": true},
			},
		},
	}

	cardIndex := 1
	decision := &ActionDecision{
		Action:    "play_card",
		CardIndex: &cardIndex,
	}

	err := ValidateActionDecision(state, decision)
	if err == nil {
		t.Fatal("ValidateActionDecision returned nil, want invalid_target error")
	}
	if !strings.Contains(err.Error(), "invalid_target") {
		t.Fatalf("ValidateActionDecision returned %q, want invalid_target", err)
	}
}

func TestValidateActionDecisionChooseMapNode(t *testing.T) {
	state := &game.StateSnapshot{
		Screen:           "MAP",
		AvailableActions: []string{"choose_map_node"},
		Map: map[string]any{
			"availableNodes": []any{
				map[string]any{"index": 0},
				map[string]any{"index": 2},
			},
		},
	}

	optionIndex := 1
	decision := &ActionDecision{
		Action:      "choose_map_node",
		OptionIndex: &optionIndex,
	}

	err := ValidateActionDecision(state, decision)
	if err == nil {
		t.Fatal("ValidateActionDecision returned nil, want invalid_action error")
	}
	if !strings.Contains(err.Error(), "invalid_action") {
		t.Fatalf("ValidateActionDecision returned %q, want invalid_action", err)
	}
}

func TestValidateActionDecisionChooseFinishedEventWithoutIndex(t *testing.T) {
	state := &game.StateSnapshot{
		Screen:           "EVENT",
		AvailableActions: []string{"choose_event_option"},
		Event: map[string]any{
			"isFinished": true,
			"options":    []any{},
		},
	}

	decision := &ActionDecision{Action: "choose_event_option"}
	if err := ValidateActionDecision(state, decision); err != nil {
		t.Fatalf("ValidateActionDecision returned unexpected error: %v", err)
	}
}

func TestParseActionDecisionNormalizesCardIndexForRewardChoice(t *testing.T) {
	raw := map[string]any{
		"action":     "choose_reward_card",
		"card_index": json.Number("2"),
	}

	decision, err := ParseActionDecision(raw, "")
	if err != nil {
		t.Fatalf("ParseActionDecision returned unexpected error: %v", err)
	}

	if decision.OptionIndex == nil || *decision.OptionIndex != 2 {
		t.Fatalf("ParseActionDecision did not normalize option_index: %#v", decision)
	}
}

func TestParseActionDecisionNormalizesCardIndexForClaimReward(t *testing.T) {
	raw := map[string]any{
		"action":     "claim_reward",
		"card_index": json.Number("1"),
	}

	decision, err := ParseActionDecision(raw, "")
	if err != nil {
		t.Fatalf("ParseActionDecision returned unexpected error: %v", err)
	}

	if decision.OptionIndex == nil || *decision.OptionIndex != 1 {
		t.Fatalf("ParseActionDecision did not normalize option_index for claim_reward: %#v", decision)
	}
}

func TestParseActionDecisionNormalizesCardIndexForShopChoices(t *testing.T) {
	tests := []struct {
		name   string
		action string
	}{
		{name: "buy card", action: "buy_card"},
		{name: "buy relic", action: "buy_relic"},
		{name: "buy potion", action: "buy_potion"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := map[string]any{
				"action":     tt.action,
				"card_index": json.Number("4"),
			}

			decision, err := ParseActionDecision(raw, "")
			if err != nil {
				t.Fatalf("ParseActionDecision returned unexpected error: %v", err)
			}
			if decision.OptionIndex == nil || *decision.OptionIndex != 4 {
				t.Fatalf("ParseActionDecision did not normalize option_index: %#v", decision)
			}
		})
	}
}

func TestParseActionDecisionNormalizesTargetIndexForMapChoice(t *testing.T) {
	raw := map[string]any{
		"action":       "choose_map_node",
		"target_index": json.Number("3"),
	}

	decision, err := ParseActionDecision(raw, "")
	if err != nil {
		t.Fatalf("ParseActionDecision returned unexpected error: %v", err)
	}

	if decision.OptionIndex == nil || *decision.OptionIndex != 3 {
		t.Fatalf("ParseActionDecision did not normalize option_index for map choice: %#v", decision)
	}
	if decision.TargetIndex != nil {
		t.Fatalf("ParseActionDecision should clear target_index for map choice: %#v", decision)
	}
}

func TestParseActionDecisionAcceptsNodeIndexAliasForMapChoice(t *testing.T) {
	raw := map[string]any{
		"action":     "choose_map_node",
		"node_index": json.Number("1"),
	}

	decision, err := ParseActionDecision(raw, "")
	if err != nil {
		t.Fatalf("ParseActionDecision returned unexpected error: %v", err)
	}

	if decision.OptionIndex == nil || *decision.OptionIndex != 1 {
		t.Fatalf("ParseActionDecision did not normalize node_index alias: %#v", decision)
	}
}

func TestParseActionDecisionNormalizesOptionIndexForPlayCard(t *testing.T) {
	raw := map[string]any{
		"action":       "play_card",
		"option_index": json.Number("4"),
	}

	decision, err := ParseActionDecision(raw, "")
	if err != nil {
		t.Fatalf("ParseActionDecision returned unexpected error: %v", err)
	}

	if decision.CardIndex == nil || *decision.CardIndex != 4 {
		t.Fatalf("ParseActionDecision did not normalize card_index: %#v", decision)
	}
	if decision.OptionIndex != nil {
		t.Fatalf("ParseActionDecision should clear option_index for play_card: %#v", decision)
	}
}

func TestParseActionDecisionFallsBackToLooseJSONExtraction(t *testing.T) {
	fallback := `{"action":"choose_reward_card","card_index":2,"reason":"Stone Armor"鍟�}`

	decision, err := ParseActionDecision(nil, fallback)
	if err != nil {
		t.Fatalf("ParseActionDecision returned unexpected error: %v", err)
	}
	if decision.Action != "choose_reward_card" {
		t.Fatalf("unexpected action: %#v", decision)
	}
	if decision.OptionIndex == nil || *decision.OptionIndex != 2 {
		t.Fatalf("expected option_index 2, got %#v", decision)
	}
}

func TestNormalizeActionRequestForStateDropsUnusedCombatTarget(t *testing.T) {
	cardIndex := 2
	targetIndex := 0
	request := game.ActionRequest{
		Action:      "play_card",
		CardIndex:   &cardIndex,
		TargetIndex: &targetIndex,
	}

	state := &game.StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card"},
		Combat: map[string]any{
			"hand": []any{
				map[string]any{
					"index":          2,
					"playable":       true,
					"requiresTarget": false,
				},
			},
		},
	}

	normalized := NormalizeActionRequestForState(state, request)
	if normalized.TargetIndex != nil {
		t.Fatalf("expected target index to be cleared, got %+v", normalized)
	}
}

func TestNormalizeActionRequestForStatePinsSingleValidTarget(t *testing.T) {
	cardIndex := 2
	targetIndex := 9
	request := game.ActionRequest{
		Action:      "play_card",
		CardIndex:   &cardIndex,
		TargetIndex: &targetIndex,
	}

	state := &game.StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card"},
		Combat: map[string]any{
			"hand": []any{
				map[string]any{
					"index":              2,
					"playable":           true,
					"requiresTarget":     true,
					"validTargetIndices": []any{0},
				},
			},
		},
	}

	normalized := NormalizeActionRequestForState(state, request)
	if normalized.TargetIndex == nil || *normalized.TargetIndex != 0 {
		t.Fatalf("expected target index to normalize to 0, got %+v", normalized)
	}
}

func TestNormalizeActionRequestForStateNormalizesMapChoiceTargetIndex(t *testing.T) {
	targetIndex := 2
	request := game.ActionRequest{
		Action:      "choose_map_node",
		TargetIndex: &targetIndex,
	}

	normalized := NormalizeActionRequestForState(nil, request)
	if normalized.OptionIndex == nil || *normalized.OptionIndex != 2 {
		t.Fatalf("expected option index to normalize to 2, got %+v", normalized)
	}
	if normalized.TargetIndex != nil || normalized.CardIndex != nil {
		t.Fatalf("expected non-option indexes to be cleared, got %+v", normalized)
	}
}

func TestNormalizeActionRequestForStateNormalizesRewardChoiceCardIndex(t *testing.T) {
	cardIndex := 1
	request := game.ActionRequest{
		Action:    "choose_reward_card",
		CardIndex: &cardIndex,
	}

	normalized := NormalizeActionRequestForState(nil, request)
	if normalized.OptionIndex == nil || *normalized.OptionIndex != 1 {
		t.Fatalf("expected option index to normalize to 1, got %+v", normalized)
	}
	if normalized.TargetIndex != nil || normalized.CardIndex != nil {
		t.Fatalf("expected non-option indexes to be cleared, got %+v", normalized)
	}
}

func TestNormalizeActionRequestForStateUsesAnyEnemyFallbackTargets(t *testing.T) {
	cardIndex := 2
	request := game.ActionRequest{
		Action:    "play_card",
		CardIndex: &cardIndex,
	}

	state := &game.StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card"},
		Combat: map[string]any{
			"hand": []any{
				map[string]any{
					"index":          2,
					"playable":       true,
					"requiresTarget": true,
					"targetType":     "AnyEnemy",
				},
			},
			"enemies": []any{
				map[string]any{"index": 0, "isHittable": true},
			},
		},
	}

	normalized := NormalizeActionRequestForState(state, request)
	if normalized.TargetIndex == nil || *normalized.TargetIndex != 0 {
		t.Fatalf("expected AnyEnemy fallback target 0, got %+v", normalized)
	}
}

func TestNormalizeActionRequestForStatePinsImplicitTargetedCard(t *testing.T) {
	cardIndex := 2
	request := game.ActionRequest{
		Action:    "play_card",
		CardIndex: &cardIndex,
	}

	state := &game.StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card"},
		Combat: map[string]any{
			"hand": []any{
				map[string]any{
					"index":          2,
					"playable":       true,
					"requiresTarget": false,
					"targetType":     "AnyEnemy",
				},
			},
			"enemies": []any{
				map[string]any{"index": 0, "isHittable": true},
			},
		},
	}

	normalized := NormalizeActionRequestForState(state, request)
	if normalized.TargetIndex == nil || *normalized.TargetIndex != 0 {
		t.Fatalf("expected implicit target card to normalize to target 0, got %+v", normalized)
	}
}

func TestNormalizeActionRequestForStatePinsSingleClaimableReward(t *testing.T) {
	request := game.ActionRequest{
		Action: "claim_reward",
	}

	state := &game.StateSnapshot{
		Screen:           "REWARD",
		AvailableActions: []string{"claim_reward", "proceed"},
		Reward: map[string]any{
			"rewards": []any{
				map[string]any{"index": 0, "claimable": true},
			},
		},
	}

	normalized := NormalizeActionRequestForState(state, request)
	if normalized.OptionIndex == nil || *normalized.OptionIndex != 0 {
		t.Fatalf("expected claim_reward to pin sole claimable reward, got %+v", normalized)
	}
}

func TestNormalizeActionRequestForStateDoesNotGuessUnsupportedTargets(t *testing.T) {
	cardIndex := 2
	request := game.ActionRequest{
		Action:    "play_card",
		CardIndex: &cardIndex,
	}

	state := &game.StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card"},
		Combat: map[string]any{
			"hand": []any{
				map[string]any{
					"index":          2,
					"playable":       true,
					"requiresTarget": true,
					"targetType":     "AnyAlly",
				},
			},
			"enemies": []any{
				map[string]any{"index": 0, "isHittable": true},
			},
		},
	}

	normalized := NormalizeActionRequestForState(state, request)
	if normalized.TargetIndex != nil {
		t.Fatalf("expected unsupported target type to remain unset, got %+v", normalized)
	}
}

func TestNormalizeActionDecisionForStatePinsSingleValidTarget(t *testing.T) {
	cardIndex := 2
	targetIndex := 9
	decision := &ActionDecision{
		Action:      "play_card",
		CardIndex:   &cardIndex,
		TargetIndex: &targetIndex,
		Reason:      "test",
	}

	state := &game.StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card"},
		Combat: map[string]any{
			"hand": []any{
				map[string]any{
					"index":              2,
					"playable":           true,
					"requiresTarget":     true,
					"validTargetIndices": []any{0},
				},
			},
		},
	}

	normalized := NormalizeActionDecisionForState(state, decision)
	if normalized == nil {
		t.Fatal("expected normalized decision")
	}
	if normalized.TargetIndex == nil || *normalized.TargetIndex != 0 {
		t.Fatalf("expected target index to normalize to 0, got %#v", normalized)
	}
	if normalized.Reason != decision.Reason {
		t.Fatalf("expected reason %q to be preserved, got %q", decision.Reason, normalized.Reason)
	}
}

func TestNormalizeActionDecisionForStateClearsUnusedTarget(t *testing.T) {
	cardIndex := 2
	targetIndex := 1
	decision := &ActionDecision{
		Action:      "play_card",
		CardIndex:   &cardIndex,
		TargetIndex: &targetIndex,
	}

	state := &game.StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card"},
		Combat: map[string]any{
			"hand": []any{
				map[string]any{
					"index":          2,
					"playable":       true,
					"requiresTarget": false,
				},
			},
		},
	}

	normalized := NormalizeActionDecisionForState(state, decision)
	if normalized == nil {
		t.Fatal("expected normalized decision")
	}
	if normalized.TargetIndex != nil {
		t.Fatalf("expected unused target index to be cleared, got %#v", normalized)
	}
}

func TestReuseDecisionOnLiveStateAllowsCompatibleDrift(t *testing.T) {
	cardIndex := 0
	targetIndex := 0
	decision := &ActionDecision{
		Action:      "play_card",
		CardIndex:   &cardIndex,
		TargetIndex: &targetIndex,
	}

	expectedState := &game.StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"hand": []any{
				map[string]any{
					"index":              0,
					"id":                 "strike",
					"name":               "Strike",
					"playable":           true,
					"requiresTarget":     true,
					"validTargetIndices": []any{0},
				},
			},
			"enemies": []any{
				map[string]any{
					"index":      0,
					"id":         "slime-small",
					"name":       "Slime",
					"isHittable": true,
					"currentHp":  34,
				},
			},
		},
	}

	liveState := &game.StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"hand": []any{
				map[string]any{
					"index":              1,
					"id":                 "strike",
					"name":               "Strike",
					"playable":           true,
					"requiresTarget":     true,
					"validTargetIndices": []any{2},
				},
				map[string]any{
					"index":          0,
					"id":             "defend",
					"name":           "Defend",
					"playable":       true,
					"requiresTarget": false,
				},
			},
			"enemies": []any{
				map[string]any{
					"index":      2,
					"id":         "slime-small",
					"name":       "Slime",
					"isHittable": true,
					"currentHp":  34,
				},
			},
		},
	}

	request, recoveryKind, ok := reuseDecisionOnLiveState(expectedState, liveState, decision)
	if !ok {
		t.Fatal("expected decision to remain reusable on live state")
	}
	if recoveryKind != "decision_remap" {
		t.Fatalf("expected decision_remap, got %q", recoveryKind)
	}
	if request.CardIndex == nil || *request.CardIndex != 1 {
		t.Fatalf("unexpected reused request: %+v", request)
	}
	if request.TargetIndex == nil || *request.TargetIndex != 2 {
		t.Fatalf("expected target to remap to 2, got %+v", request)
	}
}

func TestReuseDecisionOnLiveStateRejectsInvalidDrift(t *testing.T) {
	cardIndex := 0
	targetIndex := 0
	decision := &ActionDecision{
		Action:      "play_card",
		CardIndex:   &cardIndex,
		TargetIndex: &targetIndex,
	}

	expectedState := &game.StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"hand": []any{
				map[string]any{
					"index":              0,
					"id":                 "strike",
					"name":               "Strike",
					"playable":           true,
					"requiresTarget":     true,
					"validTargetIndices": []any{0},
				},
			},
			"enemies": []any{
				map[string]any{"index": 0, "id": "slime-a", "name": "Slime", "isHittable": true},
			},
		},
	}

	liveState := &game.StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"hand": []any{
				map[string]any{
					"index":              0,
					"id":                 "defend",
					"name":               "Defend",
					"playable":           true,
					"requiresTarget":     true,
					"validTargetIndices": []any{1},
				},
			},
			"enemies": []any{
				map[string]any{"index": 1, "isHittable": true},
			},
		},
	}

	if _, _, ok := reuseDecisionOnLiveState(expectedState, liveState, decision); ok {
		t.Fatal("expected invalid target drift to force a replan")
	}
}

func TestShouldQuietDecisionReuseForCompatibleSameScreenDrift(t *testing.T) {
	cardIndex := 1
	targetIndex := 0
	original := game.ActionRequest{
		Action:      "play_card",
		CardIndex:   &cardIndex,
		TargetIndex: &targetIndex,
	}

	if !shouldQuietDecisionReuse(driftKindSameScreenIndexDrift, "decision_reuse", original, original) {
		t.Fatal("expected compatible same-screen decision reuse to be quiet")
	}
	if !shouldQuietDecisionReuse(driftKindSameScreenStateDrift, "decision_reuse", original, original) {
		t.Fatal("expected compatible same-screen state drift reuse to be quiet")
	}
	if !shouldQuietDecisionReuse(driftKindActionWindowChanged, "decision_reuse", original, original) {
		t.Fatal("expected compatible action-window drift reuse to be quiet")
	}
	if shouldQuietDecisionReuse(driftKindRewardTransition, "decision_reuse", original, original) {
		t.Fatal("did not expect reward transition reuse to be quiet")
	}
	remappedTarget := 2
	changed := game.ActionRequest{
		Action:      "play_card",
		CardIndex:   &cardIndex,
		TargetIndex: &remappedTarget,
	}
	if shouldQuietDecisionReuse(driftKindSameScreenIndexDrift, "decision_reuse", original, changed) {
		t.Fatal("did not expect changed request to be quiet")
	}
	if shouldQuietDecisionReuse(driftKindSameScreenIndexDrift, "decision_remap", original, original) {
		t.Fatal("did not expect remapped reuse to be quiet")
	}
}

func TestReuseDecisionOnLiveStateRemapsRewardCardByCardIdentity(t *testing.T) {
	optionIndex := 0
	decision := &ActionDecision{
		Action:      "choose_reward_card",
		OptionIndex: &optionIndex,
	}

	expectedState := &game.StateSnapshot{
		Screen:           "REWARD",
		AvailableActions: []string{"choose_reward_card", "skip_reward_cards"},
		Reward: map[string]any{
			"phase": "card_choice",
			"cardOptions": []any{
				map[string]any{"index": 0, "cardId": "armaments", "name": "Armaments", "energyCost": 1},
				map[string]any{"index": 1, "cardId": "pommel_strike", "name": "Pommel Strike", "energyCost": 1},
			},
		},
	}

	liveState := &game.StateSnapshot{
		Screen:           "REWARD",
		AvailableActions: []string{"choose_reward_card", "skip_reward_cards"},
		Reward: map[string]any{
			"phase": "card_choice",
			"cardOptions": []any{
				map[string]any{"index": 1, "cardId": "armaments", "name": "Armaments", "energyCost": 1},
				map[string]any{"index": 0, "cardId": "pommel_strike", "name": "Pommel Strike", "energyCost": 1},
			},
		},
	}

	request, recoveryKind, ok := reuseDecisionOnLiveState(expectedState, liveState, decision)
	if !ok {
		t.Fatal("expected reward card decision to be remapped")
	}
	if recoveryKind != "decision_remap" {
		t.Fatalf("expected decision_remap, got %q", recoveryKind)
	}
	if request.OptionIndex == nil || *request.OptionIndex != 1 {
		t.Fatalf("expected reward option to remap to 1, got %+v", request)
	}
}

func TestReuseDecisionOnLiveStateRemapsSelectionCardByCardIdentity(t *testing.T) {
	optionIndex := 1
	decision := &ActionDecision{
		Action:      "select_deck_card",
		OptionIndex: &optionIndex,
	}

	expectedState := &game.StateSnapshot{
		Screen:           "CARD_SELECTION",
		AvailableActions: []string{"select_deck_card"},
		Selection: map[string]any{
			"kind": "upgrade",
			"cards": []any{
				map[string]any{"index": 0, "cardId": "strike", "name": "Strike", "energyCost": 1},
				map[string]any{"index": 1, "cardId": "defend", "name": "Defend", "energyCost": 1},
			},
		},
	}

	liveState := &game.StateSnapshot{
		Screen:           "CARD_SELECTION",
		AvailableActions: []string{"select_deck_card"},
		Selection: map[string]any{
			"kind": "upgrade",
			"cards": []any{
				map[string]any{"index": 5, "cardId": "defend", "name": "Defend", "energyCost": 1},
				map[string]any{"index": 4, "cardId": "strike", "name": "Strike", "energyCost": 1},
			},
		},
	}

	request, recoveryKind, ok := reuseDecisionOnLiveState(expectedState, liveState, decision)
	if !ok {
		t.Fatal("expected selection decision to be remapped")
	}
	if recoveryKind != "decision_remap" {
		t.Fatalf("expected decision_remap, got %q", recoveryKind)
	}
	if request.OptionIndex == nil || *request.OptionIndex != 5 {
		t.Fatalf("expected selection option to remap to 5, got %+v", request)
	}
}

func TestRemapPlayCardRequestOnFailurePinsSingleRemainingTarget(t *testing.T) {
	cardIndex := 0
	targetIndex := 0
	request := game.ActionRequest{
		Action:      "play_card",
		CardIndex:   &cardIndex,
		TargetIndex: &targetIndex,
	}

	expectedState := &game.StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"hand": []any{
				map[string]any{
					"index":              0,
					"id":                 "strike",
					"name":               "Strike",
					"playable":           true,
					"requiresTarget":     true,
					"validTargetIndices": []any{0, 1},
				},
			},
			"enemies": []any{
				map[string]any{"index": 0, "id": "slime-a", "name": "Slime A", "isHittable": true},
				map[string]any{"index": 1, "id": "slime-b", "name": "Slime B", "isHittable": true},
			},
		},
	}

	liveState := &game.StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"hand": []any{
				map[string]any{
					"index":              0,
					"id":                 "strike",
					"name":               "Strike",
					"playable":           true,
					"requiresTarget":     true,
					"validTargetIndices": []any{1},
				},
			},
			"enemies": []any{
				map[string]any{"index": 1, "id": "slime-b", "name": "Slime B", "isHittable": true},
			},
		},
	}

	remapped, remapState, ok := remapPlayCardRequestOnFailure(expectedState, liveState, request)
	if !ok {
		t.Fatal("expected invalid target recovery to retry on single remaining target")
	}
	if remapState != liveState {
		t.Fatal("expected live state to be returned for retry")
	}
	if remapped.TargetIndex == nil || *remapped.TargetIndex != 1 {
		t.Fatalf("expected target to pin to 1, got %+v", remapped)
	}
}

func TestReuseDecisionOnLiveStateRemapsDuplicateSelectionCardsByOrdinal(t *testing.T) {
	optionIndex := 1
	decision := &ActionDecision{
		Action:      "select_deck_card",
		OptionIndex: &optionIndex,
	}

	expectedState := &game.StateSnapshot{
		Screen:           "CARD_SELECTION",
		AvailableActions: []string{"select_deck_card"},
		Selection: map[string]any{
			"kind": "upgrade",
			"cards": []any{
				map[string]any{"index": 0, "cardId": "strike", "name": "Strike", "energyCost": 1},
				map[string]any{"index": 1, "cardId": "strike", "name": "Strike", "energyCost": 1},
				map[string]any{"index": 2, "cardId": "defend", "name": "Defend", "energyCost": 1},
			},
		},
	}

	liveState := &game.StateSnapshot{
		Screen:           "CARD_SELECTION",
		AvailableActions: []string{"select_deck_card"},
		Selection: map[string]any{
			"kind": "upgrade",
			"cards": []any{
				map[string]any{"index": 7, "cardId": "strike", "name": "Strike", "energyCost": 1},
				map[string]any{"index": 8, "cardId": "defend", "name": "Defend", "energyCost": 1},
				map[string]any{"index": 9, "cardId": "strike", "name": "Strike", "energyCost": 1},
			},
		},
	}

	request, recoveryKind, ok := reuseDecisionOnLiveState(expectedState, liveState, decision)
	if !ok {
		t.Fatal("expected duplicate selection card decision to be remapped")
	}
	if recoveryKind != "decision_remap" {
		t.Fatalf("expected decision_remap, got %q", recoveryKind)
	}
	if request.OptionIndex == nil || *request.OptionIndex != 9 {
		t.Fatalf("expected second matching strike to remap to 9, got %+v", request)
	}
}

func TestReuseDecisionOnLiveStateRemapsDuplicateEnemyTargetsByOrdinal(t *testing.T) {
	cardIndex := 0
	targetIndex := 1
	decision := &ActionDecision{
		Action:      "play_card",
		CardIndex:   &cardIndex,
		TargetIndex: &targetIndex,
	}

	expectedState := &game.StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"hand": []any{
				map[string]any{
					"index":              0,
					"cardId":             "strike",
					"name":               "Strike",
					"playable":           true,
					"requiresTarget":     true,
					"validTargetIndices": []any{0, 1},
				},
			},
			"enemies": []any{
				map[string]any{"index": 0, "enemyId": "slime-small", "name": "Slime", "isHittable": true, "currentHp": 12},
				map[string]any{"index": 1, "enemyId": "slime-small", "name": "Slime", "isHittable": true, "currentHp": 12},
			},
		},
	}

	liveState := &game.StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"hand": []any{
				map[string]any{
					"index":              4,
					"cardId":             "strike",
					"name":               "Strike",
					"playable":           true,
					"requiresTarget":     true,
					"validTargetIndices": []any{6, 8},
				},
			},
			"enemies": []any{
				map[string]any{"index": 6, "enemyId": "slime-small", "name": "Slime", "isHittable": true, "currentHp": 12},
				map[string]any{"index": 8, "enemyId": "slime-small", "name": "Slime", "isHittable": true, "currentHp": 12},
			},
		},
	}

	request, recoveryKind, ok := reuseDecisionOnLiveState(expectedState, liveState, decision)
	if !ok {
		t.Fatal("expected duplicate enemy target decision to be remapped")
	}
	if recoveryKind != "decision_remap" {
		t.Fatalf("expected decision_remap, got %q", recoveryKind)
	}
	if request.CardIndex == nil || *request.CardIndex != 4 {
		t.Fatalf("expected strike to remap to card index 4, got %+v", request)
	}
	if request.TargetIndex == nil || *request.TargetIndex != 8 {
		t.Fatalf("expected second matching slime target to remap to 8, got %+v", request)
	}
}
