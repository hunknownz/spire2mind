package agentruntime

import (
	"strings"
	"testing"

	"spire2mind/internal/config"
	"spire2mind/internal/game"
	"spire2mind/internal/i18n"
)

func TestHeuristicCombatPlannerProducesPlanForCombat(t *testing.T) {
	planner := NewCombatPlanner(config.Config{CombatPlanner: "heuristic"})
	state := &game.StateSnapshot{
		Screen: "COMBAT",
		Combat: map[string]any{
			"player": map[string]any{
				"currentHp": 20,
				"block":     3,
				"energy":    2,
			},
			"hand": []any{
				map[string]any{
					"index":          0,
					"cardId":         "DEFEND",
					"name":           "Defend",
					"playable":       true,
					"energyCost":     1,
					"requiresTarget": false,
				},
				map[string]any{
					"index":          1,
					"cardId":         "ZAP",
					"name":           "Zap",
					"playable":       true,
					"energyCost":     0,
					"requiresTarget": false,
				},
			},
			"enemies": []any{
				map[string]any{
					"index":     0,
					"enemyId":   "SLIME",
					"name":      "Slime",
					"currentHp": 7,
					"intents": []any{
						map[string]any{"intentType": "ATTACK", "totalDamage": 8},
					},
				},
			},
		},
	}

	plan := planner.Analyze(state, nil, i18n.LanguageEnglish)
	if plan == nil {
		t.Fatalf("expected combat plan")
	}
	if plan.Mode != "heuristic" {
		t.Fatalf("expected heuristic mode, got %q", plan.Mode)
	}
	if plan.PrimaryGoal == "" {
		t.Fatalf("expected non-empty primary goal")
	}
	if len(plan.FocusReasons) == 0 {
		t.Fatalf("expected focus reasons")
	}
}

func TestNoopCombatPlannerReturnsNil(t *testing.T) {
	planner := NewCombatPlanner(config.Config{CombatPlanner: "none"})
	if plan := planner.Analyze(&game.StateSnapshot{Screen: "COMBAT"}, nil, i18n.LanguageEnglish); plan != nil {
		t.Fatalf("expected nil plan for noop planner")
	}
}

func TestMCTSCombatPlannerPrefersLethalStrikeLine(t *testing.T) {
	planner := NewCombatPlanner(config.Config{CombatPlanner: "mcts"})
	state := &game.StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"player": map[string]any{
				"currentHp": 20,
				"block":     0,
				"energy":    1,
			},
			"hand": []any{
				map[string]any{
					"index":          0,
					"cardId":         "STRIKE_IRONCLAD",
					"name":           "Strike",
					"playable":       true,
					"energyCost":     1,
					"requiresTarget": true,
				},
				map[string]any{
					"index":          1,
					"cardId":         "DEFEND_IRONCLAD",
					"name":           "Defend",
					"playable":       true,
					"energyCost":     1,
					"requiresTarget": false,
				},
			},
			"enemies": []any{
				map[string]any{
					"index":      0,
					"enemyId":    "SLIME",
					"name":       "Slime",
					"currentHp":  6,
					"block":      0,
					"isHittable": true,
					"intents": []any{
						map[string]any{"intentType": "ATTACK", "totalDamage": 4},
					},
				},
			},
		},
	}

	plan := planner.Analyze(state, nil, i18n.LanguageEnglish)
	if plan == nil {
		t.Fatalf("expected combat plan")
	}
	if plan.Mode != "mcts" {
		t.Fatalf("expected mcts mode, got %q", plan.Mode)
	}
	if len(plan.Candidates) == 0 {
		t.Fatalf("expected ranked candidates")
	}
	if got := plan.Candidates[0].Label; !strings.Contains(got, "Strike") {
		t.Fatalf("expected lethal strike line first, got %q", got)
	}
}

