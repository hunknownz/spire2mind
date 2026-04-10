package agentruntime

import (
	"strings"

	"spire2mind/internal/game"
)

func ChooseRuleBasedAction(state *game.StateSnapshot, maxAttempts int, attempt int, failures *actionFailureMemory) (game.ActionRequest, string, bool) {
	if state == nil {
		return game.ActionRequest{}, "", false
	}

	if hasAction(state, "confirm_modal") {
		return game.ActionRequest{Action: "confirm_modal"}, "modal priority", true
	}
	if hasAction(state, "dismiss_modal") {
		return game.ActionRequest{Action: "dismiss_modal"}, "modal priority", true
	}

	if strings.EqualFold(state.Screen, "GAME_OVER") {
		if attemptsExhausted(maxAttempts, attempt) {
			return game.ActionRequest{}, "", false
		}
		if hasAction(state, "continue_after_game_over") {
			return game.ActionRequest{Action: "continue_after_game_over"}, "advance past the game over summary", true
		}
		if hasAction(state, "return_to_main_menu") {
			return game.ActionRequest{Action: "return_to_main_menu"}, "return to main menu for the next attempt", true
		}
	}

	switch state.Screen {
	case "MAIN_MENU":
		if hasAction(state, "continue_run") {
			return game.ActionRequest{Action: "continue_run"}, "resume existing run", true
		}
		if hasAction(state, "open_character_select") {
			return game.ActionRequest{Action: "open_character_select"}, "start a new run", true
		}
	case "CHARACTER_SELECT":
		if hasAction(state, "embark") && characterAlreadySelected(state) {
			return game.ActionRequest{Action: "embark"}, "embark with the selected character", true
		}
		if hasAction(state, "select_character") {
			if index := firstUnlockedCharacterIndex(state); index != nil {
				return game.ActionRequest{Action: "select_character", OptionIndex: index}, "select first unlocked character", true
			}
		}
		if hasAction(state, "embark") {
			return game.ActionRequest{Action: "embark"}, "embark with selected character", true
		}
	case "REWARD":
		if request, reason, ok := chooseRewardFlowAction(state, failures); ok {
			return request, reason, true
		}
	}

	if request, reason, ok := chooseSingleActionShortcut(state); ok {
		return request, reason, true
	}

	return game.ActionRequest{}, "", false
}

