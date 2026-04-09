package agentruntime

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"spire2mind/internal/i18n"
)

const (
	maxGuidebookRuns           = 24
	guidebookRecentHotspotRuns = 6
)

type GuidebookStore struct {
	root            string
	guidebookPath   string
	livingCodexPath string
	combatPlaybook  string
	eventPlaybook   string
	mutex           sync.Mutex
}

type GuidebookSnapshot struct {
	UpdatedAt                time.Time               `json:"updated_at"`
	RunsScanned              int                     `json:"runs_scanned"`
	ReflectionsScanned       int                     `json:"reflections_scanned"`
	RunQuality               RunQualitySummary       `json:"run_quality"`
	RLReadiness              RLReadiness             `json:"rl_readiness"`
	SeenContent              *SeenContentRegistry    `json:"seen_content,omitempty"`
	LessonBuckets            ReflectionLessonBuckets `json:"lesson_buckets,omitempty"`
	FailurePatterns          []string                `json:"failure_patterns,omitempty"`
	StorySeeds               []string                `json:"story_seeds,omitempty"`
	RecoveryHotspots         []RecoveryHotspot       `json:"recovery_hotspots,omitempty"`
	RecentRecoveryWindow     int                     `json:"recent_recovery_window,omitempty"`
	RecentRecoveryHotspots   []RecoveryHotspot       `json:"recent_recovery_hotspots,omitempty"`
	WeightedRecoveryHotspots []RecoveryHotspot       `json:"weighted_recovery_hotspots,omitempty"`
	RecentAttempts           []GuidebookAttempt      `json:"recent_attempts,omitempty"`
}

type GuidebookAttempt struct {
	Time        time.Time `json:"time"`
	Attempt     int       `json:"attempt"`
	RunID       string    `json:"run_id,omitempty"`
	Outcome     string    `json:"outcome,omitempty"`
	Floor       *int      `json:"floor,omitempty"`
	CharacterID string    `json:"character_id,omitempty"`
	Headline    string    `json:"headline,omitempty"`
	NextPlan    string    `json:"next_plan,omitempty"`
}

type RecoveryHotspot struct {
	RecoveryKind string  `json:"recovery_kind,omitempty"`
	DriftKind    string  `json:"drift_kind,omitempty"`
	Count        int     `json:"count,omitempty"`
	Score        float64 `json:"score,omitempty"`
}

func NewGuidebookStore(artifactsRoot string) (*GuidebookStore, error) {
	scratchRoot := filepath.Dir(filepath.Clean(artifactsRoot))
	root := filepath.Join(scratchRoot, "guidebook")
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}

	return &GuidebookStore{
		root:            root,
		guidebookPath:   filepath.Join(root, "guidebook.md"),
		livingCodexPath: filepath.Join(root, "living-codex.json"),
		combatPlaybook:  filepath.Join(root, "combat-playbook.md"),
		eventPlaybook:   filepath.Join(root, "event-playbook.md"),
	}, nil
}

func (g *GuidebookStore) GuidebookPath() string {
	if g == nil {
		return ""
	}
	return g.guidebookPath
}

func (g *GuidebookStore) LivingCodexPath() string {
	if g == nil {
		return ""
	}
	return g.livingCodexPath
}

func (g *GuidebookStore) CombatPlaybookPath() string {
	if g == nil {
		return ""
	}
	return g.combatPlaybook
}

func (g *GuidebookStore) EventPlaybookPath() string {
	if g == nil {
		return ""
	}
	return g.eventPlaybook
}

func (g *GuidebookStore) Refresh(artifactsRoot string, excludeDir string, language i18n.Language) (*GuidebookSnapshot, error) {
	if g == nil {
		return nil, nil
	}

	g.mutex.Lock()
	defer g.mutex.Unlock()

	snapshot, err := buildGuidebookSnapshot(artifactsRoot, excludeDir)
	if err != nil {
		return nil, err
	}

	bytes, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(g.livingCodexPath, bytes, 0o644); err != nil {
		return nil, err
	}

	if err := writeUTF8TextFile(g.guidebookPath, renderGuidebookSnapshot(snapshot, language)); err != nil {
		return nil, err
	}
	if err := writeUTF8TextFile(g.combatPlaybook, renderCombatPlaybookSnapshot(snapshot, language)); err != nil {
		return nil, err
	}
	if err := writeUTF8TextFile(g.eventPlaybook, renderEventPlaybookSnapshot(snapshot, language)); err != nil {
		return nil, err
	}

	return snapshot, nil
}

