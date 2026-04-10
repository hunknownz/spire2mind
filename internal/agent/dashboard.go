package agentruntime

import (
	"fmt"
	"strings"

	"spire2mind/internal/config"
	"spire2mind/internal/game"
	"spire2mind/internal/i18n"
)

type DebugDashboard struct {
	loc                                     i18n.Localizer
	mode                                    string
	provider                                string
	providerState                           string
	agentAvailable                          bool
	forceDeterministic                      bool
	model                                   string
	modelContext                            int
	backend                                 string
	status                                  string
	state                                   *game.StateSnapshot
	cycle                                   int
	attempt                                 int
	cost                                    float64
	turns                                   int
	lastCycleDurationMs                     int64
	lastInputTokens                         int
	lastOutputTokens                        int
	lastPromptSizeBytes                     int
	lastStatus                              string
	lastPrompt                              string
	lastAssistant                           string
	lastStreamerMood                        string
	lastStreamerCommentary                  string
	lastStreamerInsight                     string
	lastStreamerReflection                  string
	lastStreamerTTS                         string
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
	currentGoal                             string
	roomGoal                                string
	nextIntent                              string
	lastFailure                             string
	carryForwardPlan                        string
	carryForwardLessons                     []string
	carryForwardBuckets                     ReflectionLessonBuckets
	seenContentCounts                       map[string]int
	recentDiscoveries                       []string
	gamePrefsPath                           string
	gameFastMode                            string
	gameFastModeChanged                     bool
	gameFastModePrevious                    string
	runIndexPath                            string
	guidebookPath                           string
	livingCodexPath                         string
	combatPlaybookPath                      string
	eventPlaybookPath                       string
	guideRecentWindow                       int
	guideRecentHotspots                     []RecoveryHotspot
	guideWeightedTrends                     []RecoveryHotspot
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
	depthOdds                               []string
	combatPlannerMode                       string
	combatPlanSummary                       string
	combatPlanGoal                          string
	combatPlanTarget                        string
	combatPlanReasons                       []string
	combatPlanCandidates                    []string
	lastReflectionStory                     string
	logs                                    []string
}

func NewDebugDashboard(cfg config.Config) *DebugDashboard {
	return &DebugDashboard{
		loc:           i18n.New(cfg.Language),
		mode:          cfg.ModeLabel(),
		provider:      cfg.ProviderLabel(),
		providerState: initialProviderState(cfg),
		model:         cfg.Model,
		modelContext:  cfg.ModelContext,
		backend:       dashboardProviderEndpoint(cfg),
		status:        "running",
	}
}

