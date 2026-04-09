package agentruntime

import (
	"encoding/json"
	"fmt"
	"strings"

	"spire2mind/internal/game"
	"spire2mind/internal/i18n"
)

type PromptMode string

const (
	PromptModeCycle      PromptMode = "cycle"
	PromptModeStructured PromptMode = "structured"
)

type PromptTelemetry struct {
	Mode            PromptMode     `json:"mode"`
	Screen          string         `json:"screen,omitempty"`
	PromptSizeBytes int            `json:"prompt_size_bytes"`
	BlockBreakdown  map[string]int `json:"prompt_block_breakdown,omitempty"`
}

type PromptAssembly struct {
	Text      string          `json:"text"`
	Telemetry PromptTelemetry `json:"telemetry"`
}

type PromptAssemblyPipeline struct{}

type promptBlock struct {
	Name string
	Text string
}

type structuredPromptPolicy struct {
	IncludePlanner       bool
	IncludeTacticalHints bool
	IncludeTodo          bool
	IncludeCompact       bool
	IncludeKnowledge     bool
}

func NewPromptAssemblyPipeline() *PromptAssemblyPipeline {
	return &PromptAssemblyPipeline{}
}

func (p *PromptAssemblyPipeline) Build(mode PromptMode, state *game.StateSnapshot, todo *TodoManager, skills *SkillLibrary, compact *CompactMemory, planner *CombatPlan, language i18n.Language) PromptAssembly {
	loc := i18n.New(language)
	blocks := make([]promptBlock, 0, 10)

	switch mode {
	case PromptModeStructured:
		blocks = append(blocks,
			promptBlock{Name: "decision_contract", Text: p.structuredDecisionContract(loc)},
			promptBlock{Name: "output_contract", Text: p.structuredOutputContract(loc)},
		)
		if block := strings.TrimSpace(p.runObjectiveBlock(state, loc)); block != "" {
			blocks = append(blocks, promptBlock{Name: "run_objective", Text: block})
		}
		if block := strings.TrimSpace(p.structuredScreenGuidance(state, loc)); block != "" {
			blocks = append(blocks, promptBlock{Name: "screen_guidance", Text: block})
		}
	default:
		blocks = append(blocks, promptBlock{Name: "cycle_contract", Text: p.cycleContract(loc)})
	}

	if block := strings.TrimSpace(p.screenSummaryBlock(state, loc)); block != "" {
		blocks = append(blocks, promptBlock{Name: "screen_summary", Text: block})
	}
	if block := strings.TrimSpace(p.minimalStatePayloadBlock(state, loc)); block != "" {
		blocks = append(blocks, promptBlock{Name: "minimal_state", Text: block})
	}

	switch mode {
	case PromptModeStructured:
		p.appendStructuredStateBlocks(&blocks, state, todo, skills, compact, planner, language)
	default:
		p.appendCycleStateBlocks(&blocks, state, todo, skills, compact, planner, language)
	}

	nonEmpty := make([]string, 0, len(blocks))
	breakdown := make(map[string]int, len(blocks))
	for _, block := range blocks {
		text := strings.TrimSpace(block.Text)
		if text == "" {
			continue
		}
		nonEmpty = append(nonEmpty, text)
		breakdown[block.Name] = len([]byte(text))
	}

	text := strings.Join(nonEmpty, "\n\n")
	return PromptAssembly{
		Text: text,
		Telemetry: PromptTelemetry{
			Mode:            mode,
			Screen:          strings.TrimSpace(stateScreen(state)),
			PromptSizeBytes: len([]byte(text)),
			BlockBreakdown:  breakdown,
		},
	}
}

