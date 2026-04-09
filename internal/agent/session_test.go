package agentruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	openagent "github.com/hunknownz/open-agent-sdk-go/agent"

	"spire2mind/internal/config"
	"spire2mind/internal/game"
	"spire2mind/internal/i18n"
)

func TestRemapActionRequestForLiveStateReusesCompatiblePlayCard(t *testing.T) {
	cardIndex := 0
	targetIndex := 0
	request := game.ActionRequest{
		Action:      "play_card",
		CardIndex:   &cardIndex,
		TargetIndex: &targetIndex,
	}

	expectedState := &game.StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"hand": []any{
				map[string]any{
					"index":              0,
					"id":                 "strike",
					"name":               "Strike",
					"playable":           true,
					"requiresTarget":     true,
					"validTargetIndices": []any{0},
				},
			},
			"enemies": []any{
				map[string]any{
					"index":      0,
					"id":         "slime-small",
					"name":       "Slime",
					"isHittable": true,
					"currentHp":  12,
				},
			},
		},
	}

	liveState := &game.StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"hand": []any{
				map[string]any{
					"index":              4,
					"id":                 "strike",
					"name":               "Strike",
					"playable":           true,
					"requiresTarget":     true,
					"validTargetIndices": []any{2},
				},
			},
			"enemies": []any{
				map[string]any{
					"index":      2,
					"id":         "slime-small",
					"name":       "Slime",
					"isHittable": true,
					"currentHp":  12,
				},
			},
		},
	}

	remapped, recoveryKind, ok := remapActionRequestForLiveState(expectedState, liveState, request)
	if !ok {
		t.Fatal("expected compatible play_card request to be remapped")
	}
	if recoveryKind != "decision_remap" {
		t.Fatalf("expected decision_remap, got %q", recoveryKind)
	}
	if remapped.CardIndex == nil || *remapped.CardIndex != 4 {
		t.Fatalf("expected card index to remap to 4, got %+v", remapped)
	}
	if remapped.TargetIndex == nil || *remapped.TargetIndex != 2 {
		t.Fatalf("expected target index to remap to 2, got %+v", remapped)
	}
}

func TestRemapActionRequestForLiveStateNormalizesSingleTarget(t *testing.T) {
	cardIndex := 2
	targetIndex := 9
	request := game.ActionRequest{
		Action:      "play_card",
		CardIndex:   &cardIndex,
		TargetIndex: &targetIndex,
	}

	expectedState := &game.StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"hand": []any{
				map[string]any{
					"index":              2,
					"playable":           true,
					"requiresTarget":     true,
					"validTargetIndices": []any{0, 1},
				},
			},
			"enemies": []any{
				map[string]any{"index": 0, "isHittable": true},
				map[string]any{"index": 1, "isHittable": true},
			},
		},
	}

	liveState := &game.StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"hand": []any{
				map[string]any{
					"index":              2,
					"playable":           true,
					"requiresTarget":     true,
					"validTargetIndices": []any{1},
				},
			},
			"enemies": []any{
				map[string]any{"index": 1, "isHittable": true},
			},
		},
	}

	remapped, recoveryKind, ok := remapActionRequestForLiveState(expectedState, liveState, request)
	if !ok {
		t.Fatal("expected single-target request to normalize")
	}
	if recoveryKind != "request_normalize" {
		t.Fatalf("expected request_normalize, got %q", recoveryKind)
	}
	if remapped.TargetIndex == nil || *remapped.TargetIndex != 1 {
		t.Fatalf("expected target index to normalize to 1, got %+v", remapped)
	}
}

