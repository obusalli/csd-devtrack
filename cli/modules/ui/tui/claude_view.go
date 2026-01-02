package tui

import (
	"fmt"
	"strings"

	"csd-devtrack/cli/modules/ui/core"

	"github.com/charmbracelet/lipgloss"
)

// Claude view modes
const (
	ClaudeModeSession  = "sessions"
	ClaudeModeChat     = "chat"
	ClaudeModeSettings = "settings"
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

	// Initialize mode to chat by default (no more sessions tab)
	if m.claudeMode == "" || m.claudeMode == ClaudeModeSession {
		m.claudeMode = ClaudeModeChat
	}

	// Settings mode gets full width
	if m.claudeMode == ClaudeModeSettings {
		tabs := m.renderClaudeTabs(width)
		tabHeight := 3
		contentHeight := height - tabHeight - 2
		return lipgloss.JoinVertical(lipgloss.Left, tabs, m.renderClaudeSettings(width-2, contentHeight))
	}

	// Chat mode: tabs + split layout (chat left, sessions right)
	tabs := m.renderClaudeTabs(width)
	tabHeight := 3
	contentHeight := height - tabHeight - 2

	// Calculate sessions panel width using TreeMenu
	sessionsWidth := m.sessionsTreeMenu.CalcWidth()
	chatWidth := width - sessionsWidth - GapHorizontal

	// Configure and render TreeMenu
	m.sessionsTreeMenu.SetSize(sessionsWidth, contentHeight)
	m.sessionsTreeMenu.SetFocused(m.focusArea == FocusDetail)

	chatPanel := m.renderClaudeChatPanel(chatWidth, contentHeight)
	sessionsPanel := m.sessionsTreeMenu.Render()

	content := lipgloss.JoinHorizontal(lipgloss.Top,
		chatPanel,
		strings.Repeat(" ", GapHorizontal),
		sessionsPanel,
	)

	return lipgloss.JoinVertical(lipgloss.Left, tabs, content)
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

// renderClaudeTabs renders the tab bar for Claude view (simplified: Chat | Settings)
func (m *Model) renderClaudeTabs(width int) string {
	tabs := []struct {
		name string
		mode string
		key  string
	}{
		{"Chat", ClaudeModeChat, "1"},
		{"Settings", ClaudeModeSettings, "2"},
	}

	tabWidth := (width - 4) / len(tabs)

	var renderedTabs []string
	for _, tab := range tabs {
		var style lipgloss.Style
		displayName := fmt.Sprintf("[%s] %s", tab.key, tab.name)

		if m.claudeMode == tab.mode || (m.claudeMode == ClaudeModeSession && tab.mode == ClaudeModeChat) {
			style = lipgloss.NewStyle().
				Width(tabWidth).
				Align(lipgloss.Center).
				Bold(true).
				Foreground(ColorPrimary).
				Background(ColorBgAlt).
				Padding(0, 1)
		} else {
			style = lipgloss.NewStyle().
				Width(tabWidth).
				Align(lipgloss.Center).
				Foreground(ColorMuted).
				Padding(0, 1)
		}
		renderedTabs = append(renderedTabs, style.Render(displayName))
	}

	tabBar := lipgloss.JoinHorizontal(lipgloss.Center, renderedTabs...)

	separator := lipgloss.NewStyle().
		Foreground(ColorBorder).
		Width(width - 2).
		Render(strings.Repeat("─", width-4))

	return lipgloss.JoinVertical(lipgloss.Left,
		"",
		tabBar,
		separator,
	)
}

// renderClaudeChatPanel renders the main chat area (messages + input)
func (m *Model) renderClaudeChatPanel(width, height int) string {
	// If in terminal mode and terminal exists, render terminal
	if m.terminalMode && m.claudeActiveSession != "" {
		if t := m.terminalManager.Get(m.claudeActiveSession); t != nil {
			return m.renderTerminalPanel(t, width, height)
		}
	}

	// Legacy: chat messages (main area) + input at bottom
	inputHeight := 4
	messagesHeight := height - inputHeight - 2

	messagesPanel := m.renderChatMessages(width, messagesHeight)
	inputPanel := m.renderChatInput(width, inputHeight)

	return lipgloss.JoinVertical(lipgloss.Left, messagesPanel, inputPanel)
}

// renderTerminalPanel renders the embedded terminal
func (m *Model) renderTerminalPanel(t TerminalInterface, width, height int) string {
	// Inner dimensions (inside border)
	innerWidth := width - 2  // border left/right
	innerHeight := height - 2 // border top/bottom

	// Terminal dimensions (minus header line)
	termWidth := innerWidth - 2   // small padding
	termHeight := innerHeight - 1 // header line

	if termWidth < 20 {
		termWidth = 20
	}
	if termHeight < 5 {
		termHeight = 5
	}

	// Update terminal size
	t.SetSize(termWidth, termHeight)

	// Header with hints
	hintStyle := lipgloss.NewStyle().Foreground(ColorMuted)
	header := hintStyle.Render(fmt.Sprintf("[ESC ESC: exit | PgUp/PgDn: scroll | %dx%d]", t.Width(), t.Height()))

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

	// Build inner content with exact dimensions
	innerContent := header + "\n" + strings.Join(contentLines, "\n")

	// Apply border style
	var style lipgloss.Style
	if m.terminalMode {
		style = FocusedBorderStyle.Copy()
	} else {
		style = UnfocusedBorderStyle.Copy()
	}

	return style.
		Width(width).
		Height(height).
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

// calcSessionsPanelWidth calculates the width needed for the sessions panel
// based on the longest session/project name
func (m *Model) calcSessionsPanelWidth() int {
	const minWidth = 38
	const maxWidth = 65
	// padding: cursor(2) + active(2) + icon(2) + spacing(2) + terminal icon(3) + borders(5) + margin(3) + unicode(5) = 24
	// Unicode emojis (📂, ▶, ○, ⚡) take 2 visual columns each
	const padding = 24

	maxLen := 0

	// Build session map for name lookup
	sessionMap := make(map[string]string) // sessionID -> display name
	if m.state.Claude != nil {
		for _, sess := range m.state.Claude.Sessions {
			// Apply same transformation as in render
			displayName := sess.Name
			if idx := strings.Index(displayName, "-"); idx > 0 && strings.HasPrefix(displayName, sess.ProjectID) {
				displayName = displayName[idx+1:]
			}
			sessionMap[sess.ID] = displayName
		}
	}

	// Count sessions per project for width calculation
	projectSessionCount := make(map[string]int)
	for _, item := range m.claudeTreeItems {
		if !item.IsProject {
			projectSessionCount[item.ProjectID]++
		}
	}

	// Check project names (including session count suffix)
	if m.state.Projects != nil {
		for _, proj := range m.state.Projects.Projects {
			nameLen := len(proj.Name)
			if count := projectSessionCount[proj.ID]; count > 0 {
				nameLen += len(fmt.Sprintf(" (%d)", count))
			}
			if nameLen > maxLen {
				maxLen = nameLen
			}
		}
	}

	// Check session names from tree items
	for _, item := range m.claudeTreeItems {
		if !item.IsProject {
			name := sessionMap[item.SessionID]
			if name == "" {
				name = item.SessionID
			}
			if len(name) > maxLen {
				maxLen = len(name)
			}
		}
	}

	width := maxLen + padding
	if width < minWidth {
		width = minWidth
	}
	if width > maxWidth {
		width = maxWidth
	}
	return width
}

// renderSessionsSidePanel renders the sessions panel as a tree: Projects > Sessions
func (m *Model) renderSessionsSidePanel(width, height int) string {
	headerStyle := lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true)

	var items []string

	// Header
	items = append(items, headerStyle.Render("Sessions"))
	items = append(items, lipgloss.NewStyle().
		Foreground(ColorBorder).
		Render(strings.Repeat("─", width-4)))

	// Build session lookup map for rendering details
	sessionMap := make(map[string]core.ClaudeSessionVM)
	if m.state.Claude != nil {
		for _, sess := range m.state.Claude.Sessions {
			sessionMap[sess.ID] = sess
		}
	}

	// Build project lookup map for names
	projectNames := make(map[string]string)
	projectSessionCount := make(map[string]int)
	if m.state.Projects != nil {
		for _, proj := range m.state.Projects.Projects {
			projectNames[proj.ID] = proj.Name
		}
	}
	// Count sessions per project
	for _, item := range m.claudeTreeItems {
		if !item.IsProject {
			projectSessionCount[item.ProjectID]++
		}
	}

	if len(m.claudeTreeItems) == 0 {
		emptyStyle := lipgloss.NewStyle().Foreground(ColorMuted)
		items = append(items, emptyStyle.Render("No projects"))
	} else {
		// Render using pre-built tree items
		isFocused := m.focusArea == FocusDetail

		for idx, item := range m.claudeTreeItems {
			isNavigationCursor := idx == m.mainIndex && isFocused

			if item.IsProject {
				// Project line
				projName := projectNames[item.ProjectID]
				if projName == "" {
					projName = item.ProjectID
				}

				sessCount := projectSessionCount[item.ProjectID]
				projIcon := "📁"
				if sessCount > 0 {
					projIcon = "📂"
				}

				countSuffix := ""
				if sessCount > 0 {
					countSuffix = fmt.Sprintf(" (%d)", sessCount)
				}

				projLine := fmt.Sprintf("%s %s", projIcon, projName)
				if countSuffix != "" {
					projLine += lipgloss.NewStyle().Foreground(ColorMuted).Render(countSuffix)
				}

				var projStyle lipgloss.Style
				if isNavigationCursor {
					projStyle = lipgloss.NewStyle().
						Bold(true).
						Foreground(ColorSecondary).
						Background(ColorBgAlt).
						Width(width - 4)
				} else {
					projStyle = lipgloss.NewStyle().
						Foreground(ColorSecondary).
						Width(width - 4)
				}
				items = append(items, projStyle.Render(projLine))
			} else {
				// Session line
				sess, hasDetails := sessionMap[item.SessionID]
				isActiveSession := item.SessionID == m.claudeActiveSession

				// State indicator
				stateIcon := "○"
				stateColor := ColorMuted
				if hasDetails {
					switch sess.State {
					case "running":
						stateIcon = "●"
						stateColor = ColorSuccess
					case "waiting":
						stateIcon = "◐"
						stateColor = ColorWarning
					case "error":
						stateIcon = "✗"
						stateColor = ColorError
					}
				}

				// Terminal indicator (check from terminalManager)
				terminalIcon := ""
				if m.terminalManager != nil {
					if t := m.terminalManager.Get(item.SessionID); t != nil {
						switch t.State() {
						case TerminalRunning:
							terminalIcon = " ⚡" // Running (persistent process)
						case TerminalWaiting:
							terminalIcon = " ⏳" // Waiting for input
						}
					}
				}

				// Navigation cursor prefix (like menu)
				cursorPrefix := "  "
				if isNavigationCursor {
					cursorPrefix = "> "
				}

				// Active session indicator
				activePrefix := "  "
				if isActiveSession {
					activePrefix = "▶ "
				}

				// Session name
				sessName := item.SessionID
				if hasDetails {
					sessName = sess.Name
					if idx := strings.Index(sessName, "-"); idx > 0 && strings.HasPrefix(sessName, sess.ProjectID) {
						sessName = sessName[idx+1:]
					}
				}
				// No truncation needed - panel width adapts to content

				line := fmt.Sprintf("%s%s%s  %s%s",
					cursorPrefix,
					activePrefix,
					lipgloss.NewStyle().Foreground(stateColor).Render(stateIcon),
					sessName,
					lipgloss.NewStyle().Foreground(ColorWarning).Render(terminalIcon),
				)

				var lineStyle lipgloss.Style
				if isNavigationCursor && isActiveSession {
					// Both cursor and active
					lineStyle = lipgloss.NewStyle().
						Bold(true).
						Foreground(ColorPrimary).
						Background(ColorBgAlt).
						Width(width - 4)
				} else if isNavigationCursor {
					// Just cursor
					lineStyle = lipgloss.NewStyle().
						Foreground(ColorText).
						Background(ColorBgAlt).
						Width(width - 4)
				} else if isActiveSession {
					// Just active (no cursor)
					lineStyle = lipgloss.NewStyle().
						Bold(true).
						Foreground(ColorPrimary).
						Width(width - 4)
				} else {
					// Normal
					lineStyle = lipgloss.NewStyle().
						Foreground(ColorText).
						Width(width - 4)
				}

				items = append(items, lineStyle.Render(line))
			}
		}
	}

	// Spacer
	items = append(items, "")

	// Hints - context-aware based on selected item
	hintStyle := lipgloss.NewStyle().Foreground(ColorMuted)

	// Check if current selection is a project or session
	if m.mainIndex >= 0 && m.mainIndex < len(m.claudeTreeItems) {
		item := m.claudeTreeItems[m.mainIndex]
		if item.IsProject {
			items = append(items, hintStyle.Render("↑↓:nav n:new"))
		} else {
			items = append(items, hintStyle.Render("↑↓:nav Enter:select"))
			items = append(items, hintStyle.Render("r:rename x:del"))
		}
	} else {
		items = append(items, hintStyle.Render("↑↓:nav"))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, items...)

	var style lipgloss.Style
	if m.focusArea == FocusDetail {
		style = FocusedBorderStyle
	} else {
		style = UnfocusedBorderStyle
	}

	return style.
		Width(width).
		Height(height).
		Render(content)
}

// renderClaudeSessions renders the sessions panel (legacy, kept for compatibility)
func (m *Model) renderClaudeSessions(width, height int) string {
	// Split into two columns: session list (left) and session details (right)
	leftWidth := width * 2 / 5
	rightWidth := width - leftWidth - GapHorizontal

	leftPanel := m.renderSessionList(leftWidth, height)
	rightPanel := m.renderSessionDetails(rightWidth, height)

	return lipgloss.JoinHorizontal(lipgloss.Top,
		leftPanel,
		strings.Repeat(" ", GapHorizontal),
		rightPanel,
	)
}

// renderSessionList renders the list of sessions
func (m *Model) renderSessionList(width, height int) string {
	headerStyle := lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true).
		Padding(0, 1)

	// Header with filter info
	header := "Sessions"
	if m.claudeFilterProject != "" {
		header += fmt.Sprintf(" (%s)", m.claudeFilterProject)
	}

	// Session items
	var items []string
	items = append(items, headerStyle.Render(header))
	items = append(items, lipgloss.NewStyle().
		Foreground(ColorBorder).
		Padding(0, 1).
		Render(strings.Repeat("─", width-4)))

	if m.state.Claude == nil || len(m.state.Claude.Sessions) == 0 {
		emptyStyle := lipgloss.NewStyle().
			Foreground(ColorMuted).
			Padding(0, 1)
		items = append(items, emptyStyle.Render("No sessions yet"))
		items = append(items, emptyStyle.Render("Press 'n' to create new"))
	} else {
		displayIndex := 0
		for i, sess := range m.state.Claude.Sessions {
			// Skip if filtering by project and doesn't match
			if m.claudeFilterProject != "" && sess.ProjectID != m.claudeFilterProject {
				continue
			}

			// Selection indicator
			prefix := "  "
			if sess.ID == m.claudeActiveSession {
				prefix = "▶ "
			}

			// State indicator
			stateIcon := "○"
			stateColor := ColorMuted
			switch sess.State {
			case "running":
				stateIcon = "●"
				stateColor = ColorSuccess
			case "waiting":
				stateIcon = "◐"
				stateColor = ColorWarning
			case "error":
				stateIcon = "✗"
				stateColor = ColorError
			}

			// Persistent process indicator (fast mode)
			persistentIcon := ""
			if sess.IsPersistent {
				persistentIcon = lipgloss.NewStyle().Foreground(ColorWarning).Render("⚡")
			}

			// Format line - show rename input if active on this item
			var line string
			isSelected := i == m.mainIndex && m.claudeMode == ClaudeModeSession && m.focusArea == FocusMain

			if isSelected && m.claudeRenameActive {
				// Show rename input
				cursor := "█"
				line = fmt.Sprintf("%s%s %s%s",
					prefix,
					lipgloss.NewStyle().Foreground(stateColor).Render(stateIcon),
					m.claudeRenameText,
					cursor,
				)
			} else {
				name := truncate(sess.Name, width-18)
				msgCount := fmt.Sprintf("(%d)", sess.MessageCount)
				line = fmt.Sprintf("%s%s%s %s %s",
					prefix,
					persistentIcon,
					lipgloss.NewStyle().Foreground(stateColor).Render(stateIcon),
					name,
					lipgloss.NewStyle().Foreground(ColorMuted).Render(msgCount),
				)
			}

			var lineStyle lipgloss.Style
			if isSelected {
				lineStyle = lipgloss.NewStyle().
					Background(ColorBgAlt).
					Foreground(ColorText).
					Width(width - 4).
					Padding(0, 1)
			} else {
				lineStyle = lipgloss.NewStyle().
					Foreground(ColorText).
					Width(width - 4).
					Padding(0, 1)
			}

			items = append(items, lineStyle.Render(line))
			displayIndex++
		}
	}

	// Actions hint
	items = append(items, "")
	actionsStyle := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Padding(0, 1)
	items = append(items, actionsStyle.Render("n:new  r:rename  x:delete  Enter:open"))
	items = append(items, actionsStyle.Render("⚡ = fast mode (persistent process)"))

	content := lipgloss.JoinVertical(lipgloss.Left, items...)

	// Apply border
	var style lipgloss.Style
	if m.focusArea == FocusMain && m.claudeMode == ClaudeModeSession {
		style = FocusedBorderStyle
	} else {
		style = UnfocusedBorderStyle
	}

	return style.
		Width(width).
		Height(height).
		Render(content)
}