func buildGuidebookSnapshot(artifactsRoot string, excludeDir string) (*GuidebookSnapshot, error) {
	dirs, err := listRecentRunDirs(artifactsRoot, excludeDir, maxGuidebookRuns)
	if err != nil {
		return nil, err
	}

	catalog, err := loadCodexCatalogForArtifactsRoot(artifactsRoot)
	if err != nil {
		return nil, err
	}

	world := NewSeenContentTracker()
	evidence := newCodexEvidenceIndex()
	reflections := make([]*AttemptReflection, 0, 32)
	recoveryCounts := make(map[string]int)
	recentRecoveryCounts := make(map[string]int)
	weightedRecoveryCounts := make(map[string]float64)
	attemptQualities := make(map[string]attemptQuality)
	floor15PlusRuns := 0

	for index, dir := range dirs {
		registry, err := loadSeenContentFromRunDir(dir)
		if err != nil {
			return nil, err
		}
		world.Merge(registry)

		runReflections, err := loadAttemptReflectionsFromRunDir(dir)
		if err != nil {
			return nil, err
		}
		sort.SliceStable(runReflections, func(i, j int) bool {
			return runReflections[i].Time.After(runReflections[j].Time)
		})
		runReflections = dedupeGuidebookReflections(runReflections)
		reflections = append(reflections, runReflections...)
		evidence.absorbRun(registry, runReflections)
		for _, reflection := range runReflections {
			if reflection != nil && reflection.Floor != nil && *reflection.Floor >= 15 {
				floor15PlusRuns++
			}
		}

		runRecoveryCounts, err := loadRecoveryHotspotsFromRunDir(dir)
		if err != nil {
			return nil, err
		}
		runAttemptQualities, err := loadAttemptQualitiesFromRunDir(dir)
		if err != nil {
			return nil, err
		}
		for key, quality := range runAttemptQualities {
			attemptQualities[key] = quality
		}
		mergeIntCounts(recoveryCounts, runRecoveryCounts)
		if index < guidebookRecentHotspotRuns {
			mergeIntCounts(recentRecoveryCounts, runRecoveryCounts)
		}
		mergeWeightedCounts(weightedRecoveryCounts, runRecoveryCounts, recoveryRecencyWeight(index))
	}

	sort.SliceStable(reflections, func(i, j int) bool {
		return reflections[i].Time.After(reflections[j].Time)
	})
	reflections = dedupeGuidebookReflections(reflections)

	recentWindow := min(len(dirs), guidebookRecentHotspotRuns)
	lessonBuckets := aggregateGuideBuckets(reflections, nil)
	failurePatterns := collectGuideFailures(reflections)
	seenContent := world.Snapshot()
	enrichSeenContentRegistry(seenContent, catalog, lessonBuckets, failurePatterns, evidence)
	runQuality := buildRunQualitySummary(reflections, attemptQualities, recentWindow)
	snapshot := &GuidebookSnapshot{
		UpdatedAt:                time.Now(),
		RunsScanned:              len(dirs),
		ReflectionsScanned:       len(reflections),
		RunQuality:               runQuality,
		SeenContent:              seenContent,
		LessonBuckets:            lessonBuckets,
		FailurePatterns:          failurePatterns,
		StorySeeds:               collectGuideStorySeeds(reflections),
		RecoveryHotspots:         flattenRecoveryHotspots(recoveryCounts, 10),
		RecentRecoveryWindow:     recentWindow,
		RecentRecoveryHotspots:   flattenRecoveryHotspots(recentRecoveryCounts, 6),
		WeightedRecoveryHotspots: flattenWeightedRecoveryHotspots(weightedRecoveryCounts, 6),
		RecentAttempts:           buildGuidebookAttempts(reflections, 8),
	}
	snapshot.RLReadiness = buildRLReadiness(snapshot, floor15PlusRuns)

	return snapshot, nil
}

