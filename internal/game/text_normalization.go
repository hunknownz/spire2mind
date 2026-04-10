package game

import "spire2mind/internal/i18n"

func NormalizeStateSnapshot(state *StateSnapshot) *StateSnapshot {
	if state == nil {
		return nil
	}

	state.RunID = i18n.RepairText(state.RunID)
	state.Screen = i18n.RepairText(state.Screen)
	state.AvailableActions = normalizeStringSlice(state.AvailableActions)
	state.Session = normalizeMapAny(state.Session)
	state.Run = normalizeMapAny(state.Run)
	state.Combat = normalizeMapAny(state.Combat)
	state.Map = normalizeMapAny(state.Map)
	state.Selection = normalizeMapAny(state.Selection)
	state.Reward = normalizeMapAny(state.Reward)
	state.Event = normalizeMapAny(state.Event)
	state.CharacterSelect = normalizeMapAny(state.CharacterSelect)
	state.Chest = normalizeMapAny(state.Chest)
	state.Shop = normalizeMapAny(state.Shop)
	state.Rest = normalizeMapAny(state.Rest)
	state.Modal = normalizeMapAny(state.Modal)
	state.GameOver = normalizeMapAny(state.GameOver)
	state.AgentView = normalizeMapAny(state.AgentView)
	state.Multiplayer = normalizeMapAny(state.Multiplayer)
	state.MultiplayerLobby = normalizeMapAny(state.MultiplayerLobby)
	state.Timeline = normalizeMapAny(state.Timeline)
	return state
}

func normalizeMapAny(input map[string]any) map[string]any {
	if len(input) == 0 {
		return input
	}

	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = i18n.RepairAny(value)
	}
	return output
}

func normalizeStringSlice(input []string) []string {
	if len(input) == 0 {
		return input
	}

	output := make([]string, len(input))
	for i, value := range input {
		output[i] = i18n.RepairText(value)
	}
	return output
}
