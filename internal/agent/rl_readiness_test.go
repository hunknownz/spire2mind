package agentruntime

import "testing"

func TestBuildRLReadinessRequiresProviderBackedAndRecentCleanRuns(t *testing.T) {
	snapshot := &GuidebookSnapshot{
		RunsScanned:        24,
		ReflectionsScanned: 100,
		SeenContent:        &SeenContentRegistry{},
		RunQuality: RunQualitySummary{
			CompleteRuns:               100,
			ProviderBackedCompleteRuns: 59,
			RecentCleanRuns:            3,
			RecentFallbackRuns:         0,
			RecentProviderRetryRuns:    0,
			RecentToolErrorRuns:        0,
		},
	}
	readiness := buildRLReadiness(snapshot, 20)
	if readiness.Ready {
		t.Fatalf("expected readiness to remain false: %#v", readiness)
	}
	if readiness.ProviderBackedRuns != 59 {
		t.Fatalf("expected provider-backed runs to be tracked, got %#v", readiness)
	}
	if readiness.RecentCleanRuns != 3 {
		t.Fatalf("expected recent clean runs to be tracked, got %#v", readiness)
	}
	if readiness.StableRuntime {
		t.Fatalf("expected stable runtime to remain false when recent clean runs are below threshold: %#v", readiness)
	}
}

func TestBuildRLReadinessReadyWhenAllGatesPass(t *testing.T) {
	snapshot := &GuidebookSnapshot{
		RunsScanned:        24,
		ReflectionsScanned: 120,
		SeenContent:        &SeenContentRegistry{},
		RunQuality: RunQualitySummary{
			CompleteRuns:               120,
			ProviderBackedCompleteRuns: 70,
			RecentCleanRuns:            4,
			RecentFallbackRuns:         0,
			RecentProviderRetryRuns:    0,
			RecentToolErrorRuns:        0,
		},
		RecentRecoveryHotspots: []RecoveryHotspot{{RecoveryKind: "soft_replan", DriftKind: driftKindRewardTransition, Count: 1}, {RecoveryKind: "soft_replan", DriftKind: driftKindSameScreenIndexDrift, Count: 2}, {RecoveryKind: "soft_replan", DriftKind: driftKindSelectionSeam, Count: 1}},
	}
	readiness := buildRLReadiness(snapshot, 25)
	if !readiness.Ready {
		t.Fatalf("expected readiness to be true, got %#v", readiness)
	}
	if readiness.Status != "ready" {
		t.Fatalf("expected ready status, got %#v", readiness)
	}
}
