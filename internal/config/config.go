package config

import (
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"spire2mind/internal/i18n"
)

type ModelProvider string
type APIDecisionMode string

const (
	ModelProviderAPI       ModelProvider = "api"
	ModelProviderClaudeCLI ModelProvider = "claude-cli"

	APIDecisionModeTools      APIDecisionMode = "tools"
	APIDecisionModeStructured APIDecisionMode = "structured"
	APIDecisionModeAuto       APIDecisionMode = "auto"
)

type Config struct {
	RepoRoot      string
	BridgeURL     string
	Model         string
	ModelContext  int
	ModelProvider ModelProvider
	APIDecisionMode APIDecisionMode
	CombatPlanner   string
	ForceModelEval  bool
	StreamerEnabled bool
	StreamerMaxHistory int
	StreamerStyle string
	TTSQueueDir string
	TTSAutoSpeak bool
	APIBaseURL      string
	APIKey          string
	APIProvider     string
	APITimeoutMs    int
	APIMaxRetries   int
	ClaudeCodeDisableNonessentialTraffic string
	ClaudeCodeAttributionHeader          string
	ClaudeCLIPath                       string
	DataDir                             string
	GameDir                             string
	GameExePath                         string
	GamePrefsPath                       string
	GameFastMode                        string
	ArtifactsDir                        string
	MaxAttempts                         int
	MaxCycles                           int
	Language                            i18n.Language
}

func Load(repoRoot string) Config {
	gameDir := resolveGameDir()
	model := firstNonEmpty(os.Getenv("SPIRE2MIND_MODEL"), os.Getenv("ANTHROPIC_MODEL"), "claude-sonnet-4-6")
	apiBaseURL := firstNonEmpty(os.Getenv("SPIRE2MIND_API_BASE_URL"), os.Getenv("ANTHROPIC_BASE_URL"))
	apiKey := firstNonEmpty(os.Getenv("SPIRE2MIND_API_KEY"), os.Getenv("ANTHROPIC_AUTH_TOKEN"), os.Getenv("ANTHROPIC_API_KEY"))
	apiProvider := normalizeAPIProvider(firstNonEmpty(os.Getenv("SPIRE2MIND_API_PROVIDER"), os.Getenv("ANTHROPIC_PROVIDER")))
	apiDecisionMode := normalizeAPIDecisionMode(firstNonEmpty(os.Getenv("SPIRE2MIND_API_DECISION_MODE"), os.Getenv("SPIRE2MIND_DECISION_MODE"), "tools"))
	claudeCLIPath := resolveClaudeCLIPath()
	provider := resolveModelProvider(apiBaseURL, apiKey, claudeCLIPath)
	language := i18n.ParseLanguage(firstNonEmpty(os.Getenv("SPIRE2MIND_LANGUAGE"), os.Getenv("SPIRE2MIND_LANG"), "zh"))

	return Config{
		RepoRoot:      repoRoot,
		BridgeURL:     firstNonEmpty(os.Getenv("SPIRE2MIND_BRIDGE_URL"), "http://127.0.0.1:8080"),
		Model:         model,
		ModelContext:  normalizeModelContext(parseIntWithDefault(firstNonEmpty(os.Getenv("SPIRE2MIND_MODEL_CONTEXT"), os.Getenv("SPIRE2MIND_CONTEXT_WINDOW")), 8192)),
		ModelProvider: provider,
		APIDecisionMode: apiDecisionMode,
		CombatPlanner: NormalizeCombatPlanner(firstNonEmpty(os.Getenv("SPIRE2MIND_COMBAT_PLANNER"), "heuristic")),
		ForceModelEval: parseBoolEnv(
			firstNonEmpty(
				os.Getenv("SPIRE2MIND_FORCE_MODEL_EVAL"),
				os.Getenv("SPIRE2MIND_LOCAL_LLM_EVAL"),
			),
		),
		StreamerEnabled: parseBoolWithDefault(firstNonEmpty(os.Getenv("SPIRE2MIND_STREAMER_ENABLED")), true),
		StreamerMaxHistory: normalizeStreamerMaxHistory(parseIntWithDefault(firstNonEmpty(os.Getenv("SPIRE2MIND_STREAMER_MAX_HISTORY")), 8)),
		StreamerStyle: normalizeStreamerStyle(firstNonEmpty(os.Getenv("SPIRE2MIND_STREAMER_STYLE"), "bright-cute")),
		TTSQueueDir:    filepath.Join(repoRoot, "scratch", "tts"),
		TTSAutoSpeak:   parseBoolEnv(firstNonEmpty(os.Getenv("SPIRE2MIND_TTS_AUTO_SPEAK"), os.Getenv("SPIRE2MIND_TTS_SPEAK"))),
		APIBaseURL:    apiBaseURL,
		APIKey:        apiKey,
		APIProvider:   apiProvider,
		APITimeoutMs:  resolveAPITimeoutMs(apiBaseURL),
		APIMaxRetries: resolveAPIMaxRetries(apiBaseURL),
		ClaudeCodeDisableNonessentialTraffic: firstNonEmpty(os.Getenv("CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC")),
		ClaudeCodeAttributionHeader:          firstNonEmpty(os.Getenv("CLAUDE_CODE_ATTRIBUTION_HEADER")),
		ClaudeCLIPath: claudeCLIPath,
		DataDir:       filepath.Join(repoRoot, "data", "eng"),
		GameDir:       gameDir,
		GameExePath:   filepath.Join(gameDir, "SlayTheSpire2.exe"),
		GamePrefsPath: resolveGamePrefsPath(),
		GameFastMode:  NormalizeGameFastMode(firstNonEmpty(os.Getenv("SPIRE2MIND_GAME_FAST_MODE"), os.Getenv("SPIRE2MIND_FAST_MODE"))),
		ArtifactsDir:  filepath.Join(repoRoot, "scratch", "agent-runs"),
		MaxAttempts:   normalizeMaxAttempts(parseIntWithDefault(firstNonEmpty(os.Getenv("SPIRE2MIND_MAX_ATTEMPTS")), 1)),
		MaxCycles:     normalizeMaxCycles(parseIntWithDefault(firstNonEmpty(os.Getenv("SPIRE2MIND_MAX_CYCLES")), -1)),
		Language:      language,
	}
}

