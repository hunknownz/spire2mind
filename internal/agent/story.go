package agentruntime

import (
	"fmt"
	"strings"

	"spire2mind/internal/game"
	"spire2mind/internal/i18n"
)

func renderRunStory(reflections []*AttemptReflection) string {
	return renderRunStoryDocument(reflections, 0, nil, nil, nil, nil, i18n.LanguageEnglish)
}

func renderRunStoryDocument(
	reflections []*AttemptReflection,
	attempt int,
	state *game.StateSnapshot,
	todo *TodoManager,
	compact *CompactMemory,
	codex *SeenContentRegistry,
	language i18n.Language,
) string {
	loc := i18n.New(language)
	lines := []string{
		"# " + loc.Label("Spire2Mind Run Story", "Spire2Mind 对局故事"),
		"",
		loc.Paragraph(
			"This file tracks the live run, the agent's current intentions, and the lessons it carries from previous attempts.",
			"这份文档追踪当前对局、Agent 的实时意图，以及它从之前尝试中带来的经验。",
		),
	}

	if state != nil || todo != nil || compact != nil || attempt > 0 {
		lines = append(lines, "", "## "+loc.Label("Current Chapter", "当前章节"), "")

		if attempt > 0 {
			lines = append(lines, fmt.Sprintf("- %s: `%d`", loc.Label("Attempt", "尝试"), attempt))
		}
		if state != nil {
			if strings.TrimSpace(state.RunID) != "" {
				lines = append(lines, fmt.Sprintf("- %s: `%s`", loc.Label("Run ID", "Run 标识"), state.RunID))
			}
			if strings.TrimSpace(state.Screen) != "" {
				lines = append(lines, fmt.Sprintf("- %s: `%s`", loc.Label("Screen", "界面"), state.Screen))
			}
		}

		if summaryLines := StateSummaryLinesFor(state, loc.Language()); len(summaryLines) > 0 {
			lines = append(lines, "", "### "+loc.Label("Situation", "局面"), "")
			for _, line := range summaryLines {
				lines = append(lines, line)
			}
		}
		if hints := BuildTacticalHints(state); len(hints) > 0 {
			lines = append(lines, "", "### "+loc.Label("Tactical Lens", "战术镜头"), "")
			for _, hint := range hints {
				lines = append(lines, "- "+hint)
			}
		}

		if todo != nil {
			snapshot := todo.Snapshot()
			lines = append(lines, "", "### "+loc.Label("Intent", "意图"), "")
			lines = append(lines, fmt.Sprintf("- %s: %s", loc.Label("Current goal", "当前目标"), storyValueOrDash(snapshot.CurrentGoal)))
			lines = append(lines, fmt.Sprintf("- %s: %s", loc.Label("Room goal", "房间目标"), storyValueOrDash(snapshot.RoomGoal)))
			lines = append(lines, fmt.Sprintf("- %s: %s", loc.Label("Next intent", "下一意图"), storyValueOrDash(snapshot.NextIntent)))
			if snapshot.CarryForwardPlan != "" {
				lines = append(lines, fmt.Sprintf("- %s: %s", loc.Label("Carry-forward plan", "跨局计划"), snapshot.CarryForwardPlan))
			}
			appendStoryBucketSection(&lines, "- "+loc.Label("Carry-forward lessons by category", "分类跨局经验")+":", snapshot.CarryForwardBuckets)
			if remaining := UncategorizedLessons(snapshot.CarryForwardLessons, snapshot.CarryForwardBuckets, 8); len(remaining) > 0 {
				lines = append(lines, "- "+loc.Label("Carry-forward lessons", "跨局经验")+":")
				for _, lesson := range dedupeStoryLines(remaining) {
					lines = append(lines, "  - "+lesson)
				}
			}
		}

		if compact != nil {
			if recent := compact.RecentTimeline(8); len(recent) > 0 {
				lines = append(lines, "", "### "+loc.Label("Recent Beats", "最近节拍"), "")
				for _, beat := range recent {
					lines = append(lines, "- "+beat)
				}
			}
			if story := strings.TrimSpace(compact.lastStory); story != "" {
				lines = append(lines, "", "### "+loc.Label("Latest Reflection Echo", "最近反思回声"), "", story)
			}
		}
		if codex != nil {
			lines = append(lines, "", "### "+loc.Label("Known World", "已知世界"), "")
			lines = append(lines, seenContentCountLines(codex, loc.Language())...)
			lines = append(lines, "", "- "+loc.Label("Recent discoveries", "最近发现")+":")
			for _, line := range seenContentDiscoveryLines(codex, loc.Language(), 6) {
				lines = append(lines, "  "+line)
			}
		}
	}

	if len(reflections) == 0 {
		lines = append(lines, "", "## "+loc.Label("Previous Attempts", "之前的尝试"), "", "_"+loc.Label("No completed attempt reflections yet.", "还没有完成的尝试反思。")+"_")
		return strings.Join(lines, "\n")
	}

	lines = append(lines, "", "## "+loc.Label("Strategy Ledger", "策略账本"), "")
	ledgerSeen := make(map[string]struct{})
	if todo != nil {
		appendStrategyLedgerBuckets(&lines, ledgerSeen, todo.Snapshot().CarryForwardBuckets)
	}
	for _, lesson := range collectStoryLessons(reflections, todo) {
		key := storyLedgerKey(lesson)
		if _, ok := ledgerSeen[key]; ok {
			continue
		}
		lines = append(lines, "- "+lesson)
		ledgerSeen[key] = struct{}{}
	}

	lines = append(lines, "", "## "+loc.Label("Previous Attempts", "之前的尝试"))
	recentReflections := recentStoryReflections(reflections, 3)
	olderCount := len(reflections) - len(recentReflections)
	if olderCount > 0 {
		lines = append(lines, "", "_"+loc.Label(
			fmt.Sprintf("Showing the latest %d attempts. %d older attempt(s) are still stored in their individual reflection files.", len(recentReflections), olderCount),
			fmt.Sprintf("这里只展示最近 %d 次尝试。更早的 %d 次尝试仍保存在各自的反思文件中。", len(recentReflections), olderCount),
		)+"_")
	}

	startOrdinal := len(reflections) - len(recentReflections) + 1
	for i, reflection := range recentReflections {
		if reflection == nil {
			continue
		}

		lines = append(lines, "", storyAttemptHeading(startOrdinal+i, reflection), "", reflection.Story)
		if remaining := UncategorizedLessons(reflection.Lessons, reflection.LessonBuckets, 10); len(remaining) > 0 {
			lines = append(lines, "", loc.Label("Lessons", "经验")+":")
			for _, lesson := range dedupeStoryLines(remaining) {
				lines = append(lines, "- "+lesson)
			}
		}
		for _, section := range reflection.LessonBuckets.Sections() {
			lines = append(lines, "", section.Title+" "+loc.Label("lessons", "经验")+":")
			for _, lesson := range dedupeStoryLines(section.Lessons) {
				lines = append(lines, "- "+lesson)
			}
		}
		if len(reflection.Successes) > 0 {
			lines = append(lines, "", loc.Label("What worked", "有效之处")+":")
			for _, success := range dedupeStoryLines(reflection.Successes) {
				lines = append(lines, "- "+success)
			}
		}
		if len(reflection.Risks) > 0 {
			lines = append(lines, "", loc.Label("What hurt", "受损之处")+":")
			for _, risk := range dedupeStoryLines(reflection.Risks) {
				lines = append(lines, "- "+risk)
			}
		}
		if reflection.NextPlan != "" {
			lines = append(lines, "", loc.Label("Carry-forward plan", "跨局计划")+":", reflection.NextPlan)
		}
	}

	return strings.Join(lines, "\n")
}

