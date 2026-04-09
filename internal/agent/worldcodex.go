package agentruntime

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"spire2mind/internal/game"
	"spire2mind/internal/i18n"
)

const (
	seenCategoryCards      = "cards"
	seenCategoryRelics     = "relics"
	seenCategoryPotions    = "potions"
	seenCategoryMonsters   = "monsters"
	seenCategoryEvents     = "events"
	seenCategoryCharacters = "characters"
)

type SeenContentEntry struct {
	Category      string    `json:"category,omitempty"`
	ID            string    `json:"id"`
	Name          string    `json:"name,omitempty"`
	RawName       string    `json:"raw_name,omitempty"`
	NameEN        string    `json:"name_en,omitempty"`
	FirstSeenAt   time.Time `json:"first_seen_at"`
	LastSeenAt    time.Time `json:"last_seen_at"`
	FirstRunID    string    `json:"first_run_id,omitempty"`
	LastRunID     string    `json:"last_run_id,omitempty"`
	FirstScreen   string    `json:"first_screen,omitempty"`
	LastScreen    string    `json:"last_screen,omitempty"`
	FirstFloor    *int      `json:"first_floor,omitempty"`
	LastFloor     *int      `json:"last_floor,omitempty"`
	SeenCount     int       `json:"seen_count"`
	RiskTags      []string  `json:"risk_tags,omitempty"`
	ResponseHints []string  `json:"response_hints,omitempty"`
	FailureLinks  []string  `json:"failure_links,omitempty"`
}

type SeenContentRegistry struct {
	UpdatedAt  time.Time          `json:"updated_at"`
	Cards      []SeenContentEntry `json:"cards,omitempty"`
	Relics     []SeenContentEntry `json:"relics,omitempty"`
	Potions    []SeenContentEntry `json:"potions,omitempty"`
	Monsters   []SeenContentEntry `json:"monsters,omitempty"`
	Events     []SeenContentEntry `json:"events,omitempty"`
	Characters []SeenContentEntry `json:"characters,omitempty"`
}

type SeenContentTracker struct {
	registry SeenContentRegistry
	index    map[string]map[string]int
	seenKeys map[string]map[string]map[string]struct{}
	recent   []SeenContentEntry
}

type seenContentContext struct {
	now    time.Time
	runID  string
	screen string
	floor  *int
}

func NewSeenContentTracker() *SeenContentTracker {
	return &SeenContentTracker{
		index:    make(map[string]map[string]int),
		seenKeys: make(map[string]map[string]map[string]struct{}),
	}
}

func (t *SeenContentTracker) Merge(registry *SeenContentRegistry) {
	if t == nil || registry == nil {
		return
	}

	t.mergeEntries(seenCategoryCards, registry.Cards)
	t.mergeEntries(seenCategoryRelics, registry.Relics)
	t.mergeEntries(seenCategoryPotions, registry.Potions)
	t.mergeEntries(seenCategoryMonsters, registry.Monsters)
	t.mergeEntries(seenCategoryEvents, registry.Events)
	t.mergeEntries(seenCategoryCharacters, registry.Characters)
	if registry.UpdatedAt.After(t.registry.UpdatedAt) {
		t.registry.UpdatedAt = registry.UpdatedAt
	}
}

func (t *SeenContentTracker) Observe(state *game.StateSnapshot) []SeenContentEntry {
	if t == nil || state == nil {
		return nil
	}

	ctx := seenContentContext{
		now:    time.Now(),
		runID:  strings.TrimSpace(state.RunID),
		screen: strings.TrimSpace(state.Screen),
		floor:  seenContentFloor(state),
	}

	start := len(t.recent)

	t.observeEntries(seenCategoryCards, nestedList(state.Combat, "hand"), ctx, "cardId", "id")
	t.observeEntries(seenCategoryCards, nestedList(state.Reward, "cardOptions"), ctx, "cardId", "id")
	t.observeEntries(seenCategoryCards, nestedList(state.Selection, "cards"), ctx, "cardId", "id")
	t.observeEntries(seenCategoryCards, nestedList(state.Shop, "cards"), ctx, "cardId", "id")
	t.observeDeck(state, ctx)

	t.observeEntries(seenCategoryRelics, nestedList(state.Chest, "relicOptions"), ctx, "relicId", "id")
	t.observeEntries(seenCategoryRelics, nestedList(state.Shop, "relics"), ctx, "relicId", "id")
	t.observeEntries(seenCategoryPotions, nestedList(state.Shop, "potions"), ctx, "potionId", "id")
	t.observeEntries(seenCategoryMonsters, nestedList(state.Combat, "enemies"), ctx, "enemyId", "id")
	t.observeEvent(state, ctx)
	t.observeCharacters(state, ctx)

	t.registry.UpdatedAt = ctx.now
	if start >= len(t.recent) {
		return nil
	}

	discoveries := append([]SeenContentEntry(nil), t.recent[start:]...)
	if len(discoveries) > 8 {
		discoveries = discoveries[len(discoveries)-8:]
	}
	return discoveries
}

