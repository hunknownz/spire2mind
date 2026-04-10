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
	if state == nil || state.Combat == nil {
		return &combatDecisionFingerprint{}
	}
	player := state.Combat.Player
	payload := &combatDecisionFingerprint{
		Player: combatPlayerFingerprint{
			Energy: player.Energy,
			Block:  player.Block,
			Stars:  player.Stars,
		},
	}

	for _, card := range state.Combat.Hand {
		cardID := fallbackID(card.CardID, "")
		energyCost := 0
		if card.EnergyCost != nil {
			energyCost = *card.EnergyCost
		}
		payload.Hand = append(payload.Hand, combatCardFingerprint{
			Index:          card.Index,
			CardID:         cardID,
			Name:           fingerprintDisplayLabel(cardID, card.Name),
			EnergyCost:     energyCost,
			Playable:       card.Playable,
			RequiresTarget: cardRequiresTarget(state, card),
			ValidTargets:   append([]int(nil), card.ValidTargetIndices...),
		})
	}

	for _, enemy := range state.Combat.Enemies {
		enemyID := fallbackID(enemy.EnemyID, "")
		fingerprint := combatEnemyFingerprint{
			Index:     enemy.Index,
			EnemyID:   enemyID,
			Name:      fingerprintDisplayLabel(enemyID, enemy.Name),
			CurrentHP: enemy.CurrentHp,
			Block:     enemy.Block,
			Hittable:  enemy.IsHittable,
		}
		for _, intent := range enemy.Intents {
			intentType := intent.IntentType
			label := intent.Label
			damage := 0
			if intent.TotalDamage != nil {
				damage = *intent.TotalDamage
			} else if intent.Damage != nil {
				damage = *intent.Damage
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
	payload := &rewardDecisionFingerprint{}
	if state.Reward != nil {
		payload.Phase = state.Reward.Phase
		payload.SourceScreen = state.Reward.SourceScreen
		payload.SourceHint = state.Reward.SourceHint
		payload.PendingCardChoice = state.Reward.PendingCardChoice
		payload.CanProceed = state.Reward.CanProceed
		for _, reward := range state.Reward.Rewards {
			payload.Rewards = append(payload.Rewards, indexedFlagFingerprint{
				Index:     reward.Index,
				Label:     reward.RewardType,
				Claimable: reward.Claimable,
			})
		}
		for _, option := range state.Reward.CardOptions {
			cardID := fallbackID(option.CardID, "")
			payload.CardOptions = append(payload.CardOptions, indexedLabelFingerprint{
				Index: option.Index,
				ID:    cardID,
				Label: fingerprintDisplayLabel(cardID, option.Name),
			})
		}
	}
	return payload
}

func buildMapDecisionFingerprint(state *game.StateSnapshot) *mapDecisionFingerprint {
	payload := &mapDecisionFingerprint{}
	if state.Map == nil {
		return payload
	}
	payload.Traveling = state.Map.IsTraveling
	if state.Map.CurrentNode != nil {
		cn := state.Map.CurrentNode
		fp := mapNodeFingerprint{
			Index:    cn.Index,
			NodeType: cn.NodeType,
		}
		if cn.Row != nil {
			fp.Row = *cn.Row
		}
		if cn.Col != nil {
			fp.Col = *cn.Col
		}
		payload.CurrentNode = &fp
	}
	for _, node := range state.Map.AvailableNodes {
		fp := mapNodeFingerprint{
			Index:    node.Index,
			NodeType: node.NodeType,
		}
		if node.Row != nil {
			fp.Row = *node.Row
		}
		if node.Col != nil {
			fp.Col = *node.Col
		}
		payload.AvailableNodes = append(payload.AvailableNodes, fp)
	}
	return payload
}

func buildEventDecisionFingerprint(state *game.StateSnapshot) *eventDecisionFingerprint {
	payload := &eventDecisionFingerprint{}
	if state.Event == nil {
		return payload
	}
	payload.IsFinished = state.Event.IsFinished
	payload.Title = state.Event.Title
	for _, option := range state.Event.Options {
		payload.Options = append(payload.Options, eventOptionFingerprint{
			Index:     option.Index,
			Label:     fallbackID(option.Title, ""),
			IsLocked:  option.IsLocked,
			IsProceed: option.IsProceed,
		})
	}
	return payload
}

func buildSelectionDecisionFingerprint(state *game.StateSnapshot) *selectionDecisionFingerprint {
	payload := &selectionDecisionFingerprint{}
	if state.Selection == nil {
		return payload
	}
	payload.Kind = state.Selection.Kind
	payload.SourceScreen = state.Selection.SourceScreen
	payload.SourceHint = state.Selection.SourceHint
	payload.Mode = state.Selection.Mode
	payload.RequiresConfirmation = state.Selection.RequiresConfirmation
	payload.CanConfirm = state.Selection.CanConfirm
	for _, card := range state.Selection.Cards {
		cardID := fallbackID(card.CardID, "")
		payload.Cards = append(payload.Cards, indexedLabelFingerprint{
			Index: card.Index,
			ID:    cardID,
			Label: fingerprintDisplayLabel(cardID, card.Name),
		})
	}
	return payload
}

func buildShopDecisionFingerprint(state *game.StateSnapshot) *shopDecisionFingerprint {
	payload := &shopDecisionFingerprint{}
	if state.Shop == nil {
		return payload
	}
	for _, card := range state.Shop.Cards {
		optionID := fallbackID(card.CardID, "")
		payload.Cards = append(payload.Cards, pricedOptionFingerprint{
			Index:      card.Index,
			ID:         optionID,
			Label:      fingerprintDisplayLabel(optionID, card.Name),
			Price:      card.Price,
			EnoughGold: card.EnoughGold,
		})
	}
	for _, relic := range state.Shop.Relics {
		optionID := fallbackID(relic.RelicID, "")
		payload.Relics = append(payload.Relics, pricedOptionFingerprint{
			Index:      relic.Index,
			ID:         optionID,
			Label:      fingerprintDisplayLabel(optionID, relic.Name),
			Price:      relic.Price,
			EnoughGold: relic.EnoughGold,
		})
	}
	for _, potion := range state.Shop.Potions {
		optionID := fallbackID(potion.PotionID, "")
		payload.Potions = append(payload.Potions, pricedOptionFingerprint{
			Index:      potion.Index,
			ID:         optionID,
			Label:      fingerprintDisplayLabel(optionID, potion.Name),
			Price:      potion.Price,
			EnoughGold: potion.EnoughGold,
		})
	}
	if state.Shop.CardRemoval != nil {
		payload.CardRemoval = &pricedFlagFingerprint{
			Price:      state.Shop.CardRemoval.Price,
			Available:  state.Shop.CardRemoval.IsStocked,
			EnoughGold: state.Shop.CardRemoval.EnoughGold,
		}
	}
	return payload
}

func buildRestDecisionFingerprint(state *game.StateSnapshot) *restDecisionFingerprint {
	payload := &restDecisionFingerprint{}
	if state.Rest == nil {
		return payload
	}
	for _, option := range state.Rest.Options {
		payload.Options = append(payload.Options, restOptionFingerprint{
			Index:      option.Index,
			OptionType: option.OptionID,
			Title:      option.Title,
			IsEnabled:  option.IsEnabled,
		})
	}
	return payload
}

func buildChestDecisionFingerprint(state *game.StateSnapshot) *chestDecisionFingerprint {
	payload := &chestDecisionFingerprint{}
	if state.Chest == nil {
		return payload
	}
	payload.IsOpened = state.Chest.IsOpened
	for _, option := range state.Chest.RelicOptions {
		relicID := fallbackID(option.RelicID, "")
		payload.RelicOptions = append(payload.RelicOptions, indexedLabelFingerprint{
			Index: option.Index,
			ID:    relicID,
			Label: fingerprintDisplayLabel(relicID, option.Name),
		})
	}
	return payload
}

func buildCharacterDecisionFingerprint(state *game.StateSnapshot) *characterDecisionFingerprint {
	payload := &characterDecisionFingerprint{}
	if state.CharacterSelect == nil {
		return payload
	}
	payload.SelectedCharacterID = state.CharacterSelect.SelectedCharacter
	for _, character := range state.CharacterSelect.Characters {
		payload.Characters = append(payload.Characters, characterOptionFingerprint{
			Index:       character.Index,
			CharacterID: character.CharID,
			IsLocked:    character.IsLocked,
			IsSelected:  character.IsSelected,
			IsRandom:    character.IsRandom,
		})
	}
	return payload
}

func buildGameOverDecisionFingerprint(state *game.StateSnapshot) *gameOverDecisionFingerprint {
	if state.GameOver == nil {
		return &gameOverDecisionFingerprint{}
	}
	return &gameOverDecisionFingerprint{
		IsVictory: state.GameOver.Victory,
		Floor:     state.GameOver.Floor,
		KilledBy:  "", // GameOverState doesn't have KilledBy
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
