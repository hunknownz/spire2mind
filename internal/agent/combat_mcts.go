package agentruntime

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"spire2mind/internal/game"
	"spire2mind/internal/i18n"
)

const (
	defaultMCTSIterations   = 128
	defaultMCTSRolloutDepth = 5
	defaultMCTSExploration  = 1.15
)

type mctsCombatPlanner struct {
	iterations   int
	rolloutDepth int
	exploration  float64
}

type combatSearchState struct {
	Snapshot          CombatSnapshot
	UtilityBonus      float64
	DrawCredit        float64
	ExhaustedBadCards int
	TurnEnded         bool
}

type combatSearchNode struct {
	parent     *combatSearchNode
	action     *CombatAction
	state      combatSearchState
	unexpanded []CombatAction
	children   []*combatSearchNode
	visits     int
	totalValue float64
	maxValue   float64
	depth      int
	terminal   bool
}

type combatSearchResult struct {
	Iterations int
	Candidates []combatSearchCandidate
}

type combatSearchCandidate struct {
	Action    CombatAction
	Visits    int
	MeanValue float64
	MaxValue  float64
}

type estimatedCardEffect struct {
	Damage          int
	Block           int
	Draw            int
	ApplyVulnerable int
	Utility         float64
	ExhaustBadCard  bool
	TargetsAll      bool
	RandomHits      int
}

type combatSurvivalProfile struct {
	Label                 string
	RoutePressure         string
	UnblockedDamageWeight float64
	CoveredDamageWeight   float64
	WastedBlockWeight     float64
	ThreatReductionWeight float64
	DamageWeight          float64
	KillWeight            float64
	LethalPenalty         float64
	NearLethalPenalty     float64
}

type combatRoutePressure struct {
	Label                  string
	UnblockedDamageDelta   float64
	CoveredDamageDelta     float64
	WastedBlockDelta       float64
	ThreatReductionDelta   float64
	DamageDelta            float64
	KillDelta              float64
	LethalPenaltyDelta     float64
	NearLethalPenaltyDelta float64
}

func newMCTSCombatPlanner() CombatPlanner {
	return mctsCombatPlanner{
		iterations:   defaultMCTSIterations,
		rolloutDepth: defaultMCTSRolloutDepth,
		exploration:  defaultMCTSExploration,
	}
}

func (m mctsCombatPlanner) Name() string { return "mcts" }

func (m mctsCombatPlanner) Analyze(state *game.StateSnapshot, codex *SeenContentRegistry, language i18n.Language) *CombatPlan {
	if state == nil {
		return nil
	}
	if isCombatSelectionState(state) {
		return analyzeCombatSelection(state, language, "mcts")
	}
	if !strings.EqualFold(state.Screen, "COMBAT") {
		return nil
	}

	snapshot := buildCombatSnapshot(state, codex)
	actions := generateCombatActions(snapshot)
	if len(actions) == 0 {
		return nil
	}

	loc := i18n.New(language)
	result := runCombatMCTS(snapshot, m.iterations, m.rolloutDepth, m.exploration)
	candidates := make([]CombatPlanCandidate, 0, min(3, len(result.Candidates)))
	for _, candidate := range result.Candidates {
		candidates = append(candidates, combatPlanCandidateFromAction(snapshot, candidate.Action, candidate.MeanValue))
		if len(candidates) >= 3 {
			break
		}
	}

	primaryGoal := loc.Label("Search for the strongest short combat line this turn", "优先搜索本回合最强的短线战斗序列")
	if snapshot.IncomingDamage > snapshot.Player.Block {
		primaryGoal = loc.Label("Use search to survive this turn first, then convert tempo", "优先用搜索保证本回合存活，再争取节奏转换")
	}

	summary := fmt.Sprintf(
		"%s `%d`, %s `%d`, %s `%d`",
		loc.Label("energy", "能量"),
		snapshot.Player.Energy,
		loc.Label("branching actions", "候选动作"),
		len(actions),
		loc.Label("simulations", "搜索次数"),
		result.Iterations,
	)

	reasons := []string{
		loc.Label(
			fmt.Sprintf("Searched %d shallow combat rollouts before suggesting a line.", result.Iterations),
			fmt.Sprintf("在给出建议前，已完成 %d 次浅层战斗搜索。", result.Iterations),
		),
	}
	profile := combatSurvivalProfileFor(snapshot)
	reasons = append(reasons, loc.Label(
		fmt.Sprintf("Current survival posture: %s.", profile.Label),
		fmt.Sprintf("当前生存姿态：%s。", profile.Label),
	))
	if strings.TrimSpace(profile.RoutePressure) != "" && !strings.EqualFold(profile.RoutePressure, "none") {
		reasons = append(reasons, loc.Label(
			fmt.Sprintf("Current route pressure: %s.", profile.RoutePressure),
			fmt.Sprintf("当前路线压力：%s。", profile.RoutePressure),
		))
	}
	if len(result.Candidates) > 0 {
		best := result.Candidates[0]
		reasons = append(reasons, loc.Label(
			fmt.Sprintf("Best first action: %s (mean %.2f over %d visits).", best.Action.Label, best.MeanValue, best.Visits),
			fmt.Sprintf("当前最优首步：%s（平均分 %.2f，访问 %d 次）。", best.Action.Label, best.MeanValue, best.Visits),
		))
	}
	if snapshot.LowestEnemyLabel != "" {
		reasons = append(reasons, loc.Label(
			fmt.Sprintf("Current low-HP focus target: %s (%d HP).", snapshot.LowestEnemyLabel, snapshot.LowestEnemyHP),
			fmt.Sprintf("当前低血优先目标：%s（%d HP）。", snapshot.LowestEnemyLabel, snapshot.LowestEnemyHP),
		))
	}
	for _, cue := range snapshot.KnowledgeBiases {
		reasons = append(reasons, loc.Label(cue, cue))
	}

	return &CombatPlan{
		Mode:         "mcts",
		Summary:      summary,
		PrimaryGoal:  primaryGoal,
		TargetLabel:  snapshot.LowestEnemyLabel,
		FocusReasons: reasons,
		Candidates:   candidates,
	}
}

