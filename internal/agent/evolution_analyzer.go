package agentruntime

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// GuidebookAnalysis contains identified weaknesses in the current guidebook.
type GuidebookAnalysis struct {
	Weaknesses   []GuidebookWeakness
	Priorities   []string
	RunSummary   RunSummary
}

// RunSummary summarizes recent runs for the LLM analyzer.
type RunSummary struct {
	RecentFloors        []int
	MedianFloor         float64
	BestFloor           int
	AvgFloor            float64
	RecentPatterns      []string
	DeathPatterns       []string
	RepeatedMistakes    []string
	GuidebookWeaknesses []string
}

// GuidebookWeakness describes a specific area the guidebook needs improvement.
type GuidebookWeakness struct {
	Category    string // "combat" | "pathing" | "shop" | "reward" | "runtime"
	Severity    string // "critical" | "major" | "minor"
	Description string
	Evidence    []string
	Suggestion  string
	TargetFile  string // which guidebook file to edit
}

// AnalyzeGuidebook loads recent reflections and guidebook state,
// identifies weaknesses, and returns specific edit proposals.
func AnalyzeGuidebook(artifactsRoot, guidebookDir string, recentN int) (*GuidebookAnalysis, error) {
	analysis := &GuidebookAnalysis{}

	// Load recent reflections
	reflections, err := LoadRecentAttemptReflections(artifactsRoot, "", recentN)
	if err != nil {
		return nil, err
	}

	// Compute run summary
	analysis.RunSummary = computeRunSummary(reflections)

	// Identify guidebook weaknesses from reflection data
	analysis.Weaknesses = identifyGuidebookWeaknesses(reflections, &analysis.RunSummary)
	analysis.Priorities = prioritizeWeaknesses(analysis.Weaknesses)

	return analysis, nil
}

// identifyGuidebookWeaknesses scans reflection data for patterns that
// the guidebook should have caught but didn't.
func identifyGuidebookWeaknesses(reflections []*AttemptReflection, summary *RunSummary) []GuidebookWeakness {
	var weaknesses []GuidebookWeakness

	// Track pattern frequencies
	hpDeaths := 0
	goldWastes := 0
	earlyDeaths := 0
	lowDefensives := 0
	noScalings := 0

	var deathFloors []int
	var deathPatterns []string
	var repeatedMistakes []string

	mistakeCount := make(map[string]int)

	for _, r := range reflections {
		if r.Outcome != "defeat" {
			continue
		}
		floor := 0
		if r.Floor != nil {
			floor = *r.Floor
			deathFloors = append(deathFloors, floor)
		}

		// Count pattern types from lessons
		for _, lesson := range r.Lessons {
			lower := strings.ToLower(lesson)
			switch {
			case strings.Contains(lower, "hp"), strings.Contains(lower, "health"), strings.Contains(lower, "critical"):
				hpDeaths++
				mistakeCount["hp_death"]++
			case strings.Contains(lower, "gold"), strings.Contains(lower, "shop"), strings.Contains(lower, "unspent"):
				goldWastes++
				mistakeCount["gold_waste"]++
			case floor <= 7:
				earlyDeaths++
				mistakeCount["early_death"]++
			case strings.Contains(lower, "defense"), strings.Contains(lower, "block"), strings.Contains(lower, "defensive"):
				lowDefensives++
				mistakeCount["low_defense"]++
			case strings.Contains(lower, "scale"), strings.Contains(lower, "power"):
				noScalings++
				mistakeCount["no_scaling"]++
			}
		}

		// Collect repeated mistakes from tactical mistakes
		for _, tm := range r.TacticalMistakes {
			if tm == "" {
				continue
			}
			mistakeCount[tm]++
			if mistakeCount[tm] >= 2 && !contains(repeatedMistakes, tm) {
				repeatedMistakes = append(repeatedMistakes, tm)
			}
		}

		// Collect death patterns
		if len(r.Risks) > 0 {
			for _, risk := range r.Risks {
				if risk != "" {
					deathPatterns = append(deathPatterns, risk)
				}
			}
		}
	}

	summary.RecentPatterns = deathPatterns
	summary.DeathPatterns = deathPatterns
	summary.RepeatedMistakes = repeatedMistakes

	// HP deaths pattern
	if hpDeaths >= 2 {
		weaknesses = append(weaknesses, GuidebookWeakness{
			Category:    "combat",
			Severity:    "major",
			Description: fmt.Sprintf("%d recent deaths involve low HP situations — the combat playbook needs stronger survival discipline", hpDeaths),
			Evidence:    []string{fmt.Sprintf("%d HP-related deaths in recent %d runs", hpDeaths, len(reflections))},
			Suggestion:  "Add a specific rule: when HP < 40%, prefer block over damage; refuse to race unless you have lethal.",
			TargetFile:  "combat-playbook.md",
		})
	}

	// Gold waste pattern
	if goldWastes >= 2 {
		weaknesses = append(weaknesses, GuidebookWeakness{
			Category:    "shop",
			Severity:    "major",
			Description: fmt.Sprintf("%d recent deaths left significant gold unspent — shop economy rules are insufficient", goldWastes),
			Evidence:    []string{fmt.Sprintf("%d gold-waste incidents in recent %d runs", goldWastes, len(reflections))},
			Suggestion:  "Add explicit shop priority rules: spend 50%+ gold on card removal before floor 10; always remove basic Strikes.",
			TargetFile:  "guidebook.md",
		})
	}

	// Early death pattern
	if earlyDeaths >= 2 {
		weaknesses = append(weaknesses, GuidebookWeakness{
			Category:    "reward",
			Severity:    "critical",
			Description: fmt.Sprintf("%d deaths before floor 7 — early card picks and pathing are too aggressive", earlyDeaths),
			Evidence:    []string{fmt.Sprintf("%d early deaths in recent %d runs", earlyDeaths, len(reflections))},
			Suggestion:  "Add early-game rules: prioritize damage+block cards in first 3 floors; avoid elite risky nodes until floor 4+.",
			TargetFile:  "guidebook.md",
		})
	}

	// Low defense pattern
	if lowDefensives >= 2 {
		weaknesses = append(weaknesses, GuidebookWeakness{
			Category:    "combat",
			Severity:    "major",
			Description: fmt.Sprintf("Defense is consistently underprioritized — the deck skews too attack-heavy", lowDefensives),
			Evidence:    []string{fmt.Sprintf("%d low-defense incidents in recent %d runs", lowDefensives, len(reflections))},
			Suggestion:  "Add a deck composition rule: maintain at least 30% defense cards; audit deck ratio at floors 5, 10, 15.",
			TargetFile:  "combat-playbook.md",
		})
	}

	// No scaling pattern
	if noScalings >= 2 {
		weaknesses = append(weaknesses, GuidebookWeakness{
			Category:    "reward",
			Severity:    "minor",
			Description: fmt.Sprintf("Scaling cards not picked early enough — midgame power gap", noScalings),
			Evidence:    []string{fmt.Sprintf("%d no-scaling incidents in recent %d runs", noScalings, len(reflections))},
			Suggestion:  "Add a floor-10 checkpoint: if no scaling cards picked by floor 10, prioritize one at next reward/shop.",
			TargetFile:  "guidebook.md",
		})
	}

	return weaknesses
}

