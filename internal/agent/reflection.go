package agentruntime

import (
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"spire2mind/internal/game"
)

type AttemptReflection struct {
	Time            time.Time               `json:"time"`
	Attempt         int                     `json:"attempt"`
	RunID           string                  `json:"run_id"`
	Outcome         string                  `json:"outcome"`
	Screen          string                  `json:"screen"`
	Floor           *int                    `json:"floor,omitempty"`
	CharacterID     string                  `json:"character_id,omitempty"`
	Headline        string                  `json:"headline,omitempty"`
	TacticalHints   []string                `json:"tactical_hints,omitempty"`
	FinalRoomDetail []string                `json:"final_room_detail,omitempty"`
	RecentTimeline  []string                `json:"recent_timeline,omitempty"`
	RecentFailures  []string                `json:"recent_failures,omitempty"`
	Successes       []string                `json:"successes,omitempty"`
	Risks           []string                `json:"risks,omitempty"`
	LessonBuckets   ReflectionLessonBuckets `json:"lesson_buckets,omitempty"`
	TacticalMistakes []string               `json:"tactical_mistakes,omitempty"`
	RuntimeNoise     []string               `json:"runtime_noise,omitempty"`
	ResourceMistakes []string               `json:"resource_mistakes,omitempty"`
	NextPlan        string                  `json:"next_plan,omitempty"`
	Story           string                  `json:"story"`
	Lessons         []string                `json:"lessons,omitempty"`
}

type ReflectionLessonBuckets struct {
	Pathing        []string `json:"pathing,omitempty"`
	RewardChoice   []string `json:"reward_choice,omitempty"`
	ShopEconomy    []string `json:"shop_economy,omitempty"`
	CombatSurvival []string `json:"combat_survival,omitempty"`
	Runtime        []string `json:"runtime,omitempty"`
}

func BuildAttemptReflection(attempt int, state *game.StateSnapshot, todo *TodoManager, compact *CompactMemory) *AttemptReflection {
	if state == nil {
		return nil
	}

	reflection := &AttemptReflection{
		Time:          time.Now(),
		Attempt:       attempt,
		RunID:         state.RunID,
		Outcome:       reflectionOutcome(state),
		Screen:        state.Screen,
		Floor:         reflectionFloor(state),
		CharacterID:   reflectionCharacterID(state),
		Headline:      reflectionHeadline(state),
		TacticalHints: append([]string(nil), buildReflectionTacticalHints(state)...),
	}
	if detail := buildReflectionRoomDetail(state); len(detail) > 0 {
		reflection.FinalRoomDetail = append([]string(nil), detail...)
	}
	if compact != nil {
		reflection.RecentTimeline = compact.RecentTimeline(6)
	}
	if todo != nil {
		reflection.RecentFailures = todo.FailureHistory(4)
	}

	reflection.Successes = buildReflectionSuccesses(state, reflection)
	reflection.Risks = buildReflectionRisks(state, reflection)
	reflection.LessonBuckets = buildReflectionLessonBuckets(state, reflection)
	reflection.TacticalMistakes, reflection.RuntimeNoise, reflection.ResourceMistakes = classifyReflectionFindings(reflection)
	reflection.Lessons = reflection.LessonBuckets.Flatten(10)
	reflection.NextPlan = buildReflectionNextPlan(reflection)
	reflection.Story = buildReflectionStory(reflection)
	return reflection
}

func (r *AttemptReflection) PromptSummary() string {
	if r == nil {
		return ""
	}

	parts := []string{}
	if r.Attempt > 0 {
		parts = append(parts, fmt.Sprintf("Attempt %d ended in %s", r.Attempt, r.Outcome))
	}
	if r.Floor != nil {
		parts = append(parts, fmt.Sprintf("floor %d", *r.Floor))
	}
	if r.CharacterID != "" {
		parts = append(parts, fmt.Sprintf("character %s", r.CharacterID))
	}
	if len(r.Lessons) > 0 {
		parts = append(parts, "lesson: "+r.Lessons[0])
	}

	return strings.Join(parts, " | ")
}

