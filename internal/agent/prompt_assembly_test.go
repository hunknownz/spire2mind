package agentruntime

import (
	"strings"
	"testing"

	"spire2mind/internal/game"
	"spire2mind/internal/i18n"
)

func TestPromptAssemblyPipelineStructuredUsesMinimalStatePayload(t *testing.T) {
	state := &game.StateSnapshot{
		RunID:            "RUN-1",
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Turn:             promptAssemblyIntPtr(3),
		Run: map[string]any{
			"floor":       7,
			"currentHp":   41,
			"maxHp":       70,
			"gold":        99,
			"characterId": "IRONCLAD",
		},
		Combat: map[string]any{
			"player": map[string]any{
				"currentHp": 41,
				"maxHp":     70,
				"block":     8,
				"energy":    3,
			},
			"hand": []map[string]any{
				{
					"index":              0,
					"id":                 "defend",
					"name":               "Defend",
					"cost":               1,
					"playable":           true,
					"requiresTarget":     false,
					"validTargetIndices": []int{},
				},
			},
			"enemies": []map[string]any{
				{
					"index":      0,
					"id":         "small_beetle",
					"name":       "Small Beetle",
					"currentHp":  12,
					"block":      0,
					"intent":     "Attack",
					"isHittable": true,
				},
			},
		},
	}

	assembly := NewPromptAssemblyPipeline().Build(
		PromptModeStructured,
		state,
		NewTodoManager(),
		nil,
		NewCompactMemory(),
		nil,
		i18n.LanguageEnglish,
	)

	if !strings.Contains(assembly.Text, "Minimal state payload:") {
		t.Fatalf("expected minimal state payload block, got:\n%s", assembly.Text)
	}
	if strings.Contains(assembly.Text, "Current state snapshot JSON") {
		t.Fatalf("expected full state JSON to be removed, got:\n%s", assembly.Text)
	}
	if assembly.Telemetry.PromptSizeBytes <= 0 {
		t.Fatalf("expected prompt size telemetry, got %+v", assembly.Telemetry)
	}
	if assembly.Telemetry.BlockBreakdown["minimal_state"] == 0 {
		t.Fatalf("expected minimal_state block breakdown, got %+v", assembly.Telemetry.BlockBreakdown)
	}
}

func TestPromptAssemblyPipelineRewardUsesRewardSpecificPayload(t *testing.T) {
	state := &game.StateSnapshot{
		RunID:            "RUN-2",
		Screen:           "REWARD",
		AvailableActions: []string{"claim_reward", "choose_reward_card", "proceed"},
		Reward: map[string]any{
			"pendingCardChoice": true,
			"canProceed":        false,
			"rewards": []map[string]any{
				{"index": 0, "rewardType": "Gold", "name": "Gold", "claimable": true},
			},
			"cardOptions": []map[string]any{
				{"index": 0, "id": "bash", "name": "Bash", "cost": 2},
			},
		},
	}

	assembly := NewPromptAssemblyPipeline().Build(
		PromptModeStructured,
		state,
		NewTodoManager(),
		nil,
		NewCompactMemory(),
		nil,
		i18n.LanguageEnglish,
	)

	if !strings.Contains(assembly.Text, "\"reward\"") {
		t.Fatalf("expected reward payload in prompt, got:\n%s", assembly.Text)
	}
	if strings.Contains(assembly.Text, "\"combat\"") {
		t.Fatalf("did not expect combat payload in reward prompt, got:\n%s", assembly.Text)
	}
}

