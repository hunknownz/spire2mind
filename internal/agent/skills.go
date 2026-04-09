package agentruntime

import (
	"os"
	"path/filepath"
	"strings"

	"spire2mind/internal/game"
	"spire2mind/internal/i18n"
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
	return s.PromptBlockForLanguage(state, i18n.LanguageEnglish)
}

func (s *SkillLibrary) PromptBlockForLanguage(state *game.StateSnapshot, language i18n.Language) string {
	if state == nil {
		return ""
	}

	loc := i18n.New(language)
	orderedNames := s.relevantSkills(state)
	if len(orderedNames) == 0 {
		return ""
	}

	blocks := make([]string, 0, len(orderedNames))
	for _, name := range orderedNames {
		content := s.promptContent(name, language, false)
		if content == "" {
			continue
		}

		blocks = append(blocks, loc.Label("Skill", "技能参考")+": "+localizeSkillName(name, loc)+"\n"+content)
	}

	return strings.Join(blocks, "\n\n")
}

func (s *SkillLibrary) PromptBlockForState(state *game.StateSnapshot) string {
	return s.PromptBlockForStateLanguage(state, i18n.LanguageEnglish)
}

func (s *SkillLibrary) PromptBlockForStateLanguage(state *game.StateSnapshot, language i18n.Language) string {
	if state == nil {
		return ""
	}

	loc := i18n.New(language)
	orderedNames := s.relevantSkillsForPrompt(state)
	if len(orderedNames) == 0 {
		return ""
	}

	blocks := make([]string, 0, len(orderedNames))
	for _, name := range orderedNames {
		content := s.promptContent(name, language, true)
		if content == "" {
			continue
		}
		blocks = append(blocks, loc.Label("Skill", "技能参考")+": "+localizeSkillName(name, loc)+"\n"+content)
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

func (s *SkillLibrary) promptContent(name string, language i18n.Language, summarized bool) string {
	loc := i18n.New(language)
	if localized := localizedSkillSummary(name, loc); localized != "" {
		return localized
	}

	content := s.load(name)
	if content == "" {
		return ""
	}
	if summarized {
		return summarizeSkillContent(content, 14, 1200)
	}
	return content
}

func localizeSkillName(name string, loc i18n.Localizer) string {
	if loc.Language() == i18n.LanguageEnglish {
		return name
	}
	switch name {
	case "combat-basics":
		return "战斗基础"
	case "deck-archetypes":
		return "牌组方向"
	case "event-guide":
		return "事件处理"
	case "shop-economy":
		return "商店经济"
	case "boss-elite-routing":
		return "路线与精英"
	default:
		return name
	}
}

func localizedSkillSummary(name string, loc i18n.Localizer) string {
	if loc.Language() == i18n.LanguageEnglish {
		return ""
	}
	switch name {
	case "combat-basics":
		return strings.Join([]string{
			"- 先处理致命与高压回合，避免为了漂亮值而掉太多血。",
			"- 能减员就优先减员；安全回合再把能量转成输出或铺垫。",
			"- 如果只有一条明显合法线路，就直接执行，不要犹豫。",
		}, "\n")
	case "deck-archetypes":
		return strings.Join([]string{
			"- 早期优先拿立刻提升战力和稳定性的牌，不要太早贪远期成长。",
			"- 牌组过厚时少拿低质量牌，优先移除和高影响单卡。",
			"- 选择奖励时优先补当前最缺的环节：输出、格挡、过牌或稳定性。",
		}, "\n")
	case "event-guide":
		return strings.Join([]string{
			"- 事件优先选择稳定收益，避免明显毁局的高波动选项。",
			"- 低血时更保守，高血且资源足时才考虑高风险高收益。",
			"- 事件结束后尽快推进，不要在无价值的选项上停滞。",
		}, "\n")
	case "shop-economy":
		return strings.Join([]string{
			"- 有机会时优先考虑移除卡牌、强力遗物和关键解法。",
			"- 不要带着大量金币死去；金币要尽早转成胜率。",
			"- 避免把金币花在低影响填充物上。",
		}, "\n")
	case "boss-elite-routing":
		return strings.Join([]string{
			"- 低血时优先短线、休息点和低方差节点。",
			"- 只有当前战力明显足够时才主动找高风险 elite。",
			"- 路线目标是把这局打得更深，而不是单纯追求高收益节点。",
		}, "\n")
	default:
		return ""
	}
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
