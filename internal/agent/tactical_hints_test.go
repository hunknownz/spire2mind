package agentruntime

import (
	"strings"
	"testing"

	"spire2mind/internal/game"
)

func TestBuildCombatHintsFlagsLethalIncoming(t *testing.T) {
	state := &game.StateSnapshot{
		Screen: "COMBAT",
		Run: map[string]any{
			"currentHp": 11,
			"maxHp":     87,
		},
		Combat: map[string]any{
			"player": map[string]any{
				"block": 5,
			},
			"enemies": []any{
				map[string]any{
					"isAlive":   true,
					"currentHp": 15,
					"intents": []any{
						map[string]any{"totalDamage": 15},
					},
				},
			},
		},
	}

	hints := BuildTacticalHints(state)
	joined := strings.Join(hints, "\n")
	if !strings.Contains(joined, "Danger turn") && !strings.Contains(joined, "lethal or near-lethal") {
		t.Fatalf("expected danger-oriented combat hint, got %q", joined)
	}
}

func TestBuildShopHintsFlagsAffordableRemoval(t *testing.T) {
	state := &game.StateSnapshot{
		Screen: "SHOP",
		Run: map[string]any{
			"gold": 150,
		},
		Shop: map[string]any{
			"cardRemoval": map[string]any{
				"available":  true,
				"enoughGold": true,
				"price":      75,
			},
		},
	}

	hints := BuildTacticalHints(state)
	joined := strings.Join(hints, "\n")
	if !strings.Contains(joined, "card removal") {
		t.Fatalf("expected shop hint to mention card removal, got %q", joined)
	}
	if !strings.Contains(joined, "large pile of gold") {
		t.Fatalf("expected shop hint to warn about leaving with too much gold, got %q", joined)
	}
}

func TestBuildMapHintsWarnsAboutLowHPHighGoldRouting(t *testing.T) {
	state := &game.StateSnapshot{
		Screen: "MAP",
		Run: map[string]any{
			"currentHp": 20,
			"maxHp":     80,
			"gold":      160,
		},
	}

	hints := BuildTacticalHints(state)
	joined := strings.Join(hints, "\n")
	if !strings.Contains(joined, "Low-HP route rule") {
		t.Fatalf("expected low-HP route rule hint, got %q", joined)
	}
	if !strings.Contains(joined, "prioritize shops and removal") {
		t.Fatalf("expected low-HP high-gold routing hint, got %q", joined)
	}
}

func TestBuildRewardHintsFavorImmediateStrengthEarly(t *testing.T) {
	state := &game.StateSnapshot{
		Screen: "REWARD",
		Run: map[string]any{
			"floor":     6,
			"currentHp": 24,
			"maxHp":     80,
		},
	}

	hints := BuildTacticalHints(state)
	joined := strings.Join(hints, "\n")
	if !strings.Contains(joined, "Early-floor reward rule") {
		t.Fatalf("expected early-floor reward rule hint, got %q", joined)
	}
}