func TestPromptAssemblyPipelineStructuredCombatUsesCombatSpecificGuidance(t *testing.T) {
	state := &game.StateSnapshot{
		RunID:            "RUN-COMBAT",
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Run: map[string]any{
			"floor":     2,
			"currentHp": 50,
			"maxHp":     70,
		},
		Combat: map[string]any{
			"player": map[string]any{
				"currentHp": 50,
				"maxHp":     70,
				"block":     0,
				"energy":    3,
			},
			"hand": []map[string]any{
				{
					"index":          0,
					"id":             "bash",
					"name":           "Bash",
					"cost":           2,
					"playable":       true,
					"requiresTarget": true,
				},
			},
			"enemies": []map[string]any{
				{
					"index":      0,
					"id":         "slime",
					"name":       "Slime",
					"currentHp":  14,
					"block":      0,
					"intent":     "Attack",
					"isHittable": true,
				},
			},
		},
	}

	compact := NewCompactMemory()
	compact.lessons = []string{"Do not over-block safe turns."}
	assembly := NewPromptAssemblyPipeline().Build(
		PromptModeStructured,
		state,
		NewTodoManager(),
		NewSkillLibrary("C:\\Users\\klerc\\spire2mind"),
		compact,
		&CombatPlan{},
		i18n.LanguageEnglish,
	)

	if !strings.Contains(assembly.Text, "Combat guidance:") {
		t.Fatalf("expected combat guidance block, got:\n%s", assembly.Text)
	}
	if !strings.Contains(assembly.Text, "Prefer the planner's best legal line") {
		t.Fatalf("expected planner-first combat guidance, got:\n%s", assembly.Text)
	}
	if !strings.Contains(assembly.Text, "Run objective:") {
		t.Fatalf("expected run objective block, got:\n%s", assembly.Text)
	}
	if !strings.Contains(assembly.Text, "reach deeper floors and eventually win") {
		t.Fatalf("expected depth objective guidance, got:\n%s", assembly.Text)
	}
	if !strings.Contains(assembly.Text, "Preserve HP when the board is dangerous") {
		t.Fatalf("expected survival-oriented combat guidance, got:\n%s", assembly.Text)
	}
	if _, ok := assembly.Telemetry.BlockBreakdown["compact_memory"]; ok {
		t.Fatalf("did not expect compact memory block in combat structured prompt, got %+v", assembly.Telemetry.BlockBreakdown)
	}
	if _, ok := assembly.Telemetry.BlockBreakdown["entity_knowledge"]; !ok {
		t.Fatalf("expected combat structured prompt to include entity knowledge, got %+v", assembly.Telemetry.BlockBreakdown)
	}
}

func TestPromptAssemblyPipelineStructuredMapUsesOptionIndexGuidance(t *testing.T) {
	state := &game.StateSnapshot{
		RunID:            "RUN-MAP",
		Screen:           "MAP",
		AvailableActions: []string{"choose_map_node"},
		Map: map[string]any{
			"availableNodes": []map[string]any{
				{"index": 0, "symbol": "M", "nodeType": "monster"},
			},
		},
	}

	assembly := NewPromptAssemblyPipeline().Build(
		PromptModeStructured,
		state,
		NewTodoManager(),
		NewSkillLibrary("C:\\Users\\klerc\\spire2mind"),
		NewCompactMemory(),
		nil,
		i18n.LanguageEnglish,
	)

	if !strings.Contains(assembly.Text, "choose_map_node uses option_index") {
		t.Fatalf("expected map option_index guidance, got:\n%s", assembly.Text)
	}
	if !strings.Contains(assembly.Text, "improve expected floor depth") {
		t.Fatalf("expected map depth guidance, got:\n%s", assembly.Text)
	}
	if _, ok := assembly.Telemetry.BlockBreakdown["entity_knowledge"]; !ok {
		t.Fatalf("expected map structured prompt to include entity knowledge, got %+v", assembly.Telemetry.BlockBreakdown)
	}
}

func TestPromptAssemblyPipelineStructuredRewardUsesSurvivalGuidance(t *testing.T) {
	state := &game.StateSnapshot{
		RunID:            "RUN-REWARD-GUIDE",
		Screen:           "REWARD",
		AvailableActions: []string{"choose_reward_card", "skip_reward_cards"},
		Run: map[string]any{
			"floor":     4,
			"currentHp": 24,
			"maxHp":     80,
		},
		Reward: map[string]any{
			"pendingCardChoice": true,
			"cardOptions": []map[string]any{
				{"index": 0, "id": "shrug", "name": "Shrug It Off", "cost": 1},
			},
		},
	}

	assembly := NewPromptAssemblyPipeline().Build(
		PromptModeStructured,
		state,
		NewTodoManager(),
		NewSkillLibrary("C:\\Users\\klerc\\spire2mind"),
		NewCompactMemory(),
		nil,
		i18n.LanguageEnglish,
	)

	if !strings.Contains(assembly.Text, "Prefer picks that improve near-term survival") {
		t.Fatalf("expected reward survival guidance, got:\n%s", assembly.Text)
	}
	if !strings.Contains(assembly.Text, "Current risk posture: fragile") {
		t.Fatalf("expected fragile risk posture in run objective, got:\n%s", assembly.Text)
	}
}

