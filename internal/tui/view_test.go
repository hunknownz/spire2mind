package tui

import (
	"strings"
	"testing"

	"spire2mind/internal/i18n"
)

func TestRenderStreamerPanelIncludesTTSProfileDetails(t *testing.T) {
	model := &Model{
		loc:                i18n.New(i18n.LanguageChinese),
		ttsProfileName:     "kokoro-cute",
		ttsProfileProvider: "kokoro",
		ttsProfileVoice:    "zf_xiaoxiao",
		ttsProfileSpeed:    "1.08",
		streamerStyle:      "bright-cute",
	}

	panel := model.renderStreamerPanel(72)
	for _, token := range []string{"kokoro-cute", "kokoro", "zf_xiaoxiao", "1.08", "bright-cute"} {
		if !strings.Contains(panel, token) {
			t.Fatalf("expected streamer panel to contain %q, got: %s", token, panel)
		}
	}
}
