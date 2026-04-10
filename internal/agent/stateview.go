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

	if state.AgentView != nil && state.AgentView.Headline != "" {
		lines = append(lines, fmt.Sprintf("- %s: %s", loc.Label("Headline", "摘要"), state.AgentView.Headline))
	}
	if state.Run != nil {
		lines = append(lines, fmt.Sprintf("- %s: `%d`", loc.Label("Floor", "层数"), state.Run.Floor))
		if state.Run.MaxHp > 0 {
			lines = append(lines, fmt.Sprintf("- %s: `%d/%d`", loc.Label("HP", "生命"), state.Run.CurrentHp, state.Run.MaxHp))
		}
		lines = append(lines, fmt.Sprintf("- %s: `%d`", loc.Label("Gold", "金币"), state.Run.Gold))
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
	if state == nil || state.Combat == nil {
		return nil
	}
	lines := []string{}
	player := state.Combat.Player
	lines = append(lines, fmt.Sprintf(
		"- %s: %s `%d`, %s `%d`, %s `%d`",
		loc.Label("Player", "玩家"),
		loc.Label("energy", "能量"), player.Energy,
		loc.Label("block", "格挡"), player.Block,
		loc.Label("stars", "星能"), player.Stars,
	))

	enemies := state.Combat.Enemies
	for i, enemy := range enemies {
		if i >= maxItems {
			lines = append(lines, fmt.Sprintf("- %s: ... %d %s", loc.Label("Enemies", "敌人"), len(enemies)-maxItems, loc.Label("more", "更多")))
			break
		}
		intent := "-"
		if len(enemy.Intents) > 0 {
			intent = enemy.Intents[0].IntentType
			if label := enemy.Intents[0].Label; label != "" && label != intent {
				intent = intent + " / " + label
			}
		}
		lines = append(lines, fmt.Sprintf(
			"- %s %d: %s `%d/%d` %s, `%d` %s, %s `%s`",
			loc.Label("Enemy", "敌人"), i,
			fallbackID(enemy.Name, enemy.EnemyID),
			enemy.CurrentHp, enemy.MaxHp, loc.Label("HP", "生命"),
			enemy.Block, loc.Label("block", "格挡"),
			loc.Label("intent", "意图"), valueOrDash(intent),
		))
	}

	hand := state.Combat.Hand
	for i, card := range hand {
		if i >= maxItems {
			lines = append(lines, fmt.Sprintf("- %s: ... %d %s", loc.Label("Hand", "手牌"), len(hand)-maxItems, loc.Label("more cards", "更多牌")))
			break
		}
		cost := 0
		if card.EnergyCost != nil {
			cost = *card.EnergyCost
		}
		lines = append(lines, fmt.Sprintf(
			"- %s %d: [%d] %s %s `%d` %s `%t` %s `%t`",
			loc.Label("Hand", "手牌"), i, card.Index,
			fallbackID(card.Name, card.CardID),
			loc.Label("cost", "费用"), cost,
			loc.Label("playable", "可打出"), card.Playable,
			loc.Label("target", "需目标"), cardRequiresTarget(state, card),
		))
	}

	return lines
}

func rewardDetailLines(state *game.StateSnapshot, maxItems int, loc i18n.Localizer) []string {
	if state == nil || state.Reward == nil {
		return nil
	}
	lines := []string{}
	if phase := state.Reward.Phase; phase != "" {
		source := state.Reward.SourceScreen
		if source != "" {
			lines = append(lines, fmt.Sprintf("- %s: `%s` (%s `%s`)", loc.Label("Reward phase", "奖励阶段"), phase, loc.Label("source", "来源"), source))
		} else {
			lines = append(lines, fmt.Sprintf("- %s: `%s`", loc.Label("Reward phase", "奖励阶段"), phase))
		}
	}
	for i, reward := range state.Reward.Rewards {
		if i >= maxItems {
			lines = append(lines, fmt.Sprintf("- %s: ... %d %s", loc.Label("Rewards", "奖励"), len(state.Reward.Rewards)-maxItems, loc.Label("more", "更多")))
			break
		}
		lines = append(lines, fmt.Sprintf("- %s %d: [%d] `%s` %s `%t`", loc.Label("Reward", "奖励"), i, reward.Index, valueOrDash(reward.RewardType), loc.Label("claimable", "可领取"), reward.Claimable))
	}

	for i, card := range state.Reward.CardOptions {
		if i >= maxItems {
			lines = append(lines, fmt.Sprintf("- %s: ... %d %s", loc.Label("Card options", "卡牌选项"), len(state.Reward.CardOptions)-maxItems, loc.Label("more", "更多")))
			break
		}
		lines = append(lines, fmt.Sprintf("- %s %d: [%d] %s", loc.Label("Card option", "卡牌选项"), i, card.Index, fallbackID(card.Name, card.CardID)))
	}

	return lines
}

func mapDetailLines(state *game.StateSnapshot, maxItems int, loc i18n.Localizer) []string {
	if state == nil || state.Map == nil {
		return nil
	}
	lines := []string{}
	if cn := state.Map.CurrentNode; cn != nil {
		row, col := 0, 0
		if cn.Row != nil {
			row = *cn.Row
		}
		if cn.Col != nil {
			col = *cn.Col
		}
		lines = append(lines, fmt.Sprintf("- %s: %s `%d`, %s `%d`, %s `%s`", loc.Label("Current node", "当前节点"), loc.Label("row", "行"), row, loc.Label("col", "列"), col, loc.Label("type", "类型"), valueOrDash(cn.NodeType)))
	}

	nodes := state.Map.AvailableNodes
	for i, node := range nodes {
		if i >= maxItems {
			lines = append(lines, fmt.Sprintf("- %s: ... %d %s", loc.Label("Nodes", "节点"), len(nodes)-maxItems, loc.Label("more", "更多")))
			break
		}
		row, col := 0, 0
		if node.Row != nil {
			row = *node.Row
		}
		if node.Col != nil {
			col = *node.Col
		}
		lines = append(lines, fmt.Sprintf("- %s %d: [%d] %s `%d`, %s `%d`, %s `%s`", loc.Label("Node", "节点"), i, node.Index, loc.Label("row", "行"), row, loc.Label("col", "列"), col, loc.Label("type", "类型"), valueOrDash(node.NodeType)))
	}

	return lines
}

