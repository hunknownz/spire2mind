package agentruntime

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// EvolutionLog tracks every completed game for the ratchet evaluation system.
// It persists to combat/knowledge/guidebook/evolution-log.tsv.
//
// Format:
// cycle\tbaseline_avg\tbaseline_n\texperiment_avg\texperiment_n\tverdict\tcommit\tedit_file\tedit_summary\ttimestamp
type EvolutionLog struct {
	path   string
	entries []EvolutionEntry
}

type EvolutionEntry struct {
	Cycle              int
	BaselineAvg        float64
	BaselineN          int
	ExperimentAvg      float64
	ExperimentN        int
	BaselineComposite  float64
	ExperimentComposite float64
	Verdict            string // "keep" | "revert" | "tie" | "insufficient"
	Commit             string
	EditFile           string
	EditSummary        string
	Timestamp          time.Time
}

// newEvolutionLog loads or creates the evolution log.
func newEvolutionLog(guidebookDir string) (*EvolutionLog, error) {
	path := filepath.Join(guidebookDir, "evolution-log.tsv")
	log := &EvolutionLog{path: path}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return log, nil
		}
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		entry, err := parseEvolutionEntry(line)
		if err != nil {
			continue
		}
		log.entries = append(log.entries, *entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return log, nil
}

// recentFloors returns the floors reached in the most recent N run directories.
// It reads from the agent-runs directory.
func recentFloors(artifactsRoot string, n int) ([]int, error) {
	if n <= 0 {
		return nil, nil
	}

	entries, err := os.ReadDir(artifactsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	type candidate struct {
		dir     string
		modTime time.Time
	}
	var candidates []candidate
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		candidates = append(candidates, candidate{
			dir:     filepath.Join(artifactsRoot, entry.Name()),
			modTime: info.ModTime(),
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].modTime.After(candidates[j].modTime)
	})

	if len(candidates) > n {
		candidates = candidates[:n]
	}

	var floors []int
	for _, c := range candidates {
		reflections, err := loadAttemptReflectionsFromRunDir(c.dir)
		if err != nil || len(reflections) == 0 {
			continue
		}
		// Use the last reflection's floor (the one that ended the run)
		last := reflections[len(reflections)-1]
		if last.Floor != nil {
			floors = append(floors, *last.Floor)
		}
	}

	return floors, nil
}

// avgFloor computes the average of a list of floors.
func avgFloor(floors []int) float64 {
	if len(floors) == 0 {
		return 0
	}
	sum := 0
	for _, f := range floors {
		sum += f
	}
	return float64(sum) / float64(len(floors))
}

// RunMetrics captures multi-dimensional performance across a set of runs.
// CompositeScore is the primary ratchet signal; individual dimensions explain why.
type RunMetrics struct {
	N               int     // number of runs
	AvgFloor        float64 // primary: average floor reached
	AvgHPRatio      float64 // final_hp / max_hp at death (higher = healthier deaths)
	GoldEfficiency  float64 // 1 - (avg_gold_at_death / 200), capped [0,1]
	DeckQuality     float64 // 1 - (avg_basic_cards_left / 10), capped [0,1]
	Act2Rate        float64 // fraction of runs that reached floor 17+
	CompositeScore  float64 // weighted combination, primary ratchet signal
}

// Weights for composite score. Tunable via evolution config in future.
const (
	wFloor    = 0.50
	wHP       = 0.15
	wGold     = 0.15
	wDeck     = 0.10
	wAct2     = 0.10
)

