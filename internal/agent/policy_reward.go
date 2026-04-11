package agentruntime

import (
	"fmt"
	"strings"

	"spire2mind/internal/game"
)

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

	if state.Selection == nil {
		return nil
	}
	bestIndex := -1
	bestScore := -1e9
	for _, card := range state.Selection.Cards {
		cardMap := cardStateToMap(card)
		score := scoreRewardCardChoice(state, cardMap)
		if bestIndex < 0 || score > bestScore || (score == bestScore && card.Index < bestIndex) {
			bestIndex = card.Index
			bestScore = score
		}
	}
	if bestIndex < 0 {
		return nil
	}
	return &bestIndex
}

func isRewardCardSelection(state *game.StateSnapshot) bool {
	if state == nil || state.Selection == nil {
		return false
	}
	sourceHint := strings.ToLower(strings.TrimSpace(state.Selection.SourceHint))
	kind := strings.ToLower(strings.TrimSpace(state.Selection.Kind))
	return strings.Contains(sourceHint, "reward") ||
		strings.Contains(kind, "reward") ||
		(hasAction(state, "choose_reward_card") && len(state.Selection.Cards) > 0)
}

func rewardCardOptions(state *game.StateSnapshot) []map[string]any {
	if state == nil {
		return nil
	}
	if state.Reward != nil && len(state.Reward.CardOptions) > 0 {
		return cardStatesToMaps(state.Reward.CardOptions)
	}
	if isRewardCardSelection(state) {
		return selectionCardsMaps(state)
	}
	return nil
}

// activeKnowledge is set by Session on startup for static policy access.
var activeKnowledge *KnowledgeRetriever

func scoreRewardCardChoice(state *game.StateSnapshot, card map[string]any) float64 {
	cardID := strings.ToUpper(strings.TrimSpace(firstNonEmpty(fieldString(card, "cardId"), fieldString(card, "id"))))
	name := strings.ToLower(strings.TrimSpace(fieldString(card, "name")))

	// Try knowledge-base score first
	if activeKnowledge != nil {
		activeKnowledge.EnsureLoaded()
		if analysis, ok := activeKnowledge.cards[cardID]; ok {
			// Use knowledge-base score as primary (scale 1-10 → game score range)
			score := analysis.Score * 1.5 // Scale to ~0-15 range

			// Apply situational modifiers
			floor := runFloor(state)
			hp := hpRatio(state)

			// Timing modifier
			switch analysis.Timing {
			case "early":
				if floor <= 5 {
					score += 2.0
				} else if floor >= 12 {
					score -= 2.0
				}
			case "late":
				if floor >= 10 {
					score += 2.0
				} else if floor <= 5 {
					score -= 3.0
				}
			}

			// HP-based modifier for defensive cards
			if hp < 0.50 && (analysis.Role == "defense" || analysis.Role == "utility") {
				score += 3.0
			}
			if hp < 0.50 && analysis.Tier == "S" && analysis.Role == "scaling" {
				score -= 2.0 // Don't pick slow scaling when dying
			}

			// Basic card penalty
			if containsAny(cardID, "DEFEND", "STRIKE") {
				score -= 2.0
			}

			return score
		}
	}

	// Fallback: original hardcoded scoring
	score := estimateRewardCardDepth(state, card).Score
	floor := runFloor(state)
	hp := hpRatio(state)

	if floor <= 6 {
		score += earlyActCardBonus(cardID, name)
	}
	if hp < 0.55 {
		score += survivalCardBonus(cardID, name)
	}
	if floor <= 12 {
		score += immediatePowerBonus(cardID, name)
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

	if containsAny(cardID, "DEFEND", "STRIKE") {
		score -= 1.0
	}
	if hp < 0.50 && containsAny(cardID, "LIMIT_BREAK", "BARRICADE", "JUGGERNAUT", "DEMON_FORM") {
		score -= 3.5
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

func immediatePowerBonus(cardID string, name string) float64 {
	switch {
	case containsAny(cardID, "SHRUG_IT_OFF", "IRON_WAVE", "POMMEL_STRIKE", "THUNDERCLAP", "HEADBUTT", "CLOTHESLINE", "TRUE_GRIT"):
		return 2.5
	case containsAny(cardID, "BATTLE_TRANCE", "ARMAMENTS", "RAGE", "SPOT_WEAKNESS"):
		return 0.8
	case containsAny(cardID, "LIMIT_BREAK", "BARRICADE", "JUGGERNAUT", "DEMON_FORM", "SEARING_BLOW"):
		return -2.5
	default:
		if strings.Contains(name, "draw") || strings.Contains(name, "block") || strings.Contains(name, "damage") {
			return 0.5
		}
		return 0
	}
}

func chooseDeckSelectionAction(state *game.StateSnapshot) (game.ActionRequest, string, bool) {
	if state == nil || !strings.EqualFold(state.Screen, "CARD_SELECTION") {
		return game.ActionRequest{}, "", false
	}

	if hasAction(state, "confirm_selection") && selectionCanConfirmNow(state) {
		selectedCount := 0
		minSelect := 1
		if state.Selection != nil {
			selectedCount = state.Selection.CurrentSelection
			minSelect = state.Selection.MinSelection
			if minSelect <= 0 {
				minSelect = 1
			}
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

	if index := firstIndexedOptionTyped(state, "Selection", "cards"); index != nil {
		return game.ActionRequest{Action: "select_deck_card", OptionIndex: index}, "select the first available deck card", true
	}

	return game.ActionRequest{}, "", false
}

func firstUnselectedDeckSelectionOption(selection *game.SelectionState) *int {
	if selection == nil {
		return nil
	}
	for _, option := range selection.Cards {
		if option.IsSelected != nil && *option.IsSelected {
			continue
		}
		index := option.Index
		return &index
	}

	return nil
}

func selectionCanConfirmNow(state *game.StateSnapshot) bool {
	if state == nil || state.Selection == nil {
		return false
	}
	if !state.Selection.RequiresConfirmation || !state.Selection.CanConfirm {
		return false
	}
	selectedCount := state.Selection.CurrentSelection
	minSelect := state.Selection.MinSelection
	if minSelect <= 0 {
		minSelect = 1
	}
	return selectedCount >= minSelect
}

func firstClaimableReward(reward *game.RewardState) *int {
	if reward == nil {
		return nil
	}
	var claimable []int
	for _, option := range reward.Rewards {
		if !option.Claimable {
			continue
		}
		claimable = append(claimable, option.Index)
	}

	if len(claimable) > 0 {
		return &claimable[0]
	}

	return nil
}
