package game

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"spire2mind/internal/i18n"
)

const (
	actionableStatePollInterval = 250 * time.Millisecond
	actionableStateSettlePoll   = 75 * time.Millisecond
	actionableStateSettleWindow = 150 * time.Millisecond
	actionableStateSettleMax    = 750 * time.Millisecond
	combatStateSettleWindow     = 425 * time.Millisecond
	combatStateSettleMax        = 1750 * time.Millisecond
	combatEdgeStateSettleWindow = 900 * time.Millisecond
	combatEdgeStateSettleMax    = 6 * time.Second
	rewardStateSettleWindow     = 225 * time.Millisecond
	rewardStateSettleMax        = 1100 * time.Millisecond
	selectionStateSettleWindow  = 225 * time.Millisecond
	selectionStateSettleMax     = 1100 * time.Millisecond
	gameOverStateSettleWindow   = 200 * time.Millisecond
	gameOverStateSettleMax      = 900 * time.Millisecond
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (c *Client) GetHealth(ctx context.Context) (*Health, error) {
	health := new(Health)
	if err := c.get(ctx, "/health", nil, health); err != nil {
		return nil, err
	}
	return health, nil
}

func (c *Client) GetState(ctx context.Context) (*StateSnapshot, error) {
	state := new(StateSnapshot)
	if err := c.get(ctx, "/state", nil, state); err != nil {
		return nil, err
	}
	return NormalizeStateSnapshot(state), nil
}

func (c *Client) GetMarkdownState(ctx context.Context) (*MarkdownState, error) {
	result := new(MarkdownState)
	if err := c.get(ctx, "/state", map[string]string{"format": "markdown"}, result); err != nil {
		return nil, err
	}
	result.Markdown = i18n.RepairText(result.Markdown)
	NormalizeStateSnapshot(&result.Snapshot)
	return result, nil
}

func (c *Client) GetAvailableActions(ctx context.Context) (*AvailableActions, error) {
	result := new(AvailableActions)
	if err := c.get(ctx, "/actions/available", nil, result); err != nil {
		return nil, err
	}
	result.Screen = i18n.RepairText(result.Screen)
	result.AvailableActions = normalizeStringSlice(result.AvailableActions)
	for i := range result.Descriptors {
		result.Descriptors[i].Action = i18n.RepairText(result.Descriptors[i].Action)
		result.Descriptors[i].Description = i18n.RepairText(result.Descriptors[i].Description)
		result.Descriptors[i].RequiredParameters = normalizeStringSlice(result.Descriptors[i].RequiredParameters)
		result.Descriptors[i].OptionalParameters = normalizeStringSlice(result.Descriptors[i].OptionalParameters)
	}
	return result, nil
}

func (c *Client) Act(ctx context.Context, request ActionRequest) (*ActionResult, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("marshal action request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/action", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build action request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("post /action: %w", err)
	}
	defer resp.Body.Close()

	envelope := Envelope[ActionResult]{}
	if err := decodeEnvelope(resp, &envelope); err != nil {
		return nil, err
	}
	envelope.Data.Action = i18n.RepairText(envelope.Data.Action)
	envelope.Data.Status = i18n.RepairText(envelope.Data.Status)
	envelope.Data.Message = i18n.RepairText(envelope.Data.Message)
	NormalizeStateSnapshot(&envelope.Data.State)
	return &envelope.Data, nil
}

func (c *Client) WaitUntilActionable(ctx context.Context, timeout time.Duration) (*StateSnapshot, error) {
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	state, err := c.GetState(waitCtx)
	if err != nil {
		return nil, err
	}
	if IsActionableState(state) {
		return c.StabilizeActionableState(waitCtx, state)
	}

	type waitResult struct {
		state *StateSnapshot
		err   error
	}

	resultCh := make(chan waitResult, 2)

	go func() {
		state, err := c.waitUntilActionableViaEvents(waitCtx)
		resultCh <- waitResult{state: state, err: err}
	}()

	go func() {
		state, err := c.waitUntilActionableByPolling(waitCtx)
		resultCh <- waitResult{state: state, err: err}
	}()

	var lastErr error
	for i := 0; i < 2; i++ {
		select {
		case <-waitCtx.Done():
			if finalState, finalErr := c.getFinalActionableState(); finalErr == nil && IsActionableState(finalState) {
				return finalState, nil
			}
			if lastErr != nil {
				return nil, fmt.Errorf("%w (last wait error: %v)", waitCtx.Err(), lastErr)
			}
			return nil, waitCtx.Err()
		case result := <-resultCh:
			if result.err == nil && IsActionableState(result.state) {
				return c.StabilizeActionableState(waitCtx, result.state)
			}
			if result.err != nil {
				lastErr = result.err
			}
		}
	}

	if finalState, finalErr := c.getFinalActionableState(); finalErr == nil && IsActionableState(finalState) {
		return c.StabilizeActionableState(waitCtx, finalState)
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, context.DeadlineExceeded
}

func (c *Client) StabilizeActionableState(ctx context.Context, initial *StateSnapshot) (*StateSnapshot, error) {
	return c.stabilizeActionableState(ctx, initial)
}

func (c *Client) get(ctx context.Context, path string, query map[string]string, out any) error {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return fmt.Errorf("build url for %s: %w", path, err)
	}

	if len(query) > 0 {
		values := u.Query()
		for key, value := range query {
			values.Set(key, value)
		}
		u.RawQuery = values.Encode()
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("build request for %s: %w", path, err)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("get %s: %w", path, err)
	}
	defer resp.Body.Close()

	envelope := Envelope[json.RawMessage]{}
	if err := decodeEnvelope(resp, &envelope); err != nil {
		return err
	}

	if err := json.Unmarshal(envelope.Data, out); err != nil {
		return fmt.Errorf("decode %s payload: %w", path, err)
	}

	return nil
}

func decodeEnvelope[T any](resp *http.Response, envelope *Envelope[T]) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if err := json.Unmarshal(body, envelope); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	if !envelope.OK {
		if envelope.Error == nil {
			return fmt.Errorf("bridge returned http %d", resp.StatusCode)
		}
		return fmt.Errorf("%s: %s", envelope.Error.Code, envelope.Error.Message)
	}

	return nil
}

