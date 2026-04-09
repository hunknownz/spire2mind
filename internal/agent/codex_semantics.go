package agentruntime

import (
	"fmt"
	"sort"
	"strings"
)

func enrichSeenContentRegistry(
	registry *SeenContentRegistry,
	catalog *codexCatalog,
	lessons ReflectionLessonBuckets,
	failures []string,
	evidence *codexEvidenceIndex,
) {
	if registry == nil {
		return
	}

	enrichSeenEntries(registry.Cards, seenCategoryCards, catalog, lessons, failures, evidence)
	enrichSeenEntries(registry.Relics, seenCategoryRelics, catalog, lessons, failures, evidence)
	enrichSeenEntries(registry.Potions, seenCategoryPotions, catalog, lessons, failures, evidence)
	enrichSeenEntries(registry.Monsters, seenCategoryMonsters, catalog, lessons, failures, evidence)
	enrichSeenEntries(registry.Events, seenCategoryEvents, catalog, lessons, failures, evidence)
	enrichSeenEntries(registry.Characters, seenCategoryCharacters, catalog, lessons, failures, evidence)
}

func enrichSeenEntries(
	entries []SeenContentEntry,
	category string,
	catalog *codexCatalog,
	lessons ReflectionLessonBuckets,
	failures []string,
	evidence *codexEvidenceIndex,
) {
	for i := range entries {
		entry := &entries[i]
		if strings.TrimSpace(entry.Category) == "" {
			entry.Category = category
		}
		normalizeSeenContentDisplay(entry, catalog)
		entry.RiskTags = nil
		entry.ResponseHints = nil
		entry.FailureLinks = nil

		switch strings.TrimSpace(entry.Category) {
		case seenCategoryCards:
			enrichCardEntry(entry, catalog.card(entry.ID), evidence.lookup(entry.Category, entry.ID, entry.RawName), lessons, failures)
		case seenCategoryRelics:
			enrichRelicEntry(entry, catalog.relic(entry.ID), evidence.lookup(entry.Category, entry.ID, entry.RawName), lessons, failures)
		case seenCategoryPotions:
			enrichPotionEntry(entry, catalog.potion(entry.ID), evidence.lookup(entry.Category, entry.ID, entry.RawName), lessons, failures)
		case seenCategoryMonsters:
			enrichMonsterEntry(entry, catalog.monster(entry.ID), evidence.lookup(entry.Category, entry.ID, entry.RawName), lessons, failures)
		case seenCategoryEvents:
			enrichEventEntry(entry, catalog.event(entry.ID), evidence.lookup(entry.Category, entry.ID, entry.RawName), lessons, failures)
		case seenCategoryCharacters:
			enrichCharacterEntry(entry, catalog.character(entry.ID), evidence.lookup(entry.Category, entry.ID, entry.RawName), lessons, failures)
		}
		finalizeSeenContentSemantics(entry)
	}
}