func buildReflectionStory(reflection *AttemptReflection) string {
	if reflection == nil {
		return ""
	}

	parts := []string{}
	lead := fmt.Sprintf("Attempt %d ended in %s", reflection.Attempt, reflection.Outcome)
	if reflection.Floor != nil {
		lead += fmt.Sprintf(" on floor %d", *reflection.Floor)
	}
	if reflection.CharacterID != "" {
		lead += fmt.Sprintf(" with %s", reflection.CharacterID)
	}
	parts = append(parts, lead+".")

	if reflection.Headline != "" {
		parts = append(parts, reflection.Headline+".")
	}
	if len(reflection.TacticalHints) > 0 {
		parts = append(parts, "Final tactical picture: "+strings.Join(reflection.TacticalHints, " ")+".")
	}
	if len(reflection.FinalRoomDetail) > 0 {
		parts = append(parts, "Final board: "+strings.Join(cleanReflectionLines(reflection.FinalRoomDetail), " | ")+".")
	}
	if len(reflection.RecentTimeline) > 0 {
		parts = append(parts, "Closing sequence: "+strings.Join(reflection.RecentTimeline, " -> ")+".")
	}
	if len(reflection.RecentFailures) > 0 {
		parts = append(parts, "Main friction: "+strings.Join(reflection.RecentFailures, " | ")+".")
	}
	if len(reflection.TacticalMistakes) > 0 {
		parts = append(parts, "Tactical mistakes: "+strings.Join(reflection.TacticalMistakes, " ")+".")
	}
	if len(reflection.RuntimeNoise) > 0 {
		parts = append(parts, "Runtime noise: "+strings.Join(reflection.RuntimeNoise, " ")+".")
	}
	if len(reflection.ResourceMistakes) > 0 {
		parts = append(parts, "Resource mistakes: "+strings.Join(reflection.ResourceMistakes, " ")+".")
	}
	if len(reflection.Successes) > 0 {
		parts = append(parts, "What worked: "+strings.Join(reflection.Successes, " ")+".")
	}
	if len(reflection.Risks) > 0 {
		parts = append(parts, "What hurt: "+strings.Join(reflection.Risks, " ")+".")
	}
	if len(reflection.Lessons) > 0 {
		parts = append(parts, "Next time: "+strings.Join(reflection.Lessons, " ")+".")
	}
	if reflection.NextPlan != "" {
		parts = append(parts, "Carry-forward plan: "+reflection.NextPlan+".")
	}

	return strings.Join(parts, " ")
}

func cleanReflectionLines(lines []string) []string {
	cleaned := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(strings.TrimPrefix(line, "- "))
		line = strings.ReplaceAll(line, "`", "")
		if line == "" {
			continue
		}
		cleaned = append(cleaned, line)
	}
	return cleaned
}

func buildReflectionTacticalHints(state *game.StateSnapshot) []string {
	if state == nil {
		return nil
	}

	if hints := BuildTacticalHints(state); len(hints) > 0 {
		return hints
	}

	if strings.EqualFold(state.Screen, "GAME_OVER") && len(state.Combat) > 0 {
		combatState := *state
		combatState.Screen = "COMBAT"
		return BuildTacticalHints(&combatState)
	}

	return nil
}

func buildReflectionRoomDetail(state *game.StateSnapshot) []string {
	if state == nil {
		return nil
	}

	if detail := StateDetailLines(state, 4); len(detail) > 0 && !(len(detail) == 1 && detail[0] == "- -") {
		return detail
	}

	if strings.EqualFold(state.Screen, "GAME_OVER") && len(state.Combat) > 0 {
		combatState := *state
		combatState.Screen = "COMBAT"
		if detail := StateDetailLines(&combatState, 4); len(detail) > 0 && !(len(detail) == 1 && detail[0] == "- -") {
			return detail
		}
	}

	return nil
}

func buildReflectionSuccesses(state *game.StateSnapshot, reflection *AttemptReflection) []string {
	if state == nil || reflection == nil {
		return nil
	}

	successes := []string{}
	floor := 0
	if reflection.Floor != nil {
		floor = *reflection.Floor
	}

	switch {
	case floor >= 30:
		successes = append(successes, fmt.Sprintf("Deep run to floor %d — the deck and pathing carried well past the midgame boss", floor))
	case floor >= 20:
		successes = append(successes, fmt.Sprintf("Reached floor %d, proving the build could handle Act 1 and push into mid-Act 2", floor))
	case floor >= 14:
		successes = append(successes, fmt.Sprintf("Reached floor %d — the combat loop held through most of Act 1", floor))
	case floor >= 6:
		successes = append(successes, fmt.Sprintf("Cleared %d floors and built some deck momentum before failing", floor))
	case floor >= 2:
		successes = append(successes, "Got through multiple early encounters without structural stalls")
	}

	if len(reflection.RecentFailures) == 0 {
		successes = append(successes, "The final rooms showed clean automation with no stale-index thrashing")
	}

	if gold, ok := fieldInt(state.Run, "gold"); ok && gold >= 90 {
		successes = append(successes, fmt.Sprintf("Accumulated %d gold — economy and reward progression were healthy", gold))
	}

	if deckSize := countDeckSize(state); deckSize > 0 && deckSize <= 20 && floor >= 10 {
		successes = append(successes, fmt.Sprintf("Kept a lean deck (%d cards) which helps consistency", deckSize))
	}

	if strings.EqualFold(reflection.Outcome, "victory") {
		successes = append(successes, "The route, deck shaping, and combat pacing were strong enough to finish the run")
	}

	return successes
}

