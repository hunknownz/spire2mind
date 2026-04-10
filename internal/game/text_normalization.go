package game

import "spire2mind/internal/i18n"

func NormalizeStateSnapshot(state *StateSnapshot) *StateSnapshot {
	if state == nil {
		return nil
	}

	state.RunID = i18n.RepairText(state.RunID)
	state.Screen = i18n.RepairText(state.Screen)
	state.AvailableActions = normalizeStringSlice(state.AvailableActions)

	// Typed struct fields contain string values that may need i18n repair.
	// Repair is applied to frequently-displayed names/descriptions.
	normalizeRunState(state.Run)
	normalizeCombatState(state.Combat)

	// map[string]any fields still use generic normalization.
	state.Multiplayer = normalizeMapAny(state.Multiplayer)
	state.MultiplayerLobby = normalizeMapAny(state.MultiplayerLobby)
	state.Timeline = normalizeMapAny(state.Timeline)

	return state
}

func normalizeRunState(run *RunState) {
	if run == nil {
		return
	}
	run.Character = i18n.RepairText(run.Character)
	for i := range run.Deck {
		run.Deck[i].Name = i18n.RepairText(run.Deck[i].Name)
	}
	for i := range run.Relics {
		run.Relics[i].Name = i18n.RepairText(run.Relics[i].Name)
	}
	for i := range run.Potions {
		run.Potions[i].Name = i18n.RepairText(run.Potions[i].Name)
	}
}

func normalizeCombatState(combat *CombatState) {
	if combat == nil {
		return
	}
	for i := range combat.Hand {
		combat.Hand[i].Name = i18n.RepairText(combat.Hand[i].Name)
	}
	for i := range combat.Enemies {
		combat.Enemies[i].Name = i18n.RepairText(combat.Enemies[i].Name)
		for j := range combat.Enemies[i].Powers {
			combat.Enemies[i].Powers[j].Name = i18n.RepairText(combat.Enemies[i].Powers[j].Name)
		}
	}
	for i := range combat.Player.Powers {
		combat.Player.Powers[i].Name = i18n.RepairText(combat.Player.Powers[i].Name)
	}
	for i := range combat.DrawPile {
		combat.DrawPile[i].Name = i18n.RepairText(combat.DrawPile[i].Name)
	}
	for i := range combat.DiscardPile {
		combat.DiscardPile[i].Name = i18n.RepairText(combat.DiscardPile[i].Name)
	}
	for i := range combat.ExhaustPile {
		combat.ExhaustPile[i].Name = i18n.RepairText(combat.ExhaustPile[i].Name)
	}
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