func (p *PromptAssemblyPipeline) appendCycleStateBlocks(blocks *[]promptBlock, state *game.StateSnapshot, todo *TodoManager, skills *SkillLibrary, compact *CompactMemory, planner *CombatPlan, language i18n.Language) {
	if block := strings.TrimSpace(TacticalHintsBlock(state)); block != "" {
		*blocks = append(*blocks, promptBlock{Name: "tactical_hints", Text: block})
	}
	if planner != nil {
		if block := strings.TrimSpace(planner.PromptBlock(language)); block != "" {
			*blocks = append(*blocks, promptBlock{Name: "planner", Text: block})
		}
	}
	if todo != nil {
		if block := strings.TrimSpace(todo.PromptBlockCompact()); block != "" {
			*blocks = append(*blocks, promptBlock{Name: "todo", Text: block})
		}
	}
	if compact != nil {
		if block := strings.TrimSpace(compact.PromptBlockForState(state)); block != "" {
			*blocks = append(*blocks, promptBlock{Name: "compact_memory", Text: block})
		}
	}
	if skills != nil {
		if block := strings.TrimSpace(skills.PromptBlockForState(state)); block != "" {
			*blocks = append(*blocks, promptBlock{Name: "entity_knowledge", Text: block})
		}
	}
}

func (p *PromptAssemblyPipeline) appendStructuredStateBlocks(blocks *[]promptBlock, state *game.StateSnapshot, todo *TodoManager, skills *SkillLibrary, compact *CompactMemory, planner *CombatPlan, language i18n.Language) {
	policy := p.structuredPolicy(state)

	if policy.IncludePlanner && planner != nil {
		if block := strings.TrimSpace(planner.PromptBlock(language)); block != "" {
			*blocks = append(*blocks, promptBlock{Name: "planner", Text: block})
		}
	}
	if policy.IncludeTacticalHints {
		if block := strings.TrimSpace(TacticalHintsBlock(state)); block != "" {
			*blocks = append(*blocks, promptBlock{Name: "tactical_hints", Text: block})
		}
	}
	if policy.IncludeTodo && todo != nil {
		if block := strings.TrimSpace(todo.PromptBlockCompact()); block != "" {
			*blocks = append(*blocks, promptBlock{Name: "todo", Text: block})
		}
	}
	if policy.IncludeCompact && compact != nil {
		if block := strings.TrimSpace(compact.PromptBlockForState(state)); block != "" {
			*blocks = append(*blocks, promptBlock{Name: "compact_memory", Text: block})
		}
	}
	if policy.IncludeKnowledge && skills != nil {
		if block := strings.TrimSpace(skills.PromptBlockForState(state)); block != "" {
			*blocks = append(*blocks, promptBlock{Name: "entity_knowledge", Text: block})
		}
	}
}

func (p *PromptAssemblyPipeline) structuredPolicy(state *game.StateSnapshot) structuredPromptPolicy {
	switch strings.ToUpper(strings.TrimSpace(stateScreen(state))) {
	case "COMBAT":
		return structuredPromptPolicy{
			IncludePlanner:       true,
			IncludeTacticalHints: true,
			IncludeTodo:          true,
			IncludeCompact:       false,
			IncludeKnowledge:     true,
		}
	case "MAP", "EVENT", "SHOP", "REWARD", "CARD_SELECTION", "REST":
		return structuredPromptPolicy{
			IncludePlanner:       false,
			IncludeTacticalHints: false,
			IncludeTodo:          true,
			IncludeCompact:       true,
			IncludeKnowledge:     true,
		}
	default:
		return structuredPromptPolicy{
			IncludePlanner:       false,
			IncludeTacticalHints: false,
			IncludeTodo:          true,
			IncludeCompact:       true,
			IncludeKnowledge:     false,
		}
	}
}

func (p *PromptAssemblyPipeline) cycleContract(loc i18n.Localizer) string {
	return loc.Paragraph(
		`You are continuing a live Slay the Spire 2 run.
Use the available tools to inspect state and make forward progress.
In this cycle, you may inspect freely but execute at most one act tool call.
If the game is not actionable, call wait_until_actionable.
After one successful game action, stop.`,
		`你正在继续一局进行中的《杀戮尖塔 2》。
使用可用工具读取状态并推进游戏。
这一轮允许观察，但最多只执行一次 act 工具调用。
如果当前不可操作，就调用 wait_until_actionable。
成功执行一次游戏动作后立刻停止。`,
	)
}

