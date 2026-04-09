package agentruntime

import (
	"strings"
	"testing"

	"spire2mind/internal/game"
	"spire2mind/internal/i18n"
)

func TestRenderRunStoryDocumentBuildsStrategyLedgerAndLimitsOlderAttempts(t *testing.T) {
	todo := NewTodoManager()
	todo.ApplyReflection(&AttemptReflection{
		Lessons: []string{"Play safer at low health.", "Spend gold earlier."},
		LessonBuckets: ReflectionLessonBuckets{
			CombatSurvival: []string{"Play safer at low health."},
			ShopEconomy:    []string{"Spend gold earlier."},
		},
	})

	compact := NewCompactMemory()
	compact.lastStory = "The latest loss came from a greedy line into a danger turn."

	reflections := []*AttemptReflection{
		{Attempt: 1, Story: "Attempt one.", Lessons: []string{"Play safer at low health."}},
		{Attempt: 2, Story: "Attempt two.", Lessons: []string{"Spend gold earlier."}},
		{Attempt: 3, Story: "Attempt three.", Lessons: []string{"Prefer cleaner combat exits."}},
		{Attempt: 4, Story: "Attempt four.", Lessons: []string{"Keep block available on danger turns."}},
	}

	state := &game.StateSnapshot{
		RunID:            "RUN123",
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Run: map[string]any{
			"floor":     9,
			"currentHp": 18,
			"maxHp":     80,
			"gold":      92,
		},
	}

	codex := &SeenContentRegistry{
		Cards: []SeenContentEntry{{Category: seenCategoryCards, ID: "shrug_it_off", Name: "Shrug It Off"}},
	}

	markdown := renderRunStoryDocument(reflections, 5, state, todo, compact, codex, i18n.LanguageEnglish)
	if !strings.Contains(markdown, "## Strategy Ledger") {
		t.Fatalf("expected strategy ledger section, got %q", markdown)
	}
	if !strings.Contains(markdown, "### Known World") {
		t.Fatalf("expected known world section, got %q", markdown)
	}
	strategyLedger := markdown[strings.Index(markdown, "## Strategy Ledger"):]
	if strings.Count(strategyLedger, "- Play safer at low health.") != 1 {
		t.Fatalf("expected deduplicated carry-forward lesson inside strategy ledger, got %q", markdown)
	}
	if !strings.Contains(strategyLedger, "- Combat survival:") || !strings.Contains(strategyLedger, "- Shop economy:") {
		t.Fatalf("expected categorized strategy ledger entries, got %q", markdown)
	}
	if !strings.Contains(markdown, "_Showing the latest 3 attempts.") {
		t.Fatalf("expected older-attempt note, got %q", markdown)
	}
	if strings.Contains(markdown, "Attempt one.") {
		t.Fatalf("expected older attempt story to be omitted from the main story document, got %q", markdown)
	}
	if !strings.Contains(markdown, "Attempt four.") {
		t.Fatalf("expected recent attempts to remain visible, got %q", markdown)
	}
	if !strings.Contains(markdown, "### Attempt 2") ||
		!strings.Contains(markdown, "### Attempt 3") ||
		!strings.Contains(markdown, "### Attempt 4") {
		t.Fatalf("expected recent reflections to be renumbered sequentially in the story view, got %q", markdown)
	}
}
