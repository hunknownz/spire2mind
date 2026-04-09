package tui

import (
	"fmt"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/lipgloss"

	agentruntime "spire2mind/internal/agent"
	"spire2mind/internal/game"
)

func (m *Model) View() tea.View {
	width := m.width
	if width <= 0 {
		width = 140
	}

	header := m.renderHeader(width)
	body := m.renderBody(width)
	footer := footerStyle.Render(m.loc.Label("q quit | ctrl+c stop | this TUI mirrors the live autoplay runtime", "q 退出 | ctrl+c 停止 | 这个 TUI 镜像显示实时自动游玩运行时"))

	return tea.NewView(rootStyle.Width(width).Render(lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		body,
		footer,
	)))
}

func (m *Model) renderHeader(width int) string {
	title := headerStyle.Render("Spire2Mind")
	subtitle := subtitleStyle.Render(m.loc.Label("STS2 autoplay cockpit", "STS2 自动游玩驾驶舱"))
	left := lipgloss.JoinVertical(lipgloss.Left, title, subtitle)
	screen, _, _ := m.currentStateHeader()

	right := lipgloss.JoinHorizontal(
		lipgloss.Left,
		statusBadgeStyle(m.status).Render(m.loc.Label("status", "状态")+" "+compactValue(m.status)),
		" ",
		statusBadgeStyle(m.providerState).Render(m.loc.Label("provider", "提供方")+" "+compactValue(m.providerState)),
		" ",
		infoBadgeStyle().Render(m.loc.Label("screen", "界面")+" "+compactValue(screen)),
		" ",
		infoBadgeStyle().Render(fmt.Sprintf("%s %d", m.loc.Label("attempt", "尝试"), m.attempt)),
		" ",
		infoBadgeStyle().Render(fmt.Sprintf("%s %d", m.loc.Label("cycle", "循环"), m.cycle)),
		" ",
		infoBadgeStyle().Render(fmt.Sprintf("%s %d", m.loc.Label("turns", "回合"), m.turns)),
	)

	leftWidth := max(0, width-lipgloss.Width(right)-2)
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		lipgloss.NewStyle().Width(leftWidth).Render(left),
		right,
	)
}

func (m *Model) renderBody(width int) string {
	if width < 120 {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			m.renderOverviewPanel(width),
			m.renderModelTelemetryPanel(width),
			m.renderGoalsPanel(width),
			m.renderGuidebookPanel(width),
			m.renderTacticalPanel(width),
			m.renderSignalsPanel(width),
			m.renderPromptPanel(width),
			m.renderAssistantPanel(width),
			"",
			m.renderLogsPanel(width),
		)
	}

	leftWidth := max(50, int(float64(width)*0.56))
	rightWidth := max(42, width-leftWidth-2)

	left := lipgloss.JoinVertical(
		lipgloss.Left,
		m.renderOverviewPanel(leftWidth),
		m.renderModelTelemetryPanel(leftWidth),
		m.renderGoalsPanel(leftWidth),
		m.renderGuidebookPanel(leftWidth),
		m.renderTacticalPanel(leftWidth),
	)
	right := lipgloss.JoinVertical(
		lipgloss.Left,
		m.renderSignalsPanel(rightWidth),
		m.renderPromptPanel(rightWidth),
		m.renderAssistantPanel(rightWidth),
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Top, left, right),
		"",
		m.renderLogsPanel(width),
	)
}

func (m *Model) renderOverviewPanel(width int) string {
	screen, headline, runID := m.currentStateHeader()

	lines := []string{
		renderKV(m.loc.Label("Screen", "界面"), screen),
		renderKV(m.loc.Label("Run", "Run"), runID),
		renderKV(m.loc.Label("Headline", "摘要"), headline),
		renderKV(m.loc.Label("Mode", "模式"), m.mode),
		renderKV(m.loc.Label("Provider", "提供方"), m.provider),
		renderKV(m.loc.Label("Provider state", "提供方状态"), m.providerState),
		renderKV(m.loc.Label("Agent available", "模型可用"), fmt.Sprintf("%t", m.agentAvailable)),
		renderKV(m.loc.Label("Forced deterministic", "强制确定性"), fmt.Sprintf("%t", m.forceDeterministic)),
		renderKV(m.loc.Label("Game fast mode", "游戏加速模式"), m.gameFastMode),
		renderKV(m.loc.Label("Model", "模型"), m.model),
		renderKV(m.loc.Label("Backend", "后端"), compactValue(m.endpoint)),
		renderKV(m.loc.Label("Cost", "成本"), fmt.Sprintf("%.4f", m.cost)),
		"",
		labelStyle.Render(m.loc.Label("Actions", "动作")),
		renderActionChips(m.state, max(20, width-6)),
		"",
		labelStyle.Render(m.loc.Label("State summary", "状态摘要")),
		strings.Join(agentruntime.StateSummaryLinesFor(m.state, m.loc.Language()), "\n"),
	}

	return renderPanel(m.loc.Label("Live Run", "实时对局"), strings.Join(lines, "\n"), width)
}

