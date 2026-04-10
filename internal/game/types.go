package game

// ── HTTP Envelope ─────────────────────────────────────────────

type Envelope[T any] struct {
	OK        bool           `json:"ok"`
	RequestID string         `json:"request_id"`
	Data      T              `json:"data"`
	Error     *EnvelopeError `json:"error"`
}

type EnvelopeError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"retryable"`
}

// ── Health ────────────────────────────────────────────────────

type Health struct {
	BridgeVersion string `json:"bridgeVersion"`
	GameVersion   string `json:"gameVersion"`
	GameBuildDate string `json:"gameBuildDate"`
	Port          int    `json:"port"`
	Ready         bool   `json:"ready"`
}

// ── State Snapshot ────────────────────────────────────────────

type StateSnapshot struct {
	StateVersion     int              `json:"stateVersion"`
	RunID            string           `json:"runId"`
	Screen           string           `json:"screen"`
	InCombat         bool             `json:"inCombat"`
	Turn             *int             `json:"turn"`
	AvailableActions []string         `json:"availableActions"`
	Session          *SessionState    `json:"session"`
	Run              *RunState        `json:"run"`
	Combat           *CombatState     `json:"combat"`
	Map              *MapState        `json:"map"`
	Selection        *SelectionState  `json:"selection"`
	Reward           *RewardState     `json:"reward"`
	Event            *EventState      `json:"event"`
	CharacterSelect  *CharSelectState `json:"characterSelect"`
	Chest            *ChestState      `json:"chest"`
	Shop             *ShopState       `json:"shop"`
	Rest             *RestState       `json:"rest"`
	Modal            *ModalState      `json:"modal"`
	GameOver         *GameOverState   `json:"gameOver"`
	AgentView        *AgentViewState  `json:"agentView"`
	Multiplayer      map[string]any   `json:"multiplayer"`
	MultiplayerLobby map[string]any   `json:"multiplayerLobby"`
	Timeline         map[string]any   `json:"timeline"`
}

// ── Session ───────────────────────────────────────────────────

type SessionState struct {
	Mode         string `json:"mode"`
	Phase        string `json:"phase"`
	ControlScope string `json:"controlScope"`
}

// ── Run ───────────────────────────────────────────────────────

type RunState struct {
	Character  string           `json:"character"`
	Floor      int              `json:"floor"`
	CurrentHp  int              `json:"currentHp"`
	MaxHp      int              `json:"maxHp"`
	Gold       int              `json:"gold"`
	MaxEnergy  int              `json:"maxEnergy"`
	DeckCount  int              `json:"deckCount"`
	RelicCount int              `json:"relicCount"`
	PotionCount int             `json:"potionCount"`
	Deck       []RunCard        `json:"deck"`
	Relics     []RunRelic       `json:"relics"`
	Potions    []RunPotion      `json:"potions"`
}

type RunCard struct {
	CardID string `json:"cardId"`
	Name   string `json:"name"`
}

type RunRelic struct {
	RelicID string `json:"relicId"`
	Name    string `json:"name"`
}

type RunPotion struct {
	PotionID string `json:"potionId"`
	Name     string `json:"name"`
	IsEmpty  bool   `json:"isEmpty"`
}

// ── Combat ────────────────────────────────────────────────────

type CombatState struct {
	ActionWindowOpen      bool        `json:"actionWindowOpen"`
	IsInCardPlay          bool        `json:"isInCardPlay"`
	IsInCardSelection     bool        `json:"isInCardSelection"`
	PlayerActionsDisabled bool        `json:"playerActionsDisabled"`
	IsOverOrEnding        bool        `json:"isOverOrEnding"`
	Player                PlayerState `json:"player"`
	Enemies               []EnemyState `json:"enemies"`
	Hand                  []CardState  `json:"hand"`
	DrawPileCount         int          `json:"drawPileCount"`
	DiscardPileCount      int          `json:"discardPileCount"`
	ExhaustPileCount      int          `json:"exhaustPileCount"`
	DrawPile              []CardState  `json:"drawPile"`
	DiscardPile           []CardState  `json:"discardPile"`
	ExhaustPile           []CardState  `json:"exhaustPile"`
}

type PlayerState struct {
	CurrentHp int          `json:"currentHp"`
	MaxHp     int          `json:"maxHp"`
	Block     int          `json:"block"`
	Energy    int          `json:"energy"`
	Stars     int          `json:"stars"`
	Powers    []PowerState `json:"powers"`
}

type EnemyState struct {
	Index     int           `json:"index"`
	EnemyID   string        `json:"enemyId"`
	Name      string        `json:"name"`
	CurrentHp int           `json:"currentHp"`
	MaxHp     int           `json:"maxHp"`
	Block     int           `json:"block"`
	IsAlive   bool          `json:"isAlive"`
	IsHittable bool         `json:"isHittable"`
	MoveID    string        `json:"moveId"`
	Powers    []PowerState  `json:"powers"`
	Intents   []IntentState `json:"intents"`
}

type IntentState struct {
	Index           int    `json:"index"`
	IntentType      string `json:"intentType"`
	Label           string `json:"label"`
	Damage          *int   `json:"damage"`
	Hits            *int   `json:"hits"`
	TotalDamage     *int   `json:"totalDamage"`
	StatusCardCount *int   `json:"statusCardCount"`
}

type PowerState struct {
	PowerID   string `json:"powerId"`
	Name      string `json:"name"`
	Amount    int    `json:"amount"`
	PowerType string `json:"powerType"`
}

type CardState struct {
	Index              int    `json:"index"`
	CardID             string `json:"cardId"`
	Name               string `json:"name"`
	EnergyCost         *int   `json:"energyCost"`
	StarCost           *int   `json:"starCost"`
	CostsX             bool   `json:"costsX"`
	StarCostsX         bool   `json:"starCostsX"`
	TargetType         string `json:"targetType"`
	RequiresTarget     bool   `json:"requiresTarget"`
	Playable           bool   `json:"playable"`
	IsSelected         *bool  `json:"isSelected"`
	ValidTargetIndices []int  `json:"validTargetIndices"`
}

// ── Map ───────────────────────────────────────────────────────

type MapState struct {
	CurrentNode     *MapNode  `json:"currentNode"`
	IsTravelEnabled bool      `json:"isTravelEnabled"`
	IsTraveling     bool      `json:"isTraveling"`
	AvailableNodes  []MapNode `json:"availableNodes"`
}

type MapNode struct {
	Index    int    `json:"index"`
	Row      *int   `json:"row"`
	Col      *int   `json:"col"`
	NodeType string `json:"nodeType"`
}

// ── Selection ─────────────────────────────────────────────────

type SelectionState struct {
	Kind                  string      `json:"kind"`
	SourceScreen          string      `json:"sourceScreen"`
	SourceHint            string      `json:"sourceHint"`
	Mode                  string      `json:"mode"`
	IsCombatEmbedded      bool        `json:"isCombatEmbedded"`
	Prompt                string      `json:"prompt"`
	MinSelection          int         `json:"minSelection"`
	MaxSelection          int         `json:"maxSelection"`
	CurrentSelection      int         `json:"currentSelection"`
	RequiresConfirmation  bool        `json:"requiresConfirmation"`
	CanConfirm            bool        `json:"canConfirm"`
	Cards                 []CardState `json:"cards"`
}

// ── Reward ────────────────────────────────────────────────────

type RewardState struct {
	Phase              string         `json:"phase"`
	SourceScreen       string         `json:"sourceScreen"`
	SourceHint         string         `json:"sourceHint"`
	ScreenType         string         `json:"screenType"`
	PendingCardChoice  bool           `json:"pendingCardChoice"`
	CanProceed         bool           `json:"canProceed"`
	RewardCount        int            `json:"rewardCount"`
	ClaimableRewardCount int          `json:"claimableRewardCount"`
	AlternativeCount   int            `json:"alternativeCount"`
	Rewards            []RewardItem   `json:"rewards"`
	CardOptions        []CardState    `json:"cardOptions"`
	Alternatives       []RewardItem   `json:"alternatives"`
}

type RewardItem struct {
	Index         int    `json:"index"`
	RewardType    string `json:"rewardType"`
	Description   string `json:"description"`
	Claimable     bool   `json:"claimable"`
	BlockedReason string `json:"blockedReason"`
}

// ── Event ─────────────────────────────────────────────────────

type EventState struct {
	EventID     string        `json:"eventId"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Options     []EventOption `json:"options"`
	IsFinished  bool          `json:"isFinished"`
}

