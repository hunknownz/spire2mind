package agentruntime

import (
	"fmt"
	"strings"
)

func appendTrimmed(items []string, entry string, max int) []string {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return items
	}

	items = append(items, entry)
	if len(items) > max {
		items = items[len(items)-max:]
	}

	return items
}

func appendUniqueTrimmed(items []string, entry string, max int) []string {
	entry = strings.TrimSpace(entry)
	if entry == "" {
		return items
	}

	for _, existing := range items {
		if strings.EqualFold(strings.TrimSpace(existing), entry) {
			return items
		}
	}

	items = append(items, entry)
	if len(items) > max {
		items = items[len(items)-max:]
	}

	return items
}

func summarizeTodoSnapshot(snapshot TodoSnapshot) string {
	parts := make([]string, 0, 3)

	if currentGoal := strings.TrimSpace(snapshot.CurrentGoal); currentGoal != "" {
		parts = append(parts, "goal="+currentGoal)
	}
	if roomGoal := strings.TrimSpace(snapshot.RoomGoal); roomGoal != "" {
		parts = append(parts, "room="+roomGoal)
	}
	if nextIntent := strings.TrimSpace(snapshot.NextIntent); nextIntent != "" {
		parts = append(parts, "intent="+nextIntent)
	}

	if len(parts) == 0 {
		return ""
	}

	return fmt.Sprintf("working memory: %s", strings.Join(parts, " | "))
}