func (m *Model) renderGoalsPanel(width int) string {
	lines := []string{
		renderKV(m.loc.Label("Current", "当前"), compactValue(m.currentGoal)),
		renderKV(m.loc.Label("Room", "房间"), compactValue(m.roomGoal)),
		renderKV(m.loc.Label("Intent", "意图"), compactValue(m.nextIntent)),
		renderKV(m.loc.Label("Recent failure", "最近失败"), compactValue(m.lastFailure)),
		renderKV(m.loc.Label("Carry plan", "跨局计划"), compactValue(m.carryForwardPlan)),
	}

	if !m.carryForwardBuckets.IsEmpty() {
		lines = append(lines, "", labelStyle.Render(m.loc.Label("Carry-forward lessons", "跨局经验")))
		for _, section := range m.carryForwardBuckets.Sections() {
			lines = append(lines, positiveStyle().Render("- "+section.Title))
			for _, lesson := range section.Lessons {
				lines = append(lines, "  "+lesson)
			}
		}
	}

	remaining := agentruntime.UncategorizedLessons(m.carryForwardLessons, m.carryForwardBuckets, 8)
	if len(remaining) > 0 {
		lines = append(lines, "", labelStyle.Render(m.loc.Label("Other lessons", "其他经验")))
		for _, lesson := range remaining {
			lines = append(lines, "- "+lesson)
		}
	}

	return renderPanel(m.loc.Label("Intent & Memory", "意图与记忆"), strings.Join(lines, "\n"), width)
}

func (m *Model) renderTacticalPanel(width int) string {
	lines := []string{labelStyle.Render(m.loc.Label("Tactical hints", "战术提示"))}
	if len(m.tacticalHints) == 0 {
		lines = append(lines, mutedStyle.Render("-"))
	} else {
		for _, hint := range m.tacticalHints {
			lines = append(lines, hintStyle().Render("- "+hint))
		}
	}
	if m.combatPlanSummary != "" || m.combatPlanGoal != "" || m.combatPlanTarget != "" || len(m.combatPlanReasons) > 0 {
		lines = append(lines, "")
		lines = append(lines, labelStyle.Render(m.loc.Label("Combat planner", "战斗规划器")))
		lines = append(lines, renderKV(m.loc.Label("Mode", "模式"), compactValue(m.combatPlannerMode)))
		lines = append(lines, renderKV(m.loc.Label("Summary", "摘要"), compactValue(m.combatPlanSummary)))
		if strings.TrimSpace(m.combatPlanGoal) != "" {
			lines = append(lines, renderKV(m.loc.Label("Primary goal", "主要目标"), compactValue(m.combatPlanGoal)))
		}
		if strings.TrimSpace(m.combatPlanTarget) != "" {
			lines = append(lines, renderKV(m.loc.Label("Target bias", "目标倾向"), compactValue(m.combatPlanTarget)))
		}
		for _, reason := range m.combatPlanReasons {
			lines = append(lines, hintStyle().Render("- "+reason))
		}
		for _, candidate := range m.combatPlanCandidates {
			lines = append(lines, mutedStyle.Render("  -> "+candidate))
		}
	}

	lines = append(lines, "", labelStyle.Render(m.loc.Label("Room detail", "房间细节")))
	lines = append(lines, agentruntime.StateDetailLinesFor(m.state, 6, m.loc.Language())...)
	return renderPanel(m.loc.Label("Board Read", "局面解读"), strings.Join(lines, "\n"), width)
}

