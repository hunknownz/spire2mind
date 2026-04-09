package agentruntime

import (
	"fmt"
	"strings"

	"spire2mind/internal/game"
	"spire2mind/internal/i18n"
)

type TodoManager struct {
	currentGoal         string
	roomGoal            string
	nextIntent          string
	lastFailure         string
	failures            []string
	rewardLoop          rewardLoopMemory
	carryForwardPlan    string
	carryForwardLessons []string
	carryForwardBuckets ReflectionLessonBuckets
}

type rewardLoopMemory struct {
	runID   string
	active  bool
	handled string
}

type TodoSnapshot struct {
	CurrentGoal         string
	RoomGoal            string
	NextIntent          string
	LastFailure         string
	CarryForwardPlan    string
	CarryForwardLessons []string
	CarryForwardBuckets ReflectionLessonBuckets
}

func NewTodoManager() *TodoManager {
	return &TodoManager{
		currentGoal: "Start or continue a single-player run.",
	}
}

func (t *TodoManager) Update(state *game.StateSnapshot) {
	if state == nil {
		return
	}

	if !strings.EqualFold(state.Screen, "REWARD") || state.RunID != t.rewardLoop.runID {
		t.rewardLoop = rewardLoopMemory{}
	}
	if rewardHasPendingCardChoice(state) {
		t.rewardLoop.active = false
	}

	switch state.Screen {
	case "MAIN_MENU":
		if hasAction(state, "continue_run") {
			t.currentGoal = "Resume the existing run from the main menu."
			t.roomGoal = "Use continue_run."
			t.nextIntent = "Continue the save unless the game blocks it."
		} else {
			t.currentGoal = "Start a new single-player run."
			t.roomGoal = "Enter character select and embark."
			t.nextIntent = "Open character select, choose a strong unlocked character, then embark."
		}
	case "CHARACTER_SELECT":
		t.currentGoal = "Launch a fresh run."
		t.roomGoal = "Choose a character and embark."
		t.nextIntent = "Prefer the first unlocked non-random character unless a better reason appears."
	case "MAP":
		t.currentGoal = "Advance the run through the map."
		t.roomGoal = "Choose the strongest reasonable node."
		t.nextIntent = "Use map information to pick the next node and keep momentum."
	case "COMBAT":
		t.currentGoal = "Win the current combat."
		t.roomGoal = "Play the best legal turn."
		t.nextIntent = "Use available combat actions to maximize survival and damage."
	case "REWARD":
		t.currentGoal = "Resolve the reward screen cleanly."
		t.roomGoal = "Claim rewards, pick or skip cards, then proceed."
		t.nextIntent = "Do not leave reward value behind unless skipping is clearly better."
	case "EVENT":
		t.currentGoal = "Resolve the current event."
		t.roomGoal = "Pick the strongest available option and proceed."
		t.nextIntent = "Prefer options with durable upside and avoid obvious run-killing downside."
	case "SHOP":
		t.currentGoal = "Extract value from the merchant without stalling."
		t.roomGoal = "Open inventory if useful, evaluate purchases, then proceed."
		t.nextIntent = "Do not waste gold on weak buys; prioritize card removal and strong relics."
	case "REST":
		t.currentGoal = "Use the rest site well."
		t.roomGoal = "Choose the best rest option, complete selection if needed, then proceed."
		t.nextIntent = "Prefer smith or high-value rest actions when safe."
	case "CHEST":
		t.currentGoal = "Resolve the treasure room."
		t.roomGoal = "Open the chest, take the best relic, then proceed."
		t.nextIntent = "Do not leave the relic unclaimed."
	case "CARD_SELECTION":
		t.currentGoal = "Finish the current card selection."
		t.roomGoal = "Choose the best required card and confirm if needed."
		t.nextIntent = "Use the prompt and current run context to make the card choice."
	case "MODAL":
		t.currentGoal = "Clear the blocking modal."
		t.roomGoal = "Confirm or dismiss the modal immediately."
		t.nextIntent = "Remove the modal before anything else."
	case "GAME_OVER":
		t.currentGoal = "Resolve the game over screen cleanly."
		t.roomGoal = "Advance the summary and return to the main menu if another attempt should start."
		t.nextIntent = "Use the exposed game over actions to close this run before deciding whether to start the next attempt."
	default:
		t.currentGoal = "Keep the run moving forward."
		t.roomGoal = "Inspect the live state and choose the next best action."
		t.nextIntent = "Avoid stalling and prefer legal forward progress."
	}
}

func (t *TodoManager) RecordFailure(action string, err error) {
	if err == nil {
		return
	}

	t.lastFailure = fmt.Sprintf("%s failed: %v", action, err)
	t.failures = appendTrimmed(t.failures, t.lastFailure, 8)
}

