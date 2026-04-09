package agentruntime

import (
	"fmt"
	"strings"

	"spire2mind/internal/game"
	"spire2mind/internal/i18n"
)

func BuildTacticalHints(state *game.StateSnapshot) []string {
	return BuildTacticalHintsForLanguage(state, i18n.LanguageEnglish)
}

func BuildTacticalHintsForLanguage(state *game.StateSnapshot, language i18n.Language) []string {
	if state == nil {
		return nil
	}

	loc := i18n.New(language)
	var hints []string
	switch state.Screen {
	case "COMBAT":
		hints = append(hints, buildCombatHints(state, loc)...)
	case "MAP":
		hints = append(hints, buildMapHints(state, loc)...)
	case "SHOP":
		hints = append(hints, buildShopHints(state, loc)...)
	case "REWARD", "CARD_SELECTION":
		hints = append(hints, buildRewardHints(state, loc)...)
	case "REST":
		hints = append(hints, buildRestHints(state, loc)...)
	}

	return hints
}

func buildCombatHints(state *game.StateSnapshot, loc i18n.Localizer) []string {
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
		hints = append(hints, loc.Paragraph(
			fmt.Sprintf("Survival alert: visible incoming damage is lethal or near-lethal (%d vs %d HP after %d block). Favor any legal line that keeps the run alive this turn.", netIncoming, currentHP, playerBlock),
			fmt.Sprintf("生存警报：可见伤害在当前格挡后已经致命或接近致命（%d 对 %d 点生命，已有 %d 点格挡）。这一回合优先选择任何能活下来的合法线路。", netIncoming, currentHP, playerBlock),
		))
	case netIncoming*2 >= currentHP && netIncoming > 0:
		hints = append(hints, loc.Paragraph(
			fmt.Sprintf("Danger turn: visible incoming damage is heavy (%d vs %d HP after %d block). Bias toward defense and cleaner survival lines before greedy damage.", netIncoming, currentHP, playerBlock),
			fmt.Sprintf("危险回合：可见伤害很高（%d 对 %d 点生命，已有 %d 点格挡）。先偏向防御和稳妥活命，再考虑贪输出。", netIncoming, currentHP, playerBlock),
		))
	case totalIncoming == 0 && liveEnemies > 0:
		hints = append(hints, loc.Paragraph(
			"Safe turn: no immediate visible damage is coming in. Convert energy into damage or setup instead of panic-blocking.",
			"安全回合：当前没有立刻可见的 incoming damage。把能量换成输出或铺垫，不要空恐慌格挡。",
		))
	}

	if attackingEnemies >= 2 && currentHP <= 18 {
		hints = append(hints, loc.Paragraph(
			"Multiple attackers are active while HP is low. Favor lines that reduce incoming damage quickly, even if raw output is lower.",
			"低血量下同时有多个敌人在攻击。优先快速降低 incoming damage，就算总输出略低也值得。",
		))
	}

	if lowestEnemyHP > 0 && lowestEnemyHP <= 12 {
		hints = append(hints, loc.Paragraph(
			fmt.Sprintf("One enemy is close to death (%d HP). Shortening the fight can be worth more than a speculative setup line.", lowestEnemyHP),
			fmt.Sprintf("有一个敌人快死了（%d HP）。尽快减员通常比继续贪铺垫更值。", lowestEnemyHP),
		))
	}

	if player := asMap(state.Combat["player"]); player != nil {
		if energy, ok := fieldInt(player, "energy"); ok && energy >= 3 {
			hints = append(hints, loc.Paragraph(
				fmt.Sprintf("High energy available (%d). Spend it all before ending turn; wasted energy is wasted tempo.", energy),
				fmt.Sprintf("当前能量较高（%d）。结束回合前尽量把能量花完，浪费能量就是浪费节奏。", energy),
			))
		}
	}

	if liveEnemies >= 3 {
		hints = append(hints, loc.Paragraph(
			"Three or more enemies are alive. AoE cards and multi-hit effects gain exceptional value here.",
			"场上有三个或更多活着的敌人。AoE 和多段伤害在这里价值很高。",
		))
	} else if liveEnemies == 2 && totalIncoming > 0 {
		hints = append(hints, loc.Paragraph(
			"Two enemies are dealing damage. Consider AoE or focus-fire on the weaker one to reduce total incoming sooner.",
			"有两个敌人在造成伤害。可以考虑 AoE，或者先集火更脆的一只，尽快降低总 incoming。",
		))
	}

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
				hints = append(hints, loc.Paragraph(
					"An enemy is buffing; use this breathing room to deal damage or set up, not panic-block.",
					"有敌人在上 buff。利用这口气做输出或铺垫，不要白白慌张格挡。",
				))
				enemyBuffing = true
				break
			}
		}
	}

	return hints
}

