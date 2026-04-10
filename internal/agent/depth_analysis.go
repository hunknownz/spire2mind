package agentruntime

import (
	"fmt"
	"sort"
	"strings"

	"spire2mind/internal/game"
	"spire2mind/internal/i18n"
)

type DepthEstimate struct {
	SurvivalOdds           float64
	DepthAdvanceOdds       float64
	ResourceConversionOdds float64
	Score                  float64
	RiskBand               string
	Reason                 string
}

func (e DepthEstimate) Summary(language i18n.Language) string {
	loc := i18n.New(language)
	risk := localizedRiskBand(loc, e.RiskBand)
	if language == i18n.LanguageChinese {
		return fmt.Sprintf("生存 %.0f%% / 深层 %.0f%% / 转化 %.0f%% / 风险 %s / 分数 %.1f",
			e.SurvivalOdds*100,
			e.DepthAdvanceOdds*100,
			e.ResourceConversionOdds*100,
			risk,
			e.Score,
		)
	}
	return fmt.Sprintf("survival %.0f%% / depth %.0f%% / conversion %.0f%% / risk %s / score %.1f",
		e.SurvivalOdds*100,
		e.DepthAdvanceOdds*100,
		e.ResourceConversionOdds*100,
		risk,
		e.Score,
	)
}

func localizedRiskBand(loc i18n.Localizer, riskBand string) string {
	switch strings.ToLower(strings.TrimSpace(riskBand)) {
	case "safe":
		return loc.Label("safe", "稳健")
	case "risky":
		return loc.Label("risky", "冒险")
	case "danger":
		return loc.Label("danger", "危险")
	case "balanced", "":
		return loc.Label("balanced", "均衡")
	default:
		return strings.TrimSpace(riskBand)
	}
}

func estimateMapNodeDepth(state *game.StateSnapshot, node map[string]any) DepthEstimate {
	hp := hpRatio(state)
	gold := fieldIntValue(state.Run, "gold")
	floor := fieldIntValue(state.Run, "floor")
	nodeType := normalizeMapNodeType(fieldString(node, "nodeType"))

	survival := 0.60
	depth := 0.52
	conversion := 0.18
	reason := "balanced progression"
	riskBand := "balanced"

	switch nodeType {
	case "rest":
		survival += 0.16
		depth += 0.06
		reason = "recover HP and stabilize the next few rooms"
		if hp < 0.55 {
			survival += 0.16
			depth += 0.10
			riskBand = "safe"
		}
		if hp > 0.80 {
			depth -= 0.10
		}
	case "shop":
		survival += 0.06
		depth += 0.08
		conversion += 0.36
		reason = "convert gold into immediate power before it becomes stranded"
		if gold >= 120 {
			depth += 0.10
			conversion += 0.12
			riskBand = "safe"
		}
		if gold < 80 {
			depth -= 0.14
			conversion -= 0.12
		}
	case "event":
		survival += 0.08
		depth += 0.12
		conversion += 0.08
		reason = "take a lower-variance room that can still improve the run"
		if hp < 0.55 {
			survival += 0.05
			riskBand = "safe"
		}
	case "combat":
		survival += 0.02
		depth += 0.14
		conversion += 0.04
		reason = "earn steady rewards without elite-level variance"
		if hp < 0.55 {
			survival -= 0.12
			riskBand = "risky"
		}
	case "elite":
		survival -= 0.24
		depth += 0.26
		conversion += 0.10
		reason = "high upside only if the run can absorb the risk"
		riskBand = "risky"
		if hp >= 0.75 && floor <= 10 {
			survival += 0.14
			depth += 0.10
			riskBand = "balanced"
		}
		if hp < 0.55 {
			survival -= 0.18
			depth -= 0.10
			riskBand = "danger"
		}
	case "chest":
		survival += 0.07
		depth += 0.16
		conversion += 0.10
		reason = "take free value without paying HP"
		riskBand = "safe"
	case "boss":
		survival += 0.02
		depth += 0.20
		reason = "advance only when the route leaves no better branch"
	case "":
		survival -= 0.08
		depth -= 0.05
		reason = "unknown node type"
		riskBand = "risky"
	default:
		survival -= 0.04
		depth -= 0.02
		reason = "unclassified room"
		riskBand = "risky"
	}

	if floor <= 8 && hp < 0.70 && nodeType == "elite" {
		survival -= 0.08
	}
	if floor <= 10 && gold >= 120 && nodeType == "shop" {
		depth += 0.08
		conversion += 0.08
	}

	return finalizeDepthEstimate(survival, depth, conversion, riskBand, reason)
}

