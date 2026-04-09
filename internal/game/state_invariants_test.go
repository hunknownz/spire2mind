package game

import "testing"

func TestStateInvariantRewardSettlingIsNotActionable(t *testing.T) {
	t.Parallel()

	state := &StateSnapshot{
		Screen:           "REWARD",
		AvailableActions: []string{"claim_reward"},
		Reward: map[string]any{
			"phase": "settling",
		},
	}

	if IsActionableState(state) {
		t.Fatal("expected reward settling state to be non-actionable")
	}
}

func TestStateInvariantSelectionWithDeckChoiceIsActionable(t *testing.T) {
	t.Parallel()

	state := &StateSnapshot{
		Screen:           "CARD_SELECTION",
		AvailableActions: []string{"select_deck_card"},
		Selection: map[string]any{
			"kind":                 "upgrade",
			"requiresConfirmation": false,
			"cards": []any{
				map[string]any{"index": 1, "cardId": "STRIKE_RED"},
			},
		},
	}

	if !IsActionableState(state) {
		t.Fatal("expected deck selection state to be actionable")
	}
}

func TestStateInvariantGameOverTransitionIsNotActionable(t *testing.T) {
	t.Parallel()

	state := &StateSnapshot{
		Screen:           "GAME_OVER",
		AvailableActions: []string{"return_to_main_menu"},
		GameOver: map[string]any{
			"stage": "transition",
		},
	}

	if IsActionableState(state) {
		t.Fatal("expected game over transition state to be non-actionable")
	}
}

func TestStateInvariantFinishedEventContinuationIsActionable(t *testing.T) {
	t.Parallel()

	state := &StateSnapshot{
		Screen:           "EVENT",
		AvailableActions: []string{"choose_event_option"},
		Event: map[string]any{
			"isFinished": true,
			"options":    []any{},
		},
	}

	if !IsActionableState(state) {
		t.Fatal("expected finished event continuation state to be actionable")
	}
}

func TestStateInvariantRewardClaimPhaseIsActionable(t *testing.T) {
	t.Parallel()

	state := &StateSnapshot{
		Screen:           "REWARD",
		AvailableActions: []string{"claim_reward"},
		Reward: map[string]any{
			"phase": "claim",
		},
	}

	if !IsActionableState(state) {
		t.Fatal("expected reward claim phase to be actionable")
	}
}

func TestStateInvariantRewardProceedPhaseIsActionable(t *testing.T) {
	t.Parallel()

	state := &StateSnapshot{
		Screen:           "REWARD",
		AvailableActions: []string{"proceed"},
		Reward: map[string]any{
			"phase": "proceed",
		},
	}

	if !IsActionableState(state) {
		t.Fatal("expected reward proceed phase to be actionable")
	}
}

func TestStateInvariantCombatActionWindowClosedIsNotActionable(t *testing.T) {
	t.Parallel()

	state := &StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"end_turn"},
		Combat: map[string]any{
			"actionWindowOpen": false,
			"enemies": []any{
				map[string]any{"index": 0, "enemyId": "SLIME_RED", "isHittable": true},
			},
		},
	}

	if IsActionableState(state) {
		t.Fatal("expected combat with a closed action window to be non-actionable")
	}
}

func TestStateInvariantCombatActionWindowOpenIsActionable(t *testing.T) {
	t.Parallel()

	state := &StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"end_turn"},
		Combat: map[string]any{
			"actionWindowOpen": true,
			"enemies": []any{
				map[string]any{"index": 0, "enemyId": "SLIME_RED", "isHittable": true},
			},
		},
	}

	if !IsActionableState(state) {
		t.Fatal("expected combat with an open action window to be actionable")
	}
}

func TestStateInvariantGameOverContinueStageIsActionable(t *testing.T) {
	t.Parallel()

	state := &StateSnapshot{
		Screen:           "GAME_OVER",
		AvailableActions: []string{"continue_after_game_over"},
		GameOver: map[string]any{
			"stage": "continue",
		},
	}

	if !IsActionableState(state) {
		t.Fatal("expected settled game over continuation stage to be actionable")
	}
}

