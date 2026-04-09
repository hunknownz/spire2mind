package game

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

type Health struct {
	BridgeVersion string `json:"bridgeVersion"`
	GameVersion   string `json:"gameVersion"`
	GameBuildDate string `json:"gameBuildDate"`
	Port          int    `json:"port"`
	Ready         bool   `json:"ready"`
}

type StateSnapshot struct {
	StateVersion     int                    `json:"stateVersion"`
	RunID            string                 `json:"runId"`
	Screen           string                 `json:"screen"`
	InCombat         bool                   `json:"inCombat"`
	Turn             *int                   `json:"turn"`
	AvailableActions []string               `json:"availableActions"`
	Session          map[string]any         `json:"session"`
	Run              map[string]any         `json:"run"`
	Combat           map[string]any         `json:"combat"`
	Map              map[string]any         `json:"map"`
	Selection        map[string]any         `json:"selection"`
	Reward           map[string]any         `json:"reward"`
	Event            map[string]any         `json:"event"`
	CharacterSelect  map[string]any         `json:"characterSelect"`
	Chest            map[string]any         `json:"chest"`
	Shop             map[string]any         `json:"shop"`
	Rest             map[string]any         `json:"rest"`
	Modal            map[string]any         `json:"modal"`
	GameOver         map[string]any         `json:"gameOver"`
	AgentView        map[string]any         `json:"agentView"`
	Multiplayer      map[string]any         `json:"multiplayer"`
	MultiplayerLobby map[string]any         `json:"multiplayerLobby"`
	Timeline         map[string]any         `json:"timeline"`
}

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