func (t *SeenContentTracker) Snapshot() *SeenContentRegistry {
	if t == nil {
		return nil
	}

	snapshot := &SeenContentRegistry{
		UpdatedAt:  t.registry.UpdatedAt,
		Cards:      cloneSeenContentEntries(t.registry.Cards),
		Relics:     cloneSeenContentEntries(t.registry.Relics),
		Potions:    cloneSeenContentEntries(t.registry.Potions),
		Monsters:   cloneSeenContentEntries(t.registry.Monsters),
		Events:     cloneSeenContentEntries(t.registry.Events),
		Characters: cloneSeenContentEntries(t.registry.Characters),
	}

	sortSeenContentEntries(snapshot.Cards)
	sortSeenContentEntries(snapshot.Relics)
	sortSeenContentEntries(snapshot.Potions)
	sortSeenContentEntries(snapshot.Monsters)
	sortSeenContentEntries(snapshot.Events)
	sortSeenContentEntries(snapshot.Characters)
	return snapshot
}

func cloneSeenContentEntries(entries []SeenContentEntry) []SeenContentEntry {
	if len(entries) == 0 {
		return nil
	}
	cloned := append([]SeenContentEntry(nil), entries...)
	for i := range cloned {
		cloned[i].RawName = entries[i].RawName
		cloned[i].NameEN = entries[i].NameEN
		cloned[i].RiskTags = append([]string(nil), entries[i].RiskTags...)
		cloned[i].ResponseHints = append([]string(nil), entries[i].ResponseHints...)
		cloned[i].FailureLinks = append([]string(nil), entries[i].FailureLinks...)
	}
	return cloned
}

func (t *SeenContentTracker) Recent(limit int) []SeenContentEntry {
	if t == nil || len(t.recent) == 0 {
		return nil
	}
	if limit <= 0 || len(t.recent) <= limit {
		return append([]SeenContentEntry(nil), t.recent...)
	}
	return append([]SeenContentEntry(nil), t.recent[len(t.recent)-limit:]...)
}

func (r *SeenContentRegistry) Counts() map[string]int {
	if r == nil {
		return nil
	}

	return map[string]int{
		seenCategoryCards:      len(r.Cards),
		seenCategoryRelics:     len(r.Relics),
		seenCategoryPotions:    len(r.Potions),
		seenCategoryMonsters:   len(r.Monsters),
		seenCategoryEvents:     len(r.Events),
		seenCategoryCharacters: len(r.Characters),
	}
}

func RecentSeenContentEntries(registry *SeenContentRegistry, limit int) []SeenContentEntry {
	if registry == nil {
		return nil
	}

	entries := make([]SeenContentEntry, 0, len(registry.Cards)+len(registry.Relics)+len(registry.Potions)+len(registry.Monsters)+len(registry.Events)+len(registry.Characters))
	entries = append(entries, tagSeenContentEntries(seenCategoryCards, registry.Cards)...)
	entries = append(entries, tagSeenContentEntries(seenCategoryRelics, registry.Relics)...)
	entries = append(entries, tagSeenContentEntries(seenCategoryPotions, registry.Potions)...)
	entries = append(entries, tagSeenContentEntries(seenCategoryMonsters, registry.Monsters)...)
	entries = append(entries, tagSeenContentEntries(seenCategoryEvents, registry.Events)...)
	entries = append(entries, tagSeenContentEntries(seenCategoryCharacters, registry.Characters)...)

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].FirstSeenAt.Equal(entries[j].FirstSeenAt) {
			if entries[i].Name == entries[j].Name {
				return entries[i].ID < entries[j].ID
			}
			return entries[i].Name < entries[j].Name
		}
		return entries[i].FirstSeenAt.After(entries[j].FirstSeenAt)
	})

	if limit > 0 && len(entries) > limit {
		return append([]SeenContentEntry(nil), entries[:limit]...)
	}
	return entries
}

