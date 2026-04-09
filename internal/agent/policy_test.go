package agentruntime

import (
	"testing"

	"spire2mind/internal/game"
)

func TestChooseDeterministicActionSkipsRecentlyFailedCombatCard(t *testing.T) {
	state := &game.StateSnapshot{
		RunID:            "RUN-DET",
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"player": map[string]any{
				"energy": 2,
			},
			"hand": []any{
				map[string]any{
					"index":          0,
					"cardId":         "IRON_WAVE",
					"name":           "Iron Wave",
					"energyCost":     1,
					"playable":       true,
					"requiresTarget": true,
				},
				map[string]any{
					"index":          1,
					"cardId":         "DEFEND_RED",
					"name":           "Defend",
					"energyCost":     1,
					"playable":       true,
					"requiresTarget": false,
				},
			},
			"enemies": []any{
				map[string]any{
					"index":      0,
					"enemyId":    "SLIME",
					"name":       "Slime",
					"currentHp":  20,
					"isAlive":    true,
					"isHittable": true,
				},
			},
		},
	}

	failures := newActionFailureMemory()
	cardIndex := 0
	targetIndex := 0
	failures.Record(digestState(state), game.ActionRequest{
		Action:      "play_card",
		CardIndex:   &cardIndex,
		TargetIndex: &targetIndex,
	})

	request, _, ok := ChooseDeterministicAction(state, 1, 1, failures)
	if !ok {
		t.Fatal("expected deterministic action")
	}
	if request.CardIndex == nil || *request.CardIndex != 1 {
		t.Fatalf("expected deterministic action to skip failed card 0 and choose 1, got %+v", request)
	}
}

func TestChooseDeterministicActionInfersTargetForImplicitTargetCard(t *testing.T) {
	state := &game.StateSnapshot{
		RunID:            "RUN-DET-IMPLICIT-TARGET",
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"player": map[string]any{
				"energy": 1,
			},
			"hand": []any{
				map[string]any{
					"index":          0,
					"cardId":         "STRIKE_IRONCLAD",
					"name":           "Strike",
					"energyCost":     1,
					"playable":       true,
					"requiresTarget": false,
					"targetType":     "AnyEnemy",
				},
			},
			"enemies": []any{
				map[string]any{
					"index":      0,
					"enemyId":    "SLIME",
					"name":       "Slime",
					"currentHp":  20,
					"isAlive":    true,
					"isHittable": true,
				},
			},
		},
	}

	request, _, ok := ChooseDeterministicAction(state, 1, 1, newActionFailureMemory())
	if !ok {
		t.Fatal("expected deterministic action")
	}
	if request.Action != "play_card" {
		t.Fatalf("expected play_card, got %+v", request)
	}
	if request.TargetIndex == nil || *request.TargetIndex != 0 {
		t.Fatalf("expected inferred target index 0, got %+v", request)
	}
}

func TestChooseDeterministicActionContinuesAtGameOverWhenAttemptsUnlimited(t *testing.T) {
	state := &game.StateSnapshot{
		RunID:            "RUN-GO",
		Screen:           "GAME_OVER",
		AvailableActions: []string{"continue_after_game_over", "return_to_main_menu"},
	}

	request, _, ok := ChooseDeterministicAction(state, 0, 7, newActionFailureMemory())
	if !ok {
		t.Fatal("expected deterministic action for unlimited attempts")
	}
	if request.Action != "continue_after_game_over" {
		t.Fatalf("expected continue_after_game_over, got %+v", request)
	}
}

func TestChooseDeterministicActionAdvancesFinishedEvent(t *testing.T) {
	t.Parallel()

	state := &game.StateSnapshot{
		RunID:            "RUN-EVENT",
		Screen:           "EVENT",
		AvailableActions: []string{"choose_event_option"},
		Event: map[string]any{
			"isFinished": true,
			"options":    []any{},
		},
	}

	request, _, ok := ChooseDeterministicAction(state, 0, 1, newActionFailureMemory())
	if !ok {
		t.Fatal("expected deterministic event action")
	}
	if request.Action != "choose_event_option" {
		t.Fatalf("expected choose_event_option, got %+v", request)
	}
	if request.OptionIndex == nil || *request.OptionIndex != 0 {
		t.Fatalf("expected option_index 0 for finished event, got %+v", request)
	}
}

