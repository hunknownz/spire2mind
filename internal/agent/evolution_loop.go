package agentruntime

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// EvolutionLoop runs the full ratchet evolution cycle.
//
// Loop:
//  1. Load recent floors → establish baseline
//  2. Analyze guidebook weaknesses → propose edit (via LLM)
//  3. git commit (save experiment point)
//  4. Run N headless games
//  5. Compare new avg floor vs baseline
//  6. Keep or revert
//  7. Log to evolution-log.tsv
//  8. Checkpoint every N cycles
//
// Exploratory Rewrite: When consecutive failures reach threshold,
// propose a full file rewrite instead of incremental edits.
type EvolutionLoop struct {
	cfg                 EvolutionConfig
	log                 *EvolutionLog
	evLogOut            *os.File
	stdout              io.Writer // real-time progress output (e.g. os.Stdout)
	playerSkill         *PlayerSkillStore
	llmEvolver          *LLMEvolver
	consecutiveFailures int
	lastEditFile        string
}

// NewEvolutionLoop creates a new evolution loop.
func NewEvolutionLoop(cfg EvolutionConfig) (*EvolutionLoop, error) {
	evLog, err := newEvolutionLog(cfg.GuidebookDir)
	if err != nil {
		return nil, fmt.Errorf("evolution log: %w", err)
	}

	// Create a dedicated evolution log file for this session
	sessionLogPath := filepath.Join(cfg.GuidebookDir, fmt.Sprintf("evolution-session-%s.log", time.Now().Format("20060102-150405")))
	sessionLog, err := os.Create(sessionLogPath)
	if err != nil {
		return nil, fmt.Errorf("session log: %w", err)
	}

	loop := &EvolutionLoop{
		cfg:      cfg,
		log:      evLog,
		evLogOut: sessionLog,
		stdout:   io.Discard, // caller sets this; default to discard
	}

	// Load player skill store (non-fatal if missing)
	if cfg.PlayerSkillDir != "" {
		scratchRoot := filepath.Dir(cfg.PlayerSkillDir)
		ps, err := NewPlayerSkillStore(scratchRoot)
		if err != nil {
			fmt.Fprintf(sessionLog, "[warn] player skill store: %v\n", err)
		} else {
			loop.playerSkill = ps
		}
	}

	// Initialize LLM evolver if provider is configured
	if cfg.LLMProvider != nil {
		loop.llmEvolver = NewLLMEvolver(cfg.LLMProvider)
		fmt.Fprintf(sessionLog, "[info] LLM evolver initialized (using LLM-generated edits)\n")
	} else {
		fmt.Fprintf(sessionLog, "[info] No LLM provider configured (using template-based edits)\n")
	}

	return loop, nil
}

// Run executes the evolution loop for the given number of cycles.
// It stops early if a checkpoint is requested.
func (l *EvolutionLoop) Run(ctx context.Context, cycles int, headlessBinary string) error {
	l.logf("=== Spire2Mind Evolution Started ===")
	l.logf("Cycles: %d, Games/experiment: %d, Baseline window: %d",
		cycles, l.cfg.GamesPerExperiment, l.cfg.BaselineWindow)
	l.logf("Artifacts: %s", l.cfg.ArtifactsRoot)
	l.logf("Guidebook: %s", l.cfg.GuidebookDir)

	for cycle := 1; cycle <= cycles; cycle++ {
		l.logf("\n--- Cycle %d ---", cycle)

		result, err := l.runCycle(ctx, cycle, headlessBinary)
		if err != nil {
			l.logf("Cycle %d ERROR: %v", cycle, err)
			// Don't abort the loop on a single cycle failure
			continue
		}

		l.logf("Cycle %d: verdict=%s baseline=%.1f new=%.1f diff=%.1f edit=%s",
			cycle, result.Verdict, result.BaselineAvg, result.ExperimentAvg,
			result.ExperimentAvg-result.BaselineAvg, result.EditSummary)

		if result.Verdict == "keep" {
			l.logf("✓ KEPT: %s", result.EditSummary)
		} else if result.Verdict == "revert" {
			l.logf("✗ REVERTED: %s", result.EditSummary)
		} else {
			l.logf("≈ TIE/INSUFFICIENT: %s", result.EditSummary)
		}

		// Checkpoint
		if l.cfg.CheckpointEvery > 0 && cycle%l.cfg.CheckpointEvery == 0 {
			l.logf("\n=== Checkpoint after cycle %d ===", cycle)
			l.PrintStatus()
			// In a real interactive session this would prompt for confirmation.
			// For headless use, we just continue.
		}
	}

	l.logf("\n=== Evolution Complete ===")
	l.PrintStatus()
	return nil
}