func (p *PromptAssemblyPipeline) structuredDecisionContract(loc i18n.Localizer) string {
	return loc.Paragraph(
		`Choose exactly one legal next action for the live Slay the Spire 2 run.
Return exactly one JSON object and nothing else.

Decision priority:
- Trust the current live state first.
- Use only actions from available_actions.
- Handle blocking modal or confirmation actions before strategy.
- Prefer forward progress over explanation.
- When multiple lines are viable, pick one concrete legal action now.`,
		`请为当前这局正在进行的《杀戮尖塔 2》选择一个且仅一个合法的下一步动作。
只返回一个 JSON 对象，不要输出其他内容。

决策优先级：
- 先信任当前 live state。
- 只能使用 available_actions 里的动作。
- 先处理阻塞性的 modal 或确认动作。
- 以推进对局为先，不要写解释性长文。
- 当存在多条可行路线时，立刻选出一条具体且合法的动作。`,
	)
}

func (p *PromptAssemblyPipeline) structuredOutputContract(loc i18n.Localizer) string {
	return loc.Paragraph(
		`Output contract:
{
  "action": "one available action string",
  "card_index": 0,
  "target_index": 0,
  "option_index": 0,
  "reason": "short reason"
}

Rules:
- Include only the indexes the chosen action needs.
- Omit unused fields.
- Do not wrap the JSON in markdown fences.
- Do not add any text before or after the JSON.
- Do not simulate tool calls in plain text.
- Use option_index for choice-style actions such as map, reward, event, shop, rest, and deck-selection choices.
- Use card_index only for play_card.
- Use target_index only when the chosen play_card requires a target.`,
		`输出契约：
{
  "action": "某个 available action 字符串",
  "card_index": 0,
  "target_index": 0,
  "option_index": 0,
  "reason": "简短原因"
}

规则：
- 只保留该动作真正需要的索引字段。
- 未使用字段不要输出。
- 不要用 markdown 代码块包裹 JSON。
- 不要在 JSON 前后添加任何文本。
- 不要在普通文本里模拟工具调用。
- 地图、奖励、事件、商店、休息点、通用选牌这类选项动作使用 option_index。
- 只有 play_card 使用 card_index。
- 只有目标型 play_card 才使用 target_index。`,
	)
}

