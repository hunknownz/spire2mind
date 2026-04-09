package agentruntime

import (
	"fmt"
	"strings"

	"spire2mind/internal/game"
)

func BuildTacticalHints(state *game.StateSnapshot) []string {
	if state == nil {
		return nil
	}

	var hints []string
	switch state.Screen {
	case "COMBAT":
		hints = append(hints, buildCombatHints(state)...)
	case "MAP":
		hints = append(hints, buildMapHints(state)...)
	case "SHOP":
		hints = append(hints, buildShopHints(state)...)
	case "REWARD", "CARD_SELECTION":
		hints = append(hints, buildRewardHints(state)...)
	case "REST":
		hints = append(hints, buildRestHints(state)...)
	}

	return hints
}

func buildCombatHints(state *game.StateSnapshot) []string {
	currentHP, ok := fieldInt(state.Run, "currentHp")
	if !ok || currentHP <= 0 {
		return nil
	}

	playerBlock, _ := fieldInt(asMap(state.Combat["player"]), "block")
	totalIncoming := 0
	attackingEnemies := 0
	liveEnemies := 0
	lowestEnemyHP := -1
	for _, enemy := range nestedList(state.Combat, "enemies") {
		if !fieldBool(enemy, "isAlive") {
			continue
		}
		liveEnemies++
		if hp, ok := fieldInt(enemy, "currentHp"); ok && (lowestEnemyHP < 0 || hp < lowestEnemyHP) {
			lowestEnemyHP = hp
		}
		for _, intent := range nestedList(enemy, "intents") {
			damage, ok := fieldInt(intent, "totalDamage")
			if !ok {
				damage, _ = fieldInt(intent, "damage")
			}
			if damage > 0 {
				totalIncoming += damage
				attackingEnemies++
			}
		}
	}

	netIncoming := totalIncoming - playerBlock
	if netIncoming < 0 {
		netIncoming = 0
	}

	var hints []string
	switch {
	case netIncoming >= currentHP:
		hints = append(hints, fmt.Sprintf("Survival alert: visible incoming damage is lethal or near-lethal (%d vs %d HP after %d block). Favor any legal line that keeps the run alive this turn.", netIncoming, currentHP, playerBlock))
	case netIncoming*2 >= currentHP && netIncoming > 0:
		hints = append(hints, fmt.Sprintf("Danger turn: visible incoming damage is heavy (%d vs %d HP after %d block). Bias toward defense and cleaner survival lines before greedy damage.", netIncoming, currentHP, playerBlock))
	case totalIncoming == 0 && liveEnemies > 0:
		hints = append(hints, "Safe turn: no immediate visible damage is coming in. Convert energy into damage or setup instead of panic-blocking.")
	}

	if attackingEnemies >= 2 && currentHP <= 18 {
		hints = append(hints, "Multiple attackers are active while HP is low. Favor lines that reduce incoming damage quickly, even if raw output is lower.")
	}

	if lowestEnemyHP > 0 && lowestEnemyHP <= 12 {
		hints = append(hints, fmt.Sprintf("One enemy is close to death (%d HP). Shortening the fight can be worth more than a speculative setup line.", lowestEnemyHP))
	}

	// Energy efficiency hint.
	if player := asMap(state.Combat["player"]); player != nil {
		if energy, ok := fieldInt(player, "energy"); ok && energy >= 3 {
			hints = append(hints, fmt.Sprintf("High energy available (%d). Spend it all before ending turn — wasted energy is wasted tempo.", energy))
		}
	}

	// Multi-enemy AoE hint.
	if liveEnemies >= 3 {
		hints = append(hints, "Three or more enemies are alive. AoE cards and multi-hit effects gain exceptional value here.")
	} else if liveEnemies == 2 && totalIncoming > 0 {
		hints = append(hints, "Two enemies are dealing damage. Consider AoE or focus-fire on the weaker one to reduce total incoming sooner.")
	}

	// Buff/debuff awareness.
	enemyBuffing := false
	for _, enemy := range nestedList(state.Combat, "enemies") {
		if enemyBuffing {
			break
		}
		if !fieldBool(enemy, "isAlive") {
			continue
		}
		for _, intent := range nestedList(enemy, "intents") {
			intentType := strings.ToLower(fieldString(intent, "type"))
			if strings.Contains(intentType, "buff") || strings.Contains(intentType, "strategic") {
				hints = append(hints, "An enemy is buffing — use this breathing room to deal damage or set up, not panic-block.")
				enemyBuffing = true
				break
			}
		}
	}

	return hints
}

