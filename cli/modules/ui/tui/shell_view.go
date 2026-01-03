package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// renderShell renders the Shell view
func (m *Model) renderShell(width, height int) string {
	vm := m.state.Shell
	if vm == nil {
		return m.renderLoading()
	}

	// Check if Shell is installed
	if !vm.IsInstalled {
		return m.renderShellNotInstalled(width, height)
	}

	// Layout: 70% terminal/shell, 30% sessions panel
	sessionsWidth := width * 30 / 100
	if sessionsWidth < 25 {
		sessionsWidth = 25
	}
	mainWidth := width - sessionsWidth - 1

	// Render main panel (terminal or empty state)
	mainPanel := m.renderShellMainPanel(mainWidth, height)

	// Render sessions panel
	sessionsPanel := m.renderShellSessionsPanel(sessionsWidth, height)

	// Combine horizontally
	return lipgloss.JoinHorizontal(lipgloss.Top, mainPanel, sessionsPanel)
}

// renderShellNotInstalled renders a message when Shell is not installed
func (m *Model) renderShellNotInstalled(width, height int) string {
	msg := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Align(lipgloss.Center).
		Width(width).
		Render("Shell (bash/sh) not found\n\nThis is unexpected - please check your system")

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, msg)
}

// renderShellMainPanel renders the main panel (terminal or placeholder)
func (m *Model) renderShellMainPanel(width, height int) string {
	// Check if we have an active session with a terminal
	if m.shellActiveSession != "" {
		if t := m.terminalManager.Get(m.shellActiveSession); t != nil && t.IsRunning() {
			return m.renderTerminalPanel(t, width, height)
		}
	}

	// No active terminal - show placeholder
	placeholder := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Align(lipgloss.Center).
		Render("Select or create a session to start Shell\n\nn = new | h = home | s = sudo root | e = change shell")

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, placeholder)
}

// renderShellSessionsPanel renders the sessions panel
func (m *Model) renderShellSessionsPanel(width, height int) string {
	vm := m.state.Shell
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
	if m.shellTreeMenu != nil {
		listContent = m.shellTreeMenu.Render()
	} else {
		listContent = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Render("No sessions")
	}

	// Info panel at bottom
	infoPanel := m.renderShellInfoPanel(width)

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

// renderShellInfoPanel renders session info at the bottom
func (m *Model) renderShellInfoPanel(width int) string {
	vm := m.state.Shell
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
	lines = append(lines, fmt.Sprintf("WorkDir: %s", sess.WorkDir))

	// Show shell (default or custom)
	shellName := sess.Shell
	if shellName == "" && vm.ShellPath != "" {
		// Extract shell name from path
		parts := strings.Split(vm.ShellPath, "/")
		shellName = parts[len(parts)-1]
	}
	if shellName != "" {
		lines = append(lines, fmt.Sprintf("Shell: %s", shellName))
	}

	// Show available shells if more than one
	if len(vm.AvailableShells) > 1 {
		var shellNames []string
		for _, s := range vm.AvailableShells {
			shellNames = append(shellNames, s.Name)
		}
		lines = append(lines, fmt.Sprintf("Available: %s", strings.Join(shellNames, ", ")))
	}

	style := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Width(width).
		Padding(0, 1)

	return style.Render(strings.Join(lines, "\n"))
}
