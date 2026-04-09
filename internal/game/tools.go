package game

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/hunknownz/open-agent-sdk-go/types"
)

func NewTools(client *Client, data *DataStore) []types.Tool {
	return []types.Tool{
		&getGameStateTool{client: client},
		&getAvailableActionsTool{client: client},
		&actTool{client: client},
		&waitUntilActionableTool{client: client},
		&getGameDataTool{data: data},
	}
}

func AllowedToolNames() map[string]struct{} {
	return map[string]struct{}{
		"get_game_state":        {},
		"get_available_actions": {},
		"act":                   {},
		"wait_until_actionable": {},
		"get_game_data":         {},
	}
}

type getGameStateTool struct {
	client *Client
}

func (t *getGameStateTool) Name() string { return "get_game_state" }
func (t *getGameStateTool) Description() string {
	return "Read the latest Slay the Spire 2 game state snapshot from the local Bridge."
}
func (t *getGameStateTool) InputSchema() types.ToolInputSchema {
	return types.ToolInputSchema{Type: "object"}
}
func (t *getGameStateTool) IsConcurrencySafe(map[string]interface{}) bool { return true }
func (t *getGameStateTool) IsReadOnly(map[string]interface{}) bool        { return true }
func (t *getGameStateTool) Call(ctx context.Context, input map[string]interface{}, tCtx *types.ToolUseContext) (*types.ToolResult, error) {
	state, err := t.client.GetState(ctx)
	if err != nil {
		return toolError(err), nil
	}
	return toolJSON(state)
}

type getAvailableActionsTool struct {
	client *Client
}

func (t *getAvailableActionsTool) Name() string { return "get_available_actions" }
func (t *getAvailableActionsTool) Description() string {
	return "List legal actions that can be executed right now, plus parameter requirements."
}
func (t *getAvailableActionsTool) InputSchema() types.ToolInputSchema {
	return types.ToolInputSchema{Type: "object"}
}
func (t *getAvailableActionsTool) IsConcurrencySafe(map[string]interface{}) bool { return true }
func (t *getAvailableActionsTool) IsReadOnly(map[string]interface{}) bool        { return true }
func (t *getAvailableActionsTool) Call(ctx context.Context, input map[string]interface{}, tCtx *types.ToolUseContext) (*types.ToolResult, error) {
	actions, err := t.client.GetAvailableActions(ctx)
	if err != nil {
		return toolError(err), nil
	}
	return toolJSON(actions)
}

type actTool struct {
	client *Client
}

func (t *actTool) Name() string { return "act" }
func (t *actTool) Description() string {
	return "Execute exactly one Bridge action. The action must be present in available_actions."
}
func (t *actTool) InputSchema() types.ToolInputSchema {
	return types.ToolInputSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"action": map[string]interface{}{
				"type":        "string",
				"description": "Action name from available_actions.",
			},
			"card_index": map[string]interface{}{
				"type":        "integer",
				"description": "Card index for play_card.",
			},
			"target_index": map[string]interface{}{
				"type":        "integer",
				"description": "Target index for targeted combat actions.",
			},
			"option_index": map[string]interface{}{
				"type":        "integer",
				"description": "Option index for choice-based actions.",
			},
		},
		Required: []string{"action"},
	}
}
func (t *actTool) IsConcurrencySafe(map[string]interface{}) bool { return false }
func (t *actTool) IsReadOnly(map[string]interface{}) bool        { return false }
func (t *actTool) Call(ctx context.Context, input map[string]interface{}, tCtx *types.ToolUseContext) (*types.ToolResult, error) {
	action, _ := input["action"].(string)
	request := ActionRequest{
		Action:      action,
		CardIndex:   optionalInt(input, "card_index"),
		TargetIndex: optionalInt(input, "target_index"),
		OptionIndex: optionalInt(input, "option_index"),
	}

	result, err := t.client.Act(ctx, request)
	if err != nil {
		return toolError(err), nil
	}
	return toolJSON(result)
}

type waitUntilActionableTool struct {
	client *Client
}

