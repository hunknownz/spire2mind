package agentruntime

import (
	"strings"

	"spire2mind/internal/game"
)

func shopInventoryOpen(state *game.StateSnapshot) bool {
	if state == nil || !strings.EqualFold(strings.TrimSpace(state.Screen), "SHOP") {
		return false
	}

	for _, action := range state.AvailableActions {
		switch strings.ToLower(strings.TrimSpace(action)) {
		case "buy_card", "buy_relic", "buy_potion", "remove_card_at_shop", "close_shop_inventory":
			return true
		}
	}

	return false
}
