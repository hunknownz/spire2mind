package agentruntime

import "testing"

func TestSanitizeLiveReasoningRemovesToolInvocationNoise(t *testing.T) {
	t.Parallel()

	raw := `Something seems wrong with the tool invocation. Let me try with explicit parameters.
{"action":"play_card","card_index":3}`
	if got := sanitizeLiveReasoning(raw); got != "" {
		t.Fatalf("expected empty sanitized reasoning, got %q", got)
	}
}