func TestMCTSCombatPlannerPrefersDefenseWhenUnderLethalPressure(t *testing.T) {
	planner := NewCombatPlanner(config.Config{CombatPlanner: "mcts"})
	state := &game.StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"player": map[string]any{
				"currentHp": 5,
				"block":     0,
				"energy":    1,
			},
			"hand": []any{
				map[string]any{
					"index":          0,
					"cardId":         "STRIKE_IRONCLAD",
					"name":           "Strike",
					"playable":       true,
					"energyCost":     1,
					"requiresTarget": true,
				},
				map[string]any{
					"index":          1,
					"cardId":         "DEFEND_IRONCLAD",
					"name":           "Defend",
					"playable":       true,
					"energyCost":     1,
					"requiresTarget": false,
				},
			},
			"enemies": []any{
				map[string]any{
					"index":      0,
					"enemyId":    "SLIME",
					"name":       "Slime",
					"currentHp":  18,
					"block":      0,
					"isHittable": true,
					"intents": []any{
						map[string]any{"intentType": "ATTACK", "totalDamage": 8},
					},
				},
			},
		},
	}

	plan := planner.Analyze(state, nil, i18n.LanguageEnglish)
	if plan == nil {
		t.Fatalf("expected combat plan")
	}
	if len(plan.Candidates) == 0 {
		t.Fatalf("expected ranked candidates")
	}
	if got := plan.Candidates[0].Label; !strings.Contains(got, "Defend") {
		t.Fatalf("expected defensive line first, got %q", got)
	}
}

func TestMCTSCombatPlannerAvoidsOverblockingOnLowPressureTurn(t *testing.T) {
	planner := NewCombatPlanner(config.Config{CombatPlanner: "mcts"})
	state := &game.StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"player": map[string]any{
				"currentHp": 33,
				"maxHp":     80,
				"block":     0,
				"energy":    3,
			},
			"hand": []any{
				map[string]any{
					"index":          0,
					"cardId":         "BASH",
					"name":           "Bash",
					"playable":       true,
					"energyCost":     2,
					"requiresTarget": true,
				},
				map[string]any{
					"index":          1,
					"cardId":         "DEFEND_IRONCLAD",
					"name":           "Defend",
					"playable":       true,
					"energyCost":     1,
					"requiresTarget": false,
				},
				map[string]any{
					"index":          2,
					"cardId":         "SHRUG_IT_OFF",
					"name":           "Shrug It Off",
					"playable":       true,
					"energyCost":     1,
					"requiresTarget": false,
				},
				map[string]any{
					"index":          3,
					"cardId":         "STRIKE_IRONCLAD",
					"name":           "Strike",
					"playable":       true,
					"energyCost":     1,
					"requiresTarget": true,
				},
			},
			"enemies": []any{
				map[string]any{
					"index":      0,
					"enemyId":    "FUZZY_WURM_CRAWLER",
					"name":       "Fuzzy Wurm Crawler",
					"currentHp":  55,
					"block":      0,
					"isHittable": true,
					"intents": []any{
						map[string]any{"intentType": "Attack", "totalDamage": 4},
					},
				},
			},
		},
	}

	plan := planner.Analyze(state, nil, i18n.LanguageEnglish)
	if plan == nil {
		t.Fatalf("expected combat plan")
	}
	if len(plan.Candidates) == 0 {
		t.Fatalf("expected ranked candidates")
	}
	if got := plan.Candidates[0].Label; strings.Contains(got, "Defend") || strings.Contains(got, "Shrug") {
		t.Fatalf("expected offensive line first on low-pressure turn, got %q", got)
	}
}

