package agentruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"spire2mind/internal/config"
	"spire2mind/internal/game"
	"spire2mind/internal/i18n"
)

type StreamerBeat struct {
	Trigger        string   `json:"trigger"`
	Mood           string   `json:"mood"`
	Commentary     string   `json:"commentary"`
	GameInsight    string   `json:"game_insight"`
	LifeReflection string   `json:"life_reflection"`
	TTSText        string   `json:"tts_text"`
	TTSSegments    []string `json:"tts_segments,omitempty"`
}

type StreamerMoment struct {
	BeforeState *game.StateSnapshot
	AfterState  *game.StateSnapshot
	Action      string
	Outcome     string
	Trigger     string
}

func (m StreamerMoment) ActiveState() *game.StateSnapshot {
	if m.AfterState != nil {
		return m.AfterState
	}
	return m.BeforeState
}

type StreamerDirector struct {
	cfg        config.Config
	runtime    *Runtime
	queueDir   string
	latestPath string
	textPath   string
}

func NewStreamerDirector(cfg config.Config, runtime *Runtime) *StreamerDirector {
	queueDir := filepath.Join(cfg.TTSQueueDir, "queue")
	return &StreamerDirector{
		cfg:        cfg,
		runtime:    runtime,
		queueDir:   queueDir,
		latestPath: filepath.Join(cfg.TTSQueueDir, "latest.json"),
		textPath:   filepath.Join(cfg.TTSQueueDir, "latest.txt"),
	}
}

func (d *StreamerDirector) Enabled() bool {
	return d != nil && d.cfg.StreamerEnabled
}

func (d *StreamerDirector) ShouldCommentate(moment StreamerMoment, previousSignature string) (string, string, bool) {
	if !d.Enabled() {
		return "", "", false
	}

	state := moment.ActiveState()
	if state == nil {
		return "", "", false
	}

	trigger := strings.TrimSpace(moment.Trigger)
	if trigger == "" {
		trigger = deriveStreamerTrigger(moment)
	}
	if trigger == "" {
		return "", "", false
	}

	screen := strings.ToUpper(strings.TrimSpace(state.Screen))
	runID := strings.TrimSpace(state.RunID)
	floor, _ := fieldInt(state.Run, "floor")
	turn := 0
	if state.Turn != nil {
		turn = *state.Turn
	}

	signature := strings.Join([]string{
		runID,
		screen,
		fmt.Sprintf("%d", floor),
		fmt.Sprintf("%d", turn),
		trigger,
		strings.TrimSpace(moment.Action),
		compactStreamerSignatureText(moment.Outcome, 96),
		fieldString(state.AgentView, "headline"),
	}, "|")
	if signature == previousSignature {
		return "", "", false
	}

	return trigger, signature, true
}

func deriveStreamerTrigger(moment StreamerMoment) string {
	switch strings.TrimSpace(moment.Action) {
	case "choose_map_node":
		return "map_choice"
	case "claim_reward", "choose_reward_card", "skip_reward_cards", "proceed":
		return "reward_choice"
	case "choose_event_option":
		return "event_choice"
	case "buy_card", "buy_relic", "buy_potion", "remove_card":
		return "shop_choice"
	case "choose_rest_option":
		return "rest_choice"
	case "choose_treasure_relic", "skip_treasure_relic":
		return "chest_choice"
	case "select_deck_card", "confirm_selection":
		return "card_selection"
	case "play_card", "end_turn":
		if shouldCommentateCombatMoment(moment) {
			return "combat_opening"
		}
	}

	state := moment.ActiveState()
	if state == nil {
		return ""
	}

	screen := strings.ToUpper(strings.TrimSpace(state.Screen))
	switch screen {
	case "MAP":
		return "map_choice"
	case "REWARD":
		return "reward_choice"
	case "EVENT":
		return "event_choice"
	case "SHOP":
		return "shop_choice"
	case "REST":
		return "rest_choice"
	case "CHEST":
		return "chest_choice"
	case "GAME_OVER":
		return "game_over"
	case "CARD_SELECTION":
		return "card_selection"
	case "COMBAT":
		if shouldCommentateCombatMoment(moment) {
			return "combat_opening"
		}
	}

	return ""
}

