package tui

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	agentruntime "spire2mind/internal/agent"
	"spire2mind/internal/config"
	"spire2mind/internal/game"
	"spire2mind/internal/i18n"
)

type Model struct {
	repoRoot                                string
	session                                 *agentruntime.Session
	loc                                     i18n.Localizer
	state                                   *game.StateSnapshot
	logs                                    []string
	status                                  string
	cost                                    float64
	turns                                   int
	lastCycleDurationMs                     int64
	lastInputTokens                         int
	lastOutputTokens                        int
	lastPromptSizeBytes                     int
	cycle                                   int
	attempt                                 int
	mode                                    string
	provider                                string
	providerState                           string
	agentAvailable                          bool
	forceDeterministic                      bool
	model                                   string
	endpoint                                string
	lastStatus                              string
	lastPrompt                              string
	lastAssistant                           string
	lastStreamerMood                        string
	lastStreamerCommentary                  string
	lastStreamerInsight                     string
	lastStreamerReflection                  string
	lastStreamerTTS                         string
	ttsProfileName                          string
	ttsProfileProvider                      string
	ttsProfileVoice                         string
	ttsProfileSpeed                         string
	streamerStyle                           string
	lastDecision                            string
	lastAction                              string
	lastTool                                string
	lastToolError                           string
	lastCompact                             string
	lastReflection                          string
	lastStop                                string
	lastDriftExpected                       string
	lastDriftLive                           string
	lastDriftKind                           string
	lastRecoveryKind                        string
	lastAttemptLifecycle                    string
	lastProviderRecovery                    string
	gameFastMode                            string
	gameFastModeChanged                     bool
	gameFastModePrevious                    string
	currentGoal                             string
	roomGoal                                string
	nextIntent                              string
	lastFailure                             string
	carryForwardPlan                        string
	carryForwardLessons                     []string
	carryForwardBuckets                     agentruntime.ReflectionLessonBuckets
	seenContentCounts                       map[string]int
	recentDiscoveries                       []string
	guideRecentWindow                       int
	guideRecentHotspots                     []agentruntime.RecoveryHotspot
	guideWeightedTrends                     []agentruntime.RecoveryHotspot
	guideRLReady                            bool
	guideRLStatus                           string
	guideRLCompleteRuns                     int
	guideRLFloor15Runs                      int
	guideRLProviderBackedRuns               int
	guideRLRecentCleanRuns                  int
	guideRLRequiredRuns                     int
	guideRLRequiredFloor                    int
	guideRLRequiredProviderBackedRuns       int
	guideRLRequiredRecentCleanRuns          int
	guideRLStable                           bool
	guideRLKnowledgeOK                      bool
	guideRunQualityCleanRuns                int
	guideRunQualityRecentProviderBackedRuns int
	guideRunQualityRecentFallbackRuns       int
	guideRunQualityRecentProviderRetryRuns  int
	guideRunQualityRecentToolErrorRuns      int
	guideRunQualityRecentMedianFloor        int
	guideRunQualityRecentBestFloor          int
	guideRunQualityRecentFloor7PlusRuns     int
	guideRunQualityRecentAct2EntryRuns      int
	guideRunQualityRecentDiedWithGoldRuns   int
	guideRunQualityRecentAverageDeathGold   int
	tacticalHints                           []string
	combatPlannerMode                       string
	combatPlanSummary                       string
	combatPlanGoal                          string
	combatPlanTarget                        string
	combatPlanReasons                       []string
	combatPlanCandidates                    []string
	lastReflectionStory                     string
	width                                   int
	height                                  int
	paused                                  bool
}

type sessionEventMsg struct {
	event agentruntime.SessionEvent
}

type sessionErrMsg struct {
	err error
}

type sessionDoneMsg struct{}

func New(ctx context.Context, cfg config.Config) (*Model, error) {
	session, err := agentruntime.StartSession(ctx, cfg)
	if err != nil {
		return nil, err
	}

	model := &Model{
		repoRoot:       cfg.RepoRoot,
		session:       session,
		loc:           i18n.New(cfg.Language),
		status:        "running",
		mode:          cfg.ModeLabel(),
		provider:      cfg.ProviderLabel(),
		providerState: initialProviderState(cfg),
		model:         cfg.Model,
		endpoint:      providerEndpoint(cfg),
		streamerStyle: cfg.StreamerStyle,
	}
	model.refreshTTSProfile()
	return model, nil
}

