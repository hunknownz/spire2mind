package game

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestWaitUntilActionableFallsBackToPollingWhileEventsStayOpen(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	stateReads := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/state":
			mu.Lock()
			stateReads++
			readCount := stateReads
			mu.Unlock()

			state := &StateSnapshot{
				StateVersion: 1,
				RunID:        "run_test",
				Screen:       "COMBAT",
				InCombat:     true,
				Combat: map[string]any{
					"actionWindowOpen": true,
					"enemies": []any{
						map[string]any{"index": 0, "isHittable": true},
					},
				},
			}

			if readCount >= 3 {
				state.AvailableActions = []string{"end_turn"}
			}

			writeEnvelope(t, w, state)
		case "/events/stream":
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Fatalf("response writer does not support flushing")
			}

			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			fmt.Fprint(w, "event: bridge_event\ndata: {\"type\":\"stream_ready\"}\n\n")
			flusher.Flush()

			<-r.Context().Done()
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	state, err := client.WaitUntilActionable(ctx, 3*time.Second)
	if err != nil {
		t.Fatalf("WaitUntilActionable returned error: %v", err)
	}

	if state == nil {
		t.Fatal("WaitUntilActionable returned nil state")
	}
	if len(state.AvailableActions) != 1 || state.AvailableActions[0] != "end_turn" {
		t.Fatalf("unexpected available actions: %#v", state.AvailableActions)
	}

	mu.Lock()
	defer mu.Unlock()
	if stateReads < 3 {
		t.Fatalf("expected polling to read /state multiple times, got %d", stateReads)
	}
}

func TestWaitUntilActionableDoesNotTreatEmptyGameOverAsActionable(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	stateReads := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/state":
			mu.Lock()
			stateReads++
			readCount := stateReads
			mu.Unlock()

			state := &StateSnapshot{
				StateVersion: 1,
				RunID:        "run_test",
				Screen:       "GAME_OVER",
				GameOver: map[string]any{
					"isVictory": false,
					"floor":     17,
					"stage":     "transition",
				},
			}

			if readCount >= 3 {
				state.AvailableActions = []string{"continue_after_game_over"}
				state.GameOver["stage"] = "results"
			}

			writeEnvelope(t, w, state)
		case "/events/stream":
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Fatalf("response writer does not support flushing")
			}

			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			fmt.Fprint(w, "event: bridge_event\ndata: {\"type\":\"stream_ready\"}\n\n")
			flusher.Flush()

			<-r.Context().Done()
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	state, err := client.WaitUntilActionable(ctx, 3*time.Second)
	if err != nil {
		t.Fatalf("WaitUntilActionable returned error: %v", err)
	}
	if state == nil {
		t.Fatal("WaitUntilActionable returned nil state")
	}
	if len(state.AvailableActions) != 1 || state.AvailableActions[0] != "continue_after_game_over" {
		t.Fatalf("unexpected available actions: %#v", state.AvailableActions)
	}

	mu.Lock()
	defer mu.Unlock()
	if stateReads < 3 {
		t.Fatalf("expected polling to continue past empty GAME_OVER, got %d reads", stateReads)
	}
}

func TestWaitUntilActionableStabilizesRapidlyChangingActionableState(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	stateReads := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/state":
			mu.Lock()
			stateReads++
			readCount := stateReads
			mu.Unlock()

			state := &StateSnapshot{
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
						map[string]any{"index": 0, "name": "Strike", "playable": true},
					},
				},
			}

			if readCount >= 2 {
				state.Combat["hand"] = []any{
					map[string]any{"index": 0, "name": "Strike", "playable": true},
					map[string]any{"index": 1, "name": "Defend", "playable": true},
				}
			}

			writeEnvelope(t, w, state)
		case "/events/stream":
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Fatalf("response writer does not support flushing")
			}

			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			fmt.Fprint(w, "event: bridge_event\ndata: {\"type\":\"stream_ready\"}\n\n")
			flusher.Flush()

			<-r.Context().Done()
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	state, err := client.WaitUntilActionable(ctx, 3*time.Second)
	if err != nil {
		t.Fatalf("WaitUntilActionable returned error: %v", err)
	}

	hand, ok := state.Combat["hand"].([]interface{})
	if !ok {
		t.Fatalf("expected combat hand slice, got %#v", state.Combat["hand"])
	}
	if len(hand) != 2 {
		t.Fatalf("expected stabilized hand size 2, got %d", len(hand))
	}
}

