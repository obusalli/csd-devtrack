package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderCodex renders the Codex view
func (m *Model) renderCodex(width, height int) string {
	vm := m.state.Codex
	if vm == nil {
		return m.renderLoading()
	}

	// Check if Codex is installed
	if !vm.IsInstalled {
		return m.renderCodexNotInstalled(width, height)
	}

	// Layout: 70% terminal/chat, 30% sessions panel
	sessionsWidth := width * 30 / 100
	if sessionsWidth < 25 {
		sessionsWidth = 25
	}
	mainWidth := width - sessionsWidth - 1

	// Render main panel (terminal or empty state)
	mainPanel := m.renderCodexMainPanel(mainWidth, height)

	// Render sessions panel
	sessionsPanel := m.renderCodexSessionsPanel(sessionsWidth, height)

	// Combine horizontally
	return lipgloss.JoinHorizontal(lipgloss.Top, mainPanel, sessionsPanel)
}

// renderCodexNotInstalled renders a message when Codex is not installed
func (m *Model) renderCodexNotInstalled(width, height int) string {
	msg := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Align(lipgloss.Center).
		Width(width).
		Render("Codex CLI not found\n\nInstall with: npm install -g @openai/codex")

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, msg)
}

// renderCodexMainPanel renders the main panel (terminal or placeholder)
func (m *Model) renderCodexMainPanel(width, height int) string {
	// Check if we have an active session with a terminal
	if m.codexActiveSession != "" {
		if t := m.terminalManager.Get(m.codexActiveSession); t != nil && t.IsRunning() {
			return m.renderTerminalPanel(t, width, height)
		}
	}

	// No active terminal - show placeholder
	placeholder := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Align(lipgloss.Center).
		Render("Select or create a session to start Codex")

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, placeholder)
}

// renderCodexSessionsPanel renders the sessions panel
func (m *Model) renderCodexSessionsPanel(width, height int) string {
	vm := m.state.Codex
	if vm == nil {
		return ""
	}

	// Header
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		Width(width).
		Align(lipgloss.Center)
	header := headerStyle.Render("Sessions")

	// Session list using TreeMenu
	var listContent string
	if m.codexTreeMenu != nil {
		listContent = m.codexTreeMenu.Render()
	} else {
		listContent = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Render("No sessions")
	}

	// Info panel at bottom
	infoPanel := m.renderCodexInfoPanel(width)

	// Combine
	listHeight := height - lipgloss.Height(header) - lipgloss.Height(infoPanel) - 2
	listStyle := lipgloss.NewStyle().
		Height(listHeight).
		Width(width)

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		listStyle.Render(listContent),
		infoPanel,
	)
}

// renderCodexInfoPanel renders session info at the bottom
func (m *Model) renderCodexInfoPanel(width int) string {
	vm := m.state.Codex
	if vm == nil || vm.ActiveSession == nil {
		return ""
	}

	sess := vm.ActiveSession
	var lines []string

	// Session info
	lines = append(lines, fmt.Sprintf("Session: %s", sess.Name))
	if sess.ProjectName != "" {
		lines = append(lines, fmt.Sprintf("Project: %s", sess.ProjectName))
	}
	lines = append(lines, fmt.Sprintf("Messages: %d", sess.MessageCount))

	style := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Width(width).
		Padding(0, 1)

	return style.Render(strings.Join(lines, "\n"))
}
