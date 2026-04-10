package agentruntime

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"spire2mind/internal/game"
	"spire2mind/internal/i18n"
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

type RunStore struct {
	sessionID       string
	root            string
	eventsPath      string
	statePath       string
	sessionPath     string
	reflectionsPath string
	seenContentPath string
	indexPath       string
	dashboardPath   string
	storyPath       string
	guidePath       string
	index           *runIndex
	mutex           sync.Mutex
}

func NewRunStore(root string, sessionID string) (*RunStore, error) {
	timestamp := time.Now().Format("20060102-150405")
	runDir := filepath.Join(root, fmt.Sprintf("%s-%s", timestamp, sessionID))
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return nil, err
	}

	indexPath := filepath.Join(runDir, "run-index.sqlite")
	index, err := openRunIndex(indexPath, sessionID, runDir)
	if err != nil {
		return nil, err
	}

	return &RunStore{
		sessionID:       sessionID,
		root:            runDir,
		eventsPath:      filepath.Join(runDir, "events.jsonl"),
		statePath:       filepath.Join(runDir, "state-latest.json"),
		sessionPath:     filepath.Join(runDir, "session-latest.json"),
		reflectionsPath: filepath.Join(runDir, "attempt-reflections.jsonl"),
		seenContentPath: filepath.Join(runDir, "seen-content.json"),
		indexPath:       indexPath,
		dashboardPath:   filepath.Join(runDir, "dashboard.md"),
		storyPath:       filepath.Join(runDir, "run-story.md"),
		guidePath:       filepath.Join(runDir, "run-guide.md"),
		index:           index,
	}, nil
}

func (s *RunStore) AppendEvent(event SessionEvent) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	line, err := json.Marshal(event)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(s.eventsPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := file.Write(append(line, '\n')); err != nil {
		return err
	}

	if s.index != nil {
		if err := s.index.RecordEvent(event); err != nil {
			return err
		}
	}

	return nil
}

func (s *RunStore) WriteLatestState(state *game.StateSnapshot) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	state = game.NormalizeStateSnapshot(state)
	bytes, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.statePath, bytes, 0o644)
}

func (s *RunStore) WriteSessionResume(resume *SessionResumeState) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	bytes, err := json.MarshalIndent(resume, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.sessionPath, bytes, 0o644)
}

func (s *RunStore) WriteSeenContent(registry *SeenContentRegistry) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	bytes, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(s.seenContentPath, bytes, 0o644); err != nil {
		return err
	}
	if s.index != nil {
		if err := s.index.ReplaceSeenContent(registry); err != nil {
			return err
		}
	}
	return nil
}

func (s *RunStore) RecordPrompt(cycle int, prompt string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	path := filepath.Join(s.root, fmt.Sprintf("cycle-%04d-prompt.txt", cycle))
	return writeUTF8TextFile(path, i18n.RepairText(prompt))
}

func (s *RunStore) RecordCycleSummary(cycle int, summary *CycleSummary) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	bytes, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return err
	}

	path := filepath.Join(s.root, fmt.Sprintf("cycle-%04d-summary.json", cycle))
	if err := os.WriteFile(path, bytes, 0o644); err != nil {
		return err
	}
	if s.index != nil {
		if err := s.index.RecordCycleSummary(cycle, summary); err != nil {
			return err
		}
	}
	return nil
}

func (s *RunStore) RecordAttemptReflection(reflection *AttemptReflection, language i18n.Language) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	line, err := json.Marshal(reflection)
	if err != nil {
		return err
	}

	file, err := os.OpenFile(s.reflectionsPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()

	if _, err := file.Write(append(line, '\n')); err != nil {
		return err
	}

	bytes, err := json.MarshalIndent(reflection, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(s.root, fmt.Sprintf("attempt-%04d-reflection.json", reflection.Attempt)), bytes, 0o644); err != nil {
		return err
	}

	if s.index != nil {
		if err := s.index.RecordReflection(reflection); err != nil {
			return err
		}
	}

	story := buildReflectionMarkdown(reflection, language)
	return writeUTF8TextFile(filepath.Join(s.root, fmt.Sprintf("attempt-%04d-story.md", reflection.Attempt)), story)
}