func runCombatMCTS(snapshot CombatSnapshot, iterations int, rolloutDepth int, exploration float64) combatSearchResult {
	root := &combatSearchNode{
		state:      combatSearchState{Snapshot: cloneCombatSnapshot(snapshot)},
		unexpanded: orderedCombatActions(snapshot),
		maxValue:   math.Inf(-1),
	}
	if len(root.unexpanded) == 0 {
		root.terminal = true
	}

	for step := 0; step < iterations; step++ {
		node := root
		for len(node.unexpanded) == 0 && len(node.children) > 0 && !node.terminal {
			node = selectUCTChild(node, exploration)
		}

		if !node.terminal && len(node.unexpanded) > 0 {
			action := node.unexpanded[0]
			node.unexpanded = node.unexpanded[1:]
			nextState := simulateCombatAction(node.state, action)
			child := &combatSearchNode{
				parent:     node,
				action:     &action,
				state:      nextState,
				unexpanded: orderedCombatActions(nextState.Snapshot),
				depth:      node.depth + 1,
				maxValue:   math.Inf(-1),
			}
			if child.depth >= rolloutDepth || child.state.TurnEnded || len(child.unexpanded) == 0 {
				child.terminal = true
			}
			node.children = append(node.children, child)
			node = child
		}

		value := rolloutCombatState(snapshot, node.state, rolloutDepth-node.depth)
		for current := node; current != nil; current = current.parent {
			current.visits++
			current.totalValue += value
			if value > current.maxValue {
				current.maxValue = value
			}
		}
	}

	candidates := make([]combatSearchCandidate, 0, len(root.children))
	for _, child := range root.children {
		mean := child.totalValue
		if child.visits > 0 {
			mean /= float64(child.visits)
		}
		mean += rootActionTempoBias(snapshot, derefCombatAction(child.action))
		candidates = append(candidates, combatSearchCandidate{
			Action:    derefCombatAction(child.action),
			Visits:    child.visits,
			MeanValue: mean,
			MaxValue:  child.maxValue,
		})
	}
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].MeanValue == candidates[j].MeanValue {
			if candidates[i].Visits == candidates[j].Visits {
				return candidates[i].Action.Label < candidates[j].Action.Label
			}
			return candidates[i].Visits > candidates[j].Visits
		}
		return candidates[i].MeanValue > candidates[j].MeanValue
	})

	return combatSearchResult{
		Iterations: iterations,
		Candidates: candidates,
	}
}

func rootActionTempoBias(snapshot CombatSnapshot, action CombatAction) float64 {
	if action.Request.Action == "end_turn" {
		if snapshot.IncomingDamage == 0 && countPlayableCards(snapshot) > 0 {
			return -2.5
		}
		return 0
	}
	if !isLowPressureCombatTurn(snapshot) {
		return 0
	}
	cardIndex := derefInt(action.Request.CardIndex)
	card, found := combatCardByIndex(snapshot, cardIndex)
	if !found {
		return 0
	}
	effect := estimateCardEffect(card, snapshot.Player.Energy)
	bias := 0.0
	if effect.Damage > 0 {
		bias += 2.0 + float64(effect.Damage)*0.1
	}
	if effect.ApplyVulnerable > 0 {
		bias += 0.5
	}
	if effect.Block > 0 && effect.Damage == 0 {
		bias -= 2.4
	}
	return bias
}