func TestChooseRuleBasedActionAdvancesFinishedEventSingleAction(t *testing.T) {
	t.Parallel()

	state := &game.StateSnapshot{
		RunID:            "RUN-EVENT",
		Screen:           "EVENT",
		AvailableActions: []string{"choose_event_option"},
		Event: map[string]any{
			"isFinished": true,
			"options":    []any{},
		},
	}

	request, _, ok := ChooseRuleBasedAction(state, 0, 1, newActionFailureMemory())
	if !ok {
		t.Fatal("expected rule-based event shortcut")
	}
	if request.Action != "choose_event_option" {
		t.Fatalf("expected choose_event_option, got %+v", request)
	}
	if request.OptionIndex == nil || *request.OptionIndex != 0 {
		t.Fatalf("expected option_index 0 for finished event, got %+v", request)
	}
}

func TestChooseDeterministicActionDefaultsEventOptionWhenPayloadIncomplete(t *testing.T) {
	t.Parallel()

	state := &game.StateSnapshot{
		RunID:            "RUN-EVENT-INCOMPLETE",
		Screen:           "EVENT",
		AvailableActions: []string{"choose_event_option"},
		Event:            map[string]any{},
	}

	request, _, ok := ChooseDeterministicAction(state, 0, 1, newActionFailureMemory())
	if !ok {
		t.Fatal("expected deterministic event action")
	}
	if request.Action != "choose_event_option" {
		t.Fatalf("expected choose_event_option, got %+v", request)
	}
	if request.OptionIndex == nil || *request.OptionIndex != 0 {
		t.Fatalf("expected option_index 0 for incomplete event payload, got %+v", request)
	}
}

func TestChooseRuleBasedActionDefaultsEventOptionWhenSingleActionPayloadIncomplete(t *testing.T) {
	t.Parallel()

	state := &game.StateSnapshot{
		RunID:            "RUN-EVENT-INCOMPLETE",
		Screen:           "EVENT",
		AvailableActions: []string{"choose_event_option"},
		Event:            map[string]any{},
	}

	request, _, ok := ChooseRuleBasedAction(state, 0, 1, newActionFailureMemory())
	if !ok {
		t.Fatal("expected rule-based event action")
	}
	if request.Action != "choose_event_option" {
		t.Fatalf("expected choose_event_option, got %+v", request)
	}
	if request.OptionIndex == nil || *request.OptionIndex != 0 {
		t.Fatalf("expected option_index 0 for incomplete event payload, got %+v", request)
	}
}

func TestChooseRuleBasedActionProceedsAfterRewardClaimStallsOnSameState(t *testing.T) {
	t.Parallel()

	state := &game.StateSnapshot{
		RunID:            "RUN-REWARD-STALLED",
		Screen:           "REWARD",
		AvailableActions: []string{"claim_reward", "proceed"},
		Reward: map[string]any{
			"phase": "claim",
			"rewards": []any{
				map[string]any{
					"index":       0,
					"rewardType":  "PotionReward",
					"description": "Potion",
					"claimable":   true,
				},
			},
		},
	}

	failures := newActionFailureMemory()
	optionIndex := 0
	failures.Record(digestState(state), game.ActionRequest{
		Action:      "claim_reward",
		OptionIndex: &optionIndex,
	})

	request, _, ok := ChooseRuleBasedAction(state, 0, 1, failures)
	if !ok {
		t.Fatal("expected rule-based reward fallback")
	}
	if request.Action != "proceed" {
		t.Fatalf("expected proceed after stalled reward claim, got %+v", request)
	}
}

func TestChooseDeterministicActionSelectsNextUnselectedDeckCard(t *testing.T) {
	t.Parallel()

	state := &game.StateSnapshot{
		RunID:            "RUN-SELECTION",
		Screen:           "CARD_SELECTION",
		AvailableActions: []string{"select_deck_card"},
		Selection: map[string]any{
			"kind":          "NDeckTransformSelectScreen",
			"minSelect":     2,
			"maxSelect":     2,
			"selectedCount": 1,
			"cards": []any{
				map[string]any{"index": 0, "cardId": "STRIKE_RED", "isSelected": true},
				map[string]any{"index": 1, "cardId": "DEFEND_RED", "isSelected": false},
			},
		},
	}

	request, _, ok := ChooseDeterministicAction(state, 0, 1, newActionFailureMemory())
	if !ok {
		t.Fatal("expected deterministic selection action")
	}
	if request.Action != "select_deck_card" {
		t.Fatalf("expected select_deck_card, got %+v", request)
	}
	if request.OptionIndex == nil || *request.OptionIndex != 1 {
		t.Fatalf("expected next unselected card index 1, got %+v", request)
	}
}

