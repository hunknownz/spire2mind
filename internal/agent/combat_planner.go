package agentruntime

import (
	"fmt"
	"sort"
	"strings"

	"spire2mind/internal/config"
	"spire2mind/internal/game"
	"spire2mind/internal/i18n"
)

type CombatPlanner interface {
	Name() string
	Analyze(state *game.StateSnapshot, codex *SeenContentRegistry, language i18n.Language) *CombatPlan
}

type CombatPlan struct {
	Mode         string                `json:"mode"`
	Summary      string                `json:"summary"`
	PrimaryGoal  string                `json:"primary_goal,omitempty"`
	TargetLabel  string                `json:"target_label,omitempty"`
	FocusReasons []string              `json:"focus_reasons,omitempty"`
	Candidates   []CombatPlanCandidate `json:"candidates,omitempty"`
}

type CombatPlanCandidate struct {
	Action        string               `json:"action"`
	Label         string               `json:"label"`
	Score         float64              `json:"score"`
	CardIndex     *int                 `json:"card_index,omitempty"`
	TargetIndex   *int                 `json:"target_index,omitempty"`
	OptionIndex   *int                 `json:"option_index,omitempty"`
	TradeEstimate *CombatTradeEstimate `json:"trade_estimate,omitempty"`
}

func (c CombatPlanCandidate) ActionRequest() (game.ActionRequest, bool) {
	if strings.TrimSpace(c.Action) == "" {
		return game.ActionRequest{}, false
	}
	return game.ActionRequest{
		Action:      c.Action,
		CardIndex:   cloneIntPointer(c.CardIndex),
		TargetIndex: cloneIntPointer(c.TargetIndex),
		OptionIndex: cloneIntPointer(c.OptionIndex),
	}, true
}

func (p *CombatPlan) BestActionRequest() (game.ActionRequest, bool) {
	if p == nil || len(p.Candidates) == 0 {
		return game.ActionRequest{}, false
	}
	return p.Candidates[0].ActionRequest()
}

type CombatSnapshot struct {
	Player           CombatPlayerState
	Hand             []CombatCardState
	Enemies          []CombatEnemyState
	CanPlayCard      bool
	CanEndTurn       bool
	Floor            int
	Gold             int
	IncomingDamage   int
	LowestEnemyLabel string
	LowestEnemyHP    int
	KnowledgeBiases  []string
}

type CombatPlayerState struct {
	CurrentHP int
	MaxHP     int
	Block     int
	Energy    int
	Stars     int
}

type CombatCardState struct {
	Index          int
	CardID         string
	Name           string
	EnergyCost     int
	Playable       bool
	RequiresTarget bool
	ValidTargets   []int
	KnowledgePrior float64
}

type CombatEnemyState struct {
	Index          int
	EnemyID        string
	Name           string
	CurrentHP      int
	Block          int
	Hittable       bool
	Vulnerable     int
	Intents        []CombatIntentState
	KnowledgePrior float64
}

type CombatIntentState struct {
	IntentType  string
	Label       string
	TotalDamage int
}

type CombatAction struct {
	Request game.ActionRequest
	Label   string
}

type CombatTradeEstimate struct {
	DamageDealt     int `json:"damage_dealt,omitempty"`
	Kills           int `json:"kills,omitempty"`
	ThreatReduction int `json:"threat_reduction,omitempty"`
	CoveredDamage   int `json:"covered_damage,omitempty"`
	PredictedHPLoss int `json:"predicted_hp_loss,omitempty"`
	WastedBlock     int `json:"wasted_block,omitempty"`
}