func (d *DebugDashboard) ApplyEvent(event SessionEvent) string {
	d.cycle = max(d.cycle, event.Cycle)
	if event.Attempt > 0 {
		d.attempt = event.Attempt
	}
	if event.State != nil {
		d.state = event.State
	}
	if event.Cost > 0 {
		d.cost = event.Cost
	}
	if event.Turns > 0 {
		d.turns = event.Turns
	}
	if duration, ok := event.Data["cycle_duration_ms"].(int64); ok && duration >= 0 {
		d.lastCycleDurationMs = duration
	}
	if duration, ok := event.Data["cycle_duration_ms"].(int); ok && duration >= 0 {
		d.lastCycleDurationMs = int64(duration)
	}
	if duration, ok := event.Data["cycle_duration_ms"].(float64); ok && duration >= 0 {
		d.lastCycleDurationMs = int64(duration)
	}
	if inputTokens, ok := event.Data["input_tokens"].(int); ok && inputTokens >= 0 {
		d.lastInputTokens = inputTokens
	}
	if inputTokens, ok := event.Data["input_tokens"].(float64); ok && inputTokens >= 0 {
		d.lastInputTokens = int(inputTokens)
	}
	if outputTokens, ok := event.Data["output_tokens"].(int); ok && outputTokens >= 0 {
		d.lastOutputTokens = outputTokens
	}
	if outputTokens, ok := event.Data["output_tokens"].(float64); ok && outputTokens >= 0 {
		d.lastOutputTokens = int(outputTokens)
	}
	if promptSize, ok := event.Data["prompt_size_bytes"].(int); ok && promptSize >= 0 {
		d.lastPromptSizeBytes = promptSize
	}
	if promptSize, ok := event.Data["prompt_size_bytes"].(float64); ok && promptSize >= 0 {
		d.lastPromptSizeBytes = int(promptSize)
	}
	if currentGoal, ok := event.Data["current_goal"].(string); ok && currentGoal != "" {
		d.currentGoal = currentGoal
	}
	if roomGoal, ok := event.Data["room_goal"].(string); ok && roomGoal != "" {
		d.roomGoal = roomGoal
	}
	if nextIntent, ok := event.Data["next_intent"].(string); ok && nextIntent != "" {
		d.nextIntent = nextIntent
	}
	if lastFailure, ok := event.Data["last_failure"].(string); ok {
		d.lastFailure = lastFailure
	}
	if carryForwardPlan, ok := event.Data["carry_forward_plan"].(string); ok && carryForwardPlan != "" {
		d.carryForwardPlan = carryForwardPlan
	}
	if carryForwardLessons, ok := event.Data["carry_forward_lessons"].([]string); ok && len(carryForwardLessons) > 0 {
		d.carryForwardLessons = append([]string(nil), carryForwardLessons...)
	}
	if carryForwardLessons, ok := event.Data["carry_forward_lessons"].([]interface{}); ok && len(carryForwardLessons) > 0 {
		d.carryForwardLessons = stringifySlice(carryForwardLessons)
	}
	if buckets := LessonBucketsFromData(event.Data["carry_forward_buckets"]); !buckets.IsEmpty() {
		d.carryForwardBuckets = buckets
	}
	if counts := seenContentCountsFromData(event.Data["seen_content_counts"]); len(counts) > 0 {
		d.seenContentCounts = counts
	}
	if discoveries, ok := event.Data["recent_discoveries"].([]string); ok && len(discoveries) > 0 {
		d.recentDiscoveries = append([]string(nil), discoveries...)
	}
	if discoveries, ok := event.Data["recent_discoveries"].([]interface{}); ok && len(discoveries) > 0 {
		d.recentDiscoveries = stringifySlice(discoveries)
	}
	if gamePrefsPath, ok := event.Data["game_prefs_path"].(string); ok && strings.TrimSpace(gamePrefsPath) != "" {
		d.gamePrefsPath = gamePrefsPath
	}
	if runIndexPath, ok := event.Data["run_index_path"].(string); ok && strings.TrimSpace(runIndexPath) != "" {
		d.runIndexPath = runIndexPath
	}
	if guidebookPath, ok := event.Data["guidebook_path"].(string); ok && strings.TrimSpace(guidebookPath) != "" {
		d.guidebookPath = guidebookPath
	}
	if livingCodexPath, ok := event.Data["living_codex_path"].(string); ok && strings.TrimSpace(livingCodexPath) != "" {
		d.livingCodexPath = livingCodexPath
	}
	if combatPlaybookPath, ok := event.Data["combat_playbook_path"].(string); ok && strings.TrimSpace(combatPlaybookPath) != "" {
		d.combatPlaybookPath = combatPlaybookPath
	}
	if eventPlaybookPath, ok := event.Data["event_playbook_path"].(string); ok && strings.TrimSpace(eventPlaybookPath) != "" {
		d.eventPlaybookPath = eventPlaybookPath
	}
	if recentWindow, ok := event.Data["guide_recent_recovery_window"].(int); ok && recentWindow > 0 {
		d.guideRecentWindow = recentWindow
	}
	if recentWindow, ok := event.Data["guide_recent_recovery_window"].(float64); ok && recentWindow > 0 {
		d.guideRecentWindow = int(recentWindow)
	}
	if hotspots := recoveryHotspotsFromData(event.Data["guide_recent_recovery_hotspots"]); len(hotspots) > 0 {
		d.guideRecentHotspots = hotspots
	}
	if hotspots := recoveryHotspotsFromData(event.Data["guide_weighted_recovery_hotspots"]); len(hotspots) > 0 {
		d.guideWeightedTrends = hotspots
	}
	if ready, ok := event.Data["guide_rl_ready"].(bool); ok {
		d.guideRLReady = ready
	}
	if status, ok := event.Data["guide_rl_status"].(string); ok && strings.TrimSpace(status) != "" {
		d.guideRLStatus = status
	}
	if runs, ok := event.Data["guide_rl_complete_runs"].(int); ok {
		d.guideRLCompleteRuns = runs
	}
	if runs, ok := event.Data["guide_rl_complete_runs"].(float64); ok {
		d.guideRLCompleteRuns = int(runs)
	}
	if runs, ok := event.Data["guide_rl_floor15_runs"].(int); ok {
		d.guideRLFloor15Runs = runs
	}
	if runs, ok := event.Data["guide_rl_floor15_runs"].(float64); ok {
		d.guideRLFloor15Runs = int(runs)
	}
	if runs, ok := event.Data["guide_rl_provider_backed_runs"].(int); ok {
		d.guideRLProviderBackedRuns = runs
	}
	if runs, ok := event.Data["guide_rl_provider_backed_runs"].(float64); ok {
		d.guideRLProviderBackedRuns = int(runs)
	}
	if runs, ok := event.Data["guide_rl_recent_clean_runs"].(int); ok {
		d.guideRLRecentCleanRuns = runs
	}
	if runs, ok := event.Data["guide_rl_recent_clean_runs"].(float64); ok {
		d.guideRLRecentCleanRuns = int(runs)
	}
	if runs, ok := event.Data["guide_rl_required_runs"].(int); ok {
		d.guideRLRequiredRuns = runs
	}
	if runs, ok := event.Data["guide_rl_required_runs"].(float64); ok {
		d.guideRLRequiredRuns = int(runs)
	}
	if runs, ok := event.Data["guide_rl_required_floor15"].(int); ok {
		d.guideRLRequiredFloor = runs
	}
	if runs, ok := event.Data["guide_rl_required_floor15"].(float64); ok {
		d.guideRLRequiredFloor = int(runs)
	}
	if runs, ok := event.Data["guide_rl_required_provider_backed_runs"].(int); ok {
		d.guideRLRequiredProviderBackedRuns = runs
	}
	if runs, ok := event.Data["guide_rl_required_provider_backed_runs"].(float64); ok {
		d.guideRLRequiredProviderBackedRuns = int(runs)
	}
	if runs, ok := event.Data["guide_rl_required_recent_clean_runs"].(int); ok {
		d.guideRLRequiredRecentCleanRuns = runs
	}
	if runs, ok := event.Data["guide_rl_required_recent_clean_runs"].(float64); ok {
		d.guideRLRequiredRecentCleanRuns = int(runs)
	}
	if stable, ok := event.Data["guide_rl_stable_runtime"].(bool); ok {
		d.guideRLStable = stable
	}
	if knowledgeOK, ok := event.Data["guide_rl_knowledge_assets_ok"].(bool); ok {
		d.guideRLKnowledgeOK = knowledgeOK
	}
	if runs, ok := event.Data["guide_run_quality_clean_runs"].(int); ok {
		d.guideRunQualityCleanRuns = runs
	}
	if runs, ok := event.Data["guide_run_quality_clean_runs"].(float64); ok {
		d.guideRunQualityCleanRuns = int(runs)
	}
	if runs, ok := event.Data["guide_run_quality_recent_provider_backed_runs"].(int); ok {
		d.guideRunQualityRecentProviderBackedRuns = runs
	}
	if runs, ok := event.Data["guide_run_quality_recent_provider_backed_runs"].(float64); ok {
		d.guideRunQualityRecentProviderBackedRuns = int(runs)
	}
	if runs, ok := event.Data["guide_run_quality_recent_fallback_runs"].(int); ok {
		d.guideRunQualityRecentFallbackRuns = runs
	}
	if runs, ok := event.Data["guide_run_quality_recent_fallback_runs"].(float64); ok {
		d.guideRunQualityRecentFallbackRuns = int(runs)
	}
	if runs, ok := event.Data["guide_run_quality_recent_provider_retry_runs"].(int); ok {
		d.guideRunQualityRecentProviderRetryRuns = runs
	}
	if runs, ok := event.Data["guide_run_quality_recent_provider_retry_runs"].(float64); ok {
		d.guideRunQualityRecentProviderRetryRuns = int(runs)
	}
	if runs, ok := event.Data["guide_run_quality_recent_tool_error_runs"].(int); ok {
		d.guideRunQualityRecentToolErrorRuns = runs
	}
	if runs, ok := event.Data["guide_run_quality_recent_tool_error_runs"].(float64); ok {
		d.guideRunQualityRecentToolErrorRuns = int(runs)
	}
	if runs, ok := event.Data["guide_run_quality_recent_median_floor"].(int); ok {
		d.guideRunQualityRecentMedianFloor = runs
	}
	if runs, ok := event.Data["guide_run_quality_recent_median_floor"].(float64); ok {
		d.guideRunQualityRecentMedianFloor = int(runs)
	}
	if runs, ok := event.Data["guide_run_quality_recent_best_floor"].(int); ok {
		d.guideRunQualityRecentBestFloor = runs
	}
	if runs, ok := event.Data["guide_run_quality_recent_best_floor"].(float64); ok {
		d.guideRunQualityRecentBestFloor = int(runs)
	}
	if runs, ok := event.Data["guide_run_quality_recent_floor7_plus_runs"].(int); ok {
		d.guideRunQualityRecentFloor7PlusRuns = runs
	}
	if runs, ok := event.Data["guide_run_quality_recent_floor7_plus_runs"].(float64); ok {
		d.guideRunQualityRecentFloor7PlusRuns = int(runs)
	}
	if runs, ok := event.Data["guide_run_quality_recent_act2_entry_runs"].(int); ok {
		d.guideRunQualityRecentAct2EntryRuns = runs
	}
	if runs, ok := event.Data["guide_run_quality_recent_act2_entry_runs"].(float64); ok {
		d.guideRunQualityRecentAct2EntryRuns = int(runs)
	}
	if runs, ok := event.Data["guide_run_quality_recent_died_with_gold_runs"].(int); ok {
		d.guideRunQualityRecentDiedWithGoldRuns = runs
	}
	if runs, ok := event.Data["guide_run_quality_recent_died_with_gold_runs"].(float64); ok {
		d.guideRunQualityRecentDiedWithGoldRuns = int(runs)
	}
	if runs, ok := event.Data["guide_run_quality_recent_average_death_gold"].(int); ok {
		d.guideRunQualityRecentAverageDeathGold = runs
	}
	if runs, ok := event.Data["guide_run_quality_recent_average_death_gold"].(float64); ok {
		d.guideRunQualityRecentAverageDeathGold = int(runs)
	}
	if gameFastMode, ok := event.Data["game_fast_mode"].(string); ok && strings.TrimSpace(gameFastMode) != "" {
		d.gameFastMode = gameFastMode
	}
	if changed, ok := event.Data["game_fast_mode_changed"].(bool); ok {
		d.gameFastModeChanged = changed
	}
	if previous, ok := event.Data["game_fast_mode_previous"].(string); ok && strings.TrimSpace(previous) != "" {
		d.gameFastModePrevious = previous
	}
	if tacticalHints, ok := event.Data["tactical_hints"].([]string); ok {
		d.tacticalHints = append([]string(nil), tacticalHints...)
	}
	if tacticalHints, ok := event.Data["tactical_hints"].([]interface{}); ok {
		d.tacticalHints = stringifySlice(tacticalHints)
	}
	if depthOdds, ok := event.Data["depth_odds"].([]string); ok {
		d.depthOdds = append([]string(nil), depthOdds...)
	}
	if depthOdds, ok := event.Data["depth_odds"].([]interface{}); ok {
		d.depthOdds = stringifySlice(depthOdds)
	}
	if _, ok := event.Data["combat_plan"]; !ok && event.Kind == SessionEventState {
		d.clearCombatPlan()
	}
	if combatPlan, ok := event.Data["combat_plan"].(map[string]any); ok {
		if mode := stringData(combatPlan["mode"]); mode != "" {
			d.combatPlannerMode = mode
		}
		if summary := stringData(combatPlan["summary"]); summary != "" {
			d.combatPlanSummary = summary
		}
		if goal := stringData(combatPlan["primary_goal"]); goal != "" {
			d.combatPlanGoal = goal
		}
		if target := stringData(combatPlan["target_label"]); target != "" {
			d.combatPlanTarget = target
		}
		if reasons, ok := combatPlan["focus_reasons"].([]string); ok {
			d.combatPlanReasons = append([]string(nil), reasons...)
		}
		if reasons, ok := combatPlan["focus_reasons"].([]interface{}); ok {
			d.combatPlanReasons = stringifySlice(reasons)
		}
		if candidates, ok := combatPlan["candidates"].([]interface{}); ok {
			labels := make([]string, 0, len(candidates))
			for _, raw := range candidates {
				item, ok := raw.(map[string]any)
				if !ok {
					continue
				}
				action := stringData(item["action"])
				label := stringData(item["label"])
				score := floatData(item["score"])
				trade := stringData(item["trade_summary"])
				if label == "" {
					label = action
				}
				if strings.TrimSpace(trade) != "" {
					labels = append(labels, fmt.Sprintf("%s (%.2f | %s)", label, score, trade))
					continue
				}
				labels = append(labels, fmt.Sprintf("%s (%.2f)", label, score))
			}
			d.combatPlanCandidates = labels
		}
	}
	if providerState, ok := event.Data["provider_state"].(string); ok && strings.TrimSpace(providerState) != "" {
		d.providerState = providerState
	}
	if available, ok := event.Data["agent_available"].(bool); ok {
		d.agentAvailable = available
	}
	if forced, ok := event.Data["force_deterministic"].(bool); ok {
		d.forceDeterministic = forced
	}
	if expectedSummary, ok := event.Data["expected_state_summary"].(string); ok {
		d.lastDriftExpected = expectedSummary
	}
	if liveSummary, ok := event.Data["live_state_summary"].(string); ok {
		d.lastDriftLive = liveSummary
	}
	if driftKind, ok := event.Data["drift_kind"].(string); ok && strings.TrimSpace(driftKind) != "" {
		d.lastDriftKind = driftKind
	}
	if recoveryKind, ok := event.Data["recovery_kind"].(string); ok && strings.TrimSpace(recoveryKind) != "" {
		d.lastRecoveryKind = recoveryKind
	}
	if attemptLifecycle, ok := event.Data["attempt_lifecycle"].(string); ok && strings.TrimSpace(attemptLifecycle) != "" {
		d.lastAttemptLifecycle = attemptLifecycle
	}
	if providerRecovery, ok := event.Data["provider_recovery"].(string); ok && strings.TrimSpace(providerRecovery) != "" {
		d.lastProviderRecovery = providerRecovery
	}

	switch event.Kind {
	case SessionEventStatus:
		d.lastStatus = event.Message
		d.appendLog("[status] " + event.Message)
		if mode, ok := event.Data["mode"].(string); ok && mode != "" {
			d.status = mode
		}
		if reason, ok := event.Data["reason"].(string); ok && reason != "" {
			d.lastDecision = fmt.Sprintf("%s (%s)", event.Data["action"], reason)
		}
	case SessionEventPrompt:
		d.lastPrompt = event.Message
		if prompt, ok := event.Data["prompt"].(string); ok {
			d.lastPrompt = prompt
		}
		d.appendLog("[prompt] " + compactText(event.Message, 96))
	case SessionEventAssistant:
		d.lastAssistant = event.Message
		d.appendLog("[assistant] " + event.Message)
	case SessionEventStreamer:
		d.lastStreamerCommentary = event.Message
		if mood, ok := event.Data["mood"].(string); ok {
			d.lastStreamerMood = mood
		}
		if insight, ok := event.Data["game_insight"].(string); ok {
			d.lastStreamerInsight = insight
		}
		if reflection, ok := event.Data["life_reflection"].(string); ok {
			d.lastStreamerReflection = reflection
		}
		if tts, ok := event.Data["tts_text"].(string); ok {
			d.lastStreamerTTS = tts
		}
		d.appendLog("[streamer] " + event.Message)
	case SessionEventTool:
		d.lastTool = strings.TrimSpace(strings.TrimSpace(event.Tool + " " + event.Action))
		d.appendLog("[tool] " + d.lastTool)
	case SessionEventToolError:
		d.lastToolError = event.Message
		d.appendLog("[tool-error] " + event.Message)
	case SessionEventAction:
		d.lastAction = strings.TrimSpace(strings.TrimSpace(event.Action + " " + event.Message))
		d.appendLog("[action] " + d.lastAction)
	case SessionEventCompact:
		d.lastCompact = event.Message
		d.appendLog("[compact] " + event.Message)
	case SessionEventReflection:
		d.lastReflection = event.Message
		if story, ok := event.Data["story"].(string); ok && story != "" {
			d.lastReflectionStory = story
		}
		d.appendLog("[reflection] " + event.Message)
	case SessionEventStop:
		d.status = "done"
		d.lastStop = event.Message
		d.appendLog("[stop] " + event.Message)
	}

	return d.RenderMarkdown()
}

