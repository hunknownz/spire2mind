package agentruntime

import (
	"time"

	"spire2mind/internal/game"
)

type SessionResumeState struct {
	UpdatedAt           time.Time               `json:"updated_at"`
	AttemptCount        int                     `json:"attempt_count"`
	SeenRunIDs          map[string]int          `json:"seen_run_ids,omitempty"`
	CarryForwardPlan    string                  `json:"carry_forward_plan,omitempty"`
	CarryForwardLessons []string                `json:"carry_forward_lessons,omitempty"`
	CarryForwardBuckets ReflectionLessonBuckets `json:"carry_forward_buckets,omitempty"`
	LastRunID           string                  `json:"last_run_id,omitempty"`
	LastScreen          string                  `json:"last_screen,omitempty"`
}

func BuildSessionResumeState(attemptCount int, seenRunIDs map[string]int, todo *TodoManager, state *game.StateSnapshot) *SessionResumeState {
	resume := &SessionResumeState{
		UpdatedAt:    time.Now(),
		AttemptCount: attemptCount,
		SeenRunIDs:   cloneSeenRunIDs(seenRunIDs),
	}

	if todo != nil {
		snapshot := todo.Snapshot()
		resume.CarryForwardPlan = snapshot.CarryForwardPlan
		resume.CarryForwardLessons = append([]string(nil), snapshot.CarryForwardLessons...)
		resume.CarryForwardBuckets = snapshot.CarryForwardBuckets.Clone()
	}
	if state != nil {
		resume.LastRunID = state.RunID
		resume.LastScreen = state.Screen
	}

	return resume
}

func cloneSeenRunIDs(input map[string]int) map[string]int {
	if len(input) == 0 {
		return nil
	}

	cloned := make(map[string]int, len(input))
	for key, value := range input {
		cloned[key] = value
	}

	return cloned
}
