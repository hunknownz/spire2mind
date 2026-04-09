package agentruntime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"spire2mind/internal/game"
)

func TestStableStateGateReadStableActionableStateStabilizesImmediateCombat(t *testing.T) {
	var mu sync.Mutex
	stateReads := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/state":
			mu.Lock()
			stateReads++
			readCount := stateReads
			mu.Unlock()

			state := &game.StateSnapshot{
				StateVersion:     1,
				RunID:            "run_test",
				Screen:           "COMBAT",
				InCombat:         true,
				AvailableActions: []string{"play_card", "end_turn"},
				Combat: map[string]any{
					"actionWindowOpen": true,
					"enemies": []any{
						map[string]any{"index": 0, "isHittable": true},
					},
					"hand": []any{
						map[string]any{"index": 0, "cardId": "STRIKE_RED", "playable": true},
					},
				},
			}
			if readCount >= 2 {
				state.Screen = "REWARD"
				state.InCombat = false
				state.AvailableActions = []string{"claim_reward", "proceed"}
				state.Combat = nil
				state.Reward = map[string]any{
					"phase":        "claim",
					"sourceScreen": "COMBAT",
				}
			}

			writeSessionEnvelope(t, w, state)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	gate := NewStableStateGate(game.NewClient(server.URL))
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	state, err := gate.ReadStableActionableState(ctx, nil, stateWaitTimeout)
	if err != nil {
		t.Fatalf("ReadStableActionableState returned error: %v", err)
	}
	if state == nil || state.Screen != "REWARD" {
		t.Fatalf("expected stabilized reward state, got %#v", state)
	}
}

func TestStableStateGateStabilizeExecutionDriftWaitsThroughGhostCombatFrame(t *testing.T) {
	var mu sync.Mutex
	stateReads := 0

	expectedState := &game.StateSnapshot{
		StateVersion:     1,
		RunID:            "run_test",
		Screen:           "COMBAT",
		InCombat:         true,
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"actionWindowOpen": true,
			"enemies": []any{
				map[string]any{"index": 0, "isHittable": true},
			},
			"hand": []any{
				map[string]any{"index": 0, "cardId": "STRIKE_RED", "playable": true},
			},
		},
	}
	ghostState := &game.StateSnapshot{
		StateVersion:     1,
		RunID:            "run_test",
		Screen:           "COMBAT",
		InCombat:         true,
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"actionWindowOpen": false,
			"enemies": []any{
				map[string]any{"index": 0, "isHittable": true},
			},
			"hand": []any{
				map[string]any{"index": 0, "cardId": "STRIKE_RED", "playable": true},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/state":
			mu.Lock()
			stateReads++
			readCount := stateReads
			mu.Unlock()

			state := ghostState
			if readCount >= 2 {
				state = expectedState
			}
			writeSessionEnvelope(t, w, state)
		case "/events/stream":
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("data: {\"type\":\"stream_ready\"}\n\n"))
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
			<-r.Context().Done()
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	gate := NewStableStateGate(game.NewClient(server.URL))
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	stabilized, driftKind := gate.StabilizeExecutionDrift(ctx, expectedState, ghostState, executionDriftWaitTimeout)
	if stabilized == nil || stabilized.Screen != "COMBAT" || !game.IsActionableState(stabilized) {
		t.Fatalf("expected actionable stabilized combat state, got %#v", stabilized)
	}
	if driftKind != driftKindSameScreenIndexDrift && driftKind != driftKindSameScreenStateDrift && driftKind != driftKindActionWindowChanged {
		t.Fatalf("unexpected drift kind: %s", driftKind)
	}
}