func (d *DebugDashboard) RenderMarkdown() string {
	loc := d.loc
	lines := []string{
		"# " + loc.Label("Spire2Mind Dashboard", "Spire2Mind 仪表盘"),
		"",
		"## " + loc.Label("Runtime", "运行时"),
		"",
		fmt.Sprintf("- %s: `%s`", loc.Label("Status", "状态"), valueOrDash(d.status)),
		fmt.Sprintf("- %s: `%s`", loc.Label("Mode", "模式"), valueOrDash(d.mode)),
		fmt.Sprintf("- %s: `%s`", loc.Label("Provider", "提供方"), valueOrDash(d.provider)),
		fmt.Sprintf("- %s: `%s`", loc.Label("Provider state", "提供方状态"), valueOrDash(d.providerState)),
		fmt.Sprintf("- %s: `%t`", loc.Label("Agent available", "Agent 可用"), d.agentAvailable),
		fmt.Sprintf("- %s: `%t`", loc.Label("Forced deterministic", "强制确定性"), d.forceDeterministic),
		fmt.Sprintf("- %s: `%s`", loc.Label("Game fast mode", "游戏加速模式"), valueOrDash(d.gameFastMode)),
		fmt.Sprintf("- %s: `%s`", loc.Label("Model", "模型"), valueOrDash(d.model)),
		fmt.Sprintf("- %s: `%d`", loc.Label("Model context", "模型上下文"), d.modelContext),
		fmt.Sprintf("- %s: `%s`", loc.Label("Backend", "后端"), valueOrDash(d.backend)),
		fmt.Sprintf("- %s: `%s`", loc.Label("Run index", "Run 索引"), valueOrDash(d.runIndexPath)),
		fmt.Sprintf("- %s: `%s`", loc.Label("Guidebook", "攻略总览"), valueOrDash(d.guidebookPath)),
		fmt.Sprintf("- %s: `%s`", loc.Label("Living codex", "动态图鉴"), valueOrDash(d.livingCodexPath)),
		fmt.Sprintf("- %s: `%s`", loc.Label("Combat playbook", "战斗手册"), valueOrDash(d.combatPlaybookPath)),
		fmt.Sprintf("- %s: `%s`", loc.Label("Event playbook", "事件手册"), valueOrDash(d.eventPlaybookPath)),
		fmt.Sprintf("- %s: `%d`", loc.Label("Cycle", "循环"), d.cycle),
		fmt.Sprintf("- %s: `%d`", loc.Label("Attempt", "尝试"), d.attempt),
		fmt.Sprintf("- %s: `%d`", loc.Label("Turns", "回合"), d.turns),
		fmt.Sprintf("- %s: `%.4f`", loc.Label("Cost", "成本"), d.cost),
	}

	lines = append(lines, "", "## "+loc.Label("State", "状态"), "")
	lines = append(lines, StateSummaryLinesFor(d.state, loc.Language())...)

	lines = append(lines, "", "## "+loc.Label("Goals", "目标"), "")
	lines = append(lines,
		fmt.Sprintf("- %s: %s", loc.Label("Current goal", "当前目标"), valueOrDash(d.currentGoal)),
		fmt.Sprintf("- %s: %s", loc.Label("Room goal", "房间目标"), valueOrDash(d.roomGoal)),
		fmt.Sprintf("- %s: %s", loc.Label("Next intent", "下一意图"), valueOrDash(d.nextIntent)),
		fmt.Sprintf("- %s: %s", loc.Label("Recent failure", "最近失败"), valueOrDash(d.lastFailure)),
	)

	lines = append(lines, "", "## "+loc.Label("Carry-Forward Memory", "跨局记忆"), "")
	lines = append(lines, fmt.Sprintf("- %s: %s", loc.Label("Plan", "计划"), valueOrDash(d.carryForwardPlan)))
	if !d.carryForwardBuckets.IsEmpty() {
		lines = append(lines, "- "+loc.Label("Lessons by category", "分组经验")+":")
		for _, section := range d.carryForwardBuckets.Sections() {
			lines = append(lines, "  - "+section.Title+":")
			for _, lesson := range section.Lessons {
				lines = append(lines, "    - "+lesson)
			}
		}
	}
	remainingLessons := UncategorizedLessons(d.carryForwardLessons, d.carryForwardBuckets, 8)
	if len(remainingLessons) == 0 {
		lines = append(lines, fmt.Sprintf("- %s: -", loc.Label("Lessons", "经验")))
	} else {
		lines = append(lines, "- "+loc.Label("Lessons", "经验")+":")
		for _, lesson := range remainingLessons {
			lines = append(lines, "  - "+lesson)
		}
	}

	lines = append(lines, "", "## "+loc.Label("Known World", "已知世界"), "")
	if len(d.seenContentCounts) == 0 {
		lines = append(lines, "- -")
	} else {
		for _, category := range []string{
			seenCategoryCards,
			seenCategoryRelics,
			seenCategoryPotions,
			seenCategoryMonsters,
			seenCategoryEvents,
			seenCategoryCharacters,
		} {
			lines = append(lines, fmt.Sprintf("- %s: `%d`", seenCategoryHeading(loc, category), d.seenContentCounts[category]))
		}
	}
	if len(d.recentDiscoveries) == 0 {
		lines = append(lines, fmt.Sprintf("- %s: -", loc.Label("Recent discoveries", "最近发现")))
	} else {
		lines = append(lines, "- "+loc.Label("Recent discoveries", "最近发现")+":")
		for _, discovery := range d.recentDiscoveries {
			lines = append(lines, "  - "+discovery)
		}
	}

	lines = append(lines,
		fmt.Sprintf("- %s: `%t`", loc.Label("RL ready", "RL 就绪"), d.guideRLReady),
		fmt.Sprintf("- %s: %s", loc.Label("RL status", "RL 状态"), valueOrDash(d.guideRLStatus)),
		fmt.Sprintf("- %s: `%d / %d`", loc.Label("Complete runs", "完整 run"), d.guideRLCompleteRuns, d.guideRLRequiredRuns),
		fmt.Sprintf("- %s: `%d / %d`", loc.Label("Floor >= 15 runs", "15层以上 run"), d.guideRLFloor15Runs, d.guideRLRequiredFloor),
		fmt.Sprintf("- %s: `%d / %d`", loc.Label("Provider-backed runs", "Provider-backed runs"), d.guideRLProviderBackedRuns, d.guideRLRequiredProviderBackedRuns),
		fmt.Sprintf("- %s: `%d / %d`", loc.Label("Recent clean runs", "Recent clean runs"), d.guideRLRecentCleanRuns, d.guideRLRequiredRecentCleanRuns),
		fmt.Sprintf("- %s: `%t`", loc.Label("Stable runtime", "运行稳定"), d.guideRLStable),
		fmt.Sprintf("- %s: `%t`", loc.Label("Knowledge assets", "知识资产就绪"), d.guideRLKnowledgeOK),
	)
	lines = append(lines,
		fmt.Sprintf("- %s: `%d`", loc.Label("Clean complete runs", "Clean complete runs"), d.guideRunQualityCleanRuns),
		fmt.Sprintf("- %s: `%d`", loc.Label("Recent provider-backed runs", "Recent provider-backed runs"), d.guideRunQualityRecentProviderBackedRuns),
		fmt.Sprintf("- %s: `%d`", loc.Label("Recent fallback runs", "Recent fallback runs"), d.guideRunQualityRecentFallbackRuns),
		fmt.Sprintf("- %s: `%d`", loc.Label("Recent provider-retry runs", "Recent provider-retry runs"), d.guideRunQualityRecentProviderRetryRuns),
		fmt.Sprintf("- %s: `%d`", loc.Label("Recent tool-error runs", "Recent tool-error runs"), d.guideRunQualityRecentToolErrorRuns),
		fmt.Sprintf("- %s: `%d`", loc.Label("Recent median floor", "Recent median floor"), d.guideRunQualityRecentMedianFloor),
		fmt.Sprintf("- %s: `%d`", loc.Label("Recent best floor", "Recent best floor"), d.guideRunQualityRecentBestFloor),
		fmt.Sprintf("- %s: `%d / %d`", loc.Label("Recent floor >= 7 runs", "Recent floor >= 7 runs"), d.guideRunQualityRecentFloor7PlusRuns, d.guideRecentWindow),
		fmt.Sprintf("- %s: `%d / %d`", loc.Label("Recent Act 2 entry runs", "Recent Act 2 entry runs"), d.guideRunQualityRecentAct2EntryRuns, d.guideRecentWindow),
		fmt.Sprintf("- %s: `%d`", loc.Label("Recent died-with-gold runs", "Recent died-with-gold runs"), d.guideRunQualityRecentDiedWithGoldRuns),
		fmt.Sprintf("- %s: `%d`", loc.Label("Recent average death gold", "Recent average death gold"), d.guideRunQualityRecentAverageDeathGold),
	)

	lines = append(lines, "", "## "+loc.Label("Tactical Hints", "战术提示"), "")
	if len(d.tacticalHints) == 0 {
		lines = append(lines, "- -")
	} else {
		for _, hint := range d.tacticalHints {
			lines = append(lines, "- "+hint)
		}
	}
	lines = append(lines, "", "## "+loc.Label("Depth Odds", "深层概率"), "")
	if len(d.depthOdds) == 0 {
		lines = append(lines, "- -")
	} else {
		for _, line := range d.depthOdds {
			lines = append(lines, line)
		}
	}
	if d.combatPlanSummary != "" || d.combatPlanGoal != "" || d.combatPlanTarget != "" || len(d.combatPlanReasons) > 0 {
		lines = append(lines, "- "+loc.Label("Combat planner", "战斗规划器")+":")
		lines = append(lines, "  - "+loc.Label("Mode", "模式")+": `"+valueOrDash(d.combatPlannerMode)+"`")
		lines = append(lines, "  - "+loc.Label("Summary", "摘要")+": "+valueOrDash(d.combatPlanSummary))
		if d.combatPlanGoal != "" {
			lines = append(lines, "  - "+loc.Label("Primary goal", "主要目标")+": "+d.combatPlanGoal)
		}
		if d.combatPlanTarget != "" {
			lines = append(lines, "  - "+loc.Label("Target bias", "目标倾向")+": "+d.combatPlanTarget)
		}
		for _, reason := range d.combatPlanReasons {
			lines = append(lines, "    - "+reason)
		}
		for _, candidate := range d.combatPlanCandidates {
			lines = append(lines, "    - "+candidate)
		}
	}

	lines = append(lines, "", "## "+loc.Label("Room Detail", "房间细节"), "")
	lines = append(lines, StateDetailLinesFor(d.state, 6, loc.Language())...)
	lines = append(lines, "", "## Model Telemetry", "")
	lines = append(lines,
		fmt.Sprintf("- %s: `%s`", "Model latency", formatDashboardDurationMs(d.lastCycleDurationMs)),
		fmt.Sprintf("- %s: `%d`", "Input tokens", d.lastInputTokens),
		fmt.Sprintf("- %s: `%d`", "Output tokens", d.lastOutputTokens),
		fmt.Sprintf("- %s: `%s`", "Prompt size", formatDashboardBytes(d.lastPromptSizeBytes)),
	)

	lines = append(lines, "", "## "+loc.Label("Streamer Booth", "主播机位"), "")
	lines = append(lines,
		fmt.Sprintf("- %s: %s", loc.Label("Mood", "情绪"), valueOrDash(d.lastStreamerMood)),
		fmt.Sprintf("- %s: %s", loc.Label("Commentary", "解说"), valueOrDash(d.lastStreamerCommentary)),
		fmt.Sprintf("- %s: %s", loc.Label("Game insight", "游戏见解"), valueOrDash(d.lastStreamerInsight)),
		fmt.Sprintf("- %s: %s", loc.Label("Life reflection", "人生感慨"), valueOrDash(d.lastStreamerReflection)),
		fmt.Sprintf("- %s: %s", loc.Label("TTS text", "播报文本"), valueOrDash(d.lastStreamerTTS)),
	)

	lines = append(lines, "", "## "+loc.Label("Latest Signals", "最近信号"), "")
	lines = append(lines,
		fmt.Sprintf("- %s: %s", loc.Label("Status", "状态"), valueOrDash(d.lastStatus)),
		fmt.Sprintf("- %s: %s", loc.Label("Decision", "决策"), valueOrDash(d.lastDecision)),
		fmt.Sprintf("- %s: %s", loc.Label("Action", "动作"), valueOrDash(d.lastAction)),
		fmt.Sprintf("- %s: %s", loc.Label("Tool", "工具"), valueOrDash(d.lastTool)),
		fmt.Sprintf("- %s: %s", loc.Label("Tool error", "工具错误"), valueOrDash(d.lastToolError)),
		fmt.Sprintf("- %s: %s", loc.Label("Recovery kind", "恢复类型"), valueOrDash(d.lastRecoveryKind)),
		fmt.Sprintf("- %s: %s", loc.Label("Drift expected", "预期状态"), valueOrDash(compactText(d.lastDriftExpected, 240))),
		fmt.Sprintf("- %s: %s", loc.Label("Drift live", "实际状态"), valueOrDash(compactText(d.lastDriftLive, 240))),
		fmt.Sprintf("- %s: %s", loc.Label("Drift kind", "漂移类型"), valueOrDash(d.lastDriftKind)),
		fmt.Sprintf("- %s: %s", loc.Label("Attempt lifecycle", "尝试生命周期"), valueOrDash(d.lastAttemptLifecycle)),
		fmt.Sprintf("- %s: %s", loc.Label("Provider recovery", "提供方恢复"), valueOrDash(d.lastProviderRecovery)),
		fmt.Sprintf("- %s: %s", loc.Label("Compact", "压缩"), valueOrDash(d.lastCompact)),
		fmt.Sprintf("- %s: %s", loc.Label("Reflection", "反思"), valueOrDash(d.lastReflection)),
		fmt.Sprintf("- %s: %s", loc.Label("Reflection story", "反思故事"), valueOrDash(compactText(d.lastReflectionStory, 240))),
		fmt.Sprintf("- %s: %s", loc.Label("Stop", "停止"), valueOrDash(d.lastStop)),
	)
	if d.gameFastModeChanged {
		lines = append(lines, fmt.Sprintf("- %s: %s -> %s", loc.Label("Game fast mode change", "游戏加速模式变更"), valueOrDash(d.gameFastModePrevious), valueOrDash(d.gameFastMode)))
	}

	lines = append(lines, "", "## "+loc.Label("Prompt Preview", "提示词预览"), "", fencedText(compactText(d.lastPrompt, 1600)))
	lines = append(lines, "", "## "+loc.Label("Assistant Preview", "模型回复预览"), "", fencedText(compactText(d.lastAssistant, 800)))
	lines = append(lines, "", "## "+loc.Label("Recent Logs", "最近日志"), "")
	if len(d.logs) == 0 {
		lines = append(lines, "- -")
	} else {
		for _, line := range d.logs {
			lines = append(lines, "- "+line)
		}
	}

	lines = append(lines, "", "## "+loc.Label("Guidebook Recovery Trends", "Guidebook Recovery Trends"), "")
	lines = append(lines, fmt.Sprintf("- %s: `%d`", loc.Label("Recent hotspot window", "Recent hotspot window"), d.guideRecentWindow))
	if len(d.guideRecentHotspots) == 0 && len(d.guideWeightedTrends) == 0 {
		lines = append(lines, "- -")
	} else {
		lines = append(lines, "- "+loc.Label("Recent recovery hotspots", "Recent recovery hotspots")+":")
		for _, hotspot := range d.guideRecentHotspots {
			lines = append(lines, "  - "+fmt.Sprintf("%s: `%d`", valueOrDash(recoveryHotspotLabel(hotspot)), hotspot.Count))
		}
		lines = append(lines, "- "+loc.Label("Recency-weighted trends", "Recency-weighted trends")+":")
		for _, hotspot := range d.guideWeightedTrends {
			lines = append(lines, "  - "+fmt.Sprintf("%s: `%.2f`", valueOrDash(recoveryHotspotLabel(hotspot)), hotspot.Score))
		}
	}

	return strings.Join(lines, "\n")
}