func (p *PromptAssemblyPipeline) structuredScreenGuidance(state *game.StateSnapshot, loc i18n.Localizer) string {
	switch strings.ToUpper(strings.TrimSpace(stateScreen(state))) {
	case "COMBAT":
		return loc.Paragraph(
			`Combat guidance:
- Prefer the planner's best legal line when it is still legal in the current state.
- Preserve HP when the board is dangerous, but convert safe turns into damage or setup that improves the next turns.
- Favor lines that reduce enemy count, lethal pressure, or incoming damage this combat over flashy value.
- Use play_card only when the card is currently playable.
- For targeted combat cards, include target_index from the live valid target set.
- If only end_turn is legal, return end_turn immediately.
- Keep the reason short and concrete.`,
			`战斗指引：
- 如果 planner 的首选路线在当前状态仍然合法，优先选择它。
- 只有牌当前可打时才使用 play_card。
- 对于需要目标的战斗牌，target_index 必须来自当前 live 的有效目标集合。
- 如果只剩 end_turn 合法，立刻返回 end_turn。
- reason 保持短且具体。`,
		)
	case "MAP":
		return loc.Paragraph(
			`Map guidance:
- choose_map_node uses option_index, never card_index or target_index.
- If only one node is available, pick option_index 0.
- Prefer routes that improve expected floor depth: stable combats, useful elites only when ready, and recovery when the run is fragile.`,
			`地图指引：
- choose_map_node 使用 option_index，不要用 card_index 或 target_index。
- 如果只有一个可选节点，直接选 option_index 0。
- 优先选择稳定向前推进，不要为了理想路线而停滞。`,
		)
	case "REWARD":
		return loc.Paragraph(
			`Reward guidance:
- choose_reward_card, claim_reward, and proceed are choice-style actions; use option_index only when the action needs an index.
- If a card reward overlay is open, resolve the card choice or skip first.
- Do not use select_deck_card on reward card overlays.
- Prefer picks that improve near-term survival, deck stability, or the next few fights before speculative scaling.`,
			`奖励指引：
- choose_reward_card、claim_reward 和 proceed 都是选项型动作，只有在动作需要索引时才使用 option_index。
- 如果卡牌奖励 overlay 已经打开，先完成选牌或跳过。
- reward card overlay 上不要使用 select_deck_card。`,
		)
	case "CARD_SELECTION":
		return loc.Paragraph(
			`Card selection guidance:
- select_deck_card uses option_index.
- confirm_selection is legal only when the live state says confirmation is available.
- Choose one concrete card now; do not explain multiple alternatives.`,
			`选牌指引：
- select_deck_card 使用 option_index。
- 只有当 live state 表示可以确认时，confirm_selection 才是合法动作。
- 当前只选一张具体的牌，不要展开多个候选。`,
		)
	case "EVENT":
		return loc.Paragraph(
			`Event guidance:
- choose_event_option uses option_index.
- If the event is finished and choose_event_option is the only legal action, pick option_index 0.
- Prefer durable upside and avoid obvious run-killing downside.`,
			`事件指引：
- choose_event_option 使用 option_index。
- 如果事件已结束且 choose_event_option 是唯一合法动作，直接选 option_index 0。
- 优先持续性收益，避免明显会毁局的选项。`,
		)
	case "SHOP":
		return loc.Paragraph(
			`Shop guidance:
- buy_card, buy_relic, buy_potion, and similar choice actions use option_index.
- Do not spend gold on filler when a stronger purchase or removal is available.
- Convert gold into survivability or power before it becomes stranded in a losing run.`,
			`商店指引：
- buy_card、buy_relic、buy_potion 以及类似选项动作使用 option_index。
- 如果有更强的购买或移除路线，不要把金币花在填充物上。
- 如果没有明显值得买的东西，优先推进对局。`,
		)
	case "REST":
		return loc.Paragraph(
			`Rest guidance:
- choose_rest_option uses option_index.
- If a follow-up card selection opens, the next cycle will handle it; choose only the rest action now.
- Prefer stable upgrades or healing over speculative lines.`,
			`火堆指引：
- choose_rest_option 使用 option_index。
- 如果后续会打开选牌界面，下一轮会再处理，这一轮只选火堆动作。
- 优先稳定的强化或回血，不要走过于投机的线。`,
		)
	default:
		return ""
	}
}