func (e *CombatTradeEstimate) Summary(language i18n.Language) string {
	if e == nil {
		return ""
	}

	loc := i18n.New(language)
	parts := []string{
		loc.Label(
			fmt.Sprintf("hp loss %d", e.PredictedHPLoss),
			fmt.Sprintf("战损 %d", e.PredictedHPLoss),
		),
	}
	if e.CoveredDamage > 0 {
		parts = append(parts, loc.Label(
			fmt.Sprintf("cover %d", e.CoveredDamage),
			fmt.Sprintf("格挡 %d", e.CoveredDamage),
		))
	}
	if e.ThreatReduction > 0 {
		parts = append(parts, loc.Label(
			fmt.Sprintf("threat -%d", e.ThreatReduction),
			fmt.Sprintf("减压 %d", e.ThreatReduction),
		))
	}
	if e.DamageDealt > 0 {
		parts = append(parts, loc.Label(
			fmt.Sprintf("deal %d", e.DamageDealt),
			fmt.Sprintf("伤害 %d", e.DamageDealt),
		))
	}
	if e.Kills > 0 {
		parts = append(parts, loc.Label(
			fmt.Sprintf("kill %d", e.Kills),
			fmt.Sprintf("击杀 %d", e.Kills),
		))
	}
	if e.WastedBlock > 0 {
		parts = append(parts, loc.Label(
			fmt.Sprintf("waste %d block", e.WastedBlock),
			fmt.Sprintf("溢出格挡 %d", e.WastedBlock),
		))
	}
	return strings.Join(parts, ", ")
}

func (e *CombatTradeEstimate) DataMap(language i18n.Language) map[string]any {
	if e == nil {
		return nil
	}
	return map[string]any{
		"damage_dealt":      e.DamageDealt,
		"kills":             e.Kills,
		"threat_reduction":  e.ThreatReduction,
		"covered_damage":    e.CoveredDamage,
		"predicted_hp_loss": e.PredictedHPLoss,
		"wasted_block":      e.WastedBlock,
		"summary":           e.Summary(language),
	}
}

func (p *CombatPlan) PromptBlock(language i18n.Language) string {
	if p == nil {
		return ""
	}

	loc := i18n.New(language)
	lines := []string{
		loc.Label("Combat planner", "战斗规划器") + ":",
		fmt.Sprintf("- %s: `%s`", loc.Label("Mode", "模式"), valueOrDash(p.Mode)),
		fmt.Sprintf("- %s: %s", loc.Label("Summary", "摘要"), valueOrDash(p.Summary)),
	}
	if strings.TrimSpace(p.PrimaryGoal) != "" {
		lines = append(lines, fmt.Sprintf("- %s: %s", loc.Label("Primary goal", "主要目标"), p.PrimaryGoal))
	}
	if strings.TrimSpace(p.TargetLabel) != "" {
		lines = append(lines, fmt.Sprintf("- %s: %s", loc.Label("Target bias", "目标倾向"), p.TargetLabel))
	}
	if len(p.FocusReasons) > 0 {
		lines = append(lines, "- "+loc.Label("Planner cues", "规划提示")+":")
		for _, reason := range p.FocusReasons {
			lines = append(lines, "  - "+reason)
		}
	}
	if len(p.Candidates) > 0 {
		lines = append(lines, "- "+loc.Label("Top candidates", "候选动作")+":")
		for _, candidate := range p.Candidates {
			line := fmt.Sprintf("  - `%s` %.2f %s", candidate.Action, candidate.Score, candidate.Label)
			if trade := strings.TrimSpace(candidate.TradeEstimate.Summary(language)); trade != "" {
				line += " | " + trade
			}
			lines = append(lines, line)
		}
	}

	return strings.Join(lines, "\n")
}

func (p *CombatPlan) DataMap(language i18n.Language) map[string]any {
	if p == nil {
		return nil
	}

	data := map[string]any{
		"mode":    p.Mode,
		"summary": p.Summary,
	}
	if p.PrimaryGoal != "" {
		data["primary_goal"] = p.PrimaryGoal
	}
	if p.TargetLabel != "" {
		data["target_label"] = p.TargetLabel
	}
	if len(p.FocusReasons) > 0 {
		data["focus_reasons"] = append([]string(nil), p.FocusReasons...)
	}
	if len(p.Candidates) > 0 {
		candidates := make([]map[string]any, 0, len(p.Candidates))
		for _, candidate := range p.Candidates {
			item := map[string]any{
				"action": candidate.Action,
				"label":  candidate.Label,
				"score":  candidate.Score,
			}
			if candidate.TradeEstimate != nil {
				item["trade_estimate"] = candidate.TradeEstimate.DataMap(language)
				item["trade_summary"] = candidate.TradeEstimate.Summary(language)
			}
			candidates = append(candidates, item)
		}
		data["candidates"] = candidates
	}
	return data
}

