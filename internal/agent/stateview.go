package agentruntime

import (
	"fmt"
	"strings"

	"spire2mind/internal/game"
	"spire2mind/internal/i18n"
)

func StateSummaryLines(state *game.StateSnapshot) []string {
	return StateSummaryLinesFor(state, i18n.LanguageEnglish)
}

func StateSummaryLinesFor(state *game.StateSnapshot, language i18n.Language) []string {
	loc := i18n.New(language)
	if state == nil {
		return []string{
			fmt.Sprintf("- %s: `-`", loc.Label("Screen", "界面")),
			fmt.Sprintf("- %s: `-`", loc.Label("Run", "对局")),
			fmt.Sprintf("- %s: `-`", loc.Label("Actions", "动作")),
		}
	}

	lines := []string{
		fmt.Sprintf("- %s: `%s`", loc.Label("Screen", "界面"), valueOrDash(state.Screen)),
		fmt.Sprintf("- %s: `%s`", loc.Label("Run", "对局"), valueOrDash(state.RunID)),
		fmt.Sprintf("- %s: `%s`", loc.Label("Actions", "动作"), valueOrDash(strings.Join(state.AvailableActions, ", "))),
	}

	if headline := fieldString(state.AgentView, "headline"); headline != "" {
		lines = append(lines, fmt.Sprintf("- %s: %s", loc.Label("Headline", "摘要"), headline))
	}
	if floor, ok := fieldInt(state.Run, "floor"); ok {
		lines = append(lines, fmt.Sprintf("- %s: `%d`", loc.Label("Floor", "层数"), floor))
	}
	if currentHP, okCurrent := fieldInt(state.Run, "currentHp"); okCurrent {
		if maxHP, okMax := fieldInt(state.Run, "maxHp"); okMax {
			lines = append(lines, fmt.Sprintf("- %s: `%d/%d`", loc.Label("HP", "生命"), currentHP, maxHP))
		}
	}
	if gold, ok := fieldInt(state.Run, "gold"); ok {
		lines = append(lines, fmt.Sprintf("- %s: `%d`", loc.Label("Gold", "金币"), gold))
	}
	if state.Turn != nil {
		lines = append(lines, fmt.Sprintf("- %s: `%d`", loc.Label("Turn", "回合"), *state.Turn))
	}

	return lines
}

func StateDetailLines(state *game.StateSnapshot, maxItems int) []string {
	return StateDetailLinesFor(state, maxItems, i18n.LanguageEnglish)
}

func StateDetailLinesFor(state *game.StateSnapshot, maxItems int, language i18n.Language) []string {
	loc := i18n.New(language)
	if state == nil {
		return []string{"- -"}
	}
	if maxItems <= 0 {
		maxItems = 5
	}

	var lines []string
	switch state.Screen {
	case "COMBAT":
		lines = append(lines, combatDetailLines(state, maxItems, loc)...)
	case "REWARD":
		lines = append(lines, rewardDetailLines(state, maxItems, loc)...)
	case "MAP":
		lines = append(lines, mapDetailLines(state, maxItems, loc)...)
	case "EVENT":
		lines = append(lines, eventDetailLines(state, maxItems, loc)...)
	case "SHOP":
		lines = append(lines, shopDetailLines(state, maxItems, loc)...)
	case "REST":
		lines = append(lines, restDetailLines(state, maxItems, loc)...)
	case "CHEST":
		lines = append(lines, chestDetailLines(state, maxItems, loc)...)
	case "CARD_SELECTION":
		lines = append(lines, selectionDetailLines(state, maxItems, loc)...)
	case "GAME_OVER":
		lines = append(lines, gameOverDetailLines(state, loc)...)
	}

	if len(lines) == 0 {
		return []string{"- -"}
	}

	return lines
}