func (m *Model) renderGuidebookPanel(width int) string {
	lines := []string{
		renderKV(m.loc.Label("Recent window", "近期窗口"), fmt.Sprintf("%d", m.guideRecentWindow)),
		"",
		labelStyle.Render(m.loc.Label("Recovery hotspots", "恢复热点")),
	}
	if len(m.guideRecentHotspots) == 0 {
		lines = append(lines, mutedStyle.Render("-"))
	} else {
		for _, hotspot := range m.guideRecentHotspots {
			lines = append(lines, negativeStyle().Render("- "+formatRecoveryHotspot(hotspot)))
		}
	}

	lines = append(lines, "", labelStyle.Render(m.loc.Label("Weighted trends", "加权趋势")))
	if len(m.guideWeightedTrends) == 0 {
		lines = append(lines, mutedStyle.Render("-"))
	} else {
		for _, hotspot := range m.guideWeightedTrends {
			lines = append(lines, accentTextStyle().Render("- "+formatRecoveryTrend(hotspot)))
		}
	}

	lines = append(lines, "", labelStyle.Render(m.loc.Label("Known world", "已见世界")))
	if len(m.seenContentCounts) == 0 {
		lines = append(lines, mutedStyle.Render("-"))
	} else {
		for _, line := range m.seenContentCountLines() {
			lines = append(lines, line)
		}
	}

	lines = append(lines, "", labelStyle.Render(m.loc.Label("Recent discoveries", "最近发现")))
	if len(m.recentDiscoveries) == 0 {
		lines = append(lines, mutedStyle.Render("-"))
	} else {
		for _, discovery := range m.recentDiscoveries {
			lines = append(lines, positiveStyle().Render("- "+discovery))
		}
	}

	lines = append(lines, "", labelStyle.Render(m.loc.Label("RL readiness", "RL 就绪度")))
	lines = append(lines,
		renderKV(m.loc.Label("Ready", "就绪"), fmt.Sprintf("%t", m.guideRLReady)),
		renderKV(m.loc.Label("Status", "状态"), compactValue(m.guideRLStatus)),
		renderKV(m.loc.Label("Runs", "完整 run"), fmt.Sprintf("%d / %d", m.guideRLCompleteRuns, m.guideRLRequiredRuns)),
		renderKV(m.loc.Label("Floor >= 15", "15层以上"), fmt.Sprintf("%d / %d", m.guideRLFloor15Runs, m.guideRLRequiredFloor)),
		renderKV(m.loc.Label("Provider-backed", "Provider-backed"), fmt.Sprintf("%d / %d", m.guideRLProviderBackedRuns, m.guideRLRequiredProviderBackedRuns)),
		renderKV(m.loc.Label("Recent clean", "Recent clean"), fmt.Sprintf("%d / %d", m.guideRLRecentCleanRuns, m.guideRLRequiredRecentCleanRuns)),
		renderKV(m.loc.Label("Stable runtime", "运行稳定"), fmt.Sprintf("%t", m.guideRLStable)),
		renderKV(m.loc.Label("Knowledge assets", "知识资产"), fmt.Sprintf("%t", m.guideRLKnowledgeOK)),
		renderKV(m.loc.Label("Clean complete", "Clean complete"), fmt.Sprintf("%d", m.guideRunQualityCleanRuns)),
		renderKV(m.loc.Label("Recent provider-backed", "Recent provider-backed"), fmt.Sprintf("%d", m.guideRunQualityRecentProviderBackedRuns)),
		renderKV(m.loc.Label("Recent fallback", "Recent fallback"), fmt.Sprintf("%d", m.guideRunQualityRecentFallbackRuns)),
		renderKV(m.loc.Label("Recent retries", "Recent retries"), fmt.Sprintf("%d", m.guideRunQualityRecentProviderRetryRuns)),
		renderKV(m.loc.Label("Recent tool errors", "Recent tool errors"), fmt.Sprintf("%d", m.guideRunQualityRecentToolErrorRuns)),
	)

	return renderPanel(m.loc.Label("World & Trends", "世界与趋势"), strings.Join(lines, "\n"), width)
}

