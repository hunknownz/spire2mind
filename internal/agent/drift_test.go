package agentruntime

import (
	"strings"
	"testing"

	"spire2mind/internal/game"
)

func TestClassifyStateDriftSelectionSeam(t *testing.T) {
	t.Parallel()

	expected := &game.StateSnapshot{
		RunID:            "RUN-1",
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
	}
	live := &game.StateSnapshot{
		RunID:            "RUN-1",
		Screen:           "CARD_SELECTION",
		AvailableActions: []string{"select_deck_card"},
	}

	if got := classifyStateDrift(expected, live); got != driftKindSelectionSeam {
		t.Fatalf("classifyStateDrift = %q, want %q", got, driftKindSelectionSeam)
	}
}

func TestClassifyStateDriftSameScreenIndexDrift(t *testing.T) {
	t.Parallel()

	expected := &game.StateSnapshot{
		RunID:            "RUN-1",
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"hand": []any{
				map[string]any{"index": 0, "name": "Strike", "playable": true},
				map[string]any{"index": 1, "name": "Defend", "playable": true},
			},
		},
	}
	live := &game.StateSnapshot{
		RunID:            "RUN-1",
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"hand": []any{
				map[string]any{"index": 1, "name": "Defend", "playable": true},
			},
		},
	}

	if got := classifyStateDrift(expected, live); got != driftKindSameScreenIndexDrift {
		t.Fatalf("classifyStateDrift = %q, want %q", got, driftKindSameScreenIndexDrift)
	}
}

func TestClassifyStateDriftIgnoresDisplayNameNoiseWhenIDsMatch(t *testing.T) {
	t.Parallel()

	expected := &game.StateSnapshot{
		RunID:            "RUN-1",
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"hand": []any{
				map[string]any{"index": 0, "cardId": "STRIKE_RED", "name": "Strike", "playable": true},
			},
			"enemies": []any{
				map[string]any{"index": 0, "enemyId": "SLIME_RED", "name": "Red Slime", "isHittable": true, "currentHp": 18},
			},
		},
	}
	live := &game.StateSnapshot{
		RunID:            "RUN-1",
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"hand": []any{
				map[string]any{"index": 0, "cardId": "STRIKE_RED", "name": "鎵撳嚮", "playable": true},
			},
			"enemies": []any{
				map[string]any{"index": 0, "enemyId": "SLIME_RED", "name": "绾㈠彶鑾卞", "isHittable": true, "currentHp": 18},
			},
		},
	}

	if got := classifyStateDrift(expected, live); got != driftKindSameScreenStateDrift {
		t.Fatalf("classifyStateDrift = %q, want %q", got, driftKindSameScreenStateDrift)
	}
}

func TestClassifyStateDriftRewardTransitionWithinSameScreen(t *testing.T) {
	t.Parallel()

	expected := &game.StateSnapshot{
		RunID:  "RUN-1",
		Screen: "REWARD",
		Reward: map[string]any{
			"phase":             "claim",
			"sourceScreen":      "COMBAT",
			"pendingCardChoice": false,
		},
	}
	live := &game.StateSnapshot{
		RunID:  "RUN-1",
		Screen: "REWARD",
		Reward: map[string]any{
			"phase":             "card_choice",
			"sourceScreen":      "COMBAT",
			"pendingCardChoice": true,
		},
	}

	if got := classifyStateDrift(expected, live); got != driftKindRewardTransition {
		t.Fatalf("classifyStateDrift = %q, want %q", got, driftKindRewardTransition)
	}
}

func TestClassifyStateDriftSelectionSeamWithinSameScreen(t *testing.T) {
	t.Parallel()

	expected := &game.StateSnapshot{
		RunID:  "RUN-1",
		Screen: "CARD_SELECTION",
		Selection: map[string]any{
			"kind":         "NDeckUpgradeSelectScreen",
			"sourceScreen": "REST",
		},
	}
	live := &game.StateSnapshot{
		RunID:  "RUN-1",
		Screen: "CARD_SELECTION",
		Selection: map[string]any{
			"kind":         "combat_hand_upgrade_select",
			"sourceScreen": "COMBAT",
		},
	}

	if got := classifyStateDrift(expected, live); got != driftKindSelectionSeam {
		t.Fatalf("classifyStateDrift = %q, want %q", got, driftKindSelectionSeam)
	}
}

func TestStateUnavailableErrorIncludesDriftKind(t *testing.T) {
	t.Parallel()

	expected := &game.StateSnapshot{RunID: "RUN-1", Screen: "COMBAT"}
	live := &game.StateSnapshot{RunID: "RUN-1", Screen: "REWARD"}

	err := stateUnavailableError(expected, live)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "drift_kind="+driftKindRewardTransition) {
		t.Fatalf("expected drift kind in error, got %q", err)
	}
}

func TestIsSoftReplanDriftKind(t *testing.T) {
	t.Parallel()

	softKinds := []string{
		driftKindSelectionSeam,
		driftKindRewardTransition,
		driftKindSameScreenIndexDrift,
		driftKindActionWindowChanged,
		driftKindScreenTransition + ":combat",
	}
	for _, kind := range softKinds {
		if !isSoftReplanDriftKind(kind) {
			t.Fatalf("expected %q to be a soft replan drift", kind)
		}
	}

	if isSoftReplanDriftKind(driftKindSameScreenStateDrift) {
		t.Fatalf("did not expect %q to be a soft replan drift", driftKindSameScreenStateDrift)
	}
}
