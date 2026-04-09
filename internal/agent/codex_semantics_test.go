package agentruntime

import "testing"

func TestNormalizeSeenContentDisplayUsesCanonicalEnglishFallback(t *testing.T) {
	t.Parallel()

	entry := SeenContentEntry{
		Category: seenCategoryCards,
		ID:       "armaments",
		Name:     "姝﹁",
	}
	catalog := &codexCatalog{
		Cards: map[string]codexCardMeta{
			"armaments": {ID: "armaments", Name: "Armaments"},
		},
	}

	normalizeSeenContentDisplay(&entry, catalog)

	if entry.RawName != "姝﹁" {
		t.Fatalf("expected raw name to preserve original observation, got %#v", entry)
	}
	if entry.NameEN != "Armaments" {
		t.Fatalf("expected canonical english fallback, got %#v", entry)
	}
	if entry.Name != "Armaments" {
		t.Fatalf("expected user-facing name to become readable, got %#v", entry)
	}
}

func TestEnrichSeenContentRegistryAddsEntityLevelSemantics(t *testing.T) {
	t.Parallel()

	registry := &SeenContentRegistry{
		Cards: []SeenContentEntry{{
			Category: seenCategoryCards,
			ID:       "defend_ironclad",
			Name:     "Defend",
		}},
		Monsters: []SeenContentEntry{{
			Category: seenCategoryMonsters,
			ID:       "jaw_worm",
			Name:     "Jaw Worm",
		}},
		Events: []SeenContentEntry{{
			Category: seenCategoryEvents,
			ID:       "golden_shrine",
			Name:     "Golden Shrine",
		}},
	}
	catalog := &codexCatalog{
		Cards: map[string]codexCardMeta{
			"defend_ironclad": {
				ID:    "defend_ironclad",
				Name:  "Defend",
				Block: testIntPtr(5),
				Type:  "Skill",
			},
		},
		Monsters: map[string]codexMonsterMeta{
			"jaw_worm": {
				ID:   "jaw_worm",
				Name: "Jaw Worm",
				Type: "normal",
				DamageValues: map[string]map[string]int{
					"CHOMP": {"damage": 12},
				},
			},
		},
		Events: map[string]codexEventMeta{
			"golden_shrine": {
				ID:          "golden_shrine",
				Name:        "Golden Shrine",
				Description: "Lose HP to gain gold or take a safer line.",
				Options: []codexEventOption{
					{ID: "pray", Description: "Lose HP, gain gold."},
				},
			},
		},
	}

	enrichSeenContentRegistry(registry, catalog, ReflectionLessonBuckets{}, []string{"critical hp spiral after greedy route"}, newCodexEvidenceIndex())

	if !hasTag(registry.Cards[0].RiskTags, "survival_tool") {
		t.Fatalf("expected defend to be tagged as survival tool, got %#v", registry.Cards[0])
	}
	if !hasTag(registry.Monsters[0].RiskTags, "burst_threat") {
		t.Fatalf("expected jaw worm to be tagged as burst threat, got %#v", registry.Monsters[0])
	}
	if !hasTag(registry.Events[0].RiskTags, "resource_tradeoff") || !hasTag(registry.Events[0].RiskTags, "hp_tradeoff") {
		t.Fatalf("expected golden shrine to carry tradeoff tags, got %#v", registry.Events[0])
	}
}

func TestApplyCodexPriorsBoostsSurvivalAndThreatBiases(t *testing.T) {
	t.Parallel()

	snapshot := CombatSnapshot{
		Player: CombatPlayerState{
			CurrentHP: 10,
			MaxHP:     30,
			Block:     0,
			Energy:    1,
		},
		Hand: []CombatCardState{{
			Index:      0,
			CardID:     "defend_ironclad",
			Name:       "Defend",
			EnergyCost: 1,
			Playable:   true,
		}},
		Enemies: []CombatEnemyState{{
			Index:     0,
			EnemyID:   "jaw_worm",
			Name:      "Jaw Worm",
			CurrentHP: 40,
			Hittable:  true,
		}},
		IncomingDamage: 12,
	}
	codex := &SeenContentRegistry{
		Cards: []SeenContentEntry{{
			Category: seenCategoryCards,
			ID:       "defend_ironclad",
			Name:     "Defend",
			RiskTags: []string{"survival_tool"},
		}},
		Monsters: []SeenContentEntry{{
			Category: seenCategoryMonsters,
			ID:       "jaw_worm",
			Name:     "Jaw Worm",
			RiskTags: []string{"combat_threat", "burst_threat", "punishes_low_hp"},
		}},
	}

	applyCodexPriors(&snapshot, codex)

	if snapshot.Hand[0].KnowledgePrior <= 1.0 {
		t.Fatalf("expected strong low-HP survival boost, got %#v", snapshot.Hand[0])
	}
	if snapshot.Enemies[0].KnowledgePrior <= 1.0 {
		t.Fatalf("expected threat bias for burst enemy, got %#v", snapshot.Enemies[0])
	}
	if len(snapshot.KnowledgeBiases) == 0 {
		t.Fatalf("expected human-readable knowledge cues, got %#v", snapshot)
	}
}

func testIntPtr(value int) *int {
	return &value
}