type noopCombatPlanner struct{}

func (noopCombatPlanner) Name() string { return "none" }

func (noopCombatPlanner) Analyze(_ *game.StateSnapshot, _ *SeenContentRegistry, _ i18n.Language) *CombatPlan {
	return nil
}

type heuristicCombatPlanner struct{}

func (heuristicCombatPlanner) Name() string { return "heuristic" }

func (heuristicCombatPlanner) Analyze(state *game.StateSnapshot, codex *SeenContentRegistry, language i18n.Language) *CombatPlan {
	if state == nil {
		return nil
	}
	if isCombatSelectionState(state) {
		return analyzeCombatSelection(state, language, "heuristic")
	}
	if !strings.EqualFold(state.Screen, "COMBAT") {
		return nil
	}

	loc := i18n.New(language)
	snapshot := buildCombatSnapshot(state, codex)
	playableCount := 0
	targetedCount := 0
	zeroCostPlayable := 0
	for _, card := range snapshot.Hand {
		if card.Playable {
			playableCount++
			if card.RequiresTarget {
				targetedCount++
			}
			if card.EnergyCost <= 0 {
				zeroCostPlayable++
			}
		}
	}

	reasons := make([]string, 0, 4)
	primaryGoal := loc.Label("Advance combat safely this turn", "本回合稳健推进战斗")
	summaryParts := []string{
		fmt.Sprintf("%s `%d`", loc.Label("energy", "能量"), snapshot.Player.Energy),
		fmt.Sprintf("%s `%d`", loc.Label("playable cards", "可打出的牌"), playableCount),
	}

	if snapshot.IncomingDamage > snapshot.Player.Block {
		overhang := snapshot.IncomingDamage - snapshot.Player.Block
		if snapshot.Player.CurrentHP > 0 && overhang >= snapshot.Player.CurrentHP {
			primaryGoal = loc.Label("Prioritize survival over greed", "优先保命，不要贪")
			reasons = append(reasons, loc.Label(
				fmt.Sprintf("Incoming damage %d exceeds block %d and threatens lethal against current HP %d.", snapshot.IncomingDamage, snapshot.Player.Block, snapshot.Player.CurrentHP),
				fmt.Sprintf("敌方预计伤害 %d 超过当前格挡 %d，并且对当前生命 %d 存在致死风险。", snapshot.IncomingDamage, snapshot.Player.Block, snapshot.Player.CurrentHP),
			))
		} else {
			primaryGoal = loc.Label("Respect incoming damage before pure greed", "先处理 incoming damage，再考虑贪输出")
			reasons = append(reasons, loc.Label(
				fmt.Sprintf("Incoming damage %d exceeds current block %d.", snapshot.IncomingDamage, snapshot.Player.Block),
				fmt.Sprintf("敌方预计伤害 %d 超过当前格挡 %d。", snapshot.IncomingDamage, snapshot.Player.Block),
			))
		}
	}

	targetLabel := ""
	if snapshot.LowestEnemyLabel != "" {
		targetLabel = loc.Label(
			fmt.Sprintf("Prefer stable focus on the lowest-HP enemy: %s (%d HP).", snapshot.LowestEnemyLabel, snapshot.LowestEnemyHP),
			fmt.Sprintf("优先稳定集火最低血敌人：%s（%d HP）。", snapshot.LowestEnemyLabel, snapshot.LowestEnemyHP),
		)
		reasons = append(reasons, targetLabel)
	}

	if zeroCostPlayable > 0 {
		reasons = append(reasons, loc.Label(
			fmt.Sprintf("There are %d zero-cost playable cards; do not end turn prematurely.", zeroCostPlayable),
			fmt.Sprintf("当前有 %d 张 0 费可打出牌，不要过早结束回合。", zeroCostPlayable),
		))
	}
	if targetedCount > 0 {
		reasons = append(reasons, loc.Label(
			fmt.Sprintf("%d playable cards still require target selection, so avoid stale target reuse.", targetedCount),
			fmt.Sprintf("仍有 %d 张可打出牌需要选目标，避免复用过期 target index。", targetedCount),
		))
	}
	if playableCount == 0 {
		reasons = append(reasons, loc.Label(
			"No playable cards remain; ending turn is likely correct once the action window is stable.",
			"当前没有可打出的牌；只要动作窗口稳定，结束回合大概率是正确动作。",
		))
	}
	for _, cue := range snapshot.KnowledgeBiases {
		reasons = append(reasons, loc.Label(
			cue,
			cue,
		))
	}

	candidates := rankCombatActions(snapshot, language)

	return &CombatPlan{
		Mode:         "heuristic",
		Summary:      strings.Join(summaryParts, ", "),
		PrimaryGoal:  primaryGoal,
		TargetLabel:  targetLabel,
		FocusReasons: reasons,
		Candidates:   topCombatPlanCandidates(candidates, 3),
	}
}