func computeRunSummary(reflections []*AttemptReflection) RunSummary {
	var floors []int
	bestFloor := 0
	sum := 0

	for _, r := range reflections {
		if r.Floor != nil {
			f := *r.Floor
			floors = append(floors, f)
			sum += f
			if f > bestFloor {
				bestFloor = f
			}
		}
	}

	avg := 0.0
	if len(floors) > 0 {
		avg = float64(sum) / float64(len(floors))
	}

	median := 0.0
	if len(floors) > 0 {
		sorted := make([]int, len(floors))
		copy(sorted, floors)
		sort.Ints(sorted)
		n := len(sorted)
		if n%2 == 0 {
			median = float64(sorted[n/2-1]+sorted[n/2]) / 2
		} else {
			median = float64(sorted[n/2])
		}
	}

	return RunSummary{
		RecentFloors: floors,
		MedianFloor:  median,
		BestFloor:    bestFloor,
		AvgFloor:     avg,
	}
}

// prioritizeWeaknesses returns weakness descriptions ordered by severity.
func prioritizeWeaknesses(weaknesses []GuidebookWeakness) []string {
	severityOrder := map[string]int{"critical": 0, "major": 1, "minor": 2}
	sorted := make([]GuidebookWeakness, len(weaknesses))
	copy(sorted, weaknesses)
	sort.Slice(sorted, func(i, j int) bool {
		return severityOrder[sorted[i].Severity] < severityOrder[sorted[j].Severity]
	})

	var priorities []string
	for _, w := range sorted {
		priorities = append(priorities, fmt.Sprintf("[%s/%s] %s", w.Category, w.Severity, w.Description))
	}
	return priorities
}

// ProposeEdit generates a specific edit for a given weakness.
func (w GuidebookWeakness) ProposeEdit(guidebookPath string) (oldContent, newContent string, summary string, err error) {
	oldContent = ""

	// Read current file
	data, err := os.ReadFile(guidebookPath)
	if err != nil {
		return "", "", "", fmt.Errorf("read %s: %w", guidebookPath, err)
	}

	switch filepath.Base(guidebookPath) {
	case "guidebook.md":
		oldContent, newContent, summary, err = proposeGuidebookEdit(string(data), w)
	case "combat-playbook.md":
		oldContent, newContent, summary, err = proposeCombatPlaybookEdit(string(data), w)
	case "event-playbook.md":
		oldContent, newContent, summary, err = proposeEventPlaybookEdit(string(data), w)
	default:
		return "", "", "", fmt.Errorf("unknown file: %s", guidebookPath)
	}
	return
}

