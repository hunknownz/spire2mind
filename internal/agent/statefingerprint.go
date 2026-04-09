package agentruntime

import (
	"encoding/json"
	"strings"

	"spire2mind/internal/game"
)

type decisionFingerprint struct {
	RunID            string                        `json:"run_id,omitempty"`
	Screen           string                        `json:"screen,omitempty"`
	Turn             *int                          `json:"turn,omitempty"`
	AvailableActions []string                      `json:"available_actions,omitempty"`
	Combat           *combatDecisionFingerprint    `json:"combat,omitempty"`
	Reward           *rewardDecisionFingerprint    `json:"reward,omitempty"`
	Map              *mapDecisionFingerprint       `json:"map,omitempty"`
	Event            *eventDecisionFingerprint     `json:"event,omitempty"`
	Selection        *selectionDecisionFingerprint `json:"selection,omitempty"`
	Shop             *shopDecisionFingerprint      `json:"shop,omitempty"`
	Rest             *restDecisionFingerprint      `json:"rest,omitempty"`
	Chest            *chestDecisionFingerprint     `json:"chest,omitempty"`
	CharacterSelect  *characterDecisionFingerprint `json:"character_select,omitempty"`
	GameOver         *gameOverDecisionFingerprint  `json:"game_over,omitempty"`
}

type combatDecisionFingerprint struct {
	Player  combatPlayerFingerprint  `json:"player"`
	Hand    []combatCardFingerprint  `json:"hand,omitempty"`
	Enemies []combatEnemyFingerprint `json:"enemies,omitempty"`
}

type combatPlayerFingerprint struct {
	Energy int `json:"energy,omitempty"`
	Block  int `json:"block,omitempty"`
	Stars  int `json:"stars,omitempty"`
}

type combatCardFingerprint struct {
	Index          int    `json:"index"`
	CardID         string `json:"card_id,omitempty"`
	Name           string `json:"name,omitempty"`
	EnergyCost     int    `json:"energy_cost,omitempty"`
	Playable       bool   `json:"playable,omitempty"`
	RequiresTarget bool   `json:"requires_target,omitempty"`
	ValidTargets   []int  `json:"valid_targets,omitempty"`
}

type combatEnemyFingerprint struct {
	Index     int      `json:"index"`
	EnemyID   string   `json:"enemy_id,omitempty"`
	Name      string   `json:"name,omitempty"`
	CurrentHP int      `json:"current_hp,omitempty"`
	Block     int      `json:"block,omitempty"`
	Hittable  bool     `json:"hittable,omitempty"`
	Intents   []string `json:"intents,omitempty"`
}

type rewardDecisionFingerprint struct {
	Phase             string                    `json:"phase,omitempty"`
	SourceScreen      string                    `json:"source_screen,omitempty"`
	SourceHint        string                    `json:"source_hint,omitempty"`
	PendingCardChoice bool                      `json:"pending_card_choice,omitempty"`
	CanProceed        bool                      `json:"can_proceed,omitempty"`
	Rewards           []indexedFlagFingerprint  `json:"rewards,omitempty"`
	CardOptions       []indexedLabelFingerprint `json:"card_options,omitempty"`
}

type mapDecisionFingerprint struct {
	Traveling      bool                 `json:"traveling,omitempty"`
	CurrentNode    *mapNodeFingerprint  `json:"current_node,omitempty"`
	AvailableNodes []mapNodeFingerprint `json:"available_nodes,omitempty"`
}

type mapNodeFingerprint struct {
	Index    int    `json:"index,omitempty"`
	Row      int    `json:"row,omitempty"`
	Col      int    `json:"col,omitempty"`
	NodeType string `json:"node_type,omitempty"`
}

type eventDecisionFingerprint struct {
	IsFinished bool                     `json:"is_finished,omitempty"`
	Title      string                   `json:"title,omitempty"`
	Options    []eventOptionFingerprint `json:"options,omitempty"`
}