func shouldCommentateCombatMoment(moment StreamerMoment) bool {
	if strings.TrimSpace(moment.Action) == "" {
		return false
	}

	beforeTurn := 0
	if moment.BeforeState != nil && moment.BeforeState.Turn != nil {
		beforeTurn = *moment.BeforeState.Turn
	}
	afterTurn := 0
	if moment.AfterState != nil && moment.AfterState.Turn != nil {
		afterTurn = *moment.AfterState.Turn
	}

	return beforeTurn <= 1 || afterTurn <= 1
}

func (d *StreamerDirector) Generate(ctx context.Context, moment StreamerMoment, history []string, todo *TodoManager, compact *CompactMemory, language i18n.Language) (*StreamerBeat, error) {
	if !d.Enabled() || d.runtime == nil || d.runtime.Agent == nil {
		return nil, nil
	}

	trigger := strings.TrimSpace(moment.Trigger)
	if trigger == "" {
		trigger = deriveStreamerTrigger(moment)
	}
	if trigger == "" {
		return nil, nil
	}
	moment.Trigger = trigger

	prompt := d.buildPrompt(moment, history, todo, compact, language)
	result, err := d.runtime.Agent.Prompt(ctx, prompt)
	if err != nil {
		return nil, err
	}

	beat, err := parseStreamerBeat(result.Text)
	if err != nil {
		return fallbackStreamerBeat(moment, result.Text, language), nil
	}
	if strings.TrimSpace(beat.Trigger) == "" {
		beat.Trigger = trigger
	}
	if strings.TrimSpace(beat.TTSText) == "" {
		beat.TTSText = strings.TrimSpace(beat.Commentary)
	}
	beat.TTSSegments = d.splitTTSText(ctx, beat, language)
	return beat, nil
}

func (d *StreamerDirector) buildPrompt(moment StreamerMoment, history []string, todo *TodoManager, compact *CompactMemory, language i18n.Language) string {
	loc := i18n.New(language)
	state := moment.ActiveState()
	trigger := strings.TrimSpace(moment.Trigger)
	if trigger == "" {
		trigger = deriveStreamerTrigger(moment)
	}
	beforeSummary := "-"
	if moment.BeforeState != nil {
		beforeSummary = strings.Join(StateSummaryLinesFor(moment.BeforeState, language), "\n")
	}
	afterSummary := "-"
	if state != nil {
		afterSummary = strings.Join(StateSummaryLinesFor(state, language), "\n")
	}
	afterDetail := "-"
	if state != nil {
		afterDetail = strings.Join(StateDetailLinesFor(state, 8, language), "\n")
	}

	blocks := []string{
		loc.Paragraph(
			"You are the Chinese co-host for a live Slay the Spire 2 stream. Speak like a sharp female streamer who reacts to pressure, danger, greed, relief, and sudden swings. Do not narrate UI steps.",
			"你是《杀戮尖塔 2》直播里的中文女主播搭档。你的任务不是念流程，而是对局势里的压力、危险、贪念、松口气和突发变化做有情绪的反应。",
		),
		loc.Paragraph(
			"Your commentary must be based on the action that already landed and the board state after it. React to what just happened, not what might happen.",
			"你的解说必须基于已经落地的动作和动作之后的局面。要说刚刚发生了什么、这意味着什么，不要抢跑去讲还没发生的事。",
		),
		fmt.Sprintf("%s: %s", loc.Label("Streamer style", "主播风格"), streamerStyleInstruction(d.cfg.StreamerStyle, loc)),
		loc.Paragraph(
			"Write exactly one JSON object and nothing else.",
			"只输出一个 JSON 对象，不要输出任何额外文字。",
		),
		`{
  "trigger": "short trigger label",
  "mood": "current mood in a short Chinese phrase",
  "commentary": "2-4 Chinese sentences for viewers, with emotion first",
  "game_insight": "1 Chinese sentence about the real pressure or opportunity",
  "life_reflection": "1 Chinese sentence connecting the moment to people, pressure, restraint, luck, or greed",
  "tts_text": "1-3 short Chinese spoken sentences that are easy to read aloud"
}`,
		loc.Paragraph(
			"Do not narrate menus, indexes, or button clicks. Do not explain the whole process. Give emotional value first, then a real game read, then one light human reflection.",
			"不要播报菜单、索引、按钮点击，不要复述整段流程。先给情绪价值，再给真正有用的局势判断，最后带一句轻一点的人生感悟。",
		),
		fmt.Sprintf("%s: `%s`", loc.Label("Trigger", "触发原因"), triggerLabel(trigger, loc)),
		fmt.Sprintf("%s: `%s`", loc.Label("Action that just landed", "刚刚落地的动作"), valueOrDash(strings.TrimSpace(moment.Action))),
		fmt.Sprintf("%s: %s", loc.Label("Observed result", "观测到的结果"), valueOrDash(strings.TrimSpace(moment.Outcome))),
		fmt.Sprintf("%s:\n%s", loc.Label("State before the action", "动作前局面"), beforeSummary),
		fmt.Sprintf("%s:\n%s", loc.Label("State after the action", "动作后局面"), afterSummary),
		fmt.Sprintf("%s:\n%s", loc.Label("Current board detail", "当前局面细节"), afterDetail),
	}

	if todo != nil {
		snapshot := todo.Snapshot()
		if text := strings.TrimSpace(localizeTodoText(snapshot.CurrentGoal, loc)); text != "" {
			blocks = append(blocks, fmt.Sprintf("%s: %s", loc.Label("Current goal", "当前目标"), text))
		}
		if text := strings.TrimSpace(localizeTodoText(snapshot.RoomGoal, loc)); text != "" {
			blocks = append(blocks, fmt.Sprintf("%s: %s", loc.Label("Room goal", "房间目标"), text))
		}
		if text := strings.TrimSpace(localizeTodoText(snapshot.NextIntent, loc)); text != "" {
			blocks = append(blocks, fmt.Sprintf("%s: %s", loc.Label("Next intent", "下一步意图"), text))
		}
	}

	if compact != nil {
		if summary := strings.TrimSpace(compact.PromptBlockForStateLanguage(state, language)); summary != "" {
			blocks = append(blocks, fmt.Sprintf("%s:\n%s", loc.Label("Local run context", "本局局部上下文"), summary))
		}
	}

	if len(history) > 0 {
		blocks = append(blocks, fmt.Sprintf("%s:\n- %s", loc.Label("Recent local history", "最近局部历史"), strings.Join(history, "\n- ")))
	}

	return strings.Join(blocks, "\n\n")
}

