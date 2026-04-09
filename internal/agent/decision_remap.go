package agentruntime

import (
	"strings"

	"spire2mind/internal/game"
)

func remapDecisionForLiveState(expectedState *game.StateSnapshot, liveState *game.StateSnapshot, decision *ActionDecision) (*ActionDecision, bool) {
	if liveState == nil || decision == nil {
		return nil, false
	}

	remapped := cloneActionDecision(decision).normalize()
	if expectedState == nil {
		if err := ValidateActionDecision(liveState, remapped); err == nil {
			return remapped, false
		}
		return nil, false
	}
	if err := ValidateActionDecision(liveState, remapped); err == nil && decisionMatchesExpectedSemantics(expectedState, liveState, remapped) {
		return remapped, false
	}

	changed := false
	switch remapped.Action {
	case "play_card":
		changed = remapPlayCardDecision(expectedState, liveState, remapped)
	case "choose_reward_card":
		changed = remapRewardCardDecision(expectedState, liveState, remapped)
	case "select_deck_card":
		changed = remapSelectionCardDecision(expectedState, liveState, remapped)
	case "claim_reward":
		changed = remapRewardClaimDecision(expectedState, liveState, remapped)
	case "choose_map_node":
		changed = remapMapNodeDecision(expectedState, liveState, remapped)
	case "choose_event_option":
		changed = remapEventOptionDecision(expectedState, liveState, remapped)
	case "choose_rest_option":
		changed = remapRestOptionDecision(expectedState, liveState, remapped)
	case "buy_card":
		changed = remapShopOptionDecision(expectedState, liveState, remapped, "cards")
	case "buy_relic":
		changed = remapShopOptionDecision(expectedState, liveState, remapped, "relics")
	case "buy_potion":
		changed = remapShopOptionDecision(expectedState, liveState, remapped, "potions")
	case "choose_treasure_relic":
		changed = remapChestRelicDecision(expectedState, liveState, remapped)
	case "select_character":
		changed = remapCharacterDecision(expectedState, liveState, remapped)
	}

	if !changed {
		return nil, false
	}
	if err := ValidateActionDecision(liveState, remapped); err != nil {
		return nil, false
	}

	return remapped, true
}

func decisionMatchesExpectedSemantics(expectedState *game.StateSnapshot, liveState *game.StateSnapshot, decision *ActionDecision) bool {
	if expectedState == nil || liveState == nil || decision == nil {
		return false
	}

	switch decision.Action {
	case "play_card":
		if decision.CardIndex == nil {
			return false
		}

		expectedCard, ok := combatHandCard(expectedState, *decision.CardIndex)
		if !ok {
			return false
		}
		liveCard, ok := combatHandCard(liveState, *decision.CardIndex)
		if !ok || !matchCardOption(expectedCard, liveCard) {
			return false
		}

		expectedRequiresTarget := cardRequiresTarget(expectedState, expectedCard)
		liveRequiresTarget := cardRequiresTarget(liveState, liveCard)
		if expectedRequiresTarget != liveRequiresTarget {
			return false
		}
		if !expectedRequiresTarget {
			return true
		}
		if decision.TargetIndex == nil {
			return false
		}

		expectedTarget, ok := combatEnemyByIndex(expectedState, *decision.TargetIndex)
		if !ok {
			return false
		}
		liveTarget, ok := combatEnemyByIndex(liveState, *decision.TargetIndex)
		if !ok {
			return false
		}
		return matchEnemyOption(expectedTarget, liveTarget)
	case "choose_reward_card":
		return indexedChoiceMatchesExpected(decision.OptionIndex, nestedList(expectedState.Reward, "cardOptions"), nestedList(liveState.Reward, "cardOptions"), matchCardOption)
	case "select_deck_card":
		return indexedChoiceMatchesExpected(decision.OptionIndex, nestedList(expectedState.Selection, "cards"), nestedList(liveState.Selection, "cards"), matchCardOption)
	case "claim_reward":
		return indexedChoiceMatchesExpected(decision.OptionIndex, nestedList(expectedState.Reward, "rewards"), nestedList(liveState.Reward, "rewards"), matchRewardItem)
	case "choose_map_node":
		return indexedChoiceMatchesExpected(decision.OptionIndex, nestedList(expectedState.Map, "availableNodes"), nestedList(liveState.Map, "availableNodes"), matchMapNode)
	case "choose_event_option":
		return indexedChoiceMatchesExpected(decision.OptionIndex, nestedList(expectedState.Event, "options"), nestedList(liveState.Event, "options"), matchEventOption)
	case "choose_rest_option":
		return indexedChoiceMatchesExpected(decision.OptionIndex, nestedList(expectedState.Rest, "options"), nestedList(liveState.Rest, "options"), matchRestOption)
	case "buy_card":
		return indexedChoiceMatchesExpected(decision.OptionIndex, nestedList(expectedState.Shop, "cards"), nestedList(liveState.Shop, "cards"), matchShopOption)
	case "buy_relic":
		return indexedChoiceMatchesExpected(decision.OptionIndex, nestedList(expectedState.Shop, "relics"), nestedList(liveState.Shop, "relics"), matchShopOption)
	case "buy_potion":
		return indexedChoiceMatchesExpected(decision.OptionIndex, nestedList(expectedState.Shop, "potions"), nestedList(liveState.Shop, "potions"), matchShopOption)
	case "choose_treasure_relic":
		return indexedChoiceMatchesExpected(decision.OptionIndex, nestedList(expectedState.Chest, "relicOptions"), nestedList(liveState.Chest, "relicOptions"), matchLabeledOption)
	case "select_character":
		return indexedChoiceMatchesExpected(decision.OptionIndex, nestedList(expectedState.CharacterSelect, "characters"), nestedList(liveState.CharacterSelect, "characters"), matchCharacterOption)
	default:
		return true
	}
}