type EventOption struct {
	Index       int    `json:"index"`
	Title       string `json:"title"`
	Description string `json:"description"`
	IsLocked    bool   `json:"isLocked"`
	IsProceed   bool   `json:"isProceed"`
}

// ── Character Select ──────────────────────────────────────────

type CharSelectState struct {
	SelectedCharacter string             `json:"selectedCharacter"`
	Characters        []CharSelectOption `json:"characters"`
	CanEmbark         bool               `json:"canEmbark"`
}

type CharSelectOption struct {
	Index      int    `json:"index"`
	CharID     string `json:"characterId"`
	Name       string `json:"name"`
	IsLocked   bool   `json:"isLocked"`
	IsSelected bool   `json:"isSelected"`
	IsRandom   bool   `json:"isRandom"`
}

// ── Chest ─────────────────────────────────────────────────────

type ChestState struct {
	IsOpened    bool          `json:"isOpened"`
	RelicOptions []ChestRelic `json:"relicOptions"`
	CanProceed  bool          `json:"canProceed"`
}

type ChestRelic struct {
	Index   int    `json:"index"`
	RelicID string `json:"relicId"`
	Name    string `json:"name"`
	Rarity  string `json:"rarity"`
}

// ── Shop ──────────────────────────────────────────────────────

type ShopState struct {
	IsInventoryOpen   bool           `json:"isInventoryOpen"`
	CanOpenInventory  bool           `json:"canOpenInventory"`
	CanCloseInventory bool           `json:"canCloseInventory"`
	Cards             []ShopCard     `json:"cards"`
	Relics            []ShopRelic    `json:"relics"`
	Potions           []ShopPotion   `json:"potions"`
	CardRemoval       *ShopRemoval   `json:"cardRemoval"`
}

