package agentruntime

import (
	"os"
	"strings"
	"testing"
	"time"

	"spire2mind/internal/i18n"
)

func TestGuidebookStoreRefreshAggregatesRecentRuns(t *testing.T) {
	repoRoot := t.TempDir()
	artifactsRoot := repoRoot // For simplicity in tests, use same directory

	first, err := NewRunStore(artifactsRoot, "guidebook-a")
	if err != nil {
		t.Fatalf("new first run store: %v", err)
	}
	second, err := NewRunStore(artifactsRoot, "guidebook-b")
	if err != nil {
		t.Fatalf("new second run store: %v", err)
	}

	if err := first.WriteSeenContent(&SeenContentRegistry{
		UpdatedAt: time.Now(),
		Cards: []SeenContentEntry{{
			Category:    seenCategoryCards,
			ID:          "strike_ironclad",
			Name:        "Strike",
			FirstSeenAt: time.Now().Add(-2 * time.Hour),
			LastSeenAt:  time.Now().Add(-90 * time.Minute),
			SeenCount:   2,
		}},
		Monsters: []SeenContentEntry{{
			Category:    seenCategoryMonsters,
			ID:          "slime_small",
			Name:        "Small Slime",
			FirstSeenAt: time.Now().Add(-2 * time.Hour),
			LastSeenAt:  time.Now().Add(-90 * time.Minute),
			SeenCount:   1,
		}},
	}); err != nil {
		t.Fatalf("write first seen content: %v", err)
	}
	if err := first.AppendEvent(SessionEvent{
		Time:    time.Now().Add(-90 * time.Minute),
		Kind:    SessionEventStatus,
		Cycle:   4,
		Attempt: 1,
		Message: "soft seam recovery",
		Data: map[string]interface{}{
			"recovery_kind": "soft_replan",
			"drift_kind":    "selection_seam",
		},
	}); err != nil {
		t.Fatalf("append first recovery event: %v", err)
	}
	if err := first.RecordAttemptReflection(&AttemptReflection{
		Time:          time.Now().Add(-90 * time.Minute),
		Attempt:       1,
		RunID:         "RUN-A",
		Outcome:       "defeat",
		Screen:        "GAME_OVER",
		Floor:         intPtr(8),
		CharacterID:   "IRONCLAD",
		Headline:      "Early elite punished a thin block package.",
		Story:         "The run died after taking a greedy route into an elite with weak defense.",
		NextPlan:      "Route more safely until the deck has reliable block.",
		Lessons:       []string{"Prioritize early block before taking risky elites."},
		LessonBuckets: ReflectionLessonBuckets{Pathing: []string{"Prioritize early block before taking risky elites."}},
		Risks:         []string{"Low-health spiral from early elite greed."},
	}, i18n.LanguageEnglish); err != nil {
		t.Fatalf("record first reflection: %v", err)
	}

	if err := second.WriteSeenContent(&SeenContentRegistry{
		UpdatedAt: time.Now(),
		Relics: []SeenContentEntry{{
			Category:    seenCategoryRelics,
			ID:          "burning_blood",
			Name:        "Burning Blood",
			FirstSeenAt: time.Now().Add(-30 * time.Minute),
			LastSeenAt:  time.Now().Add(-5 * time.Minute),
			SeenCount:   1,
		}},
		Events: []SeenContentEntry{{
			Category:    seenCategoryEvents,
			ID:          "golden_shrine",
			Name:        "Golden Shrine",
			FirstSeenAt: time.Now().Add(-30 * time.Minute),
			LastSeenAt:  time.Now().Add(-5 * time.Minute),
			SeenCount:   1,
		}},
	}); err != nil {
		t.Fatalf("write second seen content: %v", err)
	}
	if err := second.AppendEvent(SessionEvent{
		Time:    time.Now().Add(-5 * time.Minute),
		Kind:    SessionEventStatus,
		Cycle:   9,
		Attempt: 2,
		Message: "reward transition recovery",
		Data: map[string]interface{}{
			"recovery_kind": "soft_replan",
			"drift_kind":    "reward_transition",
		},
	}); err != nil {
		t.Fatalf("append second recovery event: %v", err)
	}
	if err := second.RecordAttemptReflection(&AttemptReflection{
		Time:          time.Now().Add(-5 * time.Minute),
		Attempt:       2,
		RunID:         "RUN-B",
		Outcome:       "victory",
		Screen:        "GAME_OVER",
		Floor:         intPtr(22),
		CharacterID:   "IRONCLAD",
		Headline:      "Midgame scaling paid off.",
		Story:         "The deck kept scaling into act two and converted the stronger route cleanly.",
		NextPlan:      "Keep the scaling package but spend gold earlier.",
		Lessons:       []string{"Convert gold earlier into removals or relics."},
		LessonBuckets: ReflectionLessonBuckets{ShopEconomy: []string{"Convert gold earlier into removals or relics."}},
		Successes:     []string{"Stable scaling plan through the midgame."},
	}, i18n.LanguageEnglish); err != nil {
		t.Fatalf("record second reflection: %v", err)
	}

	_ = first.Close()
	_ = second.Close()

	guidebook, err := NewGuidebookStore(repoRoot)
	if err != nil {
		t.Fatalf("new guidebook store: %v", err)
	}

	snapshot, err := guidebook.Refresh(artifactsRoot, "", i18n.LanguageEnglish)
	if err != nil {
		t.Fatalf("refresh guidebook: %v", err)
	}
	if snapshot == nil {
		t.Fatalf("expected guidebook snapshot")
	}
	if snapshot.RunsScanned != 2 {
		t.Fatalf("expected 2 runs scanned, got %d", snapshot.RunsScanned)
	}
	if snapshot.ReflectionsScanned != 2 {
		t.Fatalf("expected 2 reflections scanned, got %d", snapshot.ReflectionsScanned)
	}
	if snapshot.SeenContent == nil {
		t.Fatalf("expected seen content snapshot")
	}
	counts := snapshot.SeenContent.Counts()
	if counts[seenCategoryCards] != 1 || counts[seenCategoryRelics] != 1 || counts[seenCategoryMonsters] != 1 || counts[seenCategoryEvents] != 1 {
		t.Fatalf("unexpected seen content counts: %#v", counts)
	}
	if len(snapshot.RecoveryHotspots) == 0 {
		t.Fatalf("expected recovery hotspots")
	}
	if snapshot.RecoveryHotspots[0].RecoveryKind != "soft_replan" {
		t.Fatalf("unexpected recovery kind: %#v", snapshot.RecoveryHotspots[0])
	}
	if snapshot.RecentRecoveryWindow != 2 {
		t.Fatalf("expected recent recovery window 2, got %d", snapshot.RecentRecoveryWindow)
	}
	if len(snapshot.RecentRecoveryHotspots) == 0 {
		t.Fatalf("expected recent recovery hotspots")
	}
	if len(snapshot.WeightedRecoveryHotspots) == 0 {
		t.Fatalf("expected weighted recovery hotspots")
	}
	if snapshot.WeightedRecoveryHotspots[0].DriftKind != "reward_transition" {
		t.Fatalf("expected most recent weighted hotspot to favor reward_transition, got %#v", snapshot.WeightedRecoveryHotspots[0])
	}
	if len(snapshot.RecentAttempts) != 2 {
		t.Fatalf("expected 2 recent attempts, got %d", len(snapshot.RecentAttempts))
	}
	if len(snapshot.FailurePatterns) == 0 {
		t.Fatalf("expected failure patterns")
	}
	if len(snapshot.StorySeeds) == 0 {
		t.Fatalf("expected story seeds")
	}
	if len(snapshot.SeenContent.Monsters) == 0 || len(snapshot.SeenContent.Monsters[0].RiskTags) == 0 {
		t.Fatalf("expected monster codex semantics, got %#v", snapshot.SeenContent.Monsters)
	}
	if len(snapshot.SeenContent.Events) == 0 || len(snapshot.SeenContent.Events[0].ResponseHints) == 0 {
		t.Fatalf("expected event response hints, got %#v", snapshot.SeenContent.Events)
	}
	if snapshot.RLReadiness.Ready {
		t.Fatalf("expected RL readiness to remain false, got %#v", snapshot.RLReadiness)
	}
	if snapshot.RLReadiness.CompleteRuns != 2 {
		t.Fatalf("expected 2 complete runs for RL readiness, got %#v", snapshot.RLReadiness)
	}
	if snapshot.RLReadiness.Floor15PlusRuns != 1 {
		t.Fatalf("expected 1 floor>=15 run for RL readiness, got %#v", snapshot.RLReadiness)
	}

	jsonBytes, err := os.ReadFile(guidebook.LivingCodexPath())
	if err != nil {
		t.Fatalf("read living codex: %v", err)
	}
	if !strings.Contains(string(jsonBytes), "\"runs_scanned\": 2") {
		t.Fatalf("living codex missing runs_scanned summary: %s", string(jsonBytes))
	}
	if !strings.Contains(string(jsonBytes), "\"risk_tags\"") {
		t.Fatalf("living codex missing semantic risk tags: %s", string(jsonBytes))
	}

	markdownBytes, err := os.ReadFile(guidebook.GuidebookPath())
	if err != nil {
		t.Fatalf("read guidebook markdown: %v", err)
	}
	if !strings.Contains(string(markdownBytes), "Recovery Hotspots") {
		t.Fatalf("guidebook markdown missing recovery section: %s", string(markdownBytes))
	}
	if !strings.Contains(string(markdownBytes), "RL Readiness") {
		t.Fatalf("guidebook markdown missing RL readiness section: %s", string(markdownBytes))
	}
	if !strings.Contains(string(markdownBytes), "Recent Recovery Hotspots") {
		t.Fatalf("guidebook markdown missing recent recovery section: %s", string(markdownBytes))
	}
	if !strings.Contains(string(markdownBytes), "Recency-Weighted Recovery Trends") {
		t.Fatalf("guidebook markdown missing weighted recovery section: %s", string(markdownBytes))
	}

	combatBytes, err := os.ReadFile(guidebook.CombatPlaybookPath())
	if err != nil {
		t.Fatalf("read combat playbook markdown: %v", err)
	}
	if !strings.Contains(string(combatBytes), "Combat Playbook") {
		t.Fatalf("combat playbook missing heading: %s", string(combatBytes))
	}
	if !strings.Contains(string(combatBytes), "Observed Monsters") {
		t.Fatalf("combat playbook missing monsters section: %s", string(combatBytes))
	}

	eventBytes, err := os.ReadFile(guidebook.EventPlaybookPath())
	if err != nil {
		t.Fatalf("read event playbook markdown: %v", err)
	}
	if !strings.Contains(string(eventBytes), "Event Playbook") {
		t.Fatalf("event playbook missing heading: %s", string(eventBytes))
	}
	if !strings.Contains(string(eventBytes), "Observed Events") {
		t.Fatalf("event playbook missing observed events section: %s", string(eventBytes))
	}
}