func orderedCombatActions(snapshot CombatSnapshot) []CombatAction {
	actions := generateCombatActions(snapshot)
	sort.SliceStable(actions, func(i, j int) bool {
		left := scoreMCTSActionPriority(snapshot, actions[i])
		right := scoreMCTSActionPriority(snapshot, actions[j])
		if left == right {
			return actions[i].Label < actions[j].Label
		}
		return left > right
	})
	return actions
}

func scoreMCTSActionPriority(snapshot CombatSnapshot, action CombatAction) float64 {
	base := scoreCombatAction(snapshot, action)
	next := simulateCombatAction(combatSearchState{
		Snapshot: cloneCombatSnapshot(snapshot),
	}, action)
	return evaluateCombatSearchState(snapshot, next) + base*0.35
}

func selectUCTChild(node *combatSearchNode, exploration float64) *combatSearchNode {
	if len(node.children) == 0 {
		return node
	}

	best := node.children[0]
	bestScore := math.Inf(-1)
	parentVisits := math.Max(1, float64(node.visits))
	for _, child := range node.children {
		if child.visits == 0 {
			return child
		}
		mean := child.totalValue / float64(child.visits)
		score := mean + exploration*math.Sqrt(math.Log(parentVisits)/float64(child.visits))
		if score > bestScore {
			best = child
			bestScore = score
		}
	}
	return best
}

func rolloutCombatState(initial CombatSnapshot, state combatSearchState, remainingDepth int) float64 {
	current := cloneCombatSearchState(state)
	for step := 0; step < remainingDepth && !current.TurnEnded; step++ {
		actions := orderedCombatActions(current.Snapshot)
		if len(actions) == 0 {
			break
		}

		best := actions[0]
		if best.Request.Action == "end_turn" && len(actions) > 1 {
			best = actions[1]
		}
		current = simulateCombatAction(current, best)
	}

	return evaluateCombatSearchState(initial, current)
}

func simulateCombatAction(state combatSearchState, action CombatAction) combatSearchState {
	next := cloneCombatSearchState(state)
	switch action.Request.Action {
	case "end_turn":
		next.TurnEnded = true
		next.Snapshot.CanPlayCard = false
		next.Snapshot.CanEndTurn = false
		return next
	case "play_card":
		if action.Request.CardIndex == nil {
			return next
		}
		card, found := combatCardByIndex(next.Snapshot, *action.Request.CardIndex)
		if !found {
			return next
		}
		effect := estimateCardEffect(card, next.Snapshot.Player.Energy)
		next.Snapshot.Player.Energy = max(0, next.Snapshot.Player.Energy-card.EnergyCost)
		removeCombatCard(&next.Snapshot, card.Index)

		// Apply Strength to damage, Weak reduces damage by 25%
		if effect.Damage > 0 {
			effect.Damage += next.Snapshot.Player.Strength
			if next.Snapshot.Player.Weak > 0 {
				effect.Damage = int(float64(effect.Damage) * 0.75)
			}
			effect.Damage = max(0, effect.Damage)
		}

		// Apply Dexterity to block, Frail reduces block by 25%
		if effect.Block > 0 {
			effect.Block += next.Snapshot.Player.Dexterity
			if next.Snapshot.Player.Frail > 0 {
				effect.Block = int(float64(effect.Block) * 0.75)
			}
			effect.Block = max(0, effect.Block)
		}

		next.Snapshot.Player.Block += effect.Block
		next.UtilityBonus += effect.Utility
		next.DrawCredit += float64(effect.Draw) * 0.85
		if effect.ExhaustBadCard && exhaustWorstCombatCard(&next.Snapshot) {
			next.ExhaustedBadCards++
			next.UtilityBonus += 0.6
		}

		if effect.ApplyVulnerable > 0 && action.Request.TargetIndex != nil {
			applyVulnerable(&next.Snapshot, *action.Request.TargetIndex, effect.ApplyVulnerable)
		}

		switch {
		case effect.RandomHits > 0:
			applyRandomHitStyleDamage(&next.Snapshot, effect.RandomHits, effect.Damage)
		case effect.TargetsAll:
			for _, enemy := range next.Snapshot.Enemies {
				applyDamageToEnemy(&next.Snapshot, enemy.Index, effect.Damage)
			}
		case effect.Damage > 0 && action.Request.TargetIndex != nil:
			applyDamageToEnemy(&next.Snapshot, *action.Request.TargetIndex, effect.Damage)
		case effect.Damage > 0:
			targetIndex, ok := lowestHPEnemyIndex(next.Snapshot)
			if ok {
				applyDamageToEnemy(&next.Snapshot, targetIndex, effect.Damage)
			}
		}

		recomputeCombatSnapshot(&next.Snapshot)
		if len(generateCombatActions(next.Snapshot)) == 0 {
			next.TurnEnded = true
		}
		return next
	default:
		return next
	}
}