func eventDetailLines(state *game.StateSnapshot, maxItems int, loc i18n.Localizer) []string {
	if state == nil || state.Event == nil {
		return nil
	}
	lines := []string{}
	if title := state.Event.Title; title != "" {
		lines = append(lines, fmt.Sprintf("- %s: %s", loc.Label("Title", "标题"), title))
	}
	for i, option := range state.Event.Options {
		if i >= maxItems {
			lines = append(lines, fmt.Sprintf("- %s: ... %d %s", loc.Label("Options", "选项"), len(state.Event.Options)-maxItems, loc.Label("more", "更多")))
			break
		}
		label := fallbackID(option.Title, "")
		lines = append(lines, fmt.Sprintf("- %s %d: [%d] %s %s `%t`", loc.Label("Option", "选项"), i, option.Index, valueOrDash(label), loc.Label("locked", "锁定"), option.IsLocked))
	}
	return lines
}

func shopDetailLines(state *game.StateSnapshot, maxItems int, loc i18n.Localizer) []string {
	lines := []string{}
	if !shopInventoryOpen(state) {
		lines = append(lines,
			fmt.Sprintf(
				"- %s",
				loc.Label("Shop inventory is closed; only open_shop_inventory or proceed is legal right now.", "商店背包当前关闭；这一步只能打开商店背包或直接离开。"),
			),
		)
		return lines
	}
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
	addItems(loc.Label("Card", "卡牌"), shopItemsToMaps(state, "cards"))
	addItems(loc.Label("Relic", "遗物"), shopItemsToMaps(state, "relics"))
	addItems(loc.Label("Potion", "药水"), shopItemsToMaps(state, "potions"))
	return lines
}

func restDetailLines(state *game.StateSnapshot, maxItems int, loc i18n.Localizer) []string {
	if state == nil || state.Rest == nil {
		return nil
	}
	lines := []string{}
	for i, option := range state.Rest.Options {
		if i >= maxItems {
			lines = append(lines, fmt.Sprintf("- %s: ... %d %s", loc.Label("Rest options", "营火选项"), len(state.Rest.Options)-maxItems, loc.Label("more", "更多")))
			break
		}
		lines = append(lines, fmt.Sprintf("- %s %d: [%d] `%s` %s `%t`", loc.Label("Rest option", "营火选项"), i, option.Index, valueOrDash(option.OptionID), loc.Label("enabled", "可用"), option.IsEnabled))
	}
	return lines
}

func chestDetailLines(state *game.StateSnapshot, maxItems int, loc i18n.Localizer) []string {
	if state == nil || state.Chest == nil {
		return nil
	}
	lines := []string{}
	if state.Chest.IsOpened {
		lines = append(lines, "- "+loc.Label("Chest: already opened", "宝箱：已打开"))
	}
	for i, relic := range state.Chest.RelicOptions {
		if i >= maxItems {
			lines = append(lines, fmt.Sprintf("- %s: ... %d %s", loc.Label("Relics", "遗物"), len(state.Chest.RelicOptions)-maxItems, loc.Label("more", "更多")))
			break
		}
		lines = append(lines, fmt.Sprintf("- %s %d: [%d] `%s`", loc.Label("Relic", "遗物"), i, relic.Index, valueOrDash(fallbackID(relic.Name, relic.RelicID))))
	}
	return lines
}

func selectionDetailLines(state *game.StateSnapshot, maxItems int, loc i18n.Localizer) []string {
	if state == nil || state.Selection == nil {
		return nil
	}
	lines := []string{}
	if kind := state.Selection.Kind; kind != "" {
		source := state.Selection.SourceScreen
		if source != "" {
			lines = append(lines, fmt.Sprintf("- %s: `%s` (%s `%s`)", loc.Label("Selection context", "选择上下文"), kind, loc.Label("source", "来源"), source))
		} else {
			lines = append(lines, fmt.Sprintf("- %s: `%s`", loc.Label("Selection context", "选择上下文"), kind))
		}
	}
	for i, card := range state.Selection.Cards {
		if i >= maxItems {
			lines = append(lines, fmt.Sprintf("- %s: ... %d %s", loc.Label("Selection cards", "选牌"), len(state.Selection.Cards)-maxItems, loc.Label("more", "更多")))
			break
		}
		lines = append(lines, fmt.Sprintf("- %s %d: [%d] %s", loc.Label("Selection", "选择"), i, card.Index, valueOrDash(fallbackID(card.Name, card.CardID))))
	}
	return lines
}

func gameOverDetailLines(state *game.StateSnapshot, loc i18n.Localizer) []string {
	if state == nil || state.GameOver == nil {
		return nil
	}
	lines := []string{}
	if state.GameOver.Victory {
		lines = append(lines, "- "+loc.Label("Outcome: victory", "结果：胜利"))
	} else {
		lines = append(lines, "- "+loc.Label("Outcome: defeat", "结果：失败"))
	}
	if state.GameOver.Floor > 0 {
		lines = append(lines, fmt.Sprintf("- %s: `%d`", loc.Label("Final floor", "最终层数"), state.GameOver.Floor))
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
