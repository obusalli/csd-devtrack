package tui

import (
	"fmt"
	"strings"
	"time"

	"csd-devtrack/cli/modules/ui/core"

	"github.com/charmbracelet/lipgloss"
)

// Claude view modes
const (
	ClaudeModeSession = "sessions"
	ClaudeModeChat    = "chat"
)

// claudeTreeItem represents an item in the sessions tree
type claudeTreeItem struct {
	IsProject bool   // true = project, false = session
	ProjectID string // Project ID
	SessionID string // Session ID (empty for projects)
}

// renderClaude renders the Claude AI view
// Layout: Chat on left (70%), Sessions panel on right (30%)
func (m *Model) renderClaude(width, height int) string {
	// Check if Claude is installed
	if m.state.Claude == nil || !m.state.Claude.IsInstalled {
		return m.renderClaudeNotInstalled(width, height)
	}

	// Always chat mode now (no more tabs)
	if m.claudeMode == "" || m.claudeMode == ClaudeModeSession {
		m.claudeMode = ClaudeModeChat
	}

	// Full height for content (no tabs)
	contentHeight := height - 2

	// Calculate sessions panel width using TreeMenu
	sessionsWidth := m.sessionsTreeMenu.CalcWidth()
	chatWidth := width - sessionsWidth - GapHorizontal

	// Session info takes some space at bottom
	infoHeight := 8
	treeHeight := contentHeight - infoHeight

	// Configure and render TreeMenu
	m.sessionsTreeMenu.SetSize(sessionsWidth, treeHeight)
	m.sessionsTreeMenu.SetFocused(m.focusArea == FocusDetail)

	chatPanel := m.renderClaudeChatPanel(chatWidth, contentHeight)
	treePanel := m.sessionsTreeMenu.Render()
	infoPanel := m.renderSessionInfo(sessionsWidth, infoHeight)
	sessionsPanel := lipgloss.JoinVertical(lipgloss.Left, treePanel, infoPanel)

	return lipgloss.JoinHorizontal(lipgloss.Top,
		chatPanel,
		strings.Repeat(" ", GapHorizontal),
		sessionsPanel,
	)
}

// renderSessionInfo renders selected session information
// Format inspired by Claude CLI --resume picker
func (m *Model) renderSessionInfo(width, height int) string {
	// Get selected session from TreeMenu
	var sess *core.ClaudeSessionVM
	if m.sessionsTreeMenu != nil {
		if item := m.sessionsTreeMenu.SelectedItem(); item != nil {
			if s, ok := item.Data.(core.ClaudeSessionVM); ok {
				sess = &s
			}
		}
	}

	// Use same border style as TreeMenu for alignment
	borderStyle := UnfocusedBorderStyle

	// Adjust width for right-side panel border
	renderWidth := width - 5
	if renderWidth < 20 {
		renderWidth = 20
	}

	if sess == nil {
		// No session selected - show placeholder
		content := lipgloss.NewStyle().
			Foreground(ColorMuted).
			Render("Select a session")

		return borderStyle.
			Width(renderWidth).
			Height(height).
			Align(lipgloss.Center, lipgloss.Center).
			Render(content)
	}

	// Inner dimensions (inside border)
	innerWidth := renderWidth - 2
	innerHeight := height - 2
	contentWidth := innerWidth - 2 // padding

	mutedStyle := lipgloss.NewStyle().Foreground(ColorMuted)
	valueStyle := lipgloss.NewStyle().Foreground(ColorText)

	var lines []string

	// First line: Relative time · message count
	relTime := formatRelativeTime(sess.LastActiveAt)
	msgCount := fmt.Sprintf("%d msgs", sess.MessageCount)
	infoLine := relTime + " · " + msgCount
	lines = append(lines, mutedStyle.Render(infoLine))

	// Remaining lines: Last user message (word wrapped to fill panel)
	if sess.LastUserMessage != "" {
		msg := sess.LastUserMessage
		msg = strings.ReplaceAll(msg, "\n", " ")
		msg = strings.TrimSpace(msg)

		// Word wrap to fill remaining height
		maxMsgLines := innerHeight - 1
		wrapped := wrapTextRunes(msg, contentWidth, maxMsgLines)
		for _, line := range wrapped {
			lines = append(lines, valueStyle.Render(line))
		}
	} else {
		lines = append(lines, mutedStyle.Render("No messages"))
	}

	// Pad each line to exact width
	for i, line := range lines {
		lineWidth := lipgloss.Width(line)
		if lineWidth < contentWidth {
			lines[i] = line + strings.Repeat(" ", contentWidth-lineWidth)
		}
	}

	// Pad to exact height
	for len(lines) < innerHeight {
		lines = append(lines, strings.Repeat(" ", contentWidth))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)

	return borderStyle.
		Width(renderWidth).
		Height(height).
		Padding(0, 1).
		Render(content)
}

// truncateRunes truncates a string to maxLen runes (not bytes)
func truncateRunes(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-2]) + ".."
}

// wrapTextRunes wraps text to width using runes, returns up to maxLines
func wrapTextRunes(s string, width, maxLines int) []string {
	if width <= 0 || maxLines <= 0 {
		return nil
	}

	runes := []rune(s)
	var result []string

	for len(runes) > 0 && len(result) < maxLines {
		lineLen := width
		if lineLen > len(runes) {
			lineLen = len(runes)
		}
		result = append(result, string(runes[:lineLen]))
		runes = runes[lineLen:]
	}

	// Add ellipsis if there's more text
	if len(runes) > 0 && len(result) > 0 {
		lastIdx := len(result) - 1
		lastLine := []rune(result[lastIdx])
		// If last line has room for "...", append to same line
		if len(lastLine) <= width-3 {
			result[lastIdx] = string(lastLine) + "..."
		} else if len(result) < maxLines {
			// Add "..." on a new line if we have room
			result = append(result, "...")
		} else {
			// Replace last 3 chars with "..."
			if len(lastLine) >= 3 {
				lastLine[len(lastLine)-3] = '.'
				lastLine[len(lastLine)-2] = '.'
				lastLine[len(lastLine)-1] = '.'
				result[lastIdx] = string(lastLine)
			}
		}
	}

	return result
}

