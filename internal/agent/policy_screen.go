package agentruntime

import "spire2mind/internal/game"

func firstUnlockedCharacterIndex(state *game.StateSnapshot) *int {
	if state == nil || state.CharacterSelect == nil {
		return nil
	}
	for _, option := range state.CharacterSelect.Characters {
		if option.IsLocked {
			continue
		}
		if option.IsRandom {
			continue
		}
		index := option.Index
		return &index
	}
	return nil
}

func characterAlreadySelected(state *game.StateSnapshot) bool {
	if state == nil || state.CharacterSelect == nil {
		return false
	}

	selectedID := state.CharacterSelect.SelectedCharacter
	if selectedID == "" {
		return false
	}

	for _, option := range state.CharacterSelect.Characters {
		if option.CharID != selectedID {
			continue
		}
		return option.IsSelected || !option.IsLocked
	}

	return false
}

func preferredRestOption(state *game.StateSnapshot) *int {
	if state == nil || state.Rest == nil || len(state.Rest.Options) == 0 {
		return nil
	}
	rest := state.Rest

	if hpRatio(state) < 0.55 {
		if index := firstMatchingRestOptionTyped(rest, "heal", "rest", "recover"); index != nil {
			return index
		}
	}

	if index := firstMatchingRestOptionTyped(rest, "smith", "upgrade", "enhance", "enchant"); index != nil {
		return index
	}

	var enabled []int
	for _, option := range rest.Options {
		if !option.IsEnabled {
			continue
		}
		enabled = append(enabled, option.Index)
	}

	if len(enabled) == 1 {
		return &enabled[0]
	}

	return nil
}

func firstEnabledEventOption(event *game.EventState) *int {
	if event == nil {
		return nil
	}
	var enabled []int
	for _, option := range event.Options {
		if option.IsLocked {
			continue
		}
		if option.IsProceed && len(enabled) > 0 {
			continue
		}
		enabled = append(enabled, option.Index)
	}

	if len(enabled) > 0 {
		return &enabled[0]
	}

	return nil
}