type eventOptionFingerprint struct {
	Index     int    `json:"index"`
	Label     string `json:"label,omitempty"`
	IsLocked  bool   `json:"is_locked,omitempty"`
	IsProceed bool   `json:"is_proceed,omitempty"`
}

type selectionDecisionFingerprint struct {
	Kind                 string                    `json:"kind,omitempty"`
	SourceScreen         string                    `json:"source_screen,omitempty"`
	SourceHint           string                    `json:"source_hint,omitempty"`
	Mode                 string                    `json:"mode,omitempty"`
	RequiresConfirmation bool                      `json:"requires_confirmation,omitempty"`
	CanConfirm           bool                      `json:"can_confirm,omitempty"`
	Cards                []indexedLabelFingerprint `json:"cards,omitempty"`
}

type shopDecisionFingerprint struct {
	Cards       []pricedOptionFingerprint `json:"cards,omitempty"`
	Relics      []pricedOptionFingerprint `json:"relics,omitempty"`
	Potions     []pricedOptionFingerprint `json:"potions,omitempty"`
	CardRemoval *pricedFlagFingerprint    `json:"card_removal,omitempty"`
}

type restDecisionFingerprint struct {
	Options []restOptionFingerprint `json:"options,omitempty"`
}

type chestDecisionFingerprint struct {
	IsOpened     bool                      `json:"is_opened,omitempty"`
	RelicOptions []indexedLabelFingerprint `json:"relic_options,omitempty"`
}

type characterDecisionFingerprint struct {
	SelectedCharacterID string                       `json:"selected_character_id,omitempty"`
	Characters          []characterOptionFingerprint `json:"characters,omitempty"`
}

type gameOverDecisionFingerprint struct {
	IsVictory bool   `json:"is_victory,omitempty"`
	Floor     int    `json:"floor,omitempty"`
	KilledBy  string `json:"killed_by,omitempty"`
}

type indexedLabelFingerprint struct {
	Index int    `json:"index"`
	ID    string `json:"id,omitempty"`
	Label string `json:"label,omitempty"`
}

type indexedFlagFingerprint struct {
	Index     int    `json:"index"`
	Label     string `json:"label,omitempty"`
	Claimable bool   `json:"claimable,omitempty"`
}

type pricedOptionFingerprint struct {
	Index      int    `json:"index"`
	ID         string `json:"id,omitempty"`
	Label      string `json:"label,omitempty"`
	Price      int    `json:"price,omitempty"`
	EnoughGold bool   `json:"enough_gold,omitempty"`
}

type pricedFlagFingerprint struct {
	Price      int  `json:"price,omitempty"`
	Available  bool `json:"available,omitempty"`
	EnoughGold bool `json:"enough_gold,omitempty"`
}

type restOptionFingerprint struct {
	Index      int    `json:"index"`
	OptionType string `json:"option_type,omitempty"`
	Title      string `json:"title,omitempty"`
	IsEnabled  bool   `json:"is_enabled,omitempty"`
}

type characterOptionFingerprint struct {
	Index       int    `json:"index"`
	CharacterID string `json:"character_id,omitempty"`
	IsLocked    bool   `json:"is_locked,omitempty"`
	IsSelected  bool   `json:"is_selected,omitempty"`
	IsRandom    bool   `json:"is_random,omitempty"`
}

func decisionStateDigest(state *game.StateSnapshot) string {
	if state == nil {
		return "nil"
	}

	payload := buildDecisionFingerprint(state)
	bytes, _ := json.Marshal(payload)
	return string(bytes)
}

func decisionStateSummary(state *game.StateSnapshot) string {
	if state == nil {
		return "state unavailable"
	}

	lines := append([]string(nil), StateSummaryLines(state)...)
	if details := StateDetailLines(state, 3); len(details) > 0 && !(len(details) == 1 && details[0] == "- -") {
		lines = append(lines, details...)
	}

	for i, line := range lines {
		line = strings.TrimSpace(strings.TrimPrefix(line, "- "))
		line = strings.ReplaceAll(line, "`", "")
		lines[i] = line
	}

	return compactText(strings.Join(lines, " | "), 240)
}