func buildMapHints(state *game.StateSnapshot, loc i18n.Localizer) []string {
	currentHP, okCurrent := fieldInt(state.Run, "currentHp")
	maxHP, okMax := fieldInt(state.Run, "maxHp")
	if !okCurrent || !okMax || maxHP <= 0 {
		return nil
	}

	var hints []string
	if currentHP*3 <= maxHP {
		hints = append(hints, loc.Paragraph(
			fmt.Sprintf("Pathing caution: HP is low at %d/%d. Prefer safer routes and recovery over greed unless the value is exceptional.", currentHP, maxHP),
			fmt.Sprintf("路线警戒：当前血量只有 %d/%d。除非收益特别高，否则优先更稳的路线和恢复，而不是贪。", currentHP, maxHP),
		))
		hints = append(hints, loc.Paragraph(
			"Low-HP route rule: avoid elite chains unless the deck is already ahead of curve or a rest site is immediately available.",
			"低血路线规则：除非当前战力明显超模，或者立刻就能接火堆，否则尽量别连续撞 elite。",
		))
	}

	if gold, ok := fieldInt(state.Run, "gold"); ok && gold >= 120 {
		hints = append(hints, loc.Paragraph(
			fmt.Sprintf("Economy note: %d gold is enough to justify converting resources soon at a shop or removal opportunity.", gold),
			fmt.Sprintf("经济提示：你现在有 %d 金币，已经值得尽快找商店或移除机会把资源换成强度。", gold),
		))
		if currentHP*2 <= maxHP {
			hints = append(hints, loc.Paragraph(
				"With low HP and high gold, prioritize shops and removal over speculative extra fights.",
				"低血又高金币时，优先找商店和移除，不要再去多打高波动战斗。",
			))
		}
	}

	hints = append(hints, buildDeckQualityHints(state, loc)...)

	return hints
}

func buildShopHints(state *game.StateSnapshot, loc i18n.Localizer) []string {
	var hints []string
	if cardRemoval := asMap(state.Shop["cardRemoval"]); len(cardRemoval) > 0 && fieldBool(cardRemoval, "available") && fieldBool(cardRemoval, "enoughGold") {
		if price, ok := fieldInt(cardRemoval, "price"); ok {
			hints = append(hints, loc.Paragraph(
				fmt.Sprintf("Shop priority: card removal is available and affordable at %d gold. Compare other purchases against the value of deck thinning.", price),
				fmt.Sprintf("商店优先级：移除卡牌当前可买，价格是 %d 金币。先拿它和其他购买做比较，别低估精简牌组的价值。", price),
			))
		} else {
			hints = append(hints, loc.Paragraph(
				"Shop priority: card removal is available and affordable. Compare other purchases against the value of deck thinning.",
				"商店优先级：移除卡牌当前可买。先拿它和其他购买做比较，别低估精简牌组的价值。",
			))
		}
	}
	if gold, ok := fieldInt(state.Run, "gold"); ok && gold >= 140 {
		hints = append(hints, loc.Paragraph(
			"Do not leave this shop carrying a large pile of gold unless every affordable option is low impact.",
			"除非所有买得起的东西都很低价值，否则不要带着大把金币离开这个商店。",
		))
	}
	return hints
}