func TestReadActionableStateStabilizesAlreadyActionableCombatState(t *testing.T) {
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
						map[string]any{"index": 0, "name": "Strike", "playable": true},
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

	session := &Session{
		runtime: &Runtime{
			Client: game.NewClient(server.URL),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	state, err := session.readActionableState(ctx)
	if err != nil {
		t.Fatalf("readActionableState returned error: %v", err)
	}
	if state == nil {
		t.Fatal("readActionableState returned nil state")
	}
	if state.Screen != "REWARD" {
		t.Fatalf("expected stabilized reward state, got %s", state.Screen)
	}

	mu.Lock()
	defer mu.Unlock()
	if stateReads < 2 {
		t.Fatalf("expected stabilization to reread /state, got %d reads", stateReads)
	}
}

func TestReadActionableStateWaitsWhenInitialCombatFrameHasGhostActions(t *testing.T) {
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

			state := &game.StateSnapshot{
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
						map[string]any{"index": 0, "name": "Strike", "playable": true},
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

	session := &Session{
		runtime: &Runtime{
			Client: game.NewClient(server.URL),
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	state, err := session.readActionableState(ctx)
	if err != nil {
		t.Fatalf("readActionableState returned error: %v", err)
	}
	if state == nil {
		t.Fatal("readActionableState returned nil state")
	}
	if state.Screen != "REWARD" {
		t.Fatalf("expected reward state after ghost combat frame, got %s", state.Screen)
	}

	mu.Lock()
	defer mu.Unlock()
	if stateReads < 2 {
		t.Fatalf("expected readActionableState to keep waiting past ghost combat frame, got %d reads", stateReads)
	}
}

func TestChooseStructuredFallbackDecisionUsesLocalPolicy(t *testing.T) {
	t.Parallel()

	session := &Session{
		maxAttempts: 3,
		failures:    newActionFailureMemory(),
	}

	state := &game.StateSnapshot{
		RunID:            "RUN-FALLBACK",
		Screen:           "REWARD",
		AvailableActions: []string{"choose_reward_card", "skip_reward_cards"},
		Run: map[string]any{
			"floor":     2,
			"currentHp": 70,
			"maxHp":     80,
		},
		Reward: map[string]any{
			"pendingCardChoice": true,
			"cardOptions": []any{
				map[string]any{"index": 0, "cardId": "IRON_WAVE", "name": "Iron Wave"},
				map[string]any{"index": 1, "cardId": "ARMAMENTS", "name": "Armaments"},
				map[string]any{"index": 2, "cardId": "SHRUG_IT_OFF", "name": "Shrug It Off"},
			},
		},
	}

	decision, reason, ok := session.chooseStructuredFallbackDecision(state)
	if !ok {
		t.Fatal("expected local fallback decision")
	}
	if reason != "deterministic_action" {
		t.Fatalf("expected deterministic_action fallback reason, got %q", reason)
	}
	if decision == nil || decision.Action != "choose_reward_card" {
		t.Fatalf("expected choose_reward_card fallback, got %+v", decision)
	}
	if decision.OptionIndex == nil || *decision.OptionIndex != 2 {
		t.Fatalf("expected Shrug It Off at index 2, got %+v", decision)
	}
}

func TestChooseActionUsesPlannerFallbackDuringProviderFallback(t *testing.T) {
	state := &game.StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"player": map[string]any{
				"currentHp": 40,
				"maxHp":     80,
				"block":     0,
				"energy":    3,
			},
			"hand": []any{
				map[string]any{
					"index":              1,
					"cardId":             "STRIKE_IRONCLAD",
					"name":               "Strike",
					"energyCost":         1,
					"playable":           true,
					"requiresTarget":     true,
					"validTargetIndices": []any{0},
				},
			},
			"enemies": []any{
				map[string]any{
					"index":      0,
					"enemyId":    "SLIME",
					"name":       "Slime",
					"isAlive":    true,
					"isHittable": true,
					"currentHp":  12,
				},
			},
		},
	}

	session := &Session{
		runtime:       &Runtime{Agent: &openagent.Agent{}},
		planner:       fakeCombatPlanner{plan: &CombatPlan{Mode: "mcts", Candidates: []CombatPlanCandidate{{Action: "play_card", Label: "play [1] Strike -> [0] Slime", CardIndex: intPointer(1), TargetIndex: intPointer(0), Score: 9.5}}}},
		providerState: "fallback",
		failures:      newActionFailureMemory(),
	}

	request, reason, ok := session.chooseAction(state)
	if !ok {
		t.Fatal("expected planner fallback action")
	}
	if request.Action != "play_card" || request.CardIndex == nil || *request.CardIndex != 1 || request.TargetIndex == nil || *request.TargetIndex != 0 {
		t.Fatalf("unexpected planner fallback request: %+v", request)
	}
	if !strings.Contains(reason, "planner-backed deterministic combat action") {
		t.Fatalf("expected planner fallback reason, got %q", reason)
	}
}

func TestChooseActionUsesPlannerDirectActionForSimpleHighConfidenceCombat(t *testing.T) {
	state := &game.StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"player": map[string]any{
				"currentHp": 40,
				"maxHp":     80,
				"block":     0,
				"energy":    3,
			},
			"hand": []any{
				map[string]any{
					"index":              1,
					"cardId":             "STRIKE_IRONCLAD",
					"name":               "Strike",
					"energyCost":         1,
					"playable":           true,
					"requiresTarget":     true,
					"validTargetIndices": []any{0},
				},
			},
			"enemies": []any{
				map[string]any{
					"index":      0,
					"enemyId":    "SLIME",
					"name":       "Slime",
					"isAlive":    true,
					"isHittable": true,
					"currentHp":  12,
				},
			},
		},
	}

	session := &Session{
		runtime: &Runtime{Agent: &openagent.Agent{}},
		planner: fakeCombatPlanner{plan: &CombatPlan{Mode: "mcts", Candidates: []CombatPlanCandidate{
			{Action: "play_card", Label: "play [1] Strike -> [0] Slime", CardIndex: intPointer(1), TargetIndex: intPointer(0), Score: 18.0},
			{Action: "end_turn", Label: "end_turn", Score: 2.0},
		}}},
		providerState: "healthy",
		failures:      newActionFailureMemory(),
	}

	request, reason, ok := session.chooseAction(state)
	if !ok {
		t.Fatal("expected planner direct action")
	}
	if request.Action != "play_card" || request.CardIndex == nil || *request.CardIndex != 1 || request.TargetIndex == nil || *request.TargetIndex != 0 {
		t.Fatalf("unexpected planner direct request: %+v", request)
	}
	if !strings.Contains(reason, "planner-selected combat action") {
		t.Fatalf("expected planner-selected reason, got %q", reason)
	}
}

func TestChooseActionUsesPlannerDirectActionForSingleEnemyCombat(t *testing.T) {
	state := &game.StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"player": map[string]any{
				"currentHp": 40,
				"maxHp":     80,
				"block":     0,
				"energy":    2,
			},
			"hand": []any{
				map[string]any{
					"index":              0,
					"cardId":             "STRIKE_IRONCLAD",
					"name":               "Strike",
					"energyCost":         1,
					"playable":           true,
					"requiresTarget":     true,
					"validTargetIndices": []any{0},
				},
				map[string]any{
					"index":              1,
					"cardId":             "BASH",
					"name":               "Bash",
					"energyCost":         2,
					"playable":           true,
					"requiresTarget":     true,
					"validTargetIndices": []any{0},
				},
			},
			"enemies": []any{
				map[string]any{
					"index":      0,
					"enemyId":    "SLIME",
					"name":       "Slime",
					"isAlive":    true,
					"isHittable": true,
					"currentHp":  18,
				},
			},
		},
	}

	session := &Session{
		runtime: &Runtime{Agent: &openagent.Agent{}},
		planner: fakeCombatPlanner{plan: &CombatPlan{Mode: "mcts", Candidates: []CombatPlanCandidate{
			{Action: "play_card", Label: "play [1] Bash -> [0] Slime", CardIndex: intPointer(1), TargetIndex: intPointer(0), Score: 9.1},
			{Action: "play_card", Label: "play [0] Strike -> [0] Slime", CardIndex: intPointer(0), TargetIndex: intPointer(0), Score: 8.9},
		}}},
		providerState: "healthy",
		failures:      newActionFailureMemory(),
	}

	request, reason, ok := session.chooseAction(state)
	if !ok {
		t.Fatal("expected planner direct action for single-enemy combat")
	}
	if request.Action != "play_card" || request.CardIndex == nil || *request.CardIndex != 1 || request.TargetIndex == nil || *request.TargetIndex != 0 {
		t.Fatalf("unexpected planner direct request: %+v", request)
	}
	if !strings.Contains(reason, "planner-selected combat action") {
		t.Fatalf("expected planner-selected reason, got %q", reason)
	}
}