func buildReflectionRisks(state *game.StateSnapshot, reflection *AttemptReflection) []string {
	if state == nil || reflection == nil {
		return nil
	}

	risks := []string{}
	if len(reflection.RecentFailures) > 0 {
		risks = append(risks, "Fast transitions caused friction that forced runtime recovery instead of clean flow")
	}

	currentHP, okCurrent := fieldInt(state.Run, "currentHp")
	maxHP, okMax := fieldInt(state.Run, "maxHp")
	if okCurrent && okMax && maxHP > 0 {
		hpPercent := currentHP * 100 / maxHP
		switch {
		case hpPercent <= 10:
			risks = append(risks, fmt.Sprintf("Died at critical HP (%d/%d) — the run collapsed with no safety margin left", currentHP, maxHP))
		case hpPercent <= 35:
			risks = append(risks, fmt.Sprintf("Low-health spiral (%d/%d) — pathing and reward choices needed to be more defensive sooner", currentHP, maxHP))
		}
	}

	if gold, ok := fieldInt(state.Run, "gold"); ok && gold >= 80 && strings.EqualFold(reflection.Outcome, "defeat") {
		risks = append(risks, fmt.Sprintf("Died with %d unspent gold — convert resources into survivability or power earlier", gold))
	}

	if deckSize := countDeckSize(state); deckSize > 30 {
		risks = append(risks, fmt.Sprintf("Bloated deck (%d cards) may have reduced draw consistency", deckSize))
	}

	floor := 0
	if reflection.Floor != nil {
		floor = *reflection.Floor
	}
	if floor <= 6 && strings.EqualFold(reflection.Outcome, "defeat") {
		risks = append(risks, "Early death before floor 7 — the opening card picks or pathing were too fragile for the first elite/boss")
	}

	return risks
}

func (b ReflectionLessonBuckets) Flatten(limit int) []string {
	if limit <= 0 {
		limit = 8
	}

	lessons := make([]string, 0, limit)
	appendGroup := func(items []string) {
		for _, item := range items {
			lessons = appendUniqueTrimmed(lessons, item, limit)
		}
	}

	appendGroup(b.CombatSurvival)
	appendGroup(b.Pathing)
	appendGroup(b.RewardChoice)
	appendGroup(b.ShopEconomy)
	appendGroup(b.Runtime)
	return lessons
}

func (b *ReflectionLessonBuckets) add(category string, lesson string) {
	lesson = strings.TrimSpace(lesson)
	if lesson == "" {
		return
	}

	switch category {
	case "pathing":
		b.Pathing = appendUniqueTrimmed(b.Pathing, lesson, 4)
	case "reward_choice":
		b.RewardChoice = appendUniqueTrimmed(b.RewardChoice, lesson, 4)
	case "shop_economy":
		b.ShopEconomy = appendUniqueTrimmed(b.ShopEconomy, lesson, 4)
	case "combat_survival":
		b.CombatSurvival = appendUniqueTrimmed(b.CombatSurvival, lesson, 4)
	case "runtime":
		b.Runtime = appendUniqueTrimmed(b.Runtime, lesson, 4)
	}
}