func NewCombatPlanner(cfg config.Config) CombatPlanner {
	switch strings.ToLower(strings.TrimSpace(cfg.CombatPlanner)) {
	case "", "heuristic":
		return heuristicCombatPlanner{}
	case "none", "off", "disabled":
		return noopCombatPlanner{}
	case "mcts":
		return newMCTSCombatPlanner()
	default:
		return heuristicCombatPlanner{}
	}
}

func buildCombatSnapshot(state *game.StateSnapshot, codex *SeenContentRegistry) CombatSnapshot {
	player := asMap(state.Combat["player"])
	snapshot := CombatSnapshot{
		Player: CombatPlayerState{
			CurrentHP: fieldIntValue(player, "currentHp"),
			MaxHP:     fieldIntValue(player, "maxHp"),
			Block:     fieldIntValue(player, "block"),
			Energy:    fieldIntValue(player, "energy"),
			Stars:     fieldIntValue(player, "stars"),
		},
		CanPlayCard: hasAction(state, "play_card"),
		CanEndTurn:  hasAction(state, "end_turn"),
		Floor:       fieldIntValue(state.Run, "floor"),
		Gold:        fieldIntValue(state.Run, "gold"),
	}

	for _, card := range nestedList(state.Combat, "hand") {
		snapshot.Hand = append(snapshot.Hand, CombatCardState{
			Index:          fieldIntValue(card, "index"),
			CardID:         fallbackID(fieldString(card, "cardId"), fieldString(card, "id")),
			Name:           fieldString(card, "name"),
			EnergyCost:     fieldIntValue(card, "energyCost"),
			Playable:       fieldBool(card, "playable"),
			RequiresTarget: cardRequiresTarget(state, card),
			ValidTargets:   append([]int(nil), fieldIntSlice(card, "validTargetIndices")...),
		})
	}

	for _, enemy := range nestedList(state.Combat, "enemies") {
		currentHP := fieldIntValue(enemy, "currentHp")
		label := fallbackID(fieldString(enemy, "name"), fieldString(enemy, "enemyId"))
		entry := CombatEnemyState{
			Index:     fieldIntValue(enemy, "index"),
			EnemyID:   fallbackID(fieldString(enemy, "enemyId"), fieldString(enemy, "id")),
			Name:      label,
			CurrentHP: currentHP,
			Block:     fieldIntValue(enemy, "block"),
			Hittable:  fieldBool(enemy, "isHittable"),
		}
		for _, intent := range nestedList(enemy, "intents") {
			intentState := CombatIntentState{
				IntentType: fieldString(intent, "intentType"),
				Label:      fieldString(intent, "label"),
			}
			if totalDamage, ok := fieldInt(intent, "totalDamage"); ok && totalDamage > 0 {
				intentState.TotalDamage = totalDamage
			} else if damage, ok := fieldInt(intent, "damage"); ok && damage > 0 {
				intentState.TotalDamage = damage
			}
			if intentState.TotalDamage > 0 {
				snapshot.IncomingDamage += intentState.TotalDamage
			}
			entry.Intents = append(entry.Intents, intentState)
		}
		if currentHP > 0 && (snapshot.LowestEnemyHP == 0 || currentHP < snapshot.LowestEnemyHP) {
			snapshot.LowestEnemyHP = currentHP
			snapshot.LowestEnemyLabel = label
		}
		snapshot.Enemies = append(snapshot.Enemies, entry)
	}

	applyCodexPriors(&snapshot, codex)

	return snapshot
}