func (s *RunStore) WriteDashboard(markdown string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return writeUTF8TextFile(s.dashboardPath, i18n.RepairText(markdown))
}

func (s *RunStore) WriteRunStory(markdown string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return writeUTF8TextFile(s.storyPath, i18n.RepairText(markdown))
}

func (s *RunStore) WriteRunGuide(markdown string) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	return writeUTF8TextFile(s.guidePath, i18n.RepairText(markdown))
}

func (s *RunStore) Root() string {
	return s.root
}

func (s *RunStore) IndexPath() string {
	return s.indexPath
}

func (s *RunStore) Close() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	if s.index == nil {
		return nil
	}
	err := s.index.Close()
	s.index = nil
	return err
}

func buildReflectionMarkdown(reflection *AttemptReflection, language i18n.Language) string {
	if reflection == nil {
		return ""
	}
	loc := i18n.New(language)

	lines := []string{
		fmt.Sprintf("# %s %d", loc.Label("Attempt Reflection", "尝试反思"), reflection.Attempt),
		"",
		fmt.Sprintf("- %s: `%s`", loc.Label("Outcome", "结果"), reflection.Outcome),
		fmt.Sprintf("- %s: `%s`", loc.Label("Run ID", "Run 标识"), reflection.RunID),
		fmt.Sprintf("- %s: `%s`", loc.Label("Screen", "界面"), reflection.Screen),
	}
	if reflection.Floor != nil {
		lines = append(lines, fmt.Sprintf("- %s: `%d`", loc.Label("Floor", "层数"), *reflection.Floor))
	}
	if reflection.CharacterID != "" {
		lines = append(lines, fmt.Sprintf("- %s: `%s`", loc.Label("Character", "角色"), reflection.CharacterID))
	}
	if reflection.Headline != "" {
		lines = append(lines, fmt.Sprintf("- %s: %s", loc.Label("Headline", "摘要"), reflection.Headline))
	}
	if len(reflection.TacticalHints) > 0 {
		lines = append(lines, "", "## "+loc.Label("Tactical Lens", "战术镜头"), "")
		for _, hint := range reflection.TacticalHints {
			lines = append(lines, "- "+hint)
		}
	}
	if len(reflection.FinalRoomDetail) > 0 {
		lines = append(lines, "", "## "+loc.Label("Final Room Detail", "最终房间细节"), "")
		for _, line := range cleanReflectionLines(reflection.FinalRoomDetail) {
			lines = append(lines, "- "+line)
		}
	}

	lines = append(lines, "", "## "+loc.Label("Story", "故事"), "", reflection.Story)

	if len(reflection.RecentFailures) > 0 {
		lines = append(lines, "", "## "+loc.Label("Friction", "摩擦点"), "")
		for _, failure := range reflection.RecentFailures {
			lines = append(lines, "- "+failure)
		}
	}

	if len(reflection.Successes) > 0 {
		lines = append(lines, "", "## "+loc.Label("What Worked", "有效之处"), "")
		for _, success := range reflection.Successes {
			lines = append(lines, "- "+success)
		}
	}

	if len(reflection.Risks) > 0 {
		lines = append(lines, "", "## "+loc.Label("What Hurt", "受损之处"), "")
		for _, risk := range reflection.Risks {
			lines = append(lines, "- "+risk)
		}
	}

	if len(reflection.Lessons) > 0 {
		lines = append(lines, "", "## "+loc.Label("Lessons", "经验"), "")
		for _, lesson := range reflection.Lessons {
			lines = append(lines, "- "+lesson)
		}
	}

	appendLessonGroup := func(title string, lessons []string) {
		if len(lessons) == 0 {
			return
		}
		lines = append(lines, "", "## "+title, "")
		for _, lesson := range lessons {
			lines = append(lines, "- "+lesson)
		}
	}
	appendLessonGroup(loc.Label("Pathing Lessons", "路线经验"), reflection.LessonBuckets.Pathing)
	appendLessonGroup(loc.Label("Reward Lessons", "奖励经验"), reflection.LessonBuckets.RewardChoice)
	appendLessonGroup(loc.Label("Shop Lessons", "商店经验"), reflection.LessonBuckets.ShopEconomy)
	appendLessonGroup(loc.Label("Combat Lessons", "战斗经验"), reflection.LessonBuckets.CombatSurvival)
	appendLessonGroup(loc.Label("Runtime Lessons", "运行时经验"), reflection.LessonBuckets.Runtime)

	appendLessonGroup(loc.Label("Tactical Mistakes", "战术失误"), reflection.TacticalMistakes)
	appendLessonGroup(loc.Label("Runtime Noise", "运行时噪声"), reflection.RuntimeNoise)
	appendLessonGroup(loc.Label("Resource Mistakes", "资源失误"), reflection.ResourceMistakes)

	if reflection.NextPlan != "" {
		lines = append(lines, "", "## "+loc.Label("Next Plan", "下一步计划"), "", reflection.NextPlan)
	}

	return strings.Join(lines, "\n")
}