// recentRunMetrics loads the N most recent runs and computes RunMetrics.
func recentRunMetrics(artifactsRoot string, n int) (RunMetrics, error) {
	if n <= 0 {
		return RunMetrics{}, nil
	}

	entries, err := os.ReadDir(artifactsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return RunMetrics{}, nil
		}
		return RunMetrics{}, err
	}

	type candidate struct {
		dir     string
		modTime time.Time
	}
	var candidates []candidate
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		candidates = append(candidates, candidate{
			dir:     filepath.Join(artifactsRoot, entry.Name()),
			modTime: info.ModTime(),
		})
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].modTime.After(candidates[j].modTime)
	})
	if len(candidates) > n {
		candidates = candidates[:n]
	}

	var (
		floors       []float64
		hpRatios     []float64
		goldAtDeaths []float64
		basicCards   []float64
		act2Count    int
	)

	for _, c := range candidates {
		reflections, err := loadAttemptReflectionsFromRunDir(c.dir)
		if err != nil || len(reflections) == 0 {
			continue
		}
		last := reflections[len(reflections)-1]
		if last.Floor != nil {
			floors = append(floors, float64(*last.Floor))
		}
		if last.MaxHP > 0 {
			hpRatios = append(hpRatios, float64(last.FinalHP)/float64(last.MaxHP))
		}
		goldAtDeaths = append(goldAtDeaths, float64(last.GoldAtDeath))
		basicCards = append(basicCards, float64(last.BasicCardsLeft))
		if last.ReachedAct2 {
			act2Count++
		}
	}

	m := RunMetrics{N: len(floors)}
	if m.N == 0 {
		return m, nil
	}

	m.AvgFloor = mean(floors)
	if len(hpRatios) > 0 {
		m.AvgHPRatio = mean(hpRatios)
	}
	if len(goldAtDeaths) > 0 {
		// GoldEfficiency: dying with 0 gold = 1.0, dying with 200+ gold = 0.0
		avgGold := mean(goldAtDeaths)
		m.GoldEfficiency = clamp01(1.0 - avgGold/200.0)
	}
	if len(basicCards) > 0 {
		// DeckQuality: 0 basic cards = 1.0, 10+ basic cards = 0.0
		m.DeckQuality = clamp01(1.0 - mean(basicCards)/10.0)
	}
	m.Act2Rate = float64(act2Count) / float64(m.N)

	// Normalize floor to [0,1] range assuming max meaningful floor ~= 50
	normFloor := clamp01(m.AvgFloor / 50.0)
	m.CompositeScore = wFloor*normFloor + wHP*m.AvgHPRatio + wGold*m.GoldEfficiency + wDeck*m.DeckQuality + wAct2*m.Act2Rate

	return m, nil
}