func indexedChoiceMatchesExpected(pointer *int, expectedItems []map[string]any, liveItems []map[string]any, matcher func(map[string]any, map[string]any) bool) bool {
	if pointer == nil {
		return false
	}

	expectedItem, ok := indexedItem(expectedItems, "index", *pointer)
	if !ok {
		return false
	}
	liveItem, ok := indexedItem(liveItems, "index", *pointer)
	if !ok {
		return false
	}

	return matcher(expectedItem, liveItem)
}

func cloneActionDecision(decision *ActionDecision) *ActionDecision {
	if decision == nil {
		return nil
	}

	cloned := *decision
	if decision.CardIndex != nil {
		value := *decision.CardIndex
		cloned.CardIndex = &value
	}
	if decision.TargetIndex != nil {
		value := *decision.TargetIndex
		cloned.TargetIndex = &value
	}
	if decision.OptionIndex != nil {
		value := *decision.OptionIndex
		cloned.OptionIndex = &value
	}

	return &cloned
}

func remapPlayCardDecision(expectedState *game.StateSnapshot, liveState *game.StateSnapshot, decision *ActionDecision) bool {
	if decision.CardIndex == nil {
		return false
	}

	expectedCard, ok := combatHandCard(expectedState, *decision.CardIndex)
	if !ok {
		return false
	}

	liveIndex, ok := findMatchingIndexedItemWithExpectedOrder(
		nestedList(expectedState.Combat, "hand"),
		nestedList(liveState.Combat, "hand"),
		"index",
		*decision.CardIndex,
		expectedCard,
		matchCardOption,
	)
	if !ok {
		return false
	}

	changed := replaceIntPointer(&decision.CardIndex, liveIndex)
	liveCard, ok := combatHandCard(liveState, liveIndex)
	if !ok {
		return changed
	}
	if !cardRequiresTarget(liveState, liveCard) {
		if decision.TargetIndex != nil {
			decision.TargetIndex = nil
			changed = true
		}
		return changed
	}
	if decision.TargetIndex == nil {
		validTargets := combatTargetIndicesForCard(liveState, liveCard)
		if len(validTargets) == 1 && replaceIntPointer(&decision.TargetIndex, validTargets[0]) {
			changed = true
		}
		return changed
	}

	validTargets := combatTargetIndicesForCard(liveState, liveCard)
	expectedTarget, ok := combatEnemyByIndex(expectedState, *decision.TargetIndex)
	if !ok {
		if len(validTargets) == 1 && replaceIntPointer(&decision.TargetIndex, validTargets[0]) {
			changed = true
		}
		return changed
	}

	liveTargetIndex, ok := findMatchingIndexedItemWithExpectedOrder(
		nestedList(expectedState.Combat, "enemies"),
		nestedList(liveState.Combat, "enemies"),
		"index",
		*decision.TargetIndex,
		expectedTarget,
		matchEnemyOption,
	)
	if !ok {
		if len(validTargets) == 1 && replaceIntPointer(&decision.TargetIndex, validTargets[0]) {
			changed = true
		}
		return changed
	}

	if replaceIntPointer(&decision.TargetIndex, liveTargetIndex) {
		changed = true
	}
	return changed
}