func writeUTF8TextFile(path string, content string) error {
	bytes := make([]byte, 0, len(utf8BOM)+len(content))
	bytes = append(bytes, utf8BOM...)
	bytes = append(bytes, content...)
	return os.WriteFile(path, bytes, 0o644)
}

func LoadRecentAttemptReflections(root string, excludeDir string, limit int) ([]*AttemptReflection, error) {
	if limit <= 0 {
		return nil, nil
	}

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

	var candidates []candidate
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

	var loaded []*AttemptReflection
	for _, candidate := range candidates {
		path := filepath.Join(candidate.dir, "attempt-reflections.jsonl")
		file, err := os.Open(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			if len(loaded) >= limit {
				break
			}
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			var reflection AttemptReflection
			if err := json.Unmarshal([]byte(line), &reflection); err != nil {
				continue
			}
			copy := reflection
			loaded = append(loaded, &copy)
		}
		closeErr := file.Close()
		if err := scanner.Err(); err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, closeErr
		}
		if len(loaded) >= limit {
			break
		}
	}

	sort.SliceStable(loaded, func(i, j int) bool {
		return loaded[i].Time.Before(loaded[j].Time)
	})

	return loaded, nil
}

func sameDir(left string, right string) bool {
	if strings.TrimSpace(left) == "" || strings.TrimSpace(right) == "" {
		return false
	}

	left = filepath.Clean(left)
	right = filepath.Clean(right)
	return strings.EqualFold(left, right)
}

func LoadLatestSessionResume(root string, excludeDir string) (*SessionResumeState, error) {
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

	var candidates []candidate
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

	for _, candidate := range candidates {
		path := filepath.Join(candidate.dir, "session-latest.json")
		bytes, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		var resume SessionResumeState
		if err := json.Unmarshal(bytes, &resume); err != nil {
			continue
		}
		return &resume, nil
	}

	return nil, nil
}

func LoadRecentSeenContent(root string, excludeDir string, limit int) (*SeenContentRegistry, error) {
	if limit <= 0 {
		return nil, nil
	}

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

	var candidates []candidate
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

	tracker := NewSeenContentTracker()
	loaded := 0
	for _, candidate := range candidates {
		if loaded >= limit {
			break
		}

		path := filepath.Join(candidate.dir, "seen-content.json")
		bytes, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}

		var registry SeenContentRegistry
		if err := json.Unmarshal(bytes, &registry); err != nil {
			continue
		}

		tracker.Merge(&registry)
		loaded++
	}

	return tracker.Snapshot(), nil
}
