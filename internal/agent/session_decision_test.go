package agentruntime

import "testing"

func TestClassifyStructuredDecisionFailure(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		raw          interface{}
		fallbackText string
		errText      string
		want         string
	}{
		{
			name:    "missing structured output",
			errText: "missing structured action decision",
			want:    "missing_structured_output",
		},
		{
			name:    "empty action",
			raw:     map[string]interface{}{},
			errText: "structured output missing action",
			want:    "empty_action_in_structured_output",
		},
		{
			name:    "wrong shape",
			raw:     []interface{}{},
			errText: "structured output is []interface {}, want object",
			want:    "invalid_structured_shape",
		},
		{
			name:         "fallback parse failed",
			fallbackText: "{bad json",
			errText:      "parse fallback action decision: invalid character 'b'",
			want:         "fallback_text_parse_failed",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := classifyStructuredDecisionFailure(tc.raw, tc.fallbackText, testErr(tc.errText))
			if got != tc.want {
				t.Fatalf("classifyStructuredDecisionFailure() = %q, want %q", got, tc.want)
			}
		})
	}
}

func testErr(message string) error {
	if message == "" {
		return nil
	}
	return simpleError(message)
}

type simpleError string

func (e simpleError) Error() string { return string(e) }
