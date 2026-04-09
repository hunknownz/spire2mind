package agentruntime

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"spire2mind/internal/i18n"
)

func TestLoadRecentAttemptReflections(t *testing.T) {
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

	writeReflection := func(dir string, reflection AttemptReflection) {
		t.Helper()
		path := filepath.Join(dir, "attempt-reflections.jsonl")
		file, err := os.Create(path)
		if err != nil {
			t.Fatalf("create %s: %v", path, err)
		}
		defer file.Close()

		line, err := json.Marshal(reflection)
		if err != nil {
			t.Fatalf("marshal reflection: %v", err)
		}
		if _, err := file.Write(append(line, '\n')); err != nil {
			t.Fatalf("write reflection: %v", err)
		}
	}

	writeReflection(older, AttemptReflection{
		Attempt: 1,
		RunID:   "OLD",
		Time:    time.Now().Add(-2 * time.Hour),
		Story:   "older",
	})
	writeReflection(newer, AttemptReflection{
		Attempt: 2,
		RunID:   "NEW",
		Time:    time.Now().Add(-1 * time.Hour),
		Story:   "newer",
	})

	reflections, err := LoadRecentAttemptReflections(root, excluded, 2)
	if err != nil {
		t.Fatalf("load recent reflections: %v", err)
	}
	if len(reflections) != 2 {
		t.Fatalf("expected 2 reflections, got %d", len(reflections))
	}
	if reflections[0].RunID != "OLD" || reflections[1].RunID != "NEW" {
		t.Fatalf("unexpected reflection order: %#v", reflections)
	}
}

func TestRunStoreWritesMarkdownArtifactsAsUTF8WithBOM(t *testing.T) {
	store, err := NewRunStore(t.TempDir(), "session")
	if err != nil {
		t.Fatalf("new run store: %v", err)
	}
	defer func() { _ = store.Close() }()

	if err := store.WriteDashboard("# 仪表盘\n\n- 当前屏幕: `COMBAT`"); err != nil {
		t.Fatalf("write dashboard: %v", err)
	}
	if err := store.WriteRunStory("# 故事\n\n这是一段中文回放。"); err != nil {
		t.Fatalf("write run story: %v", err)
	}

	for _, path := range []string{store.dashboardPath, store.storyPath} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if !bytes.HasPrefix(data, utf8BOM) {
			t.Fatalf("expected UTF-8 BOM prefix for %s", path)
		}
	}
}

func TestRunStoreIndexesSessionAttemptRecoveryAndSeenContent(t *testing.T) {
	store, err := NewRunStore(t.TempDir(), "session-index")
	if err != nil {
		t.Fatalf("new run store: %v", err)
	}
	defer func() { _ = store.Close() }()

	event := SessionEvent{
		Time:    time.Now(),
		Kind:    SessionEventStatus,
		Cycle:   7,
		Attempt: 2,
		Message: "soft seam recovery for choose_reward_card; replanning on the latest state",
		Screen:  "REWARD",
		RunID:   "RUN-INDEX",
		Data: map[string]interface{}{
			"mode":               "model-claude-cli",
			"provider":           "claude-cli",
			"provider_state":     "healthy",
			"model":              "claude-sonnet-4-6",
			"max_attempts":       3,
			"attempt_lifecycle":  "running",
			"recovery_kind":      "soft_replan",
			"drift_kind":         "reward_transition",
			"carry_forward_plan": "Stabilize rewards before spending retries.",
			"current_goal":       "Continue the run",
		},
	}
	if err := store.AppendEvent(event); err != nil {
		t.Fatalf("append event: %v", err)
	}

	registry := &SeenContentRegistry{
		UpdatedAt: time.Now(),
		Cards: []SeenContentEntry{{
			Category:    seenCategoryCards,
			ID:          "STRIKE_IRONCLAD",
			Name:        "Strike",
			FirstSeenAt: time.Now().Add(-time.Minute),
			LastSeenAt:  time.Now(),
			FirstRunID:  "RUN-INDEX",
			LastRunID:   "RUN-INDEX",
			FirstScreen: "COMBAT",
			LastScreen:  "REWARD",
			SeenCount:   2,
		}},
	}
	if err := store.WriteSeenContent(registry); err != nil {
		t.Fatalf("write seen content: %v", err)
	}

	db, err := sql.Open("sqlite", store.IndexPath())
	if err != nil {
		t.Fatalf("open run index: %v", err)
	}
	defer db.Close()

	var (
		mode, provider, providerState string
		lastAttempt                   int
		currentGoal                   string
	)
	if err := db.QueryRow(`SELECT mode, provider, provider_state, last_attempt, current_goal FROM session_meta WHERE id = 1`).Scan(
		&mode,
		&provider,
		&providerState,
		&lastAttempt,
		&currentGoal,
	); err != nil {
		t.Fatalf("query session meta: %v", err)
	}
	if mode != "model-claude-cli" || provider != "claude-cli" || providerState != "healthy" {
		t.Fatalf("unexpected session meta: mode=%q provider=%q provider_state=%q", mode, provider, providerState)
	}
	if lastAttempt != 2 || currentGoal != "Continue the run" {
		t.Fatalf("unexpected indexed session fields: lastAttempt=%d currentGoal=%q", lastAttempt, currentGoal)
	}

	var lifecycle, status string
	if err := db.QueryRow(`SELECT lifecycle, status FROM attempts WHERE session_id = ? AND attempt = ?`, "session-index", 2).Scan(&lifecycle, &status); err != nil {
		t.Fatalf("query indexed attempt: %v", err)
	}
	if lifecycle != "running" || status != "running" {
		t.Fatalf("unexpected attempt row: lifecycle=%q status=%q", lifecycle, status)
	}

	var recoveryKind, driftKind string
	if err := db.QueryRow(`SELECT kind, drift_kind FROM recovery_events WHERE session_id = ? ORDER BY id DESC LIMIT 1`, "session-index").Scan(&recoveryKind, &driftKind); err != nil {
		t.Fatalf("query recovery event: %v", err)
	}
	if recoveryKind != "soft_replan" || driftKind != "reward_transition" {
		t.Fatalf("unexpected recovery event: kind=%q drift=%q", recoveryKind, driftKind)
	}

	var cardCount int
	if err := db.QueryRow(`SELECT COUNT(*) FROM seen_content WHERE session_id = ? AND category = ?`, "session-index", seenCategoryCards).Scan(&cardCount); err != nil {
		t.Fatalf("query seen content: %v", err)
	}
	if cardCount != 1 {
		t.Fatalf("expected one indexed card, got %d", cardCount)
	}
}