// renderSessionDetails renders details for the selected session
func (m *Model) renderSessionDetails(width, height int) string {
	headerStyle := lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true).
		Padding(0, 1)

	var items []string
	items = append(items, headerStyle.Render("Session Details"))
	items = append(items, lipgloss.NewStyle().
		Foreground(ColorBorder).
		Padding(0, 1).
		Render(strings.Repeat("─", width-4)))

	// Find selected session
	var selectedSession *core.ClaudeSessionVM
	if m.state.Claude != nil {
		for i := range m.state.Claude.Sessions {
			if i == m.mainIndex {
				selectedSession = &m.state.Claude.Sessions[i]
				break
			}
		}
	}

	if selectedSession == nil {
		emptyStyle := lipgloss.NewStyle().
			Foreground(ColorMuted).
			Padding(0, 1)
		items = append(items, emptyStyle.Render("Select a session to view details"))
	} else {
		labelStyle := lipgloss.NewStyle().
			Foreground(ColorMuted).
			Width(12)
		valueStyle := lipgloss.NewStyle().
			Foreground(ColorText)

		items = append(items,
			lipgloss.NewStyle().Padding(0, 1).Render(
				labelStyle.Render("Name:")+valueStyle.Render(selectedSession.Name),
			),
			lipgloss.NewStyle().Padding(0, 1).Render(
				labelStyle.Render("Project:")+valueStyle.Render(selectedSession.ProjectName),
			),
			lipgloss.NewStyle().Padding(0, 1).Render(
				labelStyle.Render("Messages:")+valueStyle.Render(fmt.Sprintf("%d", selectedSession.MessageCount)),
			),
			lipgloss.NewStyle().Padding(0, 1).Render(
				labelStyle.Render("State:")+valueStyle.Render(selectedSession.State),
			),
			lipgloss.NewStyle().Padding(0, 1).Render(
				labelStyle.Render("Last Active:")+valueStyle.Render(selectedSession.LastActive),
			),
		)

		// Show usage if available
		if m.state.Claude.Usage != nil && selectedSession.IsActive {
			items = append(items, "")
			items = append(items, headerStyle.Render("Usage"))
			items = append(items, lipgloss.NewStyle().
				Foreground(ColorBorder).
				Padding(0, 1).
				Render(strings.Repeat("─", width-4)))

			usage := m.state.Claude.Usage
			items = append(items,
				lipgloss.NewStyle().Padding(0, 1).Render(
					labelStyle.Render("Input:")+valueStyle.Render(fmt.Sprintf("%d tokens", usage.InputTokens)),
				),
				lipgloss.NewStyle().Padding(0, 1).Render(
					labelStyle.Render("Output:")+valueStyle.Render(fmt.Sprintf("%d tokens", usage.OutputTokens)),
				),
				lipgloss.NewStyle().Padding(0, 1).Render(
					labelStyle.Render("Total:")+valueStyle.Render(fmt.Sprintf("%d tokens", usage.TotalTokens)),
				),
			)
			if usage.CostUSD > 0 {
				items = append(items,
					lipgloss.NewStyle().Padding(0, 1).Render(
						labelStyle.Render("Est. Cost:")+valueStyle.Render(fmt.Sprintf("$%.4f", usage.CostUSD)),
					),
				)
			}
		}
	}

	content := lipgloss.JoinVertical(lipgloss.Left, items...)

	var style lipgloss.Style
	if m.focusArea == FocusDetail && m.claudeMode == ClaudeModeSession {
		style = FocusedBorderStyle
	} else {
		style = UnfocusedBorderStyle
	}

	return style.
		Width(width).
		Height(height).
		Render(content)
}

