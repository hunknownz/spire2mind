package agentruntime

import (
	"strings"
	"testing"

	"spire2mind/internal/game"
	"spire2mind/internal/i18n"
)

func TestRenderRunGuideDocumentIncludesBucketsFailuresAndSeeds(t *testing.T) {
	t.Parallel()

	todo := NewTodoManager()
	todo.ApplyReflection(&AttemptReflection{
		Lessons: []string{"Play safer at low health.", "Spend gold earlier."},
		LessonBuckets: ReflectionLessonBuckets{
			CombatSurvival: []string{"Play safer at low health."},
			ShopEconomy:    []string{"Spend gold earlier."},
		},
		NextPlan: "Protect HP before forcing elite fights.",
	})

	reflections := []*AttemptReflection{
		{
			Attempt:        2,
			RunID:          "RUN-2",
			Outcome:        "defeat",
			Headline:       "A greedy reward pick made the next hallway fight unstable",
			RecentFailures: []string{"selection seam forced a replan before a card choice"},
			Risks:          []string{"Died with 95 unspent gold - convert resources earlier."},
			LessonBuckets: ReflectionLessonBuckets{
				RewardChoice: []string{"Prefer immediate combat value over speculative setup."},
				Runtime:      []string{"Re-read the live state after fast transitions before replaying indexed actions."},
			},
		},
	}

	state := &game.StateSnapshot{
		RunID:            "RUN-LIVE",
		Screen:           "MAP",
		AvailableActions: []string{"choose_map_node"},
		Run: map[string]any{
			"floor":     7,
			"currentHp": 18,
			"maxHp":     80,
			"gold":      95,
		},
	}

	codex := &SeenContentRegistry{
		Monsters: []SeenContentEntry{{Category: seenCategoryMonsters, ID: "slime_small", Name: "Small Slime"}},
	}

	markdown := renderRunGuideDocument(reflections, 3, state, todo, codex, i18n.LanguageEnglish)
	if !strings.Contains(markdown, "## Live Doctrine") {
		t.Fatalf("expected live doctrine section, got %q", markdown)
	}
	if !strings.Contains(markdown, "## Known World") {
		t.Fatalf("expected known world section, got %q", markdown)
	}
	if !strings.Contains(markdown, "## Strategy Ledger") {
		t.Fatalf("expected strategy ledger section, got %q", markdown)
	}
	if !strings.Contains(markdown, "Combat survival") || !strings.Contains(markdown, "Shop economy") {
		t.Fatalf("expected categorized carry-forward guide content, got %q", markdown)
	}
	if !strings.Contains(markdown, "selection seam forced a replan") {
		t.Fatalf("expected failure pattern to appear in guide, got %q", markdown)
	}
	if !strings.Contains(markdown, "Attempt 2 on floor -:") {
		t.Fatalf("expected story seed to mention reflection attempt, got %q", markdown)
	}
}
