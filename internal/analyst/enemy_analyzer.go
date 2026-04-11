package analyst

import (
	"context"
	"fmt"
)

// EnemyAnalysis is the LLM-generated strategy guide for an enemy.
type EnemyAnalysis struct {
	EnemyID       string   `json:"enemy_id"`
	Name          string   `json:"name"`
	FloorRange    string   `json:"floor_range"`
	Type          string   `json:"type"` // normal, elite, boss
	ThreatLevel   string   `json:"threat_level"` // low, medium, high, critical
	AttackPattern string   `json:"attack_pattern"`
	Strategy      string   `json:"strategy"`
	CounterCards  []string `json:"counter_cards"`
	DangerCards   []string `json:"danger_cards"` // cards that are bad against this enemy
	Notes         string   `json:"notes"`
}

// AnalyzeEnemies generates enemy strategy guides using LLM knowledge.
func (a *Analyst) AnalyzeEnemies(ctx context.Context) error {
	// For now, use LLM's built-in game knowledge to generate enemy guides
	// In the future, this will also incorporate data from historical runs
	fmt.Println("Enemy analysis: using LLM game knowledge")
	fmt.Println("(Will be enhanced with historical run data once enough runs are collected)")

	// TODO: implement enemy analysis
	// 1. Collect enemy IDs from historical runs (events.jsonl)
	// 2. Send to LLM for analysis
	// 3. Save to data/knowledge/enemies/strategies.json

	return nil
}