func enrichCardEntry(entry *SeenContentEntry, meta *codexCardMeta, evidence *codexEntityEvidence, lessons ReflectionLessonBuckets, failures []string) {
	if entry == nil {
		return
	}

	text := strings.ToLower(strings.Join([]string{
		strings.TrimSpace(entry.ID),
		strings.TrimSpace(entry.RawName),
		strings.TrimSpace(entry.NameEN),
		catalogCardText(meta),
	}, " "))

	if meta != nil {
		if intValue(meta.Block) > 0 || containsAnySubstring(text, " block", "heal", "barrier") {
			entry.RiskTags = append(entry.RiskTags, "survival_tool")
		}
		if intValue(meta.Damage) > 0 || strings.Contains(strings.ToLower(meta.Type), "attack") || containsAnySubstring(text, " damage", "attack") {
			entry.RiskTags = append(entry.RiskTags, "damage_tool")
		}
		if intValue(meta.CardsDraw) > 0 || containsAnySubstring(text, "draw") {
			entry.RiskTags = append(entry.RiskTags, "draw_tool")
		}
		if cardTargetsAllEnemies(meta) {
			entry.RiskTags = append(entry.RiskTags, "aoe_tool")
		}
		if strings.EqualFold(strings.TrimSpace(meta.Type), "Power") {
			entry.RiskTags = append(entry.RiskTags, "scaling_tool")
		}
		if isDeckCloggerCard(entry, meta, text) {
			entry.RiskTags = append(entry.RiskTags, "deck_clogger")
		}
	}
	if len(entry.RiskTags) == 0 {
		switch {
		case containsAnySubstring(text, "defend", "block", "armaments"):
			entry.RiskTags = append(entry.RiskTags, "survival_tool")
		case containsAnySubstring(text, "strike", "bash", "damage"):
			entry.RiskTags = append(entry.RiskTags, "damage_tool")
		case containsAnySubstring(text, "draw"):
			entry.RiskTags = append(entry.RiskTags, "draw_tool")
		case containsAnySubstring(text, "wound", "dazed", "burn", "void", "slimed", "curse", "status"):
			entry.RiskTags = append(entry.RiskTags, "deck_clogger")
		default:
			entry.RiskTags = append(entry.RiskTags, "utility_tool")
		}
	}
	if !hasAnyTag(entry.RiskTags, "survival_tool", "damage_tool", "aoe_tool", "draw_tool", "utility_tool", "scaling_tool", "deck_clogger") {
		entry.RiskTags = append(entry.RiskTags, "utility_tool")
	}

	displayName := bestSeenContentName(*entry)
	hints := []string{}
	for _, tag := range entry.RiskTags {
		switch tag {
		case "survival_tool":
			hints = append(hints, fmt.Sprintf("Treat %s as a stabilizer when HP is low or incoming damage is not covered.", displayName))
		case "damage_tool":
			hints = append(hints, fmt.Sprintf("Lean on %s when it secures lethal or trims a high-threat enemy.", displayName))
		case "aoe_tool":
			hints = append(hints, fmt.Sprintf("Upgrade the value of %s when multiple enemies are alive.", displayName))
		case "draw_tool":
			hints = append(hints, fmt.Sprintf("Use %s to smooth the turn before ending with unspent energy.", displayName))
		case "scaling_tool":
			hints = append(hints, fmt.Sprintf("Only invest in %s when the current turn is already safe.", displayName))
		case "deck_clogger":
			hints = append(hints, fmt.Sprintf("Treat %s as a dead draw unless it solves the immediate turn.", displayName))
		case "utility_tool":
			hints = append(hints, fmt.Sprintf("Use %s to keep sequencing flexible rather than forcing a greedy line.", displayName))
		}
	}
	if len(hints) < 2 {
		hints = append(hints, takeGuideLines(lessons.RewardChoice, 2-len(hints))...)
	}
	if len(hints) < 2 {
		hints = append(hints, takeGuideLines(lessons.CombatSurvival, 2-len(hints))...)
	}
	entry.ResponseHints = append(entry.ResponseHints, hints...)
	entry.FailureLinks = append(entry.FailureLinks, entityFailureLinks(evidence, failures, []string{"critical hp", "low-health", "early death", "fragile", "thin block"}, 4)...)
}

func enrichRelicEntry(entry *SeenContentEntry, meta *codexRelicMeta, evidence *codexEntityEvidence, lessons ReflectionLessonBuckets, failures []string) {
	if entry == nil {
		return
	}

	text := strings.ToLower(strings.Join([]string{strings.TrimSpace(entry.ID), strings.TrimSpace(entry.RawName), strings.TrimSpace(entry.NameEN), catalogRelicText(meta)}, " "))
	if containsAnySubstring(text, "gold", "shop", "remove", "transform") {
		entry.RiskTags = append(entry.RiskTags, "economy_conversion")
	}
	if containsAnySubstring(text, "combat", "block", "damage", "strength", "dexterity", "vigor", "potion") {
		entry.RiskTags = append(entry.RiskTags, "combat_swing")
	}
	if len(entry.RiskTags) == 0 {
		entry.RiskTags = append(entry.RiskTags, "tactical_resource")
	}

	displayName := bestSeenContentName(*entry)
	for _, tag := range entry.RiskTags {
		switch tag {
		case "economy_conversion":
			entry.ResponseHints = append(entry.ResponseHints, fmt.Sprintf("Convert %s into immediate deck or economy value instead of hoarding it passively.", displayName))
		case "combat_swing":
			entry.ResponseHints = append(entry.ResponseHints, fmt.Sprintf("Treat %s as a combat swing tool and route aggressively only if it already covers the risk.", displayName))
		case "tactical_resource":
			entry.ResponseHints = append(entry.ResponseHints, fmt.Sprintf("Let %s support the current line, but do not overvalue it above survival.", displayName))
		}
	}
	if len(entry.ResponseHints) < 2 {
		entry.ResponseHints = append(entry.ResponseHints, takeGuideLines(lessons.ShopEconomy, 2-len(entry.ResponseHints))...)
	}
	entry.FailureLinks = append(entry.FailureLinks, entityFailureLinks(evidence, failures, []string{"gold", "critical hp", "low-health"}, 4)...)
}