func buildDecisionFingerprint(state *game.StateSnapshot) decisionFingerprint {
	payload := decisionFingerprint{
		RunID:            strings.TrimSpace(state.RunID),
		Screen:           strings.TrimSpace(state.Screen),
		Turn:             state.Turn,
		AvailableActions: append([]string(nil), state.AvailableActions...),
	}

	switch state.Screen {
	case "COMBAT":
		payload.Combat = buildCombatDecisionFingerprint(state)
	case "REWARD":
		payload.Reward = buildRewardDecisionFingerprint(state)
	case "MAP":
		payload.Map = buildMapDecisionFingerprint(state)
	case "EVENT":
		payload.Event = buildEventDecisionFingerprint(state)
	case "CARD_SELECTION":
		payload.Selection = buildSelectionDecisionFingerprint(state)
	case "SHOP":
		payload.Shop = buildShopDecisionFingerprint(state)
	case "REST":
		payload.Rest = buildRestDecisionFingerprint(state)
	case "CHEST":
		payload.Chest = buildChestDecisionFingerprint(state)
	case "CHARACTER_SELECT":
		payload.CharacterSelect = buildCharacterDecisionFingerprint(state)
	case "GAME_OVER":
		payload.GameOver = buildGameOverDecisionFingerprint(state)
	}

	return payload
}

func buildCombatDecisionFingerprint(state *game.StateSnapshot) *combatDecisionFingerprint {
	player := asMap(state.Combat["player"])
	payload := &combatDecisionFingerprint{
		Player: combatPlayerFingerprint{
			Energy: fieldIntValue(player, "energy"),
			Block:  fieldIntValue(player, "block"),
			Stars:  fieldIntValue(player, "stars"),
		},
	}

	for _, card := range nestedList(state.Combat, "hand") {
		cardID := fallbackID(fieldString(card, "cardId"), fieldString(card, "id"))
		payload.Hand = append(payload.Hand, combatCardFingerprint{
			Index:          fieldIntValue(card, "index"),
			CardID:         cardID,
			Name:           fingerprintDisplayLabel(cardID, fieldString(card, "name")),
			EnergyCost:     fieldIntValue(card, "energyCost"),
			Playable:       fieldBool(card, "playable"),
			RequiresTarget: cardRequiresTarget(state, card),
			ValidTargets:   append([]int(nil), fieldIntSlice(card, "validTargetIndices")...),
		})
	}

	for _, enemy := range nestedList(state.Combat, "enemies") {
		enemyID := fallbackID(fieldString(enemy, "enemyId"), fieldString(enemy, "id"))
		fingerprint := combatEnemyFingerprint{
			Index:     fieldIntValue(enemy, "index"),
			EnemyID:   enemyID,
			Name:      fingerprintDisplayLabel(enemyID, fieldString(enemy, "name")),
			CurrentHP: fieldIntValue(enemy, "currentHp"),
			Block:     fieldIntValue(enemy, "block"),
			Hittable:  fieldBool(enemy, "isHittable"),
		}
		for _, intent := range nestedList(enemy, "intents") {
			intentType := fieldString(intent, "intentType")
			label := fieldString(intent, "label")
			damage, hasDamage := fieldInt(intent, "totalDamage")
			if !hasDamage {
				damage, _ = fieldInt(intent, "damage")
			}

			intentLabel := strings.TrimSpace(intentType)
			if label != "" && label != intentLabel {
				intentLabel = strings.TrimSpace(intentLabel + ":" + label)
			}
			if damage > 0 {
				intentLabel = strings.TrimSpace(intentLabel + ":" + jsonInt(damage))
			}
			if intentLabel != "" {
				fingerprint.Intents = append(fingerprint.Intents, intentLabel)
			}
		}
		payload.Enemies = append(payload.Enemies, fingerprint)
	}

	return payload
}