type CycleResult struct {
	Verdict       string
	BaselineAvg   float64
	ExperimentAvg float64
	BaselineN     int
	ExperimentN   int
	Commit        string
	EditFile      string
	EditSummary   string
}

// runCycle executes one evolution cycle.
// Uses simultaneous A/B testing: runs baseline games first, then applies edit, then runs experiment games.
// This ensures fair comparison between strategies with similar random conditions.
func (l *EvolutionLoop) runCycle(ctx context.Context, cycle int, headlessBinary string) (*CycleResult, error) {
	// Step 1: Run baseline games with current strategy (before any edit)
	l.logf("=== Running baseline games (current strategy) ===")
	l.logf("Running %d baseline games...", l.cfg.GamesPerExperiment)
	baselineFloors, err := l.runHeadlessGames(ctx, l.cfg.GamesPerExperiment, headlessBinary)
	if err != nil {
		return nil, fmt.Errorf("run baseline games: %w", err)
	}
	baselineMetrics := l.computeMetricsFromFloors(baselineFloors)
	l.logf("Baseline: n=%d floor=%.1f hp=%.2f gold_eff=%.2f deck=%.2f act2=%.0f%% composite=%.4f",
		baselineMetrics.N, baselineMetrics.AvgFloor, baselineMetrics.AvgHPRatio,
		baselineMetrics.GoldEfficiency, baselineMetrics.DeckQuality,
		baselineMetrics.Act2Rate*100, baselineMetrics.CompositeScore)

	// Step 2: Determine layer and propose edit
	var edit EvolutionEdit
	var editPaths []string

	switch {
	case cycle%3 == 0 && l.cfg.RewardWeightsPath != "":
		// Layer 3: reward card weights (deck-building)
		edit, err = l.proposeRewardWeightsEdit()
		editPaths = []string{"combat/knowledge/"}
		l.logf("Layer: reward-weights")
	case cycle%2 == 0 && l.playerSkill != nil:
		// Layer 2: player skill
		edit, err = l.proposePlayerSkillEdit()
		editPaths = []string{"combat/knowledge/player-skill/"}
		l.logf("Layer: player-skill")
	default:
		// Layer 1: guidebook rules
		edit, err = l.proposeGuidebookEdit()
		editPaths = []string{"combat/knowledge/guidebook/"}
		l.logf("Layer: guidebook")
	}
	if err != nil {
		return nil, fmt.Errorf("propose edit: %w", err)
	}
	l.logf("Edit: %s", edit.Summary)

	// Step 3: Apply the edit
	commit := ""
	if err := edit.Apply(); err != nil {
		return nil, fmt.Errorf("apply edit: %w", err)
	}
	l.logf("Applied edit to %s", filepath.Base(edit.File))

	// Step 4: Run experiment games with new strategy
	l.logf("=== Running experiment games (new strategy) ===")
	l.logf("Running %d experiment games...", l.cfg.GamesPerExperiment)
	experimentFloors, err := l.runHeadlessGames(ctx, l.cfg.GamesPerExperiment, headlessBinary)
	if err != nil {
		l.revertPaths(commit, editPaths)
		return nil, fmt.Errorf("run experiment games: %w", err)
	}
	experimentMetrics := l.computeMetricsFromFloors(experimentFloors)
	l.logf("Experiment: n=%d floor=%.1f hp=%.2f gold_eff=%.2f deck=%.2f act2=%.0f%% composite=%.4f",
		experimentMetrics.N, experimentMetrics.AvgFloor, experimentMetrics.AvgHPRatio,
		experimentMetrics.GoldEfficiency, experimentMetrics.DeckQuality,
		experimentMetrics.Act2Rate*100, experimentMetrics.CompositeScore)

	// Step 5: Compare and decide
	verdict, diff := RatchetEvalMetrics(baselineMetrics, experimentMetrics, l.cfg.ImprovementThreshold)
	l.logf("Ratchet: verdict=%s composite_diff=%.4f", verdict, diff)

	// Step 6: Keep or revert
	if verdict == "revert" {
		if err := l.revertPaths(commit, editPaths); err != nil {
			l.logf("revert: %v (non-fatal)", err)
		}
		l.logf("Reverted edit to %s", filepath.Base(edit.File))
		if l.playerSkill != nil && edit.File == l.playerSkill.Path() {
			_ = l.playerSkill.Reload()
		}
		// Track consecutive failures for exploratory rewrite
		if edit.File == l.lastEditFile {
			l.consecutiveFailures++
		} else {
			l.consecutiveFailures = 1
			l.lastEditFile = edit.File
		}
	} else if verdict == "keep" {
		commit, _ = gitCommitPaths(l.cfg.RepoRoot, fmt.Sprintf("evolution cycle %d: keep %s", cycle, edit.Summary), editPaths)
		l.logf("Kept edit, committed as %s", commit)
		// Reset failure counter on success
		l.consecutiveFailures = 0
	}

	// Step 7: Log to evolution-log.tsv
	entry := EvolutionEntry{
		Cycle:               cycle,
		BaselineAvg:         baselineMetrics.AvgFloor,
		BaselineN:           baselineMetrics.N,
		ExperimentAvg:       experimentMetrics.AvgFloor,
		ExperimentN:         experimentMetrics.N,
		BaselineComposite:   baselineMetrics.CompositeScore,
		ExperimentComposite: experimentMetrics.CompositeScore,
		Verdict:             verdict,
		Commit:              commit,
		EditFile:            edit.File,
		EditSummary:         edit.Summary,
		Timestamp:           time.Now(),
	}
	if err := l.log.appendEntry(entry); err != nil {
		l.logf("log entry: %v", err)
	}

	return &CycleResult{
		Verdict:       verdict,
		BaselineAvg:   baselineMetrics.AvgFloor,
		ExperimentAvg: experimentMetrics.AvgFloor,
		BaselineN:     baselineMetrics.N,
		ExperimentN:   experimentMetrics.N,
		Commit:        commit,
		EditFile:      edit.File,
		EditSummary:   edit.Summary,
	}, nil
}