func estimateCardEffect(card CombatCardState, availableEnergy int) estimatedCardEffect {
	key := strings.ToUpper(strings.TrimSpace(card.CardID))
	name := strings.ToLower(strings.TrimSpace(card.Name))
	switch {
	case strings.Contains(key, "STRIKE"):
		return estimatedCardEffect{Damage: 6}
	case strings.Contains(key, "DEFEND"):
		return estimatedCardEffect{Block: 5, Utility: 0.2}
	case strings.Contains(key, "BASH"):
		return estimatedCardEffect{Damage: 8, ApplyVulnerable: 2, Utility: 0.8}
	case strings.Contains(key, "WHIRLWIND"):
		hits := max(1, availableEnergy)
		return estimatedCardEffect{Damage: 5 * hits, TargetsAll: true, Utility: 1.1}
	case strings.Contains(key, "IRON_WAVE"):
		return estimatedCardEffect{Damage: 5, Block: 5, Utility: 0.8}
	case strings.Contains(key, "SHRUG_IT_OFF"):
		return estimatedCardEffect{Block: 8, Draw: 1, Utility: 0.9}
	case strings.Contains(key, "POMMEL_STRIKE"):
		return estimatedCardEffect{Damage: 9, Draw: 1, Utility: 0.9}
	case strings.Contains(key, "CLOTHESLINE"):
		return estimatedCardEffect{Damage: 12, ApplyVulnerable: 2, Utility: 1.0}
	case strings.Contains(key, "HEADBUTT"):
		return estimatedCardEffect{Damage: 9, Utility: 1.0}
	case strings.Contains(key, "PERFECTED_STRIKE"):
		return estimatedCardEffect{Damage: 14, Utility: 0.9}
	case strings.Contains(key, "CLEAVE"):
		return estimatedCardEffect{Damage: 8, TargetsAll: true, Utility: 0.9}
	case strings.Contains(key, "THUNDERCLAP"):
		return estimatedCardEffect{Damage: 4, TargetsAll: true, ApplyVulnerable: 1, Utility: 1.0}
	case strings.Contains(key, "TWIN_STRIKE"):
		return estimatedCardEffect{Damage: 10, Utility: 0.7}
	case strings.Contains(key, "ANGER"):
		return estimatedCardEffect{Damage: 6, Utility: 0.75}
	case strings.Contains(key, "TRUE_GRIT"):
		return estimatedCardEffect{Block: 7, ExhaustBadCard: true, Utility: 0.7}
	case strings.Contains(key, "ARMAMENTS"):
		return estimatedCardEffect{Block: 5, Utility: 1.0}
	case strings.Contains(key, "SWORD_BOOMERANG"):
		return estimatedCardEffect{Damage: 3, RandomHits: 3, Utility: 0.6}
	case strings.Contains(key, "BURNING_PACT"):
		return estimatedCardEffect{Draw: 2, ExhaustBadCard: true, Utility: 1.2}
	case strings.Contains(key, "SLIMED"):
		return estimatedCardEffect{Utility: 0.3}
	case strings.Contains(key, "WOUND"), strings.Contains(key, "DAZED"), strings.Contains(key, "BURN"), strings.Contains(key, "VOID"):
		return estimatedCardEffect{Utility: 0.1}
	case strings.Contains(key, "INFLAME"), strings.Contains(key, "DEMON_FORM"):
		return estimatedCardEffect{Utility: 3.5}
	case strings.Contains(key, "METALLICIZE"):
		return estimatedCardEffect{Block: 3, Utility: 2.5}
	case strings.Contains(key, "BLUDGEON"):
		return estimatedCardEffect{Damage: 32, Utility: 0.9}
	case strings.Contains(key, "BLOOD_FOR_BLOOD"):
		return estimatedCardEffect{Damage: 18, Utility: 0.8}
	case strings.Contains(key, "UPPERCUT"):
		return estimatedCardEffect{Damage: 13, ApplyVulnerable: 1, Utility: 1.1}
	case strings.Contains(key, "BODY_SLAM"):
		return estimatedCardEffect{Utility: 1.0}
	case strings.Contains(key, "HAVOC"):
		return estimatedCardEffect{Utility: 0.8}
	case strings.Contains(key, "OFFERING"):
		return estimatedCardEffect{Draw: 3, Utility: 1.5}
	case strings.Contains(key, "BATTLE_TRANCE"):
		return estimatedCardEffect{Draw: 3, Utility: 1.2}
	case strings.Contains(key, "FLEX"):
		return estimatedCardEffect{Utility: 0.6}
	case strings.Contains(key, "DISARM"):
		return estimatedCardEffect{Utility: 1.5}
	case strings.Contains(key, "DARK_SHACKLES"):
		return estimatedCardEffect{Utility: 1.3}
	case strings.Contains(key, "GHOSTLY_ARMOR"), strings.Contains(key, "IMPERVIOUS"):
		return estimatedCardEffect{Block: 13, Utility: 1.0}
	case strings.Contains(key, "SENTINEL"):
		return estimatedCardEffect{Block: 5, Utility: 0.9}
	case strings.Contains(name, "defend"), strings.Contains(name, "block"),
		strings.Contains(name, "防御"), strings.Contains(name, "格挡"):
		return estimatedCardEffect{Block: 5}
	case card.RequiresTarget:
		// Unknown targeted card: estimate damage from cost
		damage := 6
		if card.EnergyCost > 1 {
			damage += (card.EnergyCost - 1) * 4
		}
		return estimatedCardEffect{Damage: damage}
	default:
		// Unknown untargeted card: might be skill or power
		if card.EnergyCost >= 2 {
			return estimatedCardEffect{Block: 3, Utility: 1.0}
		}
		return estimatedCardEffect{Utility: 0.5}
	}
}

