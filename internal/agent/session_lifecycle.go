package agentruntime

import "spire2mind/internal/game"

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
