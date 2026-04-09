package agentruntime

import (
	"fmt"
	"strings"

	"spire2mind/internal/game"
	"spire2mind/internal/i18n"
)

func renderRunGuideDocument(
	reflections []*AttemptReflection,
	attempt int,
	state *game.StateSnapshot,
	todo *TodoManager,
	codex *SeenContentRegistry,
	language i18n.Language,
) string {
	loc := i18n.New(language)
	lines := []string{
		"# " + loc.Label("Spire2Mind Living Guide", "Spire2Mind 动态攻略"),
		"",
		loc.Paragraph(
			"This document distills the agent's current strategy, recurring failure patterns, and practical advice from recent runs.",
			"这份文档沉淀 Agent 当前策略、重复出现的失败模式，以及最近几轮对局中的实用建议。",
		),
	}

	if attempt > 0 || state != nil || todo != nil {
		lines = append(lines, "", "## "+loc.Label("Live Doctrine", "当前打法准则"), "")
		if attempt > 0 {
			lines = append(lines, fmt.Sprintf("- %s: `%d`", loc.Label("Current attempt", "当前尝试"), attempt))
		}
		if state != nil {
			lines = append(lines, fmt.Sprintf("- %s: `%s`", loc.Label("Current screen", "当前界面"), valueOrDash(state.Screen)))
			if state.RunID != "" {
				lines = append(lines, fmt.Sprintf("- %s: `%s`", loc.Label("Run ID", "Run 标识"), state.RunID))
			}
		}
		if todo != nil {
			snapshot := todo.Snapshot()
			lines = append(lines, fmt.Sprintf("- %s: %s", loc.Label("Current goal", "当前目标"), valueOrDash(snapshot.CurrentGoal)))
			lines = append(lines, fmt.Sprintf("- %s: %s", loc.Label("Room goal", "房间目标"), valueOrDash(snapshot.RoomGoal)))
			lines = append(lines, fmt.Sprintf("- %s: %s", loc.Label("Next intent", "下一意图"), valueOrDash(snapshot.NextIntent)))
			lines = append(lines, fmt.Sprintf("- %s: %s", loc.Label("Carry-forward plan", "跨局计划"), valueOrDash(snapshot.CarryForwardPlan)))
			appendGuideBucketSection(&lines, loc.Label("Carry-forward lessons", "跨局经验"), snapshot.CarryForwardBuckets)
			if remaining := UncategorizedLessons(snapshot.CarryForwardLessons, snapshot.CarryForwardBuckets, 10); len(remaining) > 0 {
				lines = append(lines, "- "+loc.Label("Other lessons", "其他经验")+":")
				for _, lesson := range dedupeStoryLines(remaining) {
					lines = append(lines, "  - "+lesson)
				}
			}
		}
		if hints := BuildTacticalHints(state); len(hints) > 0 {
			lines = append(lines, "", "## "+loc.Label("Current Tactical Lens", "当前战术镜头"), "")
			for _, hint := range hints {
				lines = append(lines, "- "+hint)
			}
		}
	}

	lines = append(lines, "", "## "+loc.Label("Known World", "已知世界"), "")
	lines = append(lines, seenContentCountLines(codex, loc.Language())...)
	lines = append(lines, "", "- "+loc.Label("Recent discoveries", "最近发现")+":")
	for _, line := range seenContentDiscoveryLines(codex, loc.Language(), 8) {
		lines = append(lines, "  "+line)
	}

	lines = append(lines, "", "## "+loc.Label("Strategy Ledger", "策略账本"), "")
	ledger := aggregateGuideBuckets(reflections, todo)
	if ledger.IsEmpty() {
		lines = append(lines, "- "+loc.Label("No stable strategy lessons yet.", "还没有稳定沉淀下来的策略经验。"))
	} else {
		appendGuideBucketSection(&lines, loc.Label("Stable heuristics", "稳定启发"), ledger)
	}

	lines = append(lines, "", "## "+loc.Label("Failure Patterns", "失败模式"), "")
	failures := collectGuideFailures(reflections)
	if len(failures) == 0 {
		lines = append(lines, "- "+loc.Label("No repeated failure pattern recorded yet.", "还没有记录到重复出现的失败模式。"))
	} else {
		for _, failure := range failures {
			lines = append(lines, "- "+failure)
		}
	}

	lines = append(lines, "", "## "+loc.Label("Story Seeds", "故事种子"), "")
	seeds := collectGuideStorySeeds(reflections)
	if len(seeds) == 0 {
		lines = append(lines, "- "+loc.Label("No finished attempt story yet.", "还没有完成尝试故事。"))
	} else {
		for _, seed := range seeds {
			lines = append(lines, "- "+seed)
		}
	}

	return strings.Join(lines, "\n")
}

