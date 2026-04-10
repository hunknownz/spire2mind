package agentruntime

import (
	"fmt"
	"strconv"
	"strings"

	"spire2mind/internal/game"
)

func nestedList(root map[string]any, key string) []map[string]any {
	if len(root) == 0 {
		return nil
	}

	raw, ok := root[key]
	if !ok {
		return nil
	}

	items, ok := raw.([]interface{})
	if !ok {
		return nil
	}

	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		if typed, ok := item.(map[string]any); ok {
			result = append(result, typed)
		}
	}

	return result
}

func nestedMap(root map[string]any, key string) map[string]any {
	if len(root) == 0 {
		return nil
	}
	if typed, ok := root[key].(map[string]any); ok {
		return typed
	}
	return nil
}

func fieldBool(root map[string]any, key string) bool {
	if len(root) == 0 {
		return false
	}

	switch value := root[key].(type) {
	case bool:
		return value
	case string:
		return strings.EqualFold(value, "true")
	default:
		return false
	}
}

func fieldInt(root map[string]any, key string) (int, bool) {
	if len(root) == 0 {
		return 0, false
	}

	switch value := root[key].(type) {
	case int:
		return value, true
	case int32:
		return int(value), true
	case int64:
		return int(value), true
	case float64:
		return int(value), true
	case string:
		parsed, err := strconv.Atoi(value)
		if err == nil {
			return parsed, true
		}
	}

	return 0, false
}

func fieldString(root map[string]any, key string) string {
	if len(root) == 0 {
		return ""
	}

	switch value := root[key].(type) {
	case string:
		return value
	default:
		return ""
	}
}

func fieldIntSlice(root map[string]any, key string) []int {
	if len(root) == 0 {
		return nil
	}

	items, ok := root[key].([]interface{})
	if !ok {
		return nil
	}

	result := make([]int, 0, len(items))
	for _, item := range items {
		switch value := item.(type) {
		case int:
			result = append(result, value)
		case int32:
			result = append(result, int(value))
		case int64:
			result = append(result, int(value))
		case float64:
			result = append(result, int(value))
		}
	}

	return result
}

func formatActionDebug(request game.ActionRequest) string {
	parts := []string{request.Action}
	if request.CardIndex != nil {
		parts = append(parts, fmt.Sprintf("card=%d", *request.CardIndex))
	}
	if request.OptionIndex != nil {
		parts = append(parts, fmt.Sprintf("option=%d", *request.OptionIndex))
	}
	if request.TargetIndex != nil {
		parts = append(parts, fmt.Sprintf("target=%d", *request.TargetIndex))
	}
	return strings.Join(parts, " ")
}

// ── Typed run-state accessors ────────────────────────────────────

func hpRatio(state *game.StateSnapshot) float64 {
	if state == nil || state.Run == nil || state.Run.MaxHp <= 0 {
		return 1
	}
	return float64(state.Run.CurrentHp) / float64(state.Run.MaxHp)
}

func runFloor(state *game.StateSnapshot) int {
	if state == nil || state.Run == nil {
		return 0
	}
	return state.Run.Floor
}

func runGold(state *game.StateSnapshot) int {
	if state == nil || state.Run == nil {
		return 0
	}
	return state.Run.Gold
}

func runDeckCount(state *game.StateSnapshot) int {
	if state == nil || state.Run == nil {
		return 0
	}
	return state.Run.DeckCount
}

func agentViewHeadline(state *game.StateSnapshot) string {
	if state == nil || state.AgentView == nil {
		return ""
	}
	return state.AgentView.Headline
}

func firstIndexedOption(root map[string]any, key string) *int {
	for _, option := range nestedList(root, key) {
		if index, ok := fieldInt(option, "index"); ok {
			return &index
		}
	}

	return nil
}

func firstMatchingRestOption(rest map[string]any, keywords ...string) *int {
	for _, option := range nestedList(rest, "options") {
		if !fieldBool(option, "isEnabled") {
			continue
		}

		label := strings.ToLower(strings.TrimSpace(fieldString(option, "optionId") + " " + fieldString(option, "title") + " " + fieldString(option, "description")))
		for _, keyword := range keywords {
			if strings.Contains(label, keyword) {
				if index, ok := fieldInt(option, "index"); ok {
					return &index
				}
			}
		}
	}

	return nil
}

func firstMatchingRestOptionTyped(rest *game.RestState, keywords ...string) *int {
	if rest == nil {
		return nil
	}
	for _, option := range rest.Options {
		if !option.IsEnabled {
			continue
		}
		label := strings.ToLower(strings.TrimSpace(option.OptionID + " " + option.Title + " " + option.Description))
		for _, keyword := range keywords {
			if strings.Contains(label, keyword) {
				index := option.Index
				return &index
			}
		}
	}
	return nil
}

func firstIndexedOptionTyped(state *game.StateSnapshot, section string, key string) *int {
	switch section + "." + key {
	case "Chest.relicOptions":
		if state == nil || state.Chest == nil {
			return nil
		}
		for _, r := range state.Chest.RelicOptions {
			index := r.Index
			return &index
		}
	case "Selection.cards":
		if state == nil || state.Selection == nil {
			return nil
		}
		for _, c := range state.Selection.Cards {
			index := c.Index
			return &index
		}
	}
	return nil
}