func renderGuidebookSnapshot(snapshot *GuidebookSnapshot, language i18n.Language) string {
	loc := i18n.New(language)
	lines := []string{
		"# " + loc.Label("Spire2Mind Guidebook", "Spire2Mind Guidebook"),
		"",
		loc.Paragraph(
			"This guidebook aggregates recent autonomous runs into a living codex, a recovery report, and practical lessons the agent can keep improving with.",
			"This guidebook aggregates recent autonomous runs into a living codex, a recovery report, and practical lessons the agent can keep improving with.",
		),
		"",
		"## " + loc.Label("Overview", "Overview"),
		"",
		fmt.Sprintf("- %s: `%s`", loc.Label("Updated", "Updated"), timestampString(snapshot.UpdatedAt)),
		fmt.Sprintf("- %s: `%d`", loc.Label("Runs scanned", "Runs scanned"), snapshot.RunsScanned),
		fmt.Sprintf("- %s: `%d`", loc.Label("Reflections scanned", "Reflections scanned"), snapshot.ReflectionsScanned),
	}

	lines = append(lines, "", "## "+loc.Label("RL Readiness", "RL Readiness"), "")
	lines = append(lines,
		fmt.Sprintf("- %s: `%t`", loc.Label("Ready", "Ready"), snapshot.RLReadiness.Ready),
		fmt.Sprintf("- %s: %s", loc.Label("Status", "Status"), valueOrDash(snapshot.RLReadiness.Status)),
		fmt.Sprintf("- %s: `%d / %d`", loc.Label("Complete runs", "Complete runs"), snapshot.RLReadiness.CompleteRuns, snapshot.RLReadiness.RequiredRuns),
		fmt.Sprintf("- %s: `%d / %d`", loc.Label("Floor >= 15 runs", "Floor >= 15 runs"), snapshot.RLReadiness.Floor15PlusRuns, snapshot.RLReadiness.RequiredFloor15),
		fmt.Sprintf("- %s: `%d / %d`", loc.Label("Provider-backed runs", "Provider-backed runs"), snapshot.RLReadiness.ProviderBackedRuns, snapshot.RLReadiness.RequiredProviderBackedRuns),
		fmt.Sprintf("- %s: `%d / %d`", loc.Label("Recent clean runs", "Recent clean runs"), snapshot.RLReadiness.RecentCleanRuns, snapshot.RLReadiness.RequiredRecentCleanRuns),
		fmt.Sprintf("- %s: `%t`", loc.Label("Stable runtime window", "Stable runtime window"), snapshot.RLReadiness.StableRuntime),
		fmt.Sprintf("- %s: `%t`", loc.Label("Knowledge assets ready", "Knowledge assets ready"), snapshot.RLReadiness.KnowledgeAssetsOK),
	)

	lines = append(lines, "", "### "+loc.Label("Run Data Quality", "Run Data Quality"), "")
	lines = append(lines,
		fmt.Sprintf("- %s: `%d`", loc.Label("Clean complete runs", "Clean complete runs"), snapshot.RunQuality.CleanCompleteRuns),
		fmt.Sprintf("- %s: `%d`", loc.Label("Recent complete runs", "Recent complete runs"), snapshot.RunQuality.RecentCompleteRuns),
		fmt.Sprintf("- %s: `%d`", loc.Label("Recent provider-backed runs", "Recent provider-backed runs"), snapshot.RunQuality.RecentProviderBackedRuns),
		fmt.Sprintf("- %s: `%d`", loc.Label("Recent fallback runs", "Recent fallback runs"), snapshot.RunQuality.RecentFallbackRuns),
		fmt.Sprintf("- %s: `%d`", loc.Label("Recent provider-retry runs", "Recent provider-retry runs"), snapshot.RunQuality.RecentProviderRetryRuns),
		fmt.Sprintf("- %s: `%d`", loc.Label("Recent tool-error runs", "Recent tool-error runs"), snapshot.RunQuality.RecentToolErrorRuns),
		fmt.Sprintf("- %s: `%d`", loc.Label("Recent median floor", "Recent median floor"), snapshot.RunQuality.RecentMedianFloor),
		fmt.Sprintf("- %s: `%d`", loc.Label("Recent best floor", "Recent best floor"), snapshot.RunQuality.RecentBestFloor),
		fmt.Sprintf("- %s: `%d / %d`", loc.Label("Recent floor >= 7 runs", "Recent floor >= 7 runs"), snapshot.RunQuality.RecentFloor7PlusRuns, snapshot.RunQuality.RecentCompleteRuns),
		fmt.Sprintf("- %s: `%d / %d`", loc.Label("Recent Act 2 entry runs", "Recent Act 2 entry runs"), snapshot.RunQuality.RecentAct2EntryRuns, snapshot.RunQuality.RecentCompleteRuns),
		fmt.Sprintf("- %s: `%d`", loc.Label("Recent died-with-gold runs", "Recent died-with-gold runs"), snapshot.RunQuality.RecentDiedWithGoldRuns),
		fmt.Sprintf("- %s: `%d`", loc.Label("Recent average death gold", "Recent average death gold"), snapshot.RunQuality.RecentAverageDeathGold),
	)

	lines = append(lines, "", "## "+loc.Label("Living Codex", "Living Codex"), "")
	lines = append(lines, seenContentCountLines(snapshot.SeenContent, language)...)
	lines = append(lines, "", "- "+loc.Label("Recent discoveries", "Recent discoveries")+":")
	for _, line := range seenContentDiscoveryLines(snapshot.SeenContent, language, 10) {
		lines = append(lines, "  "+line)
	}

	lines = append(lines, "", "## "+loc.Label("Stable Heuristics", "Stable Heuristics"), "")
	if snapshot.LessonBuckets.IsEmpty() {
		lines = append(lines, "- "+loc.Label("No stable heuristics yet.", "No stable heuristics yet."))
	} else {
		appendGuideBucketSection(&lines, loc.Label("Merged lessons", "Merged lessons"), snapshot.LessonBuckets)
	}

	lines = append(lines, "", "## "+loc.Label("Recovery Hotspots", "Recovery Hotspots"), "")
	lines = append(lines, fmt.Sprintf("- %s: `%d`", loc.Label("Recent window", "Recent window"), snapshot.RecentRecoveryWindow))
	if len(snapshot.RecentRecoveryHotspots) == 0 && len(snapshot.WeightedRecoveryHotspots) == 0 && len(snapshot.RecoveryHotspots) == 0 {
		lines = append(lines, "- "+loc.Label("No recovery hotspots recorded yet.", "No recovery hotspots recorded yet."))
	} else {
		lines = append(lines, "", "### "+loc.Label("Recent Recovery Hotspots", "Recent Recovery Hotspots"), "")
		lines = append(lines, renderRecoveryHotspotLines(snapshot.RecentRecoveryHotspots, false, loc)...)
		lines = append(lines, "", "### "+loc.Label("Recency-Weighted Recovery Trends", "Recency-Weighted Recovery Trends"), "")
		lines = append(lines, renderRecoveryHotspotLines(snapshot.WeightedRecoveryHotspots, true, loc)...)
		lines = append(lines, "", "### "+loc.Label("Historical Recovery Hotspots", "Historical Recovery Hotspots"), "")
		lines = append(lines, renderRecoveryHotspotLines(snapshot.RecoveryHotspots, false, loc)...)
	}
	if len(snapshot.RecentRecoveryHotspots) > 0 {
		lines = append(lines, "", loc.Paragraph(
			"Recent hotspots show what the latest runs are still tripping over; weighted trends keep historical context without letting old bugs dominate the signal.",
			"Recent hotspots show what the latest runs are still tripping over; weighted trends keep historical context without letting old bugs dominate the signal.",
		))
	}

	lines = append(lines, "", "## "+loc.Label("Failure Patterns", "Failure Patterns"), "")
	if len(snapshot.FailurePatterns) == 0 {
		lines = append(lines, "- "+loc.Label("No repeated failure pattern recorded yet.", "No repeated failure pattern recorded yet."))
	} else {
		for _, failure := range snapshot.FailurePatterns {
			lines = append(lines, "- "+failure)
		}
	}

	lines = append(lines, "", "## "+loc.Label("Recent Attempts", "Recent Attempts"), "")
	if len(snapshot.RecentAttempts) == 0 {
		lines = append(lines, "- "+loc.Label("No finished attempt summary yet.", "No finished attempt summary yet."))
	} else {
		for _, attempt := range snapshot.RecentAttempts {
			label := fmt.Sprintf("Attempt %d", attempt.Attempt)
			if attempt.RunID != "" {
				label += " / " + attempt.RunID
			}
			lines = append(lines, "- "+label)
			lines = append(lines, fmt.Sprintf("  - %s: %s", loc.Label("Outcome", "Outcome"), valueOrDash(attempt.Outcome)))
			if attempt.Floor != nil {
				lines = append(lines, fmt.Sprintf("  - %s: `%d`", loc.Label("Floor", "Floor"), *attempt.Floor))
			}
			if attempt.CharacterID != "" {
				lines = append(lines, fmt.Sprintf("  - %s: `%s`", loc.Label("Character", "Character"), attempt.CharacterID))
			}
			if attempt.Headline != "" {
				lines = append(lines, fmt.Sprintf("  - %s: %s", loc.Label("Headline", "Headline"), compactText(attempt.Headline, 180)))
			}
			if attempt.NextPlan != "" {
				lines = append(lines, fmt.Sprintf("  - %s: %s", loc.Label("Next plan", "Next plan"), compactText(attempt.NextPlan, 180)))
			}
		}
	}

	lines = append(lines, "", "## "+loc.Label("Story Seeds", "Story Seeds"), "")
	if len(snapshot.StorySeeds) == 0 {
		lines = append(lines, "- "+loc.Label("No story seeds yet.", "No story seeds yet."))
	} else {
		for _, seed := range snapshot.StorySeeds {
			lines = append(lines, "- "+seed)
		}
	}

	return strings.Join(lines, "\n")
}

