package agentruntime

import "testing"

func TestBuildRunQualitySummaryTracksDeeperRunMetrics(t *testing.T) {
	t.Parallel()

	floor4 := 4
	floor8 := 8
	floor18 := 18
	reflections := []*AttemptReflection{
		{RunID: "RUN-3", Attempt: 3, Outcome: "victory", Floor: &floor18},
		{RunID: "RUN-2", Attempt: 2, Outcome: "defeat", Floor: &floor8, Risks: []string{"Died with 120 unspent gold"}},
		{RunID: "RUN-1", Attempt: 1, Outcome: "defeat", Floor: &floor4},
	}
	qualities := map[string]attemptQuality{
		qualityKey("RUN-3", 3): {RunID: "RUN-3", Attempt: 3, ProviderBacked: true},
		qualityKey("RUN-2", 2): {RunID: "RUN-2", Attempt: 2, ProviderBacked: true},
		qualityKey("RUN-1", 1): {RunID: "RUN-1", Attempt: 1, ProviderBacked: true},
	}

	summary := buildRunQualitySummary(reflections, qualities, 3)
	if summary.RecentMedianFloor != 8 {
		t.Fatalf("expected median floor 8, got %+v", summary)
	}
	if summary.RecentBestFloor != 18 {
		t.Fatalf("expected best floor 18, got %+v", summary)
	}
	if summary.RecentFloor7PlusRuns != 2 {
		t.Fatalf("expected 2 recent floor>=7 runs, got %+v", summary)
	}
	if summary.RecentAct2EntryRuns != 1 {
		t.Fatalf("expected 1 recent Act 2 entry run, got %+v", summary)
	}
	if summary.RecentDiedWithGoldRuns != 1 || summary.RecentAverageDeathGold != 120 {
		t.Fatalf("expected died-with-gold stats to be tracked, got %+v", summary)
	}
}

func TestBuildRunQualitySummaryIgnoresNonDefeatGoldRisks(t *testing.T) {
	t.Parallel()

	floor10 := 10
	reflections := []*AttemptReflection{
		{RunID: "RUN-A", Attempt: 1, Outcome: "victory", Floor: &floor10, Risks: []string{"Died with 90 unspent gold"}},
	}
	qualities := map[string]attemptQuality{
		qualityKey("RUN-A", 1): {RunID: "RUN-A", Attempt: 1, ProviderBacked: true},
	}

	summary := buildRunQualitySummary(reflections, qualities, 1)
	if summary.RecentDiedWithGoldRuns != 0 || summary.RecentAverageDeathGold != 0 {
		t.Fatalf("expected non-defeat run to be ignored for died-with-gold stats, got %+v", summary)
	}
}
