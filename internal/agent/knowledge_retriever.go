package agentruntime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"spire2mind/internal/analyst"
	"spire2mind/internal/game"
	"spire2mind/internal/i18n"
)

// KnowledgeRetriever loads pre-computed knowledge from data/knowledge/
// and provides context-specific information for prompt injection.
type KnowledgeRetriever struct {
	knowledgeDir string
	cards        map[string]analyst.CardAnalysis
	enemies      map[string]analyst.EnemyAnalysis
	loaded       bool
}

// NewKnowledgeRetriever creates a retriever that reads from data/knowledge/.
func NewKnowledgeRetriever(repoRoot string) *KnowledgeRetriever {
	return &KnowledgeRetriever{
		knowledgeDir: filepath.Join(repoRoot, "data", "knowledge"),
	}
}

// EnsureLoaded loads knowledge files if not already loaded.
func (kr *KnowledgeRetriever) EnsureLoaded() {
	if kr.loaded {
		return
	}
	kr.loaded = true
	kr.cards = make(map[string]analyst.CardAnalysis)
	kr.enemies = make(map[string]analyst.EnemyAnalysis)

	analysisPath := filepath.Join(kr.knowledgeDir, "cards", "analysis.json")
	if data, err := os.ReadFile(analysisPath); err == nil {
		json.Unmarshal(data, &kr.cards)
	}

	enemiesPath := filepath.Join(kr.knowledgeDir, "enemies", "strategies.json")
	if data, err := os.ReadFile(enemiesPath); err == nil {
		json.Unmarshal(data, &kr.enemies)
	}
}

// ForCardSelection generates knowledge context for card pick decisions.
func (kr *KnowledgeRetriever) ForCardSelection(state *game.StateSnapshot, loc i18n.Language) string {
	kr.EnsureLoaded()
	if state == nil || len(kr.cards) == 0 {
		return ""
	}

	var lines []string

	// Analyze current deck composition
	deckAnalysis := kr.analyzeDeck(state)
	if deckAnalysis != "" {
		lines = append(lines, deckAnalysis)
	}

	// Evaluate candidate cards
	var candidates []game.CardState
	if state.Reward != nil {
		candidates = state.Reward.CardOptions
	}
	if state.Selection != nil {
		candidates = state.Selection.Cards
	}

	if len(candidates) > 0 {
		lines = append(lines, "")
		lines = append(lines, i18n.New(loc).Label("Candidate card analysis:", "候选卡牌分析:"))
		for _, card := range candidates {
			analysis, ok := kr.cards[strings.ToUpper(card.CardID)]
			if !ok {
				continue
			}
			synStr := ""
			if len(analysis.Synergies) > 0 {
				synStr = fmt.Sprintf(" 协同: %s", strings.Join(analysis.Synergies[:min(3, len(analysis.Synergies))], ", "))
			}
			lines = append(lines, fmt.Sprintf("[%d] %s — %s | 评分 %.1f | %s%s",
				card.Index, card.Name, analysis.Role, analysis.Score, analysis.Notes, synStr))
		}
	}

	return strings.Join(lines, "\n")
}

// ForCombat generates knowledge context for combat decisions.
func (kr *KnowledgeRetriever) ForCombat(state *game.StateSnapshot, loc i18n.Language) string {
	kr.EnsureLoaded()
	if state == nil || state.Combat == nil {
		return ""
	}

	var lines []string

	// Enemy information
	for _, enemy := range state.Combat.Enemies {
		lines = append(lines, fmt.Sprintf("敌人 %s: HP %d/%d", enemy.Name, enemy.CurrentHp, enemy.MaxHp))
		for _, intent := range enemy.Intents {
			if intent.TotalDamage != nil && *intent.TotalDamage > 0 {
				lines = append(lines, fmt.Sprintf("  意图: %s (%d 伤害)", intent.IntentType, *intent.TotalDamage))
			} else {
				lines = append(lines, fmt.Sprintf("  意图: %s", intent.IntentType))
			}
		}
		// Inject strategy from knowledge base
		strategy := kr.enemyStrategy(&enemy)
		if strategy != "" {
			lines = append(lines, fmt.Sprintf("  策略: %s", strategy))
		}
	}

	// Player powers context
	if len(state.Combat.Player.Powers) > 0 {
		lines = append(lines, "")
		var powerDescs []string
		for _, p := range state.Combat.Player.Powers {
			powerDescs = append(powerDescs, fmt.Sprintf("%s x%d", p.Name, p.Amount))
		}
		lines = append(lines, fmt.Sprintf("你的增益: %s", strings.Join(powerDescs, ", ")))
	}

	// Hand card analysis
	if len(state.Combat.Hand) > 0 {
		lines = append(lines, "")
		lines = append(lines, "手牌分析:")
		for _, card := range state.Combat.Hand {
			analysis, ok := kr.cards[strings.ToUpper(card.CardID)]
			if ok {
				lines = append(lines, fmt.Sprintf("  [%d] %s — %s", card.Index, card.Name, analysis.Notes))
			}
		}
	}

	return strings.Join(lines, "\n")
}

// enemyStrategy returns the strategy string for an enemy from the knowledge base.
// It tries matching by ID and by name (normalized).
func (kr *KnowledgeRetriever) enemyStrategy(enemy *game.EnemyState) string {
	if enemy == nil || len(kr.enemies) == 0 {
		return ""
	}

	// Try by ID (exact, then upper-cased)
	if e, ok := kr.enemies[enemy.ID]; ok {
		return e.Strategy
	}
	if e, ok := kr.enemies[strings.ToUpper(enemy.ID)]; ok {
		return e.Strategy
	}

	// Try by normalizing the name to ID-like form
	normalized := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(enemy.Name), " ", "_"))
	if e, ok := kr.enemies[normalized]; ok {
		return e.Strategy
	}

	// Partial name match
	lowerName := strings.ToLower(enemy.Name)
	for id, e := range kr.enemies {
		if strings.Contains(strings.ToLower(id), lowerName) || strings.Contains(strings.ToLower(e.Name), lowerName) {
			return e.Strategy
		}
	}

	return ""
}

// analyzeDeck analyzes the current deck composition.
func (kr *KnowledgeRetriever) analyzeDeck(state *game.StateSnapshot) string {
	if state.Run == nil || len(state.Run.Deck) == 0 {
		return ""
	}

	var attacks, skills, powers int
	archetypeCounts := make(map[string]int)

	for _, card := range state.Run.Deck {
		analysis, ok := kr.cards[strings.ToUpper(card.CardID)]
		if !ok {
			continue
		}
		switch analysis.Role {
		case "attack":
			attacks++
		case "defense":
			skills++
		case "scaling", "utility":
			powers++
		}
		for _, arch := range analysis.Archetypes {
			archetypeCounts[arch]++
		}
	}

	// Determine dominant archetype
	bestArch := ""
	bestCount := 0
	for arch, count := range archetypeCounts {
		if count > bestCount {
			bestArch = arch
			bestCount = count
		}
	}

	deckSize := len(state.Run.Deck)
	archLabel := bestArch
	if archLabel == "" {
		archLabel = "混合"
	}

	return fmt.Sprintf(
		"当前牌组分析: %d 张 | 攻击 %d / 技能 %d / 能力 %d | 构筑方向: %s (%d 张相关牌)",
		deckSize, attacks, skills, powers, archLabel, bestCount)
}