func (p *PromptAssemblyPipeline) runObjectiveBlock(state *game.StateSnapshot, loc i18n.Localizer) string {
	floor, _ := fieldInt(state.Run, "floor")
	currentHP, _ := fieldInt(state.Run, "currentHp")
	maxHP, _ := fieldInt(state.Run, "maxHp")
	lowHP := maxHP > 0 && currentHP*100 <= maxHP*45

	stageObjective := "Build a run that can keep climbing: take stable lines, convert resources into immediate power, and avoid unnecessary HP loss."
	switch {
	case floor <= 3:
		stageObjective = "Early floors decide whether the run stabilizes. Prioritize immediate combat power, reliable block, and low-variance progress."
	case floor <= 10:
		stageObjective = "Keep the run stable through early Act 1. Preserve HP while adding enough damage, defense, and consistency for the next few rooms."
	case floor <= 17:
		stageObjective = "Prepare for stronger fights ahead. Favor coherent picks that preserve HP and add scaling or burst when they are worth the risk."
	case floor > 17:
		stageObjective = "Protect a viable winning run. Avoid unnecessary risk and keep turning gold, HP, and choices into consistency."
	}

	riskDirective := "Current risk posture: normal. Do not waste HP or gold, but take strong legal lines when they clearly improve the run."
	if lowHP {
		riskDirective = "Current risk posture: fragile. Bias toward preserving HP, safer routes, defensive rewards, and lower-variance combat lines."
	}

	return loc.Paragraph(
		"Run objective:\n"+
			"- The main goal is to reach deeper floors and eventually win, not to maximize flashy short-term value.\n"+
			"- "+stageObjective+"\n"+
			"- "+riskDirective+"\n"+
			"- When two legal actions are close, prefer the one that improves survival, deck stability, or future room quality.",
		"对局目标：\n"+
			"- 主要目标是打到更深的层数并最终通关，不是只追求眼前漂亮的一步。\n"+
			"- "+stageObjective+"\n"+
			"- "+riskDirective+"\n"+
			"- 当两个合法动作差距不大时，优先选择更能提高生存、牌组稳定性或后续房间质量的那条。",
	)
}

func (p *PromptAssemblyPipeline) screenSummaryBlock(state *game.StateSnapshot, loc i18n.Localizer) string {
	if state == nil {
		return ""
	}

	lines := []string{
		fmt.Sprintf("%s: %s", loc.Label("Current screen", "当前界面"), state.Screen),
		fmt.Sprintf("%s: %s", loc.Label("Run id", "Run 标识"), state.RunID),
		fmt.Sprintf("%s: %s", loc.Label("Available actions", "当前可用动作"), strings.Join(state.AvailableActions, ", ")),
	}

	summaryLines := StateSummaryLines(state)
	detailLines := StateDetailLines(state, 4)
	if len(summaryLines) > 0 {
		lines = append(lines, "", loc.Label("State overview", "状态总览")+":")
		lines = append(lines, summaryLines...)
	}
	if len(detailLines) > 0 && detailLines[0] != "- -" {
		lines = append(lines, "", loc.Label("Room detail", "房间细节")+":")
		lines = append(lines, detailLines...)
	}

	return strings.Join(lines, "\n")
}

func (p *PromptAssemblyPipeline) minimalStatePayloadBlock(state *game.StateSnapshot, loc i18n.Localizer) string {
	if state == nil {
		return ""
	}

	payload := map[string]any{
		"run_id":            strings.TrimSpace(state.RunID),
		"screen":            strings.TrimSpace(state.Screen),
		"available_actions": append([]string(nil), state.AvailableActions...),
	}
	if state.Turn != nil {
		payload["turn"] = *state.Turn
	}
	if run := minimalRunPayload(state); len(run) > 0 {
		payload["run"] = run
	}

	switch strings.ToUpper(strings.TrimSpace(state.Screen)) {
	case "COMBAT":
		payload["combat"] = minimalCombatPayload(state)
	case "REWARD":
		payload["reward"] = minimalRewardPayload(state)
	case "CARD_SELECTION":
		payload["selection"] = minimalSelectionPayload(state)
	case "EVENT":
		payload["event"] = minimalEventPayload(state)
	case "SHOP":
		payload["shop"] = minimalShopPayload(state)
	case "REST":
		payload["rest"] = minimalRestPayload(state)
	case "MAP":
		payload["map"] = minimalMapPayload(state)
	case "GAME_OVER":
		payload["game_over"] = minimalGameOverPayload(state)
	}

	bytes, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return ""
	}
	return loc.Label("Minimal state payload", "最小状态载荷") + ":\n" + string(bytes)
}

func minimalRunPayload(state *game.StateSnapshot) map[string]any {
	if state == nil || state.Run == nil {
		return nil
	}
	run := map[string]any{}
	copyMapKey(run, "floor", state.Run, "floor")
	copyMapKey(run, "current_hp", state.Run, "currentHp")
	copyMapKey(run, "max_hp", state.Run, "maxHp")
	copyMapKey(run, "gold", state.Run, "gold")
	copyMapKey(run, "character_id", state.Run, "characterId")
	return run
}

