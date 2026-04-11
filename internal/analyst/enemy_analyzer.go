package analyst

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// EnemyAnalysis is the LLM-generated strategy guide for an enemy.
type EnemyAnalysis struct {
	EnemyID       string   `json:"enemy_id"`
	Name          string   `json:"name"`
	FloorRange    string   `json:"floor_range"`
	Type          string   `json:"type"` // normal, elite, boss
	ThreatLevel   string   `json:"threat_level"`
	AttackPattern string   `json:"attack_pattern"`
	Strategy      string   `json:"strategy"`
	CounterCards  FlexStringSlice `json:"counter_cards"`
	DangerCards   FlexStringSlice `json:"danger_cards"`
	Notes         string   `json:"notes"`
}

// AnalyzeEnemies generates enemy strategy guides using LLM knowledge.
func (a *Analyst) AnalyzeEnemies(ctx context.Context) error {
	strategiesPath := filepath.Join(a.knowledgeDir, "enemies", "strategies.json")

	existing := make(map[string]EnemyAnalysis)
	if data, err := os.ReadFile(strategiesPath); err == nil {
		json.Unmarshal(data, &existing)
	}

	if len(existing) > 0 {
		fmt.Printf("Enemy strategies already exist: %d entries\n", len(existing))
		return nil
	}

	systemPrompt := `你是杀戮尖塔 2（Slay the Spire 2）的资深策略分析师。
你需要为游戏中的敌人生成详细的策略指南。
只返回 JSON 数组，不要输出其他任何文本。`

	userPrompt := `请为杀戮尖塔 2 中铁甲战士（Ironclad）会遇到的主要敌人生成策略指南。
包括 Act 1 的普通怪、精英怪和 Boss。

对每个敌人给出：
1. enemy_id: 大写 ID（如 CULTIST, JAW_WORM, LAGAVULIN, SLIME_BOSS 等）
2. name: 中文名
3. floor_range: 出现楼层范围（如 "1-6", "6-12"）
4. type: normal/elite/boss
5. threat_level: low/medium/high/critical
6. attack_pattern: 攻击模式描述
7. strategy: 应对策略
8. counter_cards: 克制卡牌 ID 列表
9. danger_cards: 对付这个敌人时不好用的牌
10. notes: 补充说明

请至少列出 20 个敌人。返回 JSON 数组。`

	fmt.Println("Generating enemy strategies via LLM...")
	response, err := a.llm.Complete(ctx, systemPrompt, userPrompt)
	if err != nil {
		return fmt.Errorf("LLM enemy analysis: %w", err)
	}

	jsonStr := extractJSON(response)
	var results []EnemyAnalysis
	if err := json.Unmarshal([]byte(jsonStr), &results); err != nil {
		return fmt.Errorf("parse enemy analysis: %w (response: %.200s)", err, response)
	}

	for _, r := range results {
		existing[r.EnemyID] = r
	}

	if err := writeJSON(strategiesPath, existing); err != nil {
		return fmt.Errorf("write enemy strategies: %w", err)
	}

	fmt.Printf("Enemy analysis complete: %d enemies\n", len(existing))
	return nil
}
