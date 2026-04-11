package agentruntime

import (
	"context"
	"fmt"
	"io"

	"spire2mind/internal/config"
	"spire2mind/internal/game"
	"spire2mind/internal/i18n"
)

func BuildCyclePrompt(state *game.StateSnapshot, todo *TodoManager, skills *SkillLibrary, compact *CompactMemory, planner *CombatPlan, language i18n.Language) string {
	return NewPromptAssemblyPipeline().Build(PromptModeCycle, state, todo, skills, compact, planner, nil, language).Text
}

func RunHeadless(ctx context.Context, cfg config.Config, output io.Writer) error {
	session, err := StartSession(ctx, cfg)
	if err != nil {
		return err
	}
	defer session.Close()

	for event := range session.Events() {
		renderHeadlessEvent(output, event)
	}

	if err := <-session.Errors(); err != nil {
		return err
	}

	return nil
}

func renderHeadlessEvent(output io.Writer, event SessionEvent) {
	switch event.Kind {
	case SessionEventState:
		if event.State != nil {
			fmt.Fprintf(
				output,
				"[state] cycle=%d attempt=%d screen=%s run=%s actions=%s\n",
				event.Cycle,
				event.Attempt,
				event.State.Screen,
				event.State.RunID,
				stringsJoin(event.State.AvailableActions, ","),
			)
		}
	case SessionEventAction:
		fmt.Fprintf(output, "[action] cycle=%d attempt=%d %s (%s)\n", event.Cycle, event.Attempt, event.Action, event.Message)
	case SessionEventTool:
		fmt.Fprintf(output, "[tool] cycle=%d attempt=%d %s %s\n", event.Cycle, event.Attempt, event.Tool, event.Action)
	case SessionEventAssistant:
		fmt.Fprintf(output, "[assistant] cycle=%d attempt=%d %s\n", event.Cycle, event.Attempt, event.Message)
	case SessionEventStreamer:
		fmt.Fprintf(output, "[streamer] cycle=%d attempt=%d %s\n", event.Cycle, event.Attempt, event.Message)
	case SessionEventToolError:
		fmt.Fprintf(output, "[tool-error] cycle=%d attempt=%d %s\n", event.Cycle, event.Attempt, event.Message)
	case SessionEventCompact:
		fmt.Fprintf(output, "[compact] cycle=%d attempt=%d %s\n", event.Cycle, event.Attempt, event.Message)
	case SessionEventPrompt:
		fmt.Fprintf(output, "[prompt] cycle=%d attempt=%d %s\n", event.Cycle, event.Attempt, compactText(event.Message, 96))
	case SessionEventReflection:
		fmt.Fprintf(output, "[reflection] cycle=%d attempt=%d %s\n", event.Cycle, event.Attempt, event.Message)
	case SessionEventStop:
		fmt.Fprintf(output, "[stop] cycle=%d attempt=%d %s\n", event.Cycle, event.Attempt, event.Message)
	case SessionEventStatus:
		extra := ""
		if path, ok := event.Data["dashboard_path"].(string); ok && path != "" {
			extra = fmt.Sprintf(" dashboard=%s", path)
		}
		if story, ok := event.Data["run_story_path"].(string); ok && story != "" {
			extra += fmt.Sprintf(" story=%s", story)
		}
		if guide, ok := event.Data["run_guide_path"].(string); ok && guide != "" {
			extra += fmt.Sprintf(" guide=%s", guide)
		}
		if index, ok := event.Data["run_index_path"].(string); ok && index != "" {
			extra += fmt.Sprintf(" index=%s", index)
		}
		if guidebook, ok := event.Data["guidebook_path"].(string); ok && guidebook != "" {
			extra += fmt.Sprintf(" guidebook=%s", guidebook)
		}
		if duration, ok := statusMetricInt64(event.Data["cycle_duration_ms"]); ok {
			extra += fmt.Sprintf(" latency_ms=%d", duration)
		}
		if input, ok := statusMetricInt(event.Data["input_tokens"]); ok {
			extra += fmt.Sprintf(" in_tokens=%d", input)
		}
		if output, ok := statusMetricInt(event.Data["output_tokens"]); ok {
			extra += fmt.Sprintf(" out_tokens=%d", output)
		}
		if promptSize, ok := statusMetricInt(event.Data["prompt_size_bytes"]); ok {
			extra += fmt.Sprintf(" prompt_bytes=%d", promptSize)
		}
		if codex, ok := event.Data["living_codex_path"].(string); ok && codex != "" {
			extra += fmt.Sprintf(" codex=%s", codex)
		}
		if combatPlaybook, ok := event.Data["combat_playbook_path"].(string); ok && combatPlaybook != "" {
			extra += fmt.Sprintf(" combat_playbook=%s", combatPlaybook)
		}
		if eventPlaybook, ok := event.Data["event_playbook_path"].(string); ok && eventPlaybook != "" {
			extra += fmt.Sprintf(" event_playbook=%s", eventPlaybook)
		}
		if event.Cost > 0 || event.Turns > 0 {
			fmt.Fprintf(output, "[status] cycle=%d attempt=%d %s turns=%d cost=%.4f%s\n", event.Cycle, event.Attempt, event.Message, event.Turns, event.Cost, extra)
		} else {
			fmt.Fprintf(output, "[status] cycle=%d attempt=%d %s%s\n", event.Cycle, event.Attempt, event.Message, extra)
		}
	}
}

func statusMetricInt(value interface{}) (int, bool) {
	switch typed := value.(type) {
	case int:
		return typed, true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case float64:
		return int(typed), true
	default:
		return 0, false
	}
}

func statusMetricInt64(value interface{}) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int32:
		return int64(typed), true
	case int64:
		return typed, true
	case float64:
		return int64(typed), true
	default:
		return 0, false
	}
}

func stringsJoin(values []string, sep string) string {
	if len(values) == 0 {
		return ""
	}
	result := values[0]
	for i := 1; i < len(values); i++ {
		result += sep + values[i]
	}
	return result
}