func normalizeStreamerMaxHistory(value int) int {
	switch {
	case value <= 0:
		return 8
	case value > 24:
		return 24
	default:
		return value
	}
}

func normalizeStreamerStyle(value string) string {
	switch strings.TrimSpace(strings.ToLower(value)) {
	case "", "bright-cute", "cute", "energetic", "warm", "calm":
		if strings.TrimSpace(value) == "" {
			return "bright-cute"
		}
		return strings.TrimSpace(strings.ToLower(value))
	default:
		return "bright-cute"
	}
}

func parseBoolWithDefault(value string, fallback bool) bool {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	if trimmed == "" {
		return fallback
	}
	return parseBoolEnv(trimmed)
}

func NormalizeCombatPlanner(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "", "heuristic", "none", "off", "disabled", "mcts":
		return value
	default:
		return value
	}
}

func (c Config) HasModelConfig() bool {
	switch c.ModelProvider {
	case ModelProviderAPI:
		if c.APIBaseURL == "" || c.Model == "" {
			return false
		}
		return c.APIKey != "" || c.AllowsAnonymousAPI()
	case ModelProviderClaudeCLI:
		return c.ClaudeCLIPath != "" && c.Model != ""
	default:
		return false
	}
}

func (c Config) ModeLabel() string {
	switch c.ModelProvider {
	case ModelProviderAPI:
		if c.HasModelConfig() {
			return "model-api"
		}
	case ModelProviderClaudeCLI:
		if c.HasModelConfig() {
			return "model-claude-cli"
		}
	}

	return "deterministic"
}

func (c Config) ProviderLabel() string {
	switch c.ModelProvider {
	case ModelProviderAPI:
		return string(ModelProviderAPI)
	case ModelProviderClaudeCLI:
		return string(ModelProviderClaudeCLI)
	default:
		return "deterministic"
	}
}

func (c Config) UsesClaudeCLI() bool {
	return c.ModelProvider == ModelProviderClaudeCLI
}

func (c Config) UsesStructuredAPIDecisions() bool {
	if c.ModelProvider != ModelProviderAPI || !c.HasModelConfig() {
		return false
	}

	switch c.APIDecisionMode {
	case APIDecisionModeStructured:
		return true
	case APIDecisionModeAuto:
		return c.AllowsAnonymousAPI() || c.ForceModelEval
	default:
		return false
	}
}