func proposeGuidebookEdit(content string, w GuidebookWeakness) (oldSnippet, newSnippet, summary string, err error) {
	summary = fmt.Sprintf("Add %s rule to %s", w.Severity, w.Description[:min(60, len(w.Description))])

	switch w.Category {
	case "combat":
		// Add a bullet to the Stable Heuristics or add a new section
		section := findOrCreateSection(content, "Combat Priorities")
		if section == "" {
			// Add after RL Readiness section
			ins := "\n\n## Combat Priorities\n\n- " + w.Suggestion + "\n"
			return "", content + ins, summary, nil
		}
		// Append to existing section
		oldSnippet = section
		newSnippet = section + "\n- " + w.Suggestion
		return oldSnippet, newSnippet, summary, nil

	case "reward":
		section := findOrCreateSection(content, "Reward Choice")
		if section == "" {
			ins := "\n\n## Reward Choice\n\n- " + w.Suggestion + "\n"
			return "", content + ins, summary, nil
		}
		oldSnippet = section
		newSnippet = section + "\n- " + w.Suggestion
		return oldSnippet, newSnippet, summary, nil

	case "shop":
		section := findOrCreateSection(content, "Shop Economy")
		if section == "" {
			ins := "\n\n## Shop Economy\n\n- " + w.Suggestion + "\n"
			return "", content + ins, summary, nil
		}
		oldSnippet = section
		newSnippet = section + "\n- " + w.Suggestion
		return oldSnippet, newSnippet, summary, nil

	default:
		// Generic: add to end
		ins := "\n\n- " + w.Suggestion + "\n"
		return "", content + ins, summary, nil
	}
}

func proposeCombatPlaybookEdit(content string, w GuidebookWeakness) (oldSnippet, newSnippet, summary string, err error) {
	summary = fmt.Sprintf("Add %s rule to combat-playbook: %s", w.Severity, w.Description[:min(60, len(w.Description))])

	section := findOrCreateSection(content, "Combat Priorities")
	if section == "" {
		ins := "\n\n## Combat Priorities\n\n- " + w.Suggestion + "\n"
		return "", content + ins, summary, nil
	}
	oldSnippet = section
	newSnippet = section + "\n- " + w.Suggestion
	return oldSnippet, newSnippet, summary, nil
}

func proposeEventPlaybookEdit(content string, w GuidebookWeakness) (oldSnippet, newSnippet, summary string, err error) {
	summary = fmt.Sprintf("Add %s rule to event-playbook: %s", w.Severity, w.Description[:min(60, len(w.Description))])

	section := findOrCreateSection(content, "Event Decision Principles")
	if section == "" {
		ins := "\n\n## Event Decision Principles\n\n- " + w.Suggestion + "\n"
		return "", content + ins, summary, nil
	}
	oldSnippet = section
	newSnippet = section + "\n- " + w.Suggestion
	return oldSnippet, newSnippet, summary, nil
}

// findOrCreateSection extracts the content of a markdown section.
// Returns "" if section doesn't exist.
func findOrCreateSection(content, heading string) string {
	// Match ## heading (case insensitive)
	pattern := regexp.MustCompile(`(?im)^##\s+` + regexp.QuoteMeta(heading) + `\s*\n((?:[^\n]|\n(?!\n## ))+)`)
	matches := pattern.FindStringSubmatch(content)
	if len(matches) < 2 {
		return ""
	}
	return "## " + heading + "\n" + strings.TrimSpace(matches[1])
}

// ApplyEdit applies a patch to a file.
// If oldSnippet is empty, appends newSnippet to the file.
// Otherwise replaces oldSnippet with newSnippet.
func ApplyEdit(filePath, oldSnippet, newSnippet string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	content := string(data)

	if oldSnippet == "" {
		content += newSnippet
	} else {
		if !strings.Contains(content, oldSnippet) {
			return fmt.Errorf("old snippet not found in %s", filePath)
		}
		content = strings.Replace(content, oldSnippet, newSnippet, 1)
	}

	return os.WriteFile(filePath, []byte(content), 0o644)
}