func (m *Model) renderSignalsPanel(width int) string {
	lines := []string{
		renderKV(m.loc.Label("Status", "状态"), compactValue(m.lastStatus)),
		renderKV(m.loc.Label("Decision", "决策"), compactValue(m.lastDecision)),
		renderKV(m.loc.Label("Action", "动作"), compactValue(m.lastAction)),
		renderKV(m.loc.Label("Tool", "工具"), compactValue(m.lastTool)),
		renderKV(m.loc.Label("Tool error", "工具错误"), compactValue(m.lastToolError)),
		renderKV(m.loc.Label("Recovery", "恢复"), compactValue(m.lastRecoveryKind)),
		renderKV(m.loc.Label("Drift kind", "漂移类型"), compactValue(m.lastDriftKind)),
		renderKV(m.loc.Label("Attempt lifecycle", "尝试生命周期"), compactValue(m.lastAttemptLifecycle)),
		renderKV(m.loc.Label("Provider recovery", "提供方恢复"), compactValue(m.lastProviderRecovery)),
		renderKV(m.loc.Label("Game fast mode", "游戏加速模式"), compactValue(m.gameFastMode)),
		renderKV(m.loc.Label("Compact", "压缩"), compactValue(m.lastCompact)),
		renderKV(m.loc.Label("Reflection", "反思"), compactValue(m.lastReflection)),
		renderKV(m.loc.Label("Stop", "停止"), compactValue(m.lastStop)),
		"",
		labelStyle.Render(m.loc.Label("Drift expected", "预期状态")),
		compactValue(m.lastDriftExpected),
		"",
		labelStyle.Render(m.loc.Label("Drift live", "实际状态")),
		compactValue(m.lastDriftLive),
	}
	if m.gameFastModeChanged {
		lines = append(lines, renderKV(m.loc.Label("Fast mode change", "加速模式变更"), compactValue(m.gameFastModePrevious+" -> "+m.gameFastMode)))
	}

	return renderPanel(m.loc.Label("Signals & Recovery", "信号与恢复"), strings.Join(lines, "\n"), width)
}

func (m *Model) renderModelTelemetryPanel(width int) string {
	lines := []string{
		renderKV("Latency", formatDurationMs(m.lastCycleDurationMs)),
		renderKV("Input tokens", fmt.Sprintf("%d", m.lastInputTokens)),
		renderKV("Output tokens", fmt.Sprintf("%d", m.lastOutputTokens)),
		renderKV("Prompt size", formatBytes(m.lastPromptSizeBytes)),
	}

	return renderPanel("Model Telemetry", strings.Join(lines, "\n"), width)
}

func (m *Model) renderPromptPanel(width int) string {
	body := previewStyle().Width(max(20, width-6)).Render(strings.Join(previewBlock(m.lastPrompt, 12), "\n"))
	return renderPanel(m.loc.Label("Prompt Preview", "提示词预览"), body, width)
}

func (m *Model) renderAssistantPanel(width int) string {
	lines := []string{
		previewStyle().Width(max(20, width-6)).Render(strings.Join(previewBlock(m.lastAssistant, 10), "\n")),
	}
	if strings.TrimSpace(m.lastReflectionStory) != "" {
		lines = append(lines, "")
		lines = append(lines, labelStyle.Render(m.loc.Label("Latest reflection echo", "最近反思回声")))
		lines = append(lines, previewStyle().Width(max(20, width-6)).Render(strings.Join(previewBlock(m.lastReflectionStory, 8), "\n")))
	}

	return renderPanel(m.loc.Label("Assistant & Story", "模型与故事"), strings.Join(lines, "\n"), width)
}

func (m *Model) renderLogsPanel(width int) string {
	lines := make([]string, 0, len(m.logs))
	if len(m.logs) == 0 {
		lines = append(lines, mutedStyle.Render("-"))
	} else {
		for _, line := range m.logs {
			lines = append(lines, renderLogLine(line))
		}
	}

	return renderPanel(m.loc.Label("Recent Event Stream", "最近事件流"), strings.Join(lines, "\n"), width)
}

func renderPanel(title, body string, width int) string {
	innerWidth := max(20, width-6)
	return panelStyle.Width(width).Render(lipgloss.JoinVertical(
		lipgloss.Left,
		panelTitleStyle.Render(title),
		"",
		lipgloss.NewStyle().Width(innerWidth).Render(body),
	))
}

func renderKV(label, value string) string {
	return labelStyle.Render(label+": ") + valueStyle.Render(value)
}

func (m *Model) currentStateHeader() (string, string, string) {
	if m.state == nil {
		return "-", "-", "-"
	}

	screen := strings.TrimSpace(m.state.Screen)
	if screen == "" {
		screen = "-"
	}
	runID := strings.TrimSpace(m.state.RunID)
	if runID == "" {
		runID = "-"
	}

	headline := "-"
	if m.state.AgentView != nil {
		if raw, ok := m.state.AgentView["headline"].(string); ok && strings.TrimSpace(raw) != "" {
			headline = raw
		}
	}

	return screen, headline, runID
}