func TestWaitUntilActionableStabilizesDelayedCombatHandChange(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	stateReads := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/state":
			mu.Lock()
			stateReads++
			readCount := stateReads
			mu.Unlock()

			state := &StateSnapshot{
				StateVersion:     1,
				RunID:            "run_test",
				Screen:           "COMBAT",
				InCombat:         true,
				AvailableActions: []string{"play_card", "end_turn"},
				Combat: map[string]any{
					"actionWindowOpen": true,
					"enemies": []any{
						map[string]any{"index": 0, "enemyId": "SLIME_RED", "isHittable": true},
					},
					"hand": []any{
						map[string]any{"index": 0, "cardId": "STRIKE_RED", "playable": true},
					},
				},
			}

			if readCount >= 5 {
				state.Combat["hand"] = []any{
					map[string]any{"index": 0, "cardId": "STRIKE_RED", "playable": true},
					map[string]any{"index": 1, "cardId": "DEFEND_RED", "playable": true},
				}
			}

			writeEnvelope(t, w, state)
		case "/events/stream":
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Fatalf("response writer does not support flushing")
			}

			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			fmt.Fprint(w, "event: bridge_event\ndata: {\"type\":\"stream_ready\"}\n\n")
			flusher.Flush()

			<-r.Context().Done()
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	state, err := client.WaitUntilActionable(ctx, 3*time.Second)
	if err != nil {
		t.Fatalf("WaitUntilActionable returned error: %v", err)
	}

	hand, ok := state.Combat["hand"].([]interface{})
	if !ok {
		t.Fatalf("expected combat hand slice, got %#v", state.Combat["hand"])
	}
	if len(hand) != 2 {
		t.Fatalf("expected delayed combat hand change to be stabilized, got %d cards", len(hand))
	}

	mu.Lock()
	defer mu.Unlock()
	if stateReads < 5 {
		t.Fatalf("expected delayed stabilization to keep polling until read 5, got %d reads", stateReads)
	}
}

func TestWaitUntilActionableUsesLongerSettleForEndTurnOnlyCombatEdge(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	stateReads := 0
	turnFive := 5
	turnSix := 6

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/state":
			mu.Lock()
			stateReads++
			readCount := stateReads
			mu.Unlock()

			state := &StateSnapshot{
				StateVersion:     1,
				RunID:            "run_test",
				Screen:           "COMBAT",
				InCombat:         true,
				AvailableActions: []string{"end_turn"},
				Turn:             &turnFive,
				Combat: map[string]any{
					"actionWindowOpen": true,
					"player": map[string]any{
						"energy": 0,
					},
					"enemies": []any{
						map[string]any{"index": 0, "enemyId": "SLIME_RED", "isHittable": true},
					},
					"hand": []any{
						map[string]any{"index": 0, "cardId": "STRIKE_RED", "playable": false, "requiresTarget": true, "validTargetIndices": []any{0}},
						map[string]any{"index": 1, "cardId": "STRIKE_RED", "playable": false, "requiresTarget": true, "validTargetIndices": []any{0}},
					},
				},
			}

			if readCount >= 8 {
				state.Turn = &turnSix
				state.AvailableActions = []string{"play_card", "end_turn"}
				state.Combat["player"] = map[string]any{"energy": 3}
				state.Combat["hand"] = []any{
					map[string]any{"index": 0, "cardId": "STRIKE_RED", "playable": true, "requiresTarget": true, "validTargetIndices": []any{0}},
					map[string]any{"index": 1, "cardId": "DEFEND_RED", "playable": true},
				}
			}

			writeEnvelope(t, w, state)
		case "/events/stream":
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Fatalf("response writer does not support flushing")
			}

			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			fmt.Fprint(w, "event: bridge_event\ndata: {\"type\":\"stream_ready\"}\n\n")
			flusher.Flush()

			<-r.Context().Done()
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	state, err := client.WaitUntilActionable(ctx, 4*time.Second)
	if err != nil {
		t.Fatalf("WaitUntilActionable returned error: %v", err)
	}
	if state == nil {
		t.Fatal("WaitUntilActionable returned nil state")
	}
	if state.Turn == nil || *state.Turn != 6 {
		actualTurn := "<nil>"
		if state.Turn != nil {
			actualTurn = fmt.Sprintf("%d", *state.Turn)
		}
		t.Fatalf("expected stabilized combat edge to advance to turn 6, got turn %s", actualTurn)
	}
	if len(state.AvailableActions) != 2 || state.AvailableActions[0] != "play_card" {
		t.Fatalf("expected advanced actionable combat state, got %#v", state.AvailableActions)
	}

	mu.Lock()
	defer mu.Unlock()
	if stateReads < 8 {
		t.Fatalf("expected longer settle to keep polling through combat edge, got %d reads", stateReads)
	}
}