func storyValueOrDash(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}

	return value
}

func recentStoryReflections(reflections []*AttemptReflection, limit int) []*AttemptReflection {
	if limit <= 0 || len(reflections) == 0 {
		return nil
	}
	if len(reflections) <= limit {
		return append([]*AttemptReflection(nil), reflections...)
	}
	return append([]*AttemptReflection(nil), reflections[len(reflections)-limit:]...)
}

func collectStoryLessons(reflections []*AttemptReflection, todo *TodoManager) []string {
	lessons := make([]string, 0, 8)
	if todo != nil {
		snapshot := todo.Snapshot()
		for _, lesson := range UncategorizedLessons(snapshot.CarryForwardLessons, snapshot.CarryForwardBuckets, 8) {
			lessons = appendUniqueTrimmed(lessons, lesson, 8)
		}
	}
	for i := len(reflections) - 1; i >= 0; i-- {
		reflection := reflections[i]
		if reflection == nil {
			continue
		}
		for _, lesson := range UncategorizedLessons(reflection.Lessons, reflection.LessonBuckets, 8) {
			lessons = appendUniqueTrimmed(lessons, lesson, 8)
		}
	}
	return lessons
}

func dedupeStoryLines(lines []string) []string {
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		result = appendUniqueTrimmed(result, line, len(lines))
	}
	return result
}

func appendStoryBucketSection(lines *[]string, heading string, buckets ReflectionLessonBuckets) {
	if lines == nil || buckets.IsEmpty() {
		return
	}

	*lines = append(*lines, heading)
	for _, section := range buckets.Sections() {
		*lines = append(*lines, "  - "+section.Title+":")
		for _, lesson := range dedupeStoryLines(section.Lessons) {
			*lines = append(*lines, "    - "+lesson)
		}
	}
}

func appendStrategyLedgerBuckets(lines *[]string, seen map[string]struct{}, buckets ReflectionLessonBuckets) {
	if lines == nil || buckets.IsEmpty() {
		return
	}

	for _, section := range buckets.Sections() {
		*lines = append(*lines, "- "+section.Title+":")
		for _, lesson := range dedupeStoryLines(section.Lessons) {
			*lines = append(*lines, "  - "+lesson)
			seen[storyLedgerKey(lesson)] = struct{}{}
		}
	}
}

func storyAttemptHeading(ordinal int, reflection *AttemptReflection) string {
	if reflection == nil {
		return "### Attempt"
	}

	if ordinal <= 0 {
		ordinal = reflection.Attempt
	}
	parts := []string{fmt.Sprintf("Attempt %d", ordinal)}
	if runID := strings.TrimSpace(reflection.RunID); runID != "" {
		parts = append(parts, runID)
	}
	if outcome := strings.TrimSpace(reflection.Outcome); outcome != "" {
		if reflection.Floor != nil {
			parts = append(parts, fmt.Sprintf("%s on floor %d", outcome, *reflection.Floor))
		} else {
			parts = append(parts, outcome)
		}
	} else if reflection.Floor != nil {
		parts = append(parts, fmt.Sprintf("floor %d", *reflection.Floor))
	}

	return "### " + strings.Join(parts, " 路 ")
}

func storyLedgerKey(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
