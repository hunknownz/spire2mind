package agentruntime

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"spire2mind/internal/game"
	"spire2mind/internal/i18n"
)

type ActionDecision struct {
	Action      string `json:"action"`
	CardIndex   *int   `json:"card_index,omitempty"`
	TargetIndex *int   `json:"target_index,omitempty"`
	OptionIndex *int   `json:"option_index,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

func ActionDecisionJSONSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Exactly one action name from available_actions.",
			},
			"card_index": map[string]interface{}{
				"type":        "integer",
				"description": "Card index for play_card or other card-based actions when required.",
			},
			"target_index": map[string]interface{}{
				"type":        "integer",
				"description": "Target index for targeted combat actions when required.",
			},
			"option_index": map[string]interface{}{
				"type":        "integer",
				"description": "Choice index for select/choose actions when required.",
			},
			"reason": map[string]interface{}{
				"type":        "string",
				"description": "One short sentence explaining why this legal action is best right now.",
			},
		},
		"required": []string{"action"},
	}
}

func BuildStructuredDecisionPrompt(state *game.StateSnapshot, todo *TodoManager, skills *SkillLibrary, compact *CompactMemory, planner *CombatPlan, language i18n.Language) string {
	var parts []string
	loc := i18n.New(language)

	parts = append(parts, loc.Paragraph(`Choose exactly one legal next action for the live Slay the Spire 2 run.
Return exactly one JSON object and nothing else.

Decision rules:
- Use only actions from available_actions.
- Fill only the indexes that the chosen action actually needs.
- Handle modal dialogs first.
- Prefer forward progress through the run over explanation.
- When multiple lines are viable, choose one concrete legal action now.`, `请为当前这局正在进行的《杀戮尖塔 2》选择一个且仅一个合法的下一步动作。
只返回一个 JSON 对象，不要输出任何其他内容。

决策规则：
- 只能使用 available_actions 中的动作。
- 只填写所选动作真正需要的索引字段。
- 优先处理模态框。
- 以推进对局为先，不要写解释性长文。
- 当有多条可行路线时，立刻选出一条具体且合法的动作。`))

	parts = append(parts, loc.Paragraph(`Output contract:
{
  "action": "one available action string",
  "card_index": 0,
  "target_index": 0,
  "option_index": 0,
  "reason": "short reason"
}

Rules for the JSON object:
- Include only the indexes your chosen action needs.
- Omit unused fields.
- Do not wrap the JSON in markdown fences.
- Do not add commentary before or after the JSON.`, `输出契约：
{
  "action": "某个 available action 字符串",
  "card_index": 0,
  "target_index": 0,
  "option_index": 0,
  "reason": "简短原因"
}

JSON 规则：
- 只保留该动作真正需要的索引字段。
- 未使用字段不要输出。
- 不要用 markdown 代码块包裹 JSON。
- 不要在 JSON 前后添加解释。`))

	if state != nil {
		parts = append(parts, fmt.Sprintf("%s: %s", loc.Label("Current screen", "当前界面"), state.Screen))
		parts = append(parts, fmt.Sprintf("%s: %s", loc.Label("Run id", "Run 标识"), state.RunID))
		parts = append(parts, fmt.Sprintf("%s: %s", loc.Label("Available actions", "当前可用动作"), strings.Join(state.AvailableActions, ", ")))

		// Add human-readable state summary and screen-specific detail so the
		// agent can reason without parsing raw JSON for common fields.
		summaryLines := StateSummaryLines(state)
		detailLines := StateDetailLines(state, 6)
		if len(summaryLines) > 0 || len(detailLines) > 0 {
			var stateBlock []string
			stateBlock = append(stateBlock, loc.Label("State overview", "状态总览")+":")
			for _, line := range summaryLines {
				stateBlock = append(stateBlock, line)
			}
			if len(detailLines) > 0 && detailLines[0] != "- -" {
				stateBlock = append(stateBlock, "")
				stateBlock = append(stateBlock, loc.Label("Room detail", "房间细节")+":")
				for _, line := range detailLines {
					stateBlock = append(stateBlock, line)
				}
			}
			parts = append(parts, strings.Join(stateBlock, "\n"))
		}
	}
	if block := strings.TrimSpace(TacticalHintsBlock(state)); block != "" {
		parts = append(parts, block)
	}
	if planner != nil {
		if block := strings.TrimSpace(planner.PromptBlock(language)); block != "" {
			parts = append(parts, block)
		}
	}

	if todo != nil {
		if block := strings.TrimSpace(todo.PromptBlock()); block != "" {
			parts = append(parts, block)
		}
	}
	if compact != nil {
		if block := strings.TrimSpace(compact.PromptBlock()); block != "" {
			parts = append(parts, block)
		}
	}
	if skills != nil {
		if block := strings.TrimSpace(skills.PromptBlock(state)); block != "" {
			parts = append(parts, block)
		}
	}

	bytes, err := json.MarshalIndent(state, "", "  ")
	if err == nil {
		parts = append(parts, loc.Label("Current state snapshot JSON", "当前状态快照 JSON")+":\n"+string(bytes))
	}

	return strings.Join(parts, "\n\n")
}

func ParseActionDecision(raw interface{}, fallbackText string) (*ActionDecision, error) {
	if raw != nil {
		if decision, err := parseActionDecisionFromRaw(raw); err == nil {
			return decision, nil
		}
	}

	if strings.TrimSpace(fallbackText) == "" {
		return nil, fmt.Errorf("missing structured action decision")
	}

	fallbackText = normalizeDecisionJSONText(fallbackText)
	var decoded interface{}
	if err := json.Unmarshal([]byte(fallbackText), &decoded); err != nil {
		if decision, looseErr := parseLooseActionDecisionText(fallbackText); looseErr == nil {
			return decision, nil
		}
		return nil, fmt.Errorf("parse fallback action decision: %w", err)
	}

	return parseActionDecisionFromRaw(decoded)
}

func parseActionDecisionFromRaw(raw interface{}) (*ActionDecision, error) {
	root, ok := raw.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("structured output is %T, want object", raw)
	}

	action := strings.TrimSpace(stringValue(root["action"]))
	if action == "" {
		return nil, fmt.Errorf("structured output missing action")
	}

	decision := &ActionDecision{
		Action:      action,
		CardIndex:   intPointerValue(root["card_index"]),
		TargetIndex: intPointerValue(root["target_index"]),
		OptionIndex: intPointerValue(root["option_index"]),
		Reason:      strings.TrimSpace(stringValue(root["reason"])),
	}
	if decision.OptionIndex == nil {
		decision.OptionIndex = firstIntPointerValue(
			root["node_index"],
			root["selection_index"],
			root["reward_index"],
			root["shop_index"],
			root["event_option_index"],
			root["index"],
		)
	}

	return decision.normalize(), nil
}

func DecisionToActionRequest(decision *ActionDecision) game.ActionRequest {
	if decision == nil {
		return game.ActionRequest{}
	}

	return game.ActionRequest{
		Action:      decision.Action,
		CardIndex:   decision.CardIndex,
		TargetIndex: decision.TargetIndex,
		OptionIndex: decision.OptionIndex,
	}
}

func actionRequestToDecision(request game.ActionRequest) *ActionDecision {
	return (&ActionDecision{
		Action:      request.Action,
		CardIndex:   request.CardIndex,
		TargetIndex: request.TargetIndex,
		OptionIndex: request.OptionIndex,
	}).normalize()
}

func ValidateActionRequest(state *game.StateSnapshot, request game.ActionRequest) error {
	return ValidateActionDecision(state, actionRequestToDecision(request))
}

func NormalizeActionRequestForState(state *game.StateSnapshot, request game.ActionRequest) game.ActionRequest {
	switch request.Action {
	case "play_card":
		if request.CardIndex != nil {
			if card, ok := combatHandCard(state, *request.CardIndex); ok {
				if !cardRequiresTarget(state, card) {
					request.TargetIndex = nil
					break
				}

				validTargets := combatTargetIndicesForCard(state, card)
				if len(validTargets) == 1 {
					targetIndex := validTargets[0]
					request.TargetIndex = &targetIndex
					break
				}
				if request.TargetIndex != nil && !containsInt(validTargets, *request.TargetIndex) {
					request.TargetIndex = nil
				}
			}
		}
	case "claim_reward":
		claimableRewards := indexedRewardItems(state, "rewards", func(item map[string]any) bool {
			return fieldBool(item, "claimable")
		})
		request = normalizeOptionIndexedRequest(request)
		if request.OptionIndex == nil && len(claimableRewards) == 1 {
			optionIndex := claimableRewards[0]
			request.OptionIndex = &optionIndex
		}
	case "choose_map_node", "choose_reward_card", "select_deck_card",
		"buy_card", "buy_relic", "buy_potion",
		"choose_rest_option", "choose_treasure_relic", "choose_event_option":
		request = normalizeOptionIndexedRequest(request)
		if request.Action == "choose_event_option" && fieldBool(state.Event, "isFinished") {
			request.OptionIndex = nil
		}
	}

	return request
}

func NormalizeActionDecisionForState(state *game.StateSnapshot, decision *ActionDecision) *ActionDecision {
	if decision == nil {
		return nil
	}

	request := NormalizeActionRequestForState(state, DecisionToActionRequest(decision))
	normalized := actionRequestToDecision(request)
	if normalized == nil {
		return nil
	}
	normalized.Reason = decision.Reason
	return normalized
}

func reuseDecisionOnLiveState(expectedState *game.StateSnapshot, liveState *game.StateSnapshot, decision *ActionDecision) (game.ActionRequest, string, bool) {
	if liveState == nil || decision == nil {
		return game.ActionRequest{}, "", false
	}

	remappedDecision, remapped := remapDecisionForLiveState(expectedState, liveState, decision)
	if remappedDecision == nil {
		return game.ActionRequest{}, "", false
	}

	recoveryKind := "decision_reuse"
	if remapped {
		recoveryKind = "decision_remap"
	}

	return NormalizeActionRequestForState(liveState, DecisionToActionRequest(remappedDecision)), recoveryKind, true
}

func shouldQuietDecisionReuse(driftKind string, recoveryKind string, originalRequest game.ActionRequest, reusedRequest game.ActionRequest) bool {
	if recoveryKind != "decision_reuse" || !sameActionRequest(originalRequest, reusedRequest) {
		return false
	}

	switch driftKind {
	case driftKindSameScreenIndexDrift, driftKindSameScreenStateDrift, driftKindActionWindowChanged:
		return true
	default:
		return false
	}
}

func ValidateActionDecision(state *game.StateSnapshot, decision *ActionDecision) error {
	if state == nil {
		return fmt.Errorf("invalid_action: state is unavailable")
	}
	if decision == nil || strings.TrimSpace(decision.Action) == "" {
		return fmt.Errorf("invalid_action: decision is empty")
	}
	if !hasAction(state, decision.Action) {
		return fmt.Errorf("invalid_action: %s is not in available_actions", decision.Action)
	}

	switch decision.Action {
	case "play_card":
		return validatePlayCardDecision(state, decision)
	case "choose_map_node":
		return validateIndexedDecision(decision.OptionIndex, nestedList(state.Map, "availableNodes"), "index", nil, "invalid_action: option_index is out of range for choose_map_node")
	case "select_character":
		return validateIndexedDecision(decision.OptionIndex, nestedList(state.CharacterSelect, "characters"), "index", func(item map[string]any) bool {
			return !fieldBool(item, "isLocked")
		}, "invalid_action: option_index is out of range for select_character")
	case "claim_reward":
		return validateIndexedDecision(decision.OptionIndex, nestedList(state.Reward, "rewards"), "index", func(item map[string]any) bool {
			return fieldBool(item, "claimable")
		}, "invalid_action: option_index is out of range for claim_reward")
	case "choose_reward_card":
		return validateIndexedDecision(decision.OptionIndex, nestedList(state.Reward, "cardOptions"), "index", nil, "invalid_action: option_index is out of range for choose_reward_card")
	case "proceed":
		return validateProceedDecision(state)
	case "confirm_selection":
		return validateConfirmSelectionDecision(state)
	case "choose_event_option":
		if fieldBool(state.Event, "isFinished") {
			if decision.OptionIndex == nil || *decision.OptionIndex == 0 {
				return nil
			}
			return fmt.Errorf("invalid_action: finished events only accept option_index 0 for choose_event_option")
		}
		return validateIndexedDecision(decision.OptionIndex, nestedList(state.Event, "options"), "index", func(item map[string]any) bool {
			return !fieldBool(item, "isLocked")
		}, "invalid_action: option_index is out of range for choose_event_option")
	case "choose_treasure_relic":
		return validateIndexedDecision(decision.OptionIndex, nestedList(state.Chest, "relicOptions"), "index", nil, "invalid_action: option_index is out of range for choose_treasure_relic")
	case "choose_rest_option":
		return validateIndexedDecision(decision.OptionIndex, nestedList(state.Rest, "options"), "index", func(item map[string]any) bool {
			return fieldBool(item, "isEnabled")
		}, "invalid_action: option_index is out of range for choose_rest_option")
	case "buy_card":
		return validateIndexedDecision(decision.OptionIndex, nestedList(state.Shop, "cards"), "index", func(item map[string]any) bool {
			return fieldBool(item, "enoughGold")
		}, "invalid_action: option_index is out of range for buy_card")
	case "buy_relic":
		return validateIndexedDecision(decision.OptionIndex, nestedList(state.Shop, "relics"), "index", func(item map[string]any) bool {
			return fieldBool(item, "enoughGold")
		}, "invalid_action: option_index is out of range for buy_relic")
	case "buy_potion":
		return validateIndexedDecision(decision.OptionIndex, nestedList(state.Shop, "potions"), "index", func(item map[string]any) bool {
			return fieldBool(item, "enoughGold")
		}, "invalid_action: option_index is out of range for buy_potion")
	case "select_deck_card":
		return validateIndexedDecision(decision.OptionIndex, nestedList(state.Selection, "cards"), "index", nil, "invalid_action: option_index is out of range for select_deck_card")
	default:
		return nil
	}
}

func validatePlayCardDecision(state *game.StateSnapshot, decision *ActionDecision) error {
	if decision.CardIndex == nil {
		return fmt.Errorf("invalid_action: card_index is required for play_card")
	}

	card, ok := combatHandCard(state, *decision.CardIndex)
	if !ok {
		return fmt.Errorf("invalid_action: card_index is out of range for play_card")
	}
	if !fieldBool(card, "playable") {
		return fmt.Errorf("invalid_action: selected card is not currently playable")
	}
	if !cardRequiresTarget(state, card) {
		return nil
	}

	if decision.TargetIndex == nil {
		return fmt.Errorf("invalid_target: target_index is required for play_card")
	}

	validTargets := combatTargetIndicesForCard(state, card)
	for _, targetIndex := range validTargets {
		if targetIndex == *decision.TargetIndex {
			return nil
		}
	}

	return fmt.Errorf("invalid_target: target_index is out of range")
}

func validateIndexedDecision(index *int, items []map[string]any, key string, predicate func(map[string]any) bool, errMessage string) error {
	if index == nil {
		return fmt.Errorf("invalid_action: option_index is required")
	}

	for _, item := range items {
		if predicate != nil && !predicate(item) {
			continue
		}
		if itemIndex, ok := fieldInt(item, key); ok && itemIndex == *index {
			return nil
		}
	}

	return errors.New(errMessage)
}

func validateProceedDecision(state *game.StateSnapshot) error {
	if state == nil {
		return fmt.Errorf("invalid_action: state is unavailable")
	}

	if hasUnresolvedRewardOrSelection(state) {
		return fmt.Errorf("invalid_action: proceed is not available while reward or selection choices remain")
	}

	return nil
}

func validateConfirmSelectionDecision(state *game.StateSnapshot) error {
	if state == nil {
		return fmt.Errorf("invalid_action: state is unavailable")
	}

	if !fieldBool(state.Selection, "requiresConfirmation") || !fieldBool(state.Selection, "canConfirm") {
		return fmt.Errorf("invalid_action: confirm_selection is not available for the current selection")
	}

	return nil
}

func hasUnresolvedRewardOrSelection(state *game.StateSnapshot) bool {
	if state == nil {
		return false
	}

	if fieldBool(state.Reward, "pendingCardChoice") {
		return true
	}
	if len(indexedRewardItems(state, "rewards", func(item map[string]any) bool {
		return fieldBool(item, "claimable")
	})) > 0 {
		return true
	}
	if len(nestedList(state.Reward, "cardOptions")) > 0 {
		return true
	}
	if len(nestedList(state.Selection, "cards")) > 0 {
		return true
	}
	if fieldBool(state.Selection, "requiresConfirmation") && fieldBool(state.Selection, "canConfirm") {
		return true
	}

	return false
}

func indexedRewardItems(state *game.StateSnapshot, key string, predicate func(map[string]any) bool) []int {
	var indices []int
	for _, item := range nestedList(state.Reward, key) {
		if predicate != nil && !predicate(item) {
			continue
		}
		if index, ok := fieldInt(item, "index"); ok {
			indices = append(indices, index)
		}
	}
	return indices
}

func combatHandCard(state *game.StateSnapshot, cardIndex int) (map[string]any, bool) {
	for _, card := range nestedList(state.Combat, "hand") {
		if index, ok := fieldInt(card, "index"); ok && index == cardIndex {
			return card, true
		}
	}

	return nil, false
}

func combatValidTargets(state *game.StateSnapshot) []int {
	return combatEnemyTargetIndices(state)
}

func combatTargetIndicesForCard(state *game.StateSnapshot, card map[string]any) []int {
	if len(card) == 0 {
		return nil
	}
	if state == nil {
		return nil
	}

	if validTargets := append([]int(nil), fieldIntSlice(card, "validTargetIndices")...); len(validTargets) > 0 {
		return validTargets
	}

	switch strings.TrimSpace(fieldString(card, "targetType")) {
	case "AnyEnemy":
		return combatEnemyTargetIndices(state)
	case "AnyAlly":
		return combatAllyTargetIndices(state)
	default:
		return nil
	}
}

func cardRequiresTarget(state *game.StateSnapshot, card map[string]any) bool {
	if len(card) == 0 {
		return false
	}
	if fieldBool(card, "requiresTarget") {
		return true
	}
	if len(fieldIntSlice(card, "validTargetIndices")) > 0 {
		return true
	}
	switch strings.TrimSpace(fieldString(card, "targetType")) {
	case "", "None", "Self":
		return false
	default:
		if state == nil {
			return true
		}
		return len(combatTargetIndicesForCard(state, card)) > 0
	}
}

func combatEnemyTargetIndices(state *game.StateSnapshot) []int {
	var valid []int
	for _, enemy := range nestedList(state.Combat, "enemies") {
		if !fieldBool(enemy, "isHittable") {
			continue
		}
		if index, ok := fieldInt(enemy, "index"); ok {
			valid = append(valid, index)
		}
	}

	return valid
}

func combatAllyTargetIndices(state *game.StateSnapshot) []int {
	var valid []int
	for _, ally := range nestedList(state.Combat, "allies") {
		if !fieldBool(ally, "isAlive") {
			continue
		}
		if index, ok := fieldInt(ally, "index"); ok {
			valid = append(valid, index)
		}
	}

	return valid
}

func containsInt(values []int, needle int) bool {
	for _, value := range values {
		if value == needle {
			return true
		}
	}

	return false
}

func intPointerValue(value interface{}) *int {
	switch typed := value.(type) {
	case int:
		return &typed
	case int32:
		value := int(typed)
		return &value
	case int64:
		value := int(typed)
		return &value
	case float64:
		value := int(typed)
		return &value
	case json.Number:
		if parsed, err := typed.Int64(); err == nil {
			value := int(parsed)
			return &value
		}
	case string:
		if parsed, err := strconv.Atoi(strings.TrimSpace(typed)); err == nil {
			return &parsed
		}
	}

	return nil
}

func firstIntPointerValue(values ...interface{}) *int {
	for _, value := range values {
		if parsed := intPointerValue(value); parsed != nil {
			return parsed
		}
	}
	return nil
}

func stringValue(value interface{}) string {
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return ""
	}
}

func normalizeDecisionJSONText(value string) string {
	trimmed := strings.TrimSpace(value)
	trimmed = strings.TrimPrefix(trimmed, "```json")
	trimmed = strings.TrimPrefix(trimmed, "```")
	trimmed = strings.TrimSuffix(trimmed, "```")
	trimmed = strings.TrimSpace(trimmed)

	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end >= start {
		return trimmed[start : end+1]
	}

	return trimmed
}