func streamerStyleInstruction(style string, loc i18n.Localizer) string {
	switch strings.TrimSpace(strings.ToLower(style)) {
	case "cute":
		return loc.Paragraph(
			"Use a playful and sweet female streamer tone. Keep it lively, flirty, and emotionally expressive without losing accuracy.",
			"语气轻一点、俏一点，带一点可爱和撒娇感，但判断不能飘。",
		)
	case "energetic":
		return loc.Paragraph(
			"Use a brighter, punchier tempo. Lean into excitement, pressure, and momentum swings.",
			"语速更亮、更有冲劲，碰到节奏变化、抢血和翻盘点要明显兴奋起来。",
		)
	case "warm":
		return loc.Paragraph(
			"Use a warm companion-like voice. Stay emotionally present, but softer and more reassuring.",
			"语气更温柔、更像陪伴，情绪在，但不要炸得太满。",
		)
	case "calm":
		return loc.Paragraph(
			"Keep the tone restrained and composed. Use emotion sparingly and keep the read crisp.",
			"语气克制、稳一点，情绪少一些，重点放在准和稳。",
		)
	default:
		return loc.Paragraph(
			"Use a bright-cute female streamer voice: lively, a little playful, emotionally expressive, but still sharp about risk and tempo.",
			"用偏元气、偏可爱的女主播口气，情绪要活一点、俏一点，但对风险和节奏判断要准。",
		)
	}
}

