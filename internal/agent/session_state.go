package agentruntime

import (
	"time"

	"spire2mind/internal/game"
)

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
		loc := s.localizer()
		data["current_goal"] = localizeTodoText(snapshot.CurrentGoal, loc)
		data["room_goal"] = localizeTodoText(snapshot.RoomGoal, loc)
		data["next_intent"] = localizeTodoText(snapshot.NextIntent, loc)
		data["last_failure"] = localizeTodoText(snapshot.LastFailure, loc)
		data["carry_forward_plan"] = localizeTodoText(snapshot.CarryForwardPlan, loc)
		data["carry_forward_lessons"] = localizeTodoSlice(snapshot.CarryForwardLessons, loc)
		data["carry_forward_buckets"] = snapshot.CarryForwardBuckets.ToDataMap()
	}
	if hints := BuildTacticalHintsForLanguage(state, s.cfg.Language); len(hints) > 0 {
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
		Message: s.say("state updated", "state updated"),
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
	s.appendStreamerHistory(event)

	select {
	case s.events <- event:
	default:
	}
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
