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

// Color palette - Aligned with csd-core/frontend theme
var (
	ColorPrimary   = lipgloss.Color("#00d4ff") // Cyan - CSD brand accent
	ColorSecondary = lipgloss.Color("#1976d2") // Blue
	ColorSuccess   = lipgloss.Color("#48bb78") // Green
	ColorWarning   = lipgloss.Color("#ed8936") // Orange
	ColorError     = lipgloss.Color("#f56565") // Red
	ColorInfo      = lipgloss.Color("#00d4ff") // Cyan (same as primary)
	ColorMuted     = lipgloss.Color("#6B7280") // Gray
	ColorText      = lipgloss.Color("#f7fafc") // Light text
	ColorTextAlt   = lipgloss.Color("#cbd5e0") // Secondary text
	ColorBg        = lipgloss.Color("#1e1e2e") // Dark background (csd-core)
	ColorBgAlt     = lipgloss.Color("#252530") // Dark alt - selection/hover (csd-core)
	ColorSidebar   = lipgloss.Color("#1a1a25") // Sidebar background (csd-core)
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

	// Help (with footer background for proper nested styling)
	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Background(ColorBgAlt).
			Bold(true)

	HelpDescStyle = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Background(ColorBgAlt)

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
	return ApplyShortcutColorWithBg(label, pos, nil)
}

// ApplyShortcutColorWithBg applies color to the character at the given position with optional background
// Used for selected items where the shortcut needs to preserve the selection background
// When bg is provided, the ENTIRE label gets the background, with the shortcut char also getting the accent color
func ApplyShortcutColorWithBg(label string, pos int, bg lipgloss.TerminalColor) string {
	if pos < 0 || pos >= len(label) {
		// No shortcut, but still apply background if provided
		if bg != nil {
			return lipgloss.NewStyle().Background(bg).Render(label)
		}
		return label
	}
	// Don't color if position is in the "..." suffix
	if len(label) > 3 && label[len(label)-3:] == "..." && pos >= len(label)-3 {
		if bg != nil {
			return lipgloss.NewStyle().Background(bg).Render(label)
		}
		return label
	}
	runes := []rune(label)
	if pos >= len(runes) {
		if bg != nil {
			return lipgloss.NewStyle().Background(bg).Render(label)
		}
		return label
	}
	before := string(runes[:pos])
	char := string(runes[pos])
	after := string(runes[pos+1:])

	// Shortcut style with accent color
	shortcutStyle := ShortcutStyle
	if bg != nil {
		shortcutStyle = shortcutStyle.Background(bg)
	}

	// Background style for non-shortcut parts
	if bg != nil {
		bgStyle := lipgloss.NewStyle().Background(bg)
		return bgStyle.Render(before) + shortcutStyle.Render(char) + bgStyle.Render(after)
	}
	return before + shortcutStyle.Render(char) + after
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