// runHeadlessGames runs N headless games and returns the floor each ended on.
func (l *EvolutionLoop) runHeadlessGames(ctx context.Context, n int, binary string) ([]int, error) {
	var floors []int

	for i := 0; i < n; i++ {
		l.logf("  Game %d/%d...", i+1, n)

		// Run one headless game using the spire2mind binary
		cmd := exec.CommandContext(ctx, binary, "play", "--headless", "--attempts", "1")
		cmd.Dir = l.cfg.RepoRoot
		cmd.Stdout = l.evLogOut
		cmd.Stderr = l.evLogOut

		start := time.Now()
		if err := cmd.Run(); err != nil {
			l.logf("  Game %d error: %v", i+1, err)
			// Still record the floor if we can find it
		}
		l.logf("  Game %d took %s", i+1, time.Since(start))

		// Get the floor from the most recent run directory
		f, err := lastFloor(l.cfg.ArtifactsRoot)
		if err == nil && f > 0 {
			floors = append(floors, f)
		}
	}

	if len(floors) == 0 {
		return nil, fmt.Errorf("no floors recorded from %d games", n)
	}
	return floors, nil
}

// lastFloor returns the floor reached in the most recent run.
func lastFloor(artifactsRoot string) (int, error) {
	entries, err := os.ReadDir(artifactsRoot)
	if err != nil {
		return 0, err
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
	if len(candidates) == 0 {
		return 0, fmt.Errorf("no run directories found")
	}

	// Find most recent
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].modTime.After(candidates[j].modTime)
	})

	reflections, err := loadAttemptReflectionsFromRunDir(candidates[0].dir)
	if err != nil || len(reflections) == 0 {
		return 0, fmt.Errorf("no reflections in %s", candidates[0].dir)
	}
	// Use last reflection
	last := reflections[len(reflections)-1]
	if last.Floor == nil {
		return 0, fmt.Errorf("no floor in reflection")
	}
	return *last.Floor, nil
}

// revert restores the guidebook files to the given git commit.
func (l *EvolutionLoop) revert(commit string) error {
	return l.revertPaths(commit, []string{"combat/knowledge/guidebook/"})
}

