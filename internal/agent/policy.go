package agentruntime

import (
	"fmt"
	"strconv"
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
			if index := firstIndexedOption(state.Chest, "relicOptions"); index != nil {
				return game.ActionRequest{Action: "choose_treasure_relic", OptionIndex: index}, "take the first available relic", true
			}
		}
		if hasAction(state, "proceed") {
			return game.ActionRequest{Action: "proceed"}, "treasure room is complete", true
		}
	case "EVENT":
		if hasAction(state, "choose_event_option") {
			if fieldBool(state.Event, "isFinished") {
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
		if fieldBool(state.Event, "isFinished") {
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

func firstUnlockedCharacterIndex(state *game.StateSnapshot) *int {
	for _, option := range nestedList(state.CharacterSelect, "characters") {
		if fieldBool(option, "isLocked") {
			continue
		}
		if fieldBool(option, "isRandom") {
			continue
		}

		if index, ok := fieldInt(option, "index"); ok {
			return &index
		}
	}

	return nil
}

func characterAlreadySelected(state *game.StateSnapshot) bool {
	if state == nil {
		return false
	}

	selectedID := fieldString(state.CharacterSelect, "selectedCharacterId")
	if selectedID == "" {
		return false
	}

	for _, option := range nestedList(state.CharacterSelect, "characters") {
		if fieldString(option, "characterId") != selectedID {
			continue
		}

		return fieldBool(option, "isSelected") || !fieldBool(option, "isLocked")
	}

	return false
}

func preferredRestOption(state *game.StateSnapshot) *int {
	rest := state.Rest
	if len(rest) == 0 {
		return nil
	}

	if hpRatio(state) < 0.55 {
		if index := firstMatchingRestOption(rest, "heal", "rest", "recover"); index != nil {
			return index
		}
	}

	if index := firstMatchingRestOption(rest, "smith", "upgrade", "enhance", "enchant"); index != nil {
		return index
	}

	var enabled []int
	for _, option := range nestedList(rest, "options") {
		if !fieldBool(option, "isEnabled") {
			continue
		}
		if index, ok := fieldInt(option, "index"); ok {
			enabled = append(enabled, index)
		}
	}

	if len(enabled) == 1 {
		return &enabled[0]
	}

	return nil
}

func firstEnabledEventOption(event map[string]any) *int {
	var enabled []int
	for _, option := range nestedList(event, "options") {
		if fieldBool(option, "isLocked") {
			continue
		}
		if fieldBool(option, "isProceed") && len(enabled) > 0 {
			continue
		}
		if index, ok := fieldInt(option, "index"); ok {
			enabled = append(enabled, index)
		}
	}

	if len(enabled) > 0 {
		return &enabled[0]
	}

	return nil
}

func firstClaimableReward(reward map[string]any) *int {
	var claimable []int
	for _, option := range nestedList(reward, "rewards") {
		if !fieldBool(option, "claimable") {
			continue
		}
		if index, ok := fieldInt(option, "index"); ok {
			claimable = append(claimable, index)
		}
	}

	if len(claimable) > 0 {
		return &claimable[0]
	}

	return nil
}

func selectionCanConfirmNow(state *game.StateSnapshot) bool {
	if state == nil {
		return false
	}
	if !fieldBool(state.Selection, "requiresConfirmation") || !fieldBool(state.Selection, "canConfirm") {
		return false
	}
	selectedCount, ok := fieldInt(state.Selection, "selectedCount")
	if !ok {
		return false
	}
	minSelect, ok := fieldInt(state.Selection, "minSelect")
	if !ok || minSelect <= 0 {
		minSelect = 1
	}
	return selectedCount >= minSelect
}

func chooseDeckSelectionAction(state *game.StateSnapshot) (game.ActionRequest, string, bool) {
	if state == nil || !strings.EqualFold(state.Screen, "CARD_SELECTION") {
		return game.ActionRequest{}, "", false
	}

	if hasAction(state, "confirm_selection") && selectionCanConfirmNow(state) {
		selectedCount, _ := fieldInt(state.Selection, "selectedCount")
		minSelect, _ := fieldInt(state.Selection, "minSelect")
		if minSelect <= 0 {
			minSelect = 1
		}
		return game.ActionRequest{Action: "confirm_selection"}, fmt.Sprintf("confirm the current selection after choosing %d/%d cards", selectedCount, minSelect), true
	}

	if !hasAction(state, "select_deck_card") {
		if hasAction(state, "choose_reward_card") {
			if index := preferredRewardCardIndex(state); index != nil {
				return game.ActionRequest{Action: "choose_reward_card", OptionIndex: index}, "take the best immediate reward card", true
			}
		}
		return game.ActionRequest{}, "", false
	}

	if isRewardCardSelection(state) {
		if hasAction(state, "choose_reward_card") {
			if index := preferredRewardCardIndex(state); index != nil {
				return game.ActionRequest{Action: "choose_reward_card", OptionIndex: index}, "take the best immediate reward card", true
			}
		}
		if index := preferredSelectionCardIndex(state); index != nil {
			return game.ActionRequest{Action: "select_deck_card", OptionIndex: index}, "select the best reward-style card", true
		}
	}

	if index := firstUnselectedDeckSelectionOption(state.Selection); index != nil {
		return game.ActionRequest{Action: "select_deck_card", OptionIndex: index}, "select the next unselected deck card", true
	}

	if index := firstIndexedOption(state.Selection, "cards"); index != nil {
		return game.ActionRequest{Action: "select_deck_card", OptionIndex: index}, "select the first available deck card", true
	}

	return game.ActionRequest{}, "", false
}

func firstUnselectedDeckSelectionOption(selection map[string]any) *int {
	for _, option := range nestedList(selection, "cards") {
		if fieldBool(option, "isSelected") {
			continue
		}
		if index, ok := fieldInt(option, "index"); ok {
			return &index
		}
	}

	return nil
}

func shopHasAffordableOption(shop map[string]any) bool {
	for _, card := range nestedList(shop, "cards") {
		if fieldBool(card, "enoughGold") {
			return true
		}
	}
	for _, relic := range nestedList(shop, "relics") {
		if fieldBool(relic, "enoughGold") {
			return true
		}
	}
	for _, potion := range nestedList(shop, "potions") {
		if fieldBool(potion, "enoughGold") {
			return true
		}
	}

	cardRemoval := nestedMap(shop, "cardRemoval")
	return fieldBool(cardRemoval, "enoughGold")
}

func shouldOpenShopInventory(state *game.StateSnapshot) bool {
	if state == nil {
		return false
	}
	if shouldBuyCardRemoval(state) {
		return true
	}
	if _, _, ok := bestAffordableShopRelic(state); ok {
		return true
	}
	if _, _, ok := bestAffordableShopCard(state); ok {
		return true
	}
	if _, _, ok := bestAffordableShopPotion(state); ok {
		return true
	}
	return false
}

func chooseShopAction(state *game.StateSnapshot) (game.ActionRequest, string, bool) {
	if state == nil || !strings.EqualFold(state.Screen, "SHOP") {
		return game.ActionRequest{}, "", false
	}

	if hasAction(state, "remove_card_at_shop") && shouldBuyCardRemoval(state) {
		return game.ActionRequest{Action: "remove_card_at_shop"}, "use affordable card removal to improve draw quality", true
	}
	if hasAction(state, "buy_relic") {
		if index, score, ok := bestAffordableShopRelic(state); ok {
			return game.ActionRequest{Action: "buy_relic", OptionIndex: index}, fmt.Sprintf("buy the strongest affordable relic (score %.1f)", score), true
		}
	}
	if hasAction(state, "buy_card") {
		if index, score, ok := bestAffordableShopCard(state); ok {
			return game.ActionRequest{Action: "buy_card", OptionIndex: index}, fmt.Sprintf("buy the strongest affordable card for the current run (score %.1f)", score), true
		}
	}
	if hasAction(state, "buy_potion") {
		if index, score, ok := bestAffordableShopPotion(state); ok {
			return game.ActionRequest{Action: "buy_potion", OptionIndex: index}, fmt.Sprintf("buy the best affordable potion with excess gold (score %.1f)", score), true
		}
	}

	return game.ActionRequest{}, "", false
}

func shouldBuyCardRemoval(state *game.StateSnapshot) bool {
	if state == nil {
		return false
	}
	cardRemoval := nestedMap(state.Shop, "cardRemoval")
	if len(cardRemoval) == 0 || !fieldBool(cardRemoval, "available") || !fieldBool(cardRemoval, "enoughGold") {
		return false
	}

	deckCount := fieldIntValue(state.Run, "deckCount")
	floor := fieldIntValue(state.Run, "floor")
	gold := fieldIntValue(state.Run, "gold")
	if deckCount <= 0 {
		deckCount = 99
	}

	switch {
	case deckCount <= 16:
		return true
	case floor <= 12 && gold >= 120:
		return true
	case gold >= 160:
		return true
	default:
		return false
	}
}

func bestAffordableShopRelic(state *game.StateSnapshot) (*int, float64, bool) {
	return bestAffordableShopOption(state, "relics", scoreShopRelicChoice, 3.0)
}

func bestAffordableShopCard(state *game.StateSnapshot) (*int, float64, bool) {
	return bestAffordableShopOption(state, "cards", scoreShopCardChoice, 4.0)
}

func bestAffordableShopPotion(state *game.StateSnapshot) (*int, float64, bool) {
	return bestAffordableShopOption(state, "potions", scoreShopPotionChoice, 2.0)
}

func bestAffordableShopOption(state *game.StateSnapshot, key string, scorer func(*game.StateSnapshot, map[string]any) float64, minimumScore float64) (*int, float64, bool) {
	if state == nil {
		return nil, 0, false
	}

	bestIndex := -1
	bestScore := minimumScore
	for _, option := range nestedList(state.Shop, key) {
		if !fieldBool(option, "enoughGold") {
			continue
		}
		index, ok := fieldInt(option, "index")
		if !ok {
			continue
		}
		score := scorer(state, option)
		if score < minimumScore {
			continue
		}
		if bestIndex < 0 || score > bestScore || (score == bestScore && index < bestIndex) {
			bestIndex = index
			bestScore = score
		}
	}
	if bestIndex < 0 {
		return nil, 0, false
	}
	return &bestIndex, bestScore, true
}

func scoreShopCardChoice(state *game.StateSnapshot, card map[string]any) float64 {
	score := scoreRewardCardChoice(state, card)
	price := fieldIntValue(card, "price")
	score -= float64(price) / 35.0

	deckCount := fieldIntValue(state.Run, "deckCount")
	if deckCount >= 16 {
		score -= 1.0
	}
	if deckCount >= 20 {
		score -= 1.0
	}

	cardID := strings.ToUpper(strings.TrimSpace(firstNonEmpty(fieldString(card, "cardId"), fieldString(card, "id"))))
	if containsAny(cardID, "STRIKE", "DEFEND") {
		score -= 4.0
	}

	return score
}

func scoreShopRelicChoice(state *game.StateSnapshot, relic map[string]any) float64 {
	score := 7.0
	price := fieldIntValue(relic, "price")
	gold := fieldIntValue(state.Run, "gold")
	floor := fieldIntValue(state.Run, "floor")
	score -= float64(price) / 55.0

	if floor <= 10 {
		score += 1.0
	}
	if gold-price >= 80 {
		score += 0.8
	}

	label := strings.ToLower(strings.TrimSpace(firstNonEmpty(fieldString(relic, "name"), fieldString(relic, "relicId"), fieldString(relic, "id"))))
	switch {
	case strings.Contains(label, "anchor"),
		strings.Contains(label, "lantern"),
		strings.Contains(label, "vajra"),
		strings.Contains(label, "bag"),
		strings.Contains(label, "horn"),
		strings.Contains(label, "fan"),
		strings.Contains(label, "thread"),
		strings.Contains(label, "wheel"):
		score += 2.0
	case strings.Contains(label, "potion"),
		strings.Contains(label, "shop"),
		strings.Contains(label, "membership"):
		score += 0.5
	}

	return score
}

func scoreShopPotionChoice(state *game.StateSnapshot, potion map[string]any) float64 {
	gold := fieldIntValue(state.Run, "gold")
	if gold < 180 {
		return -10
	}

	score := 2.5
	price := fieldIntValue(potion, "price")
	score -= float64(price) / 45.0

	name := strings.ToLower(strings.TrimSpace(firstNonEmpty(fieldString(potion, "name"), fieldString(potion, "potionId"), fieldString(potion, "id"))))
	switch {
	case strings.Contains(name, "block"), strings.Contains(name, "armor"), strings.Contains(name, "regen"):
		score += 1.0
	case strings.Contains(name, "strength"), strings.Contains(name, "fire"), strings.Contains(name, "attack"):
		score += 0.5
	}

	return score
}

func deterministicCombatAction(state *game.StateSnapshot, failures *actionFailureMemory) (game.ActionRequest, bool) {
	hand := nestedList(state.Combat, "hand")
	playable := []map[string]any{}
	for _, card := range hand {
		if fieldBool(card, "playable") {
			playable = append(playable, card)
		}
	}

	if len(playable) == 0 {
		return game.ActionRequest{}, false
	}

	stateDigest := digestState(state)
	for _, card := range choosePlayableCards(playable) {
		cardIndex, ok := fieldInt(card, "index")
		if !ok {
			continue
		}

		request := game.ActionRequest{Action: "play_card", CardIndex: &cardIndex}
		if cardRequiresTarget(state, card) {
			if validTargets := combatTargetIndicesForCard(state, card); len(validTargets) > 0 {
				request.TargetIndex = &validTargets[0]
			} else {
				continue
			}
		}

		if failures != nil && !failures.Allows(stateDigest, request) {
			continue
		}
		return request, true
	}

	return game.ActionRequest{}, false
}

func preferredMapNodeIndex(state *game.StateSnapshot) *int {
	nodes := nestedList(state.Map, "availableNodes")
	if len(nodes) == 0 {
		return nil
	}

	type rankedNode struct {
		index    int
		priority int
	}

	hp := hpRatio(state)
	ranked := make([]rankedNode, 0, len(nodes))
	for _, node := range nodes {
		index, ok := fieldInt(node, "index")
		if !ok {
			continue
		}
		remainingGold := fieldIntValue(state.Run, "gold")
		floor := fieldIntValue(state.Run, "floor")

		ranked = append(ranked, rankedNode{
			index:    index,
			priority: mapNodePriority(fieldString(node, "nodeType"), hp, remainingGold, floor),
		})
	}

	if len(ranked) == 0 {
		return nil
	}

	best := ranked[0]
	for _, candidate := range ranked[1:] {
		if candidate.priority < best.priority || (candidate.priority == best.priority && candidate.index < best.index) {
			best = candidate
		}
	}

	return &best.index
}

func mapNodePriority(nodeType string, hpRatio float64, gold int, floor int) int {
	normalized := strings.ToLower(strings.TrimSpace(nodeType))
	if hpRatio >= 0.65 && gold >= 120 && floor <= 10 {
		switch normalized {
		case "shop":
			return 0
		case "combat":
			return 1
		case "event":
			return 2
		case "rest":
			return 3
		case "elite":
			return 4
		case "chest":
			return 5
		case "boss":
			return 6
		}
	}
	if hpRatio < 0.55 {
		switch normalized {
		case "rest":
			return 0
		case "chest":
			return 1
		case "event":
			return 2
		case "shop":
			return 3
		case "combat":
			return 4
		case "elite":
			return 5
		case "boss":
			return 6
		case "":
			return 10
		default:
			return 9
		}
	}

	switch normalized {
	case "chest":
		return 0
	case "event":
		return 1
	case "shop":
		return 2
	case "combat":
		return 3
	case "rest":
		return 4
	case "elite":
		return 5
	case "boss":
		return 6
	case "":
		return 10
	default:
		return 9
	}
}

func preferredRewardCardIndex(state *game.StateSnapshot) *int {
	if state == nil {
		return nil
	}

	cards := rewardCardOptions(state)
	if len(cards) == 0 {
		return nil
	}

	bestIndex := -1
	bestScore := -1e9
	for _, card := range cards {
		index, ok := fieldInt(card, "index")
		if !ok {
			continue
		}
		score := scoreRewardCardChoice(state, card)
		if bestIndex < 0 || score > bestScore || (score == bestScore && index < bestIndex) {
			bestIndex = index
			bestScore = score
		}
	}
	if bestIndex < 0 {
		return nil
	}
	return &bestIndex
}

func preferredSelectionCardIndex(state *game.StateSnapshot) *int {
	if state == nil {
		return nil
	}

	bestIndex := -1
	bestScore := -1e9
	for _, card := range nestedList(state.Selection, "cards") {
		index, ok := fieldInt(card, "index")
		if !ok {
			continue
		}
		score := scoreRewardCardChoice(state, card)
		if bestIndex < 0 || score > bestScore || (score == bestScore && index < bestIndex) {
			bestIndex = index
			bestScore = score
		}
	}
	if bestIndex < 0 {
		return nil
	}
	return &bestIndex
}

func isRewardCardSelection(state *game.StateSnapshot) bool {
	if state == nil {
		return false
	}
	sourceHint := strings.ToLower(strings.TrimSpace(fieldString(state.Selection, "sourceHint")))
	kind := strings.ToLower(strings.TrimSpace(fieldString(state.Selection, "kind")))
	return strings.Contains(sourceHint, "reward") ||
		strings.Contains(kind, "reward") ||
		(hasAction(state, "choose_reward_card") && len(nestedList(state.Selection, "cards")) > 0)
}

func rewardCardOptions(state *game.StateSnapshot) []map[string]any {
	if state == nil {
		return nil
	}
	if cards := nestedList(state.Reward, "cardOptions"); len(cards) > 0 {
		return cards
	}
	if isRewardCardSelection(state) {
		return nestedList(state.Selection, "cards")
	}
	return nil
}

func scoreRewardCardChoice(state *game.StateSnapshot, card map[string]any) float64 {
	cardID := strings.ToUpper(strings.TrimSpace(firstNonEmpty(fieldString(card, "cardId"), fieldString(card, "id"))))
	name := strings.ToLower(strings.TrimSpace(fieldString(card, "name")))
	score := 0.0

	floor := fieldIntValue(state.Run, "floor")
	hp := hpRatio(state)

	if floor <= 6 {
		score += earlyActCardBonus(cardID, name)
	}
	if hp < 0.55 {
		score += survivalCardBonus(cardID, name)
	}

	switch {
	case containsAny(cardID, "SHRUG_IT_OFF", "IRON_WAVE", "POMMEL_STRIKE", "THUNDERCLAP", "SWORD_BOOMERANG", "CLOTHESLINE", "HEADBUTT", "TRUE_GRIT"):
		score += 5.0
	case containsAny(cardID, "ARMAMENTS", "BATTLE_TRANCE", "RAGE", "SPOT_WEAKNESS"):
		score += 1.5
	}

	switch {
	case containsAny(cardID, "LIMIT_BREAK", "BARRICADE", "JUGGERNAUT", "FIRE_BREATHING", "SENTRY"):
		score -= 5.0
	case containsAny(cardID, "PERFECTED_STRIKE", "SEARING_BLOW"):
		score -= 3.0
	}

	if containsAny(cardID, "ARMAMENTS") && floor <= 3 {
		score -= 2.5
	}
	if containsAny(cardID, "DEFEND", "STRIKE") {
		score -= 1.0
	}
	if strings.Contains(name, "upgrade") || strings.Contains(name, "ritual") {
		score -= 0.5
	}

	return score
}

func earlyActCardBonus(cardID string, name string) float64 {
	switch {
	case containsAny(cardID, "SHRUG_IT_OFF"):
		return 9.0
	case containsAny(cardID, "IRON_WAVE", "POMMEL_STRIKE", "THUNDERCLAP", "SWORD_BOOMERANG"):
		return 8.0
	case containsAny(cardID, "CLOTHESLINE", "HEADBUTT", "TRUE_GRIT"):
		return 6.5
	case containsAny(cardID, "ARMAMENTS"):
		return 3.0
	case containsAny(cardID, "LIMIT_BREAK", "BARRICADE", "JUGGERNAUT", "FIRE_BREATHING", "SEARING_BLOW"):
		return -6.0
	default:
		if strings.Contains(name, "block") || strings.Contains(name, "damage") {
			return 2.0
		}
		return 0
	}
}

func survivalCardBonus(cardID string, name string) float64 {
	switch {
	case containsAny(cardID, "SHRUG_IT_OFF", "IRON_WAVE", "TRUE_GRIT", "CLOTHESLINE"):
		return 4.0
	case containsAny(cardID, "ARMAMENTS", "POMMEL_STRIKE", "HEADBUTT"):
		return 1.5
	default:
		if strings.Contains(name, "block") {
			return 2.0
		}
		return 0
	}
}

func cheapestAffordableIndex(root map[string]any, key string) *int {
	var bestIndex *int
	bestPrice := 0
	for _, option := range nestedList(root, key) {
		if !fieldBool(option, "enoughGold") {
			continue
		}

		index, ok := fieldInt(option, "index")
		if !ok {
			continue
		}

		price, ok := fieldInt(option, "price")
		if bestIndex == nil || (ok && price < bestPrice) {
			bestPrice = price
			bestIndex = &index
		}
	}

	return bestIndex
}

func firstIndexedOption(root map[string]any, key string) *int {
	for _, option := range nestedList(root, key) {
		if index, ok := fieldInt(option, "index"); ok {
			return &index
		}
	}

	return nil
}

func firstMatchingRestOption(rest map[string]any, keywords ...string) *int {
	for _, option := range nestedList(rest, "options") {
		if !fieldBool(option, "isEnabled") {
			continue
		}

		label := strings.ToLower(strings.TrimSpace(fieldString(option, "optionId") + " " + fieldString(option, "title") + " " + fieldString(option, "description")))
		for _, keyword := range keywords {
			if strings.Contains(label, keyword) {
				if index, ok := fieldInt(option, "index"); ok {
					return &index
				}
			}
		}
	}

	return nil
}

func choosePlayableCards(playable []map[string]any) []map[string]any {
	ordered := append([]map[string]any(nil), playable...)
	for i := 0; i < len(ordered); i++ {
		best := i
		bestScore := combatCardScore(ordered[i])
		bestIndex := fieldIntValue(ordered[i], "index")
		for j := i + 1; j < len(ordered); j++ {
			score := combatCardScore(ordered[j])
			index := fieldIntValue(ordered[j], "index")
			if score < bestScore || (score == bestScore && index < bestIndex) {
				best = j
				bestScore = score
				bestIndex = index
			}
		}
		ordered[i], ordered[best] = ordered[best], ordered[i]
	}
	return ordered
}

func combatCardScore(card map[string]any) int {
	score := 0
	if cardRequiresTarget(nil, card) {
		score += 3
	}

	if cost, ok := fieldInt(card, "energyCost"); ok {
		score += cost
	}
	if fieldBool(card, "costsX") {
		score += 4
	}
	if fieldBool(card, "starCostsX") {
		score += 4
	}

	name := strings.ToLower(fieldString(card, "name"))
	if strings.Contains(name, "strike") || strings.Contains(name, "打击") {
		score -= 2
	}
	if strings.Contains(name, "defend") || strings.Contains(name, "防御") {
		score -= 1
	}

	return score
}

func hpRatio(state *game.StateSnapshot) float64 {
	current, okCurrent := fieldInt(state.Run, "currentHp")
	maximum, okMaximum := fieldInt(state.Run, "maxHp")
	if !okCurrent || !okMaximum || maximum <= 0 {
		return 1
	}

	return float64(current) / float64(maximum)
}

func nestedList(root map[string]any, key string) []map[string]any {
	if len(root) == 0 {
		return nil
	}

	raw, ok := root[key]
	if !ok {
		return nil
	}

	items, ok := raw.([]interface{})
	if !ok {
		return nil
	}

	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if typed, ok := item.(map[string]any); ok {
			result = append(result, typed)
		}
	}

	return result
}

func nestedMap(root map[string]any, key string) map[string]any {
	if len(root) == 0 {
		return nil
	}
	if typed, ok := root[key].(map[string]any); ok {
		return typed
	}
	return nil
}

func fieldBool(root map[string]any, key string) bool {
	if len(root) == 0 {
		return false
	}

	switch value := root[key].(type) {
	case bool:
		return value
	case string:
		return strings.EqualFold(value, "true")
	default:
		return false
	}
}

func fieldInt(root map[string]any, key string) (int, bool) {
	if len(root) == 0 {
		return 0, false
	}

	switch value := root[key].(type) {
	case int:
		return value, true
	case int32:
		return int(value), true
	case int64:
		return int(value), true
	case float64:
		return int(value), true
	case string:
		parsed, err := strconv.Atoi(value)
		if err == nil {
			return parsed, true
		}
	}

	return 0, false
}

func fieldString(root map[string]any, key string) string {
	if len(root) == 0 {
		return ""
	}

	switch value := root[key].(type) {
	case string:
		return value
	default:
		return ""
	}
}

func fieldIntSlice(root map[string]any, key string) []int {
	if len(root) == 0 {
		return nil
	}

	items, ok := root[key].([]interface{})
	if !ok {
		return nil
	}

	result := make([]int, 0, len(items))
	for _, item := range items {
		switch value := item.(type) {
		case int:
			result = append(result, value)
		case int32:
			result = append(result, int(value))
		case int64:
			result = append(result, int(value))
		case float64:
			result = append(result, int(value))
		}
	}

	return result
}

func formatActionDebug(request game.ActionRequest) string {
	parts := []string{request.Action}
	if request.CardIndex != nil {
		parts = append(parts, fmt.Sprintf("card=%d", *request.CardIndex))
	}
	if request.OptionIndex != nil {
		parts = append(parts, fmt.Sprintf("option=%d", *request.OptionIndex))
	}
	if request.TargetIndex != nil {
		parts = append(parts, fmt.Sprintf("target=%d", *request.TargetIndex))
	}
	return strings.Join(parts, " ")
}
