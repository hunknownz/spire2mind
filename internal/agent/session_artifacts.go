package agentruntime

import (
	"strings"
	"time"

	"spire2mind/internal/game"
)

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
