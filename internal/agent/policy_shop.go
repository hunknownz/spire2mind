package agentruntime

import (
	"fmt"
	"strings"

	"spire2mind/internal/game"
)

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

func shopHasAffordableOption(shop *game.ShopState) bool {
	if shop == nil {
		return false
	}
	for _, card := range shop.Cards {
		if card.EnoughGold {
			return true
		}
	}
	for _, relic := range shop.Relics {
		if relic.EnoughGold {
			return true
		}
	}
	for _, potion := range shop.Potions {
		if potion.EnoughGold {
			return true
		}
	}
	if shop.CardRemoval != nil {
		return shop.CardRemoval.EnoughGold
	}
	return false
}

func shouldBuyCardRemoval(state *game.StateSnapshot) bool {
	if state == nil || state.Shop == nil || state.Shop.CardRemoval == nil {
		return false
	}
	cardRemoval := state.Shop.CardRemoval
	if !cardRemoval.IsStocked || !cardRemoval.EnoughGold {
		return false
	}

	deckCount := 99
	floor := 0
	gold := 0
	if state.Run != nil {
		deckCount = state.Run.DeckCount
		floor = state.Run.Floor
		gold = state.Run.Gold
	}
	if deckCount <= 0 {
		deckCount = 99
	}

	switch {
	case deckCount <= 16:
		return true
	case floor <= 12 && gold >= 80:
		return true
	case hpRatio(state) < 0.70 && gold >= 75:
		return true
	case gold >= 100:
		return true
	default:
		return false
	}
}

func bestAffordableShopRelic(state *game.StateSnapshot) (*int, float64, bool) {
	return bestAffordableShopOption(state, "relics", scoreShopRelicChoice, 3.0)
}

func bestAffordableShopCard(state *game.StateSnapshot) (*int, float64, bool) {
	return bestAffordableShopOption(state, "cards", scoreShopCardChoice, 3.0)
}

func bestAffordableShopPotion(state *game.StateSnapshot) (*int, float64, bool) {
	return bestAffordableShopOption(state, "potions", scoreShopPotionChoice, 1.5)
}

func bestAffordableShopRelicEstimate(state *game.StateSnapshot) (*int, DepthEstimate, bool) {
	return bestAffordableShopOptionEstimate(state, "relics", estimateShopRelicDepth, 3.0)
}

func bestAffordableShopCardEstimate(state *game.StateSnapshot) (*int, DepthEstimate, bool) {
	return bestAffordableShopOptionEstimate(state, "cards", estimateShopCardDepth, 4.0)
}

func bestAffordableShopPotionEstimate(state *game.StateSnapshot) (*int, DepthEstimate, bool) {
	return bestAffordableShopOptionEstimate(state, "potions", estimateShopPotionDepth, 2.0)
}

func bestAffordableShopOption(state *game.StateSnapshot, key string, scorer func(*game.StateSnapshot, map[string]any) float64, minimumScore float64) (*int, float64, bool) {
	if state == nil {
		return nil, 0, false
	}

	bestIndex := -1
	bestScore := minimumScore
	for _, option := range shopItemsToMaps(state, key) {
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

func bestAffordableShopOptionEstimate(state *game.StateSnapshot, key string, estimator func(*game.StateSnapshot, map[string]any) DepthEstimate, minimumScore float64) (*int, DepthEstimate, bool) {
	if state == nil {
		return nil, DepthEstimate{}, false
	}

	bestIndex := -1
	bestScore := minimumScore
	bestEstimate := DepthEstimate{}
	for _, option := range shopItemsToMaps(state, key) {
		if !fieldBool(option, "enoughGold") {
			continue
		}
		index, ok := fieldInt(option, "index")
		if !ok {
			continue
		}
		estimate := estimator(state, option)
		if estimate.Score < minimumScore {
			continue
		}
		if bestIndex < 0 || estimate.Score > bestScore || (estimate.Score == bestScore && index < bestIndex) {
			bestIndex = index
			bestScore = estimate.Score
			bestEstimate = estimate
		}
	}
	if bestIndex < 0 {
		return nil, DepthEstimate{}, false
	}
	return &bestIndex, bestEstimate, true
}

func scoreShopCardChoice(state *game.StateSnapshot, card map[string]any) float64 {
	score := scoreRewardCardChoice(state, card)
	price := fieldIntValue(card, "price")
	score -= float64(price) / 35.0
	floor := runFloor(state)
	hp := hpRatio(state)

	deckCount := runDeckCount(state)
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
	if floor <= 10 && hp < 0.60 && containsAny(cardID, "SHRUG_IT_OFF", "TRUE_GRIT", "IRON_WAVE", "CLOTHESLINE", "HEADBUTT") {
		score += 1.5
	}
	score += estimateShopCardDepth(state, card).Score

	return score
}

func scoreShopRelicChoice(state *game.StateSnapshot, relic map[string]any) float64 {
	score := 7.0 + estimateShopRelicDepth(state, relic).Score
	price := fieldIntValue(relic, "price")
	gold := runGold(state)
	floor := runFloor(state)
	score -= float64(price) / 55.0

	if floor <= 10 {
		score += 1.0
	}
	if gold-price >= 80 {
		score += 0.8
	}
	if hpRatio(state) < 0.55 {
		score += 0.5
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
	gold := runGold(state)
	if gold < 180 {
		return -10
	}

	score := 2.5 + estimateShopPotionDepth(state, potion).Score
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