func parseStreamerBeat(text string) (*StreamerBeat, error) {
	raw := strings.TrimSpace(text)
	if raw == "" {
		return nil, fmt.Errorf("empty streamer response")
	}

	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```JSON")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end < start {
		return nil, fmt.Errorf("no json object found")
	}
	raw = raw[start : end+1]

	var beat StreamerBeat
	if err := json.Unmarshal([]byte(raw), &beat); err != nil {
		return nil, err
	}
	if strings.TrimSpace(beat.Commentary) == "" && strings.TrimSpace(beat.TTSText) == "" {
		return nil, fmt.Errorf("empty streamer payload")
	}
	return &beat, nil
}

func (d *StreamerDirector) splitTTSText(ctx context.Context, beat *StreamerBeat, language i18n.Language) []string {
	if beat == nil {
		return nil
	}

	baseText := strings.TrimSpace(beat.TTSText)
	if baseText == "" {
		return nil
	}
	if d == nil || d.runtime == nil || d.runtime.Agent == nil {
		return fallbackTTSSegments(baseText)
	}

	prompt := d.buildSplitPrompt(beat, language)
	result, err := d.runtime.Agent.Prompt(ctx, prompt)
	if err != nil {
		return fallbackTTSSegments(baseText)
	}

	segments, err := parseSpeechSegments(result.Text)
	if err != nil || len(segments) == 0 {
		return fallbackTTSSegments(baseText)
	}
	return segments
}

func (d *StreamerDirector) buildSplitPrompt(beat *StreamerBeat, language i18n.Language) string {
	loc := i18n.New(language)
	blocks := []string{
		loc.Paragraph(
			"You split a short Chinese streamer line into 1 to 4 spoken chunks for TTS rhythm. You only split. Do not rewrite, add ideas, or change tone.",
			"你负责把一小段中文主播台词拆成适合 TTS 朗读的 1 到 4 个短句。你只做切分，不改写意思，不补充观点，不改变语气。",
		),
		loc.Paragraph(
			"Split only when it improves spoken rhythm: emotional pause, surprise beat, afterthought, or dramatic timing. Keep complete thoughts together.",
			"只有在有利于口语节奏时才切分：情绪停顿、意外反应、后半句补刀、戏剧性停顿。完整意思尽量放在一起。",
		),
		loc.Paragraph(
			"Write exactly one JSON object and nothing else.",
			"只输出一个 JSON 对象，不要输出任何额外文字。",
		),
		`{
  "messages": ["短句1", "短句2"]
}`,
		fmt.Sprintf("%s: %s", loc.Label("Mood", "情绪"), strings.TrimSpace(beat.Mood)),
		fmt.Sprintf("%s: %s", loc.Label("Trigger", "触发原因"), strings.TrimSpace(beat.Trigger)),
		fmt.Sprintf("%s:\n%s", loc.Label("Line to split", "待切分台词"), strings.TrimSpace(beat.TTSText)),
	}

	return strings.Join(blocks, "\n\n")
}

func parseSpeechSegments(text string) ([]string, error) {
	raw := strings.TrimSpace(text)
	if raw == "" {
		return nil, fmt.Errorf("empty split response")
	}

	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```JSON")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start < 0 || end < start {
		return nil, fmt.Errorf("no json object found")
	}
	raw = raw[start : end+1]

	var payload struct {
		Messages []string `json:"messages"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, err
	}

	segments := make([]string, 0, len(payload.Messages))
	for _, item := range payload.Messages {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		segments = append(segments, trimmed)
	}
	if len(segments) == 0 {
		return nil, fmt.Errorf("no messages in split response")
	}
	return segments, nil
}

func fallbackTTSSegments(text string) []string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil
	}

	fields := strings.FieldsFunc(trimmed, func(r rune) bool {
		switch r {
		case '。', '！', '？', '；', '\n':
			return true
		default:
			return false
		}
	})
	segments := make([]string, 0, len(fields))
	for _, field := range fields {
		part := strings.TrimSpace(field)
		if part == "" {
			continue
		}
		segments = append(segments, part)
		if len(segments) >= 4 {
			break
		}
	}
	if len(segments) == 0 {
		return []string{trimmed}
	}
	return segments
}

func fallbackStreamerBeat(moment StreamerMoment, text string, language i18n.Language) *StreamerBeat {
	trigger := strings.TrimSpace(moment.Trigger)
	if trigger == "" {
		trigger = deriveStreamerTrigger(moment)
	}

	trimmed := strings.TrimSpace(text)
	if trimmed == "" || looksLikeDecisionJSON(trimmed) {
		return fallbackStreamerBeatFromMoment(moment, trigger, language)
	}
	return &StreamerBeat{
		Trigger:        trigger,
		Mood:           i18n.New(language).Label("restrained", "克制"),
		Commentary:     trimmed,
		GameInsight:    "",
		LifeReflection: "",
		TTSText:        trimmed,
	}
}

func looksLikeDecisionJSON(text string) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, "```json") || strings.HasPrefix(trimmed, "```JSON") {
		trimmed = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(trimmed, "```json"), "```JSON"), "```"))
	}
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start < 0 || end < start {
		return false
	}
	raw := trimmed[start : end+1]
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return false
	}
	if _, ok := payload["commentary"]; ok {
		return false
	}
	_, hasAction := payload["action"]
	_, hasCardIndex := payload["card_index"]
	_, hasTargetIndex := payload["target_index"]
	_, hasOptionIndex := payload["option_index"]
	return hasAction || hasCardIndex || hasTargetIndex || hasOptionIndex
}