// revertPaths restores specific paths to the given git commit.
// For paths in submodules (e.g., combat/), it operates inside the submodule.
func (l *EvolutionLoop) revertPaths(commit string, paths []string) error {
	if commit == "" {
		// No pre-experiment commit, revert to current HEAD (discard uncommitted changes)
		commit = "HEAD"
	}

	// Check if paths are in a submodule
	var submoduleDir string
	var relativePaths []string

	for _, p := range paths {
		if strings.HasPrefix(p, "combat/") {
			submoduleDir = filepath.Join(l.cfg.RepoRoot, "combat")
			relPath := strings.TrimPrefix(p, "combat/")
			relativePaths = append(relativePaths, relPath)
		} else {
			relativePaths = append(relativePaths, p)
		}
	}

	workDir := l.cfg.RepoRoot
	if submoduleDir != "" {
		workDir = submoduleDir
	}

	// Create context with timeout to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	args := append([]string{"checkout", commit, "--"}, relativePaths...)
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git checkout: %s: %w", string(out), err)
	}
	return nil
}

// refreshGuidebook rebuilds the guidebook files from scratch after an edit.
func (l *EvolutionLoop) refreshGuidebook(targetFile string) error {
	// Re-run the guidebook store refresh
	// This is handled automatically by the next headless run
	// which calls GuidebookStore.Refresh()
	return nil
}

// proposeGuidebookEdit analyzes guidebook weaknesses and returns an edit targeting
// the ## Evolution Rules section of the appropriate guidebook file.
// If LLM evolver is configured, it uses LLM to generate the edit; otherwise uses templates.
func (l *EvolutionLoop) proposeGuidebookEdit() (EvolutionEdit, error) {
	analysis, err := AnalyzeGuidebook(l.cfg.ArtifactsRoot, l.cfg.GuidebookDir, 6)
	if err != nil {
		return EvolutionEdit{}, fmt.Errorf("analyze guidebook: %w", err)
	}

	if len(analysis.Weaknesses) == 0 {
		l.logf("No guidebook weaknesses found — using exploratory tweak")
		analysis.Weaknesses = append(analysis.Weaknesses, GuidebookWeakness{
			Category:    "combat",
			Severity:    "minor",
			Description: "Exploratory: test a minor combat rule addition",
			Suggestion:  "当前台面有 >= 2 个敌人时，优先击杀低血量目标而非高威胁目标。",
			TargetFile:  "combat-playbook.md",
		})
	}

	weakness := analysis.Weaknesses[0]
	l.logf("Guidebook weakness: [%s/%s] %s", weakness.Category, weakness.Severity, weakness.Description)

		targetFile := filepath.Join(l.cfg.GuidebookDir, weakness.TargetFile)

	// Check for exploratory rewrite trigger (consecutive failures on same file)
	const exploratoryThreshold = 3
	if l.consecutiveFailures >= exploratoryThreshold && l.lastEditFile == targetFile && l.llmEvolver != nil {
		l.logf("=== EXPLORATORY REWRITE TRIGGERED ===")
		l.logf("连续%d次失败于同一文件，尝试突破性重写...", l.consecutiveFailures)
		edit, err := l.llmEvolver.ProposeExploratoryRewrite(context.Background(), targetFile, analysis)
		if err != nil {
			l.logf("探索性重写失败: %v, 回退到常规编辑", err)
		} else {
			// Reset failure counter after exploratory rewrite
			l.consecutiveFailures = 0
			return edit, nil
		}
	}

	// Use LLM evolver if configured
	if l.llmEvolver != nil {
		l.logf("Using LLM to generate edit...")
		edit, err := l.llmEvolver.ProposeGuidebookEdit(context.Background(), targetFile, analysis)
		if err != nil {
			l.logf("LLM edit failed: %v, falling back to template", err)
		} else {
			return edit, nil
		}
	}

	// Fallback: template-based edit
	bullet := weakness.Suggestion
	if bullet == "" {
		bullet = weakness.Description
	}

	// Read existing content to check for Evolution Rules section
	existingContent, err := os.ReadFile(targetFile)
	if err != nil {
		return EvolutionEdit{}, fmt.Errorf("read target file: %w", err)
	}

	// Check if ## Evolution Rules section already exists
	var oldText, newText string
	if strings.Contains(string(existingContent), "## Evolution Rules") {
		// Append to existing section
		oldText = "## Evolution Rules\n"
		newText = "## Evolution Rules\n\n- " + bullet + "\n"
	} else {
		// Create new section
		oldText = ""
		newText = "\n\n## Evolution Rules\n\n- " + bullet + "\n"
	}

	return EvolutionEdit{
		File:     targetFile,
		OldText:  oldText,
		NewText:  newText,
		Summary:  fmt.Sprintf("[template/%s/%s] %s", weakness.Category, weakness.Severity, weakness.Description),
		Category: weakness.Category,
	}, nil
}