// renderClaudeChat renders the chat panel
func (m *Model) renderClaudeChat(width, height int) string {
	// Layout: chat messages (main area) + input at bottom
	inputHeight := 4
	messagesHeight := height - inputHeight - 2

	messagesPanel := m.renderChatMessages(width, messagesHeight)
	inputPanel := m.renderChatInput(width, inputHeight)

	return lipgloss.JoinVertical(lipgloss.Left, messagesPanel, inputPanel)
}

// renderChatMessages renders the chat message history with scroll support
func (m *Model) renderChatMessages(width, height int) string {
	var lines []string

	// Header showing message count and scroll position
	headerStyle := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Padding(0, 1)

	// Show loading state
	if m.claudeSessionLoading {
		loadingStyle := lipgloss.NewStyle().
			Foreground(ColorMuted).
			Align(lipgloss.Center).
			Width(width - 4)
		lines = append(lines, "")
		lines = append(lines, "")
		lines = append(lines, loadingStyle.Render(m.spinner.View()+" Loading session..."))
		content := lipgloss.JoinVertical(lipgloss.Left, lines...)
		return lipgloss.NewStyle().
			Width(width).
			Height(height).
			Render(content)
	}

	if m.state.Claude == nil || len(m.state.Claude.Messages) == 0 {
		if m.claudeActiveSession == "" {
			// No session selected
			emptyStyle := lipgloss.NewStyle().
				Foreground(ColorMuted).
				Align(lipgloss.Center).
				Width(width - 4)
			lines = append(lines, "")
			lines = append(lines, emptyStyle.Render("No session selected"))
			lines = append(lines, emptyStyle.Render("Select a session in the right panel"))
		} else {
			// Session selected but no messages
			emptyStyle := lipgloss.NewStyle().
				Foreground(ColorMuted).
				Align(lipgloss.Center).
				Width(width - 4)
			lines = append(lines, "")
			lines = append(lines, emptyStyle.Render("Session ready"))
			lines = append(lines, emptyStyle.Render("Press 'i' to start typing"))
		}
	} else {
		// Show message count header
		msgCount := len(m.state.Claude.Messages)
		isProcessing := m.state.Claude.IsProcessing
		statusText := fmt.Sprintf("Messages: %d", msgCount)
		if isProcessing {
			statusText += " | " + m.spinner.View() + " Processing..."
		}
		lines = append(lines, headerStyle.Render(statusText))
		lines = append(lines, headerStyle.Render(strings.Repeat("─", width-6)))

		// Render all messages
		for _, msg := range m.state.Claude.Messages {
			lines = append(lines, m.renderChatMessage(msg, width-4)...)
			lines = append(lines, "") // Spacing between messages
		}
	}

	// Show plan items if in plan mode
	if m.state.Claude != nil && m.state.Claude.PlanMode && len(m.state.Claude.PlanItems) > 0 {
		lines = append(lines, "")
		lines = append(lines, lipgloss.NewStyle().
			Foreground(ColorSecondary).
			Bold(true).
			Padding(0, 1).
			Render("Plan:"))

		for i, item := range m.state.Claude.PlanItems {
			statusIcon := "○"
			statusColor := ColorMuted
			switch item.Status {
			case "in_progress":
				statusIcon = "◐"
				statusColor = ColorPrimary
			case "completed":
				statusIcon = "✓"
				statusColor = ColorSuccess
			}

			lines = append(lines, fmt.Sprintf("  %s %d. %s",
				lipgloss.NewStyle().Foreground(statusColor).Render(statusIcon),
				i+1,
				item.Content,
			))
		}

		if m.state.Claude.PlanPending {
			lines = append(lines, "")
			lines = append(lines, lipgloss.NewStyle().
				Foreground(ColorWarning).
				Padding(0, 1).
				Render("Plan awaiting approval: [y] approve  [n] reject"))
		}
	}

	// Show interactive prompt if waiting for input
	if m.state.Claude != nil && m.state.Claude.WaitingForInput && m.state.Claude.Interactive != nil {
		interactive := m.state.Claude.Interactive
		lines = append(lines, "")

		promptStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorWarning).
			Padding(0, 1)

		var promptContent string
		switch interactive.Type {
		case "permission":
			promptContent = fmt.Sprintf("🔐 Permission Required\n\nClaude wants to use: %s\n", interactive.ToolName)
			if interactive.FilePath != "" {
				promptContent += fmt.Sprintf("File: %s\n", interactive.FilePath)
			}
			promptContent += "\n[y] Approve  [n] Deny"

		case "question":
			promptContent = fmt.Sprintf("❓ Claude is asking:\n\n%s\n", interactive.Question)
			if len(interactive.Options) > 0 {
				promptContent += "\nOptions:\n"
				for i, opt := range interactive.Options {
					promptContent += fmt.Sprintf("  [%d] %s\n", i+1, opt)
				}
			}
			promptContent += "\nType your answer or press [1-9] for options"

		case "plan":
			promptContent = "📋 Plan Mode\n\nClaude has created a plan. Review above.\n\n[y] Approve  [n] Reject"
		}

		lines = append(lines, promptStyle.Render(promptContent))
	}

	// Calculate visible area
	visibleLines := height - 2
	if visibleLines < 1 {
		visibleLines = 1
	}
	totalLines := len(lines)

	// Auto-scroll to bottom when new content arrives (unless user scrolled up)
	if m.state.Claude != nil && m.state.Claude.IsProcessing {
		// During processing, always show latest
		m.claudeChatScroll = 0
	}

	// Calculate scroll position (scroll is offset from bottom)
	startLine := 0
	if totalLines > visibleLines {
		// Default: show last N lines (bottom)
		startLine = totalLines - visibleLines - m.claudeChatScroll
		if startLine < 0 {
			startLine = 0
			// Clamp scroll to max
			m.claudeChatScroll = totalLines - visibleLines
		}
	}
	endLine := startLine + visibleLines
	if endLine > totalLines {
		endLine = totalLines
	}

	// Build visible content
	var visibleContent string
	if startLine < endLine {
		visibleContent = strings.Join(lines[startLine:endLine], "\n")
	}

	// Add status line: thinking indicator (left) + scroll info (right)
	isProcessing := m.state.Claude != nil && m.state.Claude.IsProcessing
	hasScrollUp := startLine > 0
	hasScrollDown := endLine < totalLines

	if isProcessing || hasScrollUp || hasScrollDown {
		// Build left side: thinking indicator
		leftPart := ""
		if isProcessing {
			leftPart = m.spinner.View() + " Claude is thinking..."
		}

		// Build right side: scroll indicators
		rightPart := ""
		if hasScrollUp {
			rightPart = "↑ more "
		}
		if hasScrollDown {
			if rightPart != "" {
				rightPart += "| "
			}
			rightPart += "↓ more"
		}

		// Combine on same line
		lineWidth := width - 4
		leftStyled := lipgloss.NewStyle().Foreground(ColorWarning).Render(leftPart)
		rightStyled := lipgloss.NewStyle().Foreground(ColorMuted).Render(rightPart)

		// Calculate padding between left and right
		leftLen := lipgloss.Width(leftStyled)
		rightLen := lipgloss.Width(rightStyled)
		padding := lineWidth - leftLen - rightLen
		if padding < 1 {
			padding = 1
		}

		statusLine := leftStyled + strings.Repeat(" ", padding) + rightStyled
		visibleContent += "\n" + statusLine
	}

	var style lipgloss.Style
	if m.focusArea == FocusMain && m.claudeMode == ClaudeModeChat {
		style = FocusedBorderStyle
	} else {
		style = UnfocusedBorderStyle
	}

	return style.
		Width(width).
		Height(height).
		Render(visibleContent)
}