func TestMCTSCombatPlannerPreservesShopWindowWhenFragile(t *testing.T) {
	planner := NewCombatPlanner(config.Config{CombatPlanner: "mcts"})
	state := &game.StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Run: map[string]any{
			"floor":     6,
			"gold":      150,
			"currentHp": 46,
			"maxHp":     80,
		},
		Combat: map[string]any{
			"player": map[string]any{
				"currentHp": 46,
				"maxHp":     80,
				"block":     0,
				"energy":    1,
			},
			"hand": []any{
				map[string]any{
					"index":          0,
					"cardId":         "STRIKE_IRONCLAD",
					"name":           "Strike",
					"playable":       true,
					"energyCost":     1,
					"requiresTarget": true,
				},
				map[string]any{
					"index":          1,
					"cardId":         "DEFEND_IRONCLAD",
					"name":           "Defend",
					"playable":       true,
					"energyCost":     1,
					"requiresTarget": false,
				},
			},
			"enemies": []any{
				map[string]any{
					"index":      0,
					"enemyId":    "SLIME",
					"name":       "Slime",
					"currentHp":  20,
					"block":      0,
					"isHittable": true,
					"intents": []any{
						map[string]any{"intentType": "ATTACK", "totalDamage": 8},
					},
				},
			},
		},
	}

	plan := planner.Analyze(state, nil, i18n.LanguageEnglish)
	if plan == nil {
		t.Fatalf("expected combat plan")
	}
	if len(plan.Candidates) == 0 {
		t.Fatalf("expected ranked candidates")
	}
	if got := plan.Candidates[0].Label; !strings.Contains(got, "Defend") {
		t.Fatalf("expected defensive line first when preserving a high-gold fragile run, got %q", got)
	}
}

func TestCombatSelectionPlannerPrefersExhaustingStatusCards(t *testing.T) {
	planner := NewCombatPlanner(config.Config{CombatPlanner: "heuristic"})
	state := &game.StateSnapshot{
		Screen:           "CARD_SELECTION",
		AvailableActions: []string{"select_deck_card"},
		Selection: map[string]any{
			"sourceScreen": "COMBAT",
			"sourceHint":   "combat:combat_hand_select:simpleselect",
			"prompt":       "[center]选择[blue]1[/blue]张牌来[gold]消耗[/gold]。[/center]",
			"cards": []any{
				map[string]any{"index": 0, "cardId": "STRIKE_IRONCLAD", "name": "Strike", "energyCost": 1},
				map[string]any{"index": 1, "cardId": "SLIMED", "name": "Slimed", "energyCost": 1},
				map[string]any{"index": 2, "cardId": "DEFEND_IRONCLAD", "name": "Defend", "energyCost": 1},
			},
		},
		Combat: map[string]any{
			"player": map[string]any{"currentHp": 24, "block": 0, "energy": 1},
			"enemies": []any{
				map[string]any{
					"index":      0,
					"enemyId":    "SLIME",
					"name":       "Slime",
					"currentHp":  14,
					"isHittable": true,
					"intents": []any{
						map[string]any{"intentType": "ATTACK", "totalDamage": 7},
					},
				},
			},
		},
	}

	plan := planner.Analyze(state, nil, i18n.LanguageEnglish)
	if plan == nil {
		t.Fatalf("expected combat selection plan")
	}
	if got := plan.Candidates[0].Label; !strings.Contains(got, "Slimed") {
		t.Fatalf("expected Slimed to be exhausted first, got %q", got)
	}
}