func listRecentRunDirs(root string, excludeDir string, limit int) ([]string, error) {
	entries, err := os.ReadDir(root)
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

	candidates := make([]candidate, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dir := filepath.Join(root, entry.Name())
		if sameDir(dir, excludeDir) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		candidates = append(candidates, candidate{dir: dir, modTime: info.ModTime()})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].modTime.After(candidates[j].modTime)
	})

	if limit > 0 && len(candidates) > limit {
		candidates = candidates[:limit]
	}

	dirs := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		dirs = append(dirs, candidate.dir)
	}
	return dirs, nil
}

func loadSeenContentFromRunDir(dir string) (*SeenContentRegistry, error) {
	path := filepath.Join(dir, "seen-content.json")
	bytes, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var registry SeenContentRegistry
	if err := json.Unmarshal(bytes, &registry); err != nil {
		return nil, nil
	}
	return &registry, nil
}

func loadAttemptReflectionsFromRunDir(dir string) ([]*AttemptReflection, error) {
	path := filepath.Join(dir, "attempt-reflections.jsonl")
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	var reflections []*AttemptReflection
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var reflection AttemptReflection
		if err := json.Unmarshal([]byte(line), &reflection); err != nil {
			continue
		}
		copy := reflection
		reflections = append(reflections, &copy)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return reflections, nil
}