// renderChatMessage renders a single chat message
// Format:
// [YYMMDD - HH:MM:SS] Role
// Content aligned below timestamp...
func (m *Model) renderChatMessage(msg core.ClaudeMessageVM, width int) []string {
	var lines []string

	// Timestamp style
	timeStyle := lipgloss.NewStyle().Foreground(ColorMuted)

	// Role style - different colors and symbols for user/assistant
	var roleName string
	var roleStyle lipgloss.Style
	var contentStyle lipgloss.Style

	if msg.Role == "user" {
		roleName = "› You"
		roleStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorSuccess)
		// User messages same color as input (green/success)
		contentStyle = lipgloss.NewStyle().Foreground(ColorSuccess)
	} else {
		roleName = "‹ Claude"
		if msg.IsPartial {
			roleName += " " + m.spinner.View()
		}
		roleStyle = lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary)
		// Assistant messages in normal text color
		contentStyle = lipgloss.NewStyle().Foreground(ColorText)
	}

	// Header: [TIMESTAMP] ROLE
	header := fmt.Sprintf("%s %s",
		timeStyle.Render("["+msg.TimeStr+"]"),
		roleStyle.Render(roleName),
	)
	lines = append(lines, header)

	// Content with small indent (2 spaces) - aligned below timestamp start
	const indent = "  "
	contentWidth := width - 4
	if contentWidth < 30 {
		contentWidth = 30
	}

	if msg.Content == "" && msg.IsPartial {
		// Show thinking indicator for empty partial messages
		lines = append(lines, indent+"...")
	} else {
		// Split content into lines and apply styling
		// Handle diff markers {{-...}} and {{+...}}
		contentLines := strings.Split(msg.Content, "\n")
		for _, line := range contentLines {
			styledLine := m.styleDiffLine(line, contentStyle, contentWidth)
			// Wrap long lines
			if len(line) > contentWidth {
				wrapped := wrapText(line, contentWidth)
				for _, wl := range wrapped {
					lines = append(lines, indent+m.styleDiffLine(wl, contentStyle, contentWidth))
				}
			} else {
				lines = append(lines, indent+styledLine)
			}
		}
	}

	// Add empty line after message for spacing
	lines = append(lines, "")

	return lines
}