func generateCombatActions(snapshot CombatSnapshot) []CombatAction {
	actions := make([]CombatAction, 0, len(snapshot.Hand)+1)
	if snapshot.CanPlayCard {
		for _, card := range snapshot.Hand {
			if !card.Playable {
				continue
			}
			if !card.RequiresTarget {
				actions = append(actions, CombatAction{
					Request: game.ActionRequest{
						Action:    "play_card",
						CardIndex: intPointer(card.Index),
					},
					Label: fmt.Sprintf("play [%d] %s", card.Index, fallbackID(card.Name, card.CardID)),
				})
				continue
			}

			targets := append([]int(nil), card.ValidTargets...)
			if len(targets) == 0 {
				for _, enemy := range snapshot.Enemies {
					if enemy.Hittable {
						targets = append(targets, enemy.Index)
					}
				}
			}
			for _, target := range targets {
				targetLabel := combatTargetLabel(snapshot, target)
				actions = append(actions, CombatAction{
					Request: game.ActionRequest{
						Action:      "play_card",
						CardIndex:   intPointer(card.Index),
						TargetIndex: intPointer(target),
					},
					Label: fmt.Sprintf("play [%d] %s -> [%d] %s", card.Index, fallbackID(card.Name, card.CardID), target, targetLabel),
				})
			}
		}
	}
	if snapshot.CanEndTurn {
		actions = append(actions, CombatAction{
			Request: game.ActionRequest{Action: "end_turn"},
			Label:   "end_turn",
		})
	}
	return actions
}

func rankCombatActions(snapshot CombatSnapshot, language i18n.Language) []CombatPlanCandidate {
	actions := generateCombatActions(snapshot)
	candidates := make([]CombatPlanCandidate, 0, len(actions))
	for _, action := range actions {
		score := scoreCombatAction(snapshot, action)
		candidates = append(candidates, combatPlanCandidateFromAction(snapshot, action, score))
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].Label < candidates[j].Label
		}
		return candidates[i].Score > candidates[j].Score
	})
	return candidates
}

func scoreCombatAction(snapshot CombatSnapshot, action CombatAction) float64 {
	if action.Request.Action == "end_turn" {
		score := 0.2
		playableCount := 0
		zeroCost := 0
		for _, card := range snapshot.Hand {
			if card.Playable {
				playableCount++
				if card.EnergyCost <= 0 {
					zeroCost++
				}
			}
		}
		if playableCount > 0 {
			score -= 2.0
		}
		if zeroCost > 0 {
			score -= 1.0
		}
		if snapshot.IncomingDamage > snapshot.Player.Block {
			score -= 0.5
		}
		return score
	}

	score := 1.0
	cardIndex := derefInt(action.Request.CardIndex)
	var chosen *CombatCardState
	for i := range snapshot.Hand {
		if snapshot.Hand[i].Index == cardIndex {
			chosen = &snapshot.Hand[i]
			break
		}
	}
	if chosen == nil {
		return -100
	}

	if chosen.Playable {
		score += 2.0
	}
	score += chosen.KnowledgePrior
	effect := estimateCardEffect(*chosen, snapshot.Player.Energy)
	lowPressureTurn := isLowPressureCombatTurn(snapshot)
	if chosen.EnergyCost <= 0 {
		score += 0.7
	}
	if chosen.RequiresTarget {
		score += 0.3
	}
	if lowPressureTurn {
		if effect.Damage > 0 {
			score += 1.1 + float64(effect.Damage)*0.08
		}
		if effect.ApplyVulnerable > 0 {
			score += 0.5
		}
		if effect.Block > 0 && effect.Damage == 0 {
			score -= 1.6
		}
	}
	if snapshot.IncomingDamage > snapshot.Player.Block {
		name := strings.ToLower(chosen.Name)
		if strings.Contains(name, "defend") || strings.Contains(name, "block") || strings.Contains(name, "armaments") {
			score += 1.5
		}
	} else if snapshot.IncomingDamage == 0 {
		name := strings.ToLower(chosen.Name)
		if strings.Contains(name, "defend") || strings.Contains(name, "block") {
			score -= 0.9
		}
	}
	if action.Request.TargetIndex != nil && snapshot.LowestEnemyHP > 0 {
		if target := combatTargetLabel(snapshot, *action.Request.TargetIndex); target == snapshot.LowestEnemyLabel {
			score += 0.8
		}
		for _, enemy := range snapshot.Enemies {
			if enemy.Index == *action.Request.TargetIndex {
				score += enemy.KnowledgePrior
				break
			}
		}
	}
	return score
}