func combatDetailLines(state *game.StateSnapshot, maxItems int, loc i18n.Localizer) []string {
	lines := []string{}
	if player := state.Combat["player"]; player != nil {
		playerMap, _ := player.(map[string]any)
		if playerMap != nil {
			energy, _ := fieldInt(playerMap, "energy")
			block, _ := fieldInt(playerMap, "block")
			stars, _ := fieldInt(playerMap, "stars")
			lines = append(lines, fmt.Sprintf(
				"- %s: %s `%d`, %s `%d`, %s `%d`",
				loc.Label("Player", "玩家"),
				loc.Label("energy", "能量"), energy,
				loc.Label("block", "格挡"), block,
				loc.Label("stars", "星能"), stars,
			))
		}
	}

	enemies := nestedList(state.Combat, "enemies")
	for i, enemy := range enemies {
		if i >= maxItems {
			lines = append(lines, fmt.Sprintf("- %s: ... %d %s", loc.Label("Enemies", "敌人"), len(enemies)-maxItems, loc.Label("more", "更多")))
			break
		}
		name := fieldString(enemy, "name")
		currentHP, _ := fieldInt(enemy, "currentHp")
		maxHP, _ := fieldInt(enemy, "maxHp")
		block, _ := fieldInt(enemy, "block")
		intent := "-"
		if intents := nestedList(enemy, "intents"); len(intents) > 0 {
			intent = fieldString(intents[0], "intentType")
			if label := fieldString(intents[0], "label"); label != "" && label != intent {
				intent = intent + " / " + label
			}
		}
		lines = append(lines, fmt.Sprintf(
			"- %s %d: %s `%d/%d` %s, `%d` %s, %s `%s`",
			loc.Label("Enemy", "敌人"), i,
			fallbackID(name, fieldString(enemy, "enemyId")),
			currentHP, maxHP, loc.Label("HP", "生命"),
			block, loc.Label("block", "格挡"),
			loc.Label("intent", "意图"), valueOrDash(intent),
		))
	}

	hand := nestedList(state.Combat, "hand")
	for i, card := range hand {
		if i >= maxItems {
			lines = append(lines, fmt.Sprintf("- %s: ... %d %s", loc.Label("Hand", "手牌"), len(hand)-maxItems, loc.Label("more cards", "更多牌")))
			break
		}
		index, _ := fieldInt(card, "index")
		cost, _ := fieldInt(card, "energyCost")
		playable := fieldBool(card, "playable")
		requiresTarget := cardRequiresTarget(state, card)
		lines = append(lines, fmt.Sprintf(
			"- %s %d: [%d] %s %s `%d` %s `%t` %s `%t`",
			loc.Label("Hand", "手牌"), i, index,
			fallbackID(fieldString(card, "name"), fieldString(card, "cardId")),
			loc.Label("cost", "费用"), cost,
			loc.Label("playable", "可打出"), playable,
			loc.Label("target", "需目标"), requiresTarget,
		))
	}

	return lines
}

func rewardDetailLines(state *game.StateSnapshot, maxItems int, loc i18n.Localizer) []string {
	lines := []string{}
	if phase := fieldString(state.Reward, "phase"); phase != "" {
		source := fieldString(state.Reward, "sourceScreen")
		if source != "" {
			lines = append(lines, fmt.Sprintf("- %s: `%s` (%s `%s`)", loc.Label("Reward phase", "奖励阶段"), phase, loc.Label("source", "来源"), source))
		} else {
			lines = append(lines, fmt.Sprintf("- %s: `%s`", loc.Label("Reward phase", "奖励阶段"), phase))
		}
	}
	rewards := nestedList(state.Reward, "rewards")
	for i, reward := range rewards {
		if i >= maxItems {
			lines = append(lines, fmt.Sprintf("- %s: ... %d %s", loc.Label("Rewards", "奖励"), len(rewards)-maxItems, loc.Label("more", "更多")))
			break
		}
		index, _ := fieldInt(reward, "index")
		lines = append(lines, fmt.Sprintf("- %s %d: [%d] `%s` %s `%t`", loc.Label("Reward", "奖励"), i, index, valueOrDash(fieldString(reward, "rewardType")), loc.Label("claimable", "可领取"), fieldBool(reward, "claimable")))
	}

	cardOptions := nestedList(state.Reward, "cardOptions")
	for i, card := range cardOptions {
		if i >= maxItems {
			lines = append(lines, fmt.Sprintf("- %s: ... %d %s", loc.Label("Card options", "卡牌选项"), len(cardOptions)-maxItems, loc.Label("more", "更多")))
			break
		}
		index, _ := fieldInt(card, "index")
		lines = append(lines, fmt.Sprintf("- %s %d: [%d] %s", loc.Label("Card option", "卡牌选项"), i, index, fallbackID(fieldString(card, "name"), fieldString(card, "cardId"))))
	}

	return lines
}