func buildReflectionLessonBuckets(state *game.StateSnapshot, reflection *AttemptReflection) ReflectionLessonBuckets {
	if state == nil || reflection == nil {
		return ReflectionLessonBuckets{}
	}

	buckets := ReflectionLessonBuckets{}

	if len(reflection.RecentFailures) > 0 {
		buckets.add("runtime", "Re-read the live state after fast transitions before repeating an indexed action.")
	}

	currentHP, okCurrent := fieldInt(state.Run, "currentHp")
	maxHP, okMax := fieldInt(state.Run, "maxHp")
	if okCurrent && okMax && maxHP > 0 && currentHP*100/maxHP <= 35 {
		buckets.add("combat_survival", "Play safer at low health: value block over damage and stop spending HP to race.")
		buckets.add("pathing", "At low health, bias toward shorter and safer routes with rest or lower-variance rooms.")
	}

	if gold, ok := fieldInt(state.Run, "gold"); ok && gold >= 80 && strings.EqualFold(reflection.Outcome, "defeat") {
		buckets.add("shop_economy", "Convert gold earlier: prioritize card removal, strong relics, or key shop cards before gold becomes useless.")
	}

	if deckSize := countDeckSize(state); deckSize > 30 {
		buckets.add("reward_choice", "Be more selective with card picks - a tighter deck draws key cards more reliably.")
	}

	floor := 0
	if reflection.Floor != nil {
		floor = *reflection.Floor
	}
	if floor <= 6 && strings.EqualFold(reflection.Outcome, "defeat") {
		buckets.add("reward_choice", "Prioritize early damage and defense cards over setup - the first floors demand immediate combat readiness.")
		buckets.add("pathing", "Treat the opening path as a setup lane: only take risky nodes when the deck already has enough early power.")
	}
	if floor >= 14 && floor <= 20 && strings.EqualFold(reflection.Outcome, "defeat") {
		buckets.add("reward_choice", "The midgame boss requires scaling or burst - pick at least one strong damage source before floor 17.")
	}

	if strings.EqualFold(reflection.Outcome, "victory") {
		buckets.add("pathing", "Preserve the lines that kept tempo high; reuse that route and room valuation logic in future runs.")
		buckets.add("combat_survival", "Reuse the turn patterns that preserved HP while still keeping enough damage pressure.")
	}

	if len(buckets.Flatten(10)) == 0 {
		buckets.add("runtime", "Keep momentum high, avoid stale indexes, and prefer simple legal progress when the board state is changing quickly.")
	}

	return buckets
}

func buildReflectionLessons(state *game.StateSnapshot, reflection *AttemptReflection) []string {
	if state == nil || reflection == nil {
		return nil
	}

	lessons := []string{}

	if len(reflection.RecentFailures) > 0 {
		lessons = append(lessons, "Re-read the live state after fast transitions before repeating an indexed action.")
	}

	currentHP, okCurrent := fieldInt(state.Run, "currentHp")
	maxHP, okMax := fieldInt(state.Run, "maxHp")
	if okCurrent && okMax && maxHP > 0 && currentHP*100/maxHP <= 35 {
		lessons = append(lessons, "Play safer at low health: value block over damage, pick shorter paths, and sequence rewards defensively.")
	}

	if gold, ok := fieldInt(state.Run, "gold"); ok && gold >= 80 && strings.EqualFold(reflection.Outcome, "defeat") {
		lessons = append(lessons, "Convert gold earlier: prioritize card removal, strong relics, or key shop cards before gold becomes useless.")
	}

	if deckSize := countDeckSize(state); deckSize > 30 {
		lessons = append(lessons, "Be more selective with card picks — a tighter deck draws key cards more reliably.")
	}

	floor := 0
	if reflection.Floor != nil {
		floor = *reflection.Floor
	}
	if floor <= 6 && strings.EqualFold(reflection.Outcome, "defeat") {
		lessons = append(lessons, "Prioritize early damage and defense cards over setup — the first floors demand immediate combat readiness.")
	}
	if floor >= 14 && floor <= 20 && strings.EqualFold(reflection.Outcome, "defeat") {
		lessons = append(lessons, "The midgame boss requires scaling or burst — consider picking at least one strong damage source before floor 17.")
	}

	if strings.EqualFold(reflection.Outcome, "victory") {
		lessons = append(lessons, "Preserve the lines that kept tempo high; reuse that route and card valuation logic in future runs.")
	}

	if len(lessons) == 0 {
		lessons = append(lessons, "Keep momentum high, avoid stale indexes, and prefer simple legal progress when the board state is changing quickly.")
	}

	return lessons
}

func buildReflectionNextPlan(reflection *AttemptReflection) string {
	if reflection == nil {
		return ""
	}

	steps := []string{}

	// Tailor the plan based on how far the run got.
	floor := 0
	if reflection.Floor != nil {
		floor = *reflection.Floor
	}
	switch {
	case floor <= 6:
		steps = append(steps, "focus on early combat readiness — pick damage and defense over setup cards")
	case floor <= 17:
		steps = append(steps, "stabilize the next few rooms before taking greedy lines")
	default:
		steps = append(steps, "maintain the scaling plan that got past the midgame and adapt for harder encounters")
	}

	if len(reflection.RecentFailures) > 0 {
		steps = append(steps, "double-check state after fast transitions and replan instead of repeating stale indexed actions")
	}

	// Add the most specific lesson (skip the first generic one if there are more).
	for _, lesson := range reflection.Lessons {
		trimmed := strings.TrimSuffix(lesson, ".")
		if !strings.EqualFold(trimmed, "Re-read the live state after fast transitions before repeating an indexed action") {
			steps = append(steps, trimmed)
			break
		}
	}

	if len(steps) == 0 {
		return ""
	}

	if len(steps) == 1 {
		return sentenceCap(steps[0])
	}

	return sentenceCap(strings.Join(steps[:len(steps)-1], ", ") + ", and " + steps[len(steps)-1])
}