func TestStateInvariantSelectionConfirmPhaseIsActionable(t *testing.T) {
	t.Parallel()

	state := &StateSnapshot{
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

	if !IsActionableState(state) {
		t.Fatal("expected selection confirm phase to be actionable")
	}
}

func TestStateInvariantSimpleSelectionDoesNotRequireConfirmation(t *testing.T) {
	t.Parallel()

	state := &StateSnapshot{
		Screen:           "CARD_SELECTION",
		AvailableActions: []string{"select_deck_card"},
		Selection: map[string]any{
			"kind":                 "NSimpleCardSelectScreen",
			"requiresConfirmation": true,
			"canConfirm":           false,
			"cards": []any{
				map[string]any{"index": 0, "cardId": "BASH"},
			},
		},
	}

	if IsActionableState(state) {
		t.Fatal("expected simple selection with confirmation contract drift to be non-actionable")
	}
}

func TestStateInvariantSelectionConfirmActionRequiresCanConfirm(t *testing.T) {
	t.Parallel()

	state := &StateSnapshot{
		Screen:           "CARD_SELECTION",
		AvailableActions: []string{"confirm_selection"},
		Selection: map[string]any{
			"kind":                 "combat_hand_select",
			"requiresConfirmation": true,
			"canConfirm":           false,
			"cards":                []any{},
		},
	}

	if IsActionableState(state) {
		t.Fatal("expected confirm_selection without canConfirm to be non-actionable")
	}
}

func TestStateInvariantRewardCardOverlayIsActionableWithoutDeckSelectionAction(t *testing.T) {
	t.Parallel()

	state := &StateSnapshot{
		Screen:           "CARD_SELECTION",
		AvailableActions: []string{"choose_reward_card", "skip_reward_cards"},
		Selection: map[string]any{
			"kind": "reward_cards",
			"cards": []any{
				map[string]any{"index": 0, "cardId": "IRON_WAVE"},
			},
		},
		Reward: map[string]any{
			"phase":             "card_choice",
			"pendingCardChoice": true,
			"cardOptions": []any{
				map[string]any{"index": 0, "cardId": "IRON_WAVE"},
			},
		},
	}

	if !IsActionableState(state) {
		t.Fatal("expected reward card overlay to be actionable")
	}
}

func TestStateInvariantRewardCardOverlayRejectsDeckSelectionAction(t *testing.T) {
	t.Parallel()

	state := &StateSnapshot{
		Screen:           "CARD_SELECTION",
		AvailableActions: []string{"choose_reward_card", "skip_reward_cards", "select_deck_card"},
		Selection: map[string]any{
			"kind": "reward_cards",
			"cards": []any{
				map[string]any{"index": 0, "cardId": "IRON_WAVE"},
			},
		},
		Reward: map[string]any{
			"phase":             "card_choice",
			"pendingCardChoice": true,
			"cardOptions": []any{
				map[string]any{"index": 0, "cardId": "IRON_WAVE"},
			},
		},
	}

	if IsActionableState(state) {
		t.Fatal("expected mixed reward overlay and deck selection actions to be non-actionable")
	}
}

func TestStateInvariantRewardClaimPhaseRejectsProceedWhileClaimablesRemain(t *testing.T) {
	t.Parallel()

	state := &StateSnapshot{
		Screen:           "REWARD",
		AvailableActions: []string{"claim_reward", "proceed"},
		Reward: map[string]any{
			"phase": "claim",
			"rewards": []any{
				map[string]any{
					"index":     0,
					"claimable": true,
				},
			},
		},
	}

	if IsActionableState(state) {
		t.Fatal("expected reward state with unresolved claimables and proceed to be non-actionable")
	}
}
