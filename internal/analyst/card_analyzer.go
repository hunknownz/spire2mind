package analyst

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CardAnalysis is the LLM-generated deep analysis of a single card.
type CardAnalysis struct {
	CardID        string          `json:"card_id"`
	Name          string          `json:"name"`
	Type          string          `json:"type"`
	Rarity        string          `json:"rarity"`
	Cost          *int            `json:"cost"`
	Role          string          `json:"role"`          // attack, defense, utility, scaling, draw, exhaust
	Tier          string          `json:"tier"`          // S/A/B/C/D
	Timing        string          `json:"timing"`        // early, mid, late, any
	Synergies     FlexStringSlice `json:"synergies"`     // Card IDs that synergize well
	AntiSynergies FlexStringSlice `json:"anti_synergies"` // Card IDs that conflict
	Archetypes    FlexStringSlice `json:"archetypes"`    // strength, block, exhaust, draw, mixed
	Score         float64         `json:"score"`         // 1-10 pick priority
	Notes         string          `json:"notes"`         // Free-text strategic notes
	VsEnemyType   json.RawMessage `json:"vs_enemy_type"` // Flexible: map or string
}

// FlexStringSlice handles JSON that might be a string or []string.
type FlexStringSlice []string

func (f *FlexStringSlice) UnmarshalJSON(data []byte) error {
	// Try as array first
	var arr []string
	if err := json.Unmarshal(data, &arr); err == nil {
		*f = arr
		return nil
	}
	// Try as single string
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		if s == "" {
			*f = nil
		} else {
			*f = []string{s}
		}
		return nil
	}
	*f = nil
	return nil
}

// CardIndex is the raw card data from Bridge.
type CardIndex struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Type       string `json:"type"`
	Rarity     string `json:"rarity"`
	Cost       *int   `json:"cost"`
	TargetType string `json:"targetType"`
}

// AnalyzeCards fetches all cards from Bridge and generates deep analysis.
func (a *Analyst) AnalyzeCards(ctx context.Context) error {
	// Step 1: Fetch card data from Bridge
	cards, err := a.fetchCards(ctx)
	if err != nil {
		return fmt.Errorf("fetch cards: %w", err)
	}
	fmt.Printf("Fetched %d cards from Bridge\n", len(cards))

	// Save index
	indexPath := filepath.Join(a.knowledgeDir, "cards", "index.json")
	if err := writeJSON(indexPath, cards); err != nil {
		return fmt.Errorf("write card index: %w", err)
	}

	// Step 2: Load existing analysis (for incremental updates)
	analysisPath := filepath.Join(a.knowledgeDir, "cards", "analysis.json")
	existing := loadExistingAnalysis(analysisPath)
	fmt.Printf("Existing analysis: %d cards\n", len(existing))

	// Step 3: Analyze cards in batches
	batch := make([]CardIndex, 0, 10)
	analyzed := 0
	for _, card := range cards {
		if _, ok := existing[card.ID]; ok {
			continue // Already analyzed
		}
		batch = append(batch, card)
		if len(batch) >= 10 {
			results, err := a.analyzeCardBatch(ctx, batch)
			if err != nil {
				fmt.Printf("Warning: batch analysis failed: %v\n", err)
			} else {
				for _, r := range results {
					existing[r.CardID] = r
				}
				analyzed += len(results)
				fmt.Printf("Analyzed %d cards (total: %d/%d)\n", len(results), len(existing), len(cards))
			}
			batch = batch[:0]

			// Save progress after each batch
			if err := writeJSON(analysisPath, existing); err != nil {
				return fmt.Errorf("save progress: %w", err)
			}
		}
	}

	// Process remaining cards
	if len(batch) > 0 {
		results, err := a.analyzeCardBatch(ctx, batch)
		if err != nil {
			fmt.Printf("Warning: final batch analysis failed: %v\n", err)
		} else {
			for _, r := range results {
				existing[r.CardID] = r
			}
			analyzed += len(results)
		}
	}

	// Save final results
	if err := writeJSON(analysisPath, existing); err != nil {
		return fmt.Errorf("save analysis: %w", err)
	}

	fmt.Printf("Card analysis complete: %d new, %d total\n", analyzed, len(existing))
	return nil
}

