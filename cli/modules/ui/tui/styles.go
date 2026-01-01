package tui

import (
	"github.com/charmbracelet/lipgloss"
)

// Color palette
var (
	ColorPrimary   = lipgloss.Color("#7C3AED") // Purple
	ColorSecondary = lipgloss.Color("#06B6D4") // Cyan
	ColorSuccess   = lipgloss.Color("#10B981") // Green
	ColorWarning   = lipgloss.Color("#F59E0B") // Orange
	ColorError     = lipgloss.Color("#EF4444") // Red
	ColorMuted     = lipgloss.Color("#6B7280") // Gray
	ColorText      = lipgloss.Color("#F9FAFB") // Light
	ColorBg        = lipgloss.Color("#111827") // Dark
	ColorBgAlt     = lipgloss.Color("#1F2937") // Dark alt
	ColorBorder    = lipgloss.Color("#374151") // Gray border
)

// Base styles
var (
	BaseStyle = lipgloss.NewStyle().
			Background(ColorBg).
			Foreground(ColorText)

	// Header
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary).
			Background(ColorBgAlt).
			Padding(0, 1)

	// Title
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorPrimary)

	// Subtitle
	SubtitleStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	// Status indicators
	StatusRunning = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true)

	StatusStopped = lipgloss.NewStyle().
			Foreground(ColorMuted)

	StatusSuccess = lipgloss.NewStyle().
			Foreground(ColorSuccess).
			Bold(true)

	StatusError = lipgloss.NewStyle().
			Foreground(ColorError).
			Bold(true)

	StatusWarning = lipgloss.NewStyle().
			Foreground(ColorWarning)

	StatusBuilding = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true)

	// Navigation
	NavItemStyle = lipgloss.NewStyle().
			Padding(0, 2)

	NavItemActiveStyle = lipgloss.NewStyle().
				Padding(0, 2).
				Background(ColorPrimary).
				Foreground(ColorText).
				Bold(true)

	// Panels
	PanelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(1)

	PanelTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(ColorSecondary).
			MarginBottom(1)

	// Table
	TableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorSecondary).
				BorderBottom(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(ColorBorder)

	TableRowStyle = lipgloss.NewStyle().
			Foreground(ColorText)

	TableRowSelectedStyle = lipgloss.NewStyle().
				Background(ColorBgAlt).
				Foreground(ColorText).
				Bold(true)

	// Logs
	LogInfoStyle = lipgloss.NewStyle().
			Foreground(ColorText)

	LogWarnStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)

	LogErrorStyle = lipgloss.NewStyle().
			Foreground(ColorError)

	LogDebugStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	LogTimestampStyle = lipgloss.NewStyle().
				Foreground(ColorMuted)

	LogSourceStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary)

	// Git
	GitBranchStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary)

	GitCleanStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	GitDirtyStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)

	GitAheadStyle = lipgloss.NewStyle().
			Foreground(ColorSuccess)

	GitBehindStyle = lipgloss.NewStyle().
			Foreground(ColorWarning)

	// Notifications
	NotifyInfoStyle = lipgloss.NewStyle().
			Background(ColorSecondary).
			Foreground(ColorText).
			Padding(0, 1)

	NotifySuccessStyle = lipgloss.NewStyle().
				Background(ColorSuccess).
				Foreground(ColorText).
				Padding(0, 1)

	NotifyWarningStyle = lipgloss.NewStyle().
				Background(ColorWarning).
				Foreground(ColorBg).
				Padding(0, 1)

	NotifyErrorStyle = lipgloss.NewStyle().
				Background(ColorError).
				Foreground(ColorText).
				Padding(0, 1)

	// Help
	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true)

	HelpDescStyle = lipgloss.NewStyle().
			Foreground(ColorMuted)

	// Progress bar
	ProgressBarFilled = lipgloss.NewStyle().
				Background(ColorSuccess)

	ProgressBarEmpty = lipgloss.NewStyle().
				Background(ColorBgAlt)

	// Input
	InputStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorBorder).
			Padding(0, 1)

	InputFocusedStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary).
				Padding(0, 1)

	// Dialog
	DialogStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorPrimary).
			Padding(1, 2).
			Background(ColorBgAlt)

	DialogTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorPrimary).
				MarginBottom(1)

	// Button
	ButtonStyle = lipgloss.NewStyle().
			Padding(0, 2).
			Background(ColorBgAlt).
			Foreground(ColorText)

	ButtonActiveStyle = lipgloss.NewStyle().
				Padding(0, 2).
				Background(ColorPrimary).
				Foreground(ColorText).
				Bold(true)
)

// Icons (using Unicode symbols for cross-platform compatibility)
const (
	IconRunning  = "●"
	IconStopped  = "○"
	IconError    = "✗"
	IconSuccess  = "✓"
	IconWarning  = "⚠"
	IconBuilding = "◐"
	IconGitClean = "✓"
	IconGitDirty = "✗"
	IconArrowUp  = "↑"
	IconArrowDn  = "↓"
	IconBranch   = "⎇"
	IconFolder   = "📁"
	IconFile     = "📄"
	IconGear     = "⚙"
	IconPlay     = "▶"
	IconStop     = "■"
	IconRefresh  = "↻"
)

// Helper functions
func StatusIcon(state string) string {
	switch state {
	case "running":
		return StatusRunning.Render(IconRunning)
	case "stopped":
		return StatusStopped.Render(IconStopped)
	case "crashed", "error":
		return StatusError.Render(IconError)
	case "building":
		return StatusBuilding.Render(IconBuilding)
	default:
		return StatusStopped.Render(IconStopped)
	}
}

func GitStatusIcon(clean bool) string {
	if clean {
		return GitCleanStyle.Render(IconGitClean)
	}
	return GitDirtyStyle.Render(IconGitDirty)
}