func enrichPotionEntry(entry *SeenContentEntry, meta *codexPotionMeta, evidence *codexEntityEvidence, lessons ReflectionLessonBuckets, failures []string) {
	if entry == nil {
		return
	}

	text := strings.ToLower(strings.Join([]string{strings.TrimSpace(entry.ID), strings.TrimSpace(entry.RawName), strings.TrimSpace(entry.NameEN), catalogPotionText(meta)}, " "))
	entry.RiskTags = append(entry.RiskTags, "tactical_resource")
	if containsAnySubstring(text, "block", "damage", "strength", "dexterity", "enemy", "combat", "upgrade", "draw") {
		entry.RiskTags = append(entry.RiskTags, "combat_swing")
	}

	displayName := bestSeenContentName(*entry)
	entry.ResponseHints = append(entry.ResponseHints,
		fmt.Sprintf("Spend %s to stabilize or swing a fight before the potion slot gets stranded.", displayName),
	)
	if len(entry.ResponseHints) < 2 {
		entry.ResponseHints = append(entry.ResponseHints, takeGuideLines(lessons.CombatSurvival, 1)...)
	}
	entry.FailureLinks = append(entry.FailureLinks, entityFailureLinks(evidence, failures, []string{"critical hp", "low-health", "early death"}, 4)...)
}

func enrichMonsterEntry(entry *SeenContentEntry, meta *codexMonsterMeta, evidence *codexEntityEvidence, lessons ReflectionLessonBuckets, failures []string) {
	if entry == nil {
		return
	}

	text := strings.ToLower(strings.Join([]string{strings.TrimSpace(entry.ID), strings.TrimSpace(entry.RawName), strings.TrimSpace(entry.NameEN), catalogMonsterText(meta)}, " "))
	maxDamage := maxMonsterDamage(meta)
	entry.RiskTags = append(entry.RiskTags, "combat_threat")
	if maxDamage >= 12 || strings.Contains(strings.ToLower(firstNonEmpty(metaType(meta), "")), "elite") {
		entry.RiskTags = append(entry.RiskTags, "burst_threat")
	}
	if monsterLooksMultiAttack(meta) {
		entry.RiskTags = append(entry.RiskTags, "multi_attack")
	}
	if evidenceSignal(evidence, "critical_hp")+evidenceSignal(evidence, "low_hp") > 0 {
		entry.RiskTags = append(entry.RiskTags, "punishes_low_hp")
	}
	if maxDamage > 0 && maxDamage <= 5 {
		entry.RiskTags = append(entry.RiskTags, "setup_window")
	}
	if hasEarlyRunEvidence(entry, evidence, maxDamage) {
		entry.RiskTags = append(entry.RiskTags, "early_run_gatekeeper")
	}

	displayName := bestSeenContentName(*entry)
	for _, tag := range entry.RiskTags {
		switch tag {
		case "burst_threat":
			entry.ResponseHints = append(entry.ResponseHints, fmt.Sprintf("Respect %s's burst turn; block or kill before trying to race.", displayName))
		case "multi_attack":
			entry.ResponseHints = append(entry.ResponseHints, fmt.Sprintf("Against %s, value block and AOE more highly than a single greedy hit.", displayName))
		case "punishes_low_hp":
			entry.ResponseHints = append(entry.ResponseHints, fmt.Sprintf("At low HP, do not leave %s alive without a clear survival plan.", displayName))
		case "setup_window":
			entry.ResponseHints = append(entry.ResponseHints, fmt.Sprintf("Use %s as a setup window only when the current turn is otherwise safe.", displayName))
		case "early_run_gatekeeper":
			entry.ResponseHints = append(entry.ResponseHints, fmt.Sprintf("Treat %s as an early consistency check; do not enter underpowered.", displayName))
		case "combat_threat":
			entry.ResponseHints = append(entry.ResponseHints, fmt.Sprintf("Track %s's intent first, then decide whether to race, block, or set up.", displayName))
		}
	}
	if len(entry.ResponseHints) < 2 {
		entry.ResponseHints = append(entry.ResponseHints, takeGuideLines(lessons.CombatSurvival, 2-len(entry.ResponseHints))...)
	}
	entry.FailureLinks = append(entry.FailureLinks, entityFailureLinks(evidence, failures, []string{"critical hp", "low-health", "early death", "elite"}, 4)...)
	_ = text
}