func remapRewardCardDecision(expectedState *game.StateSnapshot, liveState *game.StateSnapshot, decision *ActionDecision) bool {
	return remapIndexedChoiceBySection(
		&decision.OptionIndex,
		nestedList(expectedState.Reward, "cardOptions"),
		nestedList(liveState.Reward, "cardOptions"),
		matchCardOption,
	)
}

func remapSelectionCardDecision(expectedState *game.StateSnapshot, liveState *game.StateSnapshot, decision *ActionDecision) bool {
	return remapIndexedChoiceBySection(
		&decision.OptionIndex,
		nestedList(expectedState.Selection, "cards"),
		nestedList(liveState.Selection, "cards"),
		matchCardOption,
	)
}

func remapRewardClaimDecision(expectedState *game.StateSnapshot, liveState *game.StateSnapshot, decision *ActionDecision) bool {
	return remapIndexedChoiceBySection(
		&decision.OptionIndex,
		nestedList(expectedState.Reward, "rewards"),
		nestedList(liveState.Reward, "rewards"),
		matchRewardItem,
	)
}

func remapMapNodeDecision(expectedState *game.StateSnapshot, liveState *game.StateSnapshot, decision *ActionDecision) bool {
	return remapIndexedChoiceBySection(
		&decision.OptionIndex,
		nestedList(expectedState.Map, "availableNodes"),
		nestedList(liveState.Map, "availableNodes"),
		matchMapNode,
	)
}

func remapEventOptionDecision(expectedState *game.StateSnapshot, liveState *game.StateSnapshot, decision *ActionDecision) bool {
	return remapIndexedChoiceBySection(
		&decision.OptionIndex,
		nestedList(expectedState.Event, "options"),
		nestedList(liveState.Event, "options"),
		matchEventOption,
	)
}

func remapRestOptionDecision(expectedState *game.StateSnapshot, liveState *game.StateSnapshot, decision *ActionDecision) bool {
	return remapIndexedChoiceBySection(
		&decision.OptionIndex,
		nestedList(expectedState.Rest, "options"),
		nestedList(liveState.Rest, "options"),
		matchRestOption,
	)
}

func remapShopOptionDecision(expectedState *game.StateSnapshot, liveState *game.StateSnapshot, decision *ActionDecision, key string) bool {
	return remapIndexedChoiceBySection(
		&decision.OptionIndex,
		nestedList(expectedState.Shop, key),
		nestedList(liveState.Shop, key),
		matchShopOption,
	)
}

func remapChestRelicDecision(expectedState *game.StateSnapshot, liveState *game.StateSnapshot, decision *ActionDecision) bool {
	return remapIndexedChoiceBySection(
		&decision.OptionIndex,
		nestedList(expectedState.Chest, "relicOptions"),
		nestedList(liveState.Chest, "relicOptions"),
		matchLabeledOption,
	)
}

func remapCharacterDecision(expectedState *game.StateSnapshot, liveState *game.StateSnapshot, decision *ActionDecision) bool {
	return remapIndexedChoiceBySection(
		&decision.OptionIndex,
		nestedList(expectedState.CharacterSelect, "characters"),
		nestedList(liveState.CharacterSelect, "characters"),
		matchCharacterOption,
	)
}

func remapIndexedChoiceBySection(pointer **int, expectedItems []map[string]any, liveItems []map[string]any, matcher func(map[string]any, map[string]any) bool) bool {
	if pointer == nil || *pointer == nil {
		return false
	}

	expectedItem, ok := indexedItem(expectedItems, "index", **pointer)
	if !ok {
		return false
	}

	liveIndex, ok := findMatchingIndexedItemWithExpectedOrder(expectedItems, liveItems, "index", **pointer, expectedItem, matcher)
	if !ok {
		return false
	}

	return replaceIntPointer(pointer, liveIndex)
}

