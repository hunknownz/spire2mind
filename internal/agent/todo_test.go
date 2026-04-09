package agentruntime

import (
	"strings"
	"testing"

	"spire2mind/internal/game"
	"spire2mind/internal/i18n"
)

func TestTodoManagerProceedsAfterResolvedCardReward(t *testing.T) {
	todo := NewTodoManager()

	before := &game.StateSnapshot{
		RunID:  "RUN-1",
		Screen: "REWARD",
		Reward: map[string]any{
			"pendingCardChoice": true,
		},
	}
	after := &game.StateSnapshot{
		RunID:            "RUN-1",
		Screen:           "REWARD",
		AvailableActions: []string{"claim_reward", "proceed"},
		Reward: map[string]any{
			"pendingCardChoice": false,
			"rewards": []any{
				map[string]any{
					"index":      0,
					"rewardType": "CardReward",
					"claimable":  true,
				},
			},
		},
	}

	todo.RecordAction("skip_reward_cards", before, after)
	todo.Update(after)

	if !todo.ShouldProceedAfterResolvedCardReward(after) {
		t.Fatal("expected todo manager to force proceed after the same card reward was already skipped")
	}
}

func TestTodoManagerDoesNotProceedWhenNonCardRewardRemains(t *testing.T) {
	todo := NewTodoManager()

	state := &game.StateSnapshot{
		RunID:            "RUN-1",
		Screen:           "REWARD",
		AvailableActions: []string{"claim_reward", "proceed"},
		Reward: map[string]any{
			"rewards": []any{
				map[string]any{
					"index":      0,
					"rewardType": "GoldReward",
					"claimable":  true,
				},
			},
		},
	}

	todo.RecordAction("skip_reward_cards", &game.StateSnapshot{RunID: "RUN-1", Screen: "REWARD"}, state)
	todo.Update(state)

	if todo.ShouldProceedAfterResolvedCardReward(state) {
		t.Fatal("did not expect proceed shortcut when a non-card reward still remains")
	}
}

func TestTodoManagerAppliesReflectionToPromptBlock(t *testing.T) {
	todo := NewTodoManager()
	reflection := &AttemptReflection{
		NextPlan: "Play safer at low health and spend gold earlier.",
		Lessons: []string{
			"Re-read the live state after fast transitions before repeating an indexed action.",
			"Spend excess gold earlier on removals, relics, or strong cards instead of dying with idle resources.",
		},
		LessonBuckets: ReflectionLessonBuckets{
			CombatSurvival: []string{"Play safer at low health."},
			ShopEconomy:    []string{"Spend gold earlier."},
		},
	}

	todo.ApplyReflection(reflection)
	block := todo.PromptBlock()

	if !strings.Contains(block, "Carry-forward plan: Play safer at low health and spend gold earlier.") {
		t.Fatalf("expected carry-forward plan in prompt block, got %q", block)
	}
	if !strings.Contains(block, "Carry-forward lessons:") {
		t.Fatalf("expected carry-forward lessons in prompt block, got %q", block)
	}
	if !strings.Contains(block, reflection.Lessons[0]) {
		t.Fatalf("expected first lesson in prompt block, got %q", block)
	}
	if !strings.Contains(block, "Carry-forward lessons by category:") {
		t.Fatalf("expected categorized carry-forward lessons in prompt block, got %q", block)
	}
	if !strings.Contains(block, "Combat survival:") || !strings.Contains(block, "Shop economy:") {
		t.Fatalf("expected lesson categories in prompt block, got %q", block)
	}
}

func TestTodoManagerPromptBlockCompactForLanguageChinese(t *testing.T) {
	todo := NewTodoManager()
	state := &game.StateSnapshot{
		RunID:            "RUN-ZH",
		Screen:           "SHOP",
		AvailableActions: []string{"buy_card", "buy_relic", "proceed"},
	}
	todo.Update(state)

	block := todo.PromptBlockCompactForLanguage(i18n.LanguageChinese)

	if !strings.Contains(block, "当前目标:") {
		t.Fatalf("expected Chinese current goal heading, got %q", block)
	}
	if !strings.Contains(block, "房间目标:") {
		t.Fatalf("expected Chinese room goal heading, got %q", block)
	}
	if strings.Contains(block, "Current goal:") {
		t.Fatalf("did not expect English heading in Chinese block, got %q", block)
	}
}
