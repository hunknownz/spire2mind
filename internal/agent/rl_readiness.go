package agentruntime

import (
	"fmt"
	"strings"
)

const (
	rlRequiredCompleteRuns       = 100
	rlRequiredFloor15Runs        = 20
	rlRequiredProviderBackedRuns = 60
	rlRequiredRecentCleanRuns    = 4
)

type RLReadiness struct {
	Ready                      bool     `json:"ready"`
	Status                     string   `json:"status"`
	Blockers                   []string `json:"blockers,omitempty"`
	CompleteRuns               int      `json:"complete_runs"`
	Floor15PlusRuns            int      `json:"floor15_plus_runs"`
	ProviderBackedRuns         int      `json:"provider_backed_runs"`
	RecentCleanRuns            int      `json:"recent_clean_runs"`
	RequiredRuns               int      `json:"required_runs"`
	RequiredFloor15            int      `json:"required_floor15"`
	RequiredProviderBackedRuns int      `json:"required_provider_backed_runs"`
	RequiredRecentCleanRuns    int      `json:"required_recent_clean_runs"`
	StableRuntime              bool     `json:"stable_runtime"`
	KnowledgeAssetsOK          bool     `json:"knowledge_assets_ok"`
}

func buildRLReadiness(snapshot *GuidebookSnapshot, floor15PlusRuns int) RLReadiness {
	readiness := RLReadiness{
		CompleteRuns:               snapshot.ReflectionsScanned,
		Floor15PlusRuns:            floor15PlusRuns,
		ProviderBackedRuns:         snapshot.RunQuality.ProviderBackedCompleteRuns,
		RecentCleanRuns:            snapshot.RunQuality.RecentCleanRuns,
		RequiredRuns:               rlRequiredCompleteRuns,
		RequiredFloor15:            rlRequiredFloor15Runs,
		RequiredProviderBackedRuns: rlRequiredProviderBackedRuns,
		RequiredRecentCleanRuns:    rlRequiredRecentCleanRuns,
		StableRuntime:              stableRuntimeWindow(snapshot),
		KnowledgeAssetsOK: snapshot != nil &&
			snapshot.SeenContent != nil &&
			snapshot.RunsScanned > 0,
	}

	if readiness.CompleteRuns < readiness.RequiredRuns {
		readiness.Blockers = append(readiness.Blockers, fmt.Sprintf("complete runs %d/%d", readiness.CompleteRuns, readiness.RequiredRuns))
	}
	if readiness.Floor15PlusRuns < readiness.RequiredFloor15 {
		readiness.Blockers = append(readiness.Blockers, fmt.Sprintf("floor>=15 runs %d/%d", readiness.Floor15PlusRuns, readiness.RequiredFloor15))
	}
	if readiness.ProviderBackedRuns < readiness.RequiredProviderBackedRuns {
		readiness.Blockers = append(readiness.Blockers, fmt.Sprintf("provider-backed runs %d/%d", readiness.ProviderBackedRuns, readiness.RequiredProviderBackedRuns))
	}
	if readiness.RecentCleanRuns < readiness.RequiredRecentCleanRuns {
		readiness.Blockers = append(readiness.Blockers, fmt.Sprintf("recent clean runs %d/%d", readiness.RecentCleanRuns, readiness.RequiredRecentCleanRuns))
	}
	if !readiness.StableRuntime {
		readiness.Blockers = append(readiness.Blockers, "stable runtime=false")
	}
	if !readiness.KnowledgeAssetsOK {
		readiness.Blockers = append(readiness.Blockers, "knowledge assets=false")
	}

	readiness.Ready = len(readiness.Blockers) == 0

	if readiness.Ready {
		readiness.Status = "ready"
		return readiness
	}

	readiness.Status = "not ready yet: " + strings.Join(readiness.Blockers, ", ")
	return readiness
}

func stableRuntimeWindow(snapshot *GuidebookSnapshot) bool {
	if snapshot == nil {
		return false
	}
	reward := 0
	drift := 0
	selection := 0
	for _, hotspot := range snapshot.RecentRecoveryHotspots {
		switch hotspot.DriftKind {
		case driftKindRewardTransition:
			reward += hotspot.Count
		case driftKindSameScreenIndexDrift:
			drift += hotspot.Count
		case driftKindSelectionSeam:
			selection += hotspot.Count
		}
	}
	return reward <= 1 &&
		drift <= 2 &&
		selection <= 1 &&
		snapshot.RunQuality.RecentFallbackRuns == 0 &&
		snapshot.RunQuality.RecentProviderRetryRuns == 0 &&
		snapshot.RunQuality.RecentToolErrorRuns == 0 &&
		snapshot.RunQuality.RecentCleanRuns >= rlRequiredRecentCleanRuns
}