func mergeRecoveryHotspotsFromRunDir(dir string, counts map[string]int) error {
	before := sumRecoveryCounts(counts)
	if err := mergeRecoveryHotspots(filepath.Join(dir, "run-index.sqlite"), counts); err != nil {
		return err
	}
	if sumRecoveryCounts(counts) > before {
		return nil
	}
	return mergeRecoveryHotspotsFromEvents(filepath.Join(dir, "events.jsonl"), counts)
}

func loadRecoveryHotspotsFromRunDir(dir string) (map[string]int, error) {
	counts := make(map[string]int)
	if err := mergeRecoveryHotspotsFromRunDir(dir, counts); err != nil {
		return nil, err
	}
	return counts, nil
}

func mergeRecoveryHotspots(path string, counts map[string]int) error {
	if len(counts) == 0 {
		counts = make(map[string]int)
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return err
	}
	defer db.Close()

	rows, err := db.Query(`SELECT kind, COALESCE(drift_kind, ''), COUNT(*) FROM recovery_events GROUP BY kind, COALESCE(drift_kind, '')`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			kind      string
			driftKind string
			count     int
		)
		if err := rows.Scan(&kind, &driftKind, &count); err != nil {
			return err
		}
		key := strings.TrimSpace(kind)
		if key == "" && strings.TrimSpace(driftKind) == "" {
			continue
		}
		if strings.TrimSpace(driftKind) != "" {
			key += "|" + strings.TrimSpace(driftKind)
		}
		counts[key] += count
	}
	return rows.Err()
}

func mergeRecoveryHotspotsFromEvents(path string, counts map[string]int) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

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

		recoveryKind := firstNonEmptyString(stringData(event.Data["recovery_kind"]), stringData(event.Data["provider_recovery"]))
		if recoveryKind == "" {
			continue
		}

		key := recoveryKind
		driftKind := stringData(event.Data["drift_kind"])
		if driftKind != "" {
			key += "|" + driftKind
		}
		counts[key]++
	}

	return scanner.Err()
}

func flattenRecoveryHotspots(counts map[string]int, limit int) []RecoveryHotspot {
	if len(counts) == 0 {
		return nil
	}

	hotspots := make([]RecoveryHotspot, 0, len(counts))
	for key, count := range counts {
		recoveryKind := key
		driftKind := ""
		if split := strings.SplitN(key, "|", 2); len(split) == 2 {
			recoveryKind = split[0]
			driftKind = split[1]
		}
		recoveryKind = strings.TrimSpace(recoveryKind)
		driftKind = strings.TrimSpace(driftKind)
		if recoveryKind == "" && driftKind == "" {
			continue
		}
		hotspots = append(hotspots, RecoveryHotspot{
			RecoveryKind: recoveryKind,
			DriftKind:    driftKind,
			Count:        count,
		})
	}

	sort.SliceStable(hotspots, func(i, j int) bool {
		if hotspots[i].Count == hotspots[j].Count {
			if hotspots[i].RecoveryKind == hotspots[j].RecoveryKind {
				return hotspots[i].DriftKind < hotspots[j].DriftKind
			}
			return hotspots[i].RecoveryKind < hotspots[j].RecoveryKind
		}
		return hotspots[i].Count > hotspots[j].Count
	})

	if limit > 0 && len(hotspots) > limit {
		hotspots = hotspots[:limit]
	}
	return hotspots
}

func flattenWeightedRecoveryHotspots(counts map[string]float64, limit int) []RecoveryHotspot {
	if len(counts) == 0 {
		return nil
	}

	hotspots := make([]RecoveryHotspot, 0, len(counts))
	for key, score := range counts {
		if score <= 0 {
			continue
		}
		recoveryKind := key
		driftKind := ""
		if split := strings.SplitN(key, "|", 2); len(split) == 2 {
			recoveryKind = split[0]
			driftKind = split[1]
		}
		recoveryKind = strings.TrimSpace(recoveryKind)
		driftKind = strings.TrimSpace(driftKind)
		if recoveryKind == "" && driftKind == "" {
			continue
		}
		hotspots = append(hotspots, RecoveryHotspot{
			RecoveryKind: recoveryKind,
			DriftKind:    driftKind,
			Score:        score,
		})
	}

	sort.SliceStable(hotspots, func(i, j int) bool {
		if hotspots[i].Score == hotspots[j].Score {
			if hotspots[i].RecoveryKind == hotspots[j].RecoveryKind {
				return hotspots[i].DriftKind < hotspots[j].DriftKind
			}
			return hotspots[i].RecoveryKind < hotspots[j].RecoveryKind
		}
		return hotspots[i].Score > hotspots[j].Score
	})

	if limit > 0 && len(hotspots) > limit {
		hotspots = hotspots[:limit]
	}
	return hotspots
}

