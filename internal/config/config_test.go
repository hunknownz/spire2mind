package config

import (
	"os"
	"testing"
)

func TestEffectiveMaxCyclesDefaultsToFiniteLimitForBoundedAttempts(t *testing.T) {
	cfg := Config{MaxAttempts: 3, MaxCycles: -1}
	if got := cfg.EffectiveMaxCycles(); got != 600 {
		t.Fatalf("EffectiveMaxCycles() = %d, want 600", got)
	}
}

func TestEffectiveMaxCyclesDefaultsToUnlimitedForContinuousAttempts(t *testing.T) {
	cfg := Config{MaxAttempts: 0, MaxCycles: -1}
	if got := cfg.EffectiveMaxCycles(); got != 0 {
		t.Fatalf("EffectiveMaxCycles() = %d, want 0", got)
	}
}

func TestEffectiveMaxCyclesRespectsExplicitOverride(t *testing.T) {
	cfg := Config{MaxAttempts: 0, MaxCycles: 2400}
	if got := cfg.EffectiveMaxCycles(); got != 2400 {
		t.Fatalf("EffectiveMaxCycles() = %d, want 2400", got)
	}
}

func TestEffectiveMaxCyclesZeroMeansUnlimited(t *testing.T) {
	cfg := Config{MaxAttempts: 5, MaxCycles: 0}
	if got := cfg.EffectiveMaxCycles(); got != 0 {
		t.Fatalf("EffectiveMaxCycles() = %d, want 0", got)
	}
}

func TestClaudeCLIEnvIncludesConfiguredAnthropicOverrides(t *testing.T) {
	cfg := Config{
		APIBaseURL:                           "https://code.aipor.cc",
		APIKey:                               "sk-test",
		ClaudeCodeDisableNonessentialTraffic: "1",
		ClaudeCodeAttributionHeader:          "0",
	}

	env := cfg.ClaudeCLIEnv()
	if env["ANTHROPIC_BASE_URL"] != "https://code.aipor.cc" {
		t.Fatalf("ANTHROPIC_BASE_URL = %q", env["ANTHROPIC_BASE_URL"])
	}
	if env["ANTHROPIC_AUTH_TOKEN"] != "sk-test" {
		t.Fatalf("ANTHROPIC_AUTH_TOKEN = %q", env["ANTHROPIC_AUTH_TOKEN"])
	}
	if env["CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC"] != "1" {
		t.Fatalf("CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC = %q", env["CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC"])
	}
	if env["CLAUDE_CODE_ATTRIBUTION_HEADER"] != "0" {
		t.Fatalf("CLAUDE_CODE_ATTRIBUTION_HEADER = %q", env["CLAUDE_CODE_ATTRIBUTION_HEADER"])
	}
}

func TestHasModelConfigAllowsAnonymousLoopbackAPI(t *testing.T) {
	cfg := Config{
		ModelProvider: ModelProviderAPI,
		APIBaseURL:    "http://127.0.0.1:11434",
		Model:         "qwen3:8b",
	}

	if !cfg.HasModelConfig() {
		t.Fatal("expected loopback API config without api key to be accepted")
	}
}

func TestHasModelConfigAllowsAnonymousPrivateLANAPI(t *testing.T) {
	cfg := Config{
		ModelProvider: ModelProviderAPI,
		APIBaseURL:    "http://192.168.3.23:11434",
		Model:         "qwen3.5:35b-a3b",
	}

	if !cfg.HasModelConfig() {
		t.Fatal("expected trusted private LAN API config without api key to be accepted")
	}
}

func TestLoadReadsAPIProviderOverride(t *testing.T) {
	t.Setenv("SPIRE2MIND_MODEL_PROVIDER", "api")
	t.Setenv("SPIRE2MIND_API_BASE_URL", "http://127.0.0.1:11434")
	t.Setenv("SPIRE2MIND_MODEL", "qwen3:8b")
	t.Setenv("SPIRE2MIND_API_PROVIDER", "openai")
	t.Setenv("USERPROFILE", os.Getenv("USERPROFILE"))

	cfg := Load(`C:\repo\spire2mind`)
	if cfg.APIProvider != "openai" {
		t.Fatalf("APIProvider = %q, want openai", cfg.APIProvider)
	}
}

func TestLoadReadsForceModelEvalFlag(t *testing.T) {
	t.Setenv("SPIRE2MIND_MODEL_PROVIDER", "api")
	t.Setenv("SPIRE2MIND_API_BASE_URL", "http://127.0.0.1:11434")
	t.Setenv("SPIRE2MIND_MODEL", "qwen3:8b")
	t.Setenv("SPIRE2MIND_FORCE_MODEL_EVAL", "1")

	cfg := Load(`C:\repo\spire2mind`)
	if !cfg.ForceModelEval {
		t.Fatal("expected ForceModelEval to be true")
	}
}

func TestLoadReadsAPIDecisionModeOverride(t *testing.T) {
	t.Setenv("SPIRE2MIND_MODEL_PROVIDER", "api")
	t.Setenv("SPIRE2MIND_API_BASE_URL", "http://127.0.0.1:11434")
	t.Setenv("SPIRE2MIND_MODEL", "qwen3:8b")
	t.Setenv("SPIRE2MIND_API_DECISION_MODE", "structured")

	cfg := Load(`C:\repo\spire2mind`)
	if cfg.APIDecisionMode != APIDecisionModeStructured {
		t.Fatalf("APIDecisionMode = %q, want %q", cfg.APIDecisionMode, APIDecisionModeStructured)
	}
}