func TestPromptAssemblyPipelineStructuredPreservesChineseEntities(t *testing.T) {
	state := &game.StateSnapshot{
		RunID:            "RUN-ZH",
		Screen:           "REWARD",
		AvailableActions: []string{"choose_reward_card", "skip_reward_cards"},
		Run: map[string]any{
			"floor":     5,
			"currentHp": 47,
			"maxHp":     80,
			"gold":      45,
		},
		AgentView: map[string]any{
			"headline": "REWARD: 防御 / 蛮兽猛击",
		},
		Reward: map[string]any{
			"pendingCardChoice": true,
			"cardOptions": []map[string]any{
				{
					"index": 0,
					"id":    "DEFEND_IRONCLAD",
					"name":  "防御",
					"cost":  1,
				},
				{
					"index": 1,
					"id":    "MAWLER_STRIKE",
					"name":  "蛮兽猛击",
					"cost":  1,
				},
			},
		},
	}

	assembly := NewPromptAssemblyPipeline().Build(
		PromptModeStructured,
		state,
		NewTodoManager(),
		NewSkillLibrary("C:\\Users\\klerc\\spire2mind"),
		NewCompactMemory(),
		nil,
		i18n.LanguageEnglish,
	)

	for _, fragment := range []string{"防御", "蛮兽猛击"} {
		if !strings.Contains(assembly.Text, fragment) {
			t.Fatalf("expected prompt to preserve Chinese entity %q, got:\n%s", fragment, assembly.Text)
		}
	}
}

func TestPromptAssemblyPipelineStructuredRewardAppliesPromptBudget(t *testing.T) {
	state := &game.StateSnapshot{
		RunID:            "RUN-BUDGET-REWARD",
		Screen:           "REWARD",
		AvailableActions: []string{"choose_reward_card", "skip_reward_cards"},
		Run: map[string]any{
			"floor":     6,
			"currentHp": 32,
			"maxHp":     80,
			"gold":      190,
		},
		Reward: map[string]any{
			"pendingCardChoice": true,
			"cardOptions": []map[string]any{
				{"index": 0, "id": "shrug", "name": "Shrug It Off", "cost": 1},
				{"index": 1, "id": "pommel", "name": "Pommel Strike", "cost": 1},
			},
		},
	}

	todo := NewTodoManager()
	todo.currentGoal = strings.Repeat("survive this act and preserve hp; ", 80)
	todo.roomGoal = strings.Repeat("pick the strongest reward; ", 80)
	todo.nextIntent = strings.Repeat("resolve reward cleanly; ", 80)

	compact := NewCompactMemory()
	compact.lastSummary = strings.Repeat("summary ", 300)
	compact.recent = []string{
		strings.Repeat("timeline-one ", 120),
		strings.Repeat("timeline-two ", 120),
	}
	compact.lessons = []string{
		strings.Repeat("spend gold earlier; ", 120),
		strings.Repeat("avoid greedy lines at low hp; ", 120),
	}

	skills := NewSkillLibrary("C:\\Users\\klerc\\spire2mind")
	skills.cache["deck-archetypes"] = strings.Repeat("reward skill context\n", 800)

	pipeline := NewPromptAssemblyPipeline()
	assembly := pipeline.Build(
		PromptModeStructured,
		state,
		todo,
		skills,
		compact,
		nil,
		i18n.LanguageEnglish,
	)

	budget := pipeline.structuredPromptBudget(state)
	if assembly.Telemetry.PromptSizeBytes > budget.MaxBytes {
		t.Fatalf("expected prompt size <= %d, got %d", budget.MaxBytes, assembly.Telemetry.PromptSizeBytes)
	}
	if !strings.Contains(assembly.Text, "Reward guidance:") {
		t.Fatalf("expected reward guidance to be preserved, got:\n%s", assembly.Text)
	}
	if !strings.Contains(assembly.Text, "Minimal state payload:") {
		t.Fatalf("expected minimal state payload to be preserved, got:\n%s", assembly.Text)
	}
}