// proposePlayerSkillEdit analyzes player skill weaknesses and returns an edit
// targeting the appropriate section of sts2-player-skill.md.
func (l *EvolutionLoop) proposePlayerSkillEdit() (EvolutionEdit, error) {
	if l.playerSkill == nil {
		return EvolutionEdit{}, fmt.Errorf("player skill store not initialized")
	}

	analysis, err := AnalyzePlayerSkill(l.cfg.ArtifactsRoot, l.playerSkill.Path(), 6)
	if err != nil {
		return EvolutionEdit{}, fmt.Errorf("analyze player skill: %w", err)
	}

	top := analysis.TopWeakness()
	if top == nil {
		l.logf("No player skill weaknesses found — using exploratory directive")
		return EvolutionEdit{
			File:     l.playerSkill.Path(),
			OldText:  "",
			NewText:  "\n- Exploratory: when facing an unknown enemy pattern, default to block-first for one turn\n",
			Summary:  "[player-skill/combat/minor] Exploratory directive",
			Category: "combat",
		}, nil
	}

	l.logf("Player skill weakness: [%s/%s] %s", top.Section, top.Severity, top.Description)

	// Build the edit: append bullet to the target section
	// We use AppendToSection which handles section creation
	// But EvolutionEdit.Apply() does a raw string replace, so we need to
	// find the section end and insert there. Use a simpler approach:
	// read current content, apply via PlayerSkillStore, then return a no-op EvolutionEdit
	// that just records what happened.
	if err := l.playerSkill.AppendToSection(top.Section, top.NewBullet); err != nil {
		return EvolutionEdit{}, fmt.Errorf("append to section: %w", err)
	}

	return EvolutionEdit{
		File:     l.playerSkill.Path(),
		OldText:  "", // already applied above
		NewText:  "", // already applied above
		Summary:  fmt.Sprintf("[player-skill/%s/%s] %s", top.Section, top.Severity, top.Description),
		Category: top.Section,
	}, nil
}

// layerName returns a human-readable layer name.
func layerName(playerSkill bool) string {
	if playerSkill {
		return "player-skill"
	}
	return "guidebook"
}

// proposeRewardWeightsEdit nudges one scoring weight in reward-weights.json.
// It reads the current weights, identifies the most-penalized card category
// from recent reflections, and adjusts the relevant multiplier by ±0.1.
func (l *EvolutionLoop) proposeRewardWeightsEdit() (EvolutionEdit, error) {
	path := l.cfg.RewardWeightsPath
	data, err := os.ReadFile(path)
	if err != nil {
		return EvolutionEdit{}, fmt.Errorf("read reward-weights: %w", err)
	}

	// Analyze recent reflections to find the most common reward mistake
	reflections, err := LoadRecentAttemptReflections(l.cfg.ArtifactsRoot, "", 6)
	if err != nil || len(reflections) == 0 {
		return EvolutionEdit{}, fmt.Errorf("load reflections: %w", err)
	}

	// Count reward_choice lessons to find what's going wrong
	earlyScaleCount := 0
	lateDefenseCount := 0
	for _, r := range reflections {
		for _, lesson := range r.LessonBuckets.RewardChoice {
			lower := strings.ToLower(lesson)
			if strings.Contains(lower, "scale") || strings.Contains(lower, "power") {
				earlyScaleCount++
			}
			if strings.Contains(lower, "defense") || strings.Contains(lower, "block") || strings.Contains(lower, "survive") {
				lateDefenseCount++
			}
		}
	}

	// Pick a targeted nudge based on what's failing
	var oldText, newText, summary string
	switch {
	case earlyScaleCount >= 2:
		// Picked scaling cards too early — reduce late-timing bonus
		oldText = `"late_card_late_floor_bonus": 2.0`
		newText = `"late_card_late_floor_bonus": 1.7`
		summary = "[reward-weights/minor] reduce late_card_late_floor_bonus: scaling cards picked too early"
	case lateDefenseCount >= 2:
		// Not picking defense when low HP — increase defense HP bonus
		oldText = `"defense_low_hp_bonus": 3.0`
		newText = `"defense_low_hp_bonus": 3.5`
		summary = "[reward-weights/major] increase defense_low_hp_bonus: defense underprioritized at low HP"
	default:
		// Exploratory: slightly increase knowledge-base score weight
		oldText = `"knowledge_score_multiplier": 1.5`
		newText = `"knowledge_score_multiplier": 1.6`
		summary = "[reward-weights/minor] exploratory: increase knowledge_score_multiplier"
	}

	if !strings.Contains(string(data), oldText) {
		// Field not found — skip this cycle
		return EvolutionEdit{}, fmt.Errorf("reward-weights field not found: %q", oldText)
	}

	return EvolutionEdit{
		File:     path,
		OldText:  oldText,
		NewText:  newText,
		Summary:  summary,
		Category: "reward",
	}, nil
}

