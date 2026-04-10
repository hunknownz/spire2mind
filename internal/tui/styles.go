package tui

import "github.com/charmbracelet/lipgloss"

var (
	appBackground = lipgloss.Color("#0A1224")
	panelBg       = lipgloss.Color("#121B30")
	panelBorder   = lipgloss.Color("#34507A")
	panelGlow     = lipgloss.Color("#4E74AE")
	mutedText     = lipgloss.Color("#93A9D1")
	brightText    = lipgloss.Color("#F4F7FF")
	hotText       = lipgloss.Color("#F6C76D")
	dangerText    = lipgloss.Color("#FF8A8A")
	successText   = lipgloss.Color("#85E2B8")
	infoText      = lipgloss.Color("#82CBFF")
	accentText    = lipgloss.Color("#8BC5FF")
	violetText    = lipgloss.Color("#C4A8FF")
	cyanText      = lipgloss.Color("#8EF0FF")
)

var (
	rootStyle = lipgloss.NewStyle().
			Background(appBackground).
			Foreground(brightText).
			Padding(1, 1)

	headerStyle = lipgloss.NewStyle().
			Foreground(brightText).
			Bold(true).
			Padding(0, 1)

	subtitleStyle = lipgloss.NewStyle().
			Foreground(mutedText)

	panelStyle = lipgloss.NewStyle().
			Background(panelBg).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(panelBorder).
			Padding(1, 2)

	panelTitleStyle = lipgloss.NewStyle().
			Foreground(hotText).
			Bold(true).
			Underline(true)

	labelStyle = lipgloss.NewStyle().
			Foreground(mutedText)

	mutedStyle = lipgloss.NewStyle().
			Foreground(mutedText)

	valueStyle = lipgloss.NewStyle().
			Foreground(brightText)

	logStyle = lipgloss.NewStyle().
			Foreground(brightText)

	footerStyle = lipgloss.NewStyle().
			Foreground(mutedText).
			PaddingTop(1)
)

func statusBadgeStyle(value string) lipgloss.Style {
	style := lipgloss.NewStyle().
		Foreground(brightText).
		Background(lipgloss.Color("#27304A")).
		Padding(0, 1).
		Bold(true)

	switch value {
	case "running", "healthy":
		return style.Background(lipgloss.Color("#1E4D3A")).Foreground(successText)
	case "paused":
		return style.Background(lipgloss.Color("#374565")).Foreground(hotText)
	case "error", "fallback":
		return style.Background(lipgloss.Color("#5A1E2A")).Foreground(dangerText)
	case "done":
		return style.Background(lipgloss.Color("#2F3F63")).Foreground(infoText)
	case "recovering", "retrying", "soft_replan", "hard_replan", "decision_reuse", "decision_remap", "transport_restart", "provider_retry":
		return style.Background(lipgloss.Color("#4B3A18")).Foreground(hotText)
	default:
		return style
	}
}

func infoBadgeStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(brightText).
		Background(lipgloss.Color("#243556")).
		Padding(0, 1)
}

func actionChipStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(brightText).
		Background(lipgloss.Color("#3B2C57")).
		Padding(0, 1)
}

func hintStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(hotText)
}

func negativeStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(dangerText)
}

func positiveStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(successText)
}

func previewStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(brightText).
		Background(lipgloss.Color("#0C1427")).
		Border(lipgloss.NormalBorder()).
		BorderForeground(panelGlow).
		Padding(0, 1)
}

func headerGlowStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(cyanText).
		Bold(true)
}

func waitingBadgeStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(cyanText).
		Background(lipgloss.Color("#17374A")).
		Padding(0, 1).
		Bold(true)
}

func streamerBlockStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(brightText).
		Background(lipgloss.Color("#101932")).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(violetText).
		Padding(0, 1)
}