func minimalCombatPayload(state *game.StateSnapshot) map[string]any {
	payload := map[string]any{}
	player := nestedMap(state.Combat, "player")
	if len(player) > 0 {
		snapshot := map[string]any{}
		copyMapKey(snapshot, "current_hp", player, "currentHp")
		copyMapKey(snapshot, "max_hp", player, "maxHp")
		copyMapKey(snapshot, "block", player, "block")
		copyMapKey(snapshot, "energy", player, "energy")
		payload["player"] = snapshot
	}
	if hand := nestedList(state.Combat, "hand"); len(hand) > 0 {
		cards := make([]map[string]any, 0, minInt(len(hand), 8))
		for _, card := range hand {
			entry := map[string]any{}
			copyMapKey(entry, "index", card, "index")
			copyMapKey(entry, "id", card, "id")
			copyMapKey(entry, "name", card, "name")
			copyMapKey(entry, "cost", card, "cost")
			copyMapKey(entry, "playable", card, "playable")
			copyMapKey(entry, "requires_target", card, "requiresTarget")
			if validTargets := fieldIntSlice(card, "validTargetIndices"); len(validTargets) > 0 {
				entry["valid_target_indices"] = validTargets
			}
			cards = append(cards, entry)
		}
		payload["hand"] = cards
	}
	if enemies := nestedList(state.Combat, "enemies"); len(enemies) > 0 {
		items := make([]map[string]any, 0, minInt(len(enemies), 6))
		for _, enemy := range enemies {
			entry := map[string]any{}
			copyMapKey(entry, "index", enemy, "index")
			copyMapKey(entry, "id", enemy, "id")
			copyMapKey(entry, "name", enemy, "name")
			copyMapKey(entry, "current_hp", enemy, "currentHp")
			copyMapKey(entry, "block", enemy, "block")
			copyMapKey(entry, "intent", enemy, "intent")
			copyMapKey(entry, "is_hittable", enemy, "isHittable")
			items = append(items, entry)
		}
		payload["enemies"] = items
	}
	return payload
}

func minimalRewardPayload(state *game.StateSnapshot) map[string]any {
	payload := map[string]any{
		"pending_card_choice": fieldBool(state.Reward, "pendingCardChoice"),
		"can_proceed":         fieldBool(state.Reward, "canProceed"),
	}
	if rewards := nestedList(state.Reward, "rewards"); len(rewards) > 0 {
		items := make([]map[string]any, 0, minInt(len(rewards), 6))
		for _, reward := range rewards {
			entry := map[string]any{}
			copyMapKey(entry, "index", reward, "index")
			copyMapKey(entry, "reward_type", reward, "rewardType")
			copyMapKey(entry, "name", reward, "name")
			copyMapKey(entry, "claimable", reward, "claimable")
			items = append(items, entry)
		}
		payload["rewards"] = items
	}
	if cards := nestedList(state.Reward, "cardOptions"); len(cards) > 0 {
		items := make([]map[string]any, 0, minInt(len(cards), 5))
		for _, card := range cards {
			entry := map[string]any{}
			copyMapKey(entry, "index", card, "index")
			copyMapKey(entry, "id", card, "id")
			copyMapKey(entry, "name", card, "name")
			copyMapKey(entry, "cost", card, "cost")
			items = append(items, entry)
		}
		payload["card_options"] = items
	}
	return payload
}

