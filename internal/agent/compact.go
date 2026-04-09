package agentruntime

import (
	"fmt"
	"strings"

	openagent "github.com/hunknownz/open-agent-sdk-go/agent"

	"spire2mind/internal/game"
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
	parts := []string{}
	if c.lastSummary != "" {
		parts = append(parts, "Run summary:\n"+c.lastSummary)
	}
	if len(c.recent) > 0 {
		parts = append(parts, "Recent timeline:\n- "+strings.Join(c.recent, "\n- "))
	}
	if len(c.lessons) > 0 {
		parts = append(parts, "Persistent reminders:\n- "+strings.Join(c.lessons, "\n- "))
	}
	if len(c.reflections) > 0 {
		parts = append(parts, "Lessons from previous attempts:\n- "+strings.Join(c.reflections, "\n- "))
	}
	if c.lastStory != "" {
		parts = append(parts, "Latest run reflection:\n"+c.lastStory)
	}

	return strings.Join(parts, "\n\n")
}

func (c *CompactMemory) PromptBlockForState(state *game.StateSnapshot) string {
	if c == nil {
		return ""
	}

	var parts []string
	if summary := strings.TrimSpace(c.lastSummary); summary != "" {
		parts = append(parts, "Run summary:\n"+summary)
	}
	if recent := c.RecentTimeline(5); len(recent) > 0 {
		parts = append(parts, "Recent timeline:\n- "+strings.Join(recent, "\n- "))
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
			parts = append(parts, "Persistent reminders:\n- "+strings.Join(c.lessons[len(c.lessons)-limit:], "\n- "))
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
	if state == nil {
		return ""
	}

	lines := []string{
		fmt.Sprintf("Run: %s", state.RunID),
		fmt.Sprintf("Screen: %s", state.Screen),
		fmt.Sprintf("Available actions: %s", strings.Join(state.AvailableActions, ", ")),
	}

	if state.Run != nil {
		if floor, ok := state.Run["floor"]; ok {
			lines = append(lines, fmt.Sprintf("Floor: %v", floor))
		}
		if currentHP, ok := state.Run["currentHp"]; ok {
			maxHP := state.Run["maxHp"]
			lines = append(lines, fmt.Sprintf("HP: %v/%v", currentHP, maxHP))
		}
		if gold, ok := state.Run["gold"]; ok {
			lines = append(lines, fmt.Sprintf("Gold: %v", gold))
		}
	}

	if state.Combat != nil {
		if player, ok := state.Combat["player"].(map[string]interface{}); ok {
			lines = append(lines, fmt.Sprintf("Combat energy: %v", player["energy"]))
			lines = append(lines, fmt.Sprintf("Combat block: %v", player["block"]))
		}
	}

	return strings.Join(lines, "\n")
}
