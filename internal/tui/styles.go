package tui

import "github.com/charmbracelet/lipgloss"

var (
	appBackground = lipgloss.Color("#0B1020")
	panelBg       = lipgloss.Color("#11182A")
	panelBorder   = lipgloss.Color("#2B3859")
	mutedText     = lipgloss.Color("#8CA0C7")
	brightText    = lipgloss.Color("#E8F0FF")
	hotText       = lipgloss.Color("#F7C66A")
	dangerText    = lipgloss.Color("#FF7D7D")
	successText   = lipgloss.Color("#7EE0B1")
	infoText      = lipgloss.Color("#74C0FC")
	accentText    = lipgloss.Color("#C39BFF")
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
			Bold(true)

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
		Background(lipgloss.Color("#0C1322")).
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("#23314D")).
		Padding(0, 1)
}