func (a *Analyst) fetchCards(ctx context.Context) ([]CardIndex, error) {
	url := a.bridgeBaseURL + "/data/cards"
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bridge request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var envelope struct {
		OK   bool        `json:"ok"`
		Data []CardIndex `json:"data"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if !envelope.OK {
		return nil, fmt.Errorf("bridge returned error")
	}

	return envelope.Data, nil
}

func (a *Analyst) analyzeCardBatch(ctx context.Context, cards []CardIndex) ([]CardAnalysis, error) {
	systemPrompt := `你是杀戮尖塔 2（Slay the Spire 2）的资深策略分析师。
你需要对卡牌进行深度分析，评估其战略价值、协同关系和使用场景。
你的分析将被自动化系统用于游戏决策，所以请确保评分客观、分析具体。

重要规则：
- 只返回 JSON 数组，不要输出其他任何文本
- 每张牌的分析必须包含所有必要字段
- synergies 和 anti_synergies 用卡牌 ID（大写）
- score 是 1-10 的选牌优先级，10 最高
- archetypes 从这些选：strength, block, exhaust, draw, scaling, mixed`

	var cardDescs []string
	for _, card := range cards {
		costStr := "X"
		if card.Cost != nil {
			costStr = fmt.Sprintf("%d", *card.Cost)
		}
		cardDescs = append(cardDescs, fmt.Sprintf(
			"- ID: %s | 名称: %s | 类型: %s | 费用: %s | 稀有度: %s | 目标: %s",
			card.ID, card.Name, card.Type, costStr, card.Rarity, card.TargetType))
	}

	userPrompt := fmt.Sprintf(`请分析以下 %d 张卡牌。对每张牌给出：
1. role: 卡牌角色（attack/defense/utility/scaling/draw/exhaust）
2. tier: 评级（S/A/B/C/D）
3. timing: 最佳拿取时机（early/mid/late/any）
4. synergies: 协同卡牌 ID 列表（3-5 张）
5. anti_synergies: 冲突卡牌 ID 列表
6. archetypes: 适合的构筑方向
7. score: 选牌优先级（1-10）
8. notes: 核心策略说明（1-2 句话）
9. vs_enemy_type: 对不同敌人类型的评价（"normal"/"elite"/"boss" → 简短评价）

卡牌列表：
%s

返回 JSON 数组，每个元素包含 card_id, name, type, rarity, cost, role, tier, timing, synergies, anti_synergies, archetypes, score, notes, vs_enemy_type 字段。`, len(cards), strings.Join(cardDescs, "\n"))

	response, err := a.llm.Complete(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	// Extract JSON from response (handle markdown code blocks)
	jsonStr := extractJSON(response)

	var results []CardAnalysis
	if err := json.Unmarshal([]byte(jsonStr), &results); err != nil {
		return nil, fmt.Errorf("parse LLM response: %w (response: %.200s)", err, response)
	}

	return results, nil
}

func loadExistingAnalysis(path string) map[string]CardAnalysis {
	data, err := os.ReadFile(path)
	if err != nil {
		return make(map[string]CardAnalysis)
	}

	var existing map[string]CardAnalysis
	if err := json.Unmarshal(data, &existing); err != nil {
		return make(map[string]CardAnalysis)
	}

	return existing
}

func writeJSON(path string, data interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, bytes, 0o644)
}

func extractJSON(response string) string {
	// Try to find JSON array or object in the response
	response = strings.TrimSpace(response)

	// Remove markdown code block wrapper
	if strings.HasPrefix(response, "```") {
		lines := strings.Split(response, "\n")
		start := 1
		end := len(lines) - 1
		if end > start && strings.HasPrefix(lines[end], "```") {
			response = strings.Join(lines[start:end], "\n")
		}
	}

	response = strings.TrimSpace(response)

	// Find first [ or {
	startIdx := strings.IndexAny(response, "[{")
	if startIdx < 0 {
		return response
	}

	return response[startIdx:]
}