func TestDedupeGuidebookReflectionsPrefersLatestPerRunID(t *testing.T) {
	now := time.Now()
	reflections := []*AttemptReflection{
		{
			Time:        now,
			RunID:       "RUN-X",
			Outcome:     "defeat",
			Floor:       intPtr(17),
			Headline:    "latest headline",
			NextPlan:    "latest plan",
			CharacterID: "IRONCLAD",
		},
		{
			Time:        now.Add(-time.Minute),
			RunID:       "RUN-X",
			Outcome:     "defeat",
			Floor:       intPtr(17),
			Headline:    "older headline",
			NextPlan:    "older plan",
			CharacterID: "IRONCLAD",
		},
		{
			Time:        now.Add(-2 * time.Minute),
			RunID:       "RUN-Y",
			Outcome:     "victory",
			Floor:       intPtr(20),
			Headline:    "another run",
			NextPlan:    "another plan",
			CharacterID: "IRONCLAD",
		},
	}

	deduped := dedupeGuidebookReflections(reflections)
	if len(deduped) != 2 {
		t.Fatalf("expected 2 deduped reflections, got %d", len(deduped))
	}
	if deduped[0].RunID != "RUN-X" || deduped[0].Headline != "latest headline" {
		t.Fatalf("expected latest RUN-X reflection to be kept, got %#v", deduped[0])
	}
	if deduped[1].RunID != "RUN-Y" {
		t.Fatalf("expected RUN-Y reflection to remain, got %#v", deduped[1])
	}
}