var (
	looseDecisionActionPattern = regexp.MustCompile(`"action"\s*:\s*"([^"]+)"`)
	looseDecisionCardPattern   = regexp.MustCompile(`"card_index"\s*:\s*([0-9]+)`)
	looseDecisionTargetPattern = regexp.MustCompile(`"target_index"\s*:\s*([0-9]+)`)
	looseDecisionOptionPattern = regexp.MustCompile(`"option_index"\s*:\s*([0-9]+)`)
	looseDecisionReasonPattern = regexp.MustCompile(`"reason"\s*:\s*"([^"]*)`)
)

func parseLooseActionDecisionText(text string) (*ActionDecision, error) {
	actionMatch := looseDecisionActionPattern.FindStringSubmatch(text)
	if len(actionMatch) < 2 {
		return nil, fmt.Errorf("loose action decision is missing action")
	}

	decision := &ActionDecision{
		Action: strings.TrimSpace(actionMatch[1]),
	}
	if value, ok := parseLooseDecisionInt(looseDecisionCardPattern, text); ok {
		decision.CardIndex = &value
	}
	if value, ok := parseLooseDecisionInt(looseDecisionTargetPattern, text); ok {
		decision.TargetIndex = &value
	}
	if value, ok := parseLooseDecisionInt(looseDecisionOptionPattern, text); ok {
		decision.OptionIndex = &value
	}
	if reasonMatch := looseDecisionReasonPattern.FindStringSubmatch(text); len(reasonMatch) >= 2 {
		decision.Reason = strings.TrimSpace(reasonMatch[1])
	}

	return decision.normalize(), nil
}

