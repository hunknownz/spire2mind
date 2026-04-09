package agentruntime

import (
	"fmt"
	"strings"
)

type codexEntityEvidence struct {
	SeenRuns       int
	OutcomeCounts  map[string]int
	FailureSignals map[string]int
	FailureLinks   []string
	MinFloor       *int
	MaxFloor       *int
}

type codexEvidenceIndex struct {
	categories map[string]map[string]*codexEntityEvidence
}

func newCodexEvidenceIndex() *codexEvidenceIndex {
	return &codexEvidenceIndex{
		categories: make(map[string]map[string]*codexEntityEvidence),
	}
}

func (i *codexEvidenceIndex) absorbRun(registry *SeenContentRegistry, reflections []*AttemptReflection) {
	if i == nil || registry == nil {
		return
	}

	runEvidence := summarizeRunEvidence(reflections)
	for _, item := range iterSeenContentEntries(registry) {
		entry := i.ensure(item.Category, item.ID, item.Name)
		entry.SeenRuns++
		mergeOutcomeCounts(entry.OutcomeCounts, runEvidence.OutcomeCounts)
		mergeSignalCounts(entry.FailureSignals, runEvidence.FailureSignals)
		entry.FailureLinks = dedupeGuideLines(append(entry.FailureLinks, runEvidence.FailureLinks...), 8)
		updateOptionalFloorRange(entry, item.FirstFloor)
		updateOptionalFloorRange(entry, item.LastFloor)
		if runEvidence.MinFloor != nil {
			updateOptionalFloorRange(entry, runEvidence.MinFloor)
		}
		if runEvidence.MaxFloor != nil {
			updateOptionalFloorRange(entry, runEvidence.MaxFloor)
		}
	}
}

func (i *codexEvidenceIndex) lookup(category string, id string, name string) *codexEntityEvidence {
	if i == nil {
		return nil
	}
	id = normalizeSeenContentID(id, name)
	if id == "" {
		return nil
	}
	categoryEntries := i.categories[strings.TrimSpace(category)]
	if categoryEntries == nil {
		return nil
	}
	return categoryEntries[id]
}

func (i *codexEvidenceIndex) ensure(category string, id string, name string) *codexEntityEvidence {
	id = normalizeSeenContentID(id, name)
	if id == "" {
		return nil
	}

	category = strings.TrimSpace(category)
	categoryEntries := i.categories[category]
	if categoryEntries == nil {
		categoryEntries = make(map[string]*codexEntityEvidence)
		i.categories[category] = categoryEntries
	}
	if existing, ok := categoryEntries[id]; ok {
		return existing
	}

	created := &codexEntityEvidence{
		OutcomeCounts:  make(map[string]int),
		FailureSignals: make(map[string]int),
	}
	categoryEntries[id] = created
	return created
}

type runEvidenceSummary struct {
	OutcomeCounts  map[string]int
	FailureSignals map[string]int
	FailureLinks   []string
	MinFloor       *int
	MaxFloor       *int
}

func summarizeRunEvidence(reflections []*AttemptReflection) runEvidenceSummary {
	summary := runEvidenceSummary{
		OutcomeCounts:  make(map[string]int),
		FailureSignals: make(map[string]int),
	}

	for _, reflection := range reflections {
		if reflection == nil {
			continue
		}

		outcome := strings.TrimSpace(reflection.Outcome)
		if outcome != "" {
			summary.OutcomeCounts[outcome]++
		}
		if reflection.Floor != nil {
			updateOptionalFloorBounds(&summary.MinFloor, &summary.MaxFloor, reflection.Floor)
		}

		lines, signals := reflectionFailureEvidence(reflection)
		summary.FailureLinks = dedupeGuideLines(append(summary.FailureLinks, lines...), 8)
		mergeSignalCounts(summary.FailureSignals, signals)
	}

	return summary
}

