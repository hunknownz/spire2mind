package kbharvest

import (
	"strings"
	"testing"
)

func TestCleanWikiTextRemovesTemplatesAndKeepsReadableText(t *testing.T) {
	raw := "{{Card Infobox|Sovereign Blade||2}}\n'''Sovereign Blade''' is a [[Card]] with [[Forge|Forge keyword]].\n== Sources ==\n* [[Refine Blade]]"
	cleaned := cleanWikiText(raw)

	for _, want := range []string{"Sovereign Blade is a Card with Forge keyword.", "Sources", "- Refine Blade"} {
		if !strings.Contains(cleaned, want) {
			t.Fatalf("expected cleaned text to contain %q, got %q", want, cleaned)
		}
	}
	if strings.Contains(cleaned, "Card Infobox") {
		t.Fatalf("expected template content to be removed, got %q", cleaned)
	}
}

func TestStripNestedDelimitedHandlesUnbalancedInput(t *testing.T) {
	got := stripNestedDelimited("alpha {{beta {{gamma}} delta", "{{", "}}")
	if strings.TrimSpace(got) != "alpha" {
		t.Fatalf("expected unbalanced template stripping to keep outer text, got %q", got)
	}
}
