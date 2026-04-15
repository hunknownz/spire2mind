package agentruntime

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"spire2mind/internal/analyst"
	"spire2mind/internal/config"
	"spire2mind/internal/game"
	"spire2mind/internal/i18n"
)

const (
	maxStalledCycles             = 8
	maxRepeatedInvalidActions    = 3
	maxProviderFailures          = 3
	stateWaitTimeout             = 90 * time.Second
	bridgeBootstrapWaitTimeout   = 90 * time.Second
	agentContextMessageThreshold = 72
	executionDriftWaitTimeout    = 8 * time.Second
	plannerDirectStrongGap       = 12.0
	plannerDirectSimpleGap       = 6.0
)

type SessionEventKind string

const (
	SessionEventStatus     SessionEventKind = "status"
	SessionEventState      SessionEventKind = "state"
	SessionEventPrompt     SessionEventKind = "prompt"
	SessionEventAssistant  SessionEventKind = "assistant"
	SessionEventStreamer   SessionEventKind = "streamer"
	SessionEventTool       SessionEventKind = "tool"
	SessionEventToolError  SessionEventKind = "tool_error"
	SessionEventAction     SessionEventKind = "action"
	SessionEventCompact    SessionEventKind = "compact"
	SessionEventReflection SessionEventKind = "reflection"
	SessionEventStop       SessionEventKind = "stop"
)

