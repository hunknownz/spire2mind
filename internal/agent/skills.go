package agentruntime

import (
	"os"
	"path/filepath"
	"strings"

	"spire2mind/internal/game"
)

type SkillLibrary struct {
	root  string
	cache map[string]string
}

func NewSkillLibrary(repoRoot string) *SkillLibrary {
	return &SkillLibrary{
		root:  filepath.Join(repoRoot, "data", "skills"),
		cache: make(map[string]string),
	}
}

func (s *SkillLibrary) PromptBlock(state *game.StateSnapshot) string {
	if state == nil {
		return ""
	}

	orderedNames := s.relevantSkills(state)
	if len(orderedNames) == 0 {
		return ""
	}

	blocks := make([]string, 0, len(orderedNames))
	for _, name := range orderedNames {
		content := s.load(name)
		if content == "" {
			continue
		}

		blocks = append(blocks, "Skill: "+name+"\n"+content)
	}

	return strings.Join(blocks, "\n\n")
}

func (s *SkillLibrary) PromptBlockForState(state *game.StateSnapshot) string {
	if state == nil {
		return ""
	}

	orderedNames := s.relevantSkillsForPrompt(state)
	if len(orderedNames) == 0 {
		return ""
	}

	blocks := make([]string, 0, len(orderedNames))
	for _, name := range orderedNames {
		content := summarizeSkillContent(s.load(name), 14, 1200)
		if content == "" {
			continue
		}
		blocks = append(blocks, "Skill: "+name+"\n"+content)
	}

	return strings.Join(blocks, "\n\n")
}

func (s *SkillLibrary) relevantSkills(state *game.StateSnapshot) []string {
	switch state.Screen {
	case "COMBAT":
		return []string{"combat-basics", "deck-archetypes"}
	case "EVENT":
		return []string{"event-guide"}
	case "SHOP":
		return []string{"shop-economy", "deck-archetypes"}
	case "MAP":
		return []string{"boss-elite-routing"}
	case "REWARD", "CARD_SELECTION":
		return []string{"deck-archetypes"}
	default:
		return nil
	}
}

func (s *SkillLibrary) relevantSkillsForPrompt(state *game.StateSnapshot) []string {
	switch stateScreen(state) {
	case "COMBAT":
		if len(nestedList(state.Combat, "enemies")) > 1 {
			return []string{"combat-basics"}
		}
		return []string{"combat-basics", "deck-archetypes"}
	case "EVENT":
		return []string{"event-guide"}
	case "SHOP":
		return []string{"shop-economy"}
	case "MAP":
		return []string{"boss-elite-routing"}
	case "REWARD", "CARD_SELECTION":
		return []string{"deck-archetypes"}
	default:
		return nil
	}
}

func (s *SkillLibrary) load(name string) string {
	if content, ok := s.cache[name]; ok {
		return content
	}

	path := filepath.Join(s.root, name, "SKILL.md")
	bytes, err := os.ReadFile(path)
	if err != nil {
		s.cache[name] = ""
		return ""
	}

	content := strings.TrimSpace(string(bytes))
	s.cache[name] = content
	return content
}

func summarizeSkillContent(content string, maxLines int, maxBytes int) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}

	lines := strings.Split(content, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		filtered = append(filtered, line)
		if len(filtered) >= maxLines {
			break
		}
	}

	summary := strings.Join(filtered, "\n")
	for len([]byte(summary)) > maxBytes && len(filtered) > 1 {
		filtered = filtered[:len(filtered)-1]
		summary = strings.Join(filtered, "\n")
	}
	return summary
}