// styleDiffLine applies colors to diff lines
// Handles markers: {{-...}} for removed (red bg), {{+...}} for added (blue bg)
// Also styles lines starting with ● for tool names
func (m *Model) styleDiffLine(line string, defaultStyle lipgloss.Style, lineWidth int) string {
	// Check for diff markers
	if strings.HasPrefix(line, "{{-") && strings.HasSuffix(line, "}}") {
		// Removed line - red background (R:122 G:18 B:0 = #7A1200), full width
		content := line[3 : len(line)-2]
		return lipgloss.NewStyle().
			Background(lipgloss.Color("#7A1200")).
			Foreground(lipgloss.Color("#ffcccc")).
			Width(lineWidth).
			Render(content)
	}
	if strings.HasPrefix(line, "{{+") && strings.HasSuffix(line, "}}") {
		// Added line - blue background (R:16 G:83 B:126 = #10537E), full width
		content := line[3 : len(line)-2]
		return lipgloss.NewStyle().
			Background(lipgloss.Color("#10537E")).
			Foreground(lipgloss.Color("#cce5ff")).
			Width(lineWidth).
			Render(content)
	}

	// Check for tool header (● ToolName(...))
	if strings.HasPrefix(line, "●") {
		return lipgloss.NewStyle().Bold(true).Foreground(ColorPrimary).Render(line)
	}

	// Check for tree connector (⎿)
	if strings.Contains(line, "⎿") {
		return lipgloss.NewStyle().Foreground(ColorMuted).Render(line)
	}

	// Default styling
	return defaultStyle.Render(line)
}