func evaluateCombatSearchState(initial CombatSnapshot, state combatSearchState) float64 {
	profile := combatSurvivalProfileFor(initial)
	initialEnemyHP := totalEnemyHP(initial)
	remainingEnemyHP := totalEnemyHP(state.Snapshot)
	initialEnemies := len(initial.Enemies)
	remainingEnemies := len(state.Snapshot.Enemies)
	damageDealt := max(0, initialEnemyHP-remainingEnemyHP)
	kills := max(0, initialEnemies-remainingEnemies)
	incoming := state.Snapshot.IncomingDamage
	covered := min(incoming, state.Snapshot.Player.Block)
	unblocked := max(0, incoming-state.Snapshot.Player.Block)
	wastedBlock := max(0, state.Snapshot.Player.Block-incoming)
	lethalMargin := state.Snapshot.Player.CurrentHP - unblocked
	hpRatio := combatPlayerHPRatio(state.Snapshot)

	score := 0.0
	score += float64(damageDealt) * profile.DamageWeight
	score += float64(kills) * profile.KillWeight
	score += float64(covered) * profile.CoveredDamageWeight
	score -= float64(unblocked) * profile.UnblockedDamageWeight
	score += state.UtilityBonus
	score += state.DrawCredit
	score += float64(state.ExhaustedBadCards) * 0.8
	score += combatKnowledgeThreatReduction(initial, state) * profile.ThreatReductionWeight

	switch {
	case hpRatio <= 0.2:
		score += float64(covered) * 0.9
		score -= float64(unblocked) * 1.25
	case hpRatio <= 0.35:
		score += float64(covered) * 0.55
		score -= float64(unblocked) * 0.7
	}
	if incoming == 0 {
		score -= float64(state.Snapshot.Player.Block) * 0.45
	} else {
		score -= float64(wastedBlock) * profile.WastedBlockWeight
	}

	if remainingEnemies == 0 {
		score += 100.0
	}
	if lethalMargin <= 0 {
		score -= profile.LethalPenalty
	} else if lethalMargin <= 5 {
		score -= profile.NearLethalPenalty
	}
	if state.TurnEnded && countPlayableCards(state.Snapshot) > 0 {
		score -= 2.5
	}
	score -= float64(max(0, state.Snapshot.Player.Energy)) * 0.15

	return score
}