func reflectionFailureEvidence(reflection *AttemptReflection) ([]string, map[string]int) {
	if reflection == nil {
		return nil, nil
	}

	signals := make(map[string]int)
	lines := make([]string, 0, 8)

	if strings.EqualFold(strings.TrimSpace(reflection.Outcome), "defeat") {
		if reflection.Headline != "" {
			lines = append(lines, cleanVisibleText(reflection.Headline))
		}
		lines = append(lines, cleanVisibleTextSlice(reflection.Risks)...)
		lines = append(lines, cleanVisibleTextSlice(reflection.RecentFailures)...)
	}

	textParts := []string{
		strings.TrimSpace(reflection.Headline),
		strings.Join(reflection.Risks, " "),
		strings.Join(reflection.RecentFailures, " "),
		strings.Join(reflection.Lessons, " "),
		strings.TrimSpace(reflection.NextPlan),
	}
	combined := strings.ToLower(strings.Join(textParts, " "))

	if reflection.Floor != nil && *reflection.Floor <= 6 && strings.EqualFold(reflection.Outcome, "defeat") {
		signals["early_death"]++
		lines = append(lines, fmt.Sprintf("Early death before floor 7 (floor %d)", *reflection.Floor))
	}
	if containsAnySubstring(combined, "critical hp", "no safety margin") {
		signals["critical_hp"]++
	}
	if containsAnySubstring(combined, "low-health", "low health", "low-health spiral") {
		signals["low_hp"]++
	}
	if containsAnySubstring(combined, "unspent gold", "convert gold earlier", "gold becomes useless") {
		signals["unspent_gold"]++
	}
	if containsAnySubstring(combined, "route", "pathing", "path ") {
		signals["route_pressure"]++
	}
	if containsAnySubstring(combined, "fragile", "thin block", "weak defense", "bloated deck", "tighter deck") {
		signals["fragile_deck"]++
	}

	return dedupeGuideLines(lines, 6), signals
}

func iterSeenContentEntries(registry *SeenContentRegistry) []SeenContentEntry {
	if registry == nil {
		return nil
	}

	total := len(registry.Cards) + len(registry.Relics) + len(registry.Potions) + len(registry.Monsters) + len(registry.Events) + len(registry.Characters)
	items := make([]SeenContentEntry, 0, total)
	items = append(items, tagSeenContentEntries(seenCategoryCards, registry.Cards)...)
	items = append(items, tagSeenContentEntries(seenCategoryRelics, registry.Relics)...)
	items = append(items, tagSeenContentEntries(seenCategoryPotions, registry.Potions)...)
	items = append(items, tagSeenContentEntries(seenCategoryMonsters, registry.Monsters)...)
	items = append(items, tagSeenContentEntries(seenCategoryEvents, registry.Events)...)
	items = append(items, tagSeenContentEntries(seenCategoryCharacters, registry.Characters)...)
	return items
}

func mergeOutcomeCounts(target map[string]int, source map[string]int) {
	for key, count := range source {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		target[key] += count
	}
}

func mergeSignalCounts(target map[string]int, source map[string]int) {
	for key, count := range source {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		target[key] += count
	}
}

func updateOptionalFloorRange(entry *codexEntityEvidence, floor *int) {
	if entry == nil || floor == nil {
		return
	}
	updateOptionalFloorBounds(&entry.MinFloor, &entry.MaxFloor, floor)
}

func updateOptionalFloorBounds(minFloor **int, maxFloor **int, floor *int) {
	if floor == nil {
		return
	}
	if *minFloor == nil || *floor < **minFloor {
		copy := *floor
		*minFloor = &copy
	}
	if *maxFloor == nil || *floor > **maxFloor {
		copy := *floor
		*maxFloor = &copy
	}
}

func containsAnySubstring(text string, fragments ...string) bool {
	for _, fragment := range fragments {
		if fragment != "" && strings.Contains(text, fragment) {
			return true
		}
	}
	return false
}
