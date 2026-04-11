package analyst

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ArchetypeAnalysis describes a deck building direction.
type ArchetypeAnalysis struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	Character       string          `json:"character"`
	Description     string          `json:"description"`
	CoreCards       FlexStringSlice `json:"core_cards"`
	SupportCards    FlexStringSlice `json:"support_cards"`
	AvoidCards      FlexStringSlice `json:"avoid_cards"`
	KeyRelics       FlexStringSlice `json:"key_relics"`
	Strengths       string          `json:"strengths"`
	Weaknesses      string          `json:"weaknesses"`
	EarlyPriority   string          `json:"early_priority"`
	MidPriority     string          `json:"mid_priority"`
	LatePriority    string          `json:"late_priority"`
	PathingAdvice   string          `json:"pathing_advice"`
	TransitionFloor int             `json:"transition_floor"`
}

// AnalyzeArchetypes generates deck building direction guides.
func (a *Analyst) AnalyzeArchetypes(ctx context.Context) error {
	archetypesPath := filepath.Join(a.knowledgeDir, "archetypes", "ironclad.json")

	existing := make(map[string]ArchetypeAnalysis)
	if data, err := os.ReadFile(archetypesPath); err == nil {
		json.Unmarshal(data, &existing)
	}

	if len(existing) > 0 {
		fmt.Printf("Archetypes already exist: %d entries\n", len(existing))
		return nil
	}

	systemPrompt := `你是杀戮尖塔 2（Slay the Spire 2）的资深构筑分析师。
你需要为铁甲战士（Ironclad）角色分析所有可行的牌组构筑方向。
只返回 JSON 数组，不要输出其他任何文本。`

	userPrompt := `请分析杀戮尖塔 2 中铁甲战士（Ironclad）的主要牌组构筑方向。

对每个构筑方向给出：
1. id: 英文 ID（如 strength, block, exhaust, scaling, mixed）
2. name: 中文名（如 "力量流", "格挡流"）
3. character: "IRONCLAD"
4. description: 构筑思路描述
5. core_cards: 核心卡牌 ID 列表（5-8 张）
6. support_cards: 辅助卡牌 ID 列表（5-8 张）
7. avoid_cards: 应避免的卡牌 ID 列表
8. key_relics: 关键遗物 ID 列表
9. strengths: 优势描述
10. weaknesses: 劣势描述
11. early_priority: 前期优先事项（1-5 层）
12. mid_priority: 中期优先事项（6-10 层）
13. late_priority: 后期优先事项（11+ 层）
14. pathing_advice: 路线选择建议
15. transition_floor: 构筑转型的关键层数

请列出至少 4 种构筑方向。返回 JSON 数组。`

	fmt.Println("Generating archetype analysis via LLM...")
	response, err := a.llm.Complete(ctx, systemPrompt, userPrompt)
	if err != nil {
		return fmt.Errorf("LLM archetype analysis: %w", err)
	}

	jsonStr := extractJSON(response)
	var results []ArchetypeAnalysis
	if err := json.Unmarshal([]byte(jsonStr), &results); err != nil {
		return fmt.Errorf("parse archetype analysis: %w (response: %.200s)", err, response)
	}

	for _, r := range results {
		existing[r.ID] = r
	}

	if err := writeJSON(archetypesPath, existing); err != nil {
		return fmt.Errorf("write archetypes: %w", err)
	}

	fmt.Printf("Archetype analysis complete: %d archetypes\n", len(existing))
	return nil
}
