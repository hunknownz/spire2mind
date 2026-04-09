package agentruntime

import (
	"context"
	"fmt"
	"time"

	openagent "github.com/hunknownz/open-agent-sdk-go/agent"
	openagenttypes "github.com/hunknownz/open-agent-sdk-go/types"

	"spire2mind/internal/config"
	"spire2mind/internal/game"
)

type Runtime struct {
	Agent     *openagent.Agent
	SessionID string
	Client    *game.Client
	Data      *game.DataStore
}

func New(ctx context.Context, cfg config.Config) (*Runtime, error) {
	client := game.NewClient(cfg.BridgeURL)
	data := game.NewDataStore(cfg.DataDir)
	tools := game.NewTools(client, data)
	runtime := &Runtime{
		SessionID: fmt.Sprintf("det-%d", time.Now().UnixNano()),
		Client:    client,
		Data:      data,
	}

	if !cfg.HasModelConfig() {
		return runtime, nil
	}

	agentOptions := openagent.Options{
		Model:          cfg.Model,
		ContextWindow:  cfg.ModelContext,
		CWD:            cfg.RepoRoot,
		MaxTurns:       24,
		PermissionMode: openagenttypes.PermissionModeBypassPermissions,
	}

	switch cfg.ModelProvider {
	case config.ModelProviderAPI:
		agentOptions.Provider = openagenttypes.ProviderAPI
		agentOptions.APIKey = cfg.APIKey
		agentOptions.BaseURL = cfg.APIBaseURL
		agentOptions.APIProvider = cfg.APIProvider
		if cfg.UsesStructuredAPIDecisions() {
			agentOptions.SystemPrompt = game.ClaudeCLISystemPrompt
			agentOptions.MaxTurns = 1
			agentOptions.JSONSchema = ActionDecisionJSONSchema()
			agentOptions.AllowedTools = []string{openagent.StructuredOutputToolName}
			agentOptions.CanUseTool = func(tool openagenttypes.Tool, input map[string]interface{}) (*openagenttypes.PermissionDecision, error) {
				if tool.Name() != openagent.StructuredOutputToolName {
					return &openagenttypes.PermissionDecision{
						Behavior: openagenttypes.PermissionDeny,
						Reason:   fmt.Sprintf("tool %s is not enabled for structured spire2mind decisions", tool.Name()),
					}, nil
				}
				return &openagenttypes.PermissionDecision{Behavior: openagenttypes.PermissionAllow}, nil
			}
		} else {
			allowed := game.AllowedToolNames()
			agentOptions.SystemPrompt = game.SystemPrompt
			agentOptions.CustomTools = tools
			agentOptions.CanUseTool = func(tool openagenttypes.Tool, input map[string]interface{}) (*openagenttypes.PermissionDecision, error) {
				if _, ok := allowed[tool.Name()]; !ok {
					return &openagenttypes.PermissionDecision{
						Behavior: openagenttypes.PermissionDeny,
						Reason:   fmt.Sprintf("tool %s is not enabled for spire2mind", tool.Name()),
					}, nil
				}
				return &openagenttypes.PermissionDecision{Behavior: openagenttypes.PermissionAllow}, nil
			}
		}
	case config.ModelProviderClaudeCLI:
		agentOptions.Provider = openagenttypes.ProviderClaudeCLI
		agentOptions.CLICommand = cfg.ClaudeCLIPath
		agentOptions.SystemPrompt = game.ClaudeCLISystemPrompt
		agentOptions.MaxTurns = 1
		agentOptions.Env = cfg.ClaudeCLIEnv()
	default:
		return runtime, nil
	}

	agent := openagent.New(agentOptions)

	if err := agent.Init(ctx); err != nil {
		agent.Close()
		return nil, err
	}

	runtime.Agent = agent
	runtime.SessionID = agent.SessionID()
	return runtime, nil
}

func (r *Runtime) Close() {
	if r.Agent != nil {
		r.Agent.Close()
	}
}