func fallbackStreamerBeatFromMoment(moment StreamerMoment, trigger string, language i18n.Language) *StreamerBeat {
	loc := i18n.New(language)
	state := moment.ActiveState()
	screen := "-"
	if state != nil {
		screen = strings.ToUpper(strings.TrimSpace(state.Screen))
	}

	commentary := loc.Label(
		"The board is still settling. Hold the emotional read for one beat and speak from the next clear picture.",
		"这拍局面还在落地，先别抢着下结论。等画面更清楚一点，再把情绪和判断给观众推出来。",
	)
	insight := loc.Label(
		"When the system cannot trust the commentary payload, it should trust the board and wait for a cleaner beat.",
		"当解说结果不可靠时，应该先信局面，再信文本。等下一拍更清楚，解说才有现场感。",
	)
	reflection := loc.Label(
		"People often speak too early because silence feels awkward, not because the picture is clear.",
		"很多人抢着开口，不是因为看清了，而是因为短暂的停顿让人不安。",
	)
	mood := loc.Label("restrained", "克制")

	switch trigger {
	case "combat_opening":
		commentary = loc.Label(
			"The move has landed. Now the real point is whether this hand actually bought breathing room.",
			"动作已经落地。现在真正要讲的，不是点了哪张牌，而是这一手到底有没有把喘息空间抢回来。",
		)
		insight = loc.Label(
			"Combat commentary should follow the real board swing after the action, not the imagined best line before it.",
			"战斗解说要跟着动作之后的真实局势走，不能围着动作之前的设想打转。",
		)
		reflection = loc.Label(
			"压力最狠的时候，人最需要的不是热闹，而是一句判断准的实话。",
			"压力最大的时候，真正值钱的不是热闹，而是一句判断准的实话。",
		)
		mood = loc.Label("tense", "绷住")
	case "map_choice":
		commentary = loc.Label(
			"The route is decided. What matters now is whether this step buys stability or just postpones risk.",
			"路已经走下去了。现在最该说清楚的，是这一步到底买来了稳定，还是只是把风险往后拖了一格。",
		)
		insight = loc.Label(
			"Route commentary is about risk appetite and timing, not about reading node numbers to the audience.",
			"路线解说的价值，在于讲清楚风险偏好和时机，不在于把格子编号念一遍。",
		)
		reflection = loc.Label(
			"很多时候，真正拉开差距的不是胆子大，而是知道什么时候该收一点。",
			"很多时候，真正拉开差距的不是胆子大，而是知道什么时候该收一点。",
		)
		mood = loc.Label("measured", "拿捏")
	case "reward_choice", "card_selection":
		commentary = loc.Label(
			"奖励已经拿了。重点不是这张牌名字好不好听，而是它能不能把这局往更深的层数推过去。",
			"奖励已经拿了。重点不是名字漂不漂亮，而是这一下能不能把这局往更深的层数推过去。",
		)
		insight = loc.Label(
			"奖励解说应该围绕战力兑现和生存压力，不该沦为卡牌目录播报。",
			"奖励解说应该围绕战力兑现和生存压力，不该沦为卡牌目录播报。",
		)
		reflection = loc.Label(
			"真正能改变命运的，往往不是惊天逆转，而是一连串不炫但正确的选择。",
			"真正能改变命运的，往往不是惊天逆转，而是一连串不炫但正确的选择。",
		)
		mood = loc.Label("focused", "上头但清醒")
	case "shop_choice":
		commentary = loc.Label(
			"钱花出去以后，局势才会真的说话。现在要看这笔资源有没有变成活下去的底气。",
			"钱花出去以后，局势才会真的说话。现在要看这笔资源有没有变成活下去的底气。",
		)
		insight = loc.Label(
			"商店真正值得讲的，是资源有没有转成胜率，而不是买单流程。",
			"商店真正值得讲的，是资源有没有转成胜率，而不是买单流程。",
		)
		reflection = loc.Label(
			"人最怕的不是花错钱，而是明明该下注的时候一直攥着不放。",
			"人最怕的不是花错钱，而是明明该下注的时候一直攥着不放。",
		)
		mood = loc.Label("sharp", "带劲")
	case "game_over":
		commentary = loc.Label(
			"这一局已经落幕了。该说的不是可惜，而是到底输在了哪一步的积累。",
			"这一局已经落幕了。该说的不是可惜，而是到底输在了哪一步的积累。",
		)
		insight = loc.Label(
			"Game over commentary should name the real weakness, not stop at a sigh.",
			"结束局的解说要点出真正的薄弱环节，不能只剩一句叹气。",
		)
		reflection = loc.Label(
			"很多结局看上去像一瞬间，其实都是前面无数小决定慢慢推出来的。",
			"很多结局看上去像一瞬间，其实都是前面无数小决定慢慢推出来的。",
		)
		mood = loc.Label("low but clear", "低下来但清醒")
	}

	if screen != "-" {
		commentary += " " + loc.Label("Current screen", "当前画面") + "：" + screen + "。"
	}

	return &StreamerBeat{
		Trigger:        trigger,
		Mood:           mood,
		Commentary:     commentary,
		GameInsight:    insight,
		LifeReflection: reflection,
		TTSText:        commentary,
	}
}

