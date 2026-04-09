package agentruntime

import (
	"fmt"
	"strings"

	openagent "github.com/hunknownz/open-agent-sdk-go/agent"

	"spire2mind/internal/game"
	"spire2mind/internal/i18n"
)

type CompactMemory struct {
	recent      []string
	lessons     []string
	reflections []string
	lastSummary string
	lastStory   string
}

func NewCompactMemory() *CompactMemory {
	return &CompactMemory{}
}

func (c *CompactMemory) RecordState(state *game.StateSnapshot) {
	if state == nil {
		return
	}

	summary := fmt.Sprintf(
		"screen=%s run=%s actions=%s",
		state.Screen,
		state.RunID,
		strings.Join(state.AvailableActions, ", "),
	)
	c.appendRecent(summary)
	c.lastSummary = buildStateSummary(state)
}

func (c *CompactMemory) RecordAction(action string, state *game.StateSnapshot, message string) {
	if action == "" {
		return
	}

	summary := action
	if message != "" {
		summary += ": " + message
	}
	c.appendRecent(summary)
	if state != nil {
		c.lastSummary = buildStateSummary(state)
	}
}

func (c *CompactMemory) Apply(agent *openagent.Agent, todo *TodoManager, state *game.StateSnapshot) {
	if agent == nil {
		return
	}

	c.lastSummary = buildStateSummary(state)
	agent.Clear()
	c.appendRecent("context compacted")

	if todo != nil {
		c.lessons = appendUniqueTrimmed(c.lessons, summarizeTodoSnapshot(todo.Snapshot()), 6)
	}
}

func (c *CompactMemory) PromptBlock() string {
	return c.PromptBlockForLanguage(i18n.LanguageEnglish)
}

func (c *CompactMemory) PromptBlockForLanguage(language i18n.Language) string {
	loc := i18n.New(language)
	parts := []string{}
	if c.lastSummary != "" {
		parts = append(parts, loc.Label("Run summary", "本局摘要")+":\n"+c.lastSummary)
	}
	if len(c.recent) > 0 {
		parts = append(parts, loc.Label("Recent timeline", "最近流程")+":\n- "+strings.Join(c.recent, "\n- "))
	}
	if len(c.lessons) > 0 {
		parts = append(parts, loc.Label("Persistent reminders", "长期提醒")+":\n- "+strings.Join(c.lessons, "\n- "))
	}
	if len(c.reflections) > 0 {
		parts = append(parts, loc.Label("Lessons from previous attempts", "前几局留下的经验")+":\n- "+strings.Join(c.reflections, "\n- "))
	}
	if c.lastStory != "" {
		parts = append(parts, loc.Label("Latest run reflection", "最近一局复盘")+":\n"+c.lastStory)
	}

	return strings.Join(parts, "\n\n")
}

func (c *CompactMemory) PromptBlockForState(state *game.StateSnapshot) string {
	return c.PromptBlockForStateLanguage(state, i18n.LanguageEnglish)
}

func (c *CompactMemory) PromptBlockForStateLanguage(state *game.StateSnapshot, language i18n.Language) string {
	if c == nil {
		return ""
	}

	loc := i18n.New(language)
	var parts []string
	if summary := strings.TrimSpace(buildStateSummaryForLanguage(state, language)); summary != "" {
		parts = append(parts, loc.Label("Run summary", "本局摘要")+":\n"+summary)
	} else if summary := strings.TrimSpace(c.lastSummary); summary != "" {
		parts = append(parts, loc.Label("Run summary", "本局摘要")+":\n"+summary)
	}
	if recent := c.RecentTimeline(5); len(recent) > 0 {
		parts = append(parts, loc.Label("Recent timeline", "最近流程")+":\n- "+strings.Join(recent, "\n- "))
	}
	if len(c.lessons) > 0 {
		limit := 3
		if strings.EqualFold(stateScreen(state), "COMBAT") {
			limit = 2
		}
		if len(c.lessons) < limit {
			limit = len(c.lessons)
		}
		if limit > 0 {
			parts = append(parts, loc.Label("Persistent reminders", "长期提醒")+":\n- "+strings.Join(c.lessons[len(c.lessons)-limit:], "\n- "))
		}
	}
	return strings.Join(parts, "\n\n")
}