func enrichEventEntry(entry *SeenContentEntry, meta *codexEventMeta, evidence *codexEntityEvidence, lessons ReflectionLessonBuckets, failures []string) {
	if entry == nil {
		return
	}

	text := strings.ToLower(strings.Join([]string{strings.TrimSpace(entry.ID), strings.TrimSpace(entry.RawName), strings.TrimSpace(entry.NameEN), catalogEventText(meta)}, " "))
	entry.RiskTags = append(entry.RiskTags, "event_tradeoff")
	if containsAnySubstring(text, "gold", "relic", "potion", "resource", "gain gold", "lose gold") {
		entry.RiskTags = append(entry.RiskTags, "resource_tradeoff")
	}
	if containsAnySubstring(text, "hp", "max hp", "damage", "lose hp", "heal") {
		entry.RiskTags = append(entry.RiskTags, "hp_tradeoff")
	}
	if containsAnySubstring(text, "transform", "remove", "upgrade", "deck", "card") {
		entry.RiskTags = append(entry.RiskTags, "deck_mutation")
	}
	if evidenceSignal(evidence, "route_pressure") > 0 || containsAnySubstring(text, "route", "path", "map", "elite") {
		entry.RiskTags = append(entry.RiskTags, "route_pressure")
	}

	displayName := bestSeenContentName(*entry)
	for _, tag := range entry.RiskTags {
		switch tag {
		case "event_tradeoff":
			entry.ResponseHints = append(entry.ResponseHints, fmt.Sprintf("Treat %s as a tradeoff screen, not an automatic click.", displayName))
		case "resource_tradeoff":
			entry.ResponseHints = append(entry.ResponseHints, fmt.Sprintf("Use %s only when the current gold and relic economy can absorb the trade.", displayName))
		case "hp_tradeoff":
			entry.ResponseHints = append(entry.ResponseHints, fmt.Sprintf("Only take %s HP trades when the deck can survive the next few rooms.", displayName))
		case "deck_mutation":
			entry.ResponseHints = append(entry.ResponseHints, fmt.Sprintf("Use %s to clean weak starters or key deck gaps, not to add random bulk.", displayName))
		case "route_pressure":
			entry.ResponseHints = append(entry.ResponseHints, fmt.Sprintf("Treat %s as a route-pressure decision; preserve safer exits if HP is shaky.", displayName))
		}
	}
	if len(entry.ResponseHints) < 2 {
		entry.ResponseHints = append(entry.ResponseHints, takeGuideLines(lessons.Pathing, 2-len(entry.ResponseHints))...)
	}
	if len(entry.ResponseHints) < 2 {
		entry.ResponseHints = append(entry.ResponseHints, takeGuideLines(lessons.ShopEconomy, 2-len(entry.ResponseHints))...)
	}
	entry.FailureLinks = append(entry.FailureLinks, entityFailureLinks(evidence, failures, []string{"gold", "path", "route", "critical hp", "low-health"}, 4)...)
}

func enrichCharacterEntry(entry *SeenContentEntry, meta *codexCharacterMeta, evidence *codexEntityEvidence, lessons ReflectionLessonBuckets, failures []string) {
	if entry == nil {
		return
	}

	entry.RiskTags = append(entry.RiskTags, "baseline_run_context")
	displayName := bestSeenContentName(*entry)
	if meta != nil && strings.TrimSpace(meta.Name) != "" {
		displayName = strings.TrimSpace(meta.Name)
	}
	entry.ResponseHints = append(entry.ResponseHints,
		fmt.Sprintf("Use %s as the baseline context for card, relic, and event evaluation in this run.", displayName),
	)
	if len(entry.ResponseHints) < 2 {
		entry.ResponseHints = append(entry.ResponseHints, takeGuideLines(lessons.Pathing, 1)...)
		entry.ResponseHints = append(entry.ResponseHints, takeGuideLines(lessons.RewardChoice, 1)...)
	}
	entry.FailureLinks = append(entry.FailureLinks, entityFailureLinks(evidence, failures, []string{"early death", "critical hp", "low-health"}, 4)...)
}

