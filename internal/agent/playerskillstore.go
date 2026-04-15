package agentruntime

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const playerSkillInitial = `---
name: sts2-player
version: 1
playstyle: cautious-optimizer
description: STS2 autonomous player identity. Defines playstyle, signature patterns, and evolved preferences. Injected into every decision prompt.
updated_at: %s
runs_analyzed: 0
avg_floor: 0.0
best_floor: 0
---

# STS2 Player Identity

## Playstyle

I am a cautious optimizer. I prioritize run stability over flashy plays, convert resources early, and switch to a defensive posture when HP drops below 50%. I do not race enemies at low health.

## Signature Patterns (What Works)

- Removing basic Strikes early in Act 1 improves draw quality and deck consistency
- Maintaining at least 30%% defense cards prevents late-game collapses
- Converting gold before floor 10 avoids dying with unspent resources

## Known Weaknesses (What to Watch)

- Tendency to race at low HP instead of switching to block-first mode
- Undervaluing scaling cards in midgame (floors 10-15)
- Leaving gold unspent when no obvious purchase is immediately visible

## Playstyle Directives

- When HP < 40%%, treat survival as the primary objective; defer damage optimization
- At shops, evaluate card removal before other purchases
- On map, prefer routes with rest sites when HP < 60%% and no elite is necessary
- At floor 10 checkpoint: if no scaling card has been picked, prioritize one at next reward or shop

## Performance History

- Avg floor: 0.0 (last 0 runs)
- Best floor: 0
- Most common death cause: unknown
- Most common resource waste: unknown
- Act 2 entry rate: 0/0 runs
`

const playerSkillMaxBytes = 1200

// PlayerSkillStore manages the evolvable player identity document.
// The document lives at combat/knowledge/player-skill/sts2-player-skill.md and is
// injected into every decision prompt as a "Player Identity" block.
type PlayerSkillStore struct {
	path    string
	content string
	mu      sync.Mutex
}

// NewPlayerSkillStore creates or loads the player skill store.
// repoRoot should be the spire2mind repository root.
func NewPlayerSkillStore(repoRoot string) (*PlayerSkillStore, error) {
	dir := filepath.Join(repoRoot, "combat", "knowledge", "player-skill")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir player-skill: %w", err)
	}

	path := filepath.Join(dir, "sts2-player-skill.md")
	s := &PlayerSkillStore{path: path}

	// Write initial document if it doesn't exist
	if _, err := os.Stat(path); os.IsNotExist(err) {
		initial := fmt.Sprintf(playerSkillInitial, time.Now().UTC().Format(time.RFC3339))
		if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
			return nil, fmt.Errorf("write initial player skill: %w", err)
		}
	}

	if err := s.Load(); err != nil {
		return nil, err
	}
	return s, nil
}

// Load reads the player skill document from disk.
func (s *PlayerSkillStore) Load() error {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return fmt.Errorf("read player skill: %w", err)
	}
	s.mu.Lock()
	s.content = string(data)
	s.mu.Unlock()
	return nil
}

// Reload re-reads the document from disk (call after evolution edits).
func (s *PlayerSkillStore) Reload() error {
	return s.Load()
}

// Path returns the absolute path to the player skill document.
func (s *PlayerSkillStore) Path() string {
	return s.path
}

// PromptBlock strips YAML frontmatter and returns the markdown body,
// truncated to playerSkillMaxBytes for prompt budget.
func (s *PlayerSkillStore) PromptBlock() string {
	s.mu.Lock()
	content := s.content
	s.mu.Unlock()

	body := stripFrontmatter(content)
	if len(body) > playerSkillMaxBytes {
		body = body[:playerSkillMaxBytes]
		// Trim to last newline to avoid cutting mid-line
		if idx := strings.LastIndex(body, "\n"); idx > 0 {
			body = body[:idx]
		}
	}
	return strings.TrimSpace(body)
}

// UpdateMetrics rewrites the ## Performance History section with fresh stats.
func (s *PlayerSkillStore) UpdateMetrics(avgFloor float64, bestFloor, runsAnalyzed int) error {
	s.mu.Lock()
	content := s.content
	s.mu.Unlock()

	const section = "## Performance History"
	idx := strings.Index(content, "\n"+section)
	if idx < 0 {
		// Section not found — append it
		newSection := fmt.Sprintf("\n\n%s\n\n- Avg floor: %.1f (last %d runs)\n- Best floor: %d\n",
			section, avgFloor, runsAnalyzed, bestFloor)
		content += newSection
	} else {
		// Find end of section (next ## or EOF)
		sectionStart := idx + 1
		rest := content[sectionStart:]
		nextSection := strings.Index(rest[len(section):], "\n## ")
		var sectionEnd int
		if nextSection < 0 {
			sectionEnd = len(content)
		} else {
			sectionEnd = sectionStart + len(section) + nextSection + 1
		}

		newSection := fmt.Sprintf("%s\n\n- Avg floor: %.1f (last %d runs)\n- Best floor: %d\n",
			section, avgFloor, runsAnalyzed, bestFloor)
		content = content[:sectionStart] + newSection + content[sectionEnd:]
	}

	if err := os.WriteFile(s.path, []byte(content), 0o644); err != nil {
		return err
	}
	s.mu.Lock()
	s.content = content
	s.mu.Unlock()
	return nil
}

// AppendToSection appends a bullet point to a named section.
// If the section doesn't exist, it is created at the end of the document.
func (s *PlayerSkillStore) AppendToSection(sectionName, bullet string) error {
	s.mu.Lock()
	content := s.content
	s.mu.Unlock()

	heading := "## " + sectionName
	idx := strings.Index(content, "\n"+heading)
	if idx < 0 {
		// Create section
		content += fmt.Sprintf("\n\n%s\n\n- %s\n", heading, bullet)
	} else {
		// Find end of section
		sectionStart := idx + 1 + len(heading)
		rest := content[sectionStart:]
		nextSection := strings.Index(rest, "\n## ")
		var insertAt int
		if nextSection < 0 {
			insertAt = len(content)
			// Trim trailing whitespace before appending
			content = strings.TrimRight(content, "\n") + "\n"
			insertAt = len(content)
		} else {
			insertAt = sectionStart + nextSection + 1
		}
		content = content[:insertAt] + "- " + bullet + "\n" + content[insertAt:]
	}

	if err := os.WriteFile(s.path, []byte(content), 0o644); err != nil {
		return err
	}
	s.mu.Lock()
	s.content = content
	s.mu.Unlock()
	return nil
}

// stripFrontmatter removes YAML frontmatter (--- ... ---) from markdown content.
func stripFrontmatter(content string) string {
	if !strings.HasPrefix(content, "---") {
		return content
	}
	// Find closing ---
	rest := content[3:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return content
	}
	return rest[end+4:]
}
