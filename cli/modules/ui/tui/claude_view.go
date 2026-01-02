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
func (m *Model) renderClaude(width, height int) string {
	// Check if Claude is installed
	if m.state.Claude == nil || !m.state.Claude.IsInstalled {
		return m.renderClaudeNotInstalled(width, height)
	}

	// Initialize mode if needed
	if m.claudeMode == "" {
		m.claudeMode = ClaudeModeSession
	}

	// Layout: tabs at top, then content
	tabs := m.renderClaudeTabs(width)
	tabHeight := 3

	contentHeight := height - tabHeight - 2
	contentWidth := width - 2

	var content string
	switch m.claudeMode {
	case ClaudeModeSession:
		content = m.renderClaudeSessions(contentWidth, contentHeight)
	case ClaudeModeChat:
		content = m.renderClaudeChat(contentWidth, contentHeight)
	case ClaudeModeSettings:
		content = m.renderClaudeSettings(contentWidth, contentHeight)
	default:
		content = m.renderClaudeSessions(contentWidth, contentHeight)
	}

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

// renderClaudeTabs renders the tab bar for Claude view
func (m *Model) renderClaudeTabs(width int) string {
	tabs := []struct {
		name string
		mode string
		key  string
	}{
		{"Sessions", ClaudeModeSession, "1"},
		{"Chat", ClaudeModeChat, "2"},
		{"Settings", ClaudeModeSettings, "3"},
	}

	tabWidth := (width - 4) / len(tabs)

	var renderedTabs []string
	for _, tab := range tabs {
		var style lipgloss.Style
		displayName := fmt.Sprintf("[%s] %s", tab.key, tab.name)

		if m.claudeMode == tab.mode {
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

// renderClaudeSessions renders the sessions panel
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
				name := truncate(sess.Name, width-15)
				msgCount := fmt.Sprintf("(%d)", sess.MessageCount)
				line = fmt.Sprintf("%s%s %s %s",
					prefix,
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

// renderChatMessages renders the chat message history
func (m *Model) renderChatMessages(width, height int) string {
	var lines []string

	if m.state.Claude == nil || len(m.state.Claude.Messages) == 0 {
		if m.claudeActiveSession == "" {
			// No session selected
			emptyStyle := lipgloss.NewStyle().
				Foreground(ColorMuted).
				Align(lipgloss.Center).
				Width(width - 4)
			lines = append(lines, emptyStyle.Render("No session selected"))
			lines = append(lines, emptyStyle.Render("Go to Sessions tab to select or create one"))
		} else {
			// Session selected but no messages
			emptyStyle := lipgloss.NewStyle().
				Foreground(ColorMuted).
				Align(lipgloss.Center).
				Width(width - 4)
			lines = append(lines, emptyStyle.Render("Start typing to begin the conversation"))
		}
	} else {
		// Render messages
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
			Render("📋 Plan:"))

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
				Render("Plan awaiting approval: [y] approve  [n] reject"))
		}
	}

	// Apply scrolling
	visibleLines := height - 2
	startLine := 0
	if len(lines) > visibleLines {
		startLine = len(lines) - visibleLines - m.claudeChatScroll
		if startLine < 0 {
			startLine = 0
		}
	}
	endLine := startLine + visibleLines
	if endLine > len(lines) {
		endLine = len(lines)
	}

	visibleContent := strings.Join(lines[startLine:endLine], "\n")

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
func (m *Model) renderChatMessage(msg core.ClaudeMessageVM, width int) []string {
	var lines []string

	// Message header
	roleStyle := lipgloss.NewStyle().Bold(true)
	if msg.Role == "user" {
		roleStyle = roleStyle.Foreground(ColorSecondary)
	} else {
		roleStyle = roleStyle.Foreground(ColorPrimary)
	}

	timeStyle := lipgloss.NewStyle().Foreground(ColorMuted)

	roleName := "You"
	if msg.Role == "assistant" {
		roleName = "Claude"
		if msg.IsPartial {
			roleName += " (typing...)"
		}
	}

	header := fmt.Sprintf("%s  %s",
		roleStyle.Render(roleName),
		timeStyle.Render(msg.TimeStr),
	)
	lines = append(lines, header)

	// Message content with word wrapping
	contentStyle := lipgloss.NewStyle().
		Foreground(ColorText).
		Width(width - 2)

	// Split content into wrapped lines
	wrapped := wrapText(msg.Content, width-2)
	for _, line := range wrapped {
		lines = append(lines, contentStyle.Render(line))
	}

	return lines
}

// renderChatInput renders the chat input area
func (m *Model) renderChatInput(width, height int) string {
	// Show session info
	sessionInfo := ""
	if m.state.Claude != nil && m.state.Claude.ActiveSession != nil {
		sessionInfo = fmt.Sprintf("Session: %s", m.state.Claude.ActiveSession.Name)
	}

	// Input prompt and text
	var inputLine string
	if m.state.Claude != nil && m.state.Claude.IsProcessing {
		// Processing indicator
		inputLine = m.spinner.View() + " Claude is thinking..."
	} else if m.claudeInputActive {
		// Active input with cursor
		cursor := "█"
		inputLine = fmt.Sprintf("» %s%s", m.claudeInputText, cursor)
	} else {
		// Inactive input
		if m.claudeInputText != "" {
			inputLine = fmt.Sprintf("> %s", m.claudeInputText)
		} else {
			inputLine = lipgloss.NewStyle().Foreground(ColorMuted).Render("> Press 'i' to type a message...")
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
		hintLine = lipgloss.NewStyle().Foreground(ColorMuted).Render("Enter: send | Esc: cancel")
	}

	content := lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Foreground(ColorMuted).Render(sessionInfo),
		lipgloss.NewStyle().Foreground(ColorText).Render(inputLine),
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