func (t *TodoManager) RecordAction(action string, beforeState *game.StateSnapshot, afterState *game.StateSnapshot) {
	t.lastFailure = ""
	switch action {
	case "skip_reward_cards", "choose_reward_card":
		if beforeState != nil && strings.EqualFold(beforeState.Screen, "REWARD") {
			t.rewardLoop = rewardLoopMemory{
				runID:   beforeState.RunID,
				active:  true,
				handled: action,
			}
		}
	case "proceed":
		t.rewardLoop = rewardLoopMemory{}
	}

	if afterState != nil && !strings.EqualFold(afterState.Screen, "REWARD") {
		t.rewardLoop = rewardLoopMemory{}
	}

	if afterState == nil {
		return
	}

	t.nextIntent = fmt.Sprintf("After %s, reassess the %s state and keep moving.", action, strings.ToLower(afterState.Screen))
}

func (t *TodoManager) PromptBlock() string {
	return t.PromptBlockForLanguage(i18n.LanguageEnglish)
}

func (t *TodoManager) PromptBlockForLanguage(language i18n.Language) string {
	loc := i18n.New(language)
	lines := []string{
		loc.Label("Current goal", "当前目标") + ": " + localizeTodoText(t.currentGoal, loc),
		loc.Label("Room goal", "当前房间目标") + ": " + localizeTodoText(t.roomGoal, loc),
		loc.Label("Next intent", "下一步意图") + ": " + localizeTodoText(t.nextIntent, loc),
	}
	if t.lastFailure != "" {
		lines = append(lines, loc.Label("Recent failure", "最近失败")+": "+localizeTodoText(t.lastFailure, loc))
	}
	if t.carryForwardPlan != "" {
		lines = append(lines, loc.Label("Carry-forward plan", "沿用计划")+": "+localizeTodoText(t.carryForwardPlan, loc))
	}
	if !t.carryForwardBuckets.IsEmpty() {
		lines = append(lines, loc.Label("Carry-forward lessons by category", "按类别整理的沿用经验")+":")
		for _, section := range t.carryForwardBuckets.Sections() {
			lines = append(lines, "- "+localizeLessonBucketTitle(section.Title, loc)+":")
			for _, lesson := range section.Lessons {
				lines = append(lines, "  - "+localizeTodoText(lesson, loc))
			}
		}
	}
	if remaining := UncategorizedLessons(t.carryForwardLessons, t.carryForwardBuckets, 8); len(remaining) > 0 {
		localized := make([]string, 0, len(remaining))
		for _, lesson := range remaining {
			localized = append(localized, localizeTodoText(lesson, loc))
		}
		lines = append(lines, loc.Label("Carry-forward lessons", "沿用经验")+":\n- "+strings.Join(localized, "\n- "))
	}

	return strings.Join(lines, "\n")
}

func (t *TodoManager) PromptBlockCompact() string {
	return t.PromptBlockCompactForLanguage(i18n.LanguageEnglish)
}

func (t *TodoManager) PromptBlockCompactForLanguage(language i18n.Language) string {
	if t == nil {
		return ""
	}

	loc := i18n.New(language)
	lines := []string{
		loc.Label("Current goal", "当前目标") + ": " + localizeTodoText(t.currentGoal, loc),
		loc.Label("Room goal", "当前房间目标") + ": " + localizeTodoText(t.roomGoal, loc),
		loc.Label("Next intent", "下一步意图") + ": " + localizeTodoText(t.nextIntent, loc),
	}
	if t.lastFailure != "" {
		lines = append(lines, loc.Label("Recent failure", "最近失败")+": "+localizeTodoText(t.lastFailure, loc))
	}
	if t.carryForwardPlan != "" {
		lines = append(lines, loc.Label("Carry-forward plan", "沿用计划")+": "+localizeTodoText(t.carryForwardPlan, loc))
	}
	if remaining := UncategorizedLessons(t.carryForwardLessons, t.carryForwardBuckets, 3); len(remaining) > 0 {
		localized := make([]string, 0, len(remaining))
		for _, lesson := range remaining {
			localized = append(localized, localizeTodoText(lesson, loc))
		}
		lines = append(lines, loc.Label("Carry-forward lessons", "沿用经验")+":\n- "+strings.Join(localized, "\n- "))
	}
	return strings.Join(lines, "\n")
}

func (t *TodoManager) FailureHistory(limit int) []string {
	if limit <= 0 || len(t.failures) == 0 {
		return nil
	}

	if len(t.failures) <= limit {
		return append([]string(nil), t.failures...)
	}

	return append([]string(nil), t.failures[len(t.failures)-limit:]...)
}