func TestChooseActionDefersCombatToModelWhenProviderHealthyAndPlannerIsAmbiguous(t *testing.T) {
	state := &game.StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"player": map[string]any{
				"currentHp": 40,
				"maxHp":     80,
				"block":     0,
				"energy":    3,
			},
			"hand": []any{
				map[string]any{
					"index":              1,
					"cardId":             "STRIKE_IRONCLAD",
					"name":               "Strike",
					"energyCost":         1,
					"playable":           true,
					"requiresTarget":     true,
					"validTargetIndices": []any{0},
				},
				map[string]any{
					"index":              2,
					"cardId":             "DEFEND_IRONCLAD",
					"name":               "Defend",
					"energyCost":         1,
					"playable":           true,
					"requiresTarget":     false,
					"validTargetIndices": []any{},
				},
			},
			"enemies": []any{
				map[string]any{
					"index":      0,
					"enemyId":    "SLIME",
					"name":       "Slime",
					"isAlive":    true,
					"isHittable": true,
					"currentHp":  12,
				},
				map[string]any{
					"index":      1,
					"enemyId":    "LICE",
					"name":       "Louse",
					"isAlive":    true,
					"isHittable": true,
					"currentHp":  11,
				},
			},
		},
	}

	session := &Session{
		runtime: &Runtime{Agent: &openagent.Agent{}},
		planner: fakeCombatPlanner{plan: &CombatPlan{Mode: "mcts", Candidates: []CombatPlanCandidate{
			{Action: "play_card", Label: "play [1] Strike -> [0] Slime", CardIndex: intPointer(1), TargetIndex: intPointer(0), Score: 10.0},
			{Action: "play_card", Label: "play [2] Defend", CardIndex: intPointer(2), Score: 8.8},
		}}},
		providerState: "healthy",
		failures:      newActionFailureMemory(),
	}

	request, reason, ok := session.chooseAction(state)
	if ok {
		t.Fatalf("expected healthy provider combat to defer to the model, got request=%+v reason=%q", request, reason)
	}
}

func TestInitialSessionProviderStateUsesRuntimeAvailability(t *testing.T) {
	cfg := config.Config{ModelProvider: config.ModelProviderClaudeCLI, ClaudeCLIPath: "claude.exe", Model: "claude-sonnet-4-6"}

	if got := initialSessionProviderState(cfg, &Runtime{}); got != "deterministic" {
		t.Fatalf("expected deterministic when runtime agent is unavailable, got %q", got)
	}
	if got := initialSessionProviderState(cfg, &Runtime{Agent: &openagent.Agent{}}); got != "healthy" {
		t.Fatalf("expected healthy when runtime agent exists, got %q", got)
	}
}

func TestInitialSessionProviderStateTreatsAnonymousLoopbackAPIAsHealthy(t *testing.T) {
	cfg := config.Config{
		ModelProvider: config.ModelProviderAPI,
		APIBaseURL:    "http://127.0.0.1:11434",
		Model:         "qwen3:8b",
	}

	if got := initialSessionProviderState(cfg, &Runtime{Agent: &openagent.Agent{}}); got != "healthy" {
		t.Fatalf("expected healthy for anonymous loopback api, got %q", got)
	}
}

func TestChooseActionSkipsPlannerDirectWhenForceModelEvalEnabled(t *testing.T) {
	state := &game.StateSnapshot{
		Screen:           "COMBAT",
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"player": map[string]any{
				"currentHp": 40,
				"maxHp":     80,
				"block":     0,
				"energy":    2,
			},
			"hand": []any{
				map[string]any{
					"index":              0,
					"cardId":             "STRIKE_IRONCLAD",
					"name":               "Strike",
					"energyCost":         1,
					"playable":           true,
					"requiresTarget":     true,
					"validTargetIndices": []any{0},
				},
				map[string]any{
					"index":              1,
					"cardId":             "BASH",
					"name":               "Bash",
					"energyCost":         2,
					"playable":           true,
					"requiresTarget":     true,
					"validTargetIndices": []any{0},
				},
			},
			"enemies": []any{
				map[string]any{
					"index":      0,
					"enemyId":    "SLIME",
					"name":       "Slime",
					"isAlive":    true,
					"isHittable": true,
					"currentHp":  18,
				},
			},
		},
	}

	session := &Session{
		cfg:     config.Config{ForceModelEval: true},
		runtime: &Runtime{Agent: &openagent.Agent{}},
		planner: fakeCombatPlanner{plan: &CombatPlan{Mode: "mcts", Candidates: []CombatPlanCandidate{
			{Action: "play_card", Label: "play [1] Bash -> [0] Slime", CardIndex: intPointer(1), TargetIndex: intPointer(0), Score: 9.1},
			{Action: "play_card", Label: "play [0] Strike -> [0] Slime", CardIndex: intPointer(0), TargetIndex: intPointer(0), Score: 8.9},
		}}},
		providerState: "healthy",
		failures:      newActionFailureMemory(),
	}

	request, reason, ok := session.chooseAction(state)
	if ok {
		t.Fatalf("expected force-model-eval to defer combat to model, got request=%+v reason=%q", request, reason)
	}
}

type fakeCombatPlanner struct {
	plan *CombatPlan
}

func (f fakeCombatPlanner) Name() string { return "fake" }

func (f fakeCombatPlanner) Analyze(_ *game.StateSnapshot, _ *SeenContentRegistry, _ i18n.Language) *CombatPlan {
	return f.plan
}

