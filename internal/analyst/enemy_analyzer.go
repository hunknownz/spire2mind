package analyst

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EnemyAnalysis is the LLM-generated strategy guide for an enemy.
type EnemyAnalysis struct {
	EnemyID       string          `json:"enemy_id"`
	Name          string          `json:"name"`
	FloorRange    string          `json:"floor_range"`
	Type          string          `json:"type"` // normal, elite, boss
	ThreatLevel   string          `json:"threat_level"`
	AttackPattern string          `json:"attack_pattern"`
	Strategy      string          `json:"strategy"`
	CounterCards  FlexStringSlice `json:"counter_cards"`
	DangerCards   FlexStringSlice `json:"danger_cards"`
	Notes         string          `json:"notes"`
}

// enemyEntry is a known STS2 enemy with basic metadata for the LLM prompt.
type enemyEntry struct {
	ID         string
	Name       string
	FloorRange string
	Type       string // normal, elite, boss
}

// knownEnemies is the comprehensive list of STS2 enemies.
// LLM will fill in attack patterns, strategies, etc.
var knownEnemies = []enemyEntry{
	// Act 1 normals
	{ID: "CULTIST", Name: "Cultist", FloorRange: "1-6", Type: "normal"},
	{ID: "JAW_WORM", Name: "Jaw Worm", FloorRange: "1-6", Type: "normal"},
	{ID: "SMALL_SLIME", Name: "Small Slime", FloorRange: "1-6", Type: "normal"},
	{ID: "ACID_SLIME_L", Name: "Acid Slime (L)", FloorRange: "1-6", Type: "normal"},
	{ID: "ACID_SLIME_M", Name: "Acid Slime (M)", FloorRange: "1-6", Type: "normal"},
	{ID: "SPIKE_SLIME_L", Name: "Spike Slime (L)", FloorRange: "1-6", Type: "normal"},
	{ID: "SPIKE_SLIME_M", Name: "Spike Slime (M)", FloorRange: "1-6", Type: "normal"},
	{ID: "FUNGI_BEAST", Name: "Fungi Beast", FloorRange: "1-6", Type: "normal"},
	{ID: "LOOTER", Name: "Looter", FloorRange: "1-6", Type: "normal"},
	{ID: "MUD_GREMLIN", Name: "Mud Gremlin", FloorRange: "1-6", Type: "normal"},
	{ID: "SNEAKY_GREMLIN", Name: "Sneaky Gremlin", FloorRange: "1-6", Type: "normal"},
	{ID: "FAT_GREMLIN", Name: "Fat Gremlin", FloorRange: "1-6", Type: "normal"},
	{ID: "GREMLIN_WIZARD", Name: "Gremlin Wizard", FloorRange: "1-6", Type: "normal"},
	{ID: "SHIELD_GREMLIN", Name: "Shield Gremlin", FloorRange: "1-6", Type: "normal"},
	{ID: "LOUSE_NORMAL", Name: "Louse (Normal)", FloorRange: "1-6", Type: "normal"},
	{ID: "LOUSE_SHIELDED", Name: "Louse (Shielded)", FloorRange: "1-6", Type: "normal"},
	// Act 1 elites
	{ID: "LAGAVULIN", Name: "Lagavulin", FloorRange: "6-9", Type: "elite"},
	{ID: "GREMLIN_NOB", Name: "Gremlin Nob", FloorRange: "6-9", Type: "elite"},
	{ID: "SENTRIES", Name: "Sentries", FloorRange: "6-9", Type: "elite"},
	// Act 1 boss
	{ID: "SLIME_BOSS", Name: "Slime Boss", FloorRange: "9-10", Type: "boss"},
	{ID: "HEXAGHOST", Name: "Hexaghost", FloorRange: "9-10", Type: "boss"},
	{ID: "THE_GUARDIAN", Name: "The Guardian", FloorRange: "9-10", Type: "boss"},
	// Act 2 normals
	{ID: "BLUE_SLAVER", Name: "Blue Slaver", FloorRange: "11-16", Type: "normal"},
	{ID: "RED_SLAVER", Name: "Red Slaver", FloorRange: "11-16", Type: "normal"},
	{ID: "CHOSEN", Name: "Chosen", FloorRange: "11-16", Type: "normal"},
	{ID: "SHELL_PARASITE", Name: "Shell Parasite", FloorRange: "11-16", Type: "normal"},
	{ID: "SNAKE_PLANT", Name: "Snake Plant", FloorRange: "11-16", Type: "normal"},
	{ID: "CENTURION", Name: "Centurion", FloorRange: "11-16", Type: "normal"},
	{ID: "MYSTIC", Name: "Mystic", FloorRange: "11-16", Type: "normal"},
	{ID: "SPHERIC_GUARDIAN", Name: "Spheric Guardian", FloorRange: "11-16", Type: "normal"},
	{ID: "BYRD", Name: "Byrd", FloorRange: "11-16", Type: "normal"},
	{ID: "REPULSOR", Name: "Repulsor", FloorRange: "11-16", Type: "normal"},
	{ID: "SPIRE_GROWTH", Name: "Spire Growth", FloorRange: "11-16", Type: "normal"},
	{ID: "TRANSIENT", Name: "Transient", FloorRange: "11-16", Type: "normal"},
	// Act 2 elites
	{ID: "GREMLIN_LEADER", Name: "Gremlin Leader", FloorRange: "14-18", Type: "elite"},
	{ID: "TASKMASTER", Name: "Taskmaster", FloorRange: "14-18", Type: "elite"},
	{ID: "BOOK_OF_STABBING", Name: "Book of Stabbing", FloorRange: "14-18", Type: "elite"},
	// Act 2 boss
	{ID: "THE_COLLECTOR", Name: "The Collector", FloorRange: "17-18", Type: "boss"},
	{ID: "THE_CHAMP", Name: "The Champ", FloorRange: "17-18", Type: "boss"},
	{ID: "AUTOMATON", Name: "Bronze Automaton", FloorRange: "17-18", Type: "boss"},
	// Act 3 normals
	{ID: "DECA", Name: "Deca", FloorRange: "20-26", Type: "normal"},
	{ID: "DONU", Name: "Donu", FloorRange: "20-26", Type: "normal"},
	{ID: "EXPLODER", Name: "Exploder", FloorRange: "20-26", Type: "normal"},
	{ID: "SPIKER", Name: "Spiker", FloorRange: "20-26", Type: "normal"},
	{ID: "ORB_WALKER", Name: "Orb Walker", FloorRange: "20-26", Type: "normal"},
	{ID: "JAW_WORM_HORDE", Name: "Jaw Worm Horde", FloorRange: "20-26", Type: "normal"},
	// Act 3 elites
	{ID: "GIANT_HEAD", Name: "Giant Head", FloorRange: "24-28", Type: "elite"},
	{ID: "NEMESIS", Name: "Nemesis", FloorRange: "24-28", Type: "elite"},
	{ID: "REPTOMANCER", Name: "Reptomancer", FloorRange: "24-28", Type: "elite"},
	// Act 3 boss
	{ID: "TIME_EATER", Name: "Time Eater", FloorRange: "27-28", Type: "boss"},
	{ID: "AWAKENED_ONE", Name: "Awakened One", FloorRange: "27-28", Type: "boss"},
	{ID: "DONU_AND_DECA", Name: "Donu and Deca", FloorRange: "27-28", Type: "boss"},
	// Act 4 / heart
	{ID: "SPIRE_SHIELD", Name: "Spire Shield", FloorRange: "30", Type: "elite"},
	{ID: "SPIRE_SPEAR", Name: "Spire Spear", FloorRange: "30", Type: "elite"},
	{ID: "CORRUPT_HEART", Name: "Corrupt Heart", FloorRange: "30", Type: "boss"},
}