func TestLoadDefaultsModelContextTo8K(t *testing.T) {
	t.Setenv("SPIRE2MIND_MODEL_PROVIDER", "api")
	t.Setenv("SPIRE2MIND_API_BASE_URL", "http://127.0.0.1:11434")
	t.Setenv("SPIRE2MIND_MODEL", "qwen3:8b")

	cfg := Load(`C:\repo\spire2mind`)
	if cfg.ModelContext != 8192 {
		t.Fatalf("ModelContext = %d, want 8192", cfg.ModelContext)
	}
}

func TestLoadReadsModelContextOverride(t *testing.T) {
	t.Setenv("SPIRE2MIND_MODEL_PROVIDER", "api")
	t.Setenv("SPIRE2MIND_API_BASE_URL", "http://127.0.0.1:11434")
	t.Setenv("SPIRE2MIND_MODEL", "qwen3:8b")
	t.Setenv("SPIRE2MIND_MODEL_CONTEXT", "16384")

	cfg := Load(`C:\repo\spire2mind`)
	if cfg.ModelContext != 16384 {
		t.Fatalf("ModelContext = %d, want 16384", cfg.ModelContext)
	}
}

func TestResolveModelProviderPrefersAPIWhenBaseURLConfigured(t *testing.T) {
	t.Setenv("SPIRE2MIND_MODEL_PROVIDER", "")

	got := resolveModelProvider("https://example.invalid", "sk-test", "claude.exe")
	if got != ModelProviderAPI {
		t.Fatalf("resolveModelProvider() = %q, want %q", got, ModelProviderAPI)
	}
}

func TestResolveModelProviderFallsBackToClaudeCLIWithoutAPI(t *testing.T) {
	t.Setenv("SPIRE2MIND_MODEL_PROVIDER", "")

	got := resolveModelProvider("", "", "claude.exe")
	if got != ModelProviderClaudeCLI {
		t.Fatalf("resolveModelProvider() = %q, want %q", got, ModelProviderClaudeCLI)
	}
}

func TestResolveModelProviderSupportsAnonymousLoopbackAPI(t *testing.T) {
	t.Setenv("SPIRE2MIND_MODEL_PROVIDER", "")

	got := resolveModelProvider("http://127.0.0.1:11434", "", "claude.exe")
	if got != ModelProviderAPI {
		t.Fatalf("resolveModelProvider() = %q, want %q", got, ModelProviderAPI)
	}
}

func TestResolveModelProviderSupportsAnonymousPrivateLANAPI(t *testing.T) {
	t.Setenv("SPIRE2MIND_MODEL_PROVIDER", "")

	got := resolveModelProvider("http://192.168.3.23:11434", "", "claude.exe")
	if got != ModelProviderAPI {
		t.Fatalf("resolveModelProvider() = %q, want %q", got, ModelProviderAPI)
	}
}

func TestResolveModelProviderPrefersLoopbackAPIWithoutKeyWhenNoCLI(t *testing.T) {
	got := resolveModelProvider("http://127.0.0.1:11434", "", "")
	if got != ModelProviderAPI {
		t.Fatalf("resolveModelProvider(loopback, no key, no cli) = %q, want %q", got, ModelProviderAPI)
	}
}

func TestResolveModelProviderDoesNotAutoSelectRemoteAPIWithoutKey(t *testing.T) {
	got := resolveModelProvider("https://code.aipor.cc", "", "")
	if got != "" {
		t.Fatalf("resolveModelProvider(remote, no key, no cli) = %q, want empty", got)
	}
}

func TestResolveModelProviderPrefersLoopbackAPIOverClaudeCLIWhenProviderNotForced(t *testing.T) {
	got := resolveModelProvider("http://127.0.0.1:11434", "", `C:\tools\claude.exe`)
	if got != ModelProviderAPI {
		t.Fatalf("resolveModelProvider(loopback, no key, cli present) = %q, want %q", got, ModelProviderAPI)
	}
}

func TestUsesStructuredAPIDecisionsHonorsMode(t *testing.T) {
	cfg := Config{
		ModelProvider:    ModelProviderAPI,
		APIBaseURL:       "http://127.0.0.1:11434",
		Model:            "qwen3:8b",
		APIDecisionMode:  APIDecisionModeStructured,
	}
	if !cfg.UsesStructuredAPIDecisions() {
		t.Fatal("expected structured API mode to be enabled")
	}
}

func TestUsesStructuredAPIDecisionsAutoEnablesForLoopback(t *testing.T) {
	cfg := Config{
		ModelProvider:    ModelProviderAPI,
		APIBaseURL:       "http://127.0.0.1:11434",
		Model:            "qwen3:8b",
		APIDecisionMode:  APIDecisionModeAuto,
	}
	if !cfg.UsesStructuredAPIDecisions() {
		t.Fatal("expected auto API mode to enable structured decisions for loopback")
	}
}

func TestUsesStructuredAPIDecisionsDefaultsToTools(t *testing.T) {
	cfg := Config{
		ModelProvider:    ModelProviderAPI,
		APIBaseURL:       "http://127.0.0.1:11434",
		Model:            "qwen3:8b",
		APIDecisionMode:  APIDecisionModeTools,
	}
	if cfg.UsesStructuredAPIDecisions() {
		t.Fatal("expected tools mode to keep API tool-agent behavior")
	}
}