func finalizeSeenContentSemantics(entry *SeenContentEntry) {
	if entry == nil {
		return
	}
	entry.RiskTags = dedupeGuideLines(entry.RiskTags, 7)
	entry.ResponseHints = cleanVisibleTextSlice(dedupeGuideLines(entry.ResponseHints, 4))
	entry.FailureLinks = cleanVisibleTextSlice(dedupeGuideLines(entry.FailureLinks, 4))
}

func takeGuideLines(lines []string, limit int) []string {
	lines = dedupeGuideLines(lines, limit)
	if limit > 0 && len(lines) > limit {
		return append([]string(nil), lines[:limit]...)
	}
	return lines
}

func dedupeGuideLines(lines []string, limit int) []string {
	if len(lines) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(lines))
	deduped := make([]string, 0, len(lines))
	for _, line := range lines {
		line = cleanVisibleText(line)
		if line == "" {
			continue
		}
		key := strings.ToLower(line)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, line)
		if limit > 0 && len(deduped) >= limit {
			break
		}
	}
	return deduped
}

func cardKnowledgePrior(snapshot CombatSnapshot, card CombatCardState, entry *SeenContentEntry) (float64, []string) {
	if entry == nil {
		return 0, nil
	}

	hpRatio := combatPlayerHPRatio(snapshot)
	prior := 0.0
	cues := make([]string, 0, 3)
	multiEnemy := livingEnemyCount(snapshot) >= 2
	pressureHigh := snapshot.IncomingDamage > snapshot.Player.Block
	significantPressure := pressureHigh && snapshot.IncomingDamage >= max(4, snapshot.Player.CurrentHP/5)

	for _, tag := range entry.RiskTags {
		switch tag {
		case "survival_tool":
			if significantPressure {
				prior += 1.25
				cues = append(cues, fmt.Sprintf("%s boosted for survival coverage", fallbackID(card.Name, card.CardID)))
			}
			if hpRatio <= 0.2 && snapshot.IncomingDamage > 0 {
				prior += 0.45
			}
		case "damage_tool":
			prior += 0.45
			if card.RequiresTarget {
				prior += 0.15
			}
		case "aoe_tool":
			if multiEnemy {
				prior += 1.0
				cues = append(cues, fmt.Sprintf("%s favored into multiple enemies", fallbackID(card.Name, card.CardID)))
			} else {
				prior += 0.2
			}
		case "draw_tool":
			prior += 0.35
			if significantPressure || hpRatio <= 0.25 {
				prior += 0.25
			}
		case "utility_tool":
			prior += 0.2
		case "scaling_tool":
			if !pressureHigh && hpRatio > 0.5 {
				prior += 0.65
			} else {
				prior -= 0.35
			}
		case "deck_clogger":
			prior -= 1.0
			cues = append(cues, fmt.Sprintf("%s penalized as deck clutter", fallbackID(card.Name, card.CardID)))
		}
	}

	for _, link := range entry.FailureLinks {
		lower := strings.ToLower(link)
		switch {
		case hpRatio <= 0.35 && containsAnySubstring(lower, "critical hp", "low-health"):
			prior += 0.2
		case containsAnySubstring(lower, "fragile", "thin block", "early death") && hasTag(entry.RiskTags, "scaling_tool"):
			prior -= 0.25
		}
	}

	return prior, dedupeGuideLines(cues, 3)
}

func enemyKnowledgePrior(snapshot CombatSnapshot, enemy CombatEnemyState, entry *SeenContentEntry) (float64, []string) {
	if entry == nil {
		return 0, nil
	}

	hpRatio := combatPlayerHPRatio(snapshot)
	prior := 0.0
	cues := make([]string, 0, 3)
	for _, tag := range entry.RiskTags {
		switch tag {
		case "combat_threat":
			prior += 0.25
		case "burst_threat":
			prior += 0.75
			cues = append(cues, fmt.Sprintf("%s flagged as burst threat", fallbackID(enemy.Name, enemy.EnemyID)))
		case "multi_attack":
			prior += 0.45
		case "punishes_low_hp":
			if hpRatio <= 0.35 {
				prior += 1.0
				cues = append(cues, fmt.Sprintf("%s escalates at low HP", fallbackID(enemy.Name, enemy.EnemyID)))
			}
		case "early_run_gatekeeper":
			prior += 0.35
		case "setup_window":
			prior -= 0.25
		}
	}
	if snapshot.LowestEnemyLabel != "" && snapshot.LowestEnemyLabel == fallbackID(enemy.Name, enemy.EnemyID) {
		prior += 0.2
	}
	return prior, dedupeGuideLines(cues, 3)
}