// formatRelativeTime formats a time as relative (e.g., "2 hours ago")
func formatRelativeTime(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		secs := int(d.Seconds())
		if secs <= 1 {
			return "1 second ago"
		}
		return fmt.Sprintf("%d seconds ago", secs)
	case d < time.Hour:
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case d < 24*time.Hour:
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hours)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}

// renderClaudeNotInstalled shows message when Claude is not available
func (m *Model) renderClaudeNotInstalled(width, height int) string {
	style := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Align(lipgloss.Center, lipgloss.Center)

	icon := lipgloss.NewStyle().
		Foreground(ColorWarning).
		Bold(true).
		Render("⚠")

	title := lipgloss.NewStyle().
		Foreground(ColorText).
		Bold(true).
		Render("Claude Code Not Installed")

	msg := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Render("Install Claude Code CLI to use this feature:\nnpm install -g @anthropic-ai/claude-code")

	content := lipgloss.JoinVertical(lipgloss.Center,
		icon,
		"",
		title,
		"",
		msg,
	)

	return style.Render(content)
}
// renderClaudeChatPanel renders the main chat area (terminal or placeholder)
func (m *Model) renderClaudeChatPanel(width, height int) string {
	// Show terminal panel if there's an active session with a running terminal
	if m.claudeActiveSession != "" && m.terminalManager != nil {
		if t := m.terminalManager.Get(m.claudeActiveSession); t != nil && t.IsRunning() {
			return m.renderTerminalPanel(t, width, height)
		}
	}

	// No terminal running - show placeholder
	style := UnfocusedBorderStyle
	if m.focusArea == FocusMain {
		style = FocusedBorderStyle
	}

	var message string
	if m.claudeActiveSession == "" {
		message = "Select a session or press 'n' to create one"
	} else {
		message = "Press Enter to start Claude"
	}

	content := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Align(lipgloss.Center).
		Width(width - 4).
		Render(message)

	return style.
		Width(width - 2).
		Height(height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(content)
}

// renderTerminalPanel renders the embedded terminal
func (m *Model) renderTerminalPanel(t TerminalInterface, width, height int) string {
	// Inner dimensions (inside border)
	innerWidth := width - 2  // border left/right
	innerHeight := height - 2 // border top/bottom

	// Terminal dimensions
	termWidth := innerWidth - 2 // small padding
	termHeight := innerHeight

	if termWidth < 20 {
		termWidth = 20
	}
	if termHeight < 5 {
		termHeight = 5
	}

	// Update terminal size
	t.SetSize(termWidth, termHeight)

	// Get terminal content and split into lines
	content := t.View()
	contentLines := strings.Split(content, "\n")

	// Truncate to exact height
	if len(contentLines) > termHeight {
		contentLines = contentLines[:termHeight]
	}

	// Truncate each line to exact width and pad
	for i, line := range contentLines {
		contentLines[i] = truncateANSI(line, termWidth)
	}

	// Pad to exact height
	for len(contentLines) < termHeight {
		contentLines = append(contentLines, "")
	}

	// Calculate horizontal padding to center content (with slight right offset)
	contentWidth := termWidth
	hPadding := (innerWidth-contentWidth)/2 + 1
	if hPadding < 1 {
		hPadding = 1
	}
	leftPad := strings.Repeat(" ", hPadding)

	// Add horizontal padding to each line
	for i, line := range contentLines {
		contentLines[i] = leftPad + line
	}

	// Build inner content
	innerContent := strings.Join(contentLines, "\n")

	// Apply border style with vertical centering
	var style lipgloss.Style
	if m.terminalMode {
		style = FocusedBorderStyle.Copy()
	} else {
		style = UnfocusedBorderStyle.Copy()
	}

	return style.
		Width(width).
		Height(height).
		AlignVertical(lipgloss.Center).
		Render(innerContent)
}

// truncateANSI truncates a string with ANSI codes to a visible width
func truncateANSI(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}

	var result strings.Builder
	visibleLen := 0
	i := 0
	runes := []rune(s)

	for i < len(runes) {
		r := runes[i]

		// Check for ANSI escape sequence
		if r == '\x1b' && i+1 < len(runes) && runes[i+1] == '[' {
			// Start of CSI sequence - copy until we hit a letter
			result.WriteRune(r)
			i++
			for i < len(runes) {
				r = runes[i]
				result.WriteRune(r)
				i++
				if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
					break
				}
			}
			continue
		}

		// Check for OSC sequence (ESC ])
		if r == '\x1b' && i+1 < len(runes) && runes[i+1] == ']' {
			// Skip OSC sequence until BEL or ST
			result.WriteRune(r)
			i++
			for i < len(runes) {
				r = runes[i]
				result.WriteRune(r)
				i++
				if r == '\x07' || r == '\\' {
					break
				}
			}
			continue
		}

		// Single ESC character followed by something else
		if r == '\x1b' {
			result.WriteRune(r)
			i++
			continue
		}

		// Regular visible character
		if visibleLen >= maxWidth {
			break
		}
		result.WriteRune(r)
		visibleLen++
		i++
	}

	return result.String()
}

// Legacy chat functions removed - now using tmux terminal