// renderChatInput renders the chat input area
func (m *Model) renderChatInput(width, height int) string {
	// Show session info with persistent indicator
	sessionInfo := ""
	if m.state.Claude != nil && m.state.Claude.ActiveSession != nil {
		sessionInfo = m.state.Claude.ActiveSession.Name
		if m.state.Claude.ActiveSession.IsPersistent {
			sessionInfo += " ⚡"
		}
	}

	// Input prompt and text
	var inputLine string
	var inputStyle lipgloss.Style

	if m.state.Claude != nil && m.state.Claude.IsProcessing {
		// Processing indicator - prominent
		inputStyle = lipgloss.NewStyle().Foreground(ColorWarning)
		inputLine = m.spinner.View() + " Claude is thinking..."
	} else if m.claudeInputActive {
		// Active input - very visible with bright color
		inputStyle = lipgloss.NewStyle().Foreground(ColorSuccess).Bold(true)
		inputLine = "› " + m.claudeTextInput.View()
	} else {
		// Inactive input
		if m.claudeTextInput.Value() != "" {
			inputStyle = lipgloss.NewStyle().Foreground(ColorText)
			inputLine = "> " + m.claudeTextInput.Value()
		} else {
			inputStyle = lipgloss.NewStyle().Foreground(ColorMuted)
			inputLine = "> Press 'i' to type..."
		}
	}

	var style lipgloss.Style
	if m.claudeInputActive {
		style = FocusedBorderStyle
	} else {
		style = UnfocusedBorderStyle
	}

	// Hint line
	hintLine := ""
	if m.claudeInputActive {
		if m.state.Claude != nil && m.state.Claude.IsProcessing {
			hintLine = lipgloss.NewStyle().Foreground(ColorMuted).Render("Enter: send | Esc: interrupt | Esc×2: exit")
		} else {
			hintLine = lipgloss.NewStyle().Foreground(ColorMuted).Render("Enter: send | Esc×2: exit")
		}
	} else {
		hintLine = lipgloss.NewStyle().Foreground(ColorMuted).Render("i: type | ↑↓: scroll | g/G: top/bottom | Esc: back")
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Foreground(ColorMuted).Render(sessionInfo),
		inputStyle.Render(inputLine),
		hintLine,
	)

	return style.
		Width(width).
		Height(height).
		Render(content)
}

