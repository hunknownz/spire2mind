package agentruntime

import (
	"fmt"
	"sort"
	"strings"

	"spire2mind/internal/game"
	"spire2mind/internal/i18n"
)

type combatSelectionPurpose string

const (
	combatSelectionPurposeUnknown combatSelectionPurpose = "unknown"
	combatSelectionPurposeExhaust combatSelectionPurpose = "exhaust"
	combatSelectionPurposeUpgrade combatSelectionPurpose = "upgrade"
)

func isCombatSelectionState(state *game.StateSnapshot) bool {
	if state == nil || !strings.EqualFold(state.Screen, "CARD_SELECTION") {
		return false
	}
	if !strings.EqualFold(fieldString(state.Selection, "sourceScreen"), "COMBAT") {
		return false
	}
	return hasAction(state, "select_deck_card")
}

func analyzeCombatSelection(state *game.StateSnapshot, language i18n.Language, mode string) *CombatPlan {
	if !isCombatSelectionState(state) {
		return nil
	}

	purpose := inferCombatSelectionPurpose(state)
	if purpose == combatSelectionPurposeUnknown {
		return nil
	}

	loc := i18n.New(language)
	candidates := rankCombatSelectionCandidates(state, purpose)
	if len(candidates) == 0 {
		return nil
	}

	primaryGoal := loc.Label("Choose the combat-selection target with the best tactical value", "为当前战斗选牌挑出战术价值最高的目标")
	switch purpose {
	case combatSelectionPurposeExhaust:
		primaryGoal = loc.Label("Exhaust the lowest-value card first", "优先消耗当前价值最低的牌")
	case combatSelectionPurposeUpgrade:
		primaryGoal = loc.Label("Upgrade the highest-impact combat card first", "优先升级当前影响最大的战斗牌")
	}

	reasons := []string{
		loc.Label(
			fmt.Sprintf("Selection source hint: %s.", valueOrDash(fieldString(state.Selection, "sourceHint"))),
			fmt.Sprintf("当前选牌来源提示：%s。", valueOrDash(fieldString(state.Selection, "sourceHint"))),
		),
	}
	if incoming := buildCombatSnapshot(state, nil).IncomingDamage; incoming > 0 {
		reasons = append(reasons, loc.Label(
			fmt.Sprintf("Visible incoming damage is %d, so selection value is judged in combat context.", incoming),
			fmt.Sprintf("当前可见 incoming damage 为 %d，因此本次选牌按战斗上下文评估。", incoming),
		))
	}
	best := candidates[0]
	reasons = append(reasons, loc.Label(
		fmt.Sprintf("Best selection target right now: %s (%.2f).", best.Label, best.Score),
		fmt.Sprintf("当前最优选牌目标：%s（%.2f）。", best.Label, best.Score),
	))

	summary := loc.Label("combat-embedded card choice", "战斗内嵌选牌")
	switch purpose {
	case combatSelectionPurposeExhaust:
		summary = loc.Label("combat exhaust choice", "战斗消耗选牌")
	case combatSelectionPurposeUpgrade:
		summary = loc.Label("combat upgrade choice", "战斗升级选牌")
	}

	return &CombatPlan{
		Mode:         mode,
		Summary:      summary,
		PrimaryGoal:  primaryGoal,
		TargetLabel:  best.Label,
		FocusReasons: reasons,
		Candidates:   topCombatPlanCandidates(candidates, 3),
	}
}

func inferCombatSelectionPurpose(state *game.StateSnapshot) combatSelectionPurpose {
	text := strings.ToLower(strings.TrimSpace(fieldString(state.Selection, "prompt") + " " + fieldString(state.Selection, "sourceHint")))
	switch {
	case strings.Contains(text, "消耗"), strings.Contains(text, "exhaust"), strings.Contains(text, "burningpact"), strings.Contains(text, "true_grit"):
		return combatSelectionPurposeExhaust
	case strings.Contains(text, "升级"), strings.Contains(text, "upgrade"), strings.Contains(text, "armaments"), strings.Contains(text, "smith"):
		return combatSelectionPurposeUpgrade
	default:
		return combatSelectionPurposeUnknown
	}
}

func rankCombatSelectionCandidates(state *game.StateSnapshot, purpose combatSelectionPurpose) []CombatPlanCandidate {
	cards := nestedList(state.Selection, "cards")
	snapshot := buildCombatSnapshot(state, nil)
	candidates := make([]CombatPlanCandidate, 0, len(cards))
	for _, card := range cards {
		index := fieldIntValue(card, "index")
		label := fmt.Sprintf("select [%d] %s", index, fallbackID(fieldString(card, "name"), fieldString(card, "cardId")))
		score := scoreCombatSelectionCard(card, snapshot, purpose)
		candidates = append(candidates, CombatPlanCandidate{
			Action: "select_deck_card",
			Label:  label,
			Score:  score,
		})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].Label < candidates[j].Label
		}
		return candidates[i].Score > candidates[j].Score
	})
	return candidates
}

func scoreCombatSelectionCard(card map[string]any, snapshot CombatSnapshot, purpose combatSelectionPurpose) float64 {
	cardID := strings.ToUpper(strings.TrimSpace(firstNonEmpty(fieldString(card, "cardId"), fieldString(card, "id"))))
	name := strings.ToLower(strings.TrimSpace(fieldString(card, "name")))
	cost := fieldIntValue(card, "energyCost")

	isStatusLike := containsAny(cardID, "SLIMED", "WOUND", "DAZED", "BURN", "VOID") ||
		containsAny(strings.ToUpper(name), "SLIMED", "WOUND", "DAZED", "BURN", "VOID")

	switch purpose {
	case combatSelectionPurposeExhaust:
		score := 0.0
		if isStatusLike {
			score += 20.0
		}
		switch {
		case strings.Contains(cardID, "DEFEND"):
			score += 4.5
		case strings.Contains(cardID, "STRIKE"):
			score += 3.5
		case strings.Contains(cardID, "SLASH"), strings.Contains(cardID, "BASH"), strings.Contains(cardID, "THUNDERCLAP"):
			score += 1.5
		case strings.Contains(cardID, "BURNING_PACT"), strings.Contains(cardID, "TRUE_GRIT"), strings.Contains(cardID, "ARMAMENTS"):
			score -= 3.0
		}
		if snapshot.IncomingDamage > snapshot.Player.Block {
			if strings.Contains(cardID, "DEFEND") || strings.Contains(cardID, "TRUE_GRIT") {
				score -= 3.5
			}
		}
		score += float64(max(0, cost-1)) * 0.4
		return score

	case combatSelectionPurposeUpgrade:
		score := 0.0
		if isStatusLike {
			return -20.0
		}
		switch {
		case strings.Contains(cardID, "BASH"):
			score += 9.0
		case strings.Contains(cardID, "THUNDERCLAP"), strings.Contains(cardID, "SWORD_BOOMERANG"):
			score += 8.0
		case strings.Contains(cardID, "BURNING_PACT"), strings.Contains(cardID, "TRUE_GRIT"), strings.Contains(cardID, "ARMAMENTS"):
			score += 7.0
		case strings.Contains(cardID, "STRIKE"):
			score += 4.0
		case strings.Contains(cardID, "DEFEND"):
			score += 3.0
		default:
			score += 5.0
		}
		if snapshot.IncomingDamage > snapshot.Player.Block && strings.Contains(cardID, "DEFEND") {
			score += 1.5
		}
		return score
	default:
		return 0
	}
}

func containsAny(value string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(value, needle) {
			return true
		}
	}
	return false
}