func buildRewardDecisionFingerprint(state *game.StateSnapshot) *rewardDecisionFingerprint {
	payload := &rewardDecisionFingerprint{
		Phase:             fieldString(state.Reward, "phase"),
		SourceScreen:      fieldString(state.Reward, "sourceScreen"),
		SourceHint:        fieldString(state.Reward, "sourceHint"),
		PendingCardChoice: fieldBool(state.Reward, "pendingCardChoice"),
		CanProceed:        fieldBool(state.Reward, "canProceed"),
	}
	for _, reward := range nestedList(state.Reward, "rewards") {
		payload.Rewards = append(payload.Rewards, indexedFlagFingerprint{
			Index:     fieldIntValue(reward, "index"),
			Label:     fieldString(reward, "rewardType"),
			Claimable: fieldBool(reward, "claimable"),
		})
	}
	for _, option := range nestedList(state.Reward, "cardOptions") {
		cardID := fallbackID(fieldString(option, "cardId"), fieldString(option, "id"))
		payload.CardOptions = append(payload.CardOptions, indexedLabelFingerprint{
			Index: fieldIntValue(option, "index"),
			ID:    cardID,
			Label: fingerprintDisplayLabel(cardID, fieldString(option, "name")),
		})
	}
	return payload
}

func buildMapDecisionFingerprint(state *game.StateSnapshot) *mapDecisionFingerprint {
	payload := &mapDecisionFingerprint{
		Traveling: fieldBool(state.Map, "isTraveling"),
	}
	if currentNode := asMap(state.Map["currentNode"]); currentNode != nil {
		payload.CurrentNode = &mapNodeFingerprint{
			Index:    fieldIntValue(currentNode, "index"),
			Row:      fieldIntValue(currentNode, "row"),
			Col:      fieldIntValue(currentNode, "col"),
			NodeType: fieldString(currentNode, "nodeType"),
		}
	}
	for _, node := range nestedList(state.Map, "availableNodes") {
		payload.AvailableNodes = append(payload.AvailableNodes, mapNodeFingerprint{
			Index:    fieldIntValue(node, "index"),
			Row:      fieldIntValue(node, "row"),
			Col:      fieldIntValue(node, "col"),
			NodeType: fieldString(node, "nodeType"),
		})
	}
	return payload
}

func buildEventDecisionFingerprint(state *game.StateSnapshot) *eventDecisionFingerprint {
	payload := &eventDecisionFingerprint{
		IsFinished: fieldBool(state.Event, "isFinished"),
		Title:      fieldString(state.Event, "title"),
	}
	for _, option := range nestedList(state.Event, "options") {
		payload.Options = append(payload.Options, eventOptionFingerprint{
			Index:     fieldIntValue(option, "index"),
			Label:     fallbackID(fieldString(option, "label"), fieldString(option, "title")),
			IsLocked:  fieldBool(option, "isLocked"),
			IsProceed: fieldBool(option, "isProceed"),
		})
	}
	return payload
}

func buildSelectionDecisionFingerprint(state *game.StateSnapshot) *selectionDecisionFingerprint {
	payload := &selectionDecisionFingerprint{
		Kind:                 fieldString(state.Selection, "kind"),
		SourceScreen:         fieldString(state.Selection, "sourceScreen"),
		SourceHint:           fieldString(state.Selection, "sourceHint"),
		Mode:                 fieldString(state.Selection, "mode"),
		RequiresConfirmation: fieldBool(state.Selection, "requiresConfirmation"),
		CanConfirm:           fieldBool(state.Selection, "canConfirm"),
	}
	for _, card := range nestedList(state.Selection, "cards") {
		cardID := fallbackID(fieldString(card, "cardId"), fieldString(card, "id"))
		payload.Cards = append(payload.Cards, indexedLabelFingerprint{
			Index: fieldIntValue(card, "index"),
			ID:    cardID,
			Label: fingerprintDisplayLabel(cardID, fieldString(card, "name")),
		})
	}
	return payload
}

