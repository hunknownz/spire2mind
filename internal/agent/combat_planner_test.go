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