func mergeIntCounts(target map[string]int, source map[string]int) {
	for key, count := range source {
		target[key] += count
	}
}

func mergeWeightedCounts(target map[string]float64, source map[string]int, weight float64) {
	if weight <= 0 {
		return
	}
	for key, count := range source {
		target[key] += float64(count) * weight
	}
}

func recoveryRecencyWeight(index int) float64 {
	if index < 0 {
		return 0
	}
	return 1.0 / float64(index+1)
}

func renderRecoveryHotspotLines(hotspots []RecoveryHotspot, weighted bool, loc i18n.Localizer) []string {
	if len(hotspots) == 0 {
		return []string{"- " + loc.Label("No recovery hotspots recorded yet.", "No recovery hotspots recorded yet.")}
	}

	lines := make([]string, 0, len(hotspots))
	for _, hotspot := range hotspots {
		label := recoveryHotspotLabel(hotspot)
		if weighted {
			lines = append(lines, fmt.Sprintf("- %s: `%.2f`", valueOrDash(label), hotspot.Score))
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s: `%d`", valueOrDash(label), hotspot.Count))
	}
	return lines
}

func recoveryHotspotLabel(hotspot RecoveryHotspot) string {
	label := strings.TrimSpace(hotspot.RecoveryKind)
	if label == "" {
		label = strings.TrimSpace(hotspot.DriftKind)
	} else if strings.TrimSpace(hotspot.DriftKind) != "" {
		label += " / " + hotspot.DriftKind
	}
	return label
}

func sumRecoveryCounts(counts map[string]int) int {
	total := 0
	for _, count := range counts {
		total += count
	}
	return total
}

func buildGuidebookAttempts(reflections []*AttemptReflection, limit int) []GuidebookAttempt {
	if limit <= 0 {
		limit = 8
	}

	attempts := make([]GuidebookAttempt, 0, limit)
	for _, reflection := range reflections {
		if reflection == nil {
			continue
		}
		attempts = append(attempts, GuidebookAttempt{
			Time:        reflection.Time,
			Attempt:     reflection.Attempt,
			RunID:       strings.TrimSpace(reflection.RunID),
			Outcome:     strings.TrimSpace(reflection.Outcome),
			Floor:       cloneOptionalInt(reflection.Floor),
			CharacterID: strings.TrimSpace(reflection.CharacterID),
			Headline:    strings.TrimSpace(reflection.Headline),
			NextPlan:    strings.TrimSpace(reflection.NextPlan),
		})
		if len(attempts) >= limit {
			break
		}
	}
	return attempts
}

func dedupeGuidebookReflections(reflections []*AttemptReflection) []*AttemptReflection {
	if len(reflections) == 0 {
		return nil
	}

	deduped := make([]*AttemptReflection, 0, len(reflections))
	seen := make(map[string]struct{}, len(reflections))
	for _, reflection := range reflections {
		if reflection == nil {
			continue
		}
		key := guidebookReflectionIdentity(reflection)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, reflection)
	}
	return deduped
}

func guidebookReflectionIdentity(reflection *AttemptReflection) string {
	if reflection == nil {
		return ""
	}

	if runID := strings.TrimSpace(reflection.RunID); runID != "" {
		return "run:" + runID
	}

	floor := "-"
	if reflection.Floor != nil {
		floor = fmt.Sprintf("%d", *reflection.Floor)
	}

	runID := compactText(strings.TrimSpace(reflection.Story), 80)

	return strings.Join([]string{
		runID,
		strings.TrimSpace(reflection.Outcome),
		floor,
		strings.TrimSpace(reflection.Headline),
		strings.TrimSpace(reflection.NextPlan),
	}, "|")
}

