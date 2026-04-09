package agentruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	openagent "github.com/hunknownz/open-agent-sdk-go/agent"
	openagenttypes "github.com/hunknownz/open-agent-sdk-go/types"

	"spire2mind/internal/config"
	"spire2mind/internal/game"
)

const expectedGameVersion = "v0.99.1"

type claudeAuthStatus struct {
	LoggedIn         bool   `json:"loggedIn"`
	AuthMethod       string `json:"authMethod"`
	APIProvider      string `json:"apiProvider"`
	SubscriptionType string `json:"subscriptionType"`
}

func RunDoctor(ctx context.Context, cfg config.Config) error {
	client := game.NewClient(cfg.BridgeURL)
	var issues []string

	fmt.Printf("Repo root: %s\n", cfg.RepoRoot)
	fmt.Printf("Bridge URL: %s\n", cfg.BridgeURL)
	fmt.Printf("Data dir: %s\n", cfg.DataDir)
	fmt.Printf("Artifacts dir: %s\n", filepath.Clean(cfg.ArtifactsDir))
	fmt.Printf("Game executable: %s\n", cfg.GameExePath)
	fmt.Printf("Game prefs: %s\n", emptyLabel(cfg.GamePrefsPath))
	fmt.Printf("Requested fast mode: %s\n", emptyLabel(cfg.GameFastMode))
	fmt.Printf("Runtime mode: %s\n", cfg.ModeLabel())
	fmt.Printf("Provider: %s\n", cfg.ProviderLabel())
	fmt.Printf("Max attempts: %d\n", cfg.MaxAttempts)

	health, err := client.GetHealth(ctx)
	if err != nil {
		issues = append(issues, fmt.Sprintf("bridge check failed: %v", err))
		return joinDoctorIssues(issues)
	}

	fmt.Printf("Bridge: ok (%s, game %s, ready=%t)\n", health.BridgeVersion, health.GameVersion, health.Ready)
	if !strings.EqualFold(health.GameVersion, expectedGameVersion) {
		issues = append(issues, fmt.Sprintf("unexpected game version: got %s, expected %s", health.GameVersion, expectedGameVersion))
	}

	if state, err := client.GetState(ctx); err == nil {
		fmt.Printf("State: ok (screen=%s actions=%d run=%s)\n", state.Screen, len(state.AvailableActions), state.RunID)
	} else {
		issues = append(issues, fmt.Sprintf("state probe failed: %v", err))
	}

	if markdown, err := client.GetMarkdownState(ctx); err == nil {
		fmt.Printf("Markdown state: ok (format=%s chars=%d)\n", markdown.Format, len(markdown.Markdown))
	} else {
		issues = append(issues, fmt.Sprintf("markdown state probe failed: %v", err))
	}

	if actions, err := client.GetAvailableActions(ctx); err == nil {
		fmt.Printf("Actions: ok (screen=%s descriptors=%d)\n", actions.Screen, len(actions.Descriptors))
	} else {
		issues = append(issues, fmt.Sprintf("actions probe failed: %v", err))
	}

	if err := probeEventStream(ctx, cfg.BridgeURL); err == nil {
		fmt.Println("Event stream: ok")
	} else {
		issues = append(issues, fmt.Sprintf("event stream probe failed: %v", err))
	}

	if entries, err := filepath.Glob(filepath.Join(cfg.DataDir, "*.json")); err == nil && len(entries) > 0 {
		fmt.Printf("Game data: ok (%d collections)\n", len(entries))
	} else if err != nil {
		issues = append(issues, fmt.Sprintf("game data lookup failed: %v", err))
	} else {
		issues = append(issues, "game data directory is empty")
	}

	if stat, err := os.Stat(cfg.GameExePath); err == nil && !stat.IsDir() {
		fmt.Printf("Game executable: ok (%s)\n", cfg.GameExePath)
	} else if cfg.GameDir != "" {
		issues = append(issues, fmt.Sprintf("game executable missing: %s", cfg.GameExePath))
	}

	if strings.TrimSpace(cfg.GamePrefsPath) != "" {
		if fastMode, err := game.ReadFastMode(cfg.GamePrefsPath); err == nil {
			fmt.Printf("Game fast mode: current=%s prefs=%s\n", emptyLabel(fastMode.Current), fastMode.Path)
		} else {
			issues = append(issues, fmt.Sprintf("game prefs probe failed: %v", err))
		}
	} else {
		issues = append(issues, "game prefs path unavailable")
	}

	if err := os.MkdirAll(cfg.ArtifactsDir, 0o755); err != nil {
		issues = append(issues, fmt.Sprintf("artifacts dir unavailable: %v", err))
	} else {
		fmt.Printf("Artifacts dir: ok (%s)\n", filepath.Clean(cfg.ArtifactsDir))
	}

	if !cfg.HasModelConfig() {
		issues = append(issues, "model config incomplete: configure SPIRE2MIND_MODEL_PROVIDER=api with SPIRE2MIND_API_BASE_URL/SPIRE2MIND_MODEL and optionally SPIRE2MIND_API_KEY (not required for trusted local or LAN API), or use a logged-in Claude Code CLI")
		return joinDoctorIssues(issues)
	}

	fmt.Printf("Model: %s\n", cfg.Model)
	switch cfg.ModelProvider {
	case config.ModelProviderAPI:
		fmt.Printf("API Base URL: %s\n", cfg.APIBaseURL)
		fmt.Printf("API Provider: %s\n", emptyLabel(cfg.APIProvider))
		if cfg.APIKey == "" && cfg.AllowsAnonymousAPI() {
			fmt.Println("API Auth: anonymous trusted local/LAN")
		}
	case config.ModelProviderClaudeCLI:
		fmt.Printf("Claude CLI: %s\n", cfg.ClaudeCLIPath)
		status, err := probeClaudeAuthStatus(ctx, cfg.ClaudeCLIPath)
		if err != nil {
			issues = append(issues, fmt.Sprintf("claude auth probe failed: %v", err))
		} else {
			fmt.Printf("Claude auth: logged_in=%t method=%s api_provider=%s subscription=%s\n", status.LoggedIn, emptyLabel(status.AuthMethod), emptyLabel(status.APIProvider), emptyLabel(status.SubscriptionType))
			if !status.LoggedIn {
				issues = append(issues, "claude cli is not logged in")
			}
		}
	}

	probeOptions := openagent.Options{
		Model:          cfg.Model,
		CWD:            cfg.RepoRoot,
		MaxTurns:       1,
		PermissionMode: openagenttypes.PermissionModeBypassPermissions,
	}
	switch cfg.ModelProvider {
	case config.ModelProviderAPI:
		probeOptions.Provider = openagenttypes.ProviderAPI
		probeOptions.APIKey = cfg.APIKey
		probeOptions.BaseURL = cfg.APIBaseURL
		probeOptions.APIProvider = cfg.APIProvider
		if cfg.UsesStructuredAPIDecisions() {
			probeOptions.JSONSchema = ActionDecisionJSONSchema()
			probeOptions.AllowedTools = []string{openagent.StructuredOutputToolName}
			probeOptions.CanUseTool = func(tool openagenttypes.Tool, input map[string]interface{}) (*openagenttypes.PermissionDecision, error) {
				if tool.Name() != openagent.StructuredOutputToolName {
					return &openagenttypes.PermissionDecision{
						Behavior: openagenttypes.PermissionDeny,
						Reason:   fmt.Sprintf("tool %s is not enabled for structured probe", tool.Name()),
					}, nil
				}
				return &openagenttypes.PermissionDecision{Behavior: openagenttypes.PermissionAllow}, nil
			}
		}
	case config.ModelProviderClaudeCLI:
		probeOptions.Provider = openagenttypes.ProviderClaudeCLI
		probeOptions.CLICommand = cfg.ClaudeCLIPath
	}

	probe := openagent.New(probeOptions)
	defer probe.Close()

	if err := probe.Init(ctx); err != nil {
		issues = append(issues, fmt.Sprintf("agent init failed: %v", err))
		return joinDoctorIssues(issues)
	}

	result, err := probe.Prompt(ctx, "Reply with OK only.")
	if err != nil {
		issues = append(issues, fmt.Sprintf("model probe failed: %v", err))
		return joinDoctorIssues(issues)
	}

	fmt.Printf("Model probe: ok (%s)\n", strings.TrimSpace(result.Text))
	return joinDoctorIssues(issues)
}

func probeEventStream(ctx context.Context, bridgeURL string) error {
	probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, strings.TrimRight(bridgeURL, "/")+"/events/stream", nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	if !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		return fmt.Errorf("unexpected content type %q", resp.Header.Get("Content-Type"))
	}

	return nil
}

func probeClaudeAuthStatus(ctx context.Context, cliPath string) (*claudeAuthStatus, error) {
	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(probeCtx, cliPath, "auth", "status")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var status claudeAuthStatus
	if err := json.Unmarshal(output, &status); err != nil {
		return nil, fmt.Errorf("decode auth status: %w", err)
	}

	return &status, nil
}

func joinDoctorIssues(issues []string) error {
	if len(issues) == 0 {
		return nil
	}

	return fmt.Errorf("%s", strings.Join(issues, "\n- "))
}

func emptyLabel(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}