func (m *Model) Init() tea.Cmd {
	return listenSessionCmd(m.session.Events(), m.session.Errors())
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typed := msg.(type) {
	case tea.KeyMsg:
		switch typed.String() {
		case "q", "ctrl+c":
			m.Close()
			return m, tea.Quit
		case "p":
			if m.session != nil {
				if m.paused {
					m.session.Resume()
				} else {
					m.session.Pause()
				}
			}
			return m, nil
		}
	case tea.WindowSizeMsg:
		m.width = typed.Width
		m.height = typed.Height
	case sessionEventMsg:
		m.handleSessionEvent(typed.event)
		return m, listenSessionCmd(m.session.Events(), m.session.Errors())
	case sessionErrMsg:
		m.status = "error"
		m.logs = appendTrimmed(m.logs, "error: "+typed.err.Error(), 32)
		return m, nil
	case sessionDoneMsg:
		if m.status == "running" {
			m.status = "done"
		}
		return m, nil
	}

	return m, nil
}

func providerEndpoint(cfg config.Config) string {
	if cfg.UsesClaudeCLI() {
		return cfg.ClaudeCLIPath
	}
	return cfg.APIBaseURL
}

func initialProviderState(cfg config.Config) string {
	switch {
	case cfg.UsesClaudeCLI():
		return "healthy"
	case cfg.ModelProvider == config.ModelProviderAPI && cfg.HasModelConfig():
		return "healthy"
	default:
		return "deterministic"
	}
}

func (m *Model) Close() {
	if m.session != nil {
		m.session.Close()
	}
}