func isLowPressureCombatTurn(snapshot CombatSnapshot) bool {
	return snapshot.IncomingDamage > 0 &&
		snapshot.Player.CurrentHP > snapshot.IncomingDamage*3 &&
		snapshot.IncomingDamage <= max(4, snapshot.Player.CurrentHP/8)
}

func topCombatPlanCandidates(candidates []CombatPlanCandidate, maxCount int) []CombatPlanCandidate {
	if maxCount <= 0 || len(candidates) == 0 {
		return nil
	}
	if len(candidates) <= maxCount {
		return append([]CombatPlanCandidate(nil), candidates...)
	}
	return append([]CombatPlanCandidate(nil), candidates[:maxCount]...)
}

func combatPlanCandidateFromAction(snapshot CombatSnapshot, action CombatAction, score float64) CombatPlanCandidate {
	return CombatPlanCandidate{
		Action:        action.Request.Action,
		Label:         action.Label,
		Score:         score,
		CardIndex:     cloneIntPointer(action.Request.CardIndex),
		TargetIndex:   cloneIntPointer(action.Request.TargetIndex),
		OptionIndex:   cloneIntPointer(action.Request.OptionIndex),
		TradeEstimate: estimateCombatTrade(snapshot, simulateCombatAction(combatSearchState{Snapshot: cloneCombatSnapshot(snapshot)}, action)),
	}
}

func estimateCombatTrade(initial CombatSnapshot, state combatSearchState) *CombatTradeEstimate {
	estimate := &CombatTradeEstimate{
		DamageDealt:     max(0, totalEnemyHP(initial)-totalEnemyHP(state.Snapshot)),
		Kills:           max(0, livingEnemyCount(initial)-livingEnemyCount(state.Snapshot)),
		ThreatReduction: max(0, initial.IncomingDamage-state.Snapshot.IncomingDamage),
		CoveredDamage:   min(state.Snapshot.IncomingDamage, state.Snapshot.Player.Block),
		PredictedHPLoss: max(0, state.Snapshot.IncomingDamage-state.Snapshot.Player.Block),
		WastedBlock:     max(0, state.Snapshot.Player.Block-state.Snapshot.IncomingDamage),
	}
	if estimate.DamageDealt == 0 &&
		estimate.Kills == 0 &&
		estimate.ThreatReduction == 0 &&
		estimate.CoveredDamage == 0 &&
		estimate.PredictedHPLoss == 0 &&
		estimate.WastedBlock == 0 {
		return nil
	}
	return estimate
}

func combatTargetLabel(snapshot CombatSnapshot, targetIndex int) string {
	for _, enemy := range snapshot.Enemies {
		if enemy.Index == targetIndex {
			return fallbackID(enemy.Name, enemy.EnemyID)
		}
	}
	return fmt.Sprintf("target %d", targetIndex)
}

func intPointer(value int) *int {
	v := value
	return &v
}

func cloneIntPointer(pointer *int) *int {
	if pointer == nil {
		return nil
	}
	return intPointer(*pointer)
}

func derefInt(pointer *int) int {
	if pointer == nil {
		return 0
	}
	return *pointer
}