func (t *TodoManager) ApplyReflection(reflection *AttemptReflection) {
	if reflection == nil {
		return
	}

	if plan := strings.TrimSpace(reflection.NextPlan); plan != "" {
		t.carryForwardPlan = plan
	}
	buckets := reflection.LessonBuckets
	if buckets.IsEmpty() {
		buckets = InferLessonBuckets(reflection.Lessons)
	}
	t.carryForwardBuckets.Merge(buckets, 4)
	for _, lesson := range reflection.Lessons {
		t.carryForwardLessons = appendUniqueTrimmed(t.carryForwardLessons, lesson, 8)
	}
}

func (t *TodoManager) ApplyResume(resume *SessionResumeState) {
	if resume == nil {
		return
	}

	if plan := strings.TrimSpace(resume.CarryForwardPlan); plan != "" {
		t.carryForwardPlan = plan
	}
	buckets := resume.CarryForwardBuckets
	if buckets.IsEmpty() {
		buckets = InferLessonBuckets(resume.CarryForwardLessons)
	}
	t.carryForwardBuckets.Merge(buckets, 4)
	for _, lesson := range resume.CarryForwardLessons {
		t.carryForwardLessons = appendUniqueTrimmed(t.carryForwardLessons, lesson, 8)
	}
}

func (t *TodoManager) Snapshot() TodoSnapshot {
	return TodoSnapshot{
		CurrentGoal:         t.currentGoal,
		RoomGoal:            t.roomGoal,
		NextIntent:          t.nextIntent,
		LastFailure:         t.lastFailure,
		CarryForwardPlan:    t.carryForwardPlan,
		CarryForwardLessons: append([]string(nil), t.carryForwardLessons...),
		CarryForwardBuckets: t.carryForwardBuckets.Clone(),
	}
}

func (t *TodoManager) ShouldProceedAfterResolvedCardReward(state *game.StateSnapshot) bool {
	if state == nil || !t.rewardLoop.active || t.rewardLoop.runID == "" {
		return false
	}
	if state.RunID != t.rewardLoop.runID || !strings.EqualFold(state.Screen, "REWARD") {
		return false
	}
	if rewardHasPendingCardChoice(state) {
		return false
	}
	if !hasAction(state, "proceed") || !hasAction(state, "claim_reward") || hasAction(state, "choose_reward_card") {
		return false
	}

	claimableRewards := 0
	for _, reward := range nestedList(state.Reward, "rewards") {
		if !fieldBool(reward, "claimable") {
			continue
		}
		claimableRewards++
		rewardType := strings.ToLower(fieldString(reward, "rewardType"))
		if !strings.Contains(rewardType, "card") {
			return false
		}
	}

	return claimableRewards > 0
}

func rewardHasPendingCardChoice(state *game.StateSnapshot) bool {
	if state == nil {
		return false
	}

	return fieldBool(state.Reward, "pendingCardChoice") || len(nestedList(state.Reward, "cardOptions")) > 0
}

func hasAction(state *game.StateSnapshot, action string) bool {
	if state == nil {
		return false
	}

	for _, candidate := range state.AvailableActions {
		if candidate == action {
			return true
		}
	}

	return false
}