func TestCombatSelectionPlannerPrefersUpgradingHighImpactCard(t *testing.T) {
	planner := NewCombatPlanner(config.Config{CombatPlanner: "mcts"})
	state := &game.StateSnapshot{
		Screen:           "CARD_SELECTION",
		AvailableActions: []string{"select_deck_card"},
		Selection: map[string]any{
			"sourceScreen": "COMBAT",
			"sourceHint":   "combat:upgrade",
			"prompt":       "选择一张牌来升级",
			"cards": []any{
				map[string]any{"index": 0, "cardId": "STRIKE_IRONCLAD", "name": "Strike", "energyCost": 1},
				map[string]any{"index": 1, "cardId": "BASH", "name": "Bash", "energyCost": 2},
				map[string]any{"index": 2, "cardId": "DEFEND_IRONCLAD", "name": "Defend", "energyCost": 1},
			},
		},
		Combat: map[string]any{
			"player": map[string]any{"currentHp": 30, "block": 0, "energy": 2},
			"enemies": []any{
				map[string]any{
					"index":      0,
					"enemyId":    "SLIME",
					"name":       "Slime",
					"currentHp":  18,
					"isHittable": true,
					"intents": []any{
						map[string]any{"intentType": "ATTACK", "totalDamage": 8},
					},
				},
			},
		},
	}

	plan := planner.Analyze(state, nil, i18n.LanguageEnglish)
	if plan == nil {
		t.Fatalf("expected combat upgrade plan")
	}
	if got := plan.Candidates[0].Label; !strings.Contains(got, "Bash") {
		t.Fatalf("expected Bash to be the top upgrade target, got %q", got)
	}
}

func TestBuildCombatSnapshotAppliesCodexPriors(t *testing.T) {
	state := &game.StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"player": map[string]any{
				"currentHp": 10,
				"maxHp":     30,
				"block":     0,
				"energy":    1,
			},
			"hand": []any{
				map[string]any{
					"index":          0,
					"cardId":         "DEFEND_IRONCLAD",
					"name":           "Defend",
					"playable":       true,
					"energyCost":     1,
					"requiresTarget": false,
				},
			},
			"enemies": []any{
				map[string]any{
					"index":      0,
					"enemyId":    "SLIME",
					"name":       "Slime",
					"currentHp":  14,
					"block":      0,
					"isHittable": true,
					"intents": []any{
						map[string]any{"intentType": "ATTACK", "totalDamage": 8},
					},
				},
			},
		},
	}

	codex := &SeenContentRegistry{
		Cards: []SeenContentEntry{{
			Category:      seenCategoryCards,
			ID:            "defend_ironclad",
			Name:          "Defend",
			RiskTags:      []string{"survival_tool"},
			ResponseHints: []string{"Play safer at low health."},
		}},
		Monsters: []SeenContentEntry{{
			Category:     seenCategoryMonsters,
			ID:           "slime",
			Name:         "Slime",
			RiskTags:     []string{"combat_threat", "punishes_low_hp"},
			FailureLinks: []string{"Died at critical HP."},
		}},
	}

	snapshot := buildCombatSnapshot(state, codex)
	if len(snapshot.Hand) != 1 || snapshot.Hand[0].KnowledgePrior <= 0 {
		t.Fatalf("expected positive card knowledge prior, got %#v", snapshot.Hand)
	}
	if len(snapshot.Enemies) != 1 || snapshot.Enemies[0].KnowledgePrior <= 0 {
		t.Fatalf("expected positive enemy knowledge prior, got %#v", snapshot.Enemies)
	}
	if len(snapshot.KnowledgeBiases) == 0 {
		t.Fatalf("expected knowledge bias cues")
	}
}

func TestScoreCombatActionUsesKnowledgePrior(t *testing.T) {
	snapshot := CombatSnapshot{
		Player: CombatPlayerState{CurrentHP: 10, MaxHP: 30, Energy: 1},
		Hand: []CombatCardState{
			{Index: 0, CardID: "DEFEND_IRONCLAD", Name: "Defend", Playable: true, EnergyCost: 1, KnowledgePrior: 1.2},
			{Index: 1, CardID: "STRIKE_IRONCLAD", Name: "Strike", Playable: true, EnergyCost: 1},
		},
		Enemies: []CombatEnemyState{
			{Index: 0, EnemyID: "SLIME", Name: "Slime", CurrentHP: 12, Hittable: true},
		},
		CanPlayCard:    true,
		CanEndTurn:     true,
		IncomingDamage: 8,
	}

	defendScore := scoreCombatAction(snapshot, CombatAction{
		Request: game.ActionRequest{Action: "play_card", CardIndex: intPointer(0)},
		Label:   "play defend",
	})
	strikeScore := scoreCombatAction(snapshot, CombatAction{
		Request: game.ActionRequest{Action: "play_card", CardIndex: intPointer(1), TargetIndex: intPointer(0)},
		Label:   "play strike",
	})

	if defendScore <= strikeScore {
		t.Fatalf("expected knowledge prior to push defend above strike, got defend=%f strike=%f", defendScore, strikeScore)
	}
}