func findSeenContentEntry(registry *SeenContentRegistry, category string, id string, name string) *SeenContentEntry {
	if registry == nil {
		return nil
	}

	id = normalizeSeenContentID(id, name)
	name = strings.TrimSpace(name)

	entries := categoryEntries(registry, category)
	for i := range entries {
		if normalizeSeenContentID(entries[i].ID, firstNonEmpty(entries[i].RawName, entries[i].NameEN, entries[i].Name)) == id {
			return &entries[i]
		}
	}
	if name == "" {
		return nil
	}
	for i := range entries {
		if strings.EqualFold(strings.TrimSpace(entries[i].Name), name) ||
			strings.EqualFold(strings.TrimSpace(entries[i].RawName), name) ||
			strings.EqualFold(strings.TrimSpace(entries[i].NameEN), name) {
			return &entries[i]
		}
	}
	return nil
}

func categoryEntries(registry *SeenContentRegistry, category string) []SeenContentEntry {
	if registry == nil {
		return nil
	}
	switch category {
	case seenCategoryCards:
		return registry.Cards
	case seenCategoryRelics:
		return registry.Relics
	case seenCategoryPotions:
		return registry.Potions
	case seenCategoryMonsters:
		return registry.Monsters
	case seenCategoryEvents:
		return registry.Events
	case seenCategoryCharacters:
		return registry.Characters
	default:
		return nil
	}
}

func SummarizeSeenContentEntries(entries []SeenContentEntry, language i18n.Language) []string {
	return SummarizeSeenContentEntriesWithCatalog(entries, nil, language)
}

func SummarizeSeenContentEntriesWithCatalog(entries []SeenContentEntry, catalog *codexCatalog, language i18n.Language) []string {
	loc := i18n.New(language)
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		normalized := entry
		normalizeSeenContentDisplay(&normalized, catalog)
		name := valueOrDash(bestSeenContentName(normalized))
		category := seenContentLabel(loc, normalized.Category)
		if normalized.FirstFloor != nil {
			lines = append(lines, fmt.Sprintf("%s: %s (%s `%d`)", category, name, loc.Label("floor", "层数"), *normalized.FirstFloor))
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: %s", category, name))
	}
	return lines
}

func seenContentCountLines(registry *SeenContentRegistry, language i18n.Language) []string {
	loc := i18n.New(language)
	if registry == nil {
		return []string{"- -"}
	}

	counts := registry.Counts()
	order := []string{
		seenCategoryCards,
		seenCategoryRelics,
		seenCategoryPotions,
		seenCategoryMonsters,
		seenCategoryEvents,
		seenCategoryCharacters,
	}
	lines := make([]string, 0, len(order))
	for _, category := range order {
		lines = append(lines, fmt.Sprintf("- %s: `%d`", seenCategoryHeading(loc, category), counts[category]))
	}
	return lines
}

func seenContentCountsData(registry *SeenContentRegistry) map[string]int {
	if registry == nil {
		return map[string]int{}
	}
	return registry.Counts()
}

func seenContentCountsFromData(value interface{}) map[string]int {
	switch typed := value.(type) {
	case map[string]int:
		return typed
	case map[string]interface{}:
		counts := make(map[string]int, len(typed))
		for key, raw := range typed {
			switch value := raw.(type) {
			case int:
				counts[key] = value
			case int32:
				counts[key] = int(value)
			case int64:
				counts[key] = int(value)
			case float64:
				counts[key] = int(value)
			}
		}
		return counts
	default:
		return nil
	}
}

func (t *SeenContentTracker) observeDeck(state *game.StateSnapshot, ctx seenContentContext) {
	if state == nil || len(state.Run) == 0 {
		return
	}

	rawDeck, ok := state.Run["deck"].([]interface{})
	if !ok {
		return
	}

	for _, item := range rawDeck {
		card, ok := item.(map[string]any)
		if !ok {
			continue
		}
		t.observeEntry(seenCategoryCards, card, ctx, "cardId", "id")
	}
}