func buildShopDecisionFingerprint(state *game.StateSnapshot) *shopDecisionFingerprint {
	payload := &shopDecisionFingerprint{}
	appendOptions := func(source []map[string]any) []pricedOptionFingerprint {
		options := make([]pricedOptionFingerprint, 0, len(source))
		for _, item := range source {
			optionID := firstNonEmpty(fieldString(item, "cardId"), fieldString(item, "relicId"), fieldString(item, "potionId"), fieldString(item, "id"))
			options = append(options, pricedOptionFingerprint{
				Index:      fieldIntValue(item, "index"),
				ID:         optionID,
				Label:      fingerprintDisplayLabel(optionID, fieldString(item, "name")),
				Price:      fieldIntValue(item, "price"),
				EnoughGold: fieldBool(item, "enoughGold"),
			})
		}
		return options
	}
	payload.Cards = appendOptions(nestedList(state.Shop, "cards"))
	payload.Relics = appendOptions(nestedList(state.Shop, "relics"))
	payload.Potions = appendOptions(nestedList(state.Shop, "potions"))
	if removal := asMap(state.Shop["cardRemoval"]); removal != nil {
		payload.CardRemoval = &pricedFlagFingerprint{
			Price:      fieldIntValue(removal, "price"),
			Available:  fieldBool(removal, "available"),
			EnoughGold: fieldBool(removal, "enoughGold"),
		}
	}
	return payload
}

func buildRestDecisionFingerprint(state *game.StateSnapshot) *restDecisionFingerprint {
	payload := &restDecisionFingerprint{}
	for _, option := range nestedList(state.Rest, "options") {
		payload.Options = append(payload.Options, restOptionFingerprint{
			Index:      fieldIntValue(option, "index"),
			OptionType: fieldString(option, "optionType"),
			Title:      fieldString(option, "title"),
			IsEnabled:  fieldBool(option, "isEnabled"),
		})
	}
	return payload
}

func buildChestDecisionFingerprint(state *game.StateSnapshot) *chestDecisionFingerprint {
	payload := &chestDecisionFingerprint{
		IsOpened: fieldBool(state.Chest, "isOpened"),
	}
	for _, option := range nestedList(state.Chest, "relicOptions") {
		relicID := fallbackID(fieldString(option, "relicId"), fieldString(option, "id"))
		payload.RelicOptions = append(payload.RelicOptions, indexedLabelFingerprint{
			Index: fieldIntValue(option, "index"),
			ID:    relicID,
			Label: fingerprintDisplayLabel(relicID, fieldString(option, "name")),
		})
	}
	return payload
}

func buildCharacterDecisionFingerprint(state *game.StateSnapshot) *characterDecisionFingerprint {
	payload := &characterDecisionFingerprint{
		SelectedCharacterID: fieldString(state.CharacterSelect, "selectedCharacterId"),
	}
	for _, character := range nestedList(state.CharacterSelect, "characters") {
		payload.Characters = append(payload.Characters, characterOptionFingerprint{
			Index:       fieldIntValue(character, "index"),
			CharacterID: fieldString(character, "characterId"),
			IsLocked:    fieldBool(character, "isLocked"),
			IsSelected:  fieldBool(character, "isSelected"),
			IsRandom:    fieldBool(character, "isRandom"),
		})
	}
	return payload
}

func buildGameOverDecisionFingerprint(state *game.StateSnapshot) *gameOverDecisionFingerprint {
	return &gameOverDecisionFingerprint{
		IsVictory: fieldBool(state.GameOver, "isVictory"),
		Floor:     fieldIntValue(state.GameOver, "floor"),
		KilledBy:  fieldString(state.GameOver, "killedBy"),
	}
}

func fieldIntValue(root map[string]any, key string) int {
	value, _ := fieldInt(root, key)
	return value
}

func jsonInt(value int) string {
	bytes, _ := json.Marshal(value)
	return string(bytes)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func fingerprintDisplayLabel(id string, label string) string {
	if strings.TrimSpace(id) != "" {
		return ""
	}
	return strings.TrimSpace(label)
}
