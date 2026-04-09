package agentruntime

import (
	"strings"
	"testing"
)

func TestCompactMemoryDeduplicatesLessonsFromReflectionAndResume(t *testing.T) {
	compact := NewCompactMemory()
	reflection := &AttemptReflection{
		Attempt: 1,
		Story:   "A rough fight ended the run.",
		Lessons: []string{
			"Play safer at low health.",
			"Spend gold earlier.",
		},
	}

	compact.RecordReflection(reflection)
	compact.ApplyResume(&SessionResumeState{
		CarryForwardLessons: []string{
			"Play safer at low health.",
			"Spend gold earlier.",
		},
	})

	if len(compact.lessons) != 2 {
		t.Fatalf("expected 2 deduplicated lessons, got %#v", compact.lessons)
	}
	if compact.lessons[0] != "Play safer at low health." || compact.lessons[1] != "Spend gold earlier." {
		t.Fatalf("expected stable lesson order, got %#v", compact.lessons)
	}
}

func TestSummarizeTodoSnapshotBuildsCompactWorkingMemoryLine(t *testing.T) {
	todo := NewTodoManager()
	todo.currentGoal = "Win the current combat."
	todo.roomGoal = "Play the best legal turn."
	todo.nextIntent = "Block first, then push lethal."

	agentSummary := summarizeTodoSnapshot(todo.Snapshot())
	if !strings.Contains(agentSummary, "goal=Win the current combat.") {
		t.Fatalf("expected compact summary to include goal, got %q", agentSummary)
	}
	if !strings.Contains(agentSummary, "room=Play the best legal turn.") {
		t.Fatalf("expected compact summary to include room goal, got %q", agentSummary)
	}
	if !strings.Contains(agentSummary, "intent=Block first, then push lethal.") {
		t.Fatalf("expected compact summary to include next intent, got %q", agentSummary)
	}
}