func applyCodexPriors(snapshot *CombatSnapshot, codex *SeenContentRegistry) {
	if snapshot == nil || codex == nil {
		return
	}

	knowledgeBiases := make([]string, 0, 4)
	for i := range snapshot.Hand {
		entry := findSeenContentEntry(codex, seenCategoryCards, snapshot.Hand[i].CardID, snapshot.Hand[i].Name)
		prior, cues := cardKnowledgePrior(*snapshot, snapshot.Hand[i], entry)
		snapshot.Hand[i].KnowledgePrior = prior
		knowledgeBiases = append(knowledgeBiases, cues...)
	}
	for i := range snapshot.Enemies {
		entry := findSeenContentEntry(codex, seenCategoryMonsters, snapshot.Enemies[i].EnemyID, snapshot.Enemies[i].Name)
		prior, cues := enemyKnowledgePrior(*snapshot, snapshot.Enemies[i], entry)
		snapshot.Enemies[i].KnowledgePrior = prior
		knowledgeBiases = append(knowledgeBiases, cues...)
	}

	knowledgeBiases = dedupeGuideLines(knowledgeBiases, 5)
	sort.Strings(knowledgeBiases)
	snapshot.KnowledgeBiases = knowledgeBiases
}

func combatPlayerHPRatio(snapshot CombatSnapshot) float64 {
	if snapshot.Player.MaxHP <= 0 {
		return 1
	}
	return float64(snapshot.Player.CurrentHP) / float64(snapshot.Player.MaxHP)
}

func entityFailureLinks(evidence *codexEntityEvidence, globalFailures []string, keywords []string, limit int) []string {
	links := []string{}
	if evidence != nil {
		links = append(links, evidence.FailureLinks...)
	}
	if len(links) < limit {
		links = append(links, filterGuideLines(globalFailures, keywords)...)
	}
	return dedupeGuideLines(links, limit)
}

func evidenceSignal(evidence *codexEntityEvidence, key string) int {
	if evidence == nil || evidence.FailureSignals == nil {
		return 0
	}
	return evidence.FailureSignals[strings.TrimSpace(key)]
}

func hasEarlyRunEvidence(entry *SeenContentEntry, evidence *codexEntityEvidence, maxDamage int) bool {
	if evidence != nil && evidence.MinFloor != nil && *evidence.MinFloor <= 7 {
		return maxDamage >= 8 || evidenceSignal(evidence, "early_death") > 0
	}
	if entry != nil && entry.FirstFloor != nil && *entry.FirstFloor <= 7 {
		return maxDamage >= 8
	}
	return false
}

func cardTargetsAllEnemies(meta *codexCardMeta) bool {
	if meta == nil {
		return false
	}
	target := strings.ToLower(strings.TrimSpace(meta.Target))
	text := catalogCardText(meta)
	return containsAnySubstring(target, "allenemies", "all enemies", "all_enemy") ||
		containsAnySubstring(text, "all enemies", "each enemy")
}

func isDeckCloggerCard(entry *SeenContentEntry, meta *codexCardMeta, text string) bool {
	if entry == nil {
		return false
	}
	id := strings.ToLower(strings.TrimSpace(entry.ID))
	return containsAnySubstring(id, "wound", "dazed", "burn", "void", "slimed", "curse", "status") ||
		containsAnySubstring(text, "unplayable", "status", "curse")
}

func monsterLooksMultiAttack(meta *codexMonsterMeta) bool {
	if meta == nil {
		return false
	}
	text := catalogMonsterText(meta)
	return containsAnySubstring(text, "double", "triple", "flurry", "multi", "barrage", "swipe")
}

func metaType(meta *codexMonsterMeta) string {
	if meta == nil {
		return ""
	}
	return meta.Type
}

func hasAnyTag(tags []string, wanted ...string) bool {
	for _, tag := range wanted {
		if hasTag(tags, tag) {
			return true
		}
	}
	return false
}

func hasTag(tags []string, wanted string) bool {
	wanted = strings.TrimSpace(wanted)
	for _, tag := range tags {
		if strings.EqualFold(strings.TrimSpace(tag), wanted) {
			return true
		}
	}
	return false
}

func intValue(value *int) int {
	if value == nil {
		return 0
	}
	return *value
}

func livingEnemyCount(snapshot CombatSnapshot) int {
	count := 0
	for _, enemy := range snapshot.Enemies {
		if enemy.CurrentHP > 0 && enemy.Hittable {
			count++
		}
	}
	return count
}
