package agentruntime

import (
	"strings"
	"testing"

	"spire2mind/internal/game"
)

func TestDecisionStateDigestChangesWhenCombatHandChanges(t *testing.T) {
	base := &game.StateSnapshot{
		RunID:            "RUN-COMBAT",
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Run: map[string]any{
			"currentHp": 48,
			"maxHp":     80,
		},
		Combat: map[string]any{
			"player": map[string]any{
				"energy": 3,
				"block":  0,
			},
			"hand": []any{
				map[string]any{
					"index":      0,
					"cardId":     "STRIKE_RED",
					"name":       "Strike",
					"energyCost": 1,
					"playable":   true,
				},
			},
			"enemies": []any{
				map[string]any{
					"index":      0,
					"enemyId":    "SLIME",
					"name":       "Slime",
					"currentHp":  20,
					"isHittable": true,
				},
			},
		},
	}

	changed := *base
	changed.Combat = map[string]any{
		"player": map[string]any{
			"energy": 3,
			"block":  0,
		},
		"hand": []any{
			map[string]any{
				"index":      0,
				"cardId":     "DEFEND_RED",
				"name":       "Defend",
				"energyCost": 1,
				"playable":   true,
			},
		},
		"enemies": []any{
			map[string]any{
				"index":      0,
				"enemyId":    "SLIME",
				"name":       "Slime",
				"currentHp":  20,
				"isHittable": true,
			},
		},
	}

	left := decisionStateDigest(base)
	right := decisionStateDigest(&changed)
	if left == right {
		t.Fatalf("expected digest to change when combat hand changed; digest=%q", left)
	}
}

func TestDecisionStateDigestIgnoresCombatDisplayNameNoiseWhenIDsMatch(t *testing.T) {
	base := &game.StateSnapshot{
		RunID:            "RUN-COMBAT",
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"player": map[string]any{
				"energy": 3,
				"block":  0,
			},
			"hand": []any{
				map[string]any{
					"index":      0,
					"cardId":     "STRIKE_RED",
					"name":       "Strike",
					"energyCost": 1,
					"playable":   true,
				},
			},
			"enemies": []any{
				map[string]any{
					"index":      0,
					"enemyId":    "SLIME_RED",
					"name":       "Red Slime",
					"currentHp":  18,
					"isHittable": true,
				},
			},
		},
	}

	changed := *base
	changed.Combat = map[string]any{
		"player": map[string]any{
			"energy": 3,
			"block":  0,
		},
		"hand": []any{
			map[string]any{
				"index":      0,
				"cardId":     "STRIKE_RED",
				"name":       "打击",
				"energyCost": 1,
				"playable":   true,
			},
		},
		"enemies": []any{
			map[string]any{
				"index":      0,
				"enemyId":    "SLIME_RED",
				"name":       "红色史莱姆",
				"currentHp":  18,
				"isHittable": true,
			},
		},
	}

	left := decisionStateDigest(base)
	right := decisionStateDigest(&changed)
	if left != right {
		t.Fatalf("expected digest to stay stable when only display names changed; left=%q right=%q", left, right)
	}
}

func TestDecisionStateSummaryIncludesRoomDetail(t *testing.T) {
	state := &game.StateSnapshot{
		RunID:            "RUN-SUMMARY",
		Screen:           "REWARD",
		AvailableActions: []string{"choose_reward_card", "proceed"},
		Run: map[string]any{
			"floor":     7,
			"currentHp": 30,
			"maxHp":     80,
			"gold":      91,
		},
		Reward: map[string]any{
			"phase":        "card_choice",
			"sourceScreen": "COMBAT",
			"cardOptions": []any{
				map[string]any{
					"index":  0,
					"cardId": "IRON_WAVE",
					"name":   "Iron Wave",
				},
			},
		},
	}

	summary := decisionStateSummary(state)
	for _, fragment := range []string{
		"Screen: REWARD",
		"Run: RUN-SUMMARY",
		"Actions: choose_reward_card, proceed",
		"Reward phase: card_choice (source COMBAT)",
		"Card option 0: [0] Iron Wave",
	} {
		if !strings.Contains(summary, fragment) {
			t.Fatalf("expected summary to contain %q, got %q", fragment, summary)
		}
	}
}
