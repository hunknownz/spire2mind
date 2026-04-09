package agentruntime

import (
	"strings"
	"testing"

	"spire2mind/internal/game"
)

func TestActionResolutionPipelineResolveRequestStripsPlayCardOptionIndex(t *testing.T) {
	t.Parallel()

	cardIndex := 3
	optionIndex := 9
	targetIndex := 0
	state := &game.StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"hand": []any{
				map[string]any{
					"index":              3,
					"cardId":             "STRIKE_RED",
					"playable":           true,
					"requiresTarget":     true,
					"validTargetIndices": []any{0},
				},
			},
			"enemies": []any{
				map[string]any{"index": 0, "enemyId": "SLIME_RED", "isHittable": true},
			},
		},
	}

	resolved, err := NewActionResolutionPipeline().ResolveRequest(state, state, game.ActionRequest{
		Action:      "play_card",
		CardIndex:   &cardIndex,
		OptionIndex: &optionIndex,
		TargetIndex: &targetIndex,
	})
	if err != nil {
		t.Fatalf("ResolveRequest returned unexpected error: %v", err)
	}
	if resolved.Request.CardIndex == nil || *resolved.Request.CardIndex != 3 {
		t.Fatalf("expected card_index 3, got %#v", resolved.Request)
	}
	if resolved.Request.OptionIndex != nil {
		t.Fatalf("expected play_card option_index to be cleared, got %#v", resolved.Request)
	}
	if resolved.Request.TargetIndex == nil || *resolved.Request.TargetIndex != 0 {
		t.Fatalf("expected target_index 0, got %#v", resolved.Request)
	}
}

func TestActionResolutionPipelineResolveDecisionReturnsStateUnavailableForUnrecoverablePlayCardDrift(t *testing.T) {
	t.Parallel()

	cardIndex := 0
	targetIndex := 0
	expectedState := &game.StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"hand": []any{
				map[string]any{
					"index":              0,
					"cardId":             "STRIKE_RED",
					"playable":           true,
					"requiresTarget":     true,
					"validTargetIndices": []any{0},
				},
			},
			"enemies": []any{
				map[string]any{"index": 0, "enemyId": "SLIME_RED", "isHittable": true},
			},
		},
	}
	liveState := &game.StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"hand": []any{
				map[string]any{
					"index":              0,
					"cardId":             "DEFEND_RED",
					"playable":           true,
					"requiresTarget":     false,
					"validTargetIndices": []any{},
				},
			},
			"enemies": []any{
				map[string]any{"index": 1, "enemyId": "LICE_RED", "isHittable": true},
			},
		},
	}

	_, err := NewActionResolutionPipeline().ResolveDecision(expectedState, liveState, &ActionDecision{
		Action:      "play_card",
		CardIndex:   &cardIndex,
		TargetIndex: &targetIndex,
	})
	if err == nil {
		t.Fatal("expected unrecoverable play_card drift to return an error")
	}
	if !strings.Contains(err.Error(), "state_unavailable") {
		t.Fatalf("expected state_unavailable error, got %v", err)
	}
}