// gitCommitPaths runs git add on specific paths + commit and returns the commit hash.
// For paths in submodules (e.g., combat/), it operates inside the submodule.
func gitCommitPaths(repoRoot, message string, paths []string) (string, error) {
	// Check if paths are in a submodule
	var submoduleDir string
	var relativePaths []string

	for _, p := range paths {
		if strings.HasPrefix(p, "combat/") {
			submoduleDir = filepath.Join(repoRoot, "combat")
			// Strip "combat/" prefix for operations inside the submodule
			relPath := strings.TrimPrefix(p, "combat/")
			relativePaths = append(relativePaths, relPath)
		} else {
			relativePaths = append(relativePaths, p)
		}
	}

	workDir := repoRoot
	if submoduleDir != "" {
		workDir = submoduleDir
	}

	// Create context with timeout to prevent hanging
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Git add
	addArgs := append([]string{"add"}, relativePaths...)
	addCmd := exec.CommandContext(ctx, "git", addArgs...)
	addCmd.Dir = workDir
	addCmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_EDITOR=cat")
	addOut, err := addCmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git add %v in %s: %s: %w", relativePaths, workDir, string(addOut), err)
	}

	// Git commit
	commitCmd := exec.CommandContext(ctx, "git", "commit", "-m", message, "--no-edit")
	commitCmd.Dir = workDir
	commitCmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_EDITOR=cat")
	out, err := commitCmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "nothing to commit") {
			return "", nil
		}
		return "", fmt.Errorf("git commit in %s: %s: %w", workDir, string(out), err)
	}

	hashCmd := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	hashCmd.Dir = workDir
	hashOut, err := hashCmd.Output()
	if err != nil {
		return "", nil
	}

	commitHash := strings.TrimSpace(string(hashOut))

	// If we committed in a submodule, also update the main repo to point to the new commit
	if submoduleDir != "" {
		updateCmd := exec.CommandContext(ctx, "git", "add", "combat")
		updateCmd.Dir = repoRoot
		if _, err := updateCmd.CombinedOutput(); err != nil {
			return commitHash, nil // Return submodule hash even if main repo update fails
		}
		commitMainCmd := exec.CommandContext(ctx, "git", "commit", "-m", "Update combat submodule: "+message)
		commitMainCmd.Dir = repoRoot
		commitMainCmd.CombinedOutput() // Ignore errors (may be nothing to commit)
	}

	return commitHash, nil
}

// gitCommit runs git add + commit and returns the commit hash.
func gitCommit(repoRoot, message string) (string, error) {
	return gitCommitPaths(repoRoot, message, []string{"combat/knowledge/guidebook/"})
}

// PrintStatus prints a summary of the current evolution state.
func (l *EvolutionLoop) PrintStatus() {
	var keeps, reverts, ties int
	for _, e := range l.log.entries {
		switch e.Verdict {
		case "keep":
			keeps++
		case "revert":
			reverts++
		default:
			ties++
		}
	}

	l.logf("=== Evolution Status ===")
	l.logf("Total cycles: %d | Kept: %d | Reverted: %d | Ties: %d", len(l.log.entries), keeps, reverts, ties)

	if len(l.log.entries) > 0 {
		last := l.log.entries[len(l.log.entries)-1]
		l.logf("Current avg floor: %.1f (last %d runs) | composite: %.4f", last.ExperimentAvg, last.ExperimentN, last.ExperimentComposite)
	}

	l.logf("Log: %s", l.log.path)
}

