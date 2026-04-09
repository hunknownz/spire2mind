package agentruntime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"spire2mind/internal/game"
	"spire2mind/internal/i18n"
)

func TestLoadLatestSessionResumePrefersNewestRun(t *testing.T) {
	root := t.TempDir()
	excluded := filepath.Join(root, "current-run")
	if err := os.MkdirAll(excluded, 0o755); err != nil {
		t.Fatalf("mkdir excluded: %v", err)
	}

	older := filepath.Join(root, "older-run")
	newer := filepath.Join(root, "newer-run")
	for _, dir := range []string{older, newer} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}

	writeResume := func(dir string, resume SessionResumeState) {
		t.Helper()
		bytes, err := json.MarshalIndent(resume, "", "  ")
		if err != nil {
			t.Fatalf("marshal resume: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "session-latest.json"), bytes, 0o644); err != nil {
			t.Fatalf("write resume: %v", err)
		}
	}

	writeResume(older, SessionResumeState{
		UpdatedAt:    time.Now().Add(-2 * time.Hour),
		AttemptCount: 2,
		LastRunID:    "OLD",
	})
	writeResume(newer, SessionResumeState{
		UpdatedAt:    time.Now().Add(-1 * time.Hour),
		AttemptCount: 4,
		LastRunID:    "NEW",
	})

	resume, err := LoadLatestSessionResume(root, excluded)
	if err != nil {
		t.Fatalf("load latest resume: %v", err)
	}
	if resume == nil {
		t.Fatal("expected a resume state")
	}
	if resume.LastRunID != "NEW" {
		t.Fatalf("expected newest resume, got %#v", resume)
	}
}

func TestApplyResumeStateRestoresCarryForwardMemoryWithoutConsumingAttempts(t *testing.T) {
	session := &Session{
		todo:            NewTodoManager(),
		compact:         NewCompactMemory(),
		seenRunIDs:      map[string]int{"OLD": 1},
		reflectedRunIDs: make(map[string]bool),
	}

	resume := &SessionResumeState{
		AttemptCount: 3,
		SeenRunIDs: map[string]int{
			"OLD":   2,
			"RUN-3": 3,
		},
		CarryForwardPlan:    "Stabilize first, then spend gold earlier.",
		CarryForwardLessons: []string{"Re-read live state before repeating indexed actions."},
		CarryForwardBuckets: ReflectionLessonBuckets{
			Runtime: []string{"Re-read live state before repeating indexed actions."},
			Pathing: []string{"Take safer routes when HP is low."},
		},
		LastRunID:  "RUN-3",
		LastScreen: "MAP",
	}

	session.applyResumeState(resume)

	if session.attemptCount != 0 {
		t.Fatalf("expected session attempt count to remain fresh, got %d", session.attemptCount)
	}
	if session.seenRunIDs["OLD"] != 1 {
		t.Fatalf("expected current-session run IDs to stay untouched, got %#v", session.seenRunIDs)
	}
	if _, ok := session.seenRunIDs["RUN-3"]; ok {
		t.Fatalf("did not expect resume state to consume a current-session run slot: %#v", session.seenRunIDs)
	}

	todoBlock := session.todo.PromptBlock()
	if !strings.Contains(todoBlock, resume.CarryForwardPlan) {
		t.Fatalf("expected carry-forward plan in todo block, got %q", todoBlock)
	}
	if !strings.Contains(todoBlock, resume.CarryForwardLessons[0]) {
		t.Fatalf("expected carry-forward lesson in todo block, got %q", todoBlock)
	}
	if !strings.Contains(todoBlock, "Take safer routes when HP is low.") {
		t.Fatalf("expected categorized carry-forward lesson in todo block, got %q", todoBlock)
	}

	compactBlock := session.compact.PromptBlock()
	if !strings.Contains(compactBlock, resume.CarryForwardLessons[0]) {
		t.Fatalf("expected carry-forward lesson in compact block, got %q", compactBlock)
	}
	if !strings.Contains(compactBlock, "resumed from run=RUN-3 screen=MAP") {
		t.Fatalf("expected compact block to mention resume location, got %q", compactBlock)
	}
}

