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

func promptAssemblyIntPtr(value int) *int {
	return &value
}