func (c Config) UsesSchemaStructuredOutput() bool {
	if !c.UsesStructuredAPIDecisions() {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(c.APIProvider), "openai") {
		return false
	}
	if c.AllowsAnonymousAPI() {
		return false
	}
	return true
}

func (c Config) ClaudeCLIEnv() map[string]string {
	env := map[string]string{}
	if c.APIBaseURL != "" {
		env["ANTHROPIC_BASE_URL"] = c.APIBaseURL
	}
	if c.APIKey != "" {
		env["ANTHROPIC_AUTH_TOKEN"] = c.APIKey
	}
	if strings.TrimSpace(c.ClaudeCodeDisableNonessentialTraffic) != "" {
		env["CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC"] = c.ClaudeCodeDisableNonessentialTraffic
	}
	if strings.TrimSpace(c.ClaudeCodeAttributionHeader) != "" {
		env["CLAUDE_CODE_ATTRIBUTION_HEADER"] = c.ClaudeCodeAttributionHeader
	}
	if len(env) == 0 {
		return nil
	}
	return env
}

func (c Config) AllowsAnonymousAPI() bool {
	return isTrustedLocalAPIBaseURL(c.APIBaseURL)
}

func (c Config) UsesTrustedLANAPI() bool {
	return isTrustedLANAPIBaseURL(c.APIBaseURL)
}

func (c Config) UsesLoopbackAPI() bool {
	return isLoopbackAPIBaseURL(c.APIBaseURL)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func resolveGameDir() string {
	return firstNonEmpty(
		os.Getenv("STS2_GAME_DIR"),
		`C:\Program Files (x86)\Steam\steamapps\common\Slay the Spire 2`,
	)
}

func NormalizeGameFastMode(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	switch value {
	case "", "normal", "fast", "instant":
		return value
	default:
		return value
	}
}

func parseIntWithDefault(value string, fallback int) int {
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}

	return parsed
}

func parseBoolEnv(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func normalizeMaxAttempts(value int) int {
	if value < 0 {
		return 1
	}
	return value
}

func normalizeMaxCycles(value int) int {
	if value < -1 {
		return -1
	}
	return value
}

func normalizeModelContext(value int) int {
	if value <= 0 {
		return 8192
	}
	return value
}

func normalizeAPITimeoutMs(value int) int {
	if value <= 0 {
		return 10 * 60 * 1000
	}
	return value
}

func normalizeAPIMaxRetries(value int) int {
	if value < 0 {
		return 0
	}
	return value
}

func normalizeAPIDecisionMode(value string) APIDecisionMode {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "tools", "tool", "agent", "tool-agent", "tool_agent":
		return APIDecisionModeTools
	case "structured", "json", "decision":
		return APIDecisionModeStructured
	case "auto":
		return APIDecisionModeAuto
	default:
		return APIDecisionModeTools
	}
}

func (c Config) EffectiveMaxCycles() int {
	switch {
	case c.MaxCycles == 0:
		return 0
	case c.MaxCycles > 0:
		return c.MaxCycles
	case c.MaxAttempts <= 0:
		return 0
	default:
		return 600
	}
}

func resolveModelProvider(apiBaseURL string, apiKey string, claudeCLIPath string) ModelProvider {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("SPIRE2MIND_MODEL_PROVIDER"))) {
	case "api":
		return ModelProviderAPI
	case "claude-cli", "claude_cli", "cli":
		return ModelProviderClaudeCLI
	}

	if apiBaseURL != "" && (apiKey != "" || isTrustedLocalAPIBaseURL(apiBaseURL)) {
		return ModelProviderAPI
	}

	if claudeCLIPath != "" {
		return ModelProviderClaudeCLI
	}

	return ""
}

func resolveAPITimeoutMs(apiBaseURL string) int {
	override := parseIntWithDefault(firstNonEmpty(os.Getenv("SPIRE2MIND_API_TIMEOUT_MS"), os.Getenv("API_TIMEOUT_MS")), 0)
	if override > 0 {
		return normalizeAPITimeoutMs(override)
	}

	switch {
	case isTrustedLANAPIBaseURL(apiBaseURL):
		return 30 * 60 * 1000
	case isLoopbackAPIBaseURL(apiBaseURL):
		return 20 * 60 * 1000
	default:
		return 10 * 60 * 1000
	}
}