func estimateRewardCardDepth(state *game.StateSnapshot, card map[string]any) DepthEstimate {
	cardID := strings.ToUpper(strings.TrimSpace(firstNonEmpty(fieldString(card, "cardId"), fieldString(card, "id"))))
	name := strings.ToLower(strings.TrimSpace(fieldString(card, "name")))
	floor := fieldIntValue(state.Run, "floor")
	hp := hpRatio(state)

	survival := 0.48
	depth := 0.50
	conversion := 0.15
	reason := "neutral card pick"
	riskBand := "balanced"

	switch {
	case containsAny(cardID, "SHRUG_IT_OFF", "IRON_WAVE", "TRUE_GRIT", "CLOTHESLINE"):
		survival += 0.28
		depth += 0.18
		reason = "stabilizes early fights and preserves HP"
		riskBand = "safe"
	case containsAny(cardID, "POMMEL_STRIKE", "HEADBUTT", "THUNDERCLAP", "SWORD_BOOMERANG"):
		survival += 0.14
		depth += 0.24
		reason = "adds immediate tempo and cleaner early fights"
	case containsAny(cardID, "ARMAMENTS", "BATTLE_TRANCE", "RAGE", "SPOT_WEAKNESS"):
		survival += 0.08
		depth += 0.14
		reason = "solid support card if the deck can cash it in soon"
	case containsAny(cardID, "LIMIT_BREAK", "BARRICADE", "JUGGERNAUT", "DEMON_FORM", "FIRE_BREATHING", "SEARING_BLOW"):
		survival -= 0.12
		depth += 0.06
		reason = "slow payoff that can strand the run before it stabilizes"
		riskBand = "risky"
	case containsAny(cardID, "STRIKE", "DEFEND"):
		survival -= 0.05
		depth -= 0.08
		reason = "does not improve the deck enough to justify the slot"
		riskBand = "risky"
	default:
		if strings.Contains(name, "block") {
			survival += 0.12
			depth += 0.08
			reason = "improves defensive consistency"
		} else if strings.Contains(name, "damage") {
			survival += 0.06
			depth += 0.10
			reason = "adds immediate output"
		}
	}

	if floor <= 6 {
		depth += 0.06
	}
	if floor <= 12 && containsAny(cardID, "LIMIT_BREAK", "BARRICADE", "JUGGERNAUT", "DEMON_FORM") {
		depth -= 0.12
	}
	if hp < 0.55 {
		if containsAny(cardID, "SHRUG_IT_OFF", "IRON_WAVE", "TRUE_GRIT", "CLOTHESLINE") {
			survival += 0.10
			depth += 0.06
			riskBand = "safe"
		}
		if containsAny(cardID, "LIMIT_BREAK", "BARRICADE", "JUGGERNAUT", "DEMON_FORM") {
			survival -= 0.14
			depth -= 0.10
			riskBand = "danger"
		}
	}

	return finalizeDepthEstimate(survival, depth, conversion, riskBand, reason)
}

func estimateShopCardDepth(state *game.StateSnapshot, card map[string]any) DepthEstimate {
	estimate := estimateRewardCardDepth(state, card)
	price := fieldIntValue(card, "price")
	gold := fieldIntValue(state.Run, "gold")

	estimate.ResourceConversionOdds = clampProbability(estimate.ResourceConversionOdds + 0.34 - float64(price)/220.0)
	estimate.DepthAdvanceOdds = clampProbability(estimate.DepthAdvanceOdds - float64(price)/360.0)
	if gold-price < 60 {
		estimate.DepthAdvanceOdds = clampProbability(estimate.DepthAdvanceOdds - 0.05)
	}
	estimate.Score = depthEstimateScore(estimate.SurvivalOdds, estimate.DepthAdvanceOdds, estimate.ResourceConversionOdds)
	return estimate
}

