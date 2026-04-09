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
		width = 168
	}

	header := m.renderHeader(width)
	body := m.renderBody(width)
	footer := footerStyle.Render(m.loc.Label(
		"p pause/resume | q quit | ctrl+c stop | this TUI mirrors the live autoplay runtime",
		"p 暂停/恢复 | q 退出 | ctrl+c 停止 | 这个 TUI 镜像实时自动游玩运行时",
	))

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
	subtitle := subtitleStyle.Render(m.loc.Label("Live STS2 cockpit", "实时 STS2 驾驶舱"))
	left := lipgloss.JoinVertical(lipgloss.Left, title, subtitle)
	screen, _, _ := m.currentStateHeader()

	status := m.status
	if m.paused {
		status = "paused"
	}

	right := lipgloss.JoinHorizontal(
		lipgloss.Left,
		statusBadgeStyle(status).Render(m.loc.Label("status", "状态")+" "+compactValue(status)),
		" ",
		statusBadgeStyle(m.providerState).Render(m.loc.Label("provider", "模型")+" "+compactValue(m.providerState)),
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
	if width < 140 {
		return lipgloss.JoinVertical(
			lipgloss.Left,
			m.renderOverviewPanel(width),
			m.renderGoalsPanel(width),
			m.renderBoardPanel(width),
			m.renderGuidebookPanel(width),
			m.renderStreamerPanel(width),
			m.renderSignalsPanel(width),
			m.renderPromptPanel(width),
			m.renderAssistantPanel(width),
			m.renderLogsPanel(width),
		)
	}

	leftWidth := max(58, int(float64(width)*0.40))
	midWidth := max(54, int(float64(width)*0.30))
	rightWidth := max(48, width-leftWidth-midWidth-4)

	left := lipgloss.JoinVertical(
		lipgloss.Left,
		m.renderOverviewPanel(leftWidth),
		m.renderGoalsPanel(leftWidth),
		m.renderGuidebookPanel(leftWidth),
	)
	mid := lipgloss.JoinVertical(
		lipgloss.Left,
		m.renderBoardPanel(midWidth),
		m.renderStreamerPanel(midWidth),
		m.renderSignalsPanel(midWidth),
	)
	right := lipgloss.JoinVertical(
		lipgloss.Left,
		m.renderModelTelemetryPanel(rightWidth),
		m.renderPromptPanel(rightWidth),
		m.renderAssistantPanel(rightWidth),
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Top, left, "  ", mid, "  ", right),
		"",
		m.renderLogsPanel(width),
	)
}

func (m *Model) renderOverviewPanel(width int) string {
	screen, headline, runID := m.currentStateHeader()
	lines := []string{
		renderKV(m.loc.Label("Screen", "界面"), screen),
		renderKV(m.loc.Label("Run", "对局"), runID),
		renderKV(m.loc.Label("Headline", "摘要"), headline),
		renderKV(m.loc.Label("Mode", "模式"), m.mode),
		renderKV(m.loc.Label("Provider", "后端"), m.provider),
		renderKV(m.loc.Label("Provider state", "后端状态"), m.providerState),
		renderKV(m.loc.Label("Agent available", "模型可用"), fmt.Sprintf("%t", m.agentAvailable)),
		renderKV(m.loc.Label("Forced deterministic", "强制确定性"), fmt.Sprintf("%t", m.forceDeterministic)),
		renderKV(m.loc.Label("Fast mode", "加速模式"), m.gameFastMode),
		renderKV(m.loc.Label("Model", "模型"), m.model),
		renderKV(m.loc.Label("Backend", "地址"), compactValue(m.endpoint)),
		renderKV(m.loc.Label("Cost", "成本"), fmt.Sprintf("%.4f", m.cost)),
		"",
		labelStyle.Render(m.loc.Label("Available actions", "可用动作")),
		renderActionChips(m.state, max(24, width-6)),
		"",
		labelStyle.Render(m.loc.Label("State summary", "状态摘要")),
		strings.Join(agentruntime.StateSummaryLinesFor(m.state, m.loc.Language()), "\n"),
	}
	return renderPanel(m.loc.Label("Live Run", "实时对局"), strings.Join(lines, "\n"), width)
}

