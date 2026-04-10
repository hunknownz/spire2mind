package agentruntime

import (
	"strings"
	"testing"

	"spire2mind/internal/config"
	"spire2mind/internal/game"
	"spire2mind/internal/i18n"
)

func TestParseStreamerBeatParsesFencedJSON(t *testing.T) {
	text := "```json\n{\"trigger\":\"map_choice\",\"mood\":\"兴奋\",\"commentary\":\"这条路线不花，但很稳。观众现在最该看的，是我们怎么把节奏收住。\",\"game_insight\":\"这一拍更重要的是稳定推进，而不是贪上限。\",\"life_reflection\":\"很多时候先把局势稳下来，比一时冲动更值钱。\",\"tts_text\":\"这条路线更稳，我们先把节奏收住。\"}\n```"

	beat, err := parseStreamerBeat(text)
	if err != nil {
		t.Fatalf("parseStreamerBeat returned error: %v", err)
	}

	if beat.Trigger != "map_choice" {
		t.Fatalf("expected trigger map_choice, got %q", beat.Trigger)
	}
	if beat.Mood != "兴奋" {
		t.Fatalf("expected mood to survive parse, got %q", beat.Mood)
	}
}

func TestStreamerDirectorShouldCommentateUsesActionBasedTrigger(t *testing.T) {
	director := NewStreamerDirector(config.Config{StreamerEnabled: true}, &Runtime{})
	moment := StreamerMoment{
		Action: "choose_map_node",
		AfterState: &game.StateSnapshot{
			Screen: "COMBAT",
			RunID:  "RUN123",
			Run: map[string]any{
				"floor": 4,
			},
			AgentView: map[string]any{
				"headline": "战斗开始",
			},
		},
		Outcome: "Action completed.",
	}

	trigger, signature, ok := director.ShouldCommentate(moment, "")
	if !ok {
		t.Fatalf("expected moment to trigger commentary")
	}
	if trigger != "map_choice" {
		t.Fatalf("expected action-based trigger map_choice, got %q", trigger)
	}
	if _, _, ok := director.ShouldCommentate(moment, signature); ok {
		t.Fatalf("expected duplicate signature to suppress commentary")
	}
}

func TestStreamerBuildPromptEmphasizesResultInsteadOfProcedure(t *testing.T) {
	director := NewStreamerDirector(config.Config{StreamerEnabled: true, StreamerStyle: "bright-cute"}, &Runtime{})
	moment := StreamerMoment{
		Action:  "play_card",
		Outcome: "Action completed.",
		BeforeState: &game.StateSnapshot{
			Screen: "COMBAT",
			RunID:  "RUN123",
			Run:    map[string]any{"floor": 8},
		},
		AfterState: &game.StateSnapshot{
			Screen: "COMBAT",
			RunID:  "RUN123",
			Run:    map[string]any{"floor": 8},
		},
	}

	prompt := director.buildPromptV2(moment, []string{"[动作] play_card - 打出痛击"}, NewTodoManager(), NewCompactMemory(), i18n.LanguageChinese)
	if !strings.Contains(prompt, "刚刚落地的动作") {
		t.Fatalf("expected prompt to mention landed action, got: %s", prompt)
	}
	if !strings.Contains(prompt, "不要像系统提示一样念步骤") {
		t.Fatalf("expected prompt to discourage procedural narration, got: %s", prompt)
	}
	if !strings.Contains(prompt, "主播风格") {
		t.Fatalf("expected prompt to include streamer style guidance, got: %s", prompt)
	}
}

func TestParseSpeechSegmentsParsesFencedJSON(t *testing.T) {
	text := "```json\n{\"messages\":[\"先别慌。\",\"这一拍最重要的是保血。\"]}\n```"

	segments, err := parseSpeechSegments(text)
	if err != nil {
		t.Fatalf("parseSpeechSegments returned error: %v", err)
	}

	if len(segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segments))
	}
	if segments[0] != "先别慌。" {
		t.Fatalf("unexpected first segment: %q", segments[0])
	}
}

func TestFallbackTTSSegmentsSplitsNaturalSentenceBoundaries(t *testing.T) {
	segments := fallbackTTSSegmentsV2("先别慌。这一拍最重要的是保血！打完再想后面的事。")
	if len(segments) != 3 {
		t.Fatalf("expected 3 segments, got %d (%v)", len(segments), segments)
	}
	if segments[1] != "这一拍最重要的是保血" {
		t.Fatalf("unexpected second segment: %q", segments[1])
	}
}

func TestFallbackStreamerBeatRejectsDecisionJSON(t *testing.T) {
	raw := `{"action":"play_card","card_index":0,"reason":"先出牌"}`
	beat := fallbackStreamerBeatV2(StreamerMoment{
		Action: "play_card",
		AfterState: &game.StateSnapshot{
			Screen: "COMBAT",
		},
	}, raw, i18n.LanguageChinese)
	if strings.Contains(beat.Commentary, `"action"`) {
		t.Fatalf("expected fallback commentary to replace decision json, got %q", beat.Commentary)
	}
	if strings.TrimSpace(beat.Commentary) == "" {
		t.Fatalf("expected non-empty fallback commentary")
	}
}