func TestCombatPlanCandidateIncludesLethalTradeEstimate(t *testing.T) {
	snapshot := CombatSnapshot{
		Player: CombatPlayerState{CurrentHP: 20, MaxHP: 20, Block: 0, Energy: 1},
		Hand: []CombatCardState{
			{Index: 0, CardID: "STRIKE_IRONCLAD", Name: "Strike", Playable: true, EnergyCost: 1, RequiresTarget: true, ValidTargets: []int{0}},
		},
		Enemies: []CombatEnemyState{
			{
				Index:     0,
				EnemyID:   "SLIME",
				Name:      "Slime",
				CurrentHP: 6,
				Hittable:  true,
				Intents:   []CombatIntentState{{IntentType: "ATTACK", TotalDamage: 4}},
			},
		},
		CanPlayCard:      true,
		CanEndTurn:       true,
		IncomingDamage:   4,
		LowestEnemyLabel: "Slime",
		LowestEnemyHP:    6,
	}

	candidate := combatPlanCandidateFromAction(snapshot, CombatAction{
		Request: game.ActionRequest{Action: "play_card", CardIndex: intPointer(0), TargetIndex: intPointer(0)},
		Label:   "play [0] Strike -> [0] Slime",
	}, 12.3)

	if candidate.TradeEstimate == nil {
		t.Fatalf("expected trade estimate")
	}
	if candidate.TradeEstimate.Kills != 1 {
		t.Fatalf("expected 1 kill, got %+v", candidate.TradeEstimate)
	}
	if candidate.TradeEstimate.PredictedHPLoss != 0 {
		t.Fatalf("expected zero hp loss after lethal, got %+v", candidate.TradeEstimate)
	}
	if candidate.TradeEstimate.ThreatReduction != 4 {
		t.Fatalf("expected threat reduction 4, got %+v", candidate.TradeEstimate)
	}
}

func TestCombatPlanPromptAndDataMapIncludeTradeSummary(t *testing.T) {
	snapshot := CombatSnapshot{
		Player: CombatPlayerState{CurrentHP: 5, MaxHP: 30, Block: 0, Energy: 1},
		Hand: []CombatCardState{
			{Index: 1, CardID: "DEFEND_IRONCLAD", Name: "Defend", Playable: true, EnergyCost: 1},
		},
		Enemies: []CombatEnemyState{
			{
				Index:     0,
				EnemyID:   "SLIME",
				Name:      "Slime",
				CurrentHP: 18,
				Hittable:  true,
				Intents:   []CombatIntentState{{IntentType: "ATTACK", TotalDamage: 8}},
			},
		},
		CanPlayCard:    true,
		CanEndTurn:     true,
		IncomingDamage: 8,
	}
	candidate := combatPlanCandidateFromAction(snapshot, CombatAction{
		Request: game.ActionRequest{Action: "play_card", CardIndex: intPointer(1)},
		Label:   "play [1] Defend",
	}, 9.1)
	plan := &CombatPlan{Mode: "heuristic", Summary: "test", Candidates: []CombatPlanCandidate{candidate}}

	block := plan.PromptBlock(i18n.LanguageEnglish)
	if !strings.Contains(block, "hp loss 3") {
		t.Fatalf("expected prompt block to contain trade summary, got %q", block)
	}

	data := plan.DataMap(i18n.LanguageEnglish)
	candidates, ok := data["candidates"].([]map[string]any)
	if !ok || len(candidates) != 1 {
		t.Fatalf("expected candidate data map, got %#v", data["candidates"])
	}
	if got := candidates[0]["trade_summary"]; got != "hp loss 3, cover 5" {
		t.Fatalf("unexpected trade summary: %#v", got)
	}
}