func indexedItem(items []map[string]any, key string, index int) (map[string]any, bool) {
	for _, item := range items {
		if value, ok := fieldInt(item, key); ok && value == index {
			return item, true
		}
	}

	return nil, false
}

func findMatchingIndexedItem(items []map[string]any, key string, expected map[string]any, matcher func(map[string]any, map[string]any) bool) (int, bool) {
	if len(items) == 0 || len(expected) == 0 {
		return 0, false
	}

	matches := make([]int, 0, 1)
	for _, item := range items {
		if !matcher(expected, item) {
			continue
		}
		index, ok := fieldInt(item, key)
		if !ok {
			continue
		}
		matches = append(matches, index)
	}

	if len(matches) != 1 {
		return 0, false
	}

	return matches[0], true
}

func findMatchingIndexedItemWithExpectedOrder(expectedItems []map[string]any, liveItems []map[string]any, key string, expectedIndex int, expected map[string]any, matcher func(map[string]any, map[string]any) bool) (int, bool) {
	if len(liveItems) == 0 || len(expected) == 0 {
		return 0, false
	}

	ordinal, ok := matchingEquivalentOrdinal(expectedItems, key, expectedIndex, expected, matcher)
	if !ok {
		return findMatchingIndexedItem(liveItems, key, expected, matcher)
	}

	liveMatches := matchingIndexedItems(liveItems, key, expected, matcher)
	if len(liveMatches) == 0 {
		return 0, false
	}
	if ordinal >= len(liveMatches) {
		ordinal = len(liveMatches) - 1
	}

	return liveMatches[ordinal], true
}

func matchingEquivalentOrdinal(items []map[string]any, key string, expectedIndex int, expected map[string]any, matcher func(map[string]any, map[string]any) bool) (int, bool) {
	if len(items) == 0 || len(expected) == 0 {
		return 0, false
	}

	ordinal := 0
	for _, item := range items {
		if !matcher(expected, item) {
			continue
		}
		index, ok := fieldInt(item, key)
		if !ok {
			continue
		}
		if index == expectedIndex {
			return ordinal, true
		}
		ordinal++
	}

	return 0, false
}

func matchingIndexedItems(items []map[string]any, key string, expected map[string]any, matcher func(map[string]any, map[string]any) bool) []int {
	if len(items) == 0 || len(expected) == 0 {
		return nil
	}

	matches := make([]int, 0, 1)
	for _, item := range items {
		if !matcher(expected, item) {
			continue
		}
		index, ok := fieldInt(item, key)
		if !ok {
			continue
		}
		matches = append(matches, index)
	}

	return matches
}

func replaceIntPointer(pointer **int, value int) bool {
	if pointer == nil {
		return false
	}
	if *pointer != nil && **pointer == value {
		return false
	}
	newValue := value
	*pointer = &newValue
	return true
}

func combatEnemyByIndex(state *game.StateSnapshot, targetIndex int) (map[string]any, bool) {
	for _, enemy := range nestedList(state.Combat, "enemies") {
		if index, ok := fieldInt(enemy, "index"); ok && index == targetIndex {
			return enemy, true
		}
	}

	return nil, false
}

func matchCardOption(expected map[string]any, live map[string]any) bool {
	expectedID := stableIdentity(expected, "cardId", "id")
	liveID := stableIdentity(live, "cardId", "id")
	if expectedID != "" && liveID != "" {
		return expectedID == liveID
	}

	if !sameNormalizedText(fieldString(expected, "name"), fieldString(live, "name")) {
		return false
	}

	expectedCost, expectedCostOK := fieldInt(expected, "energyCost")
	liveCost, liveCostOK := fieldInt(live, "energyCost")
	if expectedCostOK && liveCostOK && expectedCost != liveCost {
		return false
	}

	return true
}

func matchEnemyOption(expected map[string]any, live map[string]any) bool {
	expectedID := stableIdentity(expected, "enemyId", "id")
	liveID := stableIdentity(live, "enemyId", "id")
	if expectedID != "" && liveID != "" {
		return expectedID == liveID
	}

	return sameNormalizedText(fieldString(expected, "name"), fieldString(live, "name"))
}