func (t *SeenContentTracker) observeEvent(state *game.StateSnapshot, ctx seenContentContext) {
	if state == nil || len(state.Event) == 0 {
		return
	}

	t.observeEntry(seenCategoryEvents, state.Event, ctx, "eventId", "id")
}

func (t *SeenContentTracker) observeCharacters(state *game.StateSnapshot, ctx seenContentContext) {
	if state == nil {
		return
	}

	for _, character := range nestedList(state.CharacterSelect, "characters") {
		t.observeEntry(seenCategoryCharacters, character, ctx, "characterId", "id")
	}
	if name := strings.TrimSpace(fieldString(state.Run, "character")); name != "" {
		t.observeNamed(seenCategoryCharacters, "", name, ctx)
	}
	if name := strings.TrimSpace(fieldString(state.GameOver, "characterId")); name != "" {
		t.observeNamed(seenCategoryCharacters, name, name, ctx)
	}
}

func (t *SeenContentTracker) observeEntries(category string, items []map[string]any, ctx seenContentContext, idKeys ...string) {
	for _, item := range items {
		t.observeEntry(category, item, ctx, idKeys...)
	}
}

func (t *SeenContentTracker) observeEntry(category string, item map[string]any, ctx seenContentContext, idKeys ...string) {
	if len(item) == 0 {
		return
	}

	id := stableIdentity(item, idKeys...)
	name := seenContentName(category, item)
	t.observeNamed(category, id, name, ctx)
}

func (t *SeenContentTracker) observeNamed(category string, id string, name string, ctx seenContentContext) {
	id = normalizeSeenContentID(id, name)
	name = strings.TrimSpace(name)
	if id == "" {
		return
	}

	entry, created := t.upsert(category, id, name, ctx)
	if created {
		t.recent = append(t.recent, entry)
		if len(t.recent) > 64 {
			t.recent = t.recent[len(t.recent)-64:]
		}
	}
}

func (t *SeenContentTracker) mergeEntries(category string, entries []SeenContentEntry) {
	for _, entry := range entries {
		id := normalizeSeenContentID(entry.ID, firstNonEmpty(entry.RawName, entry.NameEN, entry.Name))
		if id == "" {
			continue
		}
		t.mergeEntry(category, entry, id)
	}
}

func (t *SeenContentTracker) mergeEntry(category string, incoming SeenContentEntry, id string) {
	entries := t.categoryEntries(category)
	if entries == nil {
		return
	}

	categoryIndex := t.ensureCategoryIndex(category)
	if existingIndex, ok := categoryIndex[id]; ok {
		existing := &(*entries)[existingIndex]
		if existing.Category == "" {
			existing.Category = category
		}
		if existing.RawName == "" && firstNonEmpty(incoming.RawName, incoming.Name) != "" {
			existing.RawName = cleanVisibleText(firstNonEmpty(incoming.RawName, incoming.Name))
		}
		if existing.NameEN == "" && incoming.NameEN != "" {
			existing.NameEN = cleanVisibleText(incoming.NameEN)
		}
		if existing.Name == "" && firstNonEmpty(incoming.Name, incoming.NameEN, incoming.RawName) != "" {
			existing.Name = bestSeenContentNameValues(incoming.Name, incoming.NameEN, firstNonEmpty(incoming.RawName, incoming.ID))
		}
		if existing.FirstSeenAt.IsZero() || (!incoming.FirstSeenAt.IsZero() && incoming.FirstSeenAt.Before(existing.FirstSeenAt)) {
			existing.FirstSeenAt = incoming.FirstSeenAt
			existing.FirstRunID = incoming.FirstRunID
			existing.FirstScreen = incoming.FirstScreen
			existing.FirstFloor = cloneOptionalInt(incoming.FirstFloor)
		}
		if incoming.LastSeenAt.After(existing.LastSeenAt) {
			existing.LastSeenAt = incoming.LastSeenAt
			existing.LastRunID = incoming.LastRunID
			existing.LastScreen = incoming.LastScreen
			existing.LastFloor = cloneOptionalInt(incoming.LastFloor)
		}
		existing.SeenCount++
		t.recordHistoricalObservation(category, id, existing)
		return
	}

	entry := incoming
	entry.ID = id
	entry.Category = category
	entry.RawName = cleanVisibleText(firstNonEmpty(incoming.RawName, incoming.Name))
	entry.NameEN = cleanVisibleText(incoming.NameEN)
	entry.Name = bestSeenContentNameValues(incoming.Name, incoming.NameEN, firstNonEmpty(entry.RawName, incoming.ID))
	entry.SeenCount = 1
	*entries = append(*entries, entry)
	categoryIndex[id] = len(*entries) - 1
	t.recordHistoricalObservation(category, id, &entry)
}

