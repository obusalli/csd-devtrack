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

	// Split: 70% chat, 30% sessions
	sessionsWidth := width * 3 / 10
	if sessionsWidth < 25 {
		sessionsWidth = 25
	}
	chatWidth := width - sessionsWidth - GapHorizontal - 2

	chatPanel := m.renderClaudeChatPanel(chatWidth, contentHeight)
	sessionsPanel := m.renderSessionsSidePanel(sessionsWidth, contentHeight)

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
	// Layout: chat messages (main area) + input at bottom
	inputHeight := 4
	messagesHeight := height - inputHeight - 2

	messagesPanel := m.renderChatMessages(width, messagesHeight)
	inputPanel := m.renderChatInput(width, inputHeight)

	return lipgloss.JoinVertical(lipgloss.Left, messagesPanel, inputPanel)
}

// renderSessionsSidePanel renders the compact sessions panel on the right
func (m *Model) renderSessionsSidePanel(width, height int) string {
	headerStyle := lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true)

	var items []string

	// Header with new session shortcut
	items = append(items, headerStyle.Render("Sessions")+" "+lipgloss.NewStyle().Foreground(ColorMuted).Render("[n:new]"))
	items = append(items, lipgloss.NewStyle().
		Foreground(ColorBorder).
		Render(strings.Repeat("─", width-4)))

	if m.state.Claude == nil || len(m.state.Claude.Sessions) == 0 {
		emptyStyle := lipgloss.NewStyle().Foreground(ColorMuted)
		items = append(items, emptyStyle.Render("No sessions"))
		items = append(items, emptyStyle.Render("'n' to create"))
	} else {
		// Count available height for sessions (subtract header, hints)
		availableLines := height - 8
		if availableLines < 3 {
			availableLines = 3
		}

		displayCount := 0
		for i, sess := range m.state.Claude.Sessions {
			if displayCount >= availableLines {
				remaining := len(m.state.Claude.Sessions) - displayCount
				items = append(items, lipgloss.NewStyle().
					Foreground(ColorMuted).
					Render(fmt.Sprintf("  +%d more...", remaining)))
				break
			}

			// Skip if filtering by project and doesn't match
			if m.claudeFilterProject != "" && sess.ProjectID != m.claudeFilterProject {
				continue
			}

			// Active session indicator (very visible)
			isActive := sess.ID == m.claudeActiveSession
			isSelected := i == m.mainIndex && m.focusArea == FocusDetail

			// Build the line
			var prefix string
			if isActive {
				prefix = "▶"
			} else {
				prefix = " "
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

			// Persistent indicator
			persistIcon := ""
			if sess.IsPersistent {
				persistIcon = "⚡"
			}

			// Truncate name to fit
			name := truncate(sess.Name, width-12)

			line := fmt.Sprintf("%s%s%s %s",
				prefix,
				persistIcon,
				lipgloss.NewStyle().Foreground(stateColor).Render(stateIcon),
				name,
			)

			var lineStyle lipgloss.Style
			if isActive {
				// Active session: highlighted background
				lineStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(ColorPrimary).
					Background(ColorBgAlt).
					Width(width - 4)
			} else if isSelected {
				// Selected but not active
				lineStyle = lipgloss.NewStyle().
					Foreground(ColorText).
					Background(ColorBgAlt).
					Width(width - 4)
			} else {
				lineStyle = lipgloss.NewStyle().
					Foreground(ColorText).
					Width(width - 4)
			}

			items = append(items, lineStyle.Render(line))
			displayCount++
		}
	}

	// Spacer
	items = append(items, "")

	// Hints
	hintStyle := lipgloss.NewStyle().Foreground(ColorMuted)
	items = append(items, hintStyle.Render("↑↓ select"))
	items = append(items, hintStyle.Render("Enter: switch"))
	items = append(items, hintStyle.Render("x: delete"))

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

	if m.state.Claude == nil || len(m.state.Claude.Messages) == 0 {
		if m.claudeActiveSession == "" {
			// No session selected
			emptyStyle := lipgloss.NewStyle().
				Foreground(ColorMuted).
				Align(lipgloss.Center).
				Width(width - 4)
			lines = append(lines, "")
			lines = append(lines, emptyStyle.Render("No session selected"))
			lines = append(lines, emptyStyle.Render("Go to Sessions tab (1) to select or create one"))
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

	// Add scroll indicator if needed
	if totalLines > visibleLines {
		scrollInfo := ""
		if startLine > 0 {
			scrollInfo = "↑ more "
		}
		if endLine < totalLines {
			if scrollInfo != "" {
				scrollInfo += "| "
			}
			scrollInfo += "↓ more"
		}
		if scrollInfo != "" {
			visibleContent += "\n" + lipgloss.NewStyle().
				Foreground(ColorMuted).
				Align(lipgloss.Right).
				Width(width-4).
				Render(scrollInfo)
		}
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
		// Split content into wrapped lines
		wrapped := wrapText(msg.Content, contentWidth)
		for _, line := range wrapped {
			lines = append(lines, indent+contentStyle.Render(line))
		}
	}

	// Add empty line after message for spacing
	lines = append(lines, "")

	return lines
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
