package agentruntime

import (
	"strings"
	"testing"

	"spire2mind/internal/game"
)

func TestActionInvariantTargetedPlayableCardRequiresValidTargetIndex(t *testing.T) {
	t.Parallel()

	cardIndex := 2
	targetIndex := 0

	state := &game.StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"actionWindowOpen": true,
			"enemies": []any{
				map[string]any{
					"index":      0,
					"enemyId":    "SLIME_RED",
					"isHittable": true,
				},
			},
			"hand": []any{
				map[string]any{
					"index":              2,
					"cardId":             "STRIKE_RED",
					"playable":           true,
					"requiresTarget":     true,
					"targetType":         "AnyEnemy",
					"validTargetIndices": []any{0},
				},
			},
		},
	}

	err := ValidateActionRequest(state, game.ActionRequest{
		Action:    "play_card",
		CardIndex: &cardIndex,
	})
	if err == nil || !strings.Contains(err.Error(), "target_index is required") {
		t.Fatalf("expected missing target error, got %v", err)
	}

	err = ValidateActionRequest(state, game.ActionRequest{
		Action:      "play_card",
		CardIndex:   &cardIndex,
		TargetIndex: &targetIndex,
	})
	if err != nil {
		t.Fatalf("expected valid targeted play_card, got %v", err)
	}
}

func TestActionInvariantFinishedEventAllowsChooseEventOptionZero(t *testing.T) {
	t.Parallel()

	optionIndex := 0
	state := &game.StateSnapshot{
		Screen:           "EVENT",
		AvailableActions: []string{"choose_event_option"},
		Event: map[string]any{
			"isFinished": true,
			"options":    []any{},
		},
	}

	if err := ValidateActionRequest(state, game.ActionRequest{Action: "choose_event_option"}); err != nil {
		t.Fatalf("expected finished event continuation without option index to validate, got %v", err)
	}

	if err := ValidateActionRequest(state, game.ActionRequest{
		Action:      "choose_event_option",
		OptionIndex: &optionIndex,
	}); err != nil {
		t.Fatalf("expected finished event continuation with option index 0 to validate, got %v", err)
	}
}

func TestActionInvariantPendingRewardChoiceRequiresIndexedCardOption(t *testing.T) {
	t.Parallel()

	optionIndex := 4
	state := &game.StateSnapshot{
		Screen:           "REWARD",
		AvailableActions: []string{"choose_reward_card", "skip_reward_cards"},
		Reward: map[string]any{
			"phase":             "card_choice",
			"pendingCardChoice": true,
			"cardOptions": []any{
				map[string]any{
					"index":  4,
					"cardId": "STRIKE_RED",
				},
			},
		},
	}

	if err := ValidateActionRequest(state, game.ActionRequest{
		Action:      "choose_reward_card",
		OptionIndex: &optionIndex,
	}); err != nil {
		t.Fatalf("expected reward card choice to validate, got %v", err)
	}
}

func TestActionInvariantProceedIsBlockedWhileClaimableRewardsRemain(t *testing.T) {
	t.Parallel()

	state := &game.StateSnapshot{
		Screen:           "REWARD",
		AvailableActions: []string{"claim_reward", "proceed"},
		Reward: map[string]any{
			"phase": "claim",
			"rewards": []any{
				map[string]any{
					"index":      0,
					"rewardType": "GoldReward",
					"claimable":  true,
				},
			},
		},
	}

	err := ValidateActionRequest(state, game.ActionRequest{Action: "proceed"})
	if err == nil || !strings.Contains(err.Error(), "proceed is not available while reward or selection choices remain") {
		t.Fatalf("expected unresolved reward proceed error, got %v", err)
	}
}

func TestActionInvariantProceedIsBlockedWhileSelectionChoiceRemains(t *testing.T) {
	t.Parallel()

	state := &game.StateSnapshot{
		Screen:           "CARD_SELECTION",
		AvailableActions: []string{"select_deck_card", "proceed"},
		Selection: map[string]any{
			"kind": "upgrade",
			"cards": []any{
				map[string]any{
					"index":  1,
					"cardId": "STRIKE_RED",
				},
			},
		},
	}

	err := ValidateActionRequest(state, game.ActionRequest{Action: "proceed"})
	if err == nil || !strings.Contains(err.Error(), "proceed is not available while reward or selection choices remain") {
		t.Fatalf("expected unresolved selection proceed error, got %v", err)
	}
}

func TestActionInvariantProceedAllowsSettledRewardScreen(t *testing.T) {
	t.Parallel()

	state := &game.StateSnapshot{
		Screen:           "REWARD",
		AvailableActions: []string{"proceed"},
		Reward: map[string]any{
			"phase":             "proceed",
			"pendingCardChoice": false,
			"rewards":           []any{},
			"cardOptions":       []any{},
		},
	}

	if err := ValidateActionRequest(state, game.ActionRequest{Action: "proceed"}); err != nil {
		t.Fatalf("expected settled reward proceed to validate, got %v", err)
	}
}

func TestActionInvariantConfirmSelectionRequiresConfirmationContract(t *testing.T) {
	t.Parallel()

	state := &game.StateSnapshot{
		Screen:           "CARD_SELECTION",
		AvailableActions: []string{"confirm_selection"},
		Selection: map[string]any{
			"kind":                 "combat_hand_select",
			"requiresConfirmation": true,
			"canConfirm":           true,
			"selectedCount":        2,
			"cards":                []any{},
		},
	}

	if err := ValidateActionRequest(state, game.ActionRequest{Action: "confirm_selection"}); err != nil {
		t.Fatalf("expected confirm_selection to validate, got %v", err)
	}
}

func TestActionInvariantProceedIsBlockedWhileSelectionConfirmationRemains(t *testing.T) {
	t.Parallel()

	state := &game.StateSnapshot{
		Screen:           "CARD_SELECTION",
		AvailableActions: []string{"confirm_selection", "proceed"},
		Selection: map[string]any{
			"kind":                 "combat_hand_select",
			"requiresConfirmation": true,
			"canConfirm":           true,
			"selectedCount":        2,
			"cards":                []any{},
		},
	}

	err := ValidateActionRequest(state, game.ActionRequest{Action: "proceed"})
	if err == nil || !strings.Contains(err.Error(), "proceed is not available while reward or selection choices remain") {
		t.Fatalf("expected unresolved selection confirmation proceed error, got %v", err)
	}
}