func TestRunStoreIndexesCycleSummaryAndReflection(t *testing.T) {
	store, err := NewRunStore(t.TempDir(), "session-cycle")
	if err != nil {
		t.Fatalf("new run store: %v", err)
	}
	defer func() { _ = store.Close() }()

	summary := &CycleSummary{
		LastAssistantText: "Pick a safe reward and keep the deck lean.",
		LastAction:        "choose_reward_card",
		Decision: &ActionDecision{
			Action:      "choose_reward_card",
			OptionIndex: intPtr(1),
			Reason:      "It adds frontloaded damage without bloating the curve.",
		},
		ActCalls:         1,
		MadeProgress:     true,
		HadInvalidAction: false,
		Turns:            3,
		Cost:             0.0123,
		Metadata: map[string]interface{}{
			"provider": "claude-cli",
		},
	}
	if err := store.RecordCycleSummary(12, summary); err != nil {
		t.Fatalf("record cycle summary: %v", err)
	}

	reflection := &AttemptReflection{
		Time:     time.Now(),
		Attempt:  4,
		RunID:    "RUN-4",
		Outcome:  "defeat",
		Screen:   "GAME_OVER",
		Headline: "Died after a low-HP spiral.",
		Story:    "The deck stalled and the agent entered a low-HP spiral before the boss.",
		NextPlan: "Prioritize safer pathing and block density earlier.",
		Lessons:  []string{"Respect low-HP route pressure earlier."},
		LessonBuckets: ReflectionLessonBuckets{
			Pathing: []string{"Respect low-HP route pressure earlier."},
		},
	}
	if err := store.RecordAttemptReflection(reflection, i18n.LanguageEnglish); err != nil {
		t.Fatalf("record reflection: %v", err)
	}

	db, err := sql.Open("sqlite", store.IndexPath())
	if err != nil {
		t.Fatalf("open run index: %v", err)
	}
	defer db.Close()

	var (
		action, reason, provider string
		actCalls                 int
		madeProgress             int
	)
	if err := db.QueryRow(`SELECT action, decision_reason, provider, act_calls, made_progress FROM cycle_summaries WHERE session_id = ? AND cycle = ?`, "session-cycle", 12).Scan(
		&action, &reason, &provider, &actCalls, &madeProgress,
	); err != nil {
		t.Fatalf("query cycle summary: %v", err)
	}
	if action != "choose_reward_card" || provider != "claude-cli" || actCalls != 1 || madeProgress != 1 {
		t.Fatalf("unexpected cycle summary row: action=%q provider=%q actCalls=%d madeProgress=%d", action, provider, actCalls, madeProgress)
	}
	if !bytes.Contains([]byte(reason), []byte("frontloaded damage")) {
		t.Fatalf("unexpected decision reason: %q", reason)
	}

	var (
		outcome, story, nextPlan string
	)
	if err := db.QueryRow(`SELECT outcome, story, next_plan FROM reflections WHERE session_id = ? AND attempt = ?`, "session-cycle", 4).Scan(
		&outcome, &story, &nextPlan,
	); err != nil {
		t.Fatalf("query reflection: %v", err)
	}
	if outcome != "defeat" || nextPlan == "" || story == "" {
		t.Fatalf("unexpected reflection row: outcome=%q story=%q nextPlan=%q", outcome, story, nextPlan)
	}
}

func intPtr(value int) *int {
	return &value
}