func TestChooseDeterministicActionConfirmsDeckSelectionWhenMinimumReached(t *testing.T) {
	t.Parallel()

	state := &game.StateSnapshot{
		RunID:            "RUN-SELECTION-CONFIRM",
		Screen:           "CARD_SELECTION",
		AvailableActions: []string{"confirm_selection", "select_deck_card"},
		Selection: map[string]any{
			"kind":                 "NDeckTransformSelectScreen",
			"requiresConfirmation": true,
			"canConfirm":           true,
			"minSelect":            2,
			"maxSelect":            2,
			"selectedCount":        2,
			"cards": []any{
				map[string]any{"index": 0, "cardId": "STRIKE_RED", "isSelected": true},
				map[string]any{"index": 1, "cardId": "DEFEND_RED", "isSelected": true},
			},
		},
	}

	request, _, ok := ChooseDeterministicAction(state, 0, 1, newActionFailureMemory())
	if !ok {
		t.Fatal("expected deterministic confirm action")
	}
	if request.Action != "confirm_selection" {
		t.Fatalf("expected confirm_selection, got %+v", request)
	}
}

func TestChooseDeterministicActionPrefersImmediateRewardCardPower(t *testing.T) {
	t.Parallel()

	state := &game.StateSnapshot{
		RunID:            "RUN-REWARD-CARDS",
		Screen:           "REWARD",
		AvailableActions: []string{"choose_reward_card", "skip_reward_cards"},
		Run: map[string]any{
			"floor":     2,
			"currentHp": 70,
			"maxHp":     80,
		},
		Reward: map[string]any{
			"pendingCardChoice": true,
			"cardOptions": []any{
				map[string]any{"index": 0, "cardId": "IRON_WAVE", "name": "Iron Wave"},
				map[string]any{"index": 1, "cardId": "ARMAMENTS", "name": "Armaments"},
				map[string]any{"index": 2, "cardId": "SHRUG_IT_OFF", "name": "Shrug It Off"},
			},
		},
	}

	request, _, ok := ChooseDeterministicAction(state, 0, 1, newActionFailureMemory())
	if !ok {
		t.Fatal("expected deterministic reward action")
	}
	if request.Action != "choose_reward_card" {
		t.Fatalf("expected choose_reward_card, got %+v", request)
	}
	if request.OptionIndex == nil || *request.OptionIndex != 2 {
		t.Fatalf("expected Shrug It Off at index 2, got %+v", request)
	}
}

func TestChooseDeterministicActionPrefersRewardChoiceInsideCardSelection(t *testing.T) {
	t.Parallel()

	state := &game.StateSnapshot{
		RunID:            "RUN-CARD-SELECTION-REWARD",
		Screen:           "CARD_SELECTION",
		AvailableActions: []string{"choose_reward_card", "skip_reward_cards"},
		Run: map[string]any{
			"floor":     2,
			"currentHp": 70,
			"maxHp":     80,
		},
		Selection: map[string]any{
			"kind":       "grid_card_selection",
			"sourceHint": "combat:grid_card_selection:reward",
			"cards": []any{
				map[string]any{"index": 0, "cardId": "IRON_WAVE", "name": "Iron Wave"},
				map[string]any{"index": 1, "cardId": "ARMAMENTS", "name": "Armaments"},
				map[string]any{"index": 2, "cardId": "SHRUG_IT_OFF", "name": "Shrug It Off"},
			},
		},
	}

	request, _, ok := ChooseDeterministicAction(state, 0, 1, newActionFailureMemory())
	if !ok {
		t.Fatal("expected deterministic card-selection reward action")
	}
	if request.Action != "choose_reward_card" {
		t.Fatalf("expected choose_reward_card, got %+v", request)
	}
	if request.OptionIndex == nil || *request.OptionIndex != 2 {
		t.Fatalf("expected Shrug It Off at index 2, got %+v", request)
	}
}