func (m *Model) handleSessionEvent(event agentruntime.SessionEvent) {
	m.refreshTTSProfile()
	m.cycle = event.Cycle
	if event.Attempt > 0 {
		m.attempt = event.Attempt
	}
	if event.State != nil {
		m.state = event.State
	}

	if event.Cost > 0 {
		m.cost = event.Cost
	}
	if event.Turns > 0 {
		m.turns = event.Turns
	}
	if duration, ok := event.Data["cycle_duration_ms"].(int64); ok && duration >= 0 {
		m.lastCycleDurationMs = duration
	}
	if duration, ok := event.Data["cycle_duration_ms"].(int); ok && duration >= 0 {
		m.lastCycleDurationMs = int64(duration)
	}
	if duration, ok := event.Data["cycle_duration_ms"].(float64); ok && duration >= 0 {
		m.lastCycleDurationMs = int64(duration)
	}
	if inputTokens, ok := event.Data["input_tokens"].(int); ok && inputTokens >= 0 {
		m.lastInputTokens = inputTokens
	}
	if inputTokens, ok := event.Data["input_tokens"].(float64); ok && inputTokens >= 0 {
		m.lastInputTokens = int(inputTokens)
	}
	if outputTokens, ok := event.Data["output_tokens"].(int); ok && outputTokens >= 0 {
		m.lastOutputTokens = outputTokens
	}
	if outputTokens, ok := event.Data["output_tokens"].(float64); ok && outputTokens >= 0 {
		m.lastOutputTokens = int(outputTokens)
	}
	if promptSize, ok := event.Data["prompt_size_bytes"].(int); ok && promptSize >= 0 {
		m.lastPromptSizeBytes = promptSize
	}
	if promptSize, ok := event.Data["prompt_size_bytes"].(float64); ok && promptSize >= 0 {
		m.lastPromptSizeBytes = int(promptSize)
	}
	if currentGoal, ok := event.Data["current_goal"].(string); ok && currentGoal != "" {
		m.currentGoal = currentGoal
	}
	if roomGoal, ok := event.Data["room_goal"].(string); ok && roomGoal != "" {
		m.roomGoal = roomGoal
	}
	if nextIntent, ok := event.Data["next_intent"].(string); ok && nextIntent != "" {
		m.nextIntent = nextIntent
	}
	if lastFailure, ok := event.Data["last_failure"].(string); ok {
		m.lastFailure = lastFailure
	}
	if carryForwardPlan, ok := event.Data["carry_forward_plan"].(string); ok && carryForwardPlan != "" {
		m.carryForwardPlan = carryForwardPlan
	}
	if carryForwardLessons, ok := event.Data["carry_forward_lessons"].([]string); ok && len(carryForwardLessons) > 0 {
		m.carryForwardLessons = append([]string(nil), carryForwardLessons...)
	}
	if carryForwardLessons, ok := event.Data["carry_forward_lessons"].([]interface{}); ok && len(carryForwardLessons) > 0 {
		m.carryForwardLessons = interfaceStrings(carryForwardLessons)
	}
	if buckets := agentruntime.LessonBucketsFromData(event.Data["carry_forward_buckets"]); !buckets.IsEmpty() {
		m.carryForwardBuckets = buckets
	}
	if counts := seenContentCountsFromData(event.Data["seen_content_counts"]); len(counts) > 0 {
		m.seenContentCounts = counts
	}
	if discoveries, ok := event.Data["recent_discoveries"].([]string); ok && len(discoveries) > 0 {
		m.recentDiscoveries = append([]string(nil), discoveries...)
	}
	if discoveries, ok := event.Data["recent_discoveries"].([]interface{}); ok && len(discoveries) > 0 {
		m.recentDiscoveries = interfaceStrings(discoveries)
	}
	if recentWindow, ok := event.Data["guide_recent_recovery_window"].(int); ok && recentWindow > 0 {
		m.guideRecentWindow = recentWindow
	}
	if recentWindow, ok := event.Data["guide_recent_recovery_window"].(float64); ok && recentWindow > 0 {
		m.guideRecentWindow = int(recentWindow)
	}
	if hotspots := recoveryHotspotsFromData(event.Data["guide_recent_recovery_hotspots"]); len(hotspots) > 0 {
		m.guideRecentHotspots = hotspots
	}
	if hotspots := recoveryHotspotsFromData(event.Data["guide_weighted_recovery_hotspots"]); len(hotspots) > 0 {
		m.guideWeightedTrends = hotspots
	}
	if ready, ok := event.Data["guide_rl_ready"].(bool); ok {
		m.guideRLReady = ready
	}
	if status, ok := event.Data["guide_rl_status"].(string); ok && strings.TrimSpace(status) != "" {
		m.guideRLStatus = status
	}
	if runs, ok := event.Data["guide_rl_complete_runs"].(int); ok {
		m.guideRLCompleteRuns = runs
	}
	if runs, ok := event.Data["guide_rl_complete_runs"].(float64); ok {
		m.guideRLCompleteRuns = int(runs)
	}
	if runs, ok := event.Data["guide_rl_floor15_runs"].(int); ok {
		m.guideRLFloor15Runs = runs
	}
	if runs, ok := event.Data["guide_rl_floor15_runs"].(float64); ok {
		m.guideRLFloor15Runs = int(runs)
	}
	if runs, ok := event.Data["guide_rl_provider_backed_runs"].(int); ok {
		m.guideRLProviderBackedRuns = runs
	}
	if runs, ok := event.Data["guide_rl_provider_backed_runs"].(float64); ok {
		m.guideRLProviderBackedRuns = int(runs)
	}
	if runs, ok := event.Data["guide_rl_recent_clean_runs"].(int); ok {
		m.guideRLRecentCleanRuns = runs
	}
	if runs, ok := event.Data["guide_rl_recent_clean_runs"].(float64); ok {
		m.guideRLRecentCleanRuns = int(runs)
	}
	if runs, ok := event.Data["guide_rl_required_runs"].(int); ok {
		m.guideRLRequiredRuns = runs
	}
	if runs, ok := event.Data["guide_rl_required_runs"].(float64); ok {
		m.guideRLRequiredRuns = int(runs)
	}
	if runs, ok := event.Data["guide_rl_required_floor15"].(int); ok {
		m.guideRLRequiredFloor = runs
	}
	if runs, ok := event.Data["guide_rl_required_floor15"].(float64); ok {
		m.guideRLRequiredFloor = int(runs)
	}
	if runs, ok := event.Data["guide_rl_required_provider_backed_runs"].(int); ok {
		m.guideRLRequiredProviderBackedRuns = runs
	}
	if runs, ok := event.Data["guide_rl_required_provider_backed_runs"].(float64); ok {
		m.guideRLRequiredProviderBackedRuns = int(runs)
	}
	if runs, ok := event.Data["guide_rl_required_recent_clean_runs"].(int); ok {
		m.guideRLRequiredRecentCleanRuns = runs
	}
	if runs, ok := event.Data["guide_rl_required_recent_clean_runs"].(float64); ok {
		m.guideRLRequiredRecentCleanRuns = int(runs)
	}
	if stable, ok := event.Data["guide_rl_stable_runtime"].(bool); ok {
		m.guideRLStable = stable
	}
	if knowledgeOK, ok := event.Data["guide_rl_knowledge_assets_ok"].(bool); ok {
		m.guideRLKnowledgeOK = knowledgeOK
	}
	if runs, ok := event.Data["guide_run_quality_clean_runs"].(int); ok {
		m.guideRunQualityCleanRuns = runs
	}
	if runs, ok := event.Data["guide_run_quality_clean_runs"].(float64); ok {
		m.guideRunQualityCleanRuns = int(runs)
	}
	if runs, ok := event.Data["guide_run_quality_recent_provider_backed_runs"].(int); ok {
		m.guideRunQualityRecentProviderBackedRuns = runs
	}
	if runs, ok := event.Data["guide_run_quality_recent_provider_backed_runs"].(float64); ok {
		m.guideRunQualityRecentProviderBackedRuns = int(runs)
	}
	if runs, ok := event.Data["guide_run_quality_recent_fallback_runs"].(int); ok {
		m.guideRunQualityRecentFallbackRuns = runs
	}
	if runs, ok := event.Data["guide_run_quality_recent_fallback_runs"].(float64); ok {
		m.guideRunQualityRecentFallbackRuns = int(runs)
	}
	if runs, ok := event.Data["guide_run_quality_recent_provider_retry_runs"].(int); ok {
		m.guideRunQualityRecentProviderRetryRuns = runs
	}
	if runs, ok := event.Data["guide_run_quality_recent_provider_retry_runs"].(float64); ok {
		m.guideRunQualityRecentProviderRetryRuns = int(runs)
	}
	if runs, ok := event.Data["guide_run_quality_recent_tool_error_runs"].(int); ok {
		m.guideRunQualityRecentToolErrorRuns = runs
	}
	if runs, ok := event.Data["guide_run_quality_recent_tool_error_runs"].(float64); ok {
		m.guideRunQualityRecentToolErrorRuns = int(runs)
	}
	if runs, ok := event.Data["guide_run_quality_recent_median_floor"].(int); ok {
		m.guideRunQualityRecentMedianFloor = runs
	}
	if runs, ok := event.Data["guide_run_quality_recent_median_floor"].(float64); ok {
		m.guideRunQualityRecentMedianFloor = int(runs)
	}
	if runs, ok := event.Data["guide_run_quality_recent_best_floor"].(int); ok {
		m.guideRunQualityRecentBestFloor = runs
	}
	if runs, ok := event.Data["guide_run_quality_recent_best_floor"].(float64); ok {
		m.guideRunQualityRecentBestFloor = int(runs)
	}
	if runs, ok := event.Data["guide_run_quality_recent_floor7_plus_runs"].(int); ok {
		m.guideRunQualityRecentFloor7PlusRuns = runs
	}
	if runs, ok := event.Data["guide_run_quality_recent_floor7_plus_runs"].(float64); ok {
		m.guideRunQualityRecentFloor7PlusRuns = int(runs)
	}
	if runs, ok := event.Data["guide_run_quality_recent_act2_entry_runs"].(int); ok {
		m.guideRunQualityRecentAct2EntryRuns = runs
	}
	if runs, ok := event.Data["guide_run_quality_recent_act2_entry_runs"].(float64); ok {
		m.guideRunQualityRecentAct2EntryRuns = int(runs)
	}
	if runs, ok := event.Data["guide_run_quality_recent_died_with_gold_runs"].(int); ok {
		m.guideRunQualityRecentDiedWithGoldRuns = runs
	}
	if runs, ok := event.Data["guide_run_quality_recent_died_with_gold_runs"].(float64); ok {
		m.guideRunQualityRecentDiedWithGoldRuns = int(runs)
	}
	if runs, ok := event.Data["guide_run_quality_recent_average_death_gold"].(int); ok {
		m.guideRunQualityRecentAverageDeathGold = runs
	}
	if runs, ok := event.Data["guide_run_quality_recent_average_death_gold"].(float64); ok {
		m.guideRunQualityRecentAverageDeathGold = int(runs)
	}
	if tacticalHints, ok := event.Data["tactical_hints"].([]string); ok {
		m.tacticalHints = append([]string(nil), tacticalHints...)
	}
	if tacticalHints, ok := event.Data["tactical_hints"].([]interface{}); ok {
		m.tacticalHints = interfaceStrings(tacticalHints)
	}
	if _, ok := event.Data["combat_plan"]; !ok && event.Kind == agentruntime.SessionEventState {
		m.clearCombatPlan()
	}
	if combatPlan, ok := event.Data["combat_plan"].(map[string]any); ok {
		if mode, ok := combatPlan["mode"].(string); ok && strings.TrimSpace(mode) != "" {
			m.combatPlannerMode = mode
		}
		if summary, ok := combatPlan["summary"].(string); ok && strings.TrimSpace(summary) != "" {
			m.combatPlanSummary = summary
		}
		if goal, ok := combatPlan["primary_goal"].(string); ok && strings.TrimSpace(goal) != "" {
			m.combatPlanGoal = goal
		}
		if target, ok := combatPlan["target_label"].(string); ok && strings.TrimSpace(target) != "" {
			m.combatPlanTarget = target
		}
		if reasons, ok := combatPlan["focus_reasons"].([]string); ok {
			m.combatPlanReasons = append([]string(nil), reasons...)
		}
		if reasons, ok := combatPlan["focus_reasons"].([]interface{}); ok {
			m.combatPlanReasons = interfaceStrings(reasons)
		}
		if candidates, ok := combatPlan["candidates"].([]interface{}); ok {
			labels := make([]string, 0, len(candidates))
			for _, raw := range candidates {
				item, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				action, _ := item["action"].(string)
				label, _ := item["label"].(string)
				score, _ := item["score"].(float64)
				if strings.TrimSpace(label) == "" {
					label = action
				}
				labels = append(labels, fmt.Sprintf("%s (%.2f)", label, score))
			}
			m.combatPlanCandidates = labels
		}
	}
	if gameFastMode, ok := event.Data["game_fast_mode"].(string); ok && strings.TrimSpace(gameFastMode) != "" {
		m.gameFastMode = gameFastMode
	}
	if changed, ok := event.Data["game_fast_mode_changed"].(bool); ok {
		m.gameFastModeChanged = changed
	}
	if previous, ok := event.Data["game_fast_mode_previous"].(string); ok && strings.TrimSpace(previous) != "" {
		m.gameFastModePrevious = previous
	}
	if providerState, ok := event.Data["provider_state"].(string); ok && strings.TrimSpace(providerState) != "" {
		m.providerState = providerState
	}
	if paused, ok := event.Data["paused"].(bool); ok {
		m.paused = paused
	}
	if agentAvailable, ok := event.Data["agent_available"].(bool); ok {
		m.agentAvailable = agentAvailable
	}
	if forceDeterministic, ok := event.Data["force_deterministic"].(bool); ok {
		m.forceDeterministic = forceDeterministic
	}
	if expectedSummary, ok := event.Data["expected_state_summary"].(string); ok {
		m.lastDriftExpected = expectedSummary
	}
	if liveSummary, ok := event.Data["live_state_summary"].(string); ok {
		m.lastDriftLive = liveSummary
	}
	if driftKind, ok := event.Data["drift_kind"].(string); ok && strings.TrimSpace(driftKind) != "" {
		m.lastDriftKind = driftKind
	}
	if recoveryKind, ok := event.Data["recovery_kind"].(string); ok && strings.TrimSpace(recoveryKind) != "" {
		m.lastRecoveryKind = recoveryKind
	}
	if attemptLifecycle, ok := event.Data["attempt_lifecycle"].(string); ok && strings.TrimSpace(attemptLifecycle) != "" {
		m.lastAttemptLifecycle = attemptLifecycle
	}
	if providerRecovery, ok := event.Data["provider_recovery"].(string); ok && strings.TrimSpace(providerRecovery) != "" {
		m.lastProviderRecovery = providerRecovery
	}

	switch event.Kind {
	case agentruntime.SessionEventStatus:
		m.lastStatus = event.Message
		if action, ok := event.Data["action"].(string); ok && action != "" {
			if reason, ok := event.Data["reason"].(string); ok && reason != "" {
				m.lastDecision = action + " - " + reason
			} else {
				m.lastDecision = action
			}
		}
		if event.Message != "" {
			m.logs = appendTrimmed(m.logs, "[status] "+event.Message, 32)
		}
	case agentruntime.SessionEventPrompt:
		m.lastPrompt = event.Message
		if prompt, ok := event.Data["prompt"].(string); ok {
			m.lastPrompt = prompt
		}
		m.logs = appendTrimmed(m.logs, "[prompt] "+compactValue(m.lastPrompt), 32)
	case agentruntime.SessionEventAssistant:
		m.lastAssistant = event.Message
		m.logs = appendTrimmed(m.logs, "[assistant] "+event.Message, 32)
	case agentruntime.SessionEventStreamer:
		m.lastStreamerCommentary = event.Message
		if mood, ok := event.Data["mood"].(string); ok {
			m.lastStreamerMood = mood
		}
		if insight, ok := event.Data["game_insight"].(string); ok {
			m.lastStreamerInsight = insight
		}
		if reflection, ok := event.Data["life_reflection"].(string); ok {
			m.lastStreamerReflection = reflection
		}
		if tts, ok := event.Data["tts_text"].(string); ok {
			m.lastStreamerTTS = tts
		}
		m.logs = appendTrimmed(m.logs, "[streamer] "+event.Message, 32)
	case agentruntime.SessionEventTool:
		label := event.Tool
		if event.Action != "" {
			label += " " + event.Action
		}
		m.lastTool = label
		m.logs = appendTrimmed(m.logs, "[tool] "+label, 32)
	case agentruntime.SessionEventToolError:
		m.lastToolError = event.Message
		m.logs = appendTrimmed(m.logs, "[tool-error] "+event.Message, 32)
	case agentruntime.SessionEventAction:
		label := event.Action
		if event.Message != "" {
			label += " - " + event.Message
		}
		m.lastAction = label
		m.logs = appendTrimmed(m.logs, "[action] "+label, 32)
	case agentruntime.SessionEventCompact:
		m.lastCompact = event.Message
		m.logs = appendTrimmed(m.logs, "[compact] "+event.Message, 32)
	case agentruntime.SessionEventReflection:
		m.lastReflection = event.Message
		if story, ok := event.Data["story"].(string); ok && story != "" {
			m.lastReflectionStory = story
		}
		m.logs = appendTrimmed(m.logs, "[reflection] "+event.Message, 32)
	case agentruntime.SessionEventStop:
		m.lastStop = event.Message
		m.logs = appendTrimmed(m.logs, "[stop] "+event.Message, 32)
		m.status = "done"
	}
}

