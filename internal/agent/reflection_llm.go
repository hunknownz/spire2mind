package agentruntime

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"spire2mind/internal/game"
)

const llmReflectionTimeout = 45 * time.Second

// llmReflectionRequest is the structured request sent to the LLM.
type llmReflectionRequest struct {
	Floor         int      `json:"floor"`
	CharacterID   string   `json:"character_id"`
	Outcome       string   `json:"outcome"`
	FinalHP       int      `json:"final_hp"`
	MaxHP         int      `json:"max_hp"`
	RecentActions []string `json:"recent_actions"`
	Lessons       []string `json:"rule_lessons"`
}

// llmReflectionResponse is the expected LLM response structure.
type llmReflectionResponse struct {
	RootCause       string   `json:"root_cause"`
	KeyMistakes     []string `json:"key_mistakes"`
	ActionableTips  []string `json:"actionable_tips"`
	NextRunGuidance string   `json:"next_run_guidance"`
}

// enrichReflectionWithLLM calls the LLM to enhance the reflection with deeper insights.
// It returns the original reflection unchanged if the LLM call fails.
func (s *Session) enrichReflectionWithLLM(reflection *AttemptReflection, state *game.StateSnapshot) *AttemptReflection {
	if s.llmProvider == nil || reflection == nil {
		return reflection
	}

	ctx, cancel := context.WithTimeout(context.Background(), llmReflectionTimeout)
	defer cancel()

	// Build the LLM request
	req := s.buildLLMReflectionRequest(reflection, state)

	systemPrompt := `你是杀戮尖塔 2（Slay the Spire 2）的复盘教练。
你的任务是对一局失败的游戏进行深度分析，找出关键错误并给出可操作的改进建议。
回复必须是 JSON 格式，包含 root_cause, key_mistakes, actionable_tips, next_run_guidance 字段。`

	userPrompt := fmt.Sprintf(`请对以下失败的游戏进行深度复盘：

- 楼层: %d
- 角色: %s
- 结局: %s
- 最终血量: %d/%d

## 近期行动
%s

## 规则系统生成的教训（供参考）
%s

请返回 JSON 格式的深度分析。`,
		req.Floor,
		req.CharacterID,
		req.Outcome,
		req.FinalHP,
		req.MaxHP,
		strings.Join(req.RecentActions, "\n"),
		strings.Join(req.Lessons, "\n"),
	)

	response, err := s.llmProvider.Complete(ctx, systemPrompt, userPrompt)
	if err != nil {
		s.emit(SessionEvent{
			Time:    time.Now(),
			Kind:    SessionEventStatus,
			Cycle:   s.cycle,
			Message: s.say("LLM reflection failed, using rule-based lessons", "LLM 反思失败，使用规则教训"),
		})
		return reflection
	}

	// Parse LLM response
	llmResp := s.parseLLMReflectionResponse(response)
	if llmResp == nil {
		return reflection
	}

	// Enrich the reflection with LLM insights
	s.mergeLLMInsights(reflection, llmResp)

	s.emit(SessionEvent{
		Time:    time.Now(),
		Kind:    SessionEventStatus,
		Cycle:   s.cycle,
		Message: s.say("LLM reflection enriched with deep analysis", "LLM 反思完成，已融入深度分析"),
		Data: map[string]interface{}{
			"llm_root_cause":      llmResp.RootCause,
			"llm_key_mistakes":    llmResp.KeyMistakes,
			"llm_actionable_tips": llmResp.ActionableTips,
		},
	})

	return reflection
}

func (s *Session) buildLLMReflectionRequest(reflection *AttemptReflection, state *game.StateSnapshot) *llmReflectionRequest {
	req := &llmReflectionRequest{
		Outcome:     reflection.Outcome,
		CharacterID: reflection.CharacterID,
		Lessons:     reflection.Lessons,
	}

	if reflection.Floor != nil {
		req.Floor = *reflection.Floor
	}

	if state != nil && state.Run != nil {
		req.FinalHP = state.Run.CurrentHp
		req.MaxHP = state.Run.MaxHp
	}

	// Get recent actions from compact memory
	if s.compact != nil {
		req.RecentActions = s.compact.RecentTimeline(10)
	}

	return req
}

func (s *Session) parseLLMReflectionResponse(response string) *llmReflectionResponse {
	// Try to extract JSON from the response
	response = strings.TrimSpace(response)

	// Find JSON block if wrapped in markdown code blocks
	if idx := strings.Index(response, "```json"); idx >= 0 {
		response = response[idx+7:]
		if endIdx := strings.Index(response, "```"); endIdx >= 0 {
			response = response[:endIdx]
		}
	} else if idx := strings.Index(response, "```"); idx >= 0 {
		response = response[idx+3:]
		if endIdx := strings.Index(response, "```"); endIdx >= 0 {
			response = response[:endIdx]
		}
	}

	response = strings.TrimSpace(response)

	var llmResp llmReflectionResponse
	if err := json.Unmarshal([]byte(response), &llmResp); err != nil {
		return nil
	}

	return &llmResp
}

func (s *Session) mergeLLMInsights(reflection *AttemptReflection, llmResp *llmReflectionResponse) {
	// Prepend LLM insights to the existing lessons
	var enrichedLessons []string

	// Add LLM root cause as the primary lesson
	if llmResp.RootCause != "" {
		enrichedLessons = append(enrichedLessons, llmResp.RootCause)
	}

	// Add actionable tips
	for _, tip := range llmResp.ActionableTips {
		if tip != "" {
			enrichedLessons = append(enrichedLessons, tip)
		}
	}

	// Keep original lessons (but avoid duplicates)
	existingLessons := make(map[string]bool)
	for _, l := range enrichedLessons {
		existingLessons[strings.ToLower(l)] = true
	}

	for _, l := range reflection.Lessons {
		if !existingLessons[strings.ToLower(l)] {
			enrichedLessons = append(enrichedLessons, l)
		}
	}

	reflection.Lessons = enrichedLessons

	// Update next plan with LLM guidance
	if llmResp.NextRunGuidance != "" {
		reflection.NextPlan = llmResp.NextRunGuidance
	}

	// Add key mistakes to tactical mistakes
	for _, mistake := range llmResp.KeyMistakes {
		if mistake != "" {
			reflection.TacticalMistakes = append(reflection.TacticalMistakes, mistake)
		}
	}
}