func localizeTodoText(text string, loc i18n.Localizer) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	if loc.Language() == i18n.LanguageEnglish {
		return text
	}

	switch text {
	case "Start or continue a single-player run.":
		return "开始或继续当前单人跑局。"
	case "Resume the existing run from the main menu.":
		return "从主菜单继续当前已有跑局。"
	case "Use continue_run.":
		return "执行 continue_run。"
	case "Continue the save unless the game blocks it.":
		return "优先继续当前存档，除非游戏阻止。"
	case "Start a new single-player run.":
		return "开始一局新的单人跑局。"
	case "Enter character select and embark.":
		return "进入角色选择并出发。"
	case "Open character select, choose a strong unlocked character, then embark.":
		return "打开角色选择，选一个强度稳定的已解锁角色，然后出发。"
	case "Launch a fresh run.":
		return "开启一局新跑局。"
	case "Choose a character and embark.":
		return "选择角色并出发。"
	case "Prefer the first unlocked non-random character unless a better reason appears.":
		return "默认优先第一个已解锁且非随机的角色，除非当前有更好的理由。"
	case "Advance the run through the map.":
		return "通过地图推进这一局。"
	case "Choose the strongest reasonable node.":
		return "选择当前最合理、最强的一条节点。"
	case "Use map information to pick the next node and keep momentum.":
		return "结合地图信息选择下一节点，保持推进节奏。"
	case "Win the current combat.":
		return "赢下当前战斗。"
	case "Play the best legal turn.":
		return "打出当前最好的合法回合。"
	case "Use available combat actions to maximize survival and damage.":
		return "利用当前可用战斗动作，兼顾生存和输出最大化。"
	case "Resolve the reward screen cleanly.":
		return "干净地处理奖励界面。"
	case "Claim rewards, pick or skip cards, then proceed.":
		return "领取奖励，选牌或跳牌，然后继续。"
	case "Do not leave reward value behind unless skipping is clearly better.":
		return "除非明显更该跳过，否则不要白白留下奖励价值。"
	case "Resolve the current event.":
		return "处理当前事件。"
	case "Pick the strongest available option and proceed.":
		return "选择当前最强的可用选项并继续。"
	case "Prefer options with durable upside and avoid obvious run-killing downside.":
		return "优先长期稳定收益，避开明显会毁局的负面选项。"
	case "Extract value from the merchant without stalling.":
		return "在不拖节奏的前提下把商店价值吃满。"
	case "Open inventory if useful, evaluate purchases, then proceed.":
		return "有需要就先看背包，评估购买后继续。"
	case "Do not waste gold on weak buys; prioritize card removal and strong relics.":
		return "不要把金币花在低价值购买上，优先移除卡牌和强力遗物。"
	case "Use the rest site well.":
		return "把火堆收益用好。"
	case "Choose the best rest option, complete selection if needed, then proceed.":
		return "选择最好的火堆动作，如有后续选牌就完成它，然后继续。"
	case "Prefer smith or high-value rest actions when safe.":
		return "安全时优先锻造或高价值火堆动作。"
	case "Resolve the treasure room.":
		return "处理宝箱房。"
	case "Open the chest, take the best relic, then proceed.":
		return "开箱，拿最好的遗物，然后继续。"
	case "Do not leave the relic unclaimed.":
		return "不要漏拿遗物。"
	case "Finish the current card selection.":
		return "完成当前选牌。"
	case "Choose the best required card and confirm if needed.":
		return "选出当前最好的必要卡牌，如需确认就确认。"
	case "Use the prompt and current run context to make the card choice.":
		return "结合当前提示和整局上下文完成选牌。"
	case "Clear the blocking modal.":
		return "先清掉阻塞弹窗。"
	case "Confirm or dismiss the modal immediately.":
		return "立刻确认或关闭弹窗。"
	case "Remove the modal before anything else.":
		return "先处理弹窗，再做其他事。"
	case "Resolve the game over screen cleanly.":
		return "干净地处理结算界面。"
	case "Advance the summary and return to the main menu if another attempt should start.":
		return "推进结算流程，如需再开新局则回到主菜单。"
	case "Use the exposed game over actions to close this run before deciding whether to start the next attempt.":
		return "先用当前可用的结算动作关掉这局，再决定是否开始下一局。"
	case "Keep the run moving forward.":
		return "保持这局继续向前推进。"
	case "Inspect the live state and choose the next best action.":
		return "查看当前实时状态并选出下一步最优动作。"
	case "Avoid stalling and prefer legal forward progress.":
		return "不要停滞，优先一切合法的向前推进。"
	}

	if strings.HasPrefix(text, "After ") && strings.Contains(text, ", reassess the ") && strings.HasSuffix(text, " state and keep moving.") {
		rest := strings.TrimPrefix(text, "After ")
		parts := strings.SplitN(rest, ", reassess the ", 2)
		if len(parts) == 2 {
			screen := strings.TrimSuffix(parts[1], " state and keep moving.")
			return fmt.Sprintf("%s 之后，重新判断当前 %s 状态并继续推进。", parts[0], screen)
		}
	}
	if strings.Contains(text, " failed: ") {
		parts := strings.SplitN(text, " failed: ", 2)
		if len(parts) == 2 {
			return fmt.Sprintf("%s 失败：%s", parts[0], parts[1])
		}
	}
	return text
}

func localizeLessonBucketTitle(title string, loc i18n.Localizer) string {
	if loc.Language() == i18n.LanguageEnglish {
		return title
	}
	switch strings.TrimSpace(title) {
	case "Combat survival":
		return "战斗生存"
	case "Pathing":
		return "路线规划"
	case "Reward choice":
		return "奖励选择"
	case "Shop economy":
		return "商店经济"
	case "Runtime":
		return "运行时"
	default:
		return title
	}
}

func localizeTodoSlice(items []string, loc i18n.Localizer) []string {
	if len(items) == 0 {
		return nil
	}
	localized := make([]string, 0, len(items))
	for _, item := range items {
		localized = append(localized, localizeTodoText(item, loc))
	}
	return localized
}