// AnalyzeEnemies generates enemy strategy guides using LLM knowledge.
func (a *Analyst) AnalyzeEnemies(ctx context.Context) error {
	strategiesPath := filepath.Join(a.knowledgeDir, "enemies", "strategies.json")

	existing := loadExistingEnemies(strategiesPath)
	fmt.Printf("Existing enemy strategies: %d entries\n", len(existing))

	// Build list of enemies still needing analysis
	var todo []enemyEntry
	for _, e := range knownEnemies {
		if _, ok := existing[e.ID]; !ok {
			todo = append(todo, e)
		}
	}

	if len(todo) == 0 {
		fmt.Printf("All %d enemies already analyzed\n", len(existing))
		return nil
	}

	fmt.Printf("Analyzing %d enemies in batches of 5...\n", len(todo))

	const batchSize = 5
	for i := 0; i < len(todo); i += batchSize {
		end := i + batchSize
		if end > len(todo) {
			end = len(todo)
		}
		batch := todo[i:end]

		results, err := a.analyzeEnemyBatch(ctx, batch)
		if err != nil {
			fmt.Printf("Warning: batch %d-%d failed: %v\n", i, end, err)
			continue
		}

		for _, r := range results {
			existing[r.EnemyID] = r
		}
		fmt.Printf("Analyzed enemies %d-%d (total: %d/%d)\n", i+1, end, len(existing), len(knownEnemies))

		if err := writeJSON(strategiesPath, existing); err != nil {
			return fmt.Errorf("save progress: %w", err)
		}
	}

	fmt.Printf("Enemy analysis complete: %d enemies\n", len(existing))
	return nil
}