func matchRewardItem(expected map[string]any, live map[string]any) bool {
	if !sameNormalizedText(fieldString(expected, "rewardType"), fieldString(live, "rewardType")) {
		return false
	}

	expectedDescription := normalizeMatchText(fieldString(expected, "description"))
	liveDescription := normalizeMatchText(fieldString(live, "description"))
	if expectedDescription != "" && liveDescription != "" {
		return expectedDescription == liveDescription
	}

	return fieldBool(expected, "claimable") == fieldBool(live, "claimable")
}

func matchMapNode(expected map[string]any, live map[string]any) bool {
	expectedRow, expectedRowOK := fieldInt(expected, "row")
	liveRow, liveRowOK := fieldInt(live, "row")
	expectedCol, expectedColOK := fieldInt(expected, "col")
	liveCol, liveColOK := fieldInt(live, "col")
	if expectedRowOK && liveRowOK && expectedColOK && liveColOK {
		return expectedRow == liveRow && expectedCol == liveCol
	}

	return sameNormalizedText(fieldString(expected, "nodeType"), fieldString(live, "nodeType"))
}

func matchEventOption(expected map[string]any, live map[string]any) bool {
	if fieldBool(expected, "isProceed") != fieldBool(live, "isProceed") {
		return false
	}

	return sameNormalizedText(firstNonEmpty(fieldString(expected, "label"), fieldString(expected, "title")), firstNonEmpty(fieldString(live, "label"), fieldString(live, "title")))
}

func matchRestOption(expected map[string]any, live map[string]any) bool {
	expectedType := normalizeMatchText(firstNonEmpty(fieldString(expected, "optionType"), fieldString(expected, "title")))
	liveType := normalizeMatchText(firstNonEmpty(fieldString(live, "optionType"), fieldString(live, "title")))
	if expectedType != "" && liveType != "" {
		return expectedType == liveType
	}

	return sameNormalizedText(fieldString(expected, "title"), fieldString(live, "title"))
}

func matchShopOption(expected map[string]any, live map[string]any) bool {
	expectedID := stableIdentity(expected, "id", "cardId", "optionId")
	liveID := stableIdentity(live, "id", "cardId", "optionId")
	if expectedID != "" && liveID != "" {
		return expectedID == liveID
	}

	if !sameNormalizedText(firstNonEmpty(fieldString(expected, "label"), fieldString(expected, "name"), fieldString(expected, "title")), firstNonEmpty(fieldString(live, "label"), fieldString(live, "name"), fieldString(live, "title"))) {
		return false
	}

	expectedPrice, expectedPriceOK := fieldInt(expected, "price")
	livePrice, livePriceOK := fieldInt(live, "price")
	if expectedPriceOK && livePriceOK && expectedPrice != livePrice {
		return false
	}

	return true
}

func matchLabeledOption(expected map[string]any, live map[string]any) bool {
	expectedID := stableIdentity(expected, "id", "cardId", "optionId")
	liveID := stableIdentity(live, "id", "cardId", "optionId")
	if expectedID != "" && liveID != "" {
		return expectedID == liveID
	}

	return sameNormalizedText(firstNonEmpty(fieldString(expected, "label"), fieldString(expected, "name"), fieldString(expected, "title")), firstNonEmpty(fieldString(live, "label"), fieldString(live, "name"), fieldString(live, "title")))
}

func matchCharacterOption(expected map[string]any, live map[string]any) bool {
	expectedID := stableIdentity(expected, "characterId", "id")
	liveID := stableIdentity(live, "characterId", "id")
	if expectedID != "" && liveID != "" {
		return expectedID == liveID
	}

	return fieldBool(expected, "isRandom") == fieldBool(live, "isRandom")
}

func stableIdentity(root map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := normalizeMatchText(fieldString(root, key)); value != "" {
			return value
		}
	}

	return ""
}

func normalizeMatchText(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, "`", "")
	return value
}

func sameNormalizedText(left string, right string) bool {
	left = normalizeMatchText(left)
	right = normalizeMatchText(right)
	return left != "" && right != "" && left == right
}
