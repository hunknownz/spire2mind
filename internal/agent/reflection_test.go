package agentruntime

import (
	"testing"

	"spire2mind/internal/game"
)

func TestBuildAttemptReflectionCategorizesLessons(t *testing.T) {
	t.Parallel()

	state := &game.StateSnapshot{
		Screen: "GAME_OVER",
		Run: map[string]any{
			"floor":     5,
			"currentHp": 8,
			"maxHp":     80,
			"gold":      120,
			"deck":      make([]any, 32),
		},
		GameOver: map[string]any{
			"isVictory": false,
			"floor":     5,
		},
	}

	todo := NewTodoManager()
	todo.RecordFailure("play_card", assertErr("invalid_action: stale index"))

	reflection := BuildAttemptReflection(1, state, todo, nil)
	if reflection == nil {
		t.Fatal("expected reflection")
	}

	if len(reflection.LessonBuckets.Runtime) == 0 {
		t.Fatal("expected runtime lessons")
	}
	if len(reflection.LessonBuckets.Pathing) == 0 {
		t.Fatal("expected pathing lessons")
	}
	if len(reflection.LessonBuckets.RewardChoice) == 0 {
		t.Fatal("expected reward-choice lessons")
	}
	if len(reflection.LessonBuckets.ShopEconomy) == 0 {
		t.Fatal("expected shop-economy lessons")
	}
	if len(reflection.LessonBuckets.CombatSurvival) == 0 {
		t.Fatal("expected combat-survival lessons")
	}
	if len(reflection.Lessons) == 0 {
		t.Fatal("expected flattened lessons")
	}
	if len(reflection.RuntimeNoise) == 0 {
		t.Fatal("expected runtime noise classification")
	}
	if len(reflection.TacticalMistakes) == 0 {
		t.Fatal("expected tactical mistakes classification")
	}
	if len(reflection.ResourceMistakes) == 0 {
		t.Fatal("expected resource mistakes classification")
	}
}

type assertErr string

func (e assertErr) Error() string { return string(e) }
