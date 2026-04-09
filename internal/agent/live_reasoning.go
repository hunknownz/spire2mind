package agentruntime

import (
	"regexp"
	"strings"
)

var liveReasoningNoisePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)tool invocation`),
	regexp.MustCompile(`(?i)explicit parameters`),
	regexp.MustCompile(`(?i)try with`),
	regexp.MustCompile(`(?i)parameter form`),
	regexp.MustCompile(`(?i)calling format`),
	regexp.MustCompile(`(?i)^something seems wrong`),
	regexp.MustCompile(`(?i)^\s*\{.*"action"\s*:`),
}

func sanitizeLiveReasoning(text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}

	lines := strings.Split(text, "\n")
	filtered := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		noisy := false
		for _, pattern := range liveReasoningNoisePatterns {
			if pattern.MatchString(trimmed) {
				noisy = true
				break
			}
		}
		if noisy {
			continue
		}
		filtered = append(filtered, trimmed)
	}

	result := strings.TrimSpace(strings.Join(filtered, " "))
	if strings.HasPrefix(result, "{") && strings.Contains(result, `"action"`) {
		return ""
	}
	if result == "." {
		return ""
	}
	return result
}