func (t *SeenContentTracker) upsert(category string, id string, name string, ctx seenContentContext) (SeenContentEntry, bool) {
	entries := t.categoryEntries(category)
	if entries == nil {
		return SeenContentEntry{}, false
	}

	categoryIndex := t.ensureCategoryIndex(category)
	observationKey := seenObservationKey(ctx)
	if existingIndex, ok := categoryIndex[id]; ok {
		existing := &(*entries)[existingIndex]
		if existing.Category == "" {
			existing.Category = category
		}
		if existing.Name == "" && name != "" {
			existing.Name = cleanVisibleText(name)
		}
		if existing.RawName == "" && name != "" {
			existing.RawName = cleanVisibleText(name)
		}
		existing.LastSeenAt = ctx.now
		existing.LastRunID = ctx.runID
		existing.LastScreen = ctx.screen
		existing.LastFloor = cloneOptionalInt(ctx.floor)
		if t.recordObservation(category, id, observationKey) {
			existing.SeenCount++
		}
		return *existing, false
	}

	entry := SeenContentEntry{
		Category:    category,
		ID:          id,
		Name:        cleanVisibleText(name),
		RawName:     cleanVisibleText(name),
		FirstSeenAt: ctx.now,
		LastSeenAt:  ctx.now,
		FirstRunID:  ctx.runID,
		LastRunID:   ctx.runID,
		FirstScreen: ctx.screen,
		LastScreen:  ctx.screen,
		FirstFloor:  cloneOptionalInt(ctx.floor),
		LastFloor:   cloneOptionalInt(ctx.floor),
		SeenCount:   1,
	}
	*entries = append(*entries, entry)
	categoryIndex[id] = len(*entries) - 1
	t.recordObservation(category, id, observationKey)
	return entry, true
}

func (t *SeenContentTracker) categoryEntries(category string) *[]SeenContentEntry {
	switch category {
	case seenCategoryCards:
		return &t.registry.Cards
	case seenCategoryRelics:
		return &t.registry.Relics
	case seenCategoryPotions:
		return &t.registry.Potions
	case seenCategoryMonsters:
		return &t.registry.Monsters
	case seenCategoryEvents:
		return &t.registry.Events
	case seenCategoryCharacters:
		return &t.registry.Characters
	default:
		return nil
	}
}

func (t *SeenContentTracker) ensureCategoryIndex(category string) map[string]int {
	if existing, ok := t.index[category]; ok {
		return existing
	}
	created := make(map[string]int)
	t.index[category] = created
	return created
}

func (t *SeenContentTracker) ensureCategorySeenKeys(category string) map[string]map[string]struct{} {
	if existing, ok := t.seenKeys[category]; ok {
		return existing
	}
	created := make(map[string]map[string]struct{})
	t.seenKeys[category] = created
	return created
}

func (t *SeenContentTracker) recordObservation(category string, id string, key string) bool {
	if t == nil || strings.TrimSpace(id) == "" || strings.TrimSpace(key) == "" {
		return false
	}

	categorySeen := t.ensureCategorySeenKeys(category)
	itemSeen, ok := categorySeen[id]
	if !ok {
		itemSeen = make(map[string]struct{})
		categorySeen[id] = itemSeen
	}
	if _, exists := itemSeen[key]; exists {
		return false
	}
	itemSeen[key] = struct{}{}
	return true
}

func (t *SeenContentTracker) recordHistoricalObservation(category string, id string, entry *SeenContentEntry) {
	if t == nil || entry == nil {
		return
	}
	if key := seenObservationKey(seenContentContext{
		runID:  entry.FirstRunID,
		screen: entry.FirstScreen,
		floor:  entry.FirstFloor,
	}); key != "" {
		t.recordObservation(category, id, key)
	}
	if key := seenObservationKey(seenContentContext{
		runID:  entry.LastRunID,
		screen: entry.LastScreen,
		floor:  entry.LastFloor,
	}); key != "" {
		t.recordObservation(category, id, key)
	}
}

