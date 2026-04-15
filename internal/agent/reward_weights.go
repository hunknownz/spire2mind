package agentruntime

import (
	"encoding/json"
	"os"
	"sync"
)

// RewardWeights holds all tunable parameters for reward card scoring.
// Loaded from combat/knowledge/reward-weights.json and modifiable by evolution.
type RewardWeights struct {
	mu sync.RWMutex

	KnowledgeScoreMultiplier float64 `json:"knowledge_score_multiplier"`

	TimingModifiers struct {
		EarlyFloorThreshold         float64 `json:"early_floor_threshold"`
		LateFloorThreshold          float64 `json:"late_floor_threshold"`
		EarlyCardEarlyFloorBonus    float64 `json:"early_card_early_floor_bonus"`
		EarlyCardLateFloorPenalty   float64 `json:"early_card_late_floor_penalty"`
		LateCardLateFloorBonus      float64 `json:"late_card_late_floor_bonus"`
		LateCardEarlyFloorPenalty   float64 `json:"late_card_early_floor_penalty"`
	} `json:"timing_modifiers"`

	DefenseModifiers struct {
		LowHPThreshold       float64 `json:"low_hp_threshold"`
		DefenseLowHPBonus    float64 `json:"defense_low_hp_bonus"`
		ScalingLowHPPenalty  float64 `json:"scaling_low_hp_penalty"`
	} `json:"defense_modifiers"`

	BasicCardPenalty float64 `json:"basic_card_penalty"`

	CardSpecificBonuses struct {
		StrongCommons struct {
			Cards []string `json:"cards"`
			Bonus float64  `json:"bonus"`
		} `json:"strong_commons"`
		UtilityCommons struct {
			Cards []string `json:"cards"`
			Bonus float64  `json:"bonus"`
		} `json:"utility_commons"`
		ScalingAntiSynergy struct {
			Cards []string `json:"cards"`
			Bonus float64  `json:"bonus"`
		} `json:"scaling_anti_synergy"`
		BuildAround struct {
			Cards []string `json:"cards"`
			Bonus float64  `json:"bonus"`
		} `json:"build_around"`
		ScalingLowHPExtraPenalty struct {
			Cards []string `json:"cards"`
			Bonus float64  `json:"bonus"`
		} `json:"scaling_low_hp_extra_penalty"`
	} `json:"card_specific_bonuses"`

	EarlyActBonuses struct {
		FloorThreshold float64            `json:"floor_threshold"`
		CardBonuses    map[string]float64 `json:"card_bonuses"`
		KeywordBonuses struct {
			Block  float64 `json:"block"`
			Damage float64 `json:"damage"`
		} `json:"keyword_bonuses"`
	} `json:"early_act_bonuses"`

	SurvivalBonuses struct {
		HPThreshold  float64            `json:"hp_threshold"`
		CardBonuses  map[string]float64 `json:"card_bonuses"`
		KeywordBonus struct {
			Block float64 `json:"block"`
		} `json:"keyword_bonuses"`
	} `json:"survival_bonuses"`

	ImmediatePowerBonuses struct {
		FloorThreshold float64            `json:"floor_threshold"`
		CardBonuses    map[string]float64 `json:"card_bonuses"`
		KeywordBonuses struct {
			Draw   float64 `json:"draw"`
			Block  float64 `json:"block"`
			Damage float64 `json:"damage"`
		} `json:"keyword_bonuses"`
	} `json:"immediate_power_bonuses"`
}

// Global reward weights instance (set by Session on startup).
var globalRewardWeights *RewardWeights

// LoadRewardWeights loads reward weights from a JSON file.
func LoadRewardWeights(path string) (*RewardWeights, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var rw RewardWeights
	if err := json.Unmarshal(data, &rw); err != nil {
		return nil, err
	}

	return &rw, nil
}

// Reload reloads the weights from the same file path.
func (rw *RewardWeights) Reload(path string) error {
	newRW, err := LoadRewardWeights(path)
	if err != nil {
		return err
	}

	rw.mu.Lock()
	defer rw.mu.Unlock()
	*rw = *newRW
	return nil
}

// GetKnowledgeScoreMultiplier returns the knowledge score multiplier (thread-safe).
func (rw *RewardWeights) GetKnowledgeScoreMultiplier() float64 {
	rw.mu.RLock()
	defer rw.mu.RUnlock()
	return rw.KnowledgeScoreMultiplier
}

// GetTimingModifiers returns a copy of timing modifiers (thread-safe).
func (rw *RewardWeights) GetTimingModifiers() (earlyFloorThresh, lateFloorThresh float64,
	earlyEarlyBonus, earlyLatePenalty, lateLateBonus, lateEarlyPenalty float64) {
	rw.mu.RLock()
	defer rw.mu.RUnlock()
	return rw.TimingModifiers.EarlyFloorThreshold,
		rw.TimingModifiers.LateFloorThreshold,
		rw.TimingModifiers.EarlyCardEarlyFloorBonus,
		rw.TimingModifiers.EarlyCardLateFloorPenalty,
		rw.TimingModifiers.LateCardLateFloorBonus,
		rw.TimingModifiers.LateCardEarlyFloorPenalty
}

