package agentruntime

import (
	"context"
	"strings"
	"time"

	"spire2mind/internal/game"
)

func (s *Session) maybeEmitStreamerBeatForMoment(ctx context.Context, moment StreamerMoment) {
	if s == nil || s.streamer == nil || !s.streamer.Enabled() {
		return
	}

	state := moment.ActiveState()
	if state == nil {
		return
	}

	trigger, signature, ok := s.streamer.ShouldCommentate(moment, s.lastStreamerSignature)
	if !ok {
		return
	}
	moment.Trigger = trigger

	beat, err := s.streamer.Generate(ctx, moment, s.streamerHistory, s.todo, s.compact, s.cfg.Language)
	if err != nil || beat == nil {
		s.emit(SessionEvent{
			Time:    time.Now(),
			Kind:    SessionEventStatus,
			Cycle:   s.cycle,
			Attempt: s.currentAttemptForState(state),
			Message: s.say("streamer commentary skipped because generation failed", "主播解说生成失败，本轮跳过。"),
			Screen:  state.Screen,
			RunID:   state.RunID,
			Data: map[string]interface{}{
				"streamer_error": valueOrDash(errorString(err)),
				"trigger":        trigger,
				"action":         strings.TrimSpace(moment.Action),
			},
		})
		return
	}

	queuePath, queueErr := s.streamer.WriteTTSArtifacts(beat)
	s.lastStreamerSignature = signature

	data := map[string]interface{}{
		"trigger":         beat.Trigger,
		"mood":            beat.Mood,
		"commentary":      beat.Commentary,
		"game_insight":    beat.GameInsight,
		"life_reflection": beat.LifeReflection,
		"tts_text":        beat.TTSText,
		"tts_segments":    append([]string(nil), beat.TTSSegments...),
		"action":          strings.TrimSpace(moment.Action),
		"outcome":         strings.TrimSpace(moment.Outcome),
	}
	if queuePath != "" {
		data["tts_queue_path"] = queuePath
	}
	if queueErr != nil {
		data["tts_queue_error"] = queueErr.Error()
	}

	s.emit(SessionEvent{
		Time:    time.Now(),
		Kind:    SessionEventStreamer,
		Cycle:   s.cycle,
		Attempt: s.currentAttemptForState(state),
		Message: beat.Commentary,
		Screen:  state.Screen,
		RunID:   state.RunID,
		State:   state,
		Data:    data,
	})
}

func (s *Session) maybeEmitPassiveStreamerBeat(ctx context.Context, state *game.StateSnapshot) {
	if state == nil {
		return
	}
	switch strings.ToUpper(strings.TrimSpace(state.Screen)) {
	case "GAME_OVER":
		s.maybeEmitStreamerBeatForMoment(ctx, StreamerMoment{
			AfterState: state,
			Trigger:    "game_over",
			Outcome:    s.say("the run has ended", "这一局已经结束。"),
		})
	}
}

func (s *Session) appendStreamerHistory(event SessionEvent) {
	if s == nil || s.cfg.StreamerMaxHistory <= 0 {
		return
	}

	entry := streamerHistoryEntry(event)
	if entry == "" {
		return
	}
	s.streamerHistory = appendTrimmed(s.streamerHistory, entry, s.cfg.StreamerMaxHistory)
}

func streamerHistoryEntry(event SessionEvent) string {
	text := strings.TrimSpace(event.Message)
	if text == "" {
		return ""
	}

	switch event.Kind {
	case SessionEventAction:
		if strings.TrimSpace(event.Action) != "" {
			return "[动作] " + strings.TrimSpace(event.Action) + " - " + text
		}
		return "[动作] " + text
	case SessionEventToolError:
		return "[异常] " + text
	case SessionEventReflection:
		return "[反思] " + text
	case SessionEventStreamer:
		return "[解说] " + text
	case SessionEventStatus:
		if strings.Contains(text, "structured decision received") || strings.Contains(text, "starting agent cycle") {
			return ""
		}
		return "[状态] " + text
	default:
		return ""
	}
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