func (a *Analyst) analyzeEnemyBatch(ctx context.Context, enemies []enemyEntry) ([]EnemyAnalysis, error) {
	systemPrompt := `你是杀戮尖塔 2（Slay the Spire 2）的资深策略分析师，专注于铁甲战士（Ironclad）视角的敌人分析。
只返回 JSON 数组，不要输出其他任何文本。`

	var lines []string
	for _, e := range enemies {
		lines = append(lines, fmt.Sprintf("- ID: %s | 名称: %s | 楼层: %s | 类型: %s",
			e.ID, e.Name, e.FloorRange, e.Type))
	}

	userPrompt := fmt.Sprintf(`请为以下 %d 个杀戮尖塔 2 中的敌人生成策略指南（铁甲战士视角）：

%s

对每个敌人给出：
1. enemy_id: 大写 ID（与输入一致）
2. name: 中文名
3. floor_range: 出现楼层范围
4. type: normal/elite/boss
5. threat_level: low/medium/high/critical
6. attack_pattern: 攻击模式描述（意图循环、技能、debuff 等）
7. strategy: 应对策略（优先处理顺序、应对节奏等）
8. counter_cards: 克制该敌人的卡牌 ID 列表（用铁甲战士卡牌 ID，如 BASH, SHRUG_IT_OFF）
9. danger_cards: 对付这个敌人时需要特别小心使用的牌（或会被 debuff 影响的牌）
10. notes: 关键注意事项（1-2 句话）

返回 JSON 数组，字段：enemy_id, name, floor_range, type, threat_level, attack_pattern, strategy, counter_cards, danger_cards, notes。`,
		len(enemies), strings.Join(lines, "\n"))

	response, err := a.llm.Complete(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	jsonStr := extractJSON(response)
	var results []EnemyAnalysis
	if err := json.Unmarshal([]byte(jsonStr), &results); err != nil {
		return nil, fmt.Errorf("parse enemy analysis JSON: %w (response: %.300s)", err, response)
	}

	return results, nil
}

func loadExistingEnemies(path string) map[string]EnemyAnalysis {
	data, err := os.ReadFile(path)
	if err != nil {
		return make(map[string]EnemyAnalysis)
	}
	var existing map[string]EnemyAnalysis
	if err := json.Unmarshal(data, &existing); err != nil {
		return make(map[string]EnemyAnalysis)
	}
	return existing
}