// GetDefenseModifiers returns defense modifiers (thread-safe).
func (rw *RewardWeights) GetDefenseModifiers() (lowHPThresh, defenseBonus, scalingPenalty float64) {
	rw.mu.RLock()
	defer rw.mu.RUnlock()
	return rw.DefenseModifiers.LowHPThreshold,
		rw.DefenseModifiers.DefenseLowHPBonus,
		rw.DefenseModifiers.ScalingLowHPPenalty
}

// GetBasicCardPenalty returns the basic card penalty (thread-safe).
func (rw *RewardWeights) GetBasicCardPenalty() float64 {
	rw.mu.RLock()
	defer rw.mu.RUnlock()
	return rw.BasicCardPenalty
}

// GetCardSpecificBonus returns the bonus for a specific card ID (thread-safe).
func (rw *RewardWeights) GetCardSpecificBonus(cardID string) float64 {
	rw.mu.RLock()
	defer rw.mu.RUnlock()

	// Check each card-specific bonus group
	for _, card := range rw.CardSpecificBonuses.StrongCommons.Cards {
		if cardID == card {
			return rw.CardSpecificBonuses.StrongCommons.Bonus
		}
	}
	for _, card := range rw.CardSpecificBonuses.UtilityCommons.Cards {
		if cardID == card {
			return rw.CardSpecificBonuses.UtilityCommons.Bonus
		}
	}
	for _, card := range rw.CardSpecificBonuses.ScalingAntiSynergy.Cards {
		if cardID == card {
			return rw.CardSpecificBonuses.ScalingAntiSynergy.Bonus
		}
	}
	for _, card := range rw.CardSpecificBonuses.BuildAround.Cards {
		if cardID == card {
			return rw.CardSpecificBonuses.BuildAround.Bonus
		}
	}
	for _, card := range rw.CardSpecificBonuses.ScalingLowHPExtraPenalty.Cards {
		if cardID == card {
			return rw.CardSpecificBonuses.ScalingLowHPExtraPenalty.Bonus
		}
	}

	return 0
}

// GetEarlyActBonus returns the early act bonus for a card (thread-safe).
func (rw *RewardWeights) GetEarlyActBonus(cardID, name string) float64 {
	rw.mu.RLock()
	defer rw.mu.RUnlock()

	if bonus, ok := rw.EarlyActBonuses.CardBonuses[cardID]; ok {
		return bonus
	}

	// Keyword bonuses
	if containsAny(name, "block") {
		return rw.EarlyActBonuses.KeywordBonuses.Block
	}
	if containsAny(name, "damage") {
		return rw.EarlyActBonuses.KeywordBonuses.Damage
	}

	return 0
}

// GetSurvivalBonus returns the survival bonus for a card (thread-safe).
func (rw *RewardWeights) GetSurvivalBonus(cardID, name string) float64 {
	rw.mu.RLock()
	defer rw.mu.RUnlock()

	if bonus, ok := rw.SurvivalBonuses.CardBonuses[cardID]; ok {
		return bonus
	}

	// Keyword bonuses
	if containsAny(name, "block") {
		return rw.SurvivalBonuses.KeywordBonus.Block
	}

	return 0
}

// GetImmediatePowerBonus returns the immediate power bonus for a card (thread-safe).
func (rw *RewardWeights) GetImmediatePowerBonus(cardID, name string) float64 {
	rw.mu.RLock()
	defer rw.mu.RUnlock()

	if bonus, ok := rw.ImmediatePowerBonuses.CardBonuses[cardID]; ok {
		return bonus
	}

	// Keyword bonuses
	if containsAny(name, "draw") {
		return rw.ImmediatePowerBonuses.KeywordBonuses.Draw
	}
	if containsAny(name, "block") {
		return rw.ImmediatePowerBonuses.KeywordBonuses.Block
	}
	if containsAny(name, "damage") {
		return rw.ImmediatePowerBonuses.KeywordBonuses.Damage
	}

	return 0
}

// GetEarlyActFloorThreshold returns the early act floor threshold (thread-safe).
func (rw *RewardWeights) GetEarlyActFloorThreshold() float64 {
	rw.mu.RLock()
	defer rw.mu.RUnlock()
	return rw.EarlyActBonuses.FloorThreshold
}

// GetSurvivalHPThreshold returns the survival HP threshold (thread-safe).
func (rw *RewardWeights) GetSurvivalHPThreshold() float64 {
	rw.mu.RLock()
	defer rw.mu.RUnlock()
	return rw.SurvivalBonuses.HPThreshold
}

// GetImmediatePowerFloorThreshold returns the immediate power floor threshold (thread-safe).
func (rw *RewardWeights) GetImmediatePowerFloorThreshold() float64 {
	rw.mu.RLock()
	defer rw.mu.RUnlock()
	return rw.ImmediatePowerBonuses.FloorThreshold
}