type ShopCard struct {
	Index      int    `json:"index"`
	Category   string `json:"category"`
	CardID     string `json:"cardId"`
	Name       string `json:"name"`
	Price      int    `json:"price"`
	IsStocked  bool   `json:"isStocked"`
	IsOnSale   bool   `json:"isOnSale"`
	EnoughGold bool   `json:"enoughGold"`
}

type ShopRelic struct {
	Index      int    `json:"index"`
	RelicID    string `json:"relicId"`
	Name       string `json:"name"`
	Price      int    `json:"price"`
	Rarity     string `json:"rarity"`
	IsStocked  bool   `json:"isStocked"`
	EnoughGold bool   `json:"enoughGold"`
}

type ShopPotion struct {
	Index      int    `json:"index"`
	PotionID   string `json:"potionId"`
	Name       string `json:"name"`
	Price      int    `json:"price"`
	IsStocked  bool   `json:"isStocked"`
	EnoughGold bool   `json:"enoughGold"`
}

type ShopRemoval struct {
	Price      int  `json:"price"`
	IsStocked  bool `json:"isStocked"`
	EnoughGold bool `json:"enoughGold"`
}

// ── Rest ──────────────────────────────────────────────────────

type RestState struct {
	Options []RestOption `json:"options"`
}

type RestOption struct {
	Index       int    `json:"index"`
	OptionID    string `json:"optionId"`
	Title       string `json:"title"`
	Description string `json:"description"`
	IsEnabled   bool   `json:"isEnabled"`
}

// ── Modal ─────────────────────────────────────────────────────

type ModalState struct {
	ModalType       string `json:"modalType"`
	UnderlyingScreen string `json:"underlyingScreen"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	ConfirmLabel    string `json:"confirmLabel"`
	DismissLabel    string `json:"dismissLabel"`
	CanConfirm      bool   `json:"canConfirm"`
	CanDismiss      bool   `json:"canDismiss"`
}

// ── Game Over ─────────────────────────────────────────────────

type GameOverState struct {
	Stage       string `json:"stage"`
	Victory     bool   `json:"victory"`
	Floor       int    `json:"floor"`
	CharacterID string `json:"characterId"`
	CanContinue bool   `json:"canContinue"`
	CanReturn   bool   `json:"canReturn"`
}

// ── Agent View ────────────────────────────────────────────────

type AgentViewState struct {
	Headline           string `json:"headline"`
	Floor              *int   `json:"floor"`
	Turn               *int   `json:"turn"`
	AvailableActionCount int  `json:"availableActionCount"`
	HandCount          *int   `json:"handCount"`
	EnemyCount         *int   `json:"enemyCount"`
}

// ── Actions ───────────────────────────────────────────────────

type MarkdownState struct {
	Format   string        `json:"format"`
	Markdown string        `json:"markdown"`
	Snapshot StateSnapshot `json:"snapshot"`
}

type ActionDescriptor struct {
	Action             string   `json:"action"`
	Description        string   `json:"description"`
	RequiredParameters []string `json:"requiredParameters"`
	OptionalParameters []string `json:"optionalParameters"`
}

type AvailableActions struct {
	Screen           string             `json:"screen"`
	AvailableActions []string           `json:"availableActions"`
	Descriptors      []ActionDescriptor `json:"descriptors"`
}

type ActionRequest struct {
	Action      string `json:"action"`
	CardIndex   *int   `json:"card_index,omitempty"`
	TargetIndex *int   `json:"target_index,omitempty"`
	OptionIndex *int   `json:"option_index,omitempty"`
}

type ActionResult struct {
	Action  string        `json:"action"`
	Status  string        `json:"status"`
	Stable  bool          `json:"stable"`
	Message string        `json:"message"`
	State   StateSnapshot `json:"state"`
}

// ── SSE Events ────────────────────────────────────────────────

type BridgeEvent struct {
	Type             string         `json:"type"`
	Sequence         int64          `json:"sequence"`
	TimestampUTC     string         `json:"timestampUtc"`
	RunID            string         `json:"runId"`
	Screen           string         `json:"screen"`
	InCombat         bool           `json:"inCombat"`
	Turn             *int           `json:"turn"`
	AvailableActions []string       `json:"availableActions"`
	Data             map[string]any `json:"data"`
}
