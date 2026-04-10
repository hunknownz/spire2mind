package i18n

import (
	"bytes"
	"strings"
	"unicode"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

var suspiciousMojibakeFragments = []string{
	"寮€",
	"褰撳",
	"鍔ㄤ綔",
	"鎵撳",
	"瑙ｈ",
	"鏈€杩",
	"闃插",
	"锟",
	"鈧",
	"\uFFFD",
}

var suspiciousMojibakeRunes = map[rune]struct{}{
	'寮': {}, '褰': {}, '鍔': {}, '瑙': {}, '鏈': {}, '闃': {}, '鎵': {}, '闂': {},
	'鍗': {}, '墠': {}, '璺': {}, '眬': {}, '嚮': {}, '尽': {}, '銆': {}, '锛': {},
	'鈥': {}, '锟': {}, '鈧': {},
}

var chinesePunctuation = []rune{'，', '。', '！', '？', '；', '：', '“', '”', '‘', '’', '（', '）', '《', '》', '【', '】', '—', '…', '、'}

func RepairText(text string) string {
	if text == "" || !containsNonASCII(text) {
		return text
	}

	current := text
	for i := 0; i < 2; i++ {
		repaired, ok := repairOnce(current)
		if !ok || repaired == current {
			break
		}
		current = repaired
	}
	return current
}

func RepairAny(value any) any {
	switch typed := value.(type) {
	case string:
		return RepairText(typed)
	case []string:
		fixed := make([]string, len(typed))
		for i, item := range typed {
			fixed[i] = RepairText(item)
		}
		return fixed
	case []any:
		fixed := make([]any, len(typed))
		for i, item := range typed {
			fixed[i] = RepairAny(item)
		}
		return fixed
	case map[string]any:
		fixed := make(map[string]any, len(typed))
		for key, item := range typed {
			fixed[key] = RepairAny(item)
		}
		return fixed
	case map[string]string:
		fixed := make(map[string]string, len(typed))
		for key, item := range typed {
			fixed[key] = RepairText(item)
		}
		return fixed
	default:
		return value
	}
}

func repairOnce(text string) (string, bool) {
	encoded, _, err := transform.Bytes(simplifiedchinese.GBK.NewEncoder(), []byte(text))
	if err != nil || len(encoded) == 0 {
		return "", false
	}

	candidate := decodeRecoveredUTF8(encoded, text)
	if candidate == "" {
		return "", false
	}
	if candidate == text {
		return "", false
	}
	if !shouldPreferRepair(text, candidate) {
		return "", false
	}
	return candidate, true
}

func decodeRecoveredUTF8(encoded []byte, original string) string {
	patched := repairBrokenUTF8Punctuation(encoded)
	candidate := string(bytes.ToValidUTF8(patched, []byte("�")))
	candidate = normalizeRecoveredPunctuation(candidate, original)
	return candidate
}

func repairBrokenUTF8Punctuation(raw []byte) []byte {
	if len(raw) < 3 {
		return raw
	}

	patched := make([]byte, 0, len(raw))
	for i := 0; i < len(raw); i++ {
		if i+2 < len(raw) {
			switch {
			case raw[i] == 0xE3 && raw[i+1] == 0x80 && raw[i+2] == '?':
				// Most frequently this is a truncated Chinese full stop from mojibake like "銆?".
				patched = append(patched, 0xE3, 0x80, 0x82)
				i += 2
				continue
			case raw[i] == 0xE2 && raw[i+1] == 0x80 && raw[i+2] == '?':
				// Collapse unknown truncated smart punctuation to an ellipsis-like pause.
				patched = append(patched, 0xE2, 0x80, 0xA6)
				i += 2
				continue
			}
		}
		patched = append(patched, raw[i])
	}
	return patched
}

func normalizeRecoveredPunctuation(candidate string, original string) string {
	replaced := candidate
	if strings.Contains(original, "銆") {
		replaced = strings.ReplaceAll(replaced, "�?", "。")
		replaced = strings.ReplaceAll(replaced, "�", "。")
	}
	if strings.Contains(original, "锛") {
		replaced = strings.ReplaceAll(replaced, "�?", "！")
	}
	if strings.Contains(original, "鈥") {
		replaced = strings.ReplaceAll(replaced, "�?", "…")
	}
	return replaced
}

func shouldPreferRepair(original string, candidate string) bool {
	originalHits := suspiciousHitCount(original)
	candidateHits := suspiciousHitCount(candidate)

	if candidateHits >= originalHits && originalHits == 0 {
		return false
	}
	if candidateHits < originalHits {
		return true
	}

	originalScore := chineseSignalScore(original)
	candidateScore := chineseSignalScore(candidate)
	return candidateScore > originalScore+2
}

func suspiciousHitCount(text string) int {
	hits := 0
	for _, fragment := range suspiciousMojibakeFragments {
		hits += strings.Count(text, fragment)
	}
	for _, r := range text {
		if _, ok := suspiciousMojibakeRunes[r]; ok {
			hits++
		}
	}
	return hits
}

func chineseSignalScore(text string) int {
	score := 0
	for _, r := range text {
		switch {
		case unicode.Is(unicode.Han, r):
			score += 2
		case isChinesePunctuation(r):
			score += 3
		case unicode.IsLetter(r), unicode.IsDigit(r), unicode.IsSpace(r):
			score++
		}
	}
	return score
}

func isChinesePunctuation(r rune) bool {
	for _, punct := range chinesePunctuation {
		if punct == r {
			return true
		}
	}
	return false
}

func containsNonASCII(text string) bool {
	for _, r := range text {
		if r > unicode.MaxASCII {
			return true
		}
	}
	return false
}