func renderCombatPlaybookSnapshot(snapshot *GuidebookSnapshot, language i18n.Language) string {
	loc := i18n.New(language)
	lines := []string{
		"# " + loc.Label("Combat Playbook", "战斗作战手册"),
		"",
		loc.Paragraph(
			"This playbook distills combat-facing lessons from recent autonomous runs. It is meant to bias tactical planning, target selection, and low-health discipline.",
			"这份手册从最近的自主 runs 里提炼战斗侧经验，用来约束战术规划、目标选择和低血量时的保命纪律。",
		),
		"",
		"## " + loc.Label("Overview", "概览"),
		"",
		fmt.Sprintf("- %s: `%s`", loc.Label("Updated", "更新时间"), timestampString(snapshot.UpdatedAt)),
		fmt.Sprintf("- %s: `%d`", loc.Label("Runs scanned", "扫描 runs"), snapshot.RunsScanned),
		fmt.Sprintf("- %s: `%d`", loc.Label("Reflections scanned", "扫描反思"), snapshot.ReflectionsScanned),
	}

	lines = append(lines, "", "## "+loc.Label("Combat Priorities", "战斗优先级"), "")
	combatLessons := bucketsFromSections(
		loc,
		[][2]interface{}{
			{loc.Label("Combat survival", "生存"), snapshot.LessonBuckets.CombatSurvival},
			{loc.Label("Reward choice", "奖励选择"), snapshot.LessonBuckets.RewardChoice},
		},
	)
	if len(combatLessons) == 0 {
		lines = append(lines, "- "+loc.Label("No combat heuristics aggregated yet.", "还没有稳定沉淀出的战斗启发。"))
	} else {
		lines = append(lines, combatLessons...)
	}

	lines = append(lines, "", "## "+loc.Label("Observed Monsters", "已观测怪物"), "")
	lines = append(lines, renderSeenEntries(topSeenEntries(snapshot.SeenContent, seenCategoryMonsters, 12), loc)...)

	lines = append(lines, "", "## "+loc.Label("Observed Combat Tools", "已观测战斗资源"), "")
	toolEntries := append(topSeenEntries(snapshot.SeenContent, seenCategoryCards, 8), topSeenEntries(snapshot.SeenContent, seenCategoryRelics, 6)...)
	lines = append(lines, renderSeenEntries(toolEntries, loc)...)

	lines = append(lines, "", "## "+loc.Label("Combat Failure Patterns", "战斗失败模式"), "")
	combatFailures := filterGuideLines(snapshot.FailurePatterns, []string{"hp", "elite", "damage", "block", "survival", "fight", "combat", "gold"})
	if len(combatFailures) == 0 {
		combatFailures = snapshot.FailurePatterns
	}
	lines = append(lines, renderPlainList(combatFailures, loc.Label("No combat failure pattern recorded yet.", "还没有明确的战斗失败模式。"))...)

	lines = append(lines, "", "## "+loc.Label("Combat Seam Watchlist", "战斗接缝观察"), "")
	lines = append(lines, renderRecoveryHotspotLines(filterRecoveryHotspots(snapshot.WeightedRecoveryHotspots,
		driftKindSameScreenIndexDrift,
		driftKindActionWindowChanged,
		driftKindSameScreenStateDrift,
	), true, loc)...)

	lines = append(lines, "", "## "+loc.Label("Recent Story Seeds", "近期故事种子"), "")
	lines = append(lines, renderPlainList(snapshot.StorySeeds, loc.Label("No combat story seed yet.", "还没有战斗故事种子。"))...)

	return strings.Join(lines, "\n")
}

func renderEventPlaybookSnapshot(snapshot *GuidebookSnapshot, language i18n.Language) string {
	loc := i18n.New(language)
	lines := []string{
		"# " + loc.Label("Event Playbook", "事件决策手册"),
		"",
		loc.Paragraph(
			"This playbook distills room-event decisions from recent autonomous runs. It tracks seen events, recurring event seams, and the heuristics that should guide event choices.",
			"这份手册从最近的自主 runs 里提炼事件房决策经验，记录已见事件、事件链路里的 seam，以及应该约束事件选择的启发。",
		),
		"",
		"## " + loc.Label("Overview", "概览"),
		"",
		fmt.Sprintf("- %s: `%s`", loc.Label("Updated", "更新时间"), timestampString(snapshot.UpdatedAt)),
		fmt.Sprintf("- %s: `%d`", loc.Label("Runs scanned", "扫描 runs"), snapshot.RunsScanned),
		fmt.Sprintf("- %s: `%d`", loc.Label("Reflections scanned", "扫描反思"), snapshot.ReflectionsScanned),
	}

	lines = append(lines, "", "## "+loc.Label("Event Decision Principles", "事件决策原则"), "")
	eventLessons := bucketsFromSections(
		loc,
		[][2]interface{}{
			{loc.Label("Pathing", "路线"), snapshot.LessonBuckets.Pathing},
			{loc.Label("Reward choice", "奖励选择"), snapshot.LessonBuckets.RewardChoice},
			{loc.Label("Shop economy", "商店经济"), snapshot.LessonBuckets.ShopEconomy},
		},
	)
	if len(eventLessons) == 0 {
		lines = append(lines, "- "+loc.Label("No event heuristics aggregated yet.", "还没有稳定沉淀出的事件启发。"))
	} else {
		lines = append(lines, eventLessons...)
	}

	lines = append(lines, "", "## "+loc.Label("Observed Events", "已观测事件"), "")
	lines = append(lines, renderSeenEntries(topSeenEntries(snapshot.SeenContent, seenCategoryEvents, 12), loc)...)

	lines = append(lines, "", "## "+loc.Label("Event Seam Watchlist", "事件接缝观察"), "")
	lines = append(lines, renderRecoveryHotspotLines(filterRecoveryHotspots(snapshot.WeightedRecoveryHotspots,
		driftKindSelectionSeam,
		driftKindRewardTransition,
		driftKindScreenTransition,
	), true, loc)...)

	lines = append(lines, "", "## "+loc.Label("Event Risk Patterns", "事件风险模式"), "")
	eventFailures := filterGuideLines(snapshot.FailurePatterns, []string{"gold", "route", "path", "event", "hp", "safe"})
	if len(eventFailures) == 0 {
		eventFailures = snapshot.FailurePatterns
	}
	lines = append(lines, renderPlainList(eventFailures, loc.Label("No event failure pattern recorded yet.", "还没有明确的事件失败模式。"))...)

	lines = append(lines, "", "## "+loc.Label("Recent Event Story Seeds", "近期事件故事种子"), "")
	lines = append(lines, renderPlainList(snapshot.StorySeeds, loc.Label("No event story seed yet.", "还没有事件故事种子。"))...)

	return strings.Join(lines, "\n")
}