func combatSurvivalProfileFor(snapshot CombatSnapshot) combatSurvivalProfile {
	hpRatio := combatPlayerHPRatio(snapshot)
	profile := combatSurvivalProfile{
		Label:                 "balanced",
		RoutePressure:         "none",
		UnblockedDamageWeight: 1.65,
		CoveredDamageWeight:   0.75,
		WastedBlockWeight:     0.22,
		ThreatReductionWeight: 0.45,
		DamageWeight:          1.05,
		KillWeight:            14.0,
		LethalPenalty:         120.0,
		NearLethalPenalty:     10.0,
	}

	switch {
	case hpRatio <= 0.25:
		profile.Label = "critical-hp preservation"
		profile.UnblockedDamageWeight = 3.0
		profile.CoveredDamageWeight = 1.15
		profile.WastedBlockWeight = 0.08
		profile.ThreatReductionWeight = 0.8
		profile.DamageWeight = 0.95
		profile.KillWeight = 15.0
		profile.LethalPenalty = 190.0
		profile.NearLethalPenalty = 18.0
	case snapshot.Floor > 0 && snapshot.Floor <= 8 && hpRatio <= 0.55:
		profile.Label = "early-floor hp preservation"
		profile.UnblockedDamageWeight = 2.45
		profile.CoveredDamageWeight = 0.95
		profile.WastedBlockWeight = 0.12
		profile.ThreatReductionWeight = 0.65
		profile.DamageWeight = 1.0
		profile.KillWeight = 14.5
		profile.LethalPenalty = 150.0
		profile.NearLethalPenalty = 14.0
	case snapshot.Floor > 0 && snapshot.Floor <= 4 && hpRatio >= 0.75:
		profile.Label = "healthy early aggression"
		profile.UnblockedDamageWeight = 1.3
		profile.CoveredDamageWeight = 0.55
		profile.WastedBlockWeight = 0.34
		profile.ThreatReductionWeight = 0.38
		profile.DamageWeight = 1.15
		profile.KillWeight = 15.0
		profile.LethalPenalty = 115.0
		profile.NearLethalPenalty = 8.0
	case hpRatio <= 0.45:
		profile.Label = "mid-fight stabilization"
		profile.UnblockedDamageWeight = 2.15
		profile.CoveredDamageWeight = 0.9
		profile.WastedBlockWeight = 0.14
		profile.ThreatReductionWeight = 0.58
		profile.DamageWeight = 1.0
		profile.KillWeight = 14.0
		profile.LethalPenalty = 145.0
		profile.NearLethalPenalty = 12.0
	}

	routePressure := combatRoutePressureFor(snapshot)
	profile.RoutePressure = routePressure.Label
	profile.UnblockedDamageWeight += routePressure.UnblockedDamageDelta
	profile.CoveredDamageWeight += routePressure.CoveredDamageDelta
	profile.WastedBlockWeight += routePressure.WastedBlockDelta
	profile.ThreatReductionWeight += routePressure.ThreatReductionDelta
	profile.DamageWeight += routePressure.DamageDelta
	profile.KillWeight += routePressure.KillDelta
	profile.LethalPenalty += routePressure.LethalPenaltyDelta
	profile.NearLethalPenalty += routePressure.NearLethalPenaltyDelta
	profile.UnblockedDamageWeight = maxFloat(0.5, profile.UnblockedDamageWeight)
	profile.CoveredDamageWeight = maxFloat(0.2, profile.CoveredDamageWeight)
	profile.WastedBlockWeight = maxFloat(0.02, profile.WastedBlockWeight)
	profile.ThreatReductionWeight = maxFloat(0.1, profile.ThreatReductionWeight)
	profile.DamageWeight = maxFloat(0.4, profile.DamageWeight)
	profile.KillWeight = maxFloat(6.0, profile.KillWeight)
	profile.LethalPenalty = maxFloat(60.0, profile.LethalPenalty)
	profile.NearLethalPenalty = maxFloat(4.0, profile.NearLethalPenalty)

	return profile
}