func resolveAPIMaxRetries(apiBaseURL string) int {
	override := parseIntWithDefault(firstNonEmpty(os.Getenv("SPIRE2MIND_API_MAX_RETRIES")), -1)
	if override >= 0 {
		return normalizeAPIMaxRetries(override)
	}

	switch {
	case isTrustedLANAPIBaseURL(apiBaseURL):
		return 1
	case isLoopbackAPIBaseURL(apiBaseURL):
		return 2
	default:
		return 3
	}
}

func resolveClaudeCLIPath() string {
	override := firstNonEmpty(
		os.Getenv("SPIRE2MIND_CLAUDE_CLI_PATH"),
		os.Getenv("CLAUDE_CODE_CLI_PATH"),
	)
	if override != "" {
		return override
	}

	for _, candidate := range []string{"claude", "claude.exe"} {
		if resolved, err := exec.LookPath(candidate); err == nil {
			return resolved
		}
	}

	userProfile := os.Getenv("USERPROFILE")
	if userProfile == "" {
		return ""
	}

	patterns := []string{
		filepath.Join(userProfile, ".vscode", "extensions", "anthropic.claude-code-*", "resources", "native-binary", "claude.exe"),
		filepath.Join(userProfile, ".vscode-insiders", "extensions", "anthropic.claude-code-*", "resources", "native-binary", "claude.exe"),
	}

	var matches []string
	for _, pattern := range patterns {
		globbed, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		matches = append(matches, globbed...)
	}

	if len(matches) == 0 {
		return ""
	}

	sort.Strings(matches)
	return matches[len(matches)-1]
}

func resolveGamePrefsPath() string {
	override := firstNonEmpty(
		os.Getenv("SPIRE2MIND_GAME_PREFS_PATH"),
		os.Getenv("SPIRE2MIND_PREFS_PATH"),
	)
	if override != "" {
		return override
	}

	appData := os.Getenv("APPDATA")
	if appData == "" {
		return ""
	}

	candidates := collectPrefsCandidates(
		filepath.Join(appData, "SlayTheSpire2", "steam", "*", "modded", "profile1", "saves", "prefs.save"),
		filepath.Join(appData, "SlayTheSpire2", "steam", "*", "profile1", "saves", "prefs.save"),
	)
	if len(candidates) == 0 {
		return ""
	}

	type candidateInfo struct {
		path    string
		modTime int64
	}

	best := candidateInfo{}
	for _, candidate := range candidates {
		info, err := os.Stat(candidate)
		if err != nil || info.IsDir() {
			continue
		}
		if info.ModTime().UnixNano() > best.modTime {
			best = candidateInfo{
				path:    candidate,
				modTime: info.ModTime().UnixNano(),
			}
		}
	}

	if best.path != "" {
		return best.path
	}

	sort.Strings(candidates)
	return candidates[len(candidates)-1]
}

func collectPrefsCandidates(patterns ...string) []string {
	var candidates []string
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		candidates = append(candidates, matches...)
	}
	return candidates
}

func normalizeAPIProvider(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "openai":
		return "openai"
	case "anthropic":
		return "anthropic"
	default:
		return ""
	}
}

func isTrustedLocalAPIBaseURL(value string) bool {
	host, ip, ok := parseAPIHost(value)
	if !ok {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip.IsLoopback() {
		return true
	}
	return isPrivateIPv4(ip)
}

func isLoopbackAPIBaseURL(value string) bool {
	host, ip, ok := parseAPIHost(value)
	if !ok {
		return false
	}
	return strings.EqualFold(host, "localhost") || ip.IsLoopback()
}

func isTrustedLANAPIBaseURL(value string) bool {
	host, ip, ok := parseAPIHost(value)
	if !ok {
		return false
	}
	if strings.EqualFold(host, "localhost") || ip.IsLoopback() {
		return false
	}
	return isPrivateIPv4(ip)
}

func parseAPIHost(value string) (string, net.IP, bool) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return "", nil, false
	}

	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", nil, false
	}

	host := strings.TrimSpace(parsed.Hostname())
	if host == "" {
		return "", nil, false
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return host, nil, false
	}
	return host, ip, true
}

func isPrivateIPv4(ip net.IP) bool {
	if ip == nil {
		return false
	}

	if ipv4 := ip.To4(); ipv4 != nil {
		if ipv4[0] == 10 {
			return true
		}
		if ipv4[0] == 192 && ipv4[1] == 168 {
			return true
		}
		if ipv4[0] == 172 && ipv4[1] >= 16 && ipv4[1] <= 31 {
			return true
		}
	}
	return false
}
