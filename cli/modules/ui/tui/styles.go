package tui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

// Layout constants
const (
	GapHorizontal = 1 // Horizontal gap between panels/cards
	GapVertical   = 1 // Vertical gap between sections
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

// ShortcutStyle is the style used for highlighting shortcut keys in labels
var ShortcutStyle = lipgloss.NewStyle().
	Foreground(ColorSecondary).
	Bold(true)

// SupportsColoredShortcuts returns true if terminal supports colors for shortcuts
func SupportsColoredShortcuts() bool {
	profile := lipgloss.ColorProfile()
	return profile != termenv.Ascii
}

// StripShortcutBrackets removes [X] syntax from label, returning clean text and shortcut position
// Returns the clean label and the position of the shortcut character (-1 if not found)
// Example: "[D]ashboard" -> ("Dashboard", 0)
// Example: "Coc[K]pit" -> ("Cockpit", 3)
func StripShortcutBrackets(label string) (clean string, shortcutPos int) {
	// Find [X] pattern (single character in brackets)
	for i := 0; i < len(label); i++ {
		if label[i] == '[' && i+2 < len(label) && label[i+2] == ']' {
			// Found [X] - remove brackets
			before := label[:i]
			shortcut := string(label[i+1])
			after := label[i+3:]
			return before + shortcut + after, len(before)
		}
	}
	// No [X] pattern found
	return label, -1
}

// ApplyShortcutColor applies color to the character at the given position
// Used after truncation to color the shortcut if still visible
func ApplyShortcutColor(label string, pos int) string {
	if pos < 0 || pos >= len(label) {
		return label
	}
	// Don't color if position is in the "..." suffix
	if len(label) > 3 && label[len(label)-3:] == "..." && pos >= len(label)-3 {
		return label
	}
	runes := []rune(label)
	if pos >= len(runes) {
		return label
	}
	before := string(runes[:pos])
	char := string(runes[pos])
	after := string(runes[pos+1:])
	return before + ShortcutStyle.Render(char) + after
}

// RenderShortcutLabel renders a label with [X] syntax, using color if supported
// or keeping brackets if the terminal doesn't support styling.
// NOTE: This is the simple version. For proper truncation handling,
// use StripShortcutBrackets + truncate + ApplyShortcutColor
func RenderShortcutLabel(label string) string {
	if !SupportsColoredShortcuts() {
		// No color support - keep brackets as-is
		return label
	}
	clean, pos := StripShortcutBrackets(label)
	return ApplyShortcutColor(clean, pos)
}