func mapDetailLines(state *game.StateSnapshot, maxItems int, loc i18n.Localizer) []string {
	lines := []string{}
	if currentNode := state.Map["currentNode"]; currentNode != nil {
		current, _ := currentNode.(map[string]any)
		if current != nil {
			row, _ := fieldInt(current, "row")
			col, _ := fieldInt(current, "col")
			nodeType := fieldString(current, "nodeType")
			lines = append(lines, fmt.Sprintf("- %s: %s `%d`, %s `%d`, %s `%s`", loc.Label("Current node", "当前节点"), loc.Label("row", "行"), row, loc.Label("col", "列"), col, loc.Label("type", "类型"), valueOrDash(nodeType)))
		}
	}

	nodes := nestedList(state.Map, "availableNodes")
	for i, node := range nodes {
		if i >= maxItems {
			lines = append(lines, fmt.Sprintf("- %s: ... %d %s", loc.Label("Nodes", "节点"), len(nodes)-maxItems, loc.Label("more", "更多")))
			break
		}
		index, _ := fieldInt(node, "index")
		row, _ := fieldInt(node, "row")
		col, _ := fieldInt(node, "col")
		nodeType := fieldString(node, "nodeType")
		lines = append(lines, fmt.Sprintf("- %s %d: [%d] %s `%d`, %s `%d`, %s `%s`", loc.Label("Node", "节点"), i, index, loc.Label("row", "行"), row, loc.Label("col", "列"), col, loc.Label("type", "类型"), valueOrDash(nodeType)))
	}

	return lines
}

func eventDetailLines(state *game.StateSnapshot, maxItems int, loc i18n.Localizer) []string {
	lines := []string{}
	if title := fieldString(state.Event, "title"); title != "" {
		lines = append(lines, fmt.Sprintf("- %s: %s", loc.Label("Title", "标题"), title))
	}
	options := nestedList(state.Event, "options")
	for i, option := range options {
		if i >= maxItems {
			lines = append(lines, fmt.Sprintf("- %s: ... %d %s", loc.Label("Options", "选项"), len(options)-maxItems, loc.Label("more", "更多")))
			break
		}
		index, _ := fieldInt(option, "index")
		label := fallbackID(fieldString(option, "label"), fieldString(option, "title"))
		lines = append(lines, fmt.Sprintf("- %s %d: [%d] %s %s `%t`", loc.Label("Option", "选项"), i, index, valueOrDash(label), loc.Label("locked", "锁定"), fieldBool(option, "isLocked")))
	}
	return lines
}

func shopDetailLines(state *game.StateSnapshot, maxItems int, loc i18n.Localizer) []string {
	lines := []string{}
	addItems := func(label string, items []map[string]any) {
		for i, item := range items {
			if i >= maxItems {
				lines = append(lines, fmt.Sprintf("- %s: ... %d %s", label, len(items)-maxItems, loc.Label("more", "更多")))
				break
			}
			index, _ := fieldInt(item, "index")
			price, _ := fieldInt(item, "price")
			name := fallbackID(fieldString(item, "name"), fieldString(item, "cardId"))
			if name == "" {
				name = fallbackID(fieldString(item, "relicId"), fieldString(item, "potionId"))
			}
			lines = append(lines, fmt.Sprintf("- %s %d: [%d] %s %s `%d` %s `%t`", label, i, index, valueOrDash(name), loc.Label("price", "价格"), price, loc.Label("affordable", "买得起"), fieldBool(item, "enoughGold")))
		}
	}
	addItems(loc.Label("Card", "卡牌"), nestedList(state.Shop, "cards"))
	addItems(loc.Label("Relic", "遗物"), nestedList(state.Shop, "relics"))
	addItems(loc.Label("Potion", "药水"), nestedList(state.Shop, "potions"))
	return lines
}

