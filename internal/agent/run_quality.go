package agentruntime

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type RunQualitySummary struct {
	CompleteRuns               int `json:"complete_runs"`
	ProviderBackedCompleteRuns int `json:"provider_backed_complete_runs"`
	CleanCompleteRuns          int `json:"clean_complete_runs"`
	RecentCompleteRuns         int `json:"recent_complete_runs"`
	RecentProviderBackedRuns   int `json:"recent_provider_backed_runs"`
	RecentCleanRuns            int `json:"recent_clean_runs"`
	RecentFallbackRuns         int `json:"recent_fallback_runs"`
	RecentProviderRetryRuns    int `json:"recent_provider_retry_runs"`
	RecentToolErrorRuns        int `json:"recent_tool_error_runs"`
	RecentMedianFloor          int `json:"recent_median_floor"`
	RecentBestFloor            int `json:"recent_best_floor"`
	RecentFloor7PlusRuns       int `json:"recent_floor7_plus_runs"`
	RecentAct2EntryRuns        int `json:"recent_act2_entry_runs"`
	RecentDiedWithGoldRuns     int `json:"recent_died_with_gold_runs"`
	RecentAverageDeathGold     int `json:"recent_average_death_gold"`
}

type attemptQuality struct {
	RunID            string
	Attempt          int
	ProviderBacked   bool
	ProviderRetry    bool
	ProviderFallback bool
	ToolError        bool
}

func buildRunQualitySummary(reflections []*AttemptReflection, qualities map[string]attemptQuality, recentWindow int) RunQualitySummary {
	summary := RunQualitySummary{}
	recentFloors := make([]int, 0, recentWindow)
	recentDeathGoldTotal := 0
	for index, reflection := range reflections {
		if reflection == nil {
			continue
		}

		summary.CompleteRuns++
		key := qualityKey(reflection.RunID, reflection.Attempt)
		quality := qualities[key]
		if quality.ProviderBacked {
			summary.ProviderBackedCompleteRuns++
		}
		if isCleanAttemptQuality(quality) {
			summary.CleanCompleteRuns++
		}

		if index >= recentWindow {
			continue
		}

		summary.RecentCompleteRuns++
		if reflection.Floor != nil {
			floor := *reflection.Floor
			recentFloors = append(recentFloors, floor)
			if floor > summary.RecentBestFloor {
				summary.RecentBestFloor = floor
			}
			if floor >= 7 {
				summary.RecentFloor7PlusRuns++
			}
			if floor >= 17 {
				summary.RecentAct2EntryRuns++
			}
		}
		if quality.ProviderBacked {
			summary.RecentProviderBackedRuns++
		}
		if isCleanAttemptQuality(quality) {
			summary.RecentCleanRuns++
		}
		if quality.ProviderFallback {
			summary.RecentFallbackRuns++
		}
		if quality.ProviderRetry {
			summary.RecentProviderRetryRuns++
		}
		if quality.ToolError {
			summary.RecentToolErrorRuns++
		}
		if gold, ok := unspentGoldFromReflection(reflection); ok {
			summary.RecentDiedWithGoldRuns++
			recentDeathGoldTotal += gold
		}
	}
	if len(recentFloors) > 0 {
		summary.RecentMedianFloor = medianInt(recentFloors)
	}
	if summary.RecentDiedWithGoldRuns > 0 {
		summary.RecentAverageDeathGold = recentDeathGoldTotal / summary.RecentDiedWithGoldRuns
	}

	return summary
}

func loadAttemptQualitiesFromRunDir(dir string) (map[string]attemptQuality, error) {
	path := filepath.Join(dir, "events.jsonl")
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	qualities := make(map[string]attemptQuality)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var event SessionEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		key := qualityKey(event.RunID, event.Attempt)
		if key == "" {
			continue
		}

		quality := qualities[key]
		if quality.RunID == "" {
			quality.RunID = strings.TrimSpace(event.RunID)
		}
		if quality.Attempt == 0 && event.Attempt > 0 {
			quality.Attempt = event.Attempt
		}

		switch event.Kind {
		case SessionEventPrompt, SessionEventAssistant:
			quality.ProviderBacked = true
		case SessionEventToolError:
			quality.ToolError = true
		}

		providerState := stringData(event.Data["provider_state"])
		providerRecovery := stringData(event.Data["provider_recovery"])
		recoveryKind := stringData(event.Data["recovery_kind"])
		message := strings.ToLower(strings.TrimSpace(event.Message))

		if providerState == "fallback" || providerRecovery == "fallback" || strings.Contains(message, "falling back") {
			quality.ProviderFallback = true
		}
		if providerState == "recovering" ||
			providerRecovery == "provider_retry" ||
			providerRecovery == "transport_restart" ||
			strings.Contains(message, "retrying") {
			quality.ProviderRetry = true
		}
		if recoveryKind == "invalid_action" {
			quality.ToolError = true
		}

		qualities[key] = quality
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return qualities, nil
}

func qualityKey(runID string, attempt int) string {
	if strings.TrimSpace(runID) != "" {
		return "run:" + strings.TrimSpace(runID)
	}
	if attempt > 0 {
		return fmt.Sprintf("attempt:%d", attempt)
	}
	return ""
}

func isCleanAttemptQuality(quality attemptQuality) bool {
	return quality.ProviderBacked &&
		!quality.ProviderFallback &&
		!quality.ProviderRetry &&
		!quality.ToolError
}

func medianInt(values []int) int {
	if len(values) == 0 {
		return 0
	}
	sorted := append([]int(nil), values...)
	for i := 1; i < len(sorted); i++ {
		current := sorted[i]
		j := i - 1
		for j >= 0 && sorted[j] > current {
			sorted[j+1] = sorted[j]
			j--
		}
		sorted[j+1] = current
	}
	mid := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[mid]
	}
	return (sorted[mid-1] + sorted[mid]) / 2
}

func unspentGoldFromReflection(reflection *AttemptReflection) (int, bool) {
	if reflection == nil || !strings.EqualFold(strings.TrimSpace(reflection.Outcome), "defeat") {
		return 0, false
	}
	for _, risk := range reflection.Risks {
		risk = strings.TrimSpace(risk)
		if risk == "" {
			continue
		}
		var gold int
		if _, err := fmt.Sscanf(risk, "Died with %d unspent gold", &gold); err == nil && gold > 0 {
			return gold, true
		}
	}
	return 0, false
}