func parseLooseDecisionInt(pattern *regexp.Regexp, text string) (int, bool) {
	match := pattern.FindStringSubmatch(text)
	if len(match) < 2 {
		return 0, false
	}

	value, err := strconv.Atoi(strings.TrimSpace(match[1]))
	if err != nil {
		return 0, false
	}

	return value, true
}

func (d *ActionDecision) normalize() *ActionDecision {
	if d == nil {
		return nil
	}

	switch strings.TrimSpace(d.Action) {
	case "claim_reward", "choose_reward_card", "select_deck_card", "buy_card", "buy_relic", "buy_potion",
		"choose_map_node", "choose_rest_option", "choose_treasure_relic", "choose_event_option":
		if d.OptionIndex == nil {
			if d.CardIndex != nil {
				d.OptionIndex = d.CardIndex
			} else if d.TargetIndex != nil {
				d.OptionIndex = d.TargetIndex
			}
		}
		d.CardIndex = nil
		d.TargetIndex = nil
	case "play_card":
		if d.CardIndex == nil && d.OptionIndex != nil {
			d.CardIndex = d.OptionIndex
		}
		d.OptionIndex = nil
	}

	return d
}

func normalizeOptionIndexedRequest(request game.ActionRequest) game.ActionRequest {
	if request.OptionIndex == nil {
		if request.CardIndex != nil {
			request.OptionIndex = request.CardIndex
		} else if request.TargetIndex != nil {
			request.OptionIndex = request.TargetIndex
		}
	}
	request.CardIndex = nil
	request.TargetIndex = nil
	return request
}