func TestPromptAssemblyPipelineStructuredCombatAppliesPromptBudget(t *testing.T) {
	state := &game.StateSnapshot{
		RunID:            "RUN-BUDGET-COMBAT",
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Turn:             promptAssemblyIntPtr(5),
		Run: map[string]any{
			"floor":     15,
			"currentHp": 39,
			"maxHp":     80,
			"gold":      210,
		},
		Combat: map[string]any{
			"player": map[string]any{
				"currentHp": 39,
				"maxHp":     80,
				"block":     8,
				"energy":    3,
			},
			"hand": []map[string]any{
				{"index": 0, "id": "bash", "name": "Bash", "cost": 2, "playable": true, "requiresTarget": true, "validTargetIndices": []int{0}},
				{"index": 1, "id": "shrug", "name": "Shrug It Off", "cost": 1, "playable": true, "requiresTarget": false},
			},
			"enemies": []map[string]any{
				{"index": 0, "id": "cultist", "name": "Cultist", "currentHp": 42, "block": 0, "intent": "Attack", "isHittable": true},
				{"index": 1, "id": "slaver", "name": "Red Slaver", "currentHp": 38, "block": 0, "intent": "Attack", "isHittable": true},
			},
		},
	}

	todo := NewTodoManager()
	todo.currentGoal = strings.Repeat("win the fight without losing hp; ", 90)
	todo.roomGoal = strings.Repeat("find the best legal line; ", 90)
	todo.nextIntent = strings.Repeat("use planner output; ", 90)

	skills := NewSkillLibrary("C:\\Users\\klerc\\spire2mind")
	skills.cache["combat-basics"] = strings.Repeat("combat skill context\n", 900)
	skills.cache["deck-archetypes"] = strings.Repeat("deck skill context\n", 900)

	assembly := NewPromptAssemblyPipeline().Build(
		PromptModeStructured,
		state,
		todo,
		skills,
		NewCompactMemory(),
		&CombatPlan{Summary: strings.Repeat("planner summary ", 500)},
		i18n.LanguageEnglish,
	)

	budget := NewPromptAssemblyPipeline().structuredPromptBudget(state)
	if assembly.Telemetry.PromptSizeBytes > budget.MaxBytes {
		t.Fatalf("expected combat prompt size <= %d, got %d", budget.MaxBytes, assembly.Telemetry.PromptSizeBytes)
	}
	if !strings.Contains(assembly.Text, "Combat guidance:") {
		t.Fatalf("expected combat guidance to be preserved, got:\n%s", assembly.Text)
	}
	if !strings.Contains(assembly.Text, "Run objective:") {
		t.Fatalf("expected run objective to be preserved, got:\n%s", assembly.Text)
	}
}

func TestPromptAssemblyPipelineStructuredChineseUsesLocalizedBlocks(t *testing.T) {
	state := &game.StateSnapshot{
		RunID:            "RUN-ZH-COMBAT",
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Turn:             promptAssemblyIntPtr(1),
		Run: map[string]any{
			"floor":     3,
			"currentHp": 17,
			"maxHp":     80,
			"gold":      145,
		},
		Combat: map[string]any{
			"player": map[string]any{
				"currentHp": 17,
				"maxHp":     80,
				"block":     2,
				"energy":    3,
			},
			"hand": []map[string]any{
				{"index": 0, "id": "bash", "name": "重击", "cost": 2, "playable": true, "requiresTarget": true, "validTargetIndices": []int{0}},
			},
			"enemies": []map[string]any{
				{"index": 0, "id": "slime", "name": "酸液史莱姆", "currentHp": 9, "block": 0, "intent": "Attack", "isHittable": true, "intents": []map[string]any{{"type": "Attack", "totalDamage": 10}}},
				{"index": 1, "id": "louse", "name": "小啃兽", "currentHp": 12, "block": 0, "intent": "Attack", "isHittable": true, "intents": []map[string]any{{"type": "Attack", "totalDamage": 9}}},
			},
		},
	}

	todo := NewTodoManager()
	todo.Update(state)

	assembly := NewPromptAssemblyPipeline().Build(
		PromptModeStructured,
		state,
		todo,
		NewSkillLibrary("C:\\Users\\klerc\\spire2mind"),
		NewCompactMemory(),
		nil,
		i18n.LanguageChinese,
	)

	for _, fragment := range []string{"当前目标", "战术提示", "技能参考", "当前能量较高"} {
		if !strings.Contains(assembly.Text, fragment) {
			t.Fatalf("expected Chinese fragment %q in prompt, got:\n%s", fragment, assembly.Text)
		}
	}
	if strings.Contains(assembly.Text, "Current goal:") {
		t.Fatalf("did not expect English todo heading in Chinese prompt, got:\n%s", assembly.Text)
	}
}

func promptAssemblyIntPtr(value int) *int {
	return &value
}
