package agentruntime

import (
	"strings"

	"spire2mind/internal/game"
)

func deterministicCombatAction(state *game.StateSnapshot, failures *actionFailureMemory) (game.ActionRequest, bool) {
	if state == nil || state.Combat == nil {
		return game.ActionRequest{}, false
	}
	var playable []game.CardState
	for _, card := range state.Combat.Hand {
		if card.Playable {
			playable = append(playable, card)
		}
	}

	if len(playable) == 0 {
		return game.ActionRequest{}, false
	}

	stateDigest := digestState(state)
	for _, card := range choosePlayableCardsTyped(playable) {
		cardIndex := card.Index

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

func choosePlayableCardsTyped(playable []game.CardState) []game.CardState {
	if len(playable) == 0 {
		return nil
	}
	ordered := append([]game.CardState(nil), playable...)
	for i := 0; i < len(ordered)-1; i++ {
		best := i
		bestScore := combatCardScoreTyped(ordered[i])
		bestIndex := ordered[i].Index
		for j := i + 1; j < len(ordered); j++ {
			score := combatCardScoreTyped(ordered[j])
			index := ordered[j].Index
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
	if fieldBool(card, "requiresTarget") {
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

func combatCardScoreTyped(card game.CardState) int {
	score := 0
	if cardRequiresTarget(nil, card) {
		score += 3
	}
	if card.EnergyCost != nil {
		score += *card.EnergyCost
	}
	if card.CostsX {
		score += 4
	}
	if card.StarCostsX {
		score += 4
	}
	name := strings.ToLower(card.Name)
	if strings.Contains(name, "strike") || strings.Contains(name, "打击") {
		score -= 2
	}
	if strings.Contains(name, "defend") || strings.Contains(name, "防御") {
		score -= 1
	}
	return score
}