// SuggestionForWeakness returns a concrete rule text for a weakness.
func SuggestionForWeakness(w GuidebookWeakness) string {
	switch w.Category {
	case "combat":
		if strings.Contains(strings.ToLower(w.Description), "hp") || strings.Contains(strings.ToLower(w.Description), "health") {
			return "当 HP < 40% 时，优先叠盾而非打伤害；除非有击杀把握，拒绝换血换攻。"
		}
		return w.Suggestion
	case "shop":
		return "进入商店后优先移除基础 Strike 卡；确保在第 10 层前花掉 50% 以上金币。"
	case "reward":
		if strings.Contains(strings.ToLower(w.Description), "early") || strings.Contains(strings.ToLower(w.Description), "floor 7") {
			return "前 3 层优先选伤害+防御牌，避免第 4 层前去精英节点。"
		}
		return w.Suggestion
	default:
		return w.Suggestion
	}
}

// EditableAssets returns all files that the evolution engine can modify.
func EditableAssets(guidebookDir string) ([]string, error) {
	entries, err := os.ReadDir(guidebookDir)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && (strings.HasSuffix(e.Name(), ".md") || strings.HasSuffix(e.Name(), ".json")) {
			files = append(files, filepath.Join(guidebookDir, e.Name()))
		}
	}
	return files, nil
}

// AnalyzeWithLLM takes a GuidebookAnalysis and calls the LLM to propose
// a specific concrete edit. This supplements the rule-based analysis above.
func AnalyzeWithLLM(analysis *GuidebookAnalysis, llmProvider LLMEvolvable, language string) ([]EvolutionEdit, error) {
	if llmProvider == nil || len(analysis.Weaknesses) == 0 {
		return nil, nil
	}

	var edits []EvolutionEdit

	// Build prompt for LLM
	systemPrompt := `You are a game strategy analyst for Slay the Spire 2.
Given a analysis of recent failed runs, propose 1-2 specific, concrete edits
to the strategy guidebook files (guidebook.md, combat-playbook.md, event-playbook.md).
Each edit must be a precise diff: the exact old text and the exact new text.

Respond in JSON:
{
  "edits": [
    {
      "file": "guidebook.md",
      "old_text": "exact text to replace (empty string = append)",
      "new_text": "exact replacement text",
      "summary": "one-line description of why this helps",
      "category": "combat|pathing|shop|reward|runtime"
    }
  ]
}`

	var floors []int
	for _, f := range analysis.RunSummary.RecentFloors {
		floors = append(floors, f)
	}

	userPrompt := fmt.Sprintf(`Recent run analysis:
- Avg floor: %.1f, Median: %.1f, Best: %d
- Recent floors: %v

Weaknesses identified:
%s

Priorities:
%s

Propose 1-2 concrete edits to fix the most critical weakness(es).`, analysis.RunSummary.AvgFloor, analysis.RunSummary.MedianFloor, analysis.RunSummary.BestFloor, floors, strings.Join(analysis.Priorities, "\n"), strings.Join(analysis.Priorities, "\n"))

	response, err := llmProvider.CompleteEvolutionPrompt(systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	// Parse JSON response
	var parsed struct {
		Edits []struct {
			File    string `json:"file"`
			OldText string `json:"old_text"`
			NewText string `json:"new_text"`
			Summary string `json:"summary"`
			Category string `json:"category"`
		} `json:"edits"`
	}
	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		// Try to extract JSON from response
		if idx := strings.Index(response, "{"); idx >= 0 {
			response = response[idx:]
		}
		if endIdx := strings.LastIndex(response, "}"); endIdx >= 0 {
			response = response[:endIdx+1]
		}
		if err := json.Unmarshal([]byte(response), &parsed); err != nil {
			return nil, err
		}
	}

	for _, e := range parsed.Edits {
		edits = append(edits, EvolutionEdit{
			File:    e.File,
			OldText: e.OldText,
			NewText: e.NewText,
			Summary: e.Summary,
			Category: e.Category,
		})
	}

	return edits, nil
}

// LLMEvolvable abstracts the LLM backend for evolution analysis.
type LLMEvolvable interface {
	CompleteEvolutionPrompt(systemPrompt, userPrompt string) (string, error)
}

// EvolutionEdit describes a single proposed edit to a guidebook file.
type EvolutionEdit struct {
	File    string // absolute path
	OldText string
	NewText string
	Summary string
	Category string
}

// Apply applies the edit to the file.
func (e *EvolutionEdit) Apply() error {
	data, err := os.ReadFile(e.File)
	if err != nil {
		return err
	}
	content := string(data)

	if e.OldText == "" {
		content += e.NewText
	} else {
		if !bytes.Contains([]byte(content), []byte(e.OldText)) {
			return fmt.Errorf("old text not found in %s", filepath.Base(e.File))
		}
		content = strings.Replace(content, e.OldText, e.NewText, 1)
	}

	return os.WriteFile(e.File, []byte(content), 0o644)
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}
