package agentruntime

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type codexCatalog struct {
	Cards      map[string]codexCardMeta
	Monsters   map[string]codexMonsterMeta
	Events     map[string]codexEventMeta
	Relics     map[string]codexRelicMeta
	Potions    map[string]codexPotionMeta
	Characters map[string]codexCharacterMeta
}

type codexCardMeta struct {
	ID            string              `json:"id"`
	Name          string              `json:"name"`
	Description   string              `json:"description"`
	Type          string              `json:"type"`
	Target        string              `json:"target"`
	Damage        *int                `json:"damage"`
	Block         *int                `json:"block"`
	HitCount      *int                `json:"hit_count"`
	CardsDraw     *int                `json:"cards_draw"`
	EnergyGain    *int                `json:"energy_gain"`
	HPLoss        *int                `json:"hp_loss"`
	Keywords      []string            `json:"keywords"`
	Tags          []string            `json:"tags"`
	PowersApplied []codexPowerApplied `json:"powers_applied"`
}

type codexPowerApplied struct {
	Power  string `json:"power"`
	Amount *int   `json:"amount"`
}

type codexMonsterMeta struct {
	ID           string                    `json:"id"`
	Name         string                    `json:"name"`
	Type         string                    `json:"type"`
	Moves        []codexMonsterMove        `json:"moves"`
	DamageValues map[string]map[string]int `json:"damage_values"`
}

type codexMonsterMove struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type codexEventMeta struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Type        string             `json:"type"`
	Act         string             `json:"act"`
	Description string             `json:"description"`
	Options     []codexEventOption `json:"options"`
	Pages       []codexEventPage   `json:"pages"`
}

type codexEventOption struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type codexEventPage struct {
	ID          string             `json:"id"`
	Description string             `json:"description"`
	Options     []codexEventOption `json:"options"`
}

type codexRelicMeta struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Rarity      string `json:"rarity"`
	Pool        string `json:"pool"`
}

type codexPotionMeta struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Rarity      string `json:"rarity"`
	Pool        string `json:"pool"`
}

type codexCharacterMeta struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func loadCodexCatalogForArtifactsRoot(artifactsRoot string) (*codexCatalog, error) {
	repoRoot := filepath.Dir(filepath.Dir(filepath.Clean(artifactsRoot)))
	return loadCodexCatalogFromDataDir(filepath.Join(repoRoot, "data", "eng"))
}

func loadCodexCatalogFromDataDir(dataDir string) (*codexCatalog, error) {
	catalog := emptyCodexCatalog()
	if strings.TrimSpace(dataDir) == "" {
		return catalog, nil
	}

	info, err := os.Stat(dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return catalog, nil
		}
		return nil, err
	}
	if !info.IsDir() {
		return catalog, nil
	}

	if catalog.Cards, err = loadCatalogMap[codexCardMeta](filepath.Join(dataDir, "cards.json")); err != nil {
		return nil, err
	}
	if catalog.Monsters, err = loadCatalogMap[codexMonsterMeta](filepath.Join(dataDir, "monsters.json")); err != nil {
		return nil, err
	}
	if catalog.Events, err = loadCatalogMap[codexEventMeta](filepath.Join(dataDir, "events.json")); err != nil {
		return nil, err
	}
	if catalog.Relics, err = loadCatalogMap[codexRelicMeta](filepath.Join(dataDir, "relics.json")); err != nil {
		return nil, err
	}
	if catalog.Potions, err = loadCatalogMap[codexPotionMeta](filepath.Join(dataDir, "potions.json")); err != nil {
		return nil, err
	}
	if catalog.Characters, err = loadCatalogMap[codexCharacterMeta](filepath.Join(dataDir, "characters.json")); err != nil {
		return nil, err
	}

	return catalog, nil
}

func emptyCodexCatalog() *codexCatalog {
	return &codexCatalog{
		Cards:      make(map[string]codexCardMeta),
		Monsters:   make(map[string]codexMonsterMeta),
		Events:     make(map[string]codexEventMeta),
		Relics:     make(map[string]codexRelicMeta),
		Potions:    make(map[string]codexPotionMeta),
		Characters: make(map[string]codexCharacterMeta),
	}
}

func loadCatalogMap[T interface{ getID() string }](path string) (map[string]T, error) {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]T), nil
		}
		return nil, err
	}
	if info.IsDir() {
		return make(map[string]T), nil
	}

	bytes, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var items []T
	if err := json.Unmarshal(bytes, &items); err != nil {
		return nil, err
	}

	index := make(map[string]T, len(items))
	for _, item := range items {
		id := normalizeSeenContentID(item.getID(), item.getID())
		if id == "" {
			continue
		}
		index[id] = item
	}
	return index, nil
}

