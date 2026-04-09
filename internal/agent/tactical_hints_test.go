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
}
