package agentruntime

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

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
	cfg      config.Config
	runtime  *Runtime
	store    *RunStore
	guide    *GuidebookStore
	catalog  *codexCatalog
	planner  CombatPlanner
	resolver *ActionResolutionPipeline
	gate     *StableStateGate

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
	promptBuilder          *PromptAssemblyPipeline
	gamePrefsPath          string
	gameFastMode           string
	gameFastModeChanged    bool
	gameFastModePrevious   string
	gameFastModeWarning    string
	lastGuideRefresh       time.Time
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

	guide, err := NewGuidebookStore(cfg.ArtifactsDir)
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

	session := &Session{
		cfg:                  cfg,
		runtime:              runtime,
		store:                store,
		guide:                guide,
		catalog:              catalog,
		planner:              NewCombatPlanner(cfg),
		resolver:             NewActionResolutionPipeline(),
		gate:                 NewStableStateGate(runtime.Client),
		promptBuilder:        NewPromptAssemblyPipeline(),
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

	if err := session.loadCarryForwardState(cfg.ArtifactsDir); err != nil {
		runtime.Close()
		return nil, err
	}
	session.refreshGuidebook(true)

	go session.run(ctx)
	return session, nil
}

func (s *Session) loadCarryForwardState(root string) error {
	loadedReflections, err := LoadRecentAttemptReflections(root, s.store.Root(), 4)
	if err != nil {
		return err
	}
	// Renumber loaded reflections sequentially so cross-session attempts
	// get unique, monotonically increasing attempt numbers in the story.
	renumberAttemptReflections(loadedReflections)
	for _, reflection := range loadedReflections {
		s.todo.ApplyReflection(reflection)
		s.compact.RecordReflection(reflection)
		s.reflections = append(s.reflections, reflection)
		if reflection.Attempt > s.storyAttemptBase {
			s.storyAttemptBase = reflection.Attempt
		}
		if reflection.RunID != "" {
			s.reflectedRunIDs[reflection.RunID] = true
		}
	}

	resume, err := LoadLatestSessionResume(root, s.store.Root())
	if err != nil {
		return err
	}
	s.applyResumeState(resume)

	seenContent, err := LoadRecentSeenContent(root, s.store.Root(), 8)
	if err != nil {
		return err
	}
	if s.world != nil {
		s.world.Merge(seenContent)
	}
	return nil
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

func (s *Session) applyResumeState(resume *SessionResumeState) {
	if resume == nil {
		return
	}

	s.resumeState = resume
	s.todo.ApplyResume(resume)
	s.compact.ApplyResume(resume)
}

func (s *Session) persistSessionResume(state *game.StateSnapshot) {
	if s.store == nil {
		return
	}

	resume := BuildSessionResumeState(s.attemptCount, s.seenRunIDs, s.todo, state)
	_ = s.store.WriteSessionResume(resume)
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
			"current_goal":         snapshot.CurrentGoal,
			"room_goal":            snapshot.RoomGoal,
			"next_intent":          snapshot.NextIntent,
			"last_failure":         snapshot.LastFailure,
			"carry_forward_plan":   snapshot.CarryForwardPlan,
			"carry_forward_lessons": append(
				[]string(nil),
				snapshot.CarryForwardLessons...,
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
		if strings.EqualFold(state.Screen, "GAME_OVER") && s.shouldStopAtGameOver(state) {
			s.writeRunArtifacts(nil)
			s.emit(SessionEvent{
				Time:    time.Now(),
				Kind:    SessionEventStop,
				Cycle:   s.cycle,
				Attempt: s.currentAttemptForState(state),
				Message: s.say("run reached GAME_OVER", "閺堫剝鐤嗗鎻掑煂 GAME_OVER"),
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
		assembly := s.promptPipeline().Build(promptMode, state, s.todo, s.skills, s.compact, plan, s.cfg.Language)
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

func (s *Session) executeDirectAction(ctx context.Context, expectedState *game.StateSnapshot, request game.ActionRequest) error {
	beforeState, err := s.runtime.Client.GetState(ctx)
	if err != nil {
		return err
	}
	beforeState, err = s.freshActionableStateForExecution(ctx, beforeState)
	if err != nil {
		return err
	}
	if expectedState != nil && digestState(beforeState) != digestState(expectedState) {
		beforeState, _ = s.stabilizeLiveStateForExecution(ctx, expectedState, beforeState)
	}
	resolved, err := s.actionResolver().ResolveRequest(expectedState, beforeState, request)
	if err != nil {
		s.todo.RecordFailure(request.Action, err)
		return err
	}
	beforeState = resolved.LiveState
	request = resolved.Request
	if expectedState != nil && resolved.RecoveryKind != "" {
		driftKind := classifyStateDrift(expectedState, beforeState)
		if !shouldQuietDecisionReuse(driftKind, resolved.RecoveryKind, resolved.OriginalRequest, request) {
			s.emit(SessionEvent{
				Time:    time.Now(),
				Kind:    SessionEventStatus,
				Cycle:   s.cycle,
				Attempt: s.currentAttemptForState(beforeState),
				Message: s.say("live state changed before direct action execution; remapping the action onto the latest legal state", "live state changed before direct action execution; remapping the action onto the latest legal state"),
				Screen:  beforeState.Screen,
				RunID:   beforeState.RunID,
				State:   beforeState,
				Data: map[string]interface{}{
					"expected_state_summary":     decisionStateSummary(expectedState),
					"live_state_summary":         decisionStateSummary(beforeState),
					"expected_state_fingerprint": digestState(expectedState),
					"live_state_fingerprint":     digestState(beforeState),
					"drift_kind":                 driftKind,
					"recovery_kind":              resolved.RecoveryKind,
					"decision_reused":            resolved.DecisionReused,
				},
			})
		}
	}

	beforeState, request, err = s.rebindRequestForImmediateExecution(ctx, expectedState, beforeState, request)
	if err != nil {
		s.todo.RecordFailure(request.Action, err)
		return err
	}

	result, err := s.runtime.Client.Act(ctx, request)
	if err != nil && request.Action == "play_card" && isInvalidTargetActionError(err) {
		latestState, stateErr := s.runtime.Client.GetState(ctx)
		if stateErr == nil {
			if recovered, ok := s.actionResolver().RecoverPlayCardTarget(beforeState, latestState, request); ok {
				result, err = s.runtime.Client.Act(ctx, recovered.Request)
				if err == nil {
					beforeState = recovered.LiveState
					request = recovered.Request
				}
			}
		}
	}
	if err != nil && isInvalidActionError(err) {
		if recoveredResult, recoveredState, recoveredRequest, ok := s.recoverStaleInvalidAction(ctx, expectedState, beforeState, request, err); ok {
			result = recoveredResult
			beforeState = recoveredState
			request = recoveredRequest
			err = nil
		}
	}
	if err != nil {
		if recoveredResult, ok := s.recoverTransientActionTransport(ctx, beforeState, request, err); ok {
			result = recoveredResult
			err = nil
		}
	}
	if err != nil {
		s.todo.RecordFailure(request.Action, err)
		return err
	}
	if !result.Stable {
		result = s.settlePendingActionResult(ctx, beforeState, request, result)
	}

	beforeDigest := digestState(beforeState)
	afterDigest := digestState(&result.State)
	if !result.Stable && beforeDigest != "" && beforeDigest == afterDigest {
		if s.failures != nil {
			s.failures.Record(beforeDigest, request)
		}
	}

	s.todo.RecordAction(request.Action, beforeState, &result.State)
	s.compact.RecordAction(request.Action, &result.State, result.Message)

	s.emit(SessionEvent{
		Time:    time.Now(),
		Kind:    SessionEventAction,
		Cycle:   s.cycle,
		Attempt: s.currentAttemptForState(&result.State),
		Message: result.Message,
		Screen:  result.State.Screen,
		RunID:   result.State.RunID,
		Action:  request.Action,
		State:   &result.State,
		Data: map[string]interface{}{
			"stable": result.Stable,
			"status": result.Status,
		},
	})

	return nil
}

func (s *Session) rebindRequestForImmediateExecution(ctx context.Context, expectedState *game.StateSnapshot, liveState *game.StateSnapshot, request game.ActionRequest) (*game.StateSnapshot, game.ActionRequest, error) {
	if s == nil || s.runtime == nil || s.runtime.Client == nil {
		return liveState, request, nil
	}

	latestState, err := s.runtime.Client.GetState(ctx)
	if err != nil || latestState == nil {
		return liveState, request, nil
	}
	latestState, err = s.freshActionableStateForExecution(ctx, latestState)
	if err != nil {
		return nil, request, err
	}

	needsResolve := digestState(latestState) != digestState(liveState) || ValidateActionRequest(latestState, request) != nil
	if !needsResolve {
		return latestState, request, nil
	}

	if expectedState != nil {
		latestState, _ = s.stabilizeLiveStateForExecution(ctx, expectedState, latestState)
	}

	resolved, err := s.actionResolver().ResolveRequest(expectedState, latestState, request)
	if err != nil {
		return nil, request, err
	}
	return resolved.LiveState, resolved.Request, nil
}

func (s *Session) recoverStaleInvalidAction(ctx context.Context, expectedState *game.StateSnapshot, beforeState *game.StateSnapshot, request game.ActionRequest, actErr error) (*game.ActionResult, *game.StateSnapshot, game.ActionRequest, bool) {
	if s == nil || s.runtime == nil || s.runtime.Client == nil {
		return nil, beforeState, request, false
	}

	recoveryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	liveState, err := s.runtime.Client.GetState(recoveryCtx)
	if err != nil || liveState == nil {
		return nil, beforeState, request, false
	}
	if refreshed, refreshErr := s.freshActionableStateForExecution(recoveryCtx, liveState); refreshErr == nil && refreshed != nil {
		liveState = refreshed
	}

	resolved, err := s.actionResolver().ResolveRequest(expectedState, liveState, request)
	if err != nil || sameActionRequest(request, resolved.Request) && digestState(beforeState) == digestState(resolved.LiveState) {
		return nil, beforeState, request, false
	}

	s.emit(SessionEvent{
		Time:    time.Now(),
		Kind:    SessionEventStatus,
		Cycle:   s.cycle,
		Attempt: s.currentAttemptForState(resolved.LiveState),
		Message: s.say("action became stale before execution; retrying on the latest stable state", "action became stale before execution; retrying on the latest stable state"),
		Screen:  resolved.LiveState.Screen,
		RunID:   resolved.LiveState.RunID,
		State:   resolved.LiveState,
		Data: map[string]interface{}{
			"recovery_kind":              "invalid_action_rebind",
			"expected_state_summary":     decisionStateSummary(expectedState),
			"live_state_summary":         decisionStateSummary(resolved.LiveState),
			"expected_state_fingerprint": digestState(expectedState),
			"live_state_fingerprint":     digestState(resolved.LiveState),
			"original_error":             actErr.Error(),
		},
	})

	result, err := s.runtime.Client.Act(recoveryCtx, resolved.Request)
	if err != nil {
		return nil, beforeState, request, false
	}
	return result, resolved.LiveState, resolved.Request, true
}

func (s *Session) recoverTransientActionTransport(ctx context.Context, beforeState *game.StateSnapshot, request game.ActionRequest, actErr error) (*game.ActionResult, bool) {
	if !isTransientActionTransportError(actErr) || s == nil || s.runtime == nil || s.runtime.Client == nil {
		return nil, false
	}

	recoveryCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	liveState, err := s.runtime.Client.GetState(recoveryCtx)
	if err != nil || liveState == nil {
		return nil, false
	}
	if refreshed, refreshErr := s.freshActionableStateForExecution(recoveryCtx, liveState); refreshErr == nil && refreshed != nil {
		liveState = refreshed
	}

	if beforeState != nil && digestState(beforeState) == digestState(liveState) && ValidateActionRequest(liveState, request) == nil {
		return nil, false
	}

	s.emit(SessionEvent{
		Time:    time.Now(),
		Kind:    SessionEventStatus,
		Cycle:   s.cycle,
		Attempt: s.currentAttemptForState(liveState),
		Message: s.say("action response stream dropped; continuing from the latest live state", "action response stream dropped; continuing from the latest live state"),
		Screen:  liveState.Screen,
		RunID:   liveState.RunID,
		State:   liveState,
		Data: map[string]interface{}{
			"provider_state":             s.providerState,
			"recovery_kind":              "transport_recover",
			"expected_state_summary":     decisionStateSummary(beforeState),
			"live_state_summary":         decisionStateSummary(liveState),
			"expected_state_fingerprint": digestState(beforeState),
			"live_state_fingerprint":     digestState(liveState),
		},
	})

	return &game.ActionResult{
		Action:  request.Action,
		Status:  "completed_after_transport_recover",
		Stable:  game.IsActionableState(liveState),
		Message: s.say("Action recovered from a dropped bridge response.", "Action recovered from a dropped bridge response."),
		State:   *liveState,
	}, true
}

func (s *Session) settlePendingActionResult(ctx context.Context, beforeState *game.StateSnapshot, request game.ActionRequest, result *game.ActionResult) *game.ActionResult {
	if s == nil || s.runtime == nil || s.runtime.Client == nil || result == nil || result.Stable {
		return result
	}

	waitCtx, cancel := context.WithTimeout(ctx, executionDriftWaitTimeout)
	defer cancel()

	settled, err := s.runtime.Client.WaitUntilActionable(waitCtx, executionDriftWaitTimeout)
	if err != nil || settled == nil {
		return result
	}

	beforeDigest := digestState(beforeState)
	settledDigest := digestState(settled)
	if beforeDigest == "" || settledDigest == "" {
		return result
	}
	if beforeDigest == settledDigest && strings.EqualFold(strings.TrimSpace(request.Action), "claim_reward") {
		return result
	}

	settledResult := *result
	settledResult.State = *settled
	settledResult.Stable = true
	settledResult.Status = "completed_after_wait"
	settledResult.Message = s.say(
		"Action completed after transition settled.",
		"Action completed after transition settled.",
	)
	return &settledResult
}

func (s *Session) actionResolver() *ActionResolutionPipeline {
	if s == nil || s.resolver == nil {
		return NewActionResolutionPipeline()
	}
	return s.resolver
}

func (s *Session) stateGate() *StableStateGate {
	if s == nil {
		return nil
	}
	if s.gate != nil {
		return s.gate
	}
	if s.runtime == nil {
		return nil
	}
	return NewStableStateGate(s.runtime.Client)
}

func (s *Session) freshActionableStateForExecution(ctx context.Context, candidate *game.StateSnapshot) (*game.StateSnapshot, error) {
	gate := s.stateGate()
	if gate == nil {
		return candidate, nil
	}
	return gate.FreshActionableStateForExecution(ctx, candidate, executionDriftWaitTimeout)
}

func remapActionRequestForLiveState(expectedState *game.StateSnapshot, liveState *game.StateSnapshot, request game.ActionRequest) (game.ActionRequest, string, bool) {
	resolved, err := NewActionResolutionPipeline().ResolveRequest(expectedState, liveState, request)
	if err != nil || resolved == nil || resolved.RecoveryKind == "" {
		return game.ActionRequest{}, "", false
	}
	return resolved.Request, resolved.RecoveryKind, true
}

func (s *Session) stabilizeLiveStateForExecution(ctx context.Context, expectedState *game.StateSnapshot, liveState *game.StateSnapshot) (*game.StateSnapshot, string) {
	gate := s.stateGate()
	if gate == nil {
		return liveState, classifyStateDrift(expectedState, liveState)
	}
	return gate.StabilizeExecutionDrift(ctx, expectedState, liveState, executionDriftWaitTimeout)
}

func shouldForceExecutionReread(driftKind string) bool {
	switch driftKind {
	case driftKindSameScreenIndexDrift, driftKindActionWindowChanged:
		return true
	default:
		return false
	}
}

func shouldStabilizeExecutionDrift(driftKind string) bool {
	switch {
	case driftKind == driftKindRewardTransition,
		driftKind == driftKindSelectionSeam,
		driftKind == driftKindSameScreenIndexDrift,
		driftKind == driftKindActionWindowChanged,
		driftKind == driftKindSameScreenStateDrift:
		return true
	case strings.HasPrefix(driftKind, driftKindScreenTransition+":reward"),
		strings.HasPrefix(driftKind, driftKindScreenTransition+":card_selection"),
		strings.HasPrefix(driftKind, driftKindScreenTransition+":game_over"):
		return true
	default:
		return false
	}
}

func remapPlayCardRequestOnFailure(expectedState *game.StateSnapshot, liveState *game.StateSnapshot, request game.ActionRequest) (game.ActionRequest, *game.StateSnapshot, bool) {
	resolved, ok := NewActionResolutionPipeline().RecoverPlayCardTarget(expectedState, liveState, request)
	if !ok || resolved == nil {
		return game.ActionRequest{}, nil, false
	}
	return resolved.Request, resolved.LiveState, true
}

func sameActionRequest(left game.ActionRequest, right game.ActionRequest) bool {
	if left.Action != right.Action {
		return false
	}
	if !sameOptionalInt(left.CardIndex, right.CardIndex) {
		return false
	}
	if !sameOptionalInt(left.TargetIndex, right.TargetIndex) {
		return false
	}
	if !sameOptionalInt(left.OptionIndex, right.OptionIndex) {
		return false
	}
	return true
}

func sameOptionalInt(left *int, right *int) bool {
	switch {
	case left == nil && right == nil:
		return true
	case left == nil || right == nil:
		return false
	default:
		return *left == *right
	}
}

func isInvalidTargetActionError(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "invalid_target")
}

func isInvalidActionError(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "invalid_action")
}

func (s *Session) executeModelDecision(ctx context.Context, state *game.StateSnapshot, decision *ActionDecision) error {
	liveState, err := s.runtime.Client.GetState(ctx)
	if err != nil {
		return err
	}
	liveState, err = s.freshActionableStateForExecution(ctx, liveState)
	if err != nil {
		return err
	}
	if digestState(liveState) != digestState(state) {
		liveState, _ = s.stabilizeLiveStateForExecution(ctx, state, liveState)
	}

	resolved, err := s.actionResolver().ResolveDecision(state, liveState, decision)
	if err != nil {
		driftKind := classifyStateDrift(state, liveState)
		softReplan := isSoftReplanDriftKind(driftKind)
		if !softReplan {
			s.todo.RecordFailure("model_decision", err)
		}
		recoveryKind := "hard_replan"
		if softReplan {
			recoveryKind = "soft_replan"
		}
		message := s.sayf("live state changed before model action execution; replanning (%s)", "live state changed before model action execution; replanning (%s)", driftKind)
		if softReplan {
			message = s.sayf("live state shifted along a known seam; replanning (%s)", "live state shifted along a known seam; replanning (%s)", driftKind)
		}
		s.emit(SessionEvent{
			Time:    time.Now(),
			Kind:    SessionEventStatus,
			Cycle:   s.cycle,
			Attempt: s.currentAttemptForState(liveState),
			Message: message,
			Screen:  liveState.Screen,
			RunID:   liveState.RunID,
			State:   liveState,
			Data: map[string]interface{}{
				"expected_state_summary":     decisionStateSummary(state),
				"live_state_summary":         decisionStateSummary(liveState),
				"expected_state_fingerprint": digestState(state),
				"live_state_fingerprint":     digestState(liveState),
				"drift_kind":                 driftKind,
				"recovery_kind":              recoveryKind,
				"decision_reused":            false,
			},
		})
		if softReplan {
			err = markSoftReplanReported(err)
		}
		return err
	}

	request := resolved.Request
	if resolved.RecoveryKind != "" {
		driftKind := classifyStateDrift(state, liveState)
		if !shouldQuietDecisionReuse(driftKind, resolved.RecoveryKind, resolved.OriginalRequest, request) {
			s.emit(SessionEvent{
				Time:    time.Now(),
				Kind:    SessionEventStatus,
				Cycle:   s.cycle,
				Attempt: s.currentAttemptForState(liveState),
				Message: s.say("live state changed before model action execution; reusing the decision on the latest legal state", "live state changed before model action execution; reusing the decision on the latest legal state"),
				Screen:  liveState.Screen,
				RunID:   liveState.RunID,
				State:   liveState,
				Data: map[string]interface{}{
					"expected_state_summary":     decisionStateSummary(state),
					"live_state_summary":         decisionStateSummary(liveState),
					"expected_state_fingerprint": digestState(state),
					"live_state_fingerprint":     digestState(liveState),
					"drift_kind":                 driftKind,
					"recovery_kind":              resolved.RecoveryKind,
					"decision_reused":            true,
				},
			})
		}
	}
	if decision != nil && strings.TrimSpace(decision.Reason) != "" {
		s.emit(SessionEvent{
			Time:    time.Now(),
			Kind:    SessionEventAction,
			Cycle:   s.cycle,
			Attempt: s.currentAttemptForState(state),
			Message: decision.Reason,
			Screen:  state.Screen,
			RunID:   state.RunID,
			Action:  decision.Action,
		})
	}

	return s.executeDirectAction(ctx, resolved.LiveState, request)
}

func (s *Session) readActionableState(ctx context.Context) (*game.StateSnapshot, error) {
	state, err := s.runtime.Client.GetState(ctx)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(state.Screen, "UNKNOWN") {
		if err := s.waitForBridgeReady(ctx, bridgeBootstrapWaitTimeout); err != nil {
			return nil, err
		}
	}
	gate := s.stateGate()
	if gate == nil {
		return nil, fmt.Errorf("state gate is unavailable")
	}
	stableState, err := gate.ReadStableActionableState(ctx, state, stateWaitTimeout)
	if err != nil {
		return nil, err
	}
	if stableState != nil {
		return stableState, nil
	}
	return state, nil
}

func (s *Session) waitForBridgeReady(ctx context.Context, timeout time.Duration) error {
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		health, err := s.runtime.Client.GetHealth(waitCtx)
		if err == nil && health.Ready {
			return nil
		}

		select {
		case <-waitCtx.Done():
			return waitCtx.Err()
		case <-ticker.C:
		}
	}
}

func (s *Session) recordState(state *game.StateSnapshot) {
	attempt := s.registerAttempt(state)
	s.todo.Update(state)
	s.compact.RecordState(state)
	digest := digestState(state)
	if s.lastDigest != "" && s.lastDigest != digest {
		s.repeatedInvalidActions = 0
	}
	s.lastDigest = digest
	if s.failures != nil {
		s.failures.ResetForDigest(digest)
	}

	data := map[string]interface{}{}
	if s.todo != nil {
		snapshot := s.todo.Snapshot()
		data["current_goal"] = snapshot.CurrentGoal
		data["room_goal"] = snapshot.RoomGoal
		data["next_intent"] = snapshot.NextIntent
		data["last_failure"] = snapshot.LastFailure
		data["carry_forward_plan"] = snapshot.CarryForwardPlan
		data["carry_forward_lessons"] = snapshot.CarryForwardLessons
		data["carry_forward_buckets"] = snapshot.CarryForwardBuckets.ToDataMap()
	}
	if hints := BuildTacticalHints(state); len(hints) > 0 {
		data["tactical_hints"] = hints
	}
	if plan := s.currentCombatPlan(state); plan != nil {
		data["combat_plan"] = plan.DataMap()
		data["combat_plan_summary"] = plan.Summary
		data["combat_planner"] = plan.Mode
	}
	data["state_summary"] = decisionStateSummary(state)
	data["state_fingerprint"] = digestState(state)
	data["provider_state"] = s.providerState
	data["agent_available"] = s.runtime != nil && s.runtime.Agent != nil
	data["force_deterministic"] = s.forceDeterministic
	data["attempt_lifecycle"] = attemptLifecycleForState(state)
	if s.world != nil {
		discoveries := s.world.Observe(state)
		snapshot := s.world.Snapshot()
		_ = s.store.WriteSeenContent(snapshot)
		data["seen_content_counts"] = seenContentCountsData(snapshot)
		if len(discoveries) > 0 {
			data["recent_discoveries"] = SummarizeSeenContentEntriesWithCatalog(discoveries, s.catalog, s.cfg.Language)
			s.refreshGuidebook(true)
		}
	}
	data["guidebook_path"] = s.guide.GuidebookPath()
	data["living_codex_path"] = s.guide.LivingCodexPath()
	data["combat_playbook_path"] = s.guide.CombatPlaybookPath()
	data["event_playbook_path"] = s.guide.EventPlaybookPath()
	data = s.appendGuideSnapshotData(data)

	_ = s.store.WriteLatestState(state)
	s.persistSessionResume(state)
	s.writeRunArtifacts(state)
	s.refreshGuidebook(false)
	s.emit(SessionEvent{
		Time:    time.Now(),
		Kind:    SessionEventState,
		Cycle:   s.cycle,
		Attempt: attempt,
		Message: s.say("state updated", "閻樿埖鈧礁鍑￠弴瀛樻煀"),
		Screen:  state.Screen,
		RunID:   state.RunID,
		State:   state,
		Data:    data,
	})
}

func (s *Session) emit(event SessionEvent) {
	if event.Attempt == 0 {
		if event.RunID != "" {
			if attempt, ok := s.seenRunIDs[event.RunID]; ok {
				event.Attempt = attempt
			}
		}
		if event.Attempt == 0 {
			event.Attempt = s.attemptCount
		}
	}

	if s.store != nil {
		_ = s.store.AppendEvent(event)
	}
	if s.store != nil && s.dashboard != nil {
		_ = s.store.WriteDashboard(s.dashboard.ApplyEvent(event))
	}

	select {
	case s.events <- event:
	default:
	}
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

func (s *Session) handleProviderFailure(ctx context.Context, err error, state *game.StateSnapshot) error {
	if s.runtime.Agent == nil || s.forceDeterministic {
		return err
	}

	s.providerFailures++
	s.providerState = "recovering"
	s.todo.RecordFailure("agent_cycle", err)
	if s.providerFailures >= maxProviderFailures {
		s.forceDeterministic = true
		s.runtime.Agent.Close()
		s.runtime.Agent = nil
		s.providerState = "fallback"
		s.emit(SessionEvent{
			Time:    time.Now(),
			Kind:    SessionEventStatus,
			Cycle:   s.cycle,
			Attempt: s.currentAttemptForState(state),
			Message: s.sayf("agent provider failed %d times; falling back to deterministic mode", "agent provider failed %d times; falling back to deterministic mode", s.providerFailures),
			Screen:  state.Screen,
			RunID:   state.RunID,
			Data: map[string]interface{}{
				"mode":              "deterministic_fallback",
				"provider_state":    s.providerState,
				"provider_recovery": "fallback",
			},
		})
		return nil
	}

	backoff := time.Duration(s.providerFailures) * 2 * time.Second
	s.emit(SessionEvent{
		Time:    time.Now(),
		Kind:    SessionEventStatus,
		Cycle:   s.cycle,
		Attempt: s.currentAttemptForState(state),
		Message: s.sayf("agent cycle failed (%v); retrying in %s", "agent cycle failed (%v); retrying in %s", err, backoff),
		Screen:  state.Screen,
		RunID:   state.RunID,
		Data: map[string]interface{}{
			"provider_failures": s.providerFailures,
			"mode":              s.cfg.ModeLabel(),
			"provider_state":    s.providerState,
			"provider_recovery": classifyProviderRecovery(err),
		},
	})

	timer := time.NewTimer(backoff)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (s *Session) handleDirectActionFailure(ctx context.Context, err error, state *game.StateSnapshot, request game.ActionRequest) bool {
	if err == nil || !isRecoverableActionError(err) {
		return false
	}

	recoveryKind := recoverableActionKind(err)
	if isSoftReplanDriftKind(recoveryKind) {
		if recoveryAlreadyReported(err) {
			return true
		}
		s.emit(SessionEvent{
			Time:    time.Now(),
			Kind:    SessionEventStatus,
			Cycle:   s.cycle,
			Attempt: s.currentAttemptForState(state),
			Message: s.sayf("soft seam recovery for %s; replanning on the latest state", "%s 閸涙垝鑵戞潪?seam閿涙稒顒滈崷銊ョ唨娴滃孩娓堕弬鎵Ц閹線鍣哥憴鍕灊", formatActionDebug(request)),
			Screen:  state.Screen,
			RunID:   state.RunID,
			Data: map[string]interface{}{
				"recovery_kind":  "soft_replan",
				"drift_kind":     recoveryKind,
				"provider_state": s.providerState,
			},
		})
		return true
	}

	if liveState, driftKind, ok := s.recoverPhaseAdvanceInvalidAction(ctx, err, state, request); ok {
		s.emit(SessionEvent{
			Time:    time.Now(),
			Kind:    SessionEventStatus,
			Cycle:   s.cycle,
			Attempt: s.currentAttemptForState(liveState),
			Message: s.sayf("soft seam recovery for %s; latest state already advanced to the next legal phase (%s)", "soft seam recovery for %s; latest state already advanced to the next legal phase (%s)", formatActionDebug(request), driftKind),
			Screen:  liveState.Screen,
			RunID:   liveState.RunID,
			State:   liveState,
			Data: map[string]interface{}{
				"recovery_kind":              "soft_replan",
				"drift_kind":                 driftKind,
				"provider_state":             s.providerState,
				"expected_state_summary":     decisionStateSummary(state),
				"live_state_summary":         decisionStateSummary(liveState),
				"expected_state_fingerprint": digestState(state),
				"live_state_fingerprint":     digestState(liveState),
			},
		})
		return true
	}

	s.repeatedInvalidActions++
	if s.failures != nil {
		s.failures.Record(digestState(state), request)
	}
	s.emit(SessionEvent{
		Time:    time.Now(),
		Kind:    SessionEventToolError,
		Cycle:   s.cycle,
		Attempt: s.currentAttemptForState(state),
		Message: fmt.Sprintf("recoverable action failure for %s: %v", formatActionDebug(request), err),
		Screen:  state.Screen,
		RunID:   state.RunID,
		Data: map[string]interface{}{
			"recovery_kind":  recoveryKind,
			"provider_state": s.providerState,
		},
	})

	if s.repeatedInvalidActions >= maxRepeatedInvalidActions {
		s.emit(SessionEvent{
			Time:    time.Now(),
			Kind:    SessionEventStatus,
			Cycle:   s.cycle,
			Attempt: s.currentAttemptForState(state),
			Message: s.sayf("repeated action failures (%d) on %s", "repeated action failures (%d) on %s", s.repeatedInvalidActions, state.Screen),
			Screen:  state.Screen,
			RunID:   state.RunID,
			Data: map[string]interface{}{
				"provider_state": s.providerState,
			},
		})
	}

	return true
}

func (s *Session) recoverPhaseAdvanceInvalidAction(ctx context.Context, err error, expectedState *game.StateSnapshot, request game.ActionRequest) (*game.StateSnapshot, string, bool) {
	if !isInvalidActionError(err) || !shouldTreatInvalidActionAsPhaseAdvance(request.Action) {
		return nil, "", false
	}
	if s == nil || s.runtime == nil || s.runtime.Client == nil {
		return nil, "", false
	}

	liveState, getErr := s.runtime.Client.GetState(ctx)
	if getErr != nil || liveState == nil {
		return nil, "", false
	}

	liveState, getErr = s.freshActionableStateForExecution(ctx, liveState)
	if getErr != nil || liveState == nil {
		return nil, "", false
	}

	driftKind := classifyStateDrift(expectedState, liveState)
	if isSoftReplanDriftKind(driftKind) {
		return liveState, driftKind, true
	}

	return nil, "", false
}

func shouldTreatInvalidActionAsPhaseAdvance(action string) bool {
	switch strings.TrimSpace(strings.ToLower(action)) {
	case "claim_reward", "proceed", "continue_after_game_over", "return_to_main_menu":
		return true
	default:
		return false
	}
}

func (s *Session) reflectIfNeeded(state *game.StateSnapshot) {
	if state == nil || !strings.EqualFold(state.Screen, "GAME_OVER") {
		return
	}

	runID := strings.TrimSpace(state.RunID)
	if runID != "" && s.reflectedRunIDs[runID] {
		return
	}

	attempt := s.currentAttemptForState(state)
	if attempt <= 0 {
		return
	}

	reflection := BuildAttemptReflection(s.currentStoryAttemptForState(state), state, s.todo, s.compact)
	if reflection == nil {
		return
	}

	if runID != "" {
		s.reflectedRunIDs[runID] = true
	}
	s.reflections = append(s.reflections, reflection)
	s.todo.ApplyReflection(reflection)
	s.compact.RecordReflection(reflection)
	_ = s.store.RecordAttemptReflection(reflection, s.cfg.Language)
	s.persistSessionResume(state)
	s.writeRunArtifacts(state)
	s.refreshGuidebook(true)

	s.emit(SessionEvent{
		Time:    time.Now(),
		Kind:    SessionEventReflection,
		Cycle:   s.cycle,
		Attempt: attempt,
		Message: reflection.PromptSummary(),
		Screen:  state.Screen,
		RunID:   state.RunID,
		State:   state,
		Data: s.appendGuideSnapshotData(map[string]interface{}{
			"story":                reflection.Story,
			"lessons":              reflection.Lessons,
			"lesson_buckets":       reflection.LessonBuckets.ToDataMap(),
			"attempt_lifecycle":    "game_over",
			"guidebook_path":       s.guide.GuidebookPath(),
			"living_codex_path":    s.guide.LivingCodexPath(),
			"combat_playbook_path": s.guide.CombatPlaybookPath(),
			"event_playbook_path":  s.guide.EventPlaybookPath(),
		}),
	})
}

func (s *Session) initialStatusMessage() string {
	if s.resumeState == nil || strings.TrimSpace(s.resumeState.LastRunID) == "" {
		return s.say("autoplay session started", "autoplay session started")
	}

	if screen := strings.TrimSpace(s.resumeState.LastScreen); screen != "" {
		return s.sayf(
			"autoplay session started (resuming run %s from %s)",
			"autoplay session started (resuming run %s from %s)",
			s.resumeState.LastRunID,
			screen,
		)
	}

	return s.sayf(
		"autoplay session started (resuming run %s)",
		"autoplay session started (resuming run %s)",
		s.resumeState.LastRunID,
	)
}

func (s *Session) writeRunArtifacts(state *game.StateSnapshot) {
	if s.store == nil {
		return
	}

	story := renderRunStoryDocument(
		s.reflections,
		s.currentStoryAttemptForState(state),
		state,
		s.todo,
		s.compact,
		s.world.Snapshot(),
		s.cfg.Language,
	)
	_ = s.store.WriteRunStory(story)

	guide := renderRunGuideDocument(
		s.reflections,
		s.currentStoryAttemptForState(state),
		state,
		s.todo,
		s.world.Snapshot(),
		s.cfg.Language,
	)
	_ = s.store.WriteRunGuide(guide)
}

func (s *Session) currentStoryAttemptForState(state *game.StateSnapshot) int {
	attempt := s.currentAttemptForState(state)
	if attempt <= 0 {
		return s.storyAttemptBase
	}

	return s.storyAttemptBase + attempt
}

func (s *Session) refreshGuidebook(force bool) {
	if s.guide == nil {
		return
	}

	now := time.Now()
	if !force && !s.lastGuideRefresh.IsZero() && now.Sub(s.lastGuideRefresh) < 20*time.Second {
		return
	}

	if snapshot, err := s.guide.Refresh(s.cfg.ArtifactsDir, "", s.cfg.Language); err == nil {
		s.guideSnapshot = snapshot
		s.lastGuideRefresh = now
	}
}

func (s *Session) appendGuideSnapshotData(data map[string]interface{}) map[string]interface{} {
	if data == nil {
		data = make(map[string]interface{})
	}
	if s.guideSnapshot == nil {
		return data
	}

	if s.guideSnapshot.RecentRecoveryWindow > 0 {
		data["guide_recent_recovery_window"] = s.guideSnapshot.RecentRecoveryWindow
	}
	if len(s.guideSnapshot.RecentRecoveryHotspots) > 0 {
		data["guide_recent_recovery_hotspots"] = append([]RecoveryHotspot(nil), s.guideSnapshot.RecentRecoveryHotspots...)
	}
	if len(s.guideSnapshot.WeightedRecoveryHotspots) > 0 {
		data["guide_weighted_recovery_hotspots"] = append([]RecoveryHotspot(nil), s.guideSnapshot.WeightedRecoveryHotspots...)
	}
	data["guide_rl_ready"] = s.guideSnapshot.RLReadiness.Ready
	data["guide_rl_status"] = s.guideSnapshot.RLReadiness.Status
	data["guide_rl_complete_runs"] = s.guideSnapshot.RLReadiness.CompleteRuns
	data["guide_rl_floor15_runs"] = s.guideSnapshot.RLReadiness.Floor15PlusRuns
	data["guide_rl_provider_backed_runs"] = s.guideSnapshot.RLReadiness.ProviderBackedRuns
	data["guide_rl_recent_clean_runs"] = s.guideSnapshot.RLReadiness.RecentCleanRuns
	data["guide_rl_required_runs"] = s.guideSnapshot.RLReadiness.RequiredRuns
	data["guide_rl_required_floor15"] = s.guideSnapshot.RLReadiness.RequiredFloor15
	data["guide_rl_required_provider_backed_runs"] = s.guideSnapshot.RLReadiness.RequiredProviderBackedRuns
	data["guide_rl_required_recent_clean_runs"] = s.guideSnapshot.RLReadiness.RequiredRecentCleanRuns
	data["guide_rl_stable_runtime"] = s.guideSnapshot.RLReadiness.StableRuntime
	data["guide_rl_knowledge_assets_ok"] = s.guideSnapshot.RLReadiness.KnowledgeAssetsOK
	data["guide_run_quality_complete_runs"] = s.guideSnapshot.RunQuality.CompleteRuns
	data["guide_run_quality_provider_backed_runs"] = s.guideSnapshot.RunQuality.ProviderBackedCompleteRuns
	data["guide_run_quality_clean_runs"] = s.guideSnapshot.RunQuality.CleanCompleteRuns
	data["guide_run_quality_recent_complete_runs"] = s.guideSnapshot.RunQuality.RecentCompleteRuns
	data["guide_run_quality_recent_provider_backed_runs"] = s.guideSnapshot.RunQuality.RecentProviderBackedRuns
	data["guide_run_quality_recent_clean_runs"] = s.guideSnapshot.RunQuality.RecentCleanRuns
	data["guide_run_quality_recent_fallback_runs"] = s.guideSnapshot.RunQuality.RecentFallbackRuns
	data["guide_run_quality_recent_provider_retry_runs"] = s.guideSnapshot.RunQuality.RecentProviderRetryRuns
	data["guide_run_quality_recent_tool_error_runs"] = s.guideSnapshot.RunQuality.RecentToolErrorRuns

	return data
}

func (s *Session) currentCombatPlan(state *game.StateSnapshot) *CombatPlan {
	if s.planner == nil {
		return nil
	}
	var codex *SeenContentRegistry
	if s.guideSnapshot != nil && s.guideSnapshot.SeenContent != nil {
		codex = s.guideSnapshot.SeenContent
	} else if s.world != nil {
		codex = s.world.Snapshot()
	}
	return s.planner.Analyze(state, codex, s.cfg.Language)
}

func (s *Session) promptPipeline() *PromptAssemblyPipeline {
	if s == nil || s.promptBuilder == nil {
		return NewPromptAssemblyPipeline()
	}
	return s.promptBuilder
}

func isRecoverableActionError(err error) bool {
	if err == nil {
		return false
	}

	text := strings.ToLower(err.Error())
	return strings.Contains(text, "invalid_action") ||
		strings.Contains(text, "invalid_target") ||
		strings.Contains(text, "state_unavailable") ||
		isTransientActionTransportError(err)
}

func isTransientActionTransportError(err error) bool {
	if err == nil {
		return false
	}

	text := strings.ToLower(err.Error())
	return strings.Contains(text, "wsarecv") ||
		strings.Contains(text, "forcibly closed by the remote host") ||
		strings.Contains(text, "connection reset") ||
		strings.Contains(text, "broken pipe") ||
		strings.Contains(text, " eof") ||
		strings.HasSuffix(text, ":eof") ||
		strings.HasSuffix(text, "eof") ||
		strings.Contains(text, "unexpected eof") ||
		strings.Contains(text, "connection aborted")
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