func (m codexCardMeta) getID() string      { return m.ID }
func (m codexMonsterMeta) getID() string   { return m.ID }
func (m codexEventMeta) getID() string     { return m.ID }
func (m codexRelicMeta) getID() string     { return m.ID }
func (m codexPotionMeta) getID() string    { return m.ID }
func (m codexCharacterMeta) getID() string { return m.ID }

func (c *codexCatalog) canonicalName(category string, id string) string {
	if c == nil {
		return ""
	}
	id = normalizeSeenContentID(id, id)
	switch strings.TrimSpace(category) {
	case seenCategoryCards:
		return strings.TrimSpace(c.Cards[id].Name)
	case seenCategoryMonsters:
		return strings.TrimSpace(c.Monsters[id].Name)
	case seenCategoryEvents:
		return strings.TrimSpace(c.Events[id].Name)
	case seenCategoryRelics:
		return strings.TrimSpace(c.Relics[id].Name)
	case seenCategoryPotions:
		return strings.TrimSpace(c.Potions[id].Name)
	case seenCategoryCharacters:
		return strings.TrimSpace(c.Characters[id].Name)
	default:
		return ""
	}
}

func (c *codexCatalog) card(id string) *codexCardMeta {
	if c == nil {
		return nil
	}
	id = normalizeSeenContentID(id, id)
	meta, ok := c.Cards[id]
	if !ok {
		return nil
	}
	return &meta
}

func (c *codexCatalog) monster(id string) *codexMonsterMeta {
	if c == nil {
		return nil
	}
	id = normalizeSeenContentID(id, id)
	meta, ok := c.Monsters[id]
	if !ok {
		return nil
	}
	return &meta
}

func (c *codexCatalog) event(id string) *codexEventMeta {
	if c == nil {
		return nil
	}
	id = normalizeSeenContentID(id, id)
	meta, ok := c.Events[id]
	if !ok {
		return nil
	}
	return &meta
}

func (c *codexCatalog) relic(id string) *codexRelicMeta {
	if c == nil {
		return nil
	}
	id = normalizeSeenContentID(id, id)
	meta, ok := c.Relics[id]
	if !ok {
		return nil
	}
	return &meta
}

func (c *codexCatalog) potion(id string) *codexPotionMeta {
	if c == nil {
		return nil
	}
	id = normalizeSeenContentID(id, id)
	meta, ok := c.Potions[id]
	if !ok {
		return nil
	}
	return &meta
}

func (c *codexCatalog) character(id string) *codexCharacterMeta {
	if c == nil {
		return nil
	}
	id = normalizeSeenContentID(id, id)
	meta, ok := c.Characters[id]
	if !ok {
		return nil
	}
	return &meta
}

func catalogCardText(meta *codexCardMeta) string {
	if meta == nil {
		return ""
	}
	parts := []string{meta.Name, meta.Description, meta.Type, meta.Target}
	parts = append(parts, meta.Keywords...)
	parts = append(parts, meta.Tags...)
	for _, power := range meta.PowersApplied {
		parts = append(parts, power.Power)
	}
	return strings.ToLower(strings.Join(parts, " "))
}

func catalogMonsterText(meta *codexMonsterMeta) string {
	if meta == nil {
		return ""
	}
	parts := []string{meta.Name, meta.Type}
	for _, move := range meta.Moves {
		parts = append(parts, move.ID, move.Name)
	}
	return strings.ToLower(strings.Join(parts, " "))
}

func catalogEventText(meta *codexEventMeta) string {
	if meta == nil {
		return ""
	}
	parts := []string{meta.Name, meta.Type, meta.Act, meta.Description}
	for _, option := range meta.Options {
		parts = append(parts, option.ID, option.Title, option.Description)
	}
	for _, page := range meta.Pages {
		parts = append(parts, page.ID, page.Description)
		for _, option := range page.Options {
			parts = append(parts, option.ID, option.Title, option.Description)
		}
	}
	return strings.ToLower(strings.Join(parts, " "))
}

func catalogRelicText(meta *codexRelicMeta) string {
	if meta == nil {
		return ""
	}
	return strings.ToLower(strings.Join([]string{meta.Name, meta.Description, meta.Rarity, meta.Pool}, " "))
}

func catalogPotionText(meta *codexPotionMeta) string {
	if meta == nil {
		return ""
	}
	return strings.ToLower(strings.Join([]string{meta.Name, meta.Description, meta.Rarity, meta.Pool}, " "))
}

func maxMonsterDamage(meta *codexMonsterMeta) int {
	if meta == nil {
		return 0
	}
	best := 0
	for _, values := range meta.DamageValues {
		for _, value := range values {
			best = max(best, value)
		}
	}
	return best
}