func TestCombatSurvivalProfileForCriticalHP(t *testing.T) {
	profile := combatSurvivalProfileFor(CombatSnapshot{
		Floor:  6,
		Player: CombatPlayerState{CurrentHP: 12, MaxHP: 80},
	})

	if profile.Label != "critical-hp preservation" {
		t.Fatalf("expected critical preservation profile, got %+v", profile)
	}
	if profile.UnblockedDamageWeight <= 2.5 {
		t.Fatalf("expected very high unblocked damage penalty, got %+v", profile)
	}
}

func TestCombatSurvivalProfileForHealthyEarlyAggression(t *testing.T) {
	profile := combatSurvivalProfileFor(CombatSnapshot{
		Floor:  3,
		Player: CombatPlayerState{CurrentHP: 62, MaxHP: 80},
	})

	if profile.Label != "healthy early aggression" {
		t.Fatalf("expected healthy early aggression profile, got %+v", profile)
	}
	if profile.DamageWeight <= 1.1 {
		t.Fatalf("expected damage weight boost, got %+v", profile)
	}
	if profile.UnblockedDamageWeight >= 1.5 {
		t.Fatalf("expected lower early aggression damage penalty, got %+v", profile)
	}
}

func TestCombatSurvivalProfileForHighGoldFragileWindow(t *testing.T) {
	profile := combatSurvivalProfileFor(CombatSnapshot{
		Floor:  6,
		Gold:   150,
		Player: CombatPlayerState{CurrentHP: 46, MaxHP: 80},
	})

	if profile.RoutePressure != "protect the shop/conversion window" {
		t.Fatalf("expected shop-window route pressure, got %+v", profile)
	}
	if profile.UnblockedDamageWeight <= 2.0 {
		t.Fatalf("expected stronger HP preservation under shop-window pressure, got %+v", profile)
	}
}

func TestCombatSurvivalProfileForHealthyLowGoldSnowball(t *testing.T) {
	profile := combatSurvivalProfileFor(CombatSnapshot{
		Floor:  3,
		Gold:   40,
		Player: CombatPlayerState{CurrentHP: 68, MaxHP: 80},
	})

	if profile.RoutePressure != "push early snowball tempo" {
		t.Fatalf("expected snowball route pressure, got %+v", profile)
	}
	if profile.DamageWeight <= 1.2 {
		t.Fatalf("expected extra aggression under snowball pressure, got %+v", profile)
	}
}

func TestEstimateCardEffectWhirlwindScalesWithEnergy(t *testing.T) {
	effect := estimateCardEffect(CombatCardState{
		CardID:     "WHIRLWIND",
		Name:       "Whirlwind",
		EnergyCost: 0,
	}, 3)

	if !effect.TargetsAll {
		t.Fatalf("expected whirlwind to target all enemies, got %+v", effect)
	}
	if effect.Damage != 15 {
		t.Fatalf("expected whirlwind damage to scale with energy, got %+v", effect)
	}
}

func TestEstimateCardEffectIronWaveAndShrugItOff(t *testing.T) {
	ironWave := estimateCardEffect(CombatCardState{
		CardID:     "IRON_WAVE",
		Name:       "Iron Wave",
		EnergyCost: 1,
	}, 1)
	if ironWave.Damage != 5 || ironWave.Block != 5 {
		t.Fatalf("expected iron wave hybrid effect, got %+v", ironWave)
	}

	shrug := estimateCardEffect(CombatCardState{
		CardID:     "SHRUG_IT_OFF",
		Name:       "Shrug It Off",
		EnergyCost: 1,
	}, 1)
	if shrug.Block != 8 || shrug.Draw != 1 {
		t.Fatalf("expected shrug it off block+draw effect, got %+v", shrug)
	}
}
