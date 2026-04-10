package agentruntime

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"

	"spire2mind/internal/game"
)

type runIndex struct {
	path      string
	sessionID string
	db        *sql.DB
}

func openRunIndex(path string, sessionID string, root string) (*runIndex, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open run index: %w", err)
	}

	index := &runIndex{
		path:      path,
		sessionID: strings.TrimSpace(sessionID),
		db:        db,
	}

	if err := index.init(root); err != nil {
		_ = db.Close()
		return nil, err
	}

	return index, nil
}

func (i *runIndex) Close() error {
	if i == nil || i.db == nil {
		return nil
	}
	return i.db.Close()
}

func (i *runIndex) init(root string) error {
	stmts := []string{
		`PRAGMA journal_mode=WAL;`,
		`PRAGMA busy_timeout=5000;`,
		`CREATE TABLE IF NOT EXISTS session_meta (
			id INTEGER PRIMARY KEY CHECK (id = 1),
			session_id TEXT NOT NULL,
			root_path TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL,
			mode TEXT,
			provider TEXT,
			provider_state TEXT,
			model TEXT,
			backend TEXT,
			max_attempts INTEGER,
			game_fast_mode TEXT,
			game_prefs_path TEXT,
			status TEXT,
			last_cycle INTEGER,
			last_attempt INTEGER,
			last_turns INTEGER,
			last_cost REAL,
			last_screen TEXT,
			last_run_id TEXT,
			last_headline TEXT,
			current_goal TEXT,
			room_goal TEXT,
			next_intent TEXT,
			last_failure TEXT,
			carry_forward_plan TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS attempts (
			session_id TEXT NOT NULL,
			attempt INTEGER NOT NULL,
			run_id TEXT,
			started_at TEXT,
			ended_at TEXT,
			lifecycle TEXT,
			outcome TEXT,
			character_id TEXT,
			floor INTEGER,
			last_screen TEXT,
			headline TEXT,
			status TEXT,
			PRIMARY KEY (session_id, attempt)
		);`,
		`CREATE TABLE IF NOT EXISTS cycle_summaries (
			session_id TEXT NOT NULL,
			cycle INTEGER NOT NULL,
			attempt INTEGER,
			run_id TEXT,
			screen TEXT,
			action TEXT,
			decision_reason TEXT,
			assistant_text TEXT,
			act_calls INTEGER,
			made_progress INTEGER,
			had_invalid_action INTEGER,
			turns INTEGER,
			cost REAL,
			provider TEXT,
			metadata_json TEXT,
			recorded_at TEXT NOT NULL,
			PRIMARY KEY (session_id, cycle)
		);`,
		`CREATE TABLE IF NOT EXISTS recovery_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			session_id TEXT NOT NULL,
			cycle INTEGER,
			attempt INTEGER,
			recorded_at TEXT NOT NULL,
			screen TEXT,
			run_id TEXT,
			action TEXT,
			kind TEXT,
			drift_kind TEXT,
			provider_state TEXT,
			provider_recovery TEXT,
			message TEXT,
			expected_state_summary TEXT,
			live_state_summary TEXT
		);`,
		`CREATE TABLE IF NOT EXISTS reflections (
			session_id TEXT NOT NULL,
			attempt INTEGER NOT NULL,
			run_id TEXT,
			recorded_at TEXT NOT NULL,
			outcome TEXT,
			screen TEXT,
			floor INTEGER,
			character_id TEXT,
			headline TEXT,
			story TEXT,
			next_plan TEXT,
			lesson_buckets_json TEXT,
			lessons_json TEXT,
			PRIMARY KEY (session_id, attempt)
		);`,
		`CREATE TABLE IF NOT EXISTS seen_content (
			session_id TEXT NOT NULL,
			category TEXT NOT NULL,
			content_id TEXT NOT NULL,
			name TEXT,
			first_seen_at TEXT,
			last_seen_at TEXT,
			first_run_id TEXT,
			last_run_id TEXT,
			first_screen TEXT,
			last_screen TEXT,
			first_floor INTEGER,
			last_floor INTEGER,
			seen_count INTEGER NOT NULL DEFAULT 0,
			PRIMARY KEY (session_id, category, content_id)
		);`,
	}

	for _, stmt := range stmts {
		if _, err := i.db.Exec(stmt); err != nil {
			return fmt.Errorf("init run index: %w", err)
		}
	}

	now := timestampString(time.Now())
	_, err := i.db.Exec(
		`INSERT INTO session_meta (
			id, session_id, root_path, created_at, updated_at, status
		) VALUES (1, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			session_id = excluded.session_id,
			root_path = excluded.root_path,
			updated_at = excluded.updated_at`,
		i.sessionID,
		root,
		now,
		now,
		"starting",
	)
	if err != nil {
		return fmt.Errorf("seed session meta: %w", err)
	}

	return nil
}