func triggerLabel(trigger string, loc i18n.Localizer) string {
	switch strings.TrimSpace(trigger) {
	case "map_choice":
		return loc.Label("map choice", "地图抉择")
	case "reward_choice":
		return loc.Label("reward choice", "奖励抉择")
	case "event_choice":
		return loc.Label("event choice", "事件抉择")
	case "shop_choice":
		return loc.Label("shop choice", "商店抉择")
	case "rest_choice":
		return loc.Label("rest choice", "休息点抉择")
	case "chest_choice":
		return loc.Label("chest choice", "宝箱抉择")
	case "game_over":
		return loc.Label("game over", "本局结束")
	case "card_selection":
		return loc.Label("card selection", "选牌时刻")
	case "combat_opening":
		return loc.Label("combat reaction", "战斗落地反应")
	default:
		return valueOrDash(trigger)
	}
}

func compactStreamerSignatureText(value string, max int) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	if max <= 3 {
		return string(runes[:max])
	}
	return string(runes[:max-3]) + "..."
}

func (d *StreamerDirector) WriteTTSArtifacts(beat *StreamerBeat) (string, error) {
	if d == nil || beat == nil {
		return "", nil
	}

	if err := os.MkdirAll(filepath.Dir(d.latestPath), 0o755); err != nil {
		return "", err
	}
	if err := os.MkdirAll(d.queueDir, 0o755); err != nil {
		return "", err
	}

	payload, err := json.MarshalIndent(beat, "", "  ")
	if err != nil {
		return "", err
	}
	if err := writeUTF8TextFile(d.latestPath, string(payload)); err != nil {
		return "", err
	}
	if err := writeUTF8TextFile(d.textPath, strings.TrimSpace(beat.TTSText)); err != nil {
		return "", err
	}

	timestamp := time.Now().Format("20060102-150405")
	queuePath := filepath.Join(d.queueDir, timestamp+"-"+sanitizeFileSlug(beat.Trigger)+".json")
	if err := writeUTF8TextFile(queuePath, string(payload)); err != nil {
		return "", err
	}

	return queuePath, nil
}

func sanitizeFileSlug(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "beat"
	}
	replacer := strings.NewReplacer(
		" ", "-",
		"/", "-",
		"\\", "-",
		":", "-",
		"*", "-",
		"?", "-",
		"\"", "-",
		"<", "-",
		">", "-",
		"|", "-",
	)
	value = replacer.Replace(value)
	value = strings.Trim(value, "-.")
	if value == "" {
		return "beat"
	}
	return value
}