func TestPreferredMapNodeIndexPrefersShopWhenGoldIsIdleEarly(t *testing.T) {
	t.Parallel()

	state := &game.StateSnapshot{
		Screen: "MAP",
		Run: map[string]any{
			"floor":     5,
			"gold":      150,
			"currentHp": 64,
			"maxHp":     80,
		},
		Map: map[string]any{
			"availableNodes": []any{
				map[string]any{"index": 0, "nodeType": "combat"},
				map[string]any{"index": 1, "nodeType": "shop"},
			},
		},
	}

	index := preferredMapNodeIndex(state)
	if index == nil || *index != 1 {
		t.Fatalf("expected early idle gold to prefer shop index 1, got %v", index)
	}
}

func TestChooseDeterministicActionUsesShopRemovalBeforeCheapCard(t *testing.T) {
	t.Parallel()

	state := &game.StateSnapshot{
		RunID:            "RUN-SHOP-REMOVE",
		Screen:           "SHOP",
		AvailableActions: []string{"remove_card_at_shop", "buy_card", "close_shop_inventory"},
		Run: map[string]any{
			"floor":     5,
			"gold":      150,
			"deckCount": 12,
			"currentHp": 70,
			"maxHp":     80,
		},
		Shop: map[string]any{
			"cardRemoval": map[string]any{
				"available":  true,
				"enoughGold": true,
				"price":      75,
			},
			"cards": []any{
				map[string]any{"index": 0, "cardId": "DEFEND_IRONCLAD", "name": "Defend", "price": 45, "enoughGold": true},
			},
		},
	}

	request, _, ok := ChooseDeterministicAction(state, 0, 1, newActionFailureMemory())
	if !ok {
		t.Fatal("expected deterministic shop action")
	}
	if request.Action != "remove_card_at_shop" {
		t.Fatalf("expected remove_card_at_shop, got %+v", request)
	}
}

func TestChooseDeterministicActionPrefersStrongShopCardOverCheapestCard(t *testing.T) {
	t.Parallel()

	state := &game.StateSnapshot{
		RunID:            "RUN-SHOP-CARD",
		Screen:           "SHOP",
		AvailableActions: []string{"buy_card", "close_shop_inventory"},
		Run: map[string]any{
			"floor":     3,
			"gold":      120,
			"deckCount": 11,
			"currentHp": 74,
			"maxHp":     80,
		},
		Shop: map[string]any{
			"cards": []any{
				map[string]any{"index": 0, "cardId": "DEFEND_IRONCLAD", "name": "Defend", "price": 45, "enoughGold": true},
				map[string]any{"index": 1, "cardId": "SHRUG_IT_OFF", "name": "Shrug It Off", "price": 78, "enoughGold": true},
			},
		},
	}

	request, _, ok := ChooseDeterministicAction(state, 0, 1, newActionFailureMemory())
	if !ok {
		t.Fatal("expected deterministic shop action")
	}
	if request.Action != "buy_card" {
		t.Fatalf("expected buy_card, got %+v", request)
	}
	if request.OptionIndex == nil || *request.OptionIndex != 1 {
		t.Fatalf("expected stronger card at index 1, got %+v", request)
	}
}

func TestChooseDeterministicActionSkipsOpeningShopWithoutMeaningfulPurchase(t *testing.T) {
	t.Parallel()

	state := &game.StateSnapshot{
		RunID:            "RUN-SHOP-SKIP",
		Screen:           "SHOP",
		AvailableActions: []string{"open_shop_inventory", "proceed"},
		Run: map[string]any{
			"floor":     4,
			"gold":      90,
			"deckCount": 18,
			"currentHp": 75,
			"maxHp":     80,
		},
		Shop: map[string]any{
			"cards": []any{
				map[string]any{"index": 0, "cardId": "STRIKE_IRONCLAD", "name": "Strike", "price": 50, "enoughGold": true},
			},
		},
	}

	request, _, ok := ChooseDeterministicAction(state, 0, 1, newActionFailureMemory())
	if !ok {
		t.Fatal("expected deterministic shop action")
	}
	if request.Action != "proceed" {
		t.Fatalf("expected proceed when no meaningful purchase exists, got %+v", request)
	}
}