func (c *Client) waitUntilActionableViaEvents(ctx context.Context) (*StateSnapshot, error) {
	eventCh, errCh := c.Subscribe(ctx)
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case err, ok := <-errCh:
			if !ok || err == nil {
				return nil, fmt.Errorf("event stream closed")
			}

			return nil, err
		case event, ok := <-eventCh:
			if !ok {
				return nil, fmt.Errorf("event stream closed")
			}

			if !isUsefulActionableEvent(event.Type) {
				continue
			}

			state, err := c.GetState(ctx)
			if err != nil {
				return nil, err
			}
			if IsActionableState(state) {
				return state, nil
			}
		}
	}
}

func (c *Client) waitUntilActionableByPolling(ctx context.Context) (*StateSnapshot, error) {
	ticker := time.NewTicker(actionableStatePollInterval)
	defer ticker.Stop()

	for {
		state, err := c.GetState(ctx)
		if err != nil {
			return nil, err
		}
		if IsActionableState(state) {
			return state, nil
		}

		select {
		case <-ctx.Done():
			if finalState, finalErr := c.getFinalActionableState(); finalErr == nil && IsActionableState(finalState) {
				return finalState, nil
			}
			return state, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (c *Client) getFinalActionableState() (*StateSnapshot, error) {
	finalCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	return c.GetState(finalCtx)
}

func (c *Client) stabilizeActionableState(ctx context.Context, initial *StateSnapshot) (*StateSnapshot, error) {
	if !IsActionableState(initial) {
		return initial, nil
	}

	current := initial
	currentFingerprint := stateFingerprint(initial)
	stableSince := time.Now()
	settleWindow, settleMax := settleDurations(initial)
	deadline := time.Now().Add(settleMax)
	ticker := time.NewTicker(actionableStateSettlePoll)
	defer ticker.Stop()

	for {
		if time.Since(stableSince) >= settleWindow {
			return current, nil
		}

		if time.Now().After(deadline) {
			return current, nil
		}

		select {
		case <-ctx.Done():
			return current, ctx.Err()
		case <-ticker.C:
			next, err := c.GetState(ctx)
			if err != nil {
				return current, err
			}
			if !IsActionableState(next) {
				current = next
				currentFingerprint = stateFingerprint(next)
				stableSince = time.Now()
				continue
			}

			nextFingerprint := stateFingerprint(next)
			if nextFingerprint != currentFingerprint {
				current = next
				currentFingerprint = nextFingerprint
				stableSince = time.Now()
				continue
			}

			current = next
		}
	}
}

func settleDurations(state *StateSnapshot) (window time.Duration, max time.Duration) {
	if state == nil {
		return actionableStateSettleWindow, actionableStateSettleMax
	}

	switch strings.ToUpper(strings.TrimSpace(state.Screen)) {
	case "COMBAT":
		if shouldUseCombatEdgeSettle(state) {
			return combatEdgeStateSettleWindow, combatEdgeStateSettleMax
		}
		return combatStateSettleWindow, combatStateSettleMax
	case "REWARD":
		return rewardStateSettleWindow, rewardStateSettleMax
	case "CARD_SELECTION":
		return selectionStateSettleWindow, selectionStateSettleMax
	case "GAME_OVER":
		return gameOverStateSettleWindow, gameOverStateSettleMax
	default:
		return actionableStateSettleWindow, actionableStateSettleMax
	}
}

func shouldUseCombatEdgeSettle(state *StateSnapshot) bool {
	if state == nil || !strings.EqualFold(strings.TrimSpace(state.Screen), "COMBAT") {
		return false
	}
	if state.Combat == nil {
		return false
	}
	energy := state.Combat.Player.Energy
	if energy > 1 {
		return false
	}
	hand := state.Combat.Hand
	if len(state.AvailableActions) == 1 && strings.EqualFold(strings.TrimSpace(state.AvailableActions[0]), "end_turn") {
		if energy > 0 {
			return false
		}
		if len(hand) == 0 {
			return true
		}
		for _, card := range hand {
			if card.Playable {
				return false
			}
		}
		return true
	}

	if !hasAction(state.AvailableActions, "play_card") {
		return false
	}

	if energy != 1 {
		return false
	}

	playableCards := 0
	for _, card := range hand {
		if card.Playable {
			playableCards++
		}
	}
	if playableCards == 0 {
		return true
	}

	return playableCards <= 3
}

func hasAction(actions []string, target string) bool {
	for _, action := range actions {
		if strings.EqualFold(strings.TrimSpace(action), target) {
			return true
		}
	}
	return false
}

func stateFingerprint(state *StateSnapshot) string {
	if state == nil {
		return "nil"
	}

	payload := map[string]any{
		"runId":            state.RunID,
		"screen":           state.Screen,
		"turn":             state.Turn,
		"availableActions": state.AvailableActions,
	}

	switch strings.TrimSpace(state.Screen) {
	case "COMBAT":
		if state.Combat != nil {
			payload["actionWindowOpen"] = state.Combat.ActionWindowOpen
			payload["player"] = state.Combat.Player
			payload["hand"] = state.Combat.Hand
			payload["enemies"] = state.Combat.Enemies
		}
	case "REWARD":
		if state.Reward != nil {
			payload["phase"] = state.Reward.Phase
			payload["sourceScreen"] = state.Reward.SourceScreen
			payload["sourceHint"] = state.Reward.SourceHint
			payload["pendingCardChoice"] = state.Reward.PendingCardChoice
			payload["rewards"] = state.Reward.Rewards
			payload["cardOptions"] = state.Reward.CardOptions
		}
	case "CARD_SELECTION":
		if state.Selection != nil {
			payload["selectionKind"] = state.Selection.Kind
			payload["selectionSourceScreen"] = state.Selection.SourceScreen
			payload["selectionSourceHint"] = state.Selection.SourceHint
			payload["selectionMode"] = state.Selection.Mode
			payload["cards"] = state.Selection.Cards
		}
	case "GAME_OVER":
		if state.GameOver != nil {
			payload["stage"] = state.GameOver.Stage
		}
	}

	bytes, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(bytes)
}

// IsActionableState reports whether a state is safe for the runtime to treat
// as actionable, excluding transient combat, reward, and game-over frames.
func IsActionableState(state *StateSnapshot) bool {
	if state == nil {
		return false
	}

	if len(state.AvailableActions) == 0 {
		return false
	}

	switch strings.TrimSpace(state.Screen) {
	case "COMBAT":
		if state.Combat == nil || !state.Combat.ActionWindowOpen {
			return false
		}
		if state.Combat.IsOverOrEnding ||
			state.Combat.PlayerActionsDisabled ||
			state.Combat.IsInCardPlay ||
			state.Combat.IsInCardSelection {
			return false
		}
		if !combatHasActionableEnemies(state) {
			return false
		}
		return true
	case "REWARD":
		return rewardStateIsActionable(state)
	case "CARD_SELECTION":
		return selectionStateIsActionable(state)
	case "GAME_OVER":
		if state.GameOver == nil {
			return true
		}
		return state.GameOver.Stage == "" || !strings.EqualFold(state.GameOver.Stage, "transition")
	default:
		return true
	}
}

func rewardStateIsActionable(state *StateSnapshot) bool {
	if state == nil || state.Reward == nil {
		return false
	}

	if strings.EqualFold(state.Reward.Phase, "settling") {
		return false
	}

	hasClaimReward := hasAvailableAction(state, "claim_reward")
	hasProceed := hasAvailableAction(state, "proceed")
	hasChooseRewardCard := hasAvailableAction(state, "choose_reward_card")
	hasSkipRewardCards := hasAvailableAction(state, "skip_reward_cards")
	hasDeckSelection := hasAvailableAction(state, "select_deck_card")

	pendingCardChoice := state.Reward.PendingCardChoice || len(state.Reward.CardOptions) > 0
	hasClaimableRewards := false
	for _, reward := range state.Reward.Rewards {
		if reward.Claimable {
			hasClaimableRewards = true
			break
		}
	}

	if pendingCardChoice {
		if hasDeckSelection || hasClaimReward || hasProceed {
			return false
		}
		return hasChooseRewardCard || hasSkipRewardCards
	}

	if hasClaimableRewards {
		if hasProceed || hasChooseRewardCard || hasSkipRewardCards || hasDeckSelection {
			return false
		}
		return hasClaimReward
	}

	if state.Reward.CanProceed {
		if hasClaimReward || hasChooseRewardCard || hasSkipRewardCards || hasDeckSelection {
			return false
		}
		return hasProceed
	}

	return hasClaimReward || hasProceed
}

func selectionStateIsActionable(state *StateSnapshot) bool {
	if state == nil || state.Selection == nil {
		return false
	}

	hasChooseRewardCard := hasAvailableAction(state, "choose_reward_card")
	hasSkipRewardCards := hasAvailableAction(state, "skip_reward_cards")
	hasDeckSelection := hasAvailableAction(state, "select_deck_card")
	hasConfirmSelection := hasAvailableAction(state, "confirm_selection")

	selectionKind := state.Selection.Kind
	requiresConfirmation := state.Selection.RequiresConfirmation
	canConfirm := state.Selection.CanConfirm
	hasSelectionCards := len(state.Selection.Cards) > 0
	pendingRewardChoice := state.Reward != nil && (state.Reward.PendingCardChoice || len(state.Reward.CardOptions) > 0)

	if strings.EqualFold(selectionKind, "NSimpleCardSelectScreen") && (requiresConfirmation || canConfirm || hasConfirmSelection) {
		return false
	}

	if hasChooseRewardCard || hasSkipRewardCards {
		if !pendingRewardChoice || hasDeckSelection || hasConfirmSelection {
			return false
		}
		return true
	}

	if hasConfirmSelection {
		if !requiresConfirmation || !canConfirm {
			return false
		}
		return true
	}

	if hasDeckSelection {
		if !hasSelectionCards {
			return false
		}
		if pendingRewardChoice {
			return false
		}
		return true
	}

	return false
}

func combatHasActionableEnemies(state *StateSnapshot) bool {
	if state == nil || state.Combat == nil {
		return false
	}

	for _, enemy := range state.Combat.Enemies {
		if enemy.IsHittable {
			return true
		}
	}

	return false
}

func isUsefulActionableEvent(eventType string) bool {
	switch eventType {
	case "stream_ready",
		"state_changed",
		"screen_changed",
		"available_actions_changed",
		"player_action_window_opened",
		"combat_turn_changed",
		"combat_started",
		"combat_ended":
		return true
	default:
		return false
	}
}

func nestedBool(root map[string]any, key string) bool {
	if len(root) == 0 {
		return false
	}

	switch value := root[key].(type) {
	case bool:
		return value
	case string:
		return strings.EqualFold(strings.TrimSpace(value), "true")
	default:
		return false
	}
}

func nestedString(root map[string]any, key string) string {
	if len(root) == 0 {
		return ""
	}

	value, _ := root[key].(string)
	return strings.TrimSpace(value)
}

func nestedInt(root map[string]any, key string) (int, bool) {
	if len(root) == 0 {
		return 0, false
	}

	switch value := root[key].(type) {
	case int:
		return value, true
	case int32:
		return int(value), true
	case int64:
		return int(value), true
	case float64:
		return int(value), true
	default:
		return 0, false
	}
}

func nestedMapList(root map[string]any, key string) []map[string]any {
	if len(root) == 0 {
		return nil
	}

	rawItems, ok := root[key].([]interface{})
	if !ok {
		return nil
	}

	items := make([]map[string]any, 0, len(rawItems))
	for _, raw := range rawItems {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		items = append(items, item)
	}

	return items
}

func hasAvailableAction(state *StateSnapshot, action string) bool {
	if state == nil {
		return false
	}

	for _, available := range state.AvailableActions {
		if strings.EqualFold(strings.TrimSpace(available), action) {
			return true
		}
	}

	return false
}