func (m *Model) clearCombatPlan() {
	m.combatPlannerMode = ""
	m.combatPlanSummary = ""
	m.combatPlanGoal = ""
	m.combatPlanTarget = ""
	m.combatPlanReasons = nil
	m.combatPlanCandidates = nil
}

func seenContentCountsFromData(value interface{}) map[string]int {
	switch typed := value.(type) {
	case map[string]int:
		return typed
	case map[string]interface{}:
		counts := make(map[string]int, len(typed))
		for key, raw := range typed {
			switch cast := raw.(type) {
			case int:
				counts[key] = cast
			case int32:
				counts[key] = int(cast)
			case int64:
				counts[key] = int(cast)
			case float64:
				counts[key] = int(cast)
			}
		}
		return counts
	default:
		return nil
	}
}

func recoveryHotspotsFromData(value interface{}) []agentruntime.RecoveryHotspot {
	switch typed := value.(type) {
	case []agentruntime.RecoveryHotspot:
		return append([]agentruntime.RecoveryHotspot(nil), typed...)
	case []interface{}:
		hotspots := make([]agentruntime.RecoveryHotspot, 0, len(typed))
		for _, raw := range typed {
			item, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			hotspots = append(hotspots, agentruntime.RecoveryHotspot{
				RecoveryKind: stringMapValue(item, "recovery_kind"),
				DriftKind:    stringMapValue(item, "drift_kind"),
				Count:        int(numberMapValue(item, "count")),
				Score:        numberMapValue(item, "score"),
			})
		}
		return hotspots
	default:
		return nil
	}
}

func stringMapValue(item map[string]any, key string) string {
	value, _ := item[key].(string)
	return strings.TrimSpace(value)
}

func numberMapValue(item map[string]any, key string) float64 {
	switch typed := item[key].(type) {
	case float64:
		return typed
	case float32:
		return float64(typed)
	case int:
		return float64(typed)
	case int32:
		return float64(typed)
	case int64:
		return float64(typed)
	default:
		return 0
	}
}

func listenSessionCmd(events <-chan agentruntime.SessionEvent, errs <-chan error) tea.Cmd {
	return func() tea.Msg {
		select {
		case event, ok := <-events:
			if !ok {
				select {
				case err, ok := <-errs:
					if ok && err != nil {
						return sessionErrMsg{err: err}
					}
				default:
				}
				return sessionDoneMsg{}
			}
			return sessionEventMsg{event: event}
		case err, ok := <-errs:
			if ok && err != nil {
				return sessionErrMsg{err: err}
			}
			return sessionDoneMsg{}
		}
	}
}