func appendGuideBucketSection(lines *[]string, heading string, buckets ReflectionLessonBuckets) {
	if lines == nil || buckets.IsEmpty() {
		return
	}

	if heading != "" {
		*lines = append(*lines, "- "+heading+":")
	}
	for _, section := range buckets.Sections() {
		*lines = append(*lines, "  - "+section.Title+":")
		for _, lesson := range dedupeStoryLines(section.Lessons) {
			*lines = append(*lines, "    - "+lesson)
		}
	}
}

func aggregateGuideBuckets(reflections []*AttemptReflection, todo *TodoManager) ReflectionLessonBuckets {
	var merged ReflectionLessonBuckets
	if todo != nil {
		merged.Merge(todo.Snapshot().CarryForwardBuckets, 6)
	}
	for _, reflection := range reflections {
		if reflection == nil {
			continue
		}
		buckets := reflection.LessonBuckets
		if buckets.IsEmpty() {
			buckets = InferLessonBuckets(reflection.Lessons)
		}
		merged.Merge(buckets, 6)
	}
	return merged
}

func collectGuideFailures(reflections []*AttemptReflection) []string {
	items := make([]string, 0, 12)
	for i := len(reflections) - 1; i >= 0; i-- {
		reflection := reflections[i]
		if reflection == nil {
			continue
		}
		for _, failure := range reflection.RecentFailures {
			items = appendUniqueTrimmed(items, summarizeGuideFailure(failure), 8)
		}
		for _, risk := range reflection.Risks {
			items = appendUniqueTrimmed(items, summarizeGuideFailure(risk), 8)
		}
	}
	return items
}

func collectGuideStorySeeds(reflections []*AttemptReflection) []string {
	items := make([]string, 0, 8)
	for i := len(reflections) - 1; i >= 0; i-- {
		reflection := reflections[i]
		if reflection == nil {
			continue
		}

		headline := strings.TrimSpace(reflection.Headline)
		if headline == "" {
			headline = strings.TrimSpace(reflection.Story)
		}
		if headline == "" {
			continue
		}

		seed := fmt.Sprintf(
			"Attempt %d on floor %s: %s",
			reflection.Attempt,
			guideFloorValue(reflection.Floor),
			compactText(summarizeGuideSeed(reflection, headline), 180),
		)
		items = appendUniqueTrimmed(items, seed, 6)
	}
	return items
}

func guideFloorValue(floor *int) string {
	if floor == nil {
		return "-"
	}
	return fmt.Sprintf("%d", *floor)
}

func summarizeGuideFailure(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}

	lower := strings.ToLower(value)
	switch {
	case strings.Contains(lower, "state_unavailable: drift_kind=selection_seam"):
		return "A selection seam forced a replan before an indexed action could land."
	case strings.Contains(lower, "state_unavailable: drift_kind=same_screen_index_drift"):
		return "A same-screen index drift changed the legal card/option indexes mid-turn."
	case strings.Contains(lower, "state_unavailable: drift_kind=reward_transition"):
		return "A reward transition changed the screen before the queued reward action resolved."
	case strings.Contains(lower, "state_unavailable: drift_kind=action_window_changed"):
		return "The action window closed mid-execution; the runtime had to re-read the live state."
	case strings.Contains(lower, "state_unavailable: drift_kind=screen_transition:game_over"):
		return "The combat resolved into GAME_OVER before the queued action landed; close out the run cleanly and bootstrap the next attempt."
	case strings.Contains(lower, "state_unavailable: drift_kind="):
		start := strings.Index(lower, "state_unavailable: drift_kind=")
		if start >= 0 {
			kind := lower[start+len("state_unavailable: drift_kind="):]
			if end := strings.Index(kind, ":"); end >= 0 {
				kind = kind[:end]
			}
			return fmt.Sprintf("A live-state drift (%s) forced the runtime to abandon the stale action and replan.", kind)
		}
	case strings.Contains(lower, "invalid_action"):
		return "A queued action went stale before execution and had to be replanned."
	default:
		return value
	}

	return value
}

func summarizeGuideSeed(reflection *AttemptReflection, headline string) string {
	headline = strings.TrimSpace(headline)
	if headline == "" {
		return headline
	}

	if strings.EqualFold(headline, "GAME_OVER: 1 available actions") {
		switch {
		case len(reflection.Risks) > 0:
			return reflection.Risks[0]
		case len(reflection.Lessons) > 0:
			return reflection.Lessons[0]
		case reflection.NextPlan != "":
			return reflection.NextPlan
		}
	}

	return headline
}