func (i *runIndex) RecordEvent(event SessionEvent) error {
	if i == nil || i.db == nil {
		return nil
	}

	recordedAt := timestampString(event.Time)
	if err := i.updateSessionMeta(event, recordedAt); err != nil {
		return err
	}

	attempt := event.Attempt
	if attempt > 0 {
		lifecycle := stringData(event.Data["attempt_lifecycle"])
		if lifecycle != "" {
			if err := i.upsertAttemptLifecycle(attempt, event, lifecycle, recordedAt); err != nil {
				return err
			}
		}
	}

	if event.State != nil && attempt > 0 {
		if err := i.upsertAttemptState(attempt, event.State, recordedAt); err != nil {
			return err
		}
	}

	if err := i.insertRecoveryEvent(event, recordedAt); err != nil {
		return err
	}

	return nil
}

func (i *runIndex) RecordCycleSummary(cycle int, summary *CycleSummary) error {
	if i == nil || i.db == nil || summary == nil {
		return nil
	}

	metadataJSON, err := marshalJSONString(summary.Metadata)
	if err != nil {
		return err
	}

	var (
		action   string
		reason   string
		provider string
	)
	if summary.Decision != nil {
		action = strings.TrimSpace(summary.Decision.Action)
		reason = strings.TrimSpace(summary.Decision.Reason)
	}
	if value, ok := summary.Metadata["provider"].(string); ok {
		provider = strings.TrimSpace(value)
	}

	_, err = i.db.Exec(
		`INSERT INTO cycle_summaries (
			session_id, cycle, action, decision_reason, assistant_text, act_calls,
			made_progress, had_invalid_action, turns, cost, provider, metadata_json, recorded_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(session_id, cycle) DO UPDATE SET
			action = excluded.action,
			decision_reason = excluded.decision_reason,
			assistant_text = excluded.assistant_text,
			act_calls = excluded.act_calls,
			made_progress = excluded.made_progress,
			had_invalid_action = excluded.had_invalid_action,
			turns = excluded.turns,
			cost = excluded.cost,
			provider = excluded.provider,
			metadata_json = excluded.metadata_json,
			recorded_at = excluded.recorded_at`,
		i.sessionID,
		cycle,
		action,
		reason,
		strings.TrimSpace(summary.LastAssistantText),
		summary.ActCalls,
		boolToInt(summary.MadeProgress),
		boolToInt(summary.HadInvalidAction),
		summary.Turns,
		summary.Cost,
		provider,
		metadataJSON,
		timestampString(time.Now()),
	)
	if err != nil {
		return fmt.Errorf("record cycle summary: %w", err)
	}

	return nil
}

func (i *runIndex) RecordReflection(reflection *AttemptReflection) error {
	if i == nil || i.db == nil || reflection == nil {
		return nil
	}

	lessonsJSON, err := marshalJSONString(reflection.Lessons)
	if err != nil {
		return err
	}
	bucketsJSON, err := marshalJSONString(reflection.LessonBuckets)
	if err != nil {
		return err
	}

	_, err = i.db.Exec(
		`INSERT INTO reflections (
			session_id, attempt, run_id, recorded_at, outcome, screen, floor,
			character_id, headline, story, next_plan, lesson_buckets_json, lessons_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(session_id, attempt) DO UPDATE SET
			run_id = excluded.run_id,
			recorded_at = excluded.recorded_at,
			outcome = excluded.outcome,
			screen = excluded.screen,
			floor = excluded.floor,
			character_id = excluded.character_id,
			headline = excluded.headline,
			story = excluded.story,
			next_plan = excluded.next_plan,
			lesson_buckets_json = excluded.lesson_buckets_json,
			lessons_json = excluded.lessons_json`,
		i.sessionID,
		reflection.Attempt,
		strings.TrimSpace(reflection.RunID),
		timestampString(reflection.Time),
		strings.TrimSpace(reflection.Outcome),
		strings.TrimSpace(reflection.Screen),
		intOrNil(reflection.Floor),
		strings.TrimSpace(reflection.CharacterID),
		strings.TrimSpace(reflection.Headline),
		strings.TrimSpace(reflection.Story),
		strings.TrimSpace(reflection.NextPlan),
		bucketsJSON,
		lessonsJSON,
	)
	if err != nil {
		return fmt.Errorf("record reflection: %w", err)
	}

	return nil
}

