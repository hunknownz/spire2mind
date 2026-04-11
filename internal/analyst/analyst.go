// Package analyst provides offline deep analysis of game data using LLMs.
// It generates structured knowledge files that the real-time agent can
// reference during gameplay for card evaluation, enemy strategies,
// deck building, and strategic planning.
//
// Usage:
//
//	spire2mind analyze cards        # Analyze all cards from Bridge ModelDb
//	spire2mind analyze enemies      # Generate enemy strategy guide
//	spire2mind analyze archetypes   # Analyze deck building directions
//	spire2mind analyze synergies    # Analyze card combination synergies
//	spire2mind analyze run [runID]  # Deep review a completed run
//	spire2mind analyze all          # Run all analyses
package analyst

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"spire2mind/internal/config"
)

// Analyst orchestrates offline game knowledge analysis.
type Analyst struct {
	cfg           config.Config
	knowledgeDir  string
	bridgeBaseURL string
	llm           LLMProvider
}

// LLMProvider abstracts the LLM backend for analysis.
type LLMProvider interface {
	// Complete sends a prompt and returns the LLM response.
	Complete(ctx context.Context, systemPrompt string, userPrompt string) (string, error)
}

// New creates an Analyst with the given configuration.
func New(cfg config.Config) (*Analyst, error) {
	knowledgeDir := filepath.Join(cfg.RepoRoot, "data", "knowledge")
	if err := os.MkdirAll(knowledgeDir, 0o755); err != nil {
		return nil, fmt.Errorf("create knowledge dir: %w", err)
	}

	bridgeURL := cfg.BridgeURL
	if bridgeURL == "" {
		bridgeURL = "http://127.0.0.1:8080"
	}

	llm, err := newLLMProvider(cfg)
	if err != nil {
		return nil, fmt.Errorf("init LLM provider: %w", err)
	}

	return &Analyst{
		cfg:           cfg,
		knowledgeDir:  knowledgeDir,
		bridgeBaseURL: bridgeURL,
		llm:           llm,
	}, nil
}

// RunAll executes all analysis tasks.
func (a *Analyst) RunAll(ctx context.Context) error {
	fmt.Println("=== Analyzing cards ===")
	if err := a.AnalyzeCards(ctx); err != nil {
		return fmt.Errorf("analyze cards: %w", err)
	}

	fmt.Println("=== Analyzing enemies ===")
	if err := a.AnalyzeEnemies(ctx); err != nil {
		return fmt.Errorf("analyze enemies: %w", err)
	}

	fmt.Println("=== Analyzing archetypes ===")
	if err := a.AnalyzeArchetypes(ctx); err != nil {
		return fmt.Errorf("analyze archetypes: %w", err)
	}

	fmt.Println("=== All analysis complete ===")
	return nil
}