func combatRoutePressureFor(snapshot CombatSnapshot) combatRoutePressure {
	hpRatio := combatPlayerHPRatio(snapshot)

	switch {
	case snapshot.Floor > 0 && snapshot.Floor <= 10 && snapshot.Gold >= 120 && hpRatio <= 0.65:
		return combatRoutePressure{
			Label:                  "protect the shop/conversion window",
			UnblockedDamageDelta:   0.55,
			CoveredDamageDelta:     0.18,
			WastedBlockDelta:       -0.08,
			ThreatReductionDelta:   0.12,
			DamageDelta:            -0.05,
			KillDelta:              0.6,
			LethalPenaltyDelta:     18.0,
			NearLethalPenaltyDelta: 3.0,
		}
	case snapshot.Floor > 0 && snapshot.Floor <= 5 && hpRatio >= 0.78 && snapshot.Gold < 90:
		return combatRoutePressure{
			Label:                  "push early snowball tempo",
			UnblockedDamageDelta:   -0.18,
			CoveredDamageDelta:     -0.08,
			WastedBlockDelta:       0.10,
			ThreatReductionDelta:   0.06,
			DamageDelta:            0.12,
			KillDelta:              1.0,
			LethalPenaltyDelta:     -4.0,
			NearLethalPenaltyDelta: -1.0,
		}
	case snapshot.Floor > 0 && snapshot.Floor <= 17 && hpRatio <= 0.45:
		return combatRoutePressure{
			Label:                  "stabilize before stronger rooms",
			UnblockedDamageDelta:   0.32,
			CoveredDamageDelta:     0.12,
			WastedBlockDelta:       -0.04,
			ThreatReductionDelta:   0.10,
			KillDelta:              0.4,
			LethalPenaltyDelta:     12.0,
			NearLethalPenaltyDelta: 2.0,
		}
	default:
		return combatRoutePressure{Label: "none"}
	}
}

func cloneCombatSearchState(state combatSearchState) combatSearchState {
	return combatSearchState{
		Snapshot:          cloneCombatSnapshot(state.Snapshot),
		UtilityBonus:      state.UtilityBonus,
		DrawCredit:        state.DrawCredit,
		ExhaustedBadCards: state.ExhaustedBadCards,
		TurnEnded:         state.TurnEnded,
	}
}

func cloneCombatSnapshot(snapshot CombatSnapshot) CombatSnapshot {
	clone := snapshot
	clone.Hand = append([]CombatCardState(nil), snapshot.Hand...)
	clone.Enemies = append([]CombatEnemyState(nil), snapshot.Enemies...)
	clone.KnowledgeBiases = append([]string(nil), snapshot.KnowledgeBiases...)
	for i := range clone.Hand {
		clone.Hand[i].ValidTargets = append([]int(nil), snapshot.Hand[i].ValidTargets...)
	}
	for i := range clone.Enemies {
		clone.Enemies[i].Intents = append([]CombatIntentState(nil), snapshot.Enemies[i].Intents...)
	}
	return clone
}

func combatKnowledgeThreatReduction(initial CombatSnapshot, state combatSearchState) float64 {
	initialThreat := combatKnowledgeThreatScore(initial)
	remainingThreat := combatKnowledgeThreatScore(state.Snapshot)
	return maxFloat(0, initialThreat-remainingThreat)
}

func combatKnowledgeThreatScore(snapshot CombatSnapshot) float64 {
	score := 0.0
	for _, enemy := range snapshot.Enemies {
		if enemy.CurrentHP <= 0 {
			continue
		}
		score += enemy.KnowledgePrior * (1.0 + float64(enemy.CurrentHP)/20.0)
	}
	return score
}

func maxFloat(left float64, right float64) float64 {
	if left > right {
		return left
	}
	return right
}

func derefCombatAction(action *CombatAction) CombatAction {
	if action == nil {
		return CombatAction{}
	}
	return *action
}

func combatCardByIndex(snapshot CombatSnapshot, index int) (CombatCardState, bool) {
	for _, card := range snapshot.Hand {
		if card.Index == index {
			return card, true
		}
	}
	return CombatCardState{}, false
}

func removeCombatCard(snapshot *CombatSnapshot, index int) {
	if snapshot == nil {
		return
	}
	filtered := snapshot.Hand[:0]
	removed := false
	for _, card := range snapshot.Hand {
		if !removed && card.Index == index {
			removed = true
			continue
		}
		filtered = append(filtered, card)
	}
	snapshot.Hand = append([]CombatCardState(nil), filtered...)
}

func applyVulnerable(snapshot *CombatSnapshot, targetIndex int, amount int) {
	if snapshot == nil || amount <= 0 {
		return
	}
	for i := range snapshot.Enemies {
		if snapshot.Enemies[i].Index == targetIndex {
			snapshot.Enemies[i].Vulnerable += amount
			return
		}
	}
}

func applyRandomHitStyleDamage(snapshot *CombatSnapshot, hits int, damage int) {
	if snapshot == nil || hits <= 0 || damage <= 0 {
		return
	}
	for step := 0; step < hits; step++ {
		targetIndex, ok := lowestHPEnemyIndex(*snapshot)
		if !ok {
			return
		}
		applyDamageToEnemy(snapshot, targetIndex, damage)
	}
}

