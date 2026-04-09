package agentruntime

import (
	"fmt"
	"strings"

	"spire2mind/internal/game"
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
	lines := []string{
		"Current goal: " + t.currentGoal,
		"Room goal: " + t.roomGoal,
		"Next intent: " + t.nextIntent,
	}
	if t.lastFailure != "" {
		lines = append(lines, "Recent failure: "+t.lastFailure)
	}
	if t.carryForwardPlan != "" {
		lines = append(lines, "Carry-forward plan: "+t.carryForwardPlan)
	}
	if !t.carryForwardBuckets.IsEmpty() {
		lines = append(lines, "Carry-forward lessons by category:")
		for _, section := range t.carryForwardBuckets.Sections() {
			lines = append(lines, "- "+section.Title+":")
			for _, lesson := range section.Lessons {
				lines = append(lines, "  - "+lesson)
			}
		}
	}
	if remaining := UncategorizedLessons(t.carryForwardLessons, t.carryForwardBuckets, 8); len(remaining) > 0 {
		lines = append(lines, "Carry-forward lessons:\n- "+strings.Join(remaining, "\n- "))
	}

	return strings.Join(lines, "\n")
}

func (t *TodoManager) PromptBlockCompact() string {
	if t == nil {
		return ""
	}

	lines := []string{
		"Current goal: " + t.currentGoal,
		"Room goal: " + t.roomGoal,
		"Next intent: " + t.nextIntent,
	}
	if t.lastFailure != "" {
		lines = append(lines, "Recent failure: "+t.lastFailure)
	}
	if t.carryForwardPlan != "" {
		lines = append(lines, "Carry-forward plan: "+t.carryForwardPlan)
	}
	if remaining := UncategorizedLessons(t.carryForwardLessons, t.carryForwardBuckets, 3); len(remaining) > 0 {
		lines = append(lines, "Carry-forward lessons:\n- "+strings.Join(remaining, "\n- "))
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