func TestExecuteModelDecisionStabilizesRewardTransitionBeforeActing(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	stateReads := 0
	actionCalls := 0

	expectedState := &game.StateSnapshot{
		StateVersion:     1,
		RunID:            "run_test",
		Screen:           "REWARD",
		AvailableActions: []string{"claim_reward"},
		Reward: map[string]any{
			"phase":        "claim",
			"sourceScreen": "COMBAT",
			"sourceHint":   "combat",
			"rewards": []any{
				map[string]any{
					"index":       0,
					"rewardType":  "gold",
					"description": "gain gold",
					"claimable":   true,
				},
			},
		},
	}

	settlingState := &game.StateSnapshot{
		StateVersion:     1,
		RunID:            "run_test",
		Screen:           "REWARD",
		AvailableActions: []string{"claim_reward"},
		Reward: map[string]any{
			"phase":        "settling",
			"sourceScreen": "COMBAT",
			"sourceHint":   "combat",
			"rewards": []any{
				map[string]any{
					"index":       0,
					"rewardType":  "gold",
					"description": "gain gold",
					"claimable":   true,
				},
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

			state := expectedState
			if readCount == 1 {
				state = settlingState
			}
			writeSessionEnvelope(t, w, state)
		case "/action":
			var request game.ActionRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			mu.Lock()
			actionCalls++
			mu.Unlock()
			if request.Action != "claim_reward" {
				t.Fatalf("expected claim_reward, got %#v", request)
			}
			if request.OptionIndex == nil || *request.OptionIndex != 0 {
				t.Fatalf("expected option_index 0, got %#v", request)
			}
			writeSessionEnvelope(t, w, game.ActionResult{
				Action:  request.Action,
				Status:  "ok",
				Stable:  true,
				Message: "claimed",
				State: game.StateSnapshot{
					StateVersion:     1,
					RunID:            "run_test",
					Screen:           "REWARD",
					AvailableActions: []string{"proceed"},
					Reward: map[string]any{
						"phase":        "claim",
						"sourceScreen": "COMBAT",
						"sourceHint":   "combat",
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	store, err := NewRunStore(filepath.Join(t.TempDir(), "artifacts"), "session-test")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	session := &Session{
		runtime: &Runtime{
			Client: game.NewClient(server.URL),
		},
		store:   store,
		events:  make(chan SessionEvent, 8),
		todo:    NewTodoManager(),
		compact: NewCompactMemory(),
	}

	optionIndex := 0
	decision := &ActionDecision{
		Action:      "claim_reward",
		OptionIndex: &optionIndex,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := session.executeModelDecision(ctx, expectedState, decision); err != nil {
		t.Fatalf("executeModelDecision returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if stateReads < 3 {
		t.Fatalf("expected stabilization to reread reward state, got %d reads", stateReads)
	}
	if actionCalls != 1 {
		t.Fatalf("expected exactly one action call, got %d", actionCalls)
	}
}

func TestExecuteDirectActionSettlesPendingRewardTransitionBeforeRepeating(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	actionCalls := 0

	rewardState := game.StateSnapshot{
		StateVersion:     1,
		RunID:            "run_test",
		Screen:           "REWARD",
		AvailableActions: []string{"claim_reward"},
		Reward: map[string]any{
			"phase":        "claim",
			"sourceScreen": "COMBAT",
			"sourceHint":   "combat",
			"rewards": []any{
				map[string]any{
					"index":       0,
					"rewardType":  "card",
					"description": "card reward",
					"claimable":   true,
				},
			},
		},
	}

	selectionState := game.StateSnapshot{
		StateVersion:     1,
		RunID:            "run_test",
		Screen:           "CARD_SELECTION",
		AvailableActions: []string{"choose_reward_card", "skip_reward_cards"},
		Selection: map[string]any{
			"kind": "reward_cards",
			"cards": []any{
				map[string]any{
					"index": 0,
					"id":    "IRON_WAVE",
					"name":  "Iron Wave",
				},
			},
		},
		Reward: map[string]any{
			"phase":             "card_choice",
			"pendingCardChoice": true,
			"cardOptions": []any{
				map[string]any{
					"index":  0,
					"cardId": "IRON_WAVE",
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/state":
			mu.Lock()
			calls := actionCalls
			mu.Unlock()
			if calls == 0 {
				writeSessionEnvelope(t, w, rewardState)
				return
			}
			writeSessionEnvelope(t, w, selectionState)
		case "/action":
			mu.Lock()
			actionCalls++
			mu.Unlock()
			writeSessionEnvelope(t, w, game.ActionResult{
				Action:  "claim_reward",
				Status:  "pending",
				Stable:  false,
				Message: "Action queued but state is still transitioning.",
				State:   rewardState,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	store, err := NewRunStore(t.TempDir(), "session-test")
	if err != nil {
		t.Fatalf("NewRunStore returned error: %v", err)
	}
	defer func() { _ = store.Close() }()

	session := &Session{
		runtime: &Runtime{
			Client: game.NewClient(server.URL),
		},
		store:    store,
		events:   make(chan SessionEvent, 8),
		todo:     NewTodoManager(),
		compact:  NewCompactMemory(),
		failures: newActionFailureMemory(),
	}

	optionIndex := 0
	request := game.ActionRequest{
		Action:      "claim_reward",
		OptionIndex: &optionIndex,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := session.executeDirectAction(ctx, &rewardState, request); err != nil {
		t.Fatalf("executeDirectAction returned error: %v", err)
	}

	mu.Lock()
	if actionCalls != 1 {
		t.Fatalf("expected a single bridge action call, got %d", actionCalls)
	}
	mu.Unlock()

	select {
	case event := <-session.events:
		if event.Kind != SessionEventAction {
			t.Fatalf("expected action event, got %s", event.Kind)
		}
		if event.Screen != "CARD_SELECTION" {
			t.Fatalf("expected settled card selection state, got %s", event.Screen)
		}
		if got := fmt.Sprint(event.Data["stable"]); got != "true" {
			t.Fatalf("expected settled action to be marked stable, got %q", got)
		}
	default:
		t.Fatal("expected action event")
	}
}

func TestExecuteModelDecisionWaitsThroughActionWindowChangeBeforeActing(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	stateReads := 0
	actionCalls := 0

	expectedState := &game.StateSnapshot{
		StateVersion:     1,
		RunID:            "run_test",
		Screen:           "COMBAT",
		InCombat:         true,
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"actionWindowOpen": true,
			"player": map[string]any{
				"energy": 1,
			},
			"enemies": []any{
				map[string]any{
					"index":      0,
					"enemyId":    "slime-a",
					"name":       "Slime A",
					"isHittable": true,
					"currentHp":  6,
				},
			},
			"hand": []any{
				map[string]any{
					"index":              0,
					"cardId":             "STRIKE_IRONCLAD",
					"name":               "Strike",
					"playable":           true,
					"requiresTarget":     true,
					"validTargetIndices": []any{0},
				},
			},
		},
	}

	transientState := &game.StateSnapshot{
		StateVersion:     1,
		RunID:            "run_test",
		Screen:           "COMBAT",
		InCombat:         true,
		AvailableActions: []string{},
		Combat: map[string]any{
			"actionWindowOpen": false,
			"player": map[string]any{
				"energy": 0,
			},
			"enemies": []any{
				map[string]any{
					"index":      0,
					"enemyId":    "slime-a",
					"name":       "Slime A",
					"isHittable": true,
					"currentHp":  6,
				},
			},
			"hand": []any{
				map[string]any{
					"index":              0,
					"cardId":             "STRIKE_IRONCLAD",
					"name":               "Strike",
					"playable":           false,
					"requiresTarget":     true,
					"validTargetIndices": []any{},
				},
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

			state := expectedState
			if readCount <= 2 {
				state = transientState
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
		case "/action":
			var request game.ActionRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			mu.Lock()
			actionCalls++
			mu.Unlock()

			if request.Action != "play_card" {
				t.Fatalf("expected play_card, got %#v", request)
			}
			if request.CardIndex == nil || *request.CardIndex != 0 {
				t.Fatalf("expected card_index 0, got %#v", request)
			}
			if request.TargetIndex == nil || *request.TargetIndex != 0 {
				t.Fatalf("expected target_index 0, got %#v", request)
			}

			writeSessionEnvelope(t, w, game.ActionResult{
				Action:  request.Action,
				Status:  "ok",
				Stable:  true,
				Message: "played",
				State:   *expectedState,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	store, err := NewRunStore(filepath.Join(t.TempDir(), "artifacts"), "session-test")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	session := &Session{
		runtime: &Runtime{
			Client: game.NewClient(server.URL),
		},
		store:   store,
		events:  make(chan SessionEvent, 8),
		todo:    NewTodoManager(),
		compact: NewCompactMemory(),
	}

	cardIndex := 0
	targetIndex := 0
	decision := &ActionDecision{
		Action:      "play_card",
		CardIndex:   &cardIndex,
		TargetIndex: &targetIndex,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := session.executeModelDecision(ctx, expectedState, decision); err != nil {
		t.Fatalf("executeModelDecision returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if stateReads < 3 {
		t.Fatalf("expected execution stabilization to wait past transient combat frame, got %d reads", stateReads)
	}
	if actionCalls != 1 {
		t.Fatalf("expected exactly one action call, got %d", actionCalls)
	}
}

func TestExecuteDirectActionBlocksPendingRewardClaimWithoutStateProgress(t *testing.T) {
	t.Parallel()

	state := &game.StateSnapshot{
		StateVersion:     1,
		RunID:            "run_test",
		Screen:           "REWARD",
		AvailableActions: []string{"claim_reward"},
		Reward: map[string]any{
			"phase": "claim",
			"rewards": []any{
				map[string]any{
					"index":       0,
					"rewardType":  "PotionReward",
					"description": "Potion",
					"claimable":   true,
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/state":
			writeSessionEnvelope(t, w, state)
		case "/action":
			var request game.ActionRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			writeSessionEnvelope(t, w, game.ActionResult{
				Action:  request.Action,
				Status:  "pending",
				Stable:  false,
				Message: "Action queued but state is still transitioning.",
				State:   *state,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	store, err := NewRunStore(filepath.Join(t.TempDir(), "artifacts"), "session-test")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	session := &Session{
		runtime: &Runtime{
			Client: game.NewClient(server.URL),
		},
		store:    store,
		events:   make(chan SessionEvent, 8),
		todo:     NewTodoManager(),
		compact:  NewCompactMemory(),
		failures: newActionFailureMemory(),
	}

	optionIndex := 0
	request := game.ActionRequest{
		Action:      "claim_reward",
		OptionIndex: &optionIndex,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := session.executeDirectAction(ctx, state, request); err != nil {
		t.Fatalf("executeDirectAction returned error: %v", err)
	}

	if session.failures == nil || session.failures.Allows(digestState(state), request) {
		t.Fatal("expected stalled reward claim to be blocked for the same state digest")
	}
}

func TestExecuteDirectActionRecoversFromDroppedActionResponseWhenStateAdvanced(t *testing.T) {
	t.Parallel()

	rewardState := &game.StateSnapshot{
		StateVersion:     1,
		RunID:            "run_test",
		Screen:           "REWARD",
		AvailableActions: []string{"claim_reward"},
		Reward: map[string]any{
			"phase": "claim",
			"rewards": []any{
				map[string]any{
					"index":       0,
					"rewardType":  "PotionReward",
					"description": "Potion",
					"claimable":   true,
				},
			},
		},
	}
	mapState := &game.StateSnapshot{
		StateVersion:     1,
		RunID:            "run_test",
		Screen:           "MAP",
		AvailableActions: []string{"choose_map_node"},
		Map: map[string]any{
			"availableNodes": []any{
				map[string]any{"index": 0, "nodeType": "combat"},
			},
		},
	}

	var mu sync.Mutex
	actionCalls := 0

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/state":
			mu.Lock()
			calls := actionCalls
			mu.Unlock()
			if calls == 0 {
				writeSessionEnvelope(t, w, rewardState)
				return
			}
			writeSessionEnvelope(t, w, mapState)
		case "/action":
			mu.Lock()
			actionCalls++
			mu.Unlock()
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("response writer does not support hijacking")
			}
			conn, _, err := hijacker.Hijack()
			if err != nil {
				t.Fatalf("hijack action response: %v", err)
			}
			_ = conn.Close()
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	store, err := NewRunStore(filepath.Join(t.TempDir(), "artifacts"), "session-test")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	session := &Session{
		runtime: &Runtime{
			Client: game.NewClient(server.URL),
		},
		store:    store,
		events:   make(chan SessionEvent, 8),
		todo:     NewTodoManager(),
		compact:  NewCompactMemory(),
		failures: newActionFailureMemory(),
	}

	optionIndex := 0
	request := game.ActionRequest{Action: "claim_reward", OptionIndex: &optionIndex}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := session.executeDirectAction(ctx, rewardState, request); err != nil {
		t.Fatalf("executeDirectAction returned error: %v", err)
	}

	select {
	case event := <-session.events:
		if event.Kind != SessionEventStatus {
			t.Fatalf("expected transport recovery status event, got %s", event.Kind)
		}
	case <-time.After(time.Second):
		t.Fatal("expected transport recovery status event")
	}
	select {
	case event := <-session.events:
		if event.Kind != SessionEventAction {
			t.Fatalf("expected action event after transport recovery, got %s", event.Kind)
		}
		if event.Screen != "MAP" {
			t.Fatalf("expected recovered action to land on MAP, got %s", event.Screen)
		}
	case <-time.After(time.Second):
		t.Fatal("expected action event after transport recovery")
	}
}

func TestHandleDirectActionFailureTreatsContinueAfterGameOverInvalidActionAsSoftReplan(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/state":
			writeSessionEnvelope(t, w, &game.StateSnapshot{
				StateVersion:     1,
				RunID:            "run_test",
				Screen:           "GAME_OVER",
				AvailableActions: []string{"return_to_main_menu"},
				GameOver: map[string]any{
					"stage": "summary",
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	store, err := NewRunStore(t.TempDir(), "session-test")
	if err != nil {
		t.Fatalf("NewRunStore returned error: %v", err)
	}
	defer func() { _ = store.Close() }()

	session := &Session{
		runtime: &Runtime{
			Client: game.NewClient(server.URL),
		},
		store:  store,
		events: make(chan SessionEvent, 4),
	}

	state := &game.StateSnapshot{
		StateVersion:     1,
		RunID:            "run_test",
		Screen:           "GAME_OVER",
		AvailableActions: []string{"continue_after_game_over"},
		GameOver: map[string]any{
			"stage": "results",
		},
	}
	request := game.ActionRequest{Action: "continue_after_game_over"}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	ok := session.handleDirectActionFailure(ctx, fmt.Errorf("invalid_action: Action is not available in the current state."), state, request)
	if !ok {
		t.Fatal("expected continue_after_game_over invalid_action to be recovered")
	}
	if session.repeatedInvalidActions != 0 {
		t.Fatalf("expected repeatedInvalidActions to remain 0, got %d", session.repeatedInvalidActions)
	}

	select {
	case event := <-session.events:
		if event.Kind != SessionEventStatus {
			t.Fatalf("expected status event, got %s", event.Kind)
		}
		if got := fmt.Sprint(event.Data["recovery_kind"]); got != "soft_replan" {
			t.Fatalf("expected soft_replan recovery, got %q", got)
		}
		if got := fmt.Sprint(event.Data["drift_kind"]); got != driftKindActionWindowChanged {
			t.Fatalf("expected action_window_changed drift, got %q", got)
		}
	default:
		t.Fatal("expected recovery status event")
	}
}

func TestHandleDirectActionFailureTreatsRewardClaimInvalidActionAsSoftReplan(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/state":
			writeSessionEnvelope(t, w, &game.StateSnapshot{
				StateVersion:     1,
				RunID:            "run_test",
				Screen:           "CARD_SELECTION",
				AvailableActions: []string{"choose_reward_card", "skip_reward_cards"},
				Selection: map[string]any{
					"kind": "reward_cards",
				},
				Reward: map[string]any{
					"phase":             "card_choice",
					"pendingCardChoice": true,
					"cardOptions": []any{
						map[string]any{
							"index":  0,
							"cardId": "IRON_WAVE",
						},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	store, err := NewRunStore(t.TempDir(), "session-test")
	if err != nil {
		t.Fatalf("NewRunStore returned error: %v", err)
	}
	defer func() { _ = store.Close() }()

	session := &Session{
		runtime: &Runtime{
			Client: game.NewClient(server.URL),
		},
		store:  store,
		events: make(chan SessionEvent, 4),
	}

	optionIndex := 0
	state := &game.StateSnapshot{
		StateVersion:     1,
		RunID:            "run_test",
		Screen:           "REWARD",
		AvailableActions: []string{"claim_reward"},
		Reward: map[string]any{
			"phase": "claim",
			"rewards": []any{
				map[string]any{
					"index":       0,
					"rewardType":  "CardReward",
					"description": "choose a card",
					"claimable":   true,
				},
			},
		},
	}
	request := game.ActionRequest{Action: "claim_reward", OptionIndex: &optionIndex}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	ok := session.handleDirectActionFailure(ctx, fmt.Errorf("invalid_action: Action is not available in the current state."), state, request)
	if !ok {
		t.Fatal("expected reward claim invalid_action to be recovered")
	}
	if session.repeatedInvalidActions != 0 {
		t.Fatalf("expected repeatedInvalidActions to remain 0, got %d", session.repeatedInvalidActions)
	}

	select {
	case event := <-session.events:
		if event.Kind != SessionEventStatus {
			t.Fatalf("expected status event, got %s", event.Kind)
		}
		if got := fmt.Sprint(event.Data["recovery_kind"]); got != "soft_replan" {
			t.Fatalf("expected soft_replan recovery, got %q", got)
		}
		if got := fmt.Sprint(event.Data["drift_kind"]); got != driftKindSelectionSeam {
			t.Fatalf("expected selection_seam drift, got %q", got)
		}
	default:
		t.Fatal("expected recovery status event")
	}
}

func TestExecuteDirectActionStabilizesSameScreenIndexDriftBeforeNormalizing(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	stateReads := 0
	actionCalls := 0

	expectedState := &game.StateSnapshot{
		StateVersion:     1,
		RunID:            "run_test",
		Screen:           "COMBAT",
		InCombat:         true,
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"actionWindowOpen": true,
			"player": map[string]any{
				"energy": 3,
			},
			"enemies": []any{
				map[string]any{
					"index":      0,
					"enemyId":    "slime-a",
					"name":       "Slime A",
					"isHittable": true,
					"currentHp":  12,
				},
				map[string]any{
					"index":      1,
					"enemyId":    "slime-b",
					"name":       "Slime B",
					"isHittable": true,
					"currentHp":  12,
				},
			},
			"hand": []any{
				map[string]any{
					"index":              0,
					"cardId":             "STRIKE_IRONCLAD",
					"name":               "Strike",
					"playable":           true,
					"requiresTarget":     true,
					"validTargetIndices": []any{0, 1},
				},
			},
		},
	}

	driftedState := &game.StateSnapshot{
		StateVersion:     1,
		RunID:            "run_test",
		Screen:           "COMBAT",
		InCombat:         true,
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"actionWindowOpen": true,
			"player": map[string]any{
				"energy": 3,
			},
			"enemies": []any{
				map[string]any{
					"index":      0,
					"enemyId":    "slime-a",
					"name":       "Slime A",
					"isHittable": true,
					"currentHp":  12,
				},
				map[string]any{
					"index":      1,
					"enemyId":    "slime-b",
					"name":       "Slime B",
					"isHittable": true,
					"currentHp":  12,
				},
			},
			"hand": []any{
				map[string]any{
					"index":              0,
					"cardId":             "STRIKE_IRONCLAD",
					"name":               "Strike",
					"playable":           true,
					"requiresTarget":     true,
					"validTargetIndices": []any{1},
				},
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

			state := expectedState
			if readCount == 1 {
				state = driftedState
			}
			writeSessionEnvelope(t, w, state)
		case "/action":
			var request game.ActionRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			mu.Lock()
			actionCalls++
			mu.Unlock()

			if request.Action != "play_card" {
				t.Fatalf("expected play_card, got %#v", request)
			}
			if request.CardIndex == nil || *request.CardIndex != 0 {
				t.Fatalf("expected card_index 0, got %#v", request)
			}
			if request.TargetIndex == nil || *request.TargetIndex != 0 {
				actualTarget := "<nil>"
				if request.TargetIndex != nil {
					actualTarget = fmt.Sprintf("%d", *request.TargetIndex)
				}
				t.Fatalf("expected original target_index 0 after stabilization, got target_index=%s request=%#v", actualTarget, request)
			}

			writeSessionEnvelope(t, w, game.ActionResult{
				Action:  request.Action,
				Status:  "ok",
				Stable:  true,
				Message: "played",
				State:   *expectedState,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	store, err := NewRunStore(filepath.Join(t.TempDir(), "artifacts"), "session-test")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	session := &Session{
		runtime: &Runtime{
			Client: game.NewClient(server.URL),
		},
		store:   store,
		events:  make(chan SessionEvent, 8),
		todo:    NewTodoManager(),
		compact: NewCompactMemory(),
	}

	cardIndex := 0
	targetIndex := 0
	request := game.ActionRequest{
		Action:      "play_card",
		CardIndex:   &cardIndex,
		TargetIndex: &targetIndex,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := session.executeDirectAction(ctx, expectedState, request); err != nil {
		t.Fatalf("executeDirectAction returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if stateReads < 3 {
		t.Fatalf("expected stabilization to reread combat state, got %d reads", stateReads)
	}
	if actionCalls != 1 {
		t.Fatalf("expected exactly one action call, got %d", actionCalls)
	}
}

func TestExecuteDirectActionRejectsTargetedPlayCardWithoutTargetBeforeBridgeCall(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	actionCalls := 0

	expectedState := &game.StateSnapshot{
		StateVersion:     1,
		RunID:            "run_test",
		Screen:           "COMBAT",
		InCombat:         true,
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"actionWindowOpen": true,
			"player": map[string]any{
				"energy": 1,
			},
			"enemies": []any{
				map[string]any{
					"index":      0,
					"enemyId":    "slime-a",
					"name":       "Slime A",
					"isHittable": true,
					"currentHp":  12,
				},
				map[string]any{
					"index":      1,
					"enemyId":    "slime-b",
					"name":       "Slime B",
					"isHittable": true,
					"currentHp":  12,
				},
			},
			"hand": []any{
				map[string]any{
					"index":              0,
					"cardId":             "STRIKE_IRONCLAD",
					"name":               "Strike",
					"playable":           true,
					"requiresTarget":     true,
					"validTargetIndices": []any{0, 1},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/state":
			writeSessionEnvelope(t, w, expectedState)
		case "/action":
			mu.Lock()
			actionCalls++
			mu.Unlock()
			t.Fatalf("bridge /action should not be called for an invalid targeted play_card request")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	store, err := NewRunStore(filepath.Join(t.TempDir(), "artifacts"), "session-test")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	session := &Session{
		runtime: &Runtime{
			Client: game.NewClient(server.URL),
		},
		store:   store,
		events:  make(chan SessionEvent, 8),
		todo:    NewTodoManager(),
		compact: NewCompactMemory(),
	}

	cardIndex := 0
	request := game.ActionRequest{
		Action:    "play_card",
		CardIndex: &cardIndex,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err = session.executeDirectAction(ctx, expectedState, request)
	if err == nil {
		t.Fatal("expected executeDirectAction to reject missing target_index")
	}
	if !strings.Contains(err.Error(), "target_index is required") {
		t.Fatalf("expected missing target_index error, got %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if actionCalls != 0 {
		t.Fatalf("expected zero action calls, got %d", actionCalls)
	}
}

func TestExecuteDirectActionFreshensEndTurnStateBeforeActing(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	stateReads := 0
	actionCalls := 0

	edgeState := &game.StateSnapshot{
		StateVersion:     1,
		RunID:            "run_test",
		Screen:           "COMBAT",
		InCombat:         true,
		AvailableActions: []string{"end_turn"},
		Combat: map[string]any{
			"actionWindowOpen": true,
			"player": map[string]any{
				"energy": 0,
			},
			"enemies": []any{
				map[string]any{
					"index":      0,
					"enemyId":    "slime-a",
					"name":       "Slime A",
					"isHittable": true,
					"currentHp":  12,
				},
			},
			"hand": []any{
				map[string]any{
					"index":    0,
					"cardId":   "STRIKE_IRONCLAD",
					"name":     "Strike",
					"playable": false,
				},
			},
		},
	}

	transientState := &game.StateSnapshot{
		StateVersion:     1,
		RunID:            "run_test",
		Screen:           "COMBAT",
		InCombat:         true,
		AvailableActions: []string{},
		Combat: map[string]any{
			"actionWindowOpen":      false,
			"playerActionsDisabled": true,
			"player": map[string]any{
				"energy": 0,
			},
			"enemies": []any{
				map[string]any{
					"index":      0,
					"enemyId":    "slime-a",
					"name":       "Slime A",
					"isHittable": true,
					"currentHp":  12,
				},
			},
			"hand": []any{
				map[string]any{
					"index":    0,
					"cardId":   "STRIKE_IRONCLAD",
					"name":     "Strike",
					"playable": false,
				},
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

			state := edgeState
			switch readCount {
			case 1:
				state = edgeState
			case 2:
				state = transientState
			default:
				state = edgeState
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
		case "/action":
			var request game.ActionRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			mu.Lock()
			actionCalls++
			mu.Unlock()

			if request.Action != "end_turn" {
				t.Fatalf("expected end_turn, got %#v", request)
			}

			writeSessionEnvelope(t, w, game.ActionResult{
				Action:  request.Action,
				Status:  "ok",
				Stable:  true,
				Message: "ended turn",
				State:   *transientState,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	store, err := NewRunStore(filepath.Join(t.TempDir(), "artifacts"), "session-test")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	session := &Session{
		runtime: &Runtime{
			Client: game.NewClient(server.URL),
		},
		store:   store,
		events:  make(chan SessionEvent, 8),
		todo:    NewTodoManager(),
		compact: NewCompactMemory(),
	}

	request := game.ActionRequest{Action: "end_turn"}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	if err := session.executeDirectAction(ctx, edgeState, request); err != nil {
		t.Fatalf("executeDirectAction returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if stateReads < 3 {
		t.Fatalf("expected executeDirectAction to wait through transient end-turn frame, got %d state reads", stateReads)
	}
	if actionCalls != 1 {
		t.Fatalf("expected exactly one action call, got %d", actionCalls)
	}
}

func TestExecuteDirectActionRebindsAndRetriesStaleInvalidAction(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	stateReads := 0
	actionCalls := 0

	expectedState := &game.StateSnapshot{
		StateVersion:     1,
		RunID:            "run_test",
		Screen:           "COMBAT",
		InCombat:         true,
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"actionWindowOpen": true,
			"player":           map[string]any{"energy": 1},
			"enemies": []any{
				map[string]any{"index": 0, "enemyId": "slime-a", "name": "Slime A", "isHittable": true, "currentHp": 12},
			},
			"hand": []any{
				map[string]any{
					"index":              0,
					"cardId":             "STRIKE_IRONCLAD",
					"name":               "Strike",
					"playable":           true,
					"requiresTarget":     true,
					"validTargetIndices": []any{0},
				},
			},
		},
	}

	liveState := &game.StateSnapshot{
		StateVersion:     1,
		RunID:            "run_test",
		Screen:           "COMBAT",
		InCombat:         true,
		AvailableActions: []string{"play_card", "end_turn"},
		Combat: map[string]any{
			"actionWindowOpen": true,
			"player":           map[string]any{"energy": 1},
			"enemies": []any{
				map[string]any{"index": 0, "enemyId": "slime-a", "name": "Slime A", "isHittable": true, "currentHp": 12},
			},
			"hand": []any{
				map[string]any{
					"index":              1,
					"cardId":             "STRIKE_IRONCLAD",
					"name":               "Strike",
					"playable":           true,
					"requiresTarget":     true,
					"validTargetIndices": []any{0},
				},
			},
		},
	}

	postActionState := &game.StateSnapshot{
		StateVersion:     1,
		RunID:            "run_test",
		Screen:           "COMBAT",
		InCombat:         true,
		AvailableActions: []string{"end_turn"},
		Combat: map[string]any{
			"actionWindowOpen": true,
			"player":           map[string]any{"energy": 0},
			"enemies": []any{
				map[string]any{"index": 0, "enemyId": "slime-a", "name": "Slime A", "isHittable": true, "currentHp": 6},
			},
			"hand": []any{},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/state":
			mu.Lock()
			stateReads++
			currentActionCalls := actionCalls
			mu.Unlock()
			if currentActionCalls == 0 {
				writeSessionEnvelope(t, w, expectedState)
				return
			}
			writeSessionEnvelope(t, w, liveState)
		case "/action":
			var request game.ActionRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Fatalf("decode request: %v", err)
			}
			mu.Lock()
			actionCalls++
			call := actionCalls
			mu.Unlock()

			if call == 1 {
				if request.CardIndex == nil {
					t.Fatalf("expected first attempt to include card index, got %#v", request)
				}
				writeSessionErrorEnvelope(t, w, http.StatusOK, "invalid_action", "Action is not available in the current state.")
				return
			}

			if request.CardIndex == nil || *request.CardIndex != 1 {
				t.Fatalf("expected rebound retry to use card index 1, got %#v", request)
			}
			if request.TargetIndex == nil || *request.TargetIndex != 0 {
				t.Fatalf("expected rebound retry to keep target 0, got %#v", request)
			}
			writeSessionEnvelope(t, w, game.ActionResult{
				Action:  request.Action,
				Status:  "ok",
				Stable:  true,
				Message: "Action completed.",
				State:   *postActionState,
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	store, err := NewRunStore(filepath.Join(t.TempDir(), "artifacts"), "session-test")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer store.Close()

	session := &Session{
		runtime: &Runtime{
			Client: game.NewClient(server.URL),
		},
		store:    store,
		events:   make(chan SessionEvent, 8),
		todo:     NewTodoManager(),
		compact:  NewCompactMemory(),
		failures: newActionFailureMemory(),
	}

	cardIndex := 0
	targetIndex := 0
	request := game.ActionRequest{
		Action:      "play_card",
		CardIndex:   &cardIndex,
		TargetIndex: &targetIndex,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := session.executeDirectAction(ctx, expectedState, request); err != nil {
		t.Fatalf("executeDirectAction returned error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if actionCalls != 2 {
		t.Fatalf("expected two action calls after stale invalid_action recovery, got %d", actionCalls)
	}
	if stateReads < 2 {
		t.Fatalf("expected live state rereads during recovery, got %d", stateReads)
	}
}

func writeSessionEnvelope(t *testing.T, w http.ResponseWriter, payload any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(game.Envelope[any]{
		OK:   true,
		Data: payload,
	}); err != nil {
		t.Fatalf("encode envelope: %v", err)
	}
}

func writeSessionErrorEnvelope(t *testing.T, w http.ResponseWriter, status int, code string, message string) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]any{
		"ok": false,
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	}); err != nil {
		t.Fatalf("encode error envelope: %v", err)
	}
}
