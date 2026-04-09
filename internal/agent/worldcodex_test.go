package agentruntime

import (
	"testing"

	"spire2mind/internal/game"
	"spire2mind/internal/i18n"
)

func TestSeenContentTrackerObserveAndMerge(t *testing.T) {
	t.Parallel()

	tracker := NewSeenContentTracker()
	state := &game.StateSnapshot{
		RunID:  "RUN-1",
		Screen: "COMBAT",
		Run: map[string]any{
			"floor": 3,
		},
		Combat: map[string]any{
			"hand": []any{
				map[string]any{"index": 0, "cardId": "strike", "name": "Strike"},
			},
			"enemies": []any{
				map[string]any{"index": 0, "enemyId": "slime_small", "name": "Small Slime"},
			},
		},
		Reward: map[string]any{
			"cardOptions": []any{
				map[string]any{"index": 0, "cardId": "shrug_it_off", "name": "Shrug It Off"},
			},
		},
		Event: map[string]any{
			"eventId": "golden_shrine",
			"title":   "Golden Shrine",
		},
		Shop: map[string]any{
			"relics": []any{
				map[string]any{"index": 0, "relicId": "burning_blood", "name": "Burning Blood"},
			},
			"potions": []any{
				map[string]any{"index": 0, "potionId": "fire_potion", "name": "Fire Potion"},
			},
		},
	}

	discoveries := tracker.Observe(state)
	if len(discoveries) < 5 {
		t.Fatalf("expected multiple discoveries, got %#v", discoveries)
	}

	snapshot := tracker.Snapshot()
	if got := snapshot.Counts()[seenCategoryCards]; got != 2 {
		t.Fatalf("expected 2 cards, got %d", got)
	}
	if got := snapshot.Counts()[seenCategoryMonsters]; got != 1 {
		t.Fatalf("expected 1 monster, got %d", got)
	}
	if got := snapshot.Counts()[seenCategoryEvents]; got != 1 {
		t.Fatalf("expected 1 event, got %d", got)
	}

	merged := NewSeenContentTracker()
	merged.Merge(snapshot)
	merged.Merge(&SeenContentRegistry{
		Cards: []SeenContentEntry{
			{Category: seenCategoryCards, ID: "true_grit", Name: "True Grit", SeenCount: 1},
		},
	})

	mergedSnapshot := merged.Snapshot()
	if got := mergedSnapshot.Counts()[seenCategoryCards]; got != 3 {
		t.Fatalf("expected merged card count 3, got %d", got)
	}

	var shrug SeenContentEntry
	found := false
	for _, entry := range mergedSnapshot.Cards {
		if entry.ID == "shrug_it_off" {
			shrug = entry
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected shrug_it_off to be present after merge, got %#v", mergedSnapshot.Cards)
	}
	if shrug.SeenCount != 1 {
		t.Fatalf("expected merged guidebook-style seen count to represent run presence, got %d", shrug.SeenCount)
	}
}

func TestSeenContentTrackerDedupesRepeatedObservationContext(t *testing.T) {
	t.Parallel()

	tracker := NewSeenContentTracker()
	state := &game.StateSnapshot{
		RunID:  "RUN-1",
		Screen: "COMBAT",
		Run: map[string]any{
			"floor": 3,
		},
		Combat: map[string]any{
			"hand": []any{
				map[string]any{"index": 0, "cardId": "strike", "name": "Strike"},
			},
		},
	}

	tracker.Observe(state)
	tracker.Observe(state)

	snapshot := tracker.Snapshot()
	if len(snapshot.Cards) != 1 {
		t.Fatalf("expected exactly one tracked card, got %#v", snapshot.Cards)
	}
	if got := snapshot.Cards[0].SeenCount; got != 1 {
		t.Fatalf("expected repeated observation in same context to count once, got %d", got)
	}
}

func TestSeenContentTrackerCountsNewObservationContext(t *testing.T) {
	t.Parallel()

	tracker := NewSeenContentTracker()
	state := &game.StateSnapshot{
		RunID:  "RUN-1",
		Screen: "COMBAT",
		Run: map[string]any{
			"floor": 3,
		},
		Combat: map[string]any{
			"hand": []any{
				map[string]any{"index": 0, "cardId": "strike", "name": "Strike"},
			},
		},
	}

	tracker.Observe(state)

	nextFloor := &game.StateSnapshot{
		RunID:  "RUN-1",
		Screen: "COMBAT",
		Run: map[string]any{
			"floor": 4,
		},
		Combat: map[string]any{
			"hand": []any{
				map[string]any{"index": 0, "cardId": "strike", "name": "Strike"},
			},
		},
	}

	tracker.Observe(nextFloor)

	snapshot := tracker.Snapshot()
	if len(snapshot.Cards) != 1 {
		t.Fatalf("expected exactly one tracked card, got %#v", snapshot.Cards)
	}
	if got := snapshot.Cards[0].SeenCount; got != 2 {
		t.Fatalf("expected new floor context to increase seen_count, got %d", got)
	}
}

func TestSeenContentTrackerMergeCountsRunPresence(t *testing.T) {
	t.Parallel()

	first := NewSeenContentTracker()
	first.Merge(&SeenContentRegistry{
		Cards: []SeenContentEntry{{
			Category:  seenCategoryCards,
			ID:        "strike",
			Name:      "Strike",
			SeenCount: 12,
		}},
	})
	second := NewSeenContentTracker()
	second.Merge(&SeenContentRegistry{
		Cards: []SeenContentEntry{{
			Category:  seenCategoryCards,
			ID:        "strike",
			Name:      "Strike",
			SeenCount: 99,
		}},
	})

	first.Merge(second.Snapshot())

	snapshot := first.Snapshot()
	if len(snapshot.Cards) != 1 {
		t.Fatalf("expected one merged card entry, got %#v", snapshot.Cards)
	}
	if got := snapshot.Cards[0].SeenCount; got != 2 {
		t.Fatalf("expected merged seen_count to reflect two run presences, got %d", got)
	}
}

func TestSeenContentDiscoveryLines(t *testing.T) {
	t.Parallel()

	registry := &SeenContentRegistry{
		Cards: []SeenContentEntry{
			{Category: seenCategoryCards, ID: "strike", Name: "Strike"},
		},
		Monsters: []SeenContentEntry{
			{Category: seenCategoryMonsters, ID: "slime_small", Name: "Small Slime"},
		},
	}

	lines := seenContentDiscoveryLines(registry, i18n.LanguageEnglish, 4)
	if len(lines) != 2 {
		t.Fatalf("expected 2 discovery lines, got %#v", lines)
	}
}

func TestSummarizeSeenContentEntriesWithCatalogPrefersCanonicalEnglish(t *testing.T) {
	t.Parallel()

	catalog := &codexCatalog{
		Cards: map[string]codexCardMeta{
			"armaments": {ID: "armaments", Name: "Armaments"},
		},
	}
	entries := []SeenContentEntry{{
		Category: seenCategoryCards,
		ID:       "armaments",
		Name:     "姝﹁",
	}}

	lines := SummarizeSeenContentEntriesWithCatalog(entries, catalog, i18n.LanguageEnglish)
	if len(lines) != 1 {
		t.Fatalf("expected one summary line, got %#v", lines)
	}
	if got := lines[0]; got != "Card: Armaments" {
		t.Fatalf("expected canonical english summary, got %q", got)
	}
}