func restDetailLines(state *game.StateSnapshot, maxItems int, loc i18n.Localizer) []string {
	options := nestedList(state.Rest, "options")
	lines := []string{}
	for i, option := range options {
		if i >= maxItems {
			lines = append(lines, fmt.Sprintf("- %s: ... %d %s", loc.Label("Rest options", "营火选项"), len(options)-maxItems, loc.Label("more", "更多")))
			break
		}
		index, _ := fieldInt(option, "index")
		lines = append(lines, fmt.Sprintf("- %s %d: [%d] `%s` %s `%t`", loc.Label("Rest option", "营火选项"), i, index, valueOrDash(fieldString(option, "optionType")), loc.Label("enabled", "可用"), fieldBool(option, "isEnabled")))
	}
	return lines
}

func chestDetailLines(state *game.StateSnapshot, maxItems int, loc i18n.Localizer) []string {
	lines := []string{}
	if fieldBool(state.Chest, "isOpened") {
		lines = append(lines, "- "+loc.Label("Chest: already opened", "宝箱：已打开"))
	}
	relics := nestedList(state.Chest, "relicOptions")
	for i, relic := range relics {
		if i >= maxItems {
			lines = append(lines, fmt.Sprintf("- %s: ... %d %s", loc.Label("Relics", "遗物"), len(relics)-maxItems, loc.Label("more", "更多")))
			break
		}
		index, _ := fieldInt(relic, "index")
		lines = append(lines, fmt.Sprintf("- %s %d: [%d] `%s`", loc.Label("Relic", "遗物"), i, index, valueOrDash(fallbackID(fieldString(relic, "name"), fieldString(relic, "relicId")))))
	}
	return lines
}

func selectionDetailLines(state *game.StateSnapshot, maxItems int, loc i18n.Localizer) []string {
	lines := []string{}
	if kind := fieldString(state.Selection, "kind"); kind != "" {
		source := fieldString(state.Selection, "sourceScreen")
		if source != "" {
			lines = append(lines, fmt.Sprintf("- %s: `%s` (%s `%s`)", loc.Label("Selection context", "选择上下文"), kind, loc.Label("source", "来源"), source))
		} else {
			lines = append(lines, fmt.Sprintf("- %s: `%s`", loc.Label("Selection context", "选择上下文"), kind))
		}
	}
	cards := nestedList(state.Selection, "cards")
	for i, card := range cards {
		if i >= maxItems {
			lines = append(lines, fmt.Sprintf("- %s: ... %d %s", loc.Label("Selection cards", "选牌"), len(cards)-maxItems, loc.Label("more", "更多")))
			break
		}
		index, _ := fieldInt(card, "index")
		lines = append(lines, fmt.Sprintf("- %s %d: [%d] %s", loc.Label("Selection", "选择"), i, index, valueOrDash(fallbackID(fieldString(card, "name"), fieldString(card, "cardId")))))
	}
	return lines
}

func gameOverDetailLines(state *game.StateSnapshot, loc i18n.Localizer) []string {
	lines := []string{}
	if fieldBool(state.GameOver, "isVictory") {
		lines = append(lines, "- "+loc.Label("Outcome: victory", "结果：胜利"))
	} else {
		lines = append(lines, "- "+loc.Label("Outcome: defeat", "结果：失败"))
	}
	if floor, ok := fieldInt(state.GameOver, "floor"); ok {
		lines = append(lines, fmt.Sprintf("- %s: `%d`", loc.Label("Final floor", "最终层数"), floor))
	}
	if enemy := fieldString(state.GameOver, "killedBy"); enemy != "" {
		lines = append(lines, fmt.Sprintf("- %s: %s", loc.Label("Killed by", "击败者"), enemy))
	}
	return lines
}

func fallbackID(primary string, fallback string) string {
	primary = strings.TrimSpace(primary)
	if primary != "" {
		return primary
	}
	return strings.TrimSpace(fallback)
}