func renderLogLine(line string) string {
	trimmed := strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(trimmed, "[tool-error]"):
		return negativeStyle().Render(trimmed)
	case strings.HasPrefix(trimmed, "[reflection]"), strings.HasPrefix(trimmed, "[compact]"):
		return accentTextStyle().Render(trimmed)
	case strings.HasPrefix(trimmed, "[assistant]"):
		return positiveStyle().Render(trimmed)
	case strings.HasPrefix(trimmed, "[action]"):
		return infoBadgeStyle().Render(trimmed)
	default:
		return logStyle.Render(trimmed)
	}
}

func renderActionChips(state *game.StateSnapshot, width int) string {
	if state == nil {
		return mutedStyle.Render("-")
	}
	return renderActionChipsFromList(state.AvailableActions, width)
}

func renderActionChipsFromList(actions []string, width int) string {
	if len(actions) == 0 {
		return mutedStyle.Render("-")
	}

	chips := make([]string, 0, len(actions))
	for _, action := range actions {
		chips = append(chips, actionChipStyle().Render(action))
	}
	return wrapHorizontal(chips, width)
}

func wrapHorizontal(items []string, width int) string {
	if len(items) == 0 {
		return ""
	}
	if width <= 0 {
		return strings.Join(items, " ")
	}

	lines := []string{}
	current := items[0]
	for _, item := range items[1:] {
		candidate := current + " " + item
		if lipgloss.Width(candidate) > width {
			lines = append(lines, current)
			current = item
			continue
		}
		current = candidate
	}
	lines = append(lines, current)
	return strings.Join(lines, "\n")
}

func compactValue(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "-"
	}
	if len(value) <= 64 {
		return value
	}
	return value[:61] + "..."
}

func formatDurationMs(ms int64) string {
	if ms <= 0 {
		return "-"
	}
	return fmt.Sprintf("%d ms", ms)
}

func formatBytes(size int) string {
	if size <= 0 {
		return "-"
	}
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	return fmt.Sprintf("%.1f KB", float64(size)/1024.0)
}

func previewBlock(value string, maxLines int) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return []string{"-"}
	}

	lines := strings.Split(value, "\n")
	if len(lines) > maxLines {
		lines = append(lines[:maxLines], "...")
	}
	return lines
}

func appendTrimmed(items []string, item string, max int) []string {
	item = strings.TrimSpace(item)
	if item == "" {
		return items
	}
	items = append(items, item)
	if len(items) > max {
		items = items[len(items)-max:]
	}
	return items
}

func interfaceStrings(values []interface{}) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		text, ok := value.(string)
		if !ok {
			continue
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}
		result = append(result, text)
	}
	return result
}

func accentTextStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(accentText)
}

func (m *Model) seenContentCountLines() []string {
	keys := make([]string, 0, len(m.seenContentCounts))
	for key := range m.seenContentCounts {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	lines := make([]string, 0, len(keys))
	for _, key := range keys {
		lines = append(lines, renderKV(m.seenCategoryLabel(key), fmt.Sprintf("%d", m.seenContentCounts[key])))
	}
	return lines
}

func (m *Model) seenCategoryLabel(category string) string {
	switch strings.TrimSpace(category) {
	case "cards":
		return m.loc.Label("Cards seen", "已见卡牌")
	case "relics":
		return m.loc.Label("Relics seen", "已见遗物")
	case "potions":
		return m.loc.Label("Potions seen", "已见药水")
	case "monsters":
		return m.loc.Label("Monsters seen", "已见怪物")
	case "events":
		return m.loc.Label("Events seen", "已见事件")
	case "characters":
		return m.loc.Label("Characters seen", "已见角色")
	default:
		return category
	}
}

func formatRecoveryHotspot(hotspot agentruntime.RecoveryHotspot) string {
	return fmt.Sprintf("%s (%d)", recoveryHotspotLabel(hotspot), hotspot.Count)
}

func formatRecoveryTrend(hotspot agentruntime.RecoveryHotspot) string {
	return fmt.Sprintf("%s (%.2f)", recoveryHotspotLabel(hotspot), hotspot.Score)
}

func recoveryHotspotLabel(hotspot agentruntime.RecoveryHotspot) string {
	parts := []string{}
	if kind := strings.TrimSpace(hotspot.RecoveryKind); kind != "" {
		parts = append(parts, kind)
	}
	if kind := strings.TrimSpace(hotspot.DriftKind); kind != "" {
		parts = append(parts, kind)
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, " / ")
}