func applyDamageToEnemy(snapshot *CombatSnapshot, targetIndex int, baseDamage int) {
	if snapshot == nil || baseDamage <= 0 {
		return
	}
	for i := range snapshot.Enemies {
		enemy := &snapshot.Enemies[i]
		if enemy.Index != targetIndex {
			continue
		}
		damage := baseDamage
		if enemy.Vulnerable > 0 {
			damage = int(math.Ceil(float64(damage) * 1.5))
		}
		if enemy.Block > 0 {
			absorbed := min(enemy.Block, damage)
			enemy.Block -= absorbed
			damage -= absorbed
		}
		if damage > 0 {
			enemy.CurrentHP = max(0, enemy.CurrentHP-damage)
		}
		if enemy.Vulnerable > 0 {
			enemy.Vulnerable = max(0, enemy.Vulnerable-1)
		}
		return
	}
}

func exhaustWorstCombatCard(snapshot *CombatSnapshot) bool {
	if snapshot == nil || len(snapshot.Hand) == 0 {
		return false
	}
	bestIndex := -1
	bestScore := math.Inf(-1)
	for i, card := range snapshot.Hand {
		score := badCardScore(card)
		if score > bestScore {
			bestScore = score
			bestIndex = i
		}
	}
	if bestIndex < 0 {
		return false
	}
	index := snapshot.Hand[bestIndex].Index
	removeCombatCard(snapshot, index)
	return true
}

func badCardScore(card CombatCardState) float64 {
	key := strings.ToUpper(strings.TrimSpace(card.CardID))
	score := 0.0
	switch {
	case strings.Contains(key, "SLIMED"), strings.Contains(key, "WOUND"), strings.Contains(key, "DAZED"), strings.Contains(key, "BURN"), strings.Contains(key, "VOID"):
		score += 8.0
	}
	if !card.Playable {
		score += 4.0
	}
	score += float64(card.EnergyCost) * 0.4
	if !card.RequiresTarget && card.EnergyCost > 0 && card.CardID == "" {
		score += 0.6
	}
	return score
}

func recomputeCombatSnapshot(snapshot *CombatSnapshot) {
	if snapshot == nil {
		return
	}
	living := snapshot.Enemies[:0]
	snapshot.IncomingDamage = 0
	snapshot.LowestEnemyHP = 0
	snapshot.LowestEnemyLabel = ""
	for _, enemy := range snapshot.Enemies {
		if enemy.CurrentHP <= 0 || !enemy.Hittable {
			continue
		}
		if enemy.CurrentHP > 0 && (snapshot.LowestEnemyHP == 0 || enemy.CurrentHP < snapshot.LowestEnemyHP) {
			snapshot.LowestEnemyHP = enemy.CurrentHP
			snapshot.LowestEnemyLabel = fallbackID(enemy.Name, enemy.EnemyID)
		}
		for _, intent := range enemy.Intents {
			snapshot.IncomingDamage += intent.TotalDamage
		}
		living = append(living, enemy)
	}
	snapshot.Enemies = append([]CombatEnemyState(nil), living...)
	snapshot.CanPlayCard = countPlayableCards(*snapshot) > 0
	snapshot.CanEndTurn = true
	for i := range snapshot.Hand {
		if snapshot.Hand[i].RequiresTarget {
			snapshot.Hand[i].ValidTargets = livingEnemyIndices(*snapshot)
		}
	}
}

func lowestHPEnemyIndex(snapshot CombatSnapshot) (int, bool) {
	bestIndex := 0
	bestHP := 0
	found := false
	for _, enemy := range snapshot.Enemies {
		if !enemy.Hittable || enemy.CurrentHP <= 0 {
			continue
		}
		if !found || enemy.CurrentHP < bestHP {
			bestIndex = enemy.Index
			bestHP = enemy.CurrentHP
			found = true
		}
	}
	return bestIndex, found
}

func livingEnemyIndices(snapshot CombatSnapshot) []int {
	indices := make([]int, 0, len(snapshot.Enemies))
	for _, enemy := range snapshot.Enemies {
		if enemy.Hittable && enemy.CurrentHP > 0 {
			indices = append(indices, enemy.Index)
		}
	}
	return indices
}

func countPlayableCards(snapshot CombatSnapshot) int {
	count := 0
	for _, card := range snapshot.Hand {
		if card.Playable && card.EnergyCost <= snapshot.Player.Energy {
			count++
		}
	}
	return count
}

func totalEnemyHP(snapshot CombatSnapshot) int {
	total := 0
	for _, enemy := range snapshot.Enemies {
		if enemy.CurrentHP > 0 {
			total += enemy.CurrentHP
		}
	}
	return total
}