func estimateShopRelicDepth(state *game.StateSnapshot, relic map[string]any) DepthEstimate {
	price := fieldIntValue(relic, "price")
	gold := fieldIntValue(state.Run, "gold")
	floor := fieldIntValue(state.Run, "floor")
	name := strings.ToLower(strings.TrimSpace(firstNonEmpty(fieldString(relic, "name"), fieldString(relic, "relicId"), fieldString(relic, "id"))))

	survival := 0.56
	depth := 0.62
	conversion := 0.44
	reason := "generic relic pickup"
	riskBand := "balanced"

	switch {
	case strings.Contains(name, "anchor"),
		strings.Contains(name, "lantern"),
		strings.Contains(name, "vajra"),
		strings.Contains(name, "bag"),
		strings.Contains(name, "horn"),
		strings.Contains(name, "fan"),
		strings.Contains(name, "thread"),
		strings.Contains(name, "wheel"):
		survival += 0.10
		depth += 0.16
		reason = "strong immediate relic that pays off right away"
		riskBand = "safe"
	case strings.Contains(name, "potion"),
		strings.Contains(name, "shop"),
		strings.Contains(name, "membership"):
		depth += 0.08
		conversion += 0.08
		reason = "economy relic that can keep compounding"
	}

	if floor <= 10 {
		depth += 0.08
	}
	if gold-price >= 80 {
		conversion += 0.10
	}
	if hpRatio(state) < 0.55 {
		survival += 0.06
	}
	if price > 0 {
		depth -= float64(price) / 420.0
		conversion -= float64(price) / 500.0
	}

	return finalizeDepthEstimate(survival, depth, conversion, riskBand, reason)
}

func estimateShopPotionDepth(state *game.StateSnapshot, potion map[string]any) DepthEstimate {
	price := fieldIntValue(potion, "price")
	gold := fieldIntValue(state.Run, "gold")
	name := strings.ToLower(strings.TrimSpace(firstNonEmpty(fieldString(potion, "name"), fieldString(potion, "potionId"), fieldString(potion, "id"))))

	survival := 0.48
	depth := 0.38
	conversion := 0.24
	reason := "small tactical edge"
	riskBand := "balanced"

	switch {
	case strings.Contains(name, "block"), strings.Contains(name, "armor"), strings.Contains(name, "regen"):
		survival += 0.16
		reason = "defensive potion that can save HP in a bad fight"
	case strings.Contains(name, "strength"), strings.Contains(name, "fire"), strings.Contains(name, "attack"):
		depth += 0.08
		reason = "offensive potion that can convert a key fight"
	}
	if gold < 180 {
		depth -= 0.20
		conversion -= 0.12
		riskBand = "risky"
	}
	if price > 0 {
		conversion -= float64(price) / 360.0
	}

	return finalizeDepthEstimate(survival, depth, conversion, riskBand, reason)
}

func estimateCardRemovalDepth(state *game.StateSnapshot) DepthEstimate {
	deckCount := fieldIntValue(state.Run, "deckCount")
	floor := fieldIntValue(state.Run, "floor")
	gold := fieldIntValue(state.Run, "gold")
	hp := hpRatio(state)

	survival := 0.58
	depth := 0.62
	conversion := 0.52
	reason := "thin the deck so strong cards show up more often"
	riskBand := "safe"

	if deckCount <= 16 {
		depth += 0.10
	}
	if deckCount >= 20 {
		depth += 0.08
		conversion += 0.08
	}
	if floor <= 12 && gold >= 120 {
		depth += 0.08
	}
	if hp < 0.60 {
		survival += 0.06
	}

	return finalizeDepthEstimate(survival, depth, conversion, riskBand, reason)
}

func highLeverageProbabilityBlock(state *game.StateSnapshot, language i18n.Language) string {
	lines := highLeverageProbabilityLines(state, language)
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n")
}