func (d *DebugDashboard) clearCombatPlan() {
	d.combatPlannerMode = ""
	d.combatPlanSummary = ""
	d.combatPlanGoal = ""
	d.combatPlanTarget = ""
	d.combatPlanReasons = nil
	d.combatPlanCandidates = nil
}

func (d *DebugDashboard) appendLog(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}

	d.logs = append(d.logs, line)
	if len(d.logs) > 24 {
		d.logs = d.logs[len(d.logs)-24:]
	}
}

func valueOrDash(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}

	return value
}

func compactText(value string, max int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	runes := []rune(value)
	if max <= 0 || len(runes) <= max {
		return value
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}

func formatDashboardDurationMs(ms int64) string {
	if ms <= 0 {
		return "-"
	}
	return fmt.Sprintf("%d ms", ms)
}

func formatDashboardBytes(size int) string {
	if size <= 0 {
		return "-"
	}
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	return fmt.Sprintf("%.1f KB", float64(size)/1024.0)
}

func fencedText(value string) string {
	return "```text\n" + value + "\n```"
}

func dashboardProviderEndpoint(cfg config.Config) string {
	if cfg.UsesClaudeCLI() {
		return cfg.ClaudeCLIPath
	}

	return cfg.APIBaseURL
}

func stringifySlice(values []interface{}) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		text, ok := value.(string)
		if !ok {
			continue
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		result = append(result, text)
	}
	return result
}

func floatData(value interface{}) float64 {
	switch typed := value.(type) {
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

func recoveryHotspotsFromData(value interface{}) []RecoveryHotspot {
	switch typed := value.(type) {
	case []RecoveryHotspot:
		return append([]RecoveryHotspot(nil), typed...)
	case []interface{}:
		hotspots := make([]RecoveryHotspot, 0, len(typed))
		for _, raw := range typed {
			item, ok := raw.(map[string]any)
			if !ok {
				continue
			}
			hotspots = append(hotspots, RecoveryHotspot{
				RecoveryKind: stringData(item["recovery_kind"]),
				DriftKind:    stringData(item["drift_kind"]),
				Count:        int(floatData(item["count"])),
				Score:        floatData(item["score"]),
			})
		}
		return hotspots
	default:
		return nil
	}
}