func mean(vals []float64) float64 {
	if len(vals) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range vals {
		sum += v
	}
	return sum / float64(len(vals))
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// RatchetEvalMetrics compares two RunMetrics sets and returns a verdict.
// Uses CompositeScore as the primary signal; threshold is in composite units [0,1].
func RatchetEvalMetrics(baseline, experiment RunMetrics, threshold float64) (verdict string, diff float64) {
	if experiment.N < 2 {
		return "insufficient", 0
	}
	diff = experiment.CompositeScore - baseline.CompositeScore
	if diff > threshold {
		return "keep", diff
	}
	if diff < -threshold {
		return "revert", diff
	}
	return "tie", diff
}

// lastNConverged returns the last N entries that form a "converged" window
// (i.e., where the ratchet kept the change). This gives a fair baseline that
// excludes reverted experiments.
func (l *EvolutionLog) lastNConverged(n int) []EvolutionEntry {
	var converged []EvolutionEntry
	for i := len(l.entries) - 1; i >= 0; i-- {
		if l.entries[i].Verdict == "keep" {
			converged = append(converged, l.entries[i])
		}
		if len(converged) >= n {
			break
		}
	}
	// Reverse to chronological order
	for i, j := 0, len(converged)-1; i < j; i, j = i+1, j-1 {
		converged[i], converged[j] = converged[j], converged[i]
	}
	return converged
}

// appendEntry adds a new entry and persists it.
func (l *EvolutionLog) appendEntry(e EvolutionEntry) error {
	l.entries = append(l.entries, e)

	file, err := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	line := formatEvolutionEntry(e)
	_, err = file.WriteString(line + "\n")
	return err
}

func (l *EvolutionLog) entriesForFile(file string) []EvolutionEntry {
	var result []EvolutionEntry
	for _, e := range l.entries {
		if e.EditFile == file {
			result = append(result, e)
		}
	}
	return result
}

func parseEvolutionEntry(line string) (*EvolutionEntry, error) {
	parts := strings.Split(line, "\t")
	if len(parts) < 10 {
		return nil, fmt.Errorf("too few fields")
	}

	var e EvolutionEntry
	if _, err := fmt.Sscanf(parts[0], "%d", &e.Cycle); err != nil {
		return nil, err
	}
	if _, err := fmt.Sscanf(parts[1], "%f", &e.BaselineAvg); err != nil {
		return nil, err
	}
	if _, err := fmt.Sscanf(parts[2], "%d", &e.BaselineN); err != nil {
		return nil, err
	}
	if _, err := fmt.Sscanf(parts[3], "%f", &e.ExperimentAvg); err != nil {
		return nil, err
	}
	if _, err := fmt.Sscanf(parts[4], "%d", &e.ExperimentN); err != nil {
		return nil, err
	}
	e.Verdict = parts[5]
	e.Commit = parts[6]
	e.EditFile = parts[7]
	e.EditSummary = parts[8]
	if ts, err := time.Parse(time.RFC3339, parts[9]); err == nil {
		e.Timestamp = ts
	}
	// Optional composite scores (fields 10, 11) — backward compatible
	if len(parts) > 10 {
		fmt.Sscanf(parts[10], "%f", &e.BaselineComposite)
	}
	if len(parts) > 11 {
		fmt.Sscanf(parts[11], "%f", &e.ExperimentComposite)
	}
	return &e, nil
}

func formatEvolutionEntry(e EvolutionEntry) string {
	return fmt.Sprintf("%d\t%.2f\t%d\t%.2f\t%d\t%s\t%s\t%s\t%s\t%s\t%.4f\t%.4f",
		e.Cycle,
		e.BaselineAvg,
		e.BaselineN,
		e.ExperimentAvg,
		e.ExperimentN,
		e.Verdict,
		e.Commit,
		e.EditFile,
		e.EditSummary,
		e.Timestamp.Format(time.RFC3339),
		e.BaselineComposite,
		e.ExperimentComposite,
	)
}

// EvolutionConfig tunes the ratchet evaluation.
type EvolutionConfig struct {
	// BaselineWindow is how many recent converged runs to compare against.
	BaselineWindow int
	// GamesPerExperiment is how many headless games to run per experiment.
	GamesPerExperiment int
	// ImprovementThreshold is the minimum floor increase to keep a change (absolute).
	ImprovementThreshold float64
	// CheckpointEvery pauses and asks for confirmation every N cycles.
	CheckpointEvery int
	// Cycles is how many evolution cycles to run (0 = infinite).
	Cycles int
	// GuidebookDir is the combat/knowledge/guidebook directory.
	GuidebookDir string
	// ArtifactsRoot is the scratch/agent-runs directory.
	ArtifactsRoot string
	// RepoRoot is the spire2mind repo root.
	RepoRoot string
	// PlayerSkillDir is the combat/knowledge/player-skill directory.
	PlayerSkillDir string
	// RewardWeightsPath is the path to combat/knowledge/reward-weights.json.
	RewardWeightsPath string
	// LLMProvider is the LLM provider for generating edits (optional).
	// If nil, evolution uses template-based rules instead of LLM-generated content.
	LLMProvider CompleteEvolutioner
}

// DefaultEvolutionConfig returns sensible defaults.
func DefaultEvolutionConfig(repoRoot string) EvolutionConfig {
	return EvolutionConfig{
		BaselineWindow:       10,
		GamesPerExperiment:    10,
		ImprovementThreshold:  0.005, // composite score units [0,1]; ~0.5 floor equivalent
		CheckpointEvery:       10,
		GuidebookDir:          filepath.Join(repoRoot, "combat", "knowledge", "guidebook"),
		ArtifactsRoot:         filepath.Join(repoRoot, "scratch", "agent-runs"),
		RepoRoot:              repoRoot,
		PlayerSkillDir:        filepath.Join(repoRoot, "combat", "knowledge", "player-skill"),
		RewardWeightsPath:     filepath.Join(repoRoot, "combat", "knowledge", "reward-weights.json"),
	}
}

// RatchetEval compares a new set of results against the baseline.
func RatchetEval(baselineFloors, experimentFloors []int, threshold float64) (verdict string, newAvg float64, baselineAvg float64) {
	baselineAvg = avgFloor(baselineFloors)
	newAvg = avgFloor(experimentFloors)
	diff := newAvg - baselineAvg

	if len(experimentFloors) < 2 {
		return "insufficient", newAvg, baselineAvg
	}

	if diff > threshold {
		return "keep", newAvg, baselineAvg
	}
	if diff < -threshold {
		return "revert", newAvg, baselineAvg
	}
	return "tie", newAvg, baselineAvg
}