func (m *Model) renderGoalsPanel(width int) string {
	lines := []string{
		renderKV(m.loc.Label("Current goal", "当前目标"), compactValue(m.currentGoal)),
		renderKV(m.loc.Label("Room goal", "房间目标"), compactValue(m.roomGoal)),
		renderKV(m.loc.Label("Next intent", "下一步意图"), compactValue(m.nextIntent)),
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

	return renderPanel(m.loc.Label("Plan & Memory", "计划与记忆"), strings.Join(lines, "\n"), width)
}

func (m *Model) renderBoardPanel(width int) string {
	lines := []string{}
	if len(m.tacticalHints) > 0 {
		lines = append(lines, labelStyle.Render(m.loc.Label("Tactical hints", "战术提示")))
		for _, hint := range m.tacticalHints {
			lines = append(lines, hintStyle().Render("- "+hint))
		}
		lines = append(lines, "")
	}

	if m.combatPlanSummary != "" || m.combatPlanGoal != "" || m.combatPlanTarget != "" || len(m.combatPlanCandidates) > 0 {
		lines = append(lines, labelStyle.Render(m.loc.Label("Combat planner", "战斗规划器")))
		lines = append(lines, renderKV(m.loc.Label("Mode", "模式"), compactValue(m.combatPlannerMode)))
		lines = append(lines, renderKV(m.loc.Label("Summary", "摘要"), compactValue(m.combatPlanSummary)))
		if strings.TrimSpace(m.combatPlanGoal) != "" {
			lines = append(lines, renderKV(m.loc.Label("Primary goal", "主目标"), compactValue(m.combatPlanGoal)))
		}
		if strings.TrimSpace(m.combatPlanTarget) != "" {
			lines = append(lines, renderKV(m.loc.Label("Target bias", "目标偏置"), compactValue(m.combatPlanTarget)))
		}
		for _, reason := range m.combatPlanReasons {
			lines = append(lines, "- "+reason)
		}
		for _, candidate := range m.combatPlanCandidates {
			lines = append(lines, mutedStyle.Render("  -> "+candidate))
		}
		lines = append(lines, "")
	}

	lines = append(lines, labelStyle.Render(m.loc.Label("Room detail", "房间细节")))
	lines = append(lines, agentruntime.StateDetailLinesFor(m.state, 8, m.loc.Language())...)
	return renderPanel(m.loc.Label("Board Read", "局面解读"), strings.Join(lines, "\n"), width)
}

func (m *Model) renderGuidebookPanel(width int) string {
	lines := []string{
		renderKV(m.loc.Label("Recent median floor", "最近中位层数"), fmt.Sprintf("%d", m.guideRunQualityRecentMedianFloor)),
		renderKV(m.loc.Label("Recent best floor", "最近最佳层数"), fmt.Sprintf("%d", m.guideRunQualityRecentBestFloor)),
		renderKV(m.loc.Label("Floor >= 7", "到达 7 层及以上"), fmt.Sprintf("%d", m.guideRunQualityRecentFloor7PlusRuns)),
		renderKV(m.loc.Label("Act 2 entries", "进入 Act 2"), fmt.Sprintf("%d", m.guideRunQualityRecentAct2EntryRuns)),
		renderKV(m.loc.Label("Died with gold", "带钱阵亡"), fmt.Sprintf("%d", m.guideRunQualityRecentDiedWithGoldRuns)),
		renderKV(m.loc.Label("Average death gold", "阵亡平均金币"), fmt.Sprintf("%d", m.guideRunQualityRecentAverageDeathGold)),
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

	lines = append(lines, "", labelStyle.Render(m.loc.Label("Known world", "已知世界")))
	if len(m.seenContentCounts) == 0 {
		lines = append(lines, mutedStyle.Render("-"))
	} else {
		lines = append(lines, m.seenContentCountLines()...)
	}

	return renderPanel(m.loc.Label("World & Progress", "世界与进度"), strings.Join(lines, "\n"), width)
}

func (m *Model) renderStreamerPanel(width int) string {
	lines := []string{
		renderKV(m.loc.Label("TTS profile", "TTS 方案"), compactValue(m.ttsProfileName)),
		renderKV(m.loc.Label("TTS provider", "TTS 后端"), compactValue(m.ttsProfileProvider)),
		renderKV(m.loc.Label("Voice", "音色"), compactValue(m.ttsProfileVoice)),
		renderKV(m.loc.Label("Speed", "语速"), compactValue(m.ttsProfileSpeed)),
		renderKV(m.loc.Label("Streamer style", "主播风格"), compactValue(m.streamerStyle)),
		"",
		renderKV(m.loc.Label("Mood", "情绪"), compactValue(m.lastStreamerMood)),
		renderKV(m.loc.Label("Commentary", "解说"), compactValue(m.lastStreamerCommentary)),
		renderKV(m.loc.Label("Game insight", "游戏见解"), compactValue(m.lastStreamerInsight)),
		renderKV(m.loc.Label("Life reflection", "人生感慨"), compactValue(m.lastStreamerReflection)),
		renderKV(m.loc.Label("TTS text", "播报文本"), compactValue(m.lastStreamerTTS)),
	}
	return renderPanel(m.loc.Label("Streamer Booth", "主播机位"), strings.Join(lines, "\n"), width)
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
		renderKV(m.loc.Label("Provider recovery", "模型恢复"), compactValue(m.lastProviderRecovery)),
		renderKV(m.loc.Label("Compact", "压缩"), compactValue(m.lastCompact)),
		renderKV(m.loc.Label("Reflection", "反思"), compactValue(m.lastReflection)),
	}
	return renderPanel(m.loc.Label("Signals & Recovery", "信号与恢复"), strings.Join(lines, "\n"), width)
}

func (m *Model) renderModelTelemetryPanel(width int) string {
	lines := []string{
		renderKV(m.loc.Label("Latency", "延迟"), formatDurationMs(m.lastCycleDurationMs)),
		renderKV(m.loc.Label("Input tokens", "输入 tokens"), fmt.Sprintf("%d", m.lastInputTokens)),
		renderKV(m.loc.Label("Output tokens", "输出 tokens"), fmt.Sprintf("%d", m.lastOutputTokens)),
		renderKV(m.loc.Label("Prompt size", "提示词大小"), formatBytes(m.lastPromptSizeBytes)),
	}
	return renderPanel(m.loc.Label("Model Telemetry", "模型遥测"), strings.Join(lines, "\n"), width)
}

func (m *Model) renderPromptPanel(width int) string {
	body := previewStyle().Width(max(24, width-6)).Render(strings.Join(previewBlock(m.lastPrompt, 18), "\n"))
	return renderPanel(m.loc.Label("Prompt Preview", "提示词预览"), body, width)
}

func (m *Model) renderAssistantPanel(width int) string {
	lines := []string{
		previewStyle().Width(max(24, width-6)).Render(strings.Join(previewBlock(m.lastAssistant, 14), "\n")),
	}
	if strings.TrimSpace(m.lastReflectionStory) != "" {
		lines = append(lines, "")
		lines = append(lines, labelStyle.Render(m.loc.Label("Latest reflection story", "最近反思故事")))
		lines = append(lines, previewStyle().Width(max(24, width-6)).Render(strings.Join(previewBlock(m.lastReflectionStory, 10), "\n")))
	}
	return renderPanel(m.loc.Label("Model & Story", "模型与故事"), strings.Join(lines, "\n"), width)
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
	case strings.HasPrefix(trimmed, "[assistant]"), strings.HasPrefix(trimmed, "[streamer]"):
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
	runes := []rune(value)
	if len(runes) <= 80 {
		return value
	}
	return string(runes[:77]) + "..."
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