func (c *CompactMemory) appendRecent(entry string) {
	c.recent = appendTrimmed(c.recent, entry, 20)
}

func (c *CompactMemory) RecentTimeline(limit int) []string {
	if limit <= 0 || len(c.recent) == 0 {
		return nil
	}

	if len(c.recent) <= limit {
		return append([]string(nil), c.recent...)
	}

	return append([]string(nil), c.recent[len(c.recent)-limit:]...)
}

func (c *CompactMemory) RecordReflection(reflection *AttemptReflection) {
	if reflection == nil {
		return
	}

	if summary := strings.TrimSpace(reflection.PromptSummary()); summary != "" {
		c.reflections = appendUniqueTrimmed(c.reflections, summary, 8)
	}
	if reflection.Story != "" {
		c.lastStory = reflection.Story
		c.appendRecent("reflection: " + reflection.Story)
	}
	for _, lesson := range reflection.Lessons {
		c.lessons = appendUniqueTrimmed(c.lessons, lesson, 8)
	}
}

func (c *CompactMemory) ApplyResume(resume *SessionResumeState) {
	if resume == nil {
		return
	}

	for _, lesson := range resume.CarryForwardLessons {
		c.lessons = appendUniqueTrimmed(c.lessons, lesson, 8)
	}

	if resume.LastRunID != "" || resume.LastScreen != "" {
		switch {
		case resume.LastRunID != "" && resume.LastScreen != "":
			c.appendRecent(fmt.Sprintf("resumed from run=%s screen=%s", resume.LastRunID, resume.LastScreen))
		case resume.LastRunID != "":
			c.appendRecent(fmt.Sprintf("resumed from run=%s", resume.LastRunID))
		case resume.LastScreen != "":
			c.appendRecent(fmt.Sprintf("resumed from screen=%s", resume.LastScreen))
		}
	}
}
func buildStateSummary(state *game.StateSnapshot) string {
	return buildStateSummaryForLanguage(state, i18n.LanguageEnglish)
}

func buildStateSummaryForLanguage(state *game.StateSnapshot, language i18n.Language) string {
	if state == nil {
		return ""
	}

	loc := i18n.New(language)
	lines := []string{
		fmt.Sprintf("%s: %s", loc.Label("Run", "本局"), state.RunID),
		fmt.Sprintf("%s: %s", loc.Label("Screen", "界面"), state.Screen),
		fmt.Sprintf("%s: %s", loc.Label("Available actions", "可用动作"), strings.Join(state.AvailableActions, ", ")),
	}

	if state.Run != nil {
		if floor, ok := state.Run["floor"]; ok {
			lines = append(lines, fmt.Sprintf("%s: %v", loc.Label("Floor", "层数"), floor))
		}
		if currentHP, ok := state.Run["currentHp"]; ok {
			maxHP := state.Run["maxHp"]
			lines = append(lines, fmt.Sprintf("%s: %v/%v", loc.Label("HP", "生命"), currentHP, maxHP))
		}
		if gold, ok := state.Run["gold"]; ok {
			lines = append(lines, fmt.Sprintf("%s: %v", loc.Label("Gold", "金币"), gold))
		}
	}

	if state.Combat != nil {
		if player, ok := state.Combat["player"].(map[string]interface{}); ok {
			lines = append(lines, fmt.Sprintf("%s: %v", loc.Label("Combat energy", "战斗能量"), player["energy"]))
			lines = append(lines, fmt.Sprintf("%s: %v", loc.Label("Combat block", "战斗格挡"), player["block"]))
		}
	}

	return strings.Join(lines, "\n")
}