// renderClaudeSettings renders the settings panel
func (m *Model) renderClaudeSettings(width, height int) string {
	headerStyle := lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true).
		Padding(0, 1)

	labelStyle := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Width(20)

	valueStyle := lipgloss.NewStyle().
		Foreground(ColorText)

	checkStyle := lipgloss.NewStyle().
		Foreground(ColorSuccess)

	uncheckStyle := lipgloss.NewStyle().
		Foreground(ColorMuted)

	var items []string
	items = append(items, headerStyle.Render("Claude Settings"))
	items = append(items, lipgloss.NewStyle().
		Foreground(ColorBorder).
		Padding(0, 1).
		Render(strings.Repeat("─", width-4)))

	// Get settings or use defaults
	settings := m.state.Claude.Settings
	if settings == nil {
		settings = &core.ClaudeSettingsVM{
			AllowedTools:    []string{"Read", "Glob", "Grep"},
			AutoApprove:     false,
			OutputFormat:    "stream-json",
			MaxTurns:        10,
			PlanModeEnabled: true,
		}
	}

	// Allowed Tools
	items = append(items,
		lipgloss.NewStyle().Padding(0, 1).Render(
			labelStyle.Render("Allowed Tools:")+valueStyle.Render(strings.Join(settings.AllowedTools, ", ")),
		),
	)

	// Auto Approve
	autoApproveCheck := uncheckStyle.Render("[ ]")
	if settings.AutoApprove {
		autoApproveCheck = checkStyle.Render("[✓]")
	}
	items = append(items,
		lipgloss.NewStyle().Padding(0, 1).Render(
			labelStyle.Render("Auto Approve:")+autoApproveCheck+valueStyle.Render(" Safe operations"),
		),
	)

	// Output Format
	items = append(items,
		lipgloss.NewStyle().Padding(0, 1).Render(
			labelStyle.Render("Output Format:")+valueStyle.Render(settings.OutputFormat),
		),
	)

	// Max Turns
	items = append(items,
		lipgloss.NewStyle().Padding(0, 1).Render(
			labelStyle.Render("Max Turns:")+valueStyle.Render(fmt.Sprintf("%d", settings.MaxTurns)),
		),
	)

	// Plan Mode
	planModeCheck := uncheckStyle.Render("[ ]")
	if settings.PlanModeEnabled {
		planModeCheck = checkStyle.Render("[✓]")
	}
	items = append(items,
		lipgloss.NewStyle().Padding(0, 1).Render(
			labelStyle.Render("Plan Mode:")+planModeCheck+valueStyle.Render(" For complex tasks"),
		),
	)

	// Separator
	items = append(items, "")
	items = append(items, lipgloss.NewStyle().
		Foreground(ColorBorder).
		Padding(0, 1).
		Render(strings.Repeat("─", width-4)))

	// Claude info
	items = append(items, headerStyle.Render("Claude Code Info"))
	if m.state.Claude != nil && m.state.Claude.ClaudePath != "" {
		items = append(items,
			lipgloss.NewStyle().Padding(0, 1).Render(
				labelStyle.Render("Path:")+valueStyle.Render(m.state.Claude.ClaudePath),
			),
		)
	}

	// Hints
	items = append(items, "")
	items = append(items, lipgloss.NewStyle().
		Foreground(ColorMuted).
		Padding(0, 1).
		Render("Use ↑↓ to navigate, Enter to toggle, number keys to edit"))

	content := lipgloss.JoinVertical(lipgloss.Left, items...)

	var style lipgloss.Style
	if m.focusArea == FocusMain && m.claudeMode == ClaudeModeSettings {
		style = FocusedBorderStyle
	} else {
		style = UnfocusedBorderStyle
	}

	return style.
		Width(width).
		Height(height).
		Render(content)
}

// wrapText wraps text to fit within the given width
func wrapText(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}

	var lines []string
	paragraphs := strings.Split(text, "\n")

	for _, para := range paragraphs {
		if len(para) <= width {
			lines = append(lines, para)
			continue
		}

		// Word wrap
		words := strings.Fields(para)
		var currentLine string
		for _, word := range words {
			if currentLine == "" {
				currentLine = word
			} else if len(currentLine)+1+len(word) <= width {
				currentLine += " " + word
			} else {
				lines = append(lines, currentLine)
				currentLine = word
			}
		}
		if currentLine != "" {
			lines = append(lines, currentLine)
		}
	}

	return lines
}