func topSeenEntries(registry *SeenContentRegistry, category string, limit int) []SeenContentEntry {
	if registry == nil {
		return nil
	}

	var entries []SeenContentEntry
	switch category {
	case seenCategoryCards:
		entries = append(entries, registry.Cards...)
	case seenCategoryRelics:
		entries = append(entries, registry.Relics...)
	case seenCategoryPotions:
		entries = append(entries, registry.Potions...)
	case seenCategoryMonsters:
		entries = append(entries, registry.Monsters...)
	case seenCategoryEvents:
		entries = append(entries, registry.Events...)
	case seenCategoryCharacters:
		entries = append(entries, registry.Characters...)
	default:
		return nil
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].SeenCount == entries[j].SeenCount {
			if entries[i].LastSeenAt.Equal(entries[j].LastSeenAt) {
				return bestSeenContentName(entries[i]) < bestSeenContentName(entries[j])
			}
			return entries[i].LastSeenAt.After(entries[j].LastSeenAt)
		}
		return entries[i].SeenCount > entries[j].SeenCount
	})

	if limit > 0 && len(entries) > limit {
		return append([]SeenContentEntry(nil), entries[:limit]...)
	}
	return entries
}

func renderSeenEntries(entries []SeenContentEntry, loc i18n.Localizer) []string {
	if len(entries) == 0 {
		return []string{"- " + loc.Label("No observed entries yet.", "还没有观测记录。")}
	}

	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		label := fmt.Sprintf("%s: %s", seenContentLabel(loc, entry.Category), valueOrDash(bestSeenContentName(entry)))
		detail := fmt.Sprintf("%s `%d`", loc.Label("seen", "出现"), max(entry.SeenCount, 1))
		if entry.LastFloor != nil {
			detail += fmt.Sprintf(", %s `%d`", loc.Label("floor", "层数"), *entry.LastFloor)
		}
		lines = append(lines, fmt.Sprintf("- %s (%s)", label, detail))
	}
	return lines
}

func renderPlainList(items []string, empty string) []string {
	items = dedupeStoryLines(items)
	if len(items) == 0 {
		return []string{"- " + empty}
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		lines = append(lines, "- "+item)
	}
	return lines
}

func filterGuideLines(items []string, keywords []string) []string {
	if len(items) == 0 || len(keywords) == 0 {
		return nil
	}

	filtered := make([]string, 0, len(items))
	for _, item := range items {
		lower := strings.ToLower(strings.TrimSpace(item))
		if lower == "" {
			continue
		}
		for _, keyword := range keywords {
			if strings.Contains(lower, keyword) {
				filtered = append(filtered, item)
				break
			}
		}
	}
	return dedupeStoryLines(filtered)
}

func filterRecoveryHotspots(hotspots []RecoveryHotspot, driftKinds ...string) []RecoveryHotspot {
	if len(hotspots) == 0 || len(driftKinds) == 0 {
		return nil
	}
	allowed := make(map[string]struct{}, len(driftKinds))
	for _, driftKind := range driftKinds {
		driftKind = strings.TrimSpace(driftKind)
		if driftKind == "" {
			continue
		}
		allowed[driftKind] = struct{}{}
	}

	filtered := make([]RecoveryHotspot, 0, len(hotspots))
	for _, hotspot := range hotspots {
		if _, ok := allowed[strings.TrimSpace(hotspot.DriftKind)]; ok {
			filtered = append(filtered, hotspot)
		}
	}
	return filtered
}

func bucketsFromSections(loc i18n.Localizer, sections [][2]interface{}) []string {
	lines := make([]string, 0, len(sections))
	for _, section := range sections {
		title, _ := section[0].(string)
		lessons, _ := section[1].([]string)
		lessons = dedupeStoryLines(lessons)
		if len(lessons) == 0 {
			continue
		}
		lines = append(lines, "- "+title+":")
		for _, lesson := range lessons {
			lines = append(lines, "  - "+lesson)
		}
	}
	if len(lines) == 0 {
		return []string{"- " + loc.Label("No stable heuristics yet.", "还没有稳定启发。")}
	}
	return lines
}