func TestLoadCarryForwardStateKeepsCurrentSessionAttemptsFresh(t *testing.T) {
	root := t.TempDir()
	priorStore, err := NewRunStore(root, "prior")
	if err != nil {
		t.Fatalf("new prior store: %v", err)
	}
	defer func() { _ = priorStore.Close() }()
	if err := priorStore.RecordAttemptReflection(&AttemptReflection{
		Time:    time.Now().Add(-time.Hour),
		Attempt: 7,
		RunID:   "OLD-RUN",
		Outcome: "defeat",
		Screen:  "GAME_OVER",
		Story:   "Old run story.",
		Lessons: []string{"Spend gold earlier."},
	}, i18n.LanguageEnglish); err != nil {
		t.Fatalf("record prior reflection: %v", err)
	}

	currentStore, err := NewRunStore(root, "current")
	if err != nil {
		t.Fatalf("new current store: %v", err)
	}
	defer func() { _ = currentStore.Close() }()

	session := &Session{
		todo:            NewTodoManager(),
		compact:         NewCompactMemory(),
		store:           currentStore,
		seenRunIDs:      make(map[string]int),
		reflectedRunIDs: make(map[string]bool),
	}

	if err := session.loadCarryForwardState(root); err != nil {
		t.Fatalf("load carry-forward state: %v", err)
	}

	if session.attemptCount != 0 {
		t.Fatalf("expected no attempts to be consumed by historical reflections, got %d", session.attemptCount)
	}
	if session.storyAttemptBase != 1 {
		t.Fatalf("expected story attempt base to track loaded reflections, got %d", session.storyAttemptBase)
	}
	if len(session.reflections) != 1 {
		t.Fatalf("expected one loaded reflection, got %d", len(session.reflections))
	}
	if !session.reflectedRunIDs["OLD-RUN"] {
		t.Fatalf("expected reflected run ID to be recorded")
	}

	state := &game.StateSnapshot{
		RunID:            "NEW-RUN",
		Screen:           "MAP",
		AvailableActions: []string{"choose_map_node"},
	}
	if attempt := session.registerAttempt(state); attempt != 1 {
		t.Fatalf("expected first current-session attempt to be 1, got %d", attempt)
	}
	if storyAttempt := session.currentStoryAttemptForState(state); storyAttempt != 2 {
		t.Fatalf("expected current story attempt to continue after history, got %d", storyAttempt)
	}
}

func TestRecordStateWritesSessionResumeSnapshot(t *testing.T) {
	store, err := NewRunStore(t.TempDir(), "session-test")
	if err != nil {
		t.Fatalf("new run store: %v", err)
	}
	defer func() { _ = store.Close() }()

	session := &Session{
		store:       store,
		todo:        NewTodoManager(),
		compact:     NewCompactMemory(),
		seenRunIDs:  make(map[string]int),
		events:      make(chan SessionEvent, 4),
		maxAttempts: 3,
	}
	session.todo.ApplyReflection(&AttemptReflection{
		NextPlan: "Take safer paths at low health.",
		LessonBuckets: ReflectionLessonBuckets{
			Pathing: []string{"Take safer paths at low health."},
		},
		Lessons: []string{"Take safer paths at low health."},
	})

	state := &game.StateSnapshot{
		RunID:            "RUN-42",
		Screen:           "MAP",
		AvailableActions: []string{"choose_map_node"},
	}

	session.recordState(state)

	bytes, err := os.ReadFile(store.sessionPath)
	if err != nil {
		t.Fatalf("read session resume: %v", err)
	}

	var resume SessionResumeState
	if err := json.Unmarshal(bytes, &resume); err != nil {
		t.Fatalf("unmarshal session resume: %v", err)
	}
	if resume.AttemptCount != 1 {
		t.Fatalf("expected attempt count 1, got %d", resume.AttemptCount)
	}
	if resume.SeenRunIDs["RUN-42"] != 1 {
		t.Fatalf("expected RUN-42 to map to attempt 1, got %#v", resume.SeenRunIDs)
	}
	if resume.LastRunID != "RUN-42" || resume.LastScreen != "MAP" {
		t.Fatalf("unexpected resume snapshot: %#v", resume)
	}
	if resume.CarryForwardBuckets.IsEmpty() {
		t.Fatalf("expected structured carry-forward buckets in resume snapshot, got %#v", resume)
	}
}