func sentenceCap(input string) string {
	if input == "" {
		return ""
	}

	r, size := utf8.DecodeRuneInString(input)
	if r == utf8.RuneError && size == 0 {
		return input
	}

	return string(unicode.ToUpper(r)) + input[size:]
}

func reflectionOutcome(state *game.StateSnapshot) string {
	if state == nil {
		return "unknown"
	}

	if fieldBool(state.GameOver, "isVictory") {
		return "victory"
	}
	if strings.EqualFold(state.Screen, "GAME_OVER") {
		return "defeat"
	}

	return "unknown"
}

func reflectionFloor(state *game.StateSnapshot) *int {
	if state == nil {
		return nil
	}

	if floor, ok := fieldInt(state.GameOver, "floor"); ok {
		return &floor
	}
	if floor, ok := fieldInt(state.Run, "floor"); ok {
		return &floor
	}

	return nil
}

func reflectionCharacterID(state *game.StateSnapshot) string {
	if state == nil {
		return ""
	}

	if characterID := fieldString(state.GameOver, "characterId"); characterID != "" {
		return characterID
	}
	if characterID := fieldString(state.Run, "characterId"); characterID != "" {
		return characterID
	}

	return ""
}

func countDeckSize(state *game.StateSnapshot) int {
	if state == nil || state.Run == nil {
		return 0
	}
	raw, ok := state.Run["deck"]
	if !ok {
		return 0
	}
	// deck can be []any (card objects or strings) or []map[string]any.
	switch v := raw.(type) {
	case []any:
		return len(v)
	case []map[string]any:
		return len(v)
	default:
		return 0
	}
}

func reflectionHeadline(state *game.StateSnapshot) string {
	if state == nil || len(state.AgentView) == 0 {
		return ""
	}

	return fieldString(state.AgentView, "headline")
}

func classifyReflectionFindings(reflection *AttemptReflection) ([]string, []string, []string) {
	if reflection == nil {
		return nil, nil, nil
	}

	tactical := make([]string, 0, 4)
	runtime := make([]string, 0, 4)
	resource := make([]string, 0, 4)

	for _, failure := range reflection.RecentFailures {
		lower := strings.ToLower(strings.TrimSpace(failure))
		switch {
		case strings.Contains(lower, "drift_kind="), strings.Contains(lower, "invalid_action"), strings.Contains(lower, "invalid_target"):
			runtime = appendUniqueTrimmed(runtime, summarizeGuideFailure(failure), 4)
		default:
			tactical = appendUniqueTrimmed(tactical, strings.TrimSpace(failure), 4)
		}
	}

	for _, risk := range reflection.Risks {
		lower := strings.ToLower(strings.TrimSpace(risk))
		switch {
		case strings.Contains(lower, "gold"), strings.Contains(lower, "deck"), strings.Contains(lower, "pathing"):
			resource = appendUniqueTrimmed(resource, strings.TrimSpace(risk), 4)
		case strings.Contains(lower, "low-health"), strings.Contains(lower, "critical hp"), strings.Contains(lower, "safety margin"):
			tactical = appendUniqueTrimmed(tactical, strings.TrimSpace(risk), 4)
		default:
			resource = appendUniqueTrimmed(resource, strings.TrimSpace(risk), 4)
		}
	}

	if len(runtime) == 0 && len(reflection.LessonBuckets.Runtime) > 0 {
		runtime = appendUniqueTrimmed(runtime, reflection.LessonBuckets.Runtime[0], 4)
	}
	if len(tactical) == 0 && len(reflection.LessonBuckets.CombatSurvival) > 0 {
		tactical = appendUniqueTrimmed(tactical, reflection.LessonBuckets.CombatSurvival[0], 4)
	}
	if len(resource) == 0 {
		if len(reflection.LessonBuckets.RewardChoice) > 0 {
			resource = appendUniqueTrimmed(resource, reflection.LessonBuckets.RewardChoice[0], 4)
		}
		if len(reflection.LessonBuckets.ShopEconomy) > 0 {
			resource = appendUniqueTrimmed(resource, reflection.LessonBuckets.ShopEconomy[0], 4)
		}
		if len(reflection.LessonBuckets.Pathing) > 0 {
			resource = appendUniqueTrimmed(resource, reflection.LessonBuckets.Pathing[0], 4)
		}
	}

	return tactical, runtime, resource
}