func seenObservationKey(ctx seenContentContext) string {
	parts := []string{
		strings.TrimSpace(ctx.runID),
		strings.TrimSpace(ctx.screen),
	}
	if ctx.floor != nil {
		parts = append(parts, fmt.Sprintf("floor:%d", *ctx.floor))
	}
	key := strings.TrimSpace(strings.Join(parts, "|"))
	if key == "" || key == "|" {
		return ""
	}
	return key
}

func seenContentFloor(state *game.StateSnapshot) *int {
	if state == nil {
		return nil
	}
	if floor, ok := fieldInt(state.Run, "floor"); ok {
		return &floor
	}
	if floor, ok := fieldInt(state.GameOver, "floor"); ok {
		return &floor
	}
	return nil
}

func seenContentName(category string, item map[string]any) string {
	switch category {
	case seenCategoryEvents:
		return fallbackID(fieldString(item, "title"), fieldString(item, "eventId"))
	case seenCategoryCharacters:
		return fallbackID(fieldString(item, "name"), fieldString(item, "characterId"))
	default:
		return firstNonEmpty(
			strings.TrimSpace(fieldString(item, "name")),
			strings.TrimSpace(fieldString(item, "label")),
			strings.TrimSpace(fieldString(item, "title")),
			strings.TrimSpace(fieldString(item, "cardId")),
			strings.TrimSpace(fieldString(item, "relicId")),
			strings.TrimSpace(fieldString(item, "potionId")),
			strings.TrimSpace(fieldString(item, "enemyId")),
		)
	}
}

func normalizeSeenContentID(id string, fallbackName string) string {
	id = normalizeMatchText(id)
	if id != "" {
		return id
	}
	fallbackName = normalizeMatchText(fallbackName)
	if fallbackName == "" {
		return ""
	}
	return "name:" + fallbackName
}

func sortSeenContentEntries(entries []SeenContentEntry) {
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].ID == entries[j].ID {
			return entries[i].Name < entries[j].Name
		}
		return entries[i].ID < entries[j].ID
	})
}

func tagSeenContentEntries(category string, entries []SeenContentEntry) []SeenContentEntry {
	tagged := make([]SeenContentEntry, 0, len(entries))
	for _, entry := range entries {
		copy := entry
		copy.Category = category
		tagged = append(tagged, copy)
	}
	return tagged
}

func seenContentLabel(loc i18n.Localizer, category string) string {
	switch category {
	case seenCategoryCards:
		return loc.Label("Card", "卡牌")
	case seenCategoryRelics:
		return loc.Label("Relic", "遗物")
	case seenCategoryPotions:
		return loc.Label("Potion", "药水")
	case seenCategoryMonsters:
		return loc.Label("Monster", "怪物")
	case seenCategoryEvents:
		return loc.Label("Event", "事件")
	case seenCategoryCharacters:
		return loc.Label("Character", "角色")
	default:
		return category
	}
}

func seenCategoryHeading(loc i18n.Localizer, category string) string {
	switch category {
	case seenCategoryCards:
		return loc.Label("Cards seen", "已见卡牌")
	case seenCategoryRelics:
		return loc.Label("Relics seen", "已见遗物")
	case seenCategoryPotions:
		return loc.Label("Potions seen", "已见药水")
	case seenCategoryMonsters:
		return loc.Label("Monsters seen", "已见怪物")
	case seenCategoryEvents:
		return loc.Label("Events seen", "已见事件")
	case seenCategoryCharacters:
		return loc.Label("Characters seen", "已见角色")
	default:
		return category
	}
}

func seenContentDiscoveryLines(registry *SeenContentRegistry, language i18n.Language, limit int) []string {
	entries := RecentSeenContentEntries(registry, limit)
	if len(entries) == 0 {
		return []string{"- -"}
	}

	loc := i18n.New(language)
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		normalized := entry
		normalizeSeenContentDisplay(&normalized, nil)
		name := valueOrDash(bestSeenContentName(normalized))
		category := seenContentLabel(loc, normalized.Category)
		if normalized.FirstFloor != nil {
			lines = append(lines, fmt.Sprintf("- %s: %s (%s `%d`)", category, name, loc.Label("floor", "层数"), *normalized.FirstFloor))
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", category, name))
	}
	return lines
}

func cloneOptionalInt(value *int) *int {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