func (t *waitUntilActionableTool) Name() string { return "wait_until_actionable" }
func (t *waitUntilActionableTool) Description() string {
	return "Poll the Bridge until the game becomes actionable or timeout is reached."
}
func (t *waitUntilActionableTool) InputSchema() types.ToolInputSchema {
	return types.ToolInputSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"timeout_seconds": map[string]interface{}{
				"type":        "number",
				"description": "Optional timeout in seconds. Default is 20.",
			},
		},
	}
}
func (t *waitUntilActionableTool) IsConcurrencySafe(map[string]interface{}) bool { return false }
func (t *waitUntilActionableTool) IsReadOnly(map[string]interface{}) bool        { return true }
func (t *waitUntilActionableTool) Call(ctx context.Context, input map[string]interface{}, tCtx *types.ToolUseContext) (*types.ToolResult, error) {
	timeout := 20 * time.Second
	if raw, ok := input["timeout_seconds"]; ok {
		if seconds, err := numberToDurationSeconds(raw); err == nil {
			timeout = seconds
		}
	}

	state, err := t.client.WaitUntilActionable(ctx, timeout)
	if err != nil {
		return toolError(err), nil
	}
	return toolJSON(state)
}

type getGameDataTool struct {
	data *DataStore
}

func (t *getGameDataTool) Name() string { return "get_game_data" }
func (t *getGameDataTool) Description() string {
	return "Query local static game data such as cards, relics, potions, events, monsters, and characters."
}
func (t *getGameDataTool) InputSchema() types.ToolInputSchema {
	return types.ToolInputSchema{
		Type: "object",
		Properties: map[string]interface{}{
			"collection": map[string]interface{}{
				"type":        "string",
				"description": "Collection name such as cards, relics, potions, events, monsters, or characters.",
			},
			"item_id": map[string]interface{}{
				"type":        "string",
				"description": "Optional item id inside the collection.",
			},
		},
		Required: []string{"collection"},
	}
}
func (t *getGameDataTool) IsConcurrencySafe(map[string]interface{}) bool { return true }
func (t *getGameDataTool) IsReadOnly(map[string]interface{}) bool        { return true }
func (t *getGameDataTool) Call(ctx context.Context, input map[string]interface{}, tCtx *types.ToolUseContext) (*types.ToolResult, error) {
	collection, _ := input["collection"].(string)
	itemID, _ := input["item_id"].(string)

	result, err := t.data.Query(collection, itemID)
	if err != nil {
		return toolError(err), nil
	}
	return toolJSON(result)
}

func optionalInt(input map[string]interface{}, key string) *int {
	value, ok := input[key]
	if !ok {
		return nil
	}

	switch typed := value.(type) {
	case int:
		return &typed
	case int32:
		v := int(typed)
		return &v
	case int64:
		v := int(typed)
		return &v
	case float64:
		v := int(typed)
		return &v
	case json.Number:
		if parsed, err := typed.Int64(); err == nil {
			v := int(parsed)
			return &v
		}
	case string:
		if parsed, err := strconv.Atoi(typed); err == nil {
			return &parsed
		}
	}

	return nil
}

func numberToDurationSeconds(value interface{}) (time.Duration, error) {
	switch typed := value.(type) {
	case float64:
		return time.Duration(typed * float64(time.Second)), nil
	case int:
		return time.Duration(typed) * time.Second, nil
	case string:
		parsed, err := strconv.ParseFloat(typed, 64)
		if err != nil {
			return 0, err
		}
		return time.Duration(parsed * float64(time.Second)), nil
	default:
		return 0, fmt.Errorf("unsupported timeout value %T", value)
	}
}

func toolJSON(value interface{}) (*types.ToolResult, error) {
	bytes, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}

	return &types.ToolResult{
		Data: value,
		Content: []types.ContentBlock{
			{Type: types.ContentBlockText, Text: string(bytes)},
		},
	}, nil
}

func toolError(err error) *types.ToolResult {
	return &types.ToolResult{
		IsError: true,
		Error:   err.Error(),
		Content: []types.ContentBlock{
			{Type: types.ContentBlockText, Text: err.Error()},
		},
	}
}