func buildMapHints(state *game.StateSnapshot) []string {
	currentHP, okCurrent := fieldInt(state.Run, "currentHp")
	maxHP, okMax := fieldInt(state.Run, "maxHp")
	if !okCurrent || !okMax || maxHP <= 0 {
		return nil
	}

	var hints []string
	if currentHP*3 <= maxHP {
		hints = append(hints, fmt.Sprintf("Pathing caution: HP is low at %d/%d. Prefer safer routes and recovery over greed unless the value is exceptional.", currentHP, maxHP))
	}

	if gold, ok := fieldInt(state.Run, "gold"); ok && gold >= 120 {
		hints = append(hints, fmt.Sprintf("Economy note: %d gold is enough to justify converting resources soon at a shop or removal opportunity.", gold))
	}

	hints = append(hints, buildDeckQualityHints(state)...)

	return hints
}

func buildShopHints(state *game.StateSnapshot) []string {
	var hints []string
	if cardRemoval := asMap(state.Shop["cardRemoval"]); len(cardRemoval) > 0 && fieldBool(cardRemoval, "available") && fieldBool(cardRemoval, "enoughGold") {
		if price, ok := fieldInt(cardRemoval, "price"); ok {
			hints = append(hints, fmt.Sprintf("Shop priority: card removal is available and affordable at %d gold. Compare other purchases against the value of deck thinning.", price))
		} else {
			hints = append(hints, "Shop priority: card removal is available and affordable. Compare other purchases against the value of deck thinning.")
		}
	}
	return hints
}

func buildRewardHints(state *game.StateSnapshot) []string {
	var hints []string

	currentHP, okCurrent := fieldInt(state.Run, "currentHp")
	maxHP, okMax := fieldInt(state.Run, "maxHp")
	if okCurrent && okMax && maxHP > 0 && currentHP*3 <= maxHP {
		hints = append(hints, fmt.Sprintf("Reward bias: HP is low at %d/%d. Prefer cards and choices that stabilize survival, consistency, or immediate tempo over speculative greed.", currentHP, maxHP))
	}

	hints = append(hints, buildDeckQualityHints(state)...)
	return hints
}

func buildDeckQualityHints(state *game.StateSnapshot) []string {
	if state == nil || state.Run == nil {
		return nil
	}
	raw, ok := state.Run["deck"]
	if !ok {
		return nil
	}
	items, ok := raw.([]interface{})
	if !ok || len(items) == 0 {
		return nil
	}

	deckSize := len(items)
	var hints []string

	switch {
	case deckSize >= 35:
		hints = append(hints, fmt.Sprintf("Deck quality: %d cards is bloated — skip weak card picks and look for removal to improve draw consistency.", deckSize))
	case deckSize <= 15:
		hints = append(hints, fmt.Sprintf("Deck quality: %d cards is lean — each add/remove has high impact, be selective.", deckSize))
	}

	// Count attack vs. defense cards by type field if available.
	attacks, defenses := 0, 0
	for _, item := range items {
		card, ok := item.(map[string]any)
		if !ok {
			continue
		}
		cardType := strings.ToLower(fieldString(card, "type"))
		switch {
		case strings.Contains(cardType, "attack"):
			attacks++
		case strings.Contains(cardType, "skill"), strings.Contains(cardType, "defense"):
			defenses++
		}
	}
	if attacks > 0 && defenses > 0 {
		if attacks > defenses*3 {
			hints = append(hints, fmt.Sprintf("Deck balance: heavy on attacks (%d attack vs %d defense) — consider adding defensive cards.", attacks, defenses))
		} else if defenses > attacks*3 {
			hints = append(hints, fmt.Sprintf("Deck balance: heavy on defense (%d defense vs %d attack) — consider adding damage or scaling.", defenses, attacks))
		}
	}

	return hints
}

func buildRestHints(state *game.StateSnapshot) []string {
	currentHP, okCurrent := fieldInt(state.Run, "currentHp")
	maxHP, okMax := fieldInt(state.Run, "maxHp")
	if !okCurrent || !okMax || maxHP <= 0 {
		return nil
	}

	var hints []string
	hpPercent := currentHP * 100 / maxHP
	if hpPercent <= 40 {
		hints = append(hints, fmt.Sprintf("HP is low (%d/%d). Rest to heal unless an upgrade is critical for the next boss.", currentHP, maxHP))
	} else if hpPercent >= 80 {
		hints = append(hints, fmt.Sprintf("HP is healthy (%d/%d). Prefer smithing an upgrade over resting.", currentHP, maxHP))
	}
	return hints
}

func TacticalHintsBlock(state *game.StateSnapshot) string {
	hints := BuildTacticalHints(state)
	if len(hints) == 0 {
		return ""
	}

	return "Tactical hints:\n- " + strings.Join(hints, "\n- ")
}

func asMap(value any) map[string]any {
	result, _ := value.(map[string]any)
	return result
}