func (l *EvolutionLoop) logf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("15:04:05")
	line := fmt.Sprintf("[%s] %s", timestamp, msg)
	fmt.Fprintln(l.evLogOut, line)
	fmt.Fprintln(l.stdout, line)
}

// SetStdout configures a writer for real-time progress output (e.g. os.Stdout).
func (l *EvolutionLoop) SetStdout(w io.Writer) {
	l.stdout = w
}

// Close closes the evolution loop.
func (l *EvolutionLoop) Close() error {
	return l.evLogOut.Close()
}

// computeMetricsFromFloors computes RunMetrics from the most recent N runs.
// N is determined by the length of floors slice passed in.
// It reads the full attempt reflections to compute all metrics dimensions.
func (l *EvolutionLoop) computeMetricsFromFloors(floors []int) RunMetrics {
	if len(floors) == 0 {
		return RunMetrics{}
	}

	// Get the most recent N run directories
	n := len(floors)
	entries, err := os.ReadDir(l.cfg.ArtifactsRoot)
	if err != nil {
		// Fallback: compute floor-only metrics
		return RunMetrics{
			N:            len(floors),
			AvgFloor:     avgFloor(floors),
			CompositeScore: clamp01(avgFloor(floors) / 50.0) * wFloor,
		}
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
			dir:     filepath.Join(l.cfg.ArtifactsRoot, entry.Name()),
			modTime: info.ModTime(),
		})
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].modTime.After(candidates[j].modTime)
	})

	// Take the most recent N runs
	if len(candidates) > n {
		candidates = candidates[:n]
	}

	var (
		floorVals     []float64
		hpRatios      []float64
		goldAtDeaths  []float64
		basicCards    []float64
		act2Count     int
	)

	for _, c := range candidates {
		reflections, err := loadAttemptReflectionsFromRunDir(c.dir)
		if err != nil || len(reflections) == 0 {
			continue
		}
		last := reflections[len(reflections)-1]
		if last.Floor != nil {
			floorVals = append(floorVals, float64(*last.Floor))
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

	m := RunMetrics{N: len(floorVals)}
	if m.N == 0 {
		return RunMetrics{}
	}

	m.AvgFloor = mean(floorVals)
	if len(hpRatios) > 0 {
		m.AvgHPRatio = mean(hpRatios)
	}
	if len(goldAtDeaths) > 0 {
		avgGold := mean(goldAtDeaths)
		m.GoldEfficiency = clamp01(1.0 - avgGold/200.0)
	}
	if len(basicCards) > 0 {
		m.DeckQuality = clamp01(1.0 - mean(basicCards)/10.0)
	}
	m.Act2Rate = float64(act2Count) / float64(m.N)

	// Compute composite score
	normFloor := clamp01(m.AvgFloor / 50.0)
	m.CompositeScore = wFloor*normFloor + wHP*m.AvgHPRatio + wGold*m.GoldEfficiency + wDeck*m.DeckQuality + wAct2*m.Act2Rate

	return m
}

// BuildEvolverBinary returns the path to the compiled spire2mind binary.
func BuildEvolverBinary(repoRoot string) (string, error) {
	// Find the binary in standard locations
	candidates := []string{
		filepath.Join(repoRoot, "bin", "spire2mind"),
		filepath.Join(repoRoot, "spire2mind"),
		"spire2mind",
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "spire2mind", nil // rely on PATH
}

// LLMAnalyzer wraps analyst.LLMProvider to implement LLMEvolvable.
type LLMAnalyzer struct {
	provider CompleteEvolutioner
}

type CompleteEvolutioner interface {
	Complete(ctx context.Context, systemPrompt string, userPrompt string) (string, error)
}

// CompleteEvolutionPrompt calls the LLM provider.
func (a *LLMAnalyzer) CompleteEvolutionPrompt(systemPrompt, userPrompt string) (string, error) {
	return a.provider.Complete(context.Background(), systemPrompt, userPrompt)
}