func highLeverageProbabilityLines(state *game.StateSnapshot, language i18n.Language) []string {
	if state == nil {
		return nil
	}
	loc := i18n.New(language)
	switch strings.ToUpper(strings.TrimSpace(state.Screen)) {
	case "MAP":
		type option struct {
			label    string
			estimate DepthEstimate
		}
		var options []option
		for _, node := range nestedList(state.Map, "availableNodes") {
			index, ok := fieldInt(node, "index")
			if !ok {
				continue
			}
			nodeType := fieldString(node, "nodeType")
			label := fmt.Sprintf("%s %d (%s)", loc.Label("Node", "节点"), index, nodeType)
			options = append(options, option{label: label, estimate: estimateMapNodeDepth(state, node)})
		}
		if len(options) == 0 {
			return nil
		}
		sort.SliceStable(options, func(i, j int) bool {
			if options[i].estimate.Score == options[j].estimate.Score {
				return options[i].label < options[j].label
			}
			return options[i].estimate.Score > options[j].estimate.Score
		})
		lines := []string{loc.Label("Depth odds", "深层概率") + ":"}
		for _, option := range options {
			lines = append(lines, fmt.Sprintf("- %s: %s", option.label, option.estimate.Summary(language)))
		}
		return lines
	case "REWARD", "CARD_SELECTION":
		cards := rewardCardOptions(state)
		if len(cards) == 0 && strings.EqualFold(state.Screen, "CARD_SELECTION") {
			cards = nestedList(state.Selection, "cards")
		}
		if len(cards) == 0 {
			return nil
		}
		type option struct {
			label    string
			estimate DepthEstimate
		}
		var options []option
		for _, card := range cards {
			index, ok := fieldInt(card, "index")
			if !ok {
				continue
			}
			label := fmt.Sprintf("%s %d (%s)", loc.Label("Card", "卡牌"), index, firstNonEmpty(fieldString(card, "name"), fieldString(card, "cardId"), fieldString(card, "id")))
			options = append(options, option{label: label, estimate: estimateRewardCardDepth(state, card)})
		}
		if len(options) == 0 {
			return nil
		}
		sort.SliceStable(options, func(i, j int) bool {
			if options[i].estimate.Score == options[j].estimate.Score {
				return options[i].label < options[j].label
			}
			return options[i].estimate.Score > options[j].estimate.Score
		})
		lines := []string{loc.Label("Depth odds", "深层概率") + ":"}
		for _, option := range options {
			lines = append(lines, fmt.Sprintf("- %s: %s", option.label, option.estimate.Summary(language)))
		}
		return lines
	case "SHOP":
		if !shopInventoryOpen(state) {
			return nil
		}
		lines := []string{loc.Label("Depth odds", "深层概率") + ":"}
		if hasAffordableCardRemoval(state) {
			estimate := estimateCardRemovalDepth(state)
			lines = append(lines, fmt.Sprintf("- %s: %s", loc.Label("Card removal", "移除卡牌"), estimate.Summary(language)))
		}
		if index, estimate, ok := bestAffordableShopRelicEstimate(state); ok {
			lines = append(lines, fmt.Sprintf("- %s %d: %s", loc.Label("Best relic", "最佳遗物"), *index, estimate.Summary(language)))
		}
		if index, estimate, ok := bestAffordableShopCardEstimate(state); ok {
			lines = append(lines, fmt.Sprintf("- %s %d: %s", loc.Label("Best card", "最佳卡牌"), *index, estimate.Summary(language)))
		}
		if index, estimate, ok := bestAffordableShopPotionEstimate(state); ok {
			lines = append(lines, fmt.Sprintf("- %s %d: %s", loc.Label("Best potion", "最佳药水"), *index, estimate.Summary(language)))
		}
		if len(lines) == 1 {
			return nil
		}
		return lines
	default:
		return nil
	}
}

func normalizeMapNodeType(nodeType string) string {
	switch strings.ToLower(strings.TrimSpace(nodeType)) {
	case "monster", "combat", "fight", "enemy":
		return "combat"
	case "shop", "merchant", "store":
		return "shop"
	case "rest", "campfire":
		return "rest"
	case "event", "question", "?":
		return "event"
	case "elite":
		return "elite"
	case "chest", "treasure":
		return "chest"
	case "boss":
		return "boss"
	default:
		return strings.ToLower(strings.TrimSpace(nodeType))
	}
}

func finalizeDepthEstimate(survival float64, depth float64, conversion float64, riskBand string, reason string) DepthEstimate {
	survival = clampProbability(survival)
	depth = clampProbability(depth)
	conversion = clampProbability(conversion)
	return DepthEstimate{
		SurvivalOdds:           survival,
		DepthAdvanceOdds:       depth,
		ResourceConversionOdds: conversion,
		Score:                  depthEstimateScore(survival, depth, conversion),
		RiskBand:               strings.TrimSpace(riskBand),
		Reason:                 strings.TrimSpace(reason),
	}
}

func depthEstimateScore(survival float64, depth float64, conversion float64) float64 {
	return survival*6.0 + depth*4.0 + conversion*2.0
}

func clampProbability(value float64) float64 {
	switch {
	case value < 0.05:
		return 0.05
	case value > 0.95:
		return 0.95
	default:
		return value
	}
}

func hasAffordableCardRemoval(state *game.StateSnapshot) bool {
	if state == nil {
		return false
	}
	cardRemoval := nestedMap(state.Shop, "cardRemoval")
	return len(cardRemoval) > 0 && fieldBool(cardRemoval, "available") && fieldBool(cardRemoval, "enoughGold")
}