func (i *runIndex) ReplaceSeenContent(registry *SeenContentRegistry) error {
	if i == nil || i.db == nil || registry == nil {
		return nil
	}

	tx, err := i.db.Begin()
	if err != nil {
		return fmt.Errorf("begin seen content tx: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.Exec(`DELETE FROM seen_content WHERE session_id = ?`, i.sessionID); err != nil {
		return fmt.Errorf("clear seen content: %w", err)
	}

	insert := func(category string, entries []SeenContentEntry) error {
		for _, entry := range entries {
			_, execErr := tx.Exec(
				`INSERT INTO seen_content (
					session_id, category, content_id, name, first_seen_at, last_seen_at,
					first_run_id, last_run_id, first_screen, last_screen, first_floor, last_floor, seen_count
				) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
				i.sessionID,
				category,
				strings.TrimSpace(entry.ID),
				strings.TrimSpace(entry.Name),
				timestampString(entry.FirstSeenAt),
				timestampString(entry.LastSeenAt),
				strings.TrimSpace(entry.FirstRunID),
				strings.TrimSpace(entry.LastRunID),
				strings.TrimSpace(entry.FirstScreen),
				strings.TrimSpace(entry.LastScreen),
				intOrNil(entry.FirstFloor),
				intOrNil(entry.LastFloor),
				entry.SeenCount,
			)
			if execErr != nil {
				return execErr
			}
		}
		return nil
	}

	if err = insert(seenCategoryCards, registry.Cards); err != nil {
		return fmt.Errorf("insert seen cards: %w", err)
	}
	if err = insert(seenCategoryRelics, registry.Relics); err != nil {
		return fmt.Errorf("insert seen relics: %w", err)
	}
	if err = insert(seenCategoryPotions, registry.Potions); err != nil {
		return fmt.Errorf("insert seen potions: %w", err)
	}
	if err = insert(seenCategoryMonsters, registry.Monsters); err != nil {
		return fmt.Errorf("insert seen monsters: %w", err)
	}
	if err = insert(seenCategoryEvents, registry.Events); err != nil {
		return fmt.Errorf("insert seen events: %w", err)
	}
	if err = insert(seenCategoryCharacters, registry.Characters); err != nil {
		return fmt.Errorf("insert seen characters: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commit seen content tx: %w", err)
	}

	return nil
}

func (i *runIndex) updateSessionMeta(event SessionEvent, recordedAt string) error {
	if i == nil || i.db == nil {
		return nil
	}

	mode := stringData(event.Data["mode"])
	provider := stringData(event.Data["provider"])
	providerState := stringData(event.Data["provider_state"])
	model := stringData(event.Data["model"])
	backend := firstNonEmptyString(
		stringData(event.Data["base_url"]),
		stringData(event.Data["claude_cli_path"]),
	)
	status := sessionStatusFromEvent(event)
	if status == "" {
		status = "running"
	}

	_, err := i.db.Exec(
		`UPDATE session_meta SET
			updated_at = ?,
			mode = COALESCE(NULLIF(?, ''), mode),
			provider = COALESCE(NULLIF(?, ''), provider),
			provider_state = COALESCE(NULLIF(?, ''), provider_state),
			model = COALESCE(NULLIF(?, ''), model),
			backend = COALESCE(NULLIF(?, ''), backend),
			max_attempts = COALESCE(?, max_attempts),
			game_fast_mode = COALESCE(NULLIF(?, ''), game_fast_mode),
			game_prefs_path = COALESCE(NULLIF(?, ''), game_prefs_path),
			status = COALESCE(NULLIF(?, ''), status),
			last_cycle = CASE WHEN ? > 0 THEN ? ELSE last_cycle END,
			last_attempt = CASE WHEN ? > 0 THEN ? ELSE last_attempt END,
			last_turns = CASE WHEN ? > 0 THEN ? ELSE last_turns END,
			last_cost = CASE WHEN ? > 0 THEN ? ELSE last_cost END,
			last_screen = COALESCE(NULLIF(?, ''), last_screen),
			last_run_id = COALESCE(NULLIF(?, ''), last_run_id),
			last_headline = COALESCE(NULLIF(?, ''), last_headline),
			current_goal = COALESCE(NULLIF(?, ''), current_goal),
			room_goal = COALESCE(NULLIF(?, ''), room_goal),
			next_intent = COALESCE(NULLIF(?, ''), next_intent),
			last_failure = COALESCE(?, last_failure),
			carry_forward_plan = COALESCE(NULLIF(?, ''), carry_forward_plan)
		WHERE id = 1`,
		recordedAt,
		mode,
		provider,
		providerState,
		model,
		backend,
		intData(event.Data["max_attempts"]),
		firstNonEmptyString(stringData(event.Data["game_fast_mode"]), stringData(event.Data["requested_fast_mode"])),
		stringData(event.Data["game_prefs_path"]),
		status,
		event.Cycle,
		event.Cycle,
		event.Attempt,
		event.Attempt,
		event.Turns,
		event.Turns,
		event.Cost,
		event.Cost,
		firstNonEmptyString(event.Screen, stateScreen(event.State)),
		firstNonEmptyString(event.RunID, stateRunID(event.State)),
		eventHeadline(event),
		stringData(event.Data["current_goal"]),
		stringData(event.Data["room_goal"]),
		stringData(event.Data["next_intent"]),
		stringPointerData(event.Data["last_failure"]),
		stringData(event.Data["carry_forward_plan"]),
	)
	if err != nil {
		return fmt.Errorf("update session meta: %w", err)
	}

	return nil
}

func (i *runIndex) upsertAttemptLifecycle(attempt int, event SessionEvent, lifecycle string, recordedAt string) error {
	status := "running"
	if lifecycle == "game_over" {
		status = "finished"
	}

	_, err := i.db.Exec(
		`INSERT INTO attempts (
			session_id, attempt, run_id, started_at, ended_at, lifecycle, status, last_screen
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(session_id, attempt) DO UPDATE SET
			run_id = COALESCE(NULLIF(excluded.run_id, ''), attempts.run_id),
			started_at = CASE
				WHEN attempts.started_at IS NULL AND excluded.started_at IS NOT NULL THEN excluded.started_at
				ELSE attempts.started_at
			END,
			ended_at = CASE
				WHEN excluded.ended_at IS NOT NULL THEN excluded.ended_at
				ELSE attempts.ended_at
			END,
			lifecycle = excluded.lifecycle,
			status = excluded.status,
			last_screen = COALESCE(NULLIF(excluded.last_screen, ''), attempts.last_screen)`,
		i.sessionID,
		attempt,
		firstNonEmptyString(event.RunID, stateRunID(event.State)),
		startedAtForLifecycle(lifecycle, recordedAt),
		endedAtForLifecycle(lifecycle, recordedAt),
		lifecycle,
		status,
		firstNonEmptyString(event.Screen, stateScreen(event.State)),
	)
	if err != nil {
		return fmt.Errorf("upsert attempt lifecycle: %w", err)
	}

	return nil
}

func (i *runIndex) upsertAttemptState(attempt int, state *game.StateSnapshot, recordedAt string) error {
	if state == nil {
		return nil
	}

	headline := agentViewHeadline(state)
	characterID := reflectionCharacterID(state)
	outcome := reflectionOutcome(state)
	floor := reflectionFloor(state)
	lifecycle := attemptLifecycleForState(state)
	status := "running"
	if lifecycle == "game_over" {
		status = "finished"
	}

	_, err := i.db.Exec(
		`INSERT INTO attempts (
			session_id, attempt, run_id, started_at, ended_at, lifecycle, outcome,
			character_id, floor, last_screen, headline, status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(session_id, attempt) DO UPDATE SET
			run_id = COALESCE(NULLIF(excluded.run_id, ''), attempts.run_id),
			started_at = CASE
				WHEN attempts.started_at IS NULL AND excluded.started_at IS NOT NULL THEN excluded.started_at
				ELSE attempts.started_at
			END,
			ended_at = CASE
				WHEN excluded.ended_at IS NOT NULL THEN excluded.ended_at
				ELSE attempts.ended_at
			END,
			lifecycle = excluded.lifecycle,
			outcome = COALESCE(NULLIF(excluded.outcome, ''), attempts.outcome),
			character_id = COALESCE(NULLIF(excluded.character_id, ''), attempts.character_id),
			floor = COALESCE(excluded.floor, attempts.floor),
			last_screen = COALESCE(NULLIF(excluded.last_screen, ''), attempts.last_screen),
			headline = COALESCE(NULLIF(excluded.headline, ''), attempts.headline),
			status = excluded.status`,
		i.sessionID,
		attempt,
		strings.TrimSpace(state.RunID),
		recordedAt,
		endedAtForLifecycle(lifecycle, recordedAt),
		lifecycle,
		outcome,
		characterID,
		intOrNil(floor),
		strings.TrimSpace(state.Screen),
		headline,
		status,
	)
	if err != nil {
		return fmt.Errorf("upsert attempt state: %w", err)
	}

	return nil
}

func (i *runIndex) insertRecoveryEvent(event SessionEvent, recordedAt string) error {
	recoveryKind := stringData(event.Data["recovery_kind"])
	providerRecovery := stringData(event.Data["provider_recovery"])
	if recoveryKind == "" && providerRecovery == "" {
		return nil
	}

	_, err := i.db.Exec(
		`INSERT INTO recovery_events (
			session_id, cycle, attempt, recorded_at, screen, run_id, action, kind,
			drift_kind, provider_state, provider_recovery, message,
			expected_state_summary, live_state_summary
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		i.sessionID,
		event.Cycle,
		event.Attempt,
		recordedAt,
		firstNonEmptyString(event.Screen, stateScreen(event.State)),
		firstNonEmptyString(event.RunID, stateRunID(event.State)),
		event.Action,
		recoveryKind,
		stringData(event.Data["drift_kind"]),
		stringData(event.Data["provider_state"]),
		providerRecovery,
		strings.TrimSpace(event.Message),
		stringData(event.Data["expected_state_summary"]),
		stringData(event.Data["live_state_summary"]),
	)
	if err != nil {
		return fmt.Errorf("insert recovery event: %w", err)
	}

	return nil
}

func sessionStatusFromEvent(event SessionEvent) string {
	switch event.Kind {
	case SessionEventStop:
		return "stopped"
	case SessionEventToolError:
		return "recovering"
	case SessionEventReflection:
		return "game_over"
	case SessionEventStatus:
		message := strings.ToLower(strings.TrimSpace(event.Message))
		switch {
		case strings.Contains(message, "falling back"):
			return "fallback"
		case strings.Contains(message, "retrying"):
			return "recovering"
		}
	}
	if providerState := stringData(event.Data["provider_state"]); providerState != "" {
		switch providerState {
		case "fallback":
			return "fallback"
		case "recovering":
			return "recovering"
		}
	}
	return ""
}

func startedAtForLifecycle(lifecycle string, recordedAt string) interface{} {
	switch lifecycle {
	case "start", "bootstrap_next", "running", "map", "combat", "selection":
		return recordedAt
	default:
		return nil
	}
}

func endedAtForLifecycle(lifecycle string, recordedAt string) interface{} {
	if lifecycle == "game_over" || lifecycle == "wrapup" {
		return recordedAt
	}
	return nil
}

func stateScreen(state *game.StateSnapshot) string {
	if state == nil {
		return ""
	}
	return strings.TrimSpace(state.Screen)
}

func stateRunID(state *game.StateSnapshot) string {
	if state == nil {
		return ""
	}
	return strings.TrimSpace(state.RunID)
}

func eventHeadline(event SessionEvent) string {
	if event.State != nil {
		if headline := strings.TrimSpace(agentViewHeadline(event.State)); headline != "" {
			return headline
		}
	}
	return ""
}

func timestampString(value time.Time) string {
	if value.IsZero() {
		value = time.Now()
	}
	return value.UTC().Format(time.RFC3339Nano)
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}

func intOrNil(value *int) interface{} {
	if value == nil {
		return nil
	}
	return *value
}

func marshalJSONString(value interface{}) (string, error) {
	if value == nil {
		return "", nil
	}
	bytes, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func stringData(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return ""
	}
}

func stringPointerData(value interface{}) interface{} {
	text := stringData(value)
	if text == "" {
		return nil
	}
	return text
}

func intData(value interface{}) interface{} {
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	default:
		return nil
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