func minimalSelectionPayload(state *game.StateSnapshot) map[string]any {
	payload := map[string]any{
		"kind":                  fieldString(state.Selection, "kind"),
		"source_screen":         fieldString(state.Selection, "sourceScreen"),
		"requires_confirmation": fieldBool(state.Selection, "requiresConfirmation"),
		"can_confirm":           fieldBool(state.Selection, "canConfirm"),
	}
	if cards := nestedList(state.Selection, "cards"); len(cards) > 0 {
		items := make([]map[string]any, 0, minInt(len(cards), 8))
		for _, card := range cards {
			entry := map[string]any{}
			copyMapKey(entry, "index", card, "index")
			copyMapKey(entry, "id", card, "id")
			copyMapKey(entry, "name", card, "name")
			items = append(items, entry)
		}
		payload["cards"] = items
	}
	return payload
}

func minimalEventPayload(state *game.StateSnapshot) map[string]any {
	payload := map[string]any{
		"id":          fieldString(state.Event, "id"),
		"name":        firstNonEmpty(fieldString(state.Event, "name"), fieldString(state.Event, "title")),
		"is_finished": fieldBool(state.Event, "isFinished"),
	}
	if options := nestedList(state.Event, "options"); len(options) > 0 {
		items := make([]map[string]any, 0, minInt(len(options), 6))
		for _, option := range options {
			entry := map[string]any{}
			copyMapKey(entry, "index", option, "index")
			copyMapKey(entry, "label", option, "label")
			copyMapKey(entry, "is_locked", option, "isLocked")
			items = append(items, entry)
		}
		payload["options"] = items
	}
	return payload
}

func minimalShopPayload(state *game.StateSnapshot) map[string]any {
	payload := map[string]any{}
	for _, section := range []struct {
		source string
		target string
	}{{"cards", "cards"}, {"relics", "relics"}, {"potions", "potions"}} {
		items := nestedList(state.Shop, section.source)
		if len(items) == 0 {
			continue
		}
		out := make([]map[string]any, 0, minInt(len(items), 6))
		for _, item := range items {
			entry := map[string]any{}
			copyMapKey(entry, "index", item, "index")
			copyMapKey(entry, "name", item, "name")
			copyMapKey(entry, "price", item, "price")
			copyMapKey(entry, "enough_gold", item, "enoughGold")
			out = append(out, entry)
		}
		payload[section.target] = out
	}
	return payload
}

func minimalRestPayload(state *game.StateSnapshot) map[string]any {
	payload := map[string]any{}
	if options := nestedList(state.Rest, "options"); len(options) > 0 {
		items := make([]map[string]any, 0, minInt(len(options), 6))
		for _, option := range options {
			entry := map[string]any{}
			copyMapKey(entry, "index", option, "index")
			copyMapKey(entry, "label", option, "label")
			copyMapKey(entry, "is_enabled", option, "isEnabled")
			items = append(items, entry)
		}
		payload["options"] = items
	}
	return payload
}

func minimalMapPayload(state *game.StateSnapshot) map[string]any {
	payload := map[string]any{}
	if nodes := nestedList(state.Map, "availableNodes"); len(nodes) > 0 {
		items := make([]map[string]any, 0, minInt(len(nodes), 8))
		for _, node := range nodes {
			entry := map[string]any{}
			copyMapKey(entry, "index", node, "index")
			copyMapKey(entry, "symbol", node, "symbol")
			copyMapKey(entry, "node_type", node, "nodeType")
			items = append(items, entry)
		}
		payload["available_nodes"] = items
	}
	return payload
}

func minimalGameOverPayload(state *game.StateSnapshot) map[string]any {
	payload := map[string]any{}
	copyMapKey(payload, "can_continue", state.GameOver, "canContinue")
	copyMapKey(payload, "can_return_to_main_menu", state.GameOver, "canReturnToMainMenu")
	copyMapKey(payload, "summary_complete", state.GameOver, "summaryComplete")
	return payload
}

func copyMapKey(dst map[string]any, targetKey string, src map[string]any, sourceKey string) {
	if len(src) == 0 {
		return
	}
	value, ok := src[sourceKey]
	if !ok || value == nil {
		return
	}
	dst[targetKey] = value
}

func minInt(left int, right int) int {
	if left < right {
		return left
	}
	return right
}