func buildRewardHints(state *game.StateSnapshot, loc i18n.Localizer) []string {
	var hints []string

	currentHP, okCurrent := fieldInt(state.Run, "currentHp")
	maxHP, okMax := fieldInt(state.Run, "maxHp")
	if okCurrent && okMax && maxHP > 0 && currentHP*3 <= maxHP {
		hints = append(hints, loc.Paragraph(
			fmt.Sprintf("Reward bias: HP is low at %d/%d. Prefer cards and choices that stabilize survival, consistency, or immediate tempo over speculative greed.", currentHP, maxHP),
			fmt.Sprintf("奖励偏置：当前血量只有 %d/%d。优先能稳住生存、提高稳定性或立刻带来节奏的选择，不要贪远期。", currentHP, maxHP),
		))
	}
	if floor := fieldIntValue(state.Run, "floor"); floor > 0 && floor <= 12 {
		hints = append(hints, loc.Paragraph(
			"Early-floor reward rule: prioritize immediate combat strength and reliable block before niche scaling.",
			"前中期奖励规则：先拿立刻能提升战斗力和稳定格挡的东西，再考虑偏门成长。",
		))
	}

	hints = append(hints, buildDeckQualityHints(state, loc)...)
	return hints
}

func buildDeckQualityHints(state *game.StateSnapshot, loc i18n.Localizer) []string {
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
		hints = append(hints, loc.Paragraph(
			fmt.Sprintf("Deck quality: %d cards is bloated; skip weak card picks and look for removal to improve draw consistency.", deckSize),
			fmt.Sprintf("牌组质量：现在有 %d 张牌，已经偏臃肿。弱牌尽量不拿，优先找移除提升抽牌稳定性。", deckSize),
		))
	case deckSize <= 15:
		hints = append(hints, loc.Paragraph(
			fmt.Sprintf("Deck quality: %d cards is lean; each add/remove has high impact, be selective.", deckSize),
			fmt.Sprintf("牌组质量：现在只有 %d 张牌，增删每一张的影响都很大，要更谨慎。", deckSize),
		))
	}

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
			hints = append(hints, loc.Paragraph(
				fmt.Sprintf("Deck balance: heavy on attacks (%d attack vs %d defense); consider adding defensive cards.", attacks, defenses),
				fmt.Sprintf("牌组平衡：攻击牌明显偏多（%d 攻击对 %d 防御），可以考虑补一些防御。", attacks, defenses),
			))
		} else if defenses > attacks*3 {
			hints = append(hints, loc.Paragraph(
				fmt.Sprintf("Deck balance: heavy on defense (%d defense vs %d attack); consider adding damage or scaling.", defenses, attacks),
				fmt.Sprintf("牌组平衡：防御牌明显偏多（%d 防御对 %d 攻击），可以考虑补输出或成长。", defenses, attacks),
			))
		}
	}

	return hints
}

func buildRestHints(state *game.StateSnapshot, loc i18n.Localizer) []string {
	currentHP, okCurrent := fieldInt(state.Run, "currentHp")
	maxHP, okMax := fieldInt(state.Run, "maxHp")
	if !okCurrent || !okMax || maxHP <= 0 {
		return nil
	}

	var hints []string
	hpPercent := currentHP * 100 / maxHP
	if hpPercent <= 40 {
		hints = append(hints, loc.Paragraph(
			fmt.Sprintf("HP is low (%d/%d). Rest to heal unless an upgrade is critical for the next boss.", currentHP, maxHP),
			fmt.Sprintf("血量偏低（%d/%d）。除非某个升级对接下来的关键战斗极其重要，否则优先休息回血。", currentHP, maxHP),
		))
	} else if hpPercent >= 80 {
		hints = append(hints, loc.Paragraph(
			fmt.Sprintf("HP is healthy (%d/%d). Prefer smithing an upgrade over resting.", currentHP, maxHP),
			fmt.Sprintf("血量很健康（%d/%d）。优先考虑锻造升级，而不是休息。", currentHP, maxHP),
		))
	}
	return hints
}

func TacticalHintsBlock(state *game.StateSnapshot) string {
	return TacticalHintsBlockForLanguage(state, i18n.LanguageEnglish)
}

func TacticalHintsBlockForLanguage(state *game.StateSnapshot, language i18n.Language) string {
	hints := BuildTacticalHintsForLanguage(state, language)
	if len(hints) == 0 {
		return ""
	}

	loc := i18n.New(language)
	return loc.Label("Tactical hints", "战术提示") + ":\n- " + strings.Join(hints, "\n- ")
}

func asMap(value any) map[string]any {
	result, _ := value.(map[string]any)
	return result
}