func ChooseDeterministicAction(state *game.StateSnapshot, maxAttempts int, attempt int, failures *actionFailureMemory) (game.ActionRequest, string, bool) {
	if state == nil {
		return game.ActionRequest{}, "", false
	}

	if hasAction(state, "confirm_modal") {
		return game.ActionRequest{Action: "confirm_modal"}, "modal priority", true
	}
	if hasAction(state, "dismiss_modal") {
		return game.ActionRequest{Action: "dismiss_modal"}, "modal priority", true
	}

	switch state.Screen {
	case "GAME_OVER":
		if attemptsExhausted(maxAttempts, attempt) {
			return game.ActionRequest{}, "", false
		}
		if hasAction(state, "continue_after_game_over") {
			return game.ActionRequest{Action: "continue_after_game_over"}, "advance past the game over summary", true
		}
		if hasAction(state, "return_to_main_menu") {
			return game.ActionRequest{Action: "return_to_main_menu"}, "return to main menu for the next attempt", true
		}
	case "MAIN_MENU":
		if hasAction(state, "continue_run") {
			return game.ActionRequest{Action: "continue_run"}, "resume existing run", true
		}
		if hasAction(state, "open_character_select") {
			return game.ActionRequest{Action: "open_character_select"}, "start a new run", true
		}
	case "CHARACTER_SELECT":
		if hasAction(state, "embark") && characterAlreadySelected(state) {
			return game.ActionRequest{Action: "embark"}, "embark with the selected character", true
		}
		if hasAction(state, "select_character") {
			if index := firstUnlockedCharacterIndex(state); index != nil {
				return game.ActionRequest{Action: "select_character", OptionIndex: index}, "select first unlocked character", true
			}
		}
		if hasAction(state, "embark") {
			return game.ActionRequest{Action: "embark"}, "embark with selected character", true
		}
	case "MAP":
		if hasAction(state, "choose_map_node") {
			if index := preferredMapNodeIndex(state); index != nil {
				return game.ActionRequest{Action: "choose_map_node", OptionIndex: index}, "advance along the current map route", true
			}
		}
	case "SHOP":
		if hasAction(state, "open_shop_inventory") {
			if shouldOpenShopInventory(state) {
				return game.ActionRequest{Action: "open_shop_inventory"}, "open merchant inventory because a meaningful purchase is available", true
			}
			if hasAction(state, "proceed") {
				return game.ActionRequest{Action: "proceed"}, "skip merchant because nothing is affordable", true
			}
		}
		if request, reason, ok := chooseShopAction(state); ok {
			return request, reason, true
		}
		if hasAction(state, "close_shop_inventory") && !shopHasAffordableOption(state.Shop) {
			return game.ActionRequest{Action: "close_shop_inventory"}, "close merchant inventory because no purchase is affordable", true
		}
		if hasAction(state, "close_shop_inventory") {
			return game.ActionRequest{Action: "close_shop_inventory"}, "close merchant inventory because no meaningful purchase remains", true
		}
		if hasAction(state, "proceed") {
			return game.ActionRequest{Action: "proceed"}, "only proceed is available", true
		}
	case "REST":
		if hasAction(state, "choose_rest_option") {
			if index := preferredRestOption(state); index != nil {
				return game.ActionRequest{Action: "choose_rest_option", OptionIndex: index}, "take the best available rest option", true
			}
		}
		if hasAction(state, "proceed") {
			return game.ActionRequest{Action: "proceed"}, "rest site is complete", true
		}
	case "CHEST":
		if hasAction(state, "open_chest") {
			return game.ActionRequest{Action: "open_chest"}, "open the treasure chest", true
		}
		if hasAction(state, "choose_treasure_relic") {
			if index := firstIndexedOptionTyped(state, "Chest", "relicOptions"); index != nil {
				return game.ActionRequest{Action: "choose_treasure_relic", OptionIndex: index}, "take the first available relic", true
			}
		}
		if hasAction(state, "proceed") {
			return game.ActionRequest{Action: "proceed"}, "treasure room is complete", true
		}
	case "EVENT":
		if hasAction(state, "choose_event_option") {
			if state.Event != nil && state.Event.IsFinished {
				optionIndex := 0
				return game.ActionRequest{Action: "choose_event_option", OptionIndex: &optionIndex}, "advance past the finished event", true
			}
			if index := firstEnabledEventOption(state.Event); index != nil {
				return game.ActionRequest{Action: "choose_event_option", OptionIndex: index}, "take the first unlocked event option", true
			}
			optionIndex := 0
			return game.ActionRequest{Action: "choose_event_option", OptionIndex: &optionIndex}, "advance the event via the default option", true
		}
	case "REWARD":
		if request, reason, ok := chooseRewardFlowAction(state, failures); ok {
			return request, reason, true
		}
		if hasAction(state, "choose_reward_card") {
			if index := preferredRewardCardIndex(state); index != nil {
				return game.ActionRequest{Action: "choose_reward_card", OptionIndex: index}, "take the best immediate reward card", true
			}
		}
		if hasAction(state, "skip_reward_cards") && !hasAction(state, "choose_reward_card") {
			return game.ActionRequest{Action: "skip_reward_cards"}, "skip reward cards when no explicit card choice is exposed", true
		}
		if hasAction(state, "proceed") {
			return game.ActionRequest{Action: "proceed"}, "reward screen only allows proceed", true
		}
	case "CARD_SELECTION":
		if request, reason, ok := chooseDeckSelectionAction(state); ok {
			return request, reason, true
		}
	case "COMBAT":
		if hasAction(state, "play_card") {
			if request, ok := deterministicCombatAction(state, failures); ok {
				return request, "play the next deterministic combat action", true
			}
		}
		if hasAction(state, "end_turn") {
			return game.ActionRequest{Action: "end_turn"}, "no other combat action is legal", true
		}
	}

	return game.ActionRequest{}, "", false
}

func chooseRewardFlowAction(state *game.StateSnapshot, failures *actionFailureMemory) (game.ActionRequest, string, bool) {
	if state == nil || !strings.EqualFold(state.Screen, "REWARD") {
		return game.ActionRequest{}, "", false
	}
	if !hasAction(state, "claim_reward") {
		return game.ActionRequest{}, "", false
	}
	index := firstClaimableReward(state.Reward)
	if index == nil {
		return game.ActionRequest{}, "", false
	}

	request := game.ActionRequest{Action: "claim_reward", OptionIndex: index}
	if failures != nil && !failures.Allows(digestState(state), request) && hasAction(state, "proceed") {
		return game.ActionRequest{Action: "proceed"}, "reward claim already stalled on this exact screen; proceed to break the seam", true
	}

	return request, "claim the first available reward", true
}

func attemptsExhausted(maxAttempts int, attempt int) bool {
	if maxAttempts <= 0 {
		return false
	}
	return attempt >= maxAttempts
}

func chooseSingleActionShortcut(state *game.StateSnapshot) (game.ActionRequest, string, bool) {
	if state == nil || len(state.AvailableActions) != 1 {
		return game.ActionRequest{}, "", false
	}

	action := state.AvailableActions[0]
	if action == "choose_event_option" {
		optionIndex := 0
		if state.Event != nil && state.Event.IsFinished {
			return game.ActionRequest{Action: action, OptionIndex: &optionIndex}, "single legal finished-event action", true
		}
		return game.ActionRequest{Action: action, OptionIndex: &optionIndex}, "single legal event action", true
	}

	switch action {
	case "confirm_modal", "dismiss_modal", "continue_run", "abandon_run", "open_character_select",
		"embark", "open_chest", "proceed", "end_turn", "open_shop_inventory",
		"close_shop_inventory", "remove_card_at_shop", "continue_after_game_over",
		"confirm_selection",
		"return_to_main_menu":
		return game.ActionRequest{Action: action}, "single legal action", true
	default:
		return game.ActionRequest{}, "", false
	}
}