func TestWaitUntilActionableUsesLongerSettleForLowEnergyLateTurnCombat(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	stateReads := 0
	turnFive := 5
	turnSix := 6

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/state":
			mu.Lock()
			stateReads++
			readCount := stateReads
			mu.Unlock()

			state := &StateSnapshot{
				StateVersion:     1,
				RunID:            "run_test",
				Screen:           "COMBAT",
				InCombat:         true,
				AvailableActions: []string{"play_card", "end_turn"},
				Turn:             &turnFive,
				Combat: map[string]any{
					"actionWindowOpen": true,
					"player": map[string]any{
						"energy": 1,
					},
					"enemies": []any{
						map[string]any{"index": 0, "enemyId": "SLIME_RED", "isHittable": true},
					},
					"hand": []any{
						map[string]any{"index": 0, "cardId": "STRIKE_RED", "playable": true, "requiresTarget": true, "validTargetIndices": []any{0}},
						map[string]any{"index": 1, "cardId": "DEFEND_RED", "playable": true},
						map[string]any{"index": 2, "cardId": "DEFEND_RED", "playable": true},
					},
				},
			}

			if readCount >= 12 {
				state.Turn = &turnSix
				state.AvailableActions = []string{"play_card", "end_turn"}
				state.Combat["player"] = map[string]any{"energy": 3}
				state.Combat["hand"] = []any{
					map[string]any{"index": 0, "cardId": "STRIKE_RED", "playable": true, "requiresTarget": true, "validTargetIndices": []any{0}},
					map[string]any{"index": 1, "cardId": "DEFEND_RED", "playable": true},
					map[string]any{"index": 2, "cardId": "DEFEND_RED", "playable": true},
					map[string]any{"index": 3, "cardId": "DEFEND_RED", "playable": true},
					map[string]any{"index": 4, "cardId": "BASH_RED", "playable": true, "requiresTarget": true, "validTargetIndices": []any{0}},
				}
			}

			writeEnvelope(t, w, state)
		case "/events/stream":
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Fatalf("response writer does not support flushing")
			}

			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			fmt.Fprint(w, "event: bridge_event\ndata: {\"type\":\"stream_ready\"}\n\n")
			flusher.Flush()

			<-r.Context().Done()
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	state, err := client.WaitUntilActionable(ctx, 4*time.Second)
	if err != nil {
		t.Fatalf("WaitUntilActionable returned error: %v", err)
	}
	if state == nil {
		t.Fatal("WaitUntilActionable returned nil state")
	}
	if state.Turn == nil || *state.Turn != 6 {
		actualTurn := "<nil>"
		if state.Turn != nil {
			actualTurn = fmt.Sprintf("%d", *state.Turn)
		}
		t.Fatalf("expected stabilized late-turn combat to advance to turn 6, got turn %s", actualTurn)
	}
	if len(state.AvailableActions) != 2 || state.AvailableActions[0] != "play_card" {
		t.Fatalf("expected advanced actionable combat state, got %#v", state.AvailableActions)
	}

	mu.Lock()
	defer mu.Unlock()
	if stateReads < 12 {
		t.Fatalf("expected longer settle to keep polling through low-energy late-turn combat, got %d reads", stateReads)
	}
}

func TestWaitUntilActionableDoesNotTreatCombatWrapupAsActionable(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	stateReads := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/state":
			mu.Lock()
			stateReads++
			readCount := stateReads
			mu.Unlock()

			state := &StateSnapshot{
				StateVersion:     1,
				RunID:            "run_test",
				Screen:           "COMBAT",
				InCombat:         true,
				AvailableActions: []string{"play_card", "end_turn"},
				Combat: map[string]any{
					"actionWindowOpen":      true,
					"isOverOrEnding":        true,
					"playerActionsDisabled": true,
					"enemies": []any{
						map[string]any{"index": 0, "isHittable": false},
					},
				},
			}

			if readCount >= 3 {
				state.Screen = "REWARD"
				state.InCombat = false
				state.AvailableActions = []string{"claim_reward", "proceed"}
				state.Combat = nil
				state.Reward = map[string]any{
					"phase": "claim",
				}
			}

			writeEnvelope(t, w, state)
		case "/events/stream":
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Fatalf("response writer does not support flushing")
			}

			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			fmt.Fprint(w, "event: bridge_event\ndata: {\"type\":\"stream_ready\"}\n\n")
			flusher.Flush()

			<-r.Context().Done()
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient(server.URL)
	ctx := context.Background()

	state, err := client.WaitUntilActionable(ctx, 3*time.Second)
	if err != nil {
		t.Fatalf("WaitUntilActionable returned error: %v", err)
	}
	if state == nil {
		t.Fatal("WaitUntilActionable returned nil state")
	}
	if state.Screen != "REWARD" {
		t.Fatalf("expected reward state after combat wrapup, got %s", state.Screen)
	}

	mu.Lock()
	defer mu.Unlock()
	if stateReads < 3 {
		t.Fatalf("expected polling to continue past combat wrapup, got %d reads", stateReads)
	}
}

func writeEnvelope(t *testing.T, w http.ResponseWriter, payload any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(Envelope[any]{
		OK:   true,
		Data: payload,
	}); err != nil {
		t.Fatalf("encode envelope: %v", err)
	}
}
