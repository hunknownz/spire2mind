package agentruntime

import (
	"spire2mind/internal/game"
	"spire2mind/internal/i18n"
)

func normalizeSessionEvent(event SessionEvent) SessionEvent {
	event.Message = i18n.RepairText(event.Message)
	event.Screen = i18n.RepairText(event.Screen)
	event.RunID = i18n.RepairText(event.RunID)
	event.Action = i18n.RepairText(event.Action)
	event.Tool = i18n.RepairText(event.Tool)
	event.State = game.NormalizeStateSnapshot(event.State)
	if event.Data != nil {
		if repaired, ok := i18n.RepairAny(event.Data).(map[string]any); ok {
			event.Data = repaired
		}
	}
	return event
}