type SessionEvent struct {
	Time    time.Time              `json:"time"`
	Kind    SessionEventKind       `json:"kind"`
	Cycle   int                    `json:"cycle"`
	Attempt int                    `json:"attempt,omitempty"`
	Message string                 `json:"message,omitempty"`
	Screen  string                 `json:"screen,omitempty"`
	RunID   string                 `json:"run_id,omitempty"`
	Action  string                 `json:"action,omitempty"`
	Tool    string                 `json:"tool,omitempty"`
	Cost    float64                `json:"cost,omitempty"`
	Turns   int                    `json:"turns,omitempty"`
	State   *game.StateSnapshot    `json:"state,omitempty"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

type Session struct {
	cfg         config.Config
	runtime     *Runtime
	store       *RunStore
	guide       *GuidebookStore
	catalog     *codexCatalog
	planner     CombatPlanner
	resolver    *ActionResolutionPipeline
	gate        *StableStateGate
	streamer    *StreamerDirector
	llmProvider analyst.LLMProvider

	events chan SessionEvent
	errs   chan error

	todo     *TodoManager
	skills   *SkillLibrary
	compact  *CompactMemory
	failures *actionFailureMemory
	world    *SeenContentTracker

	cycle                  int
	stalledCycles          int
	repeatedInvalidActions int
	providerFailures       int
	forceDeterministic     bool
	lastDigest             string
	maxAttempts            int
	attemptCount           int
	storyAttemptBase       int
	seenRunIDs             map[string]int
	reflectedRunIDs        map[string]bool
	reflections            []*AttemptReflection
	resumeState            *SessionResumeState
	dashboard              *DebugDashboard
	guideSnapshot          *GuidebookSnapshot
	providerState          string
	knowledge              *KnowledgeRetriever
	promptBuilder          *PromptAssemblyPipeline
	playerSkill            *PlayerSkillStore
	gamePrefsPath          string
	gameFastMode           string
	gameFastModeChanged    bool
	gameFastModePrevious   string
	gameFastModeWarning    string
	lastGuideRefresh       time.Time
	streamerHistory        []string
	lastStreamerSignature  string
	pauseMu                sync.Mutex
	paused                 bool
	pauseSignal            chan struct{}
	closeSignal            chan struct{}
	closeOnce              sync.Once
}

func StartSession(ctx context.Context, cfg config.Config) (*Session, error) {
	fastModeStatus, fastModeWarning := prepareGameFastMode(cfg)

	runtime, err := New(ctx, cfg)
	if err != nil {
		return nil, err
	}

	if runtime.SessionID == "" {
		runtime.SessionID = fmt.Sprintf("det-%d", time.Now().UnixNano())
	}

	store, err := NewRunStore(cfg.ArtifactsDir, runtime.SessionID)
	if err != nil {
		runtime.Close()
		return nil, err
	}

	guide, err := NewGuidebookStore(cfg.RepoRoot)
	if err != nil {
		_ = store.Close()
		runtime.Close()
		return nil, err
	}
	catalog, err := loadCodexCatalogFromDataDir(cfg.DataDir)
	if err != nil {
		_ = store.Close()
		runtime.Close()
		return nil, err
	}


		llmProvider, _ := analyst.NewLLMProvider(cfg)

	// Load player skill store (non-fatal)
	playerSkill, _ := NewPlayerSkillStore(cfg.RepoRoot)

	// Load reward weights (non-fatal)
	rewardWeightsPath := filepath.Join(cfg.RepoRoot, "combat", "knowledge", "reward-weights.json")
	rewardWeights, _ := LoadRewardWeights(rewardWeightsPath)
	if rewardWeights != nil {
		globalRewardWeights = rewardWeights
	}

	session := &Session{
		cfg:                  cfg,
		runtime:              runtime,
		store:                store,
		guide:                guide,
		catalog:              catalog,
		planner:              NewCombatPlanner(cfg),
		resolver:             NewActionResolutionPipeline(),
		gate:                 NewStableStateGate(runtime.Client),
		streamer:             NewStreamerDirector(cfg, runtime),
		llmProvider:          llmProvider,
		knowledge:            NewKnowledgeRetriever(cfg.RepoRoot),
		promptBuilder:        NewPromptAssemblyPipeline(),
		playerSkill:          playerSkill,
		events:               make(chan SessionEvent, 128),
		errs:                 make(chan error, 1),
		todo:                 NewTodoManager(),
		skills:               NewSkillLibrary(cfg.RepoRoot),
		compact:              NewCompactMemory(),
		failures:             newActionFailureMemory(),
		world:                NewSeenContentTracker(),
		maxAttempts:          cfg.MaxAttempts,
		seenRunIDs:           make(map[string]int),
		reflectedRunIDs:      make(map[string]bool),
		dashboard:            NewDebugDashboard(cfg),
		providerState:        initialSessionProviderState(cfg, runtime),
		gamePrefsPath:        fastModeStatus.Path,
		gameFastMode:         fastModeStatus.Current,
		gameFastModeChanged:  fastModeStatus.Changed,
		gameFastModePrevious: fastModeStatus.Previous,
		gameFastModeWarning:  fastModeWarning,
		pauseSignal:          make(chan struct{}),
		closeSignal:          make(chan struct{}),
	}

	// Set global knowledge reference for static policy functions
	activeKnowledge = session.knowledge

	if err := session.loadCarryForwardState(cfg.ArtifactsDir); err != nil {
		runtime.Close()
		return nil, err
	}
	session.refreshGuidebook(true)

	go session.run(ctx)
	return session, nil
}

func prepareGameFastMode(cfg config.Config) (*game.FastModeStatus, string) {
	status := &game.FastModeStatus{
		Path:    strings.TrimSpace(cfg.GamePrefsPath),
		Desired: strings.TrimSpace(cfg.GameFastMode),
	}

	if strings.TrimSpace(cfg.GamePrefsPath) == "" {
		return status, ""
	}

	if strings.TrimSpace(cfg.GameFastMode) == "" {
		current, err := game.ReadFastMode(cfg.GamePrefsPath)
		if err != nil {
			return status, fmt.Sprintf("game fast mode probe failed: %v", err)
		}
		return current, ""
	}

	current, err := game.EnsureFastMode(cfg.GamePrefsPath, cfg.GameFastMode)
	if err != nil {
		return status, fmt.Sprintf("game fast mode update failed: %v", err)
	}
	return current, ""
}

func (s *Session) Events() <-chan SessionEvent {
	return s.events
}

func (s *Session) Errors() <-chan error {
	return s.errs
}

func (s *Session) Close() {
	s.closeOnce.Do(func() {
		close(s.closeSignal)
	})
	if s.runtime != nil {
		s.runtime.Close()
	}
	if s.store != nil {
		_ = s.store.Close()
	}
}

func (s *Session) Pause() bool {
	if s == nil {
		return false
	}

	s.pauseMu.Lock()
	if s.paused {
		s.pauseMu.Unlock()
		return false
	}
	s.paused = true
	s.rotatePauseSignalLocked()
	s.pauseMu.Unlock()

	s.emit(SessionEvent{
		Time:    time.Now(),
		Kind:    SessionEventStatus,
		Cycle:   s.cycle,
		Attempt: s.attemptCount,
		Message: s.say("autoplay paused", "autoplay paused"),
		Data: map[string]interface{}{
			"paused": true,
		},
	})
	return true
}

func (s *Session) Resume() bool {
	if s == nil {
		return false
	}

	s.pauseMu.Lock()
	if !s.paused {
		s.pauseMu.Unlock()
		return false
	}
	s.paused = false
	s.rotatePauseSignalLocked()
	s.pauseMu.Unlock()

	s.emit(SessionEvent{
		Time:    time.Now(),
		Kind:    SessionEventStatus,
		Cycle:   s.cycle,
		Attempt: s.attemptCount,
		Message: s.say("autoplay resumed", "autoplay resumed"),
		Data: map[string]interface{}{
			"paused": false,
		},
	})
	return true
}

func (s *Session) IsPaused() bool {
	if s == nil {
		return false
	}
	s.pauseMu.Lock()
	defer s.pauseMu.Unlock()
	return s.paused
}

func (s *Session) rotatePauseSignalLocked() {
	close(s.pauseSignal)
	s.pauseSignal = make(chan struct{})
}

func (s *Session) waitIfPaused(ctx context.Context) error {
	for {
		s.pauseMu.Lock()
		paused := s.paused
		signal := s.pauseSignal
		s.pauseMu.Unlock()

		if !paused {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.closeSignal:
			return context.Canceled
		case <-signal:
		}
	}
}

func (s *Session) run(ctx context.Context) {
	defer close(s.events)
	defer close(s.errs)
	defer s.Close()

	snapshot := s.todo.Snapshot()
	effectiveMaxCycles := s.cfg.EffectiveMaxCycles()
	s.emit(SessionEvent{
		Time:    time.Now(),
		Kind:    SessionEventStatus,
		Message: s.initialStatusMessage(),
		Data: s.appendGuideSnapshotData(map[string]interface{}{
			"mode":                 s.cfg.ModeLabel(),
			"provider":             s.cfg.ProviderLabel(),
			"model":                s.cfg.Model,
			"base_url":             s.cfg.APIBaseURL,
			"claude_cli_path":      s.cfg.ClaudeCLIPath,
			"max_attempts":         s.maxAttempts,
			"max_cycles":           s.cfg.MaxCycles,
			"effective_max_cycles": effectiveMaxCycles,
			"artifacts_dir":        s.store.Root(),
			"run_index_path":       s.store.IndexPath(),
			"guidebook_path":       s.guide.GuidebookPath(),
			"living_codex_path":    s.guide.LivingCodexPath(),
			"combat_playbook_path": s.guide.CombatPlaybookPath(),
			"event_playbook_path":  s.guide.EventPlaybookPath(),
			"dashboard_path":       filepath.Join(s.store.Root(), "dashboard.md"),
			"run_story_path":       filepath.Join(s.store.Root(), "run-story.md"),
			"run_guide_path":       filepath.Join(s.store.Root(), "run-guide.md"),
			"loaded_reflections":   len(s.compact.reflections),
			"provider_state":       s.providerState,
			"agent_available":      s.runtime != nil && s.runtime.Agent != nil,
			"force_deterministic":  s.forceDeterministic,
			"current_goal":         localizeTodoText(snapshot.CurrentGoal, s.localizer()),
			"room_goal":            localizeTodoText(snapshot.RoomGoal, s.localizer()),
			"next_intent":          localizeTodoText(snapshot.NextIntent, s.localizer()),
			"last_failure":         localizeTodoText(snapshot.LastFailure, s.localizer()),
			"carry_forward_plan":   localizeTodoText(snapshot.CarryForwardPlan, s.localizer()),
			"carry_forward_lessons": append(
				[]string(nil),
				localizeTodoSlice(snapshot.CarryForwardLessons, s.localizer())...,
			),
			"carry_forward_buckets":   snapshot.CarryForwardBuckets.ToDataMap(),
			"seen_content_counts":     seenContentCountsData(s.world.Snapshot()),
			"game_prefs_path":         s.gamePrefsPath,
			"game_fast_mode":          s.gameFastMode,
			"game_fast_mode_changed":  s.gameFastModeChanged,
			"game_fast_mode_previous": s.gameFastModePrevious,
		}),
	})
	if strings.TrimSpace(s.gameFastModeWarning) != "" {
		s.emit(SessionEvent{
			Time:    time.Now(),
			Kind:    SessionEventStatus,
			Message: s.gameFastModeWarning,
			Data: map[string]interface{}{
				"game_prefs_path": s.gamePrefsPath,
				"game_fast_mode":  s.gameFastMode,
			},
		})
	} else if s.gameFastModeChanged {
		previousMode := strings.TrimSpace(s.gameFastModePrevious)
		if previousMode == "" {
			previousMode = "-"
		}
		currentMode := strings.TrimSpace(s.gameFastMode)
		if currentMode == "" {
			currentMode = "-"
		}
		s.emit(SessionEvent{
			Time: time.Now(),
			Kind: SessionEventStatus,
			Message: s.sayf(
				"game fast mode applied: %s -> %s",
				"game fast mode applied: %s -> %s",
				previousMode,
				currentMode,
			),
			Data: map[string]interface{}{
				"game_prefs_path":         s.gamePrefsPath,
				"game_fast_mode":          s.gameFastMode,
				"game_fast_mode_changed":  true,
				"game_fast_mode_previous": s.gameFastModePrevious,
			},
		})
	}

	if err := s.loop(ctx); err != nil && !errors.Is(err, context.Canceled) {
		s.errs <- err
	}
}

func (s *Session) loop(ctx context.Context) error {
	maxCycles := s.cfg.EffectiveMaxCycles()
	for s.cycle = 1; maxCycles <= 0 || s.cycle <= maxCycles; s.cycle++ {
		if err := s.waitIfPaused(ctx); err != nil {
			return err
		}
		state, err := s.readActionableState(ctx)
		if err != nil {
			return err
		}
		if err := s.waitIfPaused(ctx); err != nil {
			return err
		}

		s.recordState(state)
		s.reflectIfNeeded(state)
		s.maybeEmitPassiveStreamerBeat(ctx, state)
		if strings.EqualFold(state.Screen, "GAME_OVER") && s.shouldStopAtGameOver(state) {
			s.writeRunArtifacts(nil)
			s.emit(SessionEvent{
				Time:    time.Now(),
				Kind:    SessionEventStop,
				Cycle:   s.cycle,
				Attempt: s.currentAttemptForState(state),
								Message: s.say("run reached GAME_OVER", "游戏结束，到达 GAME_OVER"),
				Screen:  state.Screen,
				RunID:   state.RunID,
				State:   state,
			})
			return nil
		}

		if request, reason, ok := s.chooseAction(state); ok {
			if err := s.waitIfPaused(ctx); err != nil {
				return err
			}
			s.emit(SessionEvent{
				Time:    time.Now(),
				Kind:    SessionEventAction,
				Cycle:   s.cycle,
				Attempt: s.currentAttemptForState(state),
				Message: reason,
				Screen:  state.Screen,
				RunID:   state.RunID,
				Action:  request.Action,
				State:   state,
			})

			if err := s.executeDirectAction(ctx, state, request); err != nil {
				if s.handleDirectActionFailure(ctx, err, state, request) {
					continue
				}
				return err
			}

			continue
		}

		if s.runtime.Agent == nil {
			return fmt.Errorf("model configuration is unavailable and no deterministic action matched screen %s (actions=%s)", state.Screen, strings.Join(state.AvailableActions, ","))
		}

		if len(s.runtime.Agent.GetMessages()) >= agentContextMessageThreshold {
			s.compact.Apply(s.runtime.Agent, s.todo, state)
			s.emit(SessionEvent{
				Time:    time.Now(),
				Kind:    SessionEventCompact,
				Cycle:   s.cycle,
				Attempt: s.currentAttemptForState(state),
				Message: s.say("agent context compacted", "agent context compacted"),
				Screen:  state.Screen,
				RunID:   state.RunID,
				Data: map[string]interface{}{
					"summary": s.compact.PromptBlock(),
				},
			})
		}

		plan := s.currentCombatPlan(state)
		promptMode := PromptModeCycle
		if s.usesStructuredDecisionMode() {
			promptMode = PromptModeStructured
		}
		assembly := s.promptPipeline().Build(promptMode, state, s.todo, s.skills, s.compact, plan, s.knowledge, s.cfg.Language, s.playerSkill)
		prompt := assembly.Text
		if err := s.store.RecordPrompt(s.cycle, prompt); err != nil {
			return err
		}

		s.emit(SessionEvent{
			Time:    time.Now(),
			Kind:    SessionEventPrompt,
			Cycle:   s.cycle,
			Attempt: s.currentAttemptForState(state),
			Message: s.say("starting agent cycle", "starting agent cycle"),
			Screen:  state.Screen,
			RunID:   state.RunID,
			Data: map[string]interface{}{
				"prompt":                 prompt,
				"prompt_mode":            string(assembly.Telemetry.Mode),
				"prompt_screen":          assembly.Telemetry.Screen,
				"prompt_size_bytes":      assembly.Telemetry.PromptSizeBytes,
				"prompt_block_breakdown": cloneIntMap(assembly.Telemetry.BlockBreakdown),
			},
		})

		if err := s.waitIfPaused(ctx); err != nil {
			return err
		}

		cycleResult, err := s.runAgentCycle(ctx, prompt, state)
		if err != nil {
			if retryErr := s.handleProviderFailure(ctx, err, state); retryErr == nil {
				continue
			}
			return err
		}
		if cycleResult.Metadata == nil {
			cycleResult.Metadata = make(map[string]interface{})
		}
		cycleResult.Metadata["prompt_mode"] = string(assembly.Telemetry.Mode)
		cycleResult.Metadata["prompt_screen"] = assembly.Telemetry.Screen
		cycleResult.Metadata["prompt_size_bytes"] = assembly.Telemetry.PromptSizeBytes
		cycleResult.Metadata["prompt_block_breakdown"] = cloneIntMap(assembly.Telemetry.BlockBreakdown)
		if s.providerFailures > 0 && s.runtime.Agent != nil && !s.forceDeterministic {
			s.emit(SessionEvent{
				Time:    time.Now(),
				Kind:    SessionEventStatus,
				Cycle:   s.cycle,
				Attempt: s.currentAttemptForState(state),
				Message: s.say("agent provider recovered and resumed model-backed play", "agent provider recovered and resumed model-backed play"),
				Screen:  state.Screen,
				RunID:   state.RunID,
				Data: map[string]interface{}{
					"provider_state":    "healthy",
					"provider_recovery": "recovered",
				},
			})
		}
		s.providerFailures = 0
		if s.runtime.Agent != nil && !s.forceDeterministic {
			s.providerState = "healthy"
		}

		if cycleResult.Decision != nil {
			if err := s.executeModelDecision(ctx, state, cycleResult.Decision); err != nil {
				if s.handleDirectActionFailure(ctx, err, state, DecisionToActionRequest(cycleResult.Decision)) {
					cycleResult.HadInvalidAction = true
					if err := s.store.RecordCycleSummary(s.cycle, cycleResult); err != nil {
						return err
					}
					continue
				}
				return err
			}
			cycleResult.MadeProgress = true
			cycleResult.LastAction = cycleResult.Decision.Action
		}

		if err := s.store.RecordCycleSummary(s.cycle, cycleResult); err != nil {
			return err
		}

		if cycleResult.HadInvalidAction {
			s.repeatedInvalidActions++
		} else {
			s.repeatedInvalidActions = 0
		}

		if s.repeatedInvalidActions >= maxRepeatedInvalidActions {
			s.compact.Apply(s.runtime.Agent, s.todo, state)
			s.repeatedInvalidActions = 0
			s.emit(SessionEvent{
				Time:    time.Now(),
				Kind:    SessionEventCompact,
				Cycle:   s.cycle,
				Attempt: s.currentAttemptForState(state),
				Message: s.say("context compacted after repeated invalid actions", "context compacted after repeated invalid actions"),
				Screen:  state.Screen,
				RunID:   state.RunID,
			})
		}

		afterState, err := s.readActionableState(ctx)
		if err != nil {
			return err
		}

		s.recordState(afterState)

		if !cycleResult.MadeProgress && s.lastDigest == digestState(afterState) {
			s.stalledCycles++
		} else {
			s.stalledCycles = 0
		}

		if s.stalledCycles >= maxStalledCycles {
			return fmt.Errorf("autoplay stalled for %d cycles on screen %s", s.stalledCycles, afterState.Screen)
		}
	}

	return fmt.Errorf("autoplay reached cycle limit (%d)", maxCycles)
}

func (s *Session) chooseAction(state *game.StateSnapshot) (game.ActionRequest, string, bool) {
	if s.todo != nil && s.todo.ShouldProceedAfterResolvedCardReward(state) {
		return game.ActionRequest{Action: "proceed"}, "card reward was already resolved on this reward screen", true
	}

	if request, reason, ok := ChooseRuleBasedAction(state, s.maxAttempts, s.currentAttemptForState(state), s.failures); ok {
		return request, reason, true
	}

	if s.canUseModelAgent() {
		if request, reason, ok := s.choosePlannerDirectAction(state); ok {
			return request, reason, true
		}
		return game.ActionRequest{}, "", false
	}

	if request, reason, ok := s.choosePlannerFallbackAction(state); ok {
		return request, reason, true
	}

	return ChooseDeterministicAction(state, s.maxAttempts, s.currentAttemptForState(state), s.failures)
}

func (s *Session) shouldStopAtGameOver(state *game.StateSnapshot) bool {
	return attemptsExhausted(s.maxAttempts, s.currentAttemptForState(state))
}

func (s *Session) currentAttemptForState(state *game.StateSnapshot) int {
	if state != nil && state.RunID != "" {
		if attempt, ok := s.seenRunIDs[state.RunID]; ok {
			return attempt
		}
	}

	return s.attemptCount
}

func (s *Session) registerAttempt(state *game.StateSnapshot) int {
	if state == nil || state.RunID == "" || state.RunID == "run_unknown" {
		return s.attemptCount
	}

	if attempt, ok := s.seenRunIDs[state.RunID]; ok {
		return attempt
	}

	s.attemptCount++
	s.seenRunIDs[state.RunID] = s.attemptCount
	s.emit(SessionEvent{
		Time:    time.Now(),
		Kind:    SessionEventStatus,
		Cycle:   s.cycle,
		Attempt: s.attemptCount,
		Message: s.say(
			fmt.Sprintf("attempt %s started", formatAttemptProgress(s.attemptCount, s.maxAttempts)),
			fmt.Sprintf("attempt %s started", formatAttemptProgress(s.attemptCount, s.maxAttempts)),
		),
		Screen: state.Screen,
		RunID:  state.RunID,
		Data: map[string]interface{}{
			"attempt":           s.attemptCount,
			"max_attempts":      s.maxAttempts,
			"attempt_lifecycle": "start",
		},
	})

	return s.attemptCount
}

func formatAttemptProgress(attempt int, maxAttempts int) string {
	if maxAttempts <= 0 {
		return fmt.Sprintf("%d/continuous", attempt)
	}
	return fmt.Sprintf("%d/%d", attempt, maxAttempts)
}

// renumberAttemptReflections assigns sequential attempt numbers (1, 2, 3, ...)
// to reflections loaded from multiple sessions, where each session may have
// started numbering from 1 independently.
func renumberAttemptReflections(reflections []*AttemptReflection) {
	for i, r := range reflections {
		if r != nil {
			r.Attempt = i + 1
		}
	}
}

func digestState(state *game.StateSnapshot) string {
	return decisionStateDigest(state)
}

func cloneMap(input map[string]interface{}) map[string]interface{} {
	if len(input) == 0 {
		return nil
	}

	clone := make(map[string]interface{}, len(input))
	for key, value := range input {
		clone[key] = value
	}

	return clone
}

func cloneIntMap(input map[string]int) map[string]int {
	if len(input) == 0 {
		return nil
	}

	clone := make(map[string]int, len(input))
	for key, value := range input {
		clone[key] = value
	}

	return clone
}

func attemptLifecycleForState(state *game.StateSnapshot) string {
	if state == nil {
		return ""
	}

	switch {
	case strings.EqualFold(state.Screen, "GAME_OVER"):
		return "game_over"
	case strings.TrimSpace(state.RunID) == "" || strings.EqualFold(state.RunID, "run_unknown"):
		return "bootstrap_next"
	default:
		return "running"
	}
}

func (s *Session) say(english string, chinese string) string {
	return i18n.New(s.cfg.Language).Label(english, chinese)
}

func (s *Session) localizer() i18n.Localizer {
	return i18n.New(s.cfg.Language)
}

func (s *Session) sayf(english string, chinese string, args ...interface{}) string {
	loc := i18n.New(s.cfg.Language)
	switch loc.Language() {
	case i18n.LanguageChinese:
		return fmt.Sprintf(chinese, args...)
	case i18n.LanguageBilingual:
		return fmt.Sprintf(chinese, args...) + " / " + fmt.Sprintf(english, args...)
	default:
		return fmt.Sprintf(english, args...)
	}
}
