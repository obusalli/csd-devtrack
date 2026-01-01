package tui

import (
	"fmt"
	"strings"
	"time"

	"csd-devtrack/cli/modules"
	"csd-devtrack/cli/modules/platform/config"
	"csd-devtrack/cli/modules/ui/core"

	"github.com/charmbracelet/lipgloss"
)

// Styles for focus states
var (
	FocusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorPrimary)

	UnfocusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(ColorBorder)

	FocusIndicator   = lipgloss.NewStyle().Foreground(ColorPrimary).Render("▶")
	UnfocusIndicator = " "
)

// renderHeader renders the top header bar
func (m *Model) renderHeader() string {
	title := TitleStyle.Render(modules.AppName)
	version := SubtitleStyle.Render("v" + modules.AppVersion)

	// Status indicators
	var status string
	if m.state.IsConnected {
		status = StatusRunning.Render("● Connected")
	} else {
		status = StatusStopped.Render("○ Disconnected")
	}

	// Running processes count
	running := len(core.SelectRunningProcesses(m.state))
	runningStr := ""
	if running > 0 {
		runningStr = StatusRunning.Render(fmt.Sprintf(" %d running", running))
	}

	// Current view indicator
	viewName := strings.ToUpper(string(m.currentView))

	left := fmt.Sprintf(" %s %s │ %s", title, version, viewName)
	right := fmt.Sprintf("%s%s ", status, runningStr)

	padding := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if padding < 0 {
		padding = 0
	}

	header := lipgloss.JoinHorizontal(
		lipgloss.Center,
		left,
		strings.Repeat(" ", padding),
		right,
	)

	return lipgloss.NewStyle().
		Background(ColorBgAlt).
		Width(m.width).
		Render(header)
}

// sidebarViews defines the navigation menu items
var sidebarViews = []struct {
	key   string
	name  string // Name with [X] shortcut highlighted
	vtype core.ViewModelType
}{
	{"1", "[D]ashboard", core.VMDashboard},
	{"2", "[P]rojects", core.VMProjects},
	{"3", "[B]uild", core.VMBuild},
	{"4", "Pr[o]cesses", core.VMProcesses},
	{"5", "[L]ogs", core.VMLogs},
	{"6", "[G]it", core.VMGit},
	{"7", "[C]onfig", core.VMConfig},
}

// getSidebarWidth returns a fixed width that fits all menu items
func getSidebarWidth() int {
	// Find longest name
	maxLen := 0
	for _, v := range sidebarViews {
		// Name like "[D]ashboard" = 11 chars
		if len(v.name) > maxLen {
			maxLen = len(v.name)
		}
	}
	// Format: "> 1 [D]ashboard" = prefix(2) + key(1) + space(1) + name
	// + borders(2) + padding(4) + margin(2)
	return maxLen + 12
}

// renderSidebar renders the left navigation sidebar
func (m *Model) renderSidebar() string {
	width := getSidebarWidth()
	itemWidth := width - 4

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorSecondary).
		Padding(0, 2).
		Width(itemWidth)
	title := titleStyle.Render("≡ MENU")
	separator := lipgloss.NewStyle().
		Foreground(ColorBorder).
		Padding(0, 2).
		Width(itemWidth).
		Render(strings.Repeat("─", itemWidth-4))

	var items []string
	items = append(items, title, separator)

	for i, v := range sidebarViews {
		// Selection indicator prefix
		var prefix string
		if i == m.sidebarIndex {
			if m.focusArea == FocusSidebar {
				prefix = "> "
			} else {
				prefix = "* "
			}
		} else {
			prefix = "  "
		}

		// Highlight the [X] shortcut in the name
		displayName := highlightShortcut(v.name)

		// Format: prefix + key + space + name
		item := fmt.Sprintf("%s%s %s", prefix, v.key, displayName)

		// Apply consistent styling with same padding for all states
		if m.currentView == v.vtype {
			// Current active view
			item = NavItemActiveStyle.Width(itemWidth).Render(item)
		} else if i == m.sidebarIndex && m.focusArea == FocusSidebar {
			// Selected with focus (but not current view)
			item = lipgloss.NewStyle().
				Padding(0, 2). // Same padding as NavItemStyle
				Width(itemWidth).
				Background(ColorBgAlt).
				Foreground(ColorText).
				Render(item)
		} else {
			// Normal item
			item = NavItemStyle.Width(itemWidth).Render(item)
		}
		items = append(items, item)
	}

	// Scroll indicator at bottom if sidebar has focus
	if m.focusArea == FocusSidebar && len(sidebarViews) > 0 {
		scrollInfo := SubtitleStyle.Render(fmt.Sprintf("  %d/%d", m.sidebarIndex+1, len(sidebarViews)))
		items = append(items, "", scrollInfo)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, items...)

	// Apply focus style
	var style lipgloss.Style
	if m.focusArea == FocusSidebar {
		style = FocusedBorderStyle
	} else {
		style = UnfocusedBorderStyle
	}

	return style.
		Width(width).
		Height(m.height - 6).
		Render(content)
}

// renderMainContent renders the main content area
func (m *Model) renderMainContent() string {
	sidebarWidth := getSidebarWidth()
	width := m.width - sidebarWidth - 2
	height := m.height - 6

	var content string

	switch m.currentView {
	case core.VMDashboard:
		content = m.renderDashboard(width, height)
	case core.VMProjects:
		content = m.renderProjects(width, height)
	case core.VMBuild:
		content = m.renderBuild(width, height)
	case core.VMProcesses:
		content = m.renderProcesses(width, height)
	case core.VMLogs:
		content = m.renderLogs(width, height)
	case core.VMGit:
		content = m.renderGit(width, height)
	case core.VMConfig:
		content = m.renderConfig(width, height)
	default:
		content = m.renderDashboard(width, height)
	}

	// Overlay dialog if showing
	if m.showDialog {
		return m.renderDialogOverlay(content, width, height)
	}

	// Overlay help if showing
	if m.showHelp {
		return m.renderHelpOverlay(content, width, height)
	}

	// Overlay filter if active
	if m.filterActive {
		content = m.renderFilterOverlay(content, width, height)
	}

	return content
}

// renderFooter renders the bottom help bar
func (m *Model) renderFooter() string {
	// Context-sensitive shortcuts based on current view and focus
	var shortcuts []string

	// Navigation hints
	navHint := HelpKeyStyle.Render("↑↓") + HelpDescStyle.Render(" nav  ")
	tabHint := HelpKeyStyle.Render("Tab") + HelpDescStyle.Render(" switch  ")

	shortcuts = append(shortcuts, navHint, tabHint)

	// View-specific shortcuts
	switch m.currentView {
	case core.VMDashboard, core.VMProjects:
		shortcuts = append(shortcuts,
			HelpKeyStyle.Render("b")+HelpDescStyle.Render(" build  "),
			HelpKeyStyle.Render("r")+HelpDescStyle.Render(" run  "),
			HelpKeyStyle.Render("s")+HelpDescStyle.Render(" stop  "),
		)
	case core.VMBuild:
		shortcuts = append(shortcuts,
			HelpKeyStyle.Render("1")+HelpDescStyle.Render(" dev  "),
			HelpKeyStyle.Render("2")+HelpDescStyle.Render(" test  "),
			HelpKeyStyle.Render("3")+HelpDescStyle.Render(" prod  "),
			HelpKeyStyle.Render("b")+HelpDescStyle.Render(" build  "),
			HelpKeyStyle.Render("B")+HelpDescStyle.Render(" all  "),
		)
	case core.VMProcesses:
		shortcuts = append(shortcuts,
			HelpKeyStyle.Render("s")+HelpDescStyle.Render(" stop  "),
			HelpKeyStyle.Render("R")+HelpDescStyle.Render(" restart  "),
			HelpKeyStyle.Render("K")+HelpDescStyle.Render(" kill  "),
			HelpKeyStyle.Render("L")+HelpDescStyle.Render(" logs  "),
		)
	case core.VMLogs:
		shortcuts = append(shortcuts,
			HelpKeyStyle.Render("/")+HelpDescStyle.Render(" search  "),
			HelpKeyStyle.Render("e")+HelpDescStyle.Render(" err  "),
			HelpKeyStyle.Render("w")+HelpDescStyle.Render(" warn  "),
			HelpKeyStyle.Render("i")+HelpDescStyle.Render(" info  "),
			HelpKeyStyle.Render("a")+HelpDescStyle.Render(" all  "),
			HelpKeyStyle.Render("x")+HelpDescStyle.Render(" clear  "),
		)
	case core.VMGit:
		shortcuts = append(shortcuts,
			HelpKeyStyle.Render("d")+HelpDescStyle.Render(" diff  "),
			HelpKeyStyle.Render("c")+HelpDescStyle.Render(" commits  "),
		)
	}

	// Always show help and quit
	shortcuts = append(shortcuts,
		HelpKeyStyle.Render("?")+HelpDescStyle.Render(" help  "),
		HelpKeyStyle.Render("q")+HelpDescStyle.Render(" quit"),
	)

	left := " " + strings.Join(shortcuts, "")

	// Right side: notifications or errors
	var right string
	if m.lastError != "" && time.Since(m.lastErrorTime) < 5*time.Second {
		right = StatusError.Render(" " + truncate(m.lastError, 40) + " ")
	} else if len(m.notifications) > 0 {
		n := m.notifications[len(m.notifications)-1]
		style := NotifyInfoStyle
		switch n.Type {
		case core.NotifySuccess:
			style = NotifySuccessStyle
		case core.NotifyWarning:
			style = NotifyWarningStyle
		case core.NotifyError:
			style = NotifyErrorStyle
		}
		right = style.Render(" " + truncate(n.Message, 40) + " ")
	}

	padding := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if padding < 0 {
		padding = 0
	}

	footer := lipgloss.JoinHorizontal(
		lipgloss.Center,
		left,
		strings.Repeat(" ", padding),
		right,
	)

	// Second line: focus indicator
	focusLine := SubtitleStyle.Render(m.getFocusIndicatorLine())

	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Width(m.width).Background(ColorBgAlt).Render(footer),
		focusLine,
	)
}

// getFocusIndicatorLine returns a visual indicator of current focus
func (m *Model) getFocusIndicatorLine() string {
	areas := []string{"Sidebar", "Main"}
	if m.hasDetailPanel() {
		areas = append(areas, "Detail")
	}

	var parts []string
	for i, area := range areas {
		if FocusArea(i) == m.focusArea {
			parts = append(parts, lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true).Render("["+area+"]"))
		} else {
			parts = append(parts, SubtitleStyle.Render(" "+area+" "))
		}
	}

	return " Focus: " + strings.Join(parts, " → ")
}

// renderDashboard renders the dashboard view with split panes
func (m *Model) renderDashboard(width, height int) string {
	vm := m.state.Dashboard
	if vm == nil {
		return m.renderLoading()
	}

	// Stats row (compact)
	stats := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.renderStatBox("Projects", fmt.Sprintf("%d", vm.ProjectCount), ColorSecondary),
		m.renderStatBox("Running", fmt.Sprintf("%d", vm.RunningCount), ColorSuccess),
		m.renderStatBox("Building", fmt.Sprintf("%d", vm.BuildingCount), ColorWarning),
		m.renderStatBox("Errors", fmt.Sprintf("%d", vm.ErrorCount), ColorError),
	)

	// Calculate panel sizes
	panelHeight := height - 10
	leftWidth := width / 2
	rightWidth := width - leftWidth - 2

	// Left: Projects list
	projectsPanel := m.renderProjectsList(vm.Projects, leftWidth, panelHeight, m.focusArea == FocusMain)

	// Right: Split between Processes (top) and Logs (bottom)
	processHeight := panelHeight / 2
	logsHeight := panelHeight - processHeight

	processesPanel := m.renderProcessesList(vm.RunningProcesses, rightWidth, processHeight, false)
	logsPanel := m.renderMiniLogs(rightWidth, logsHeight)

	rightPane := lipgloss.JoinVertical(lipgloss.Left, processesPanel, logsPanel)

	panels := lipgloss.JoinHorizontal(lipgloss.Top, projectsPanel, rightPane)

	return lipgloss.JoinVertical(lipgloss.Left,
		PanelTitleStyle.Render("Dashboard"),
		"",
		stats,
		"",
		panels,
	)
}

// renderMiniLogs renders a compact logs panel for dashboard
func (m *Model) renderMiniLogs(width, height int) string {
	title := PanelTitleStyle.Render("Recent Logs")

	var lines []string
	if m.state.Logs != nil && len(m.state.Logs.Lines) > 0 {
		// Show last N lines that fit
		maxLines := height - 4
		start := len(m.state.Logs.Lines) - maxLines
		if start < 0 {
			start = 0
		}

		for _, line := range m.state.Logs.Lines[start:] {
			// Compact format: [source] message
			var levelStyle lipgloss.Style
			switch line.Level {
			case "error":
				levelStyle = LogErrorStyle
			case "warn":
				levelStyle = LogWarnStyle
			default:
				levelStyle = LogInfoStyle
			}
			logLine := fmt.Sprintf("%s %s",
				LogSourceStyle.Render(fmt.Sprintf("[%-8s]", truncate(line.Source, 8))),
				levelStyle.Render(truncate(line.Message, width-14)))
			lines = append(lines, logLine)
		}
	}

	if len(lines) == 0 {
		lines = append(lines, SubtitleStyle.Render("  No recent logs"))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)

	return UnfocusedBorderStyle.Width(width).Height(height).Render(
		lipgloss.JoinVertical(lipgloss.Left, title, content),
	)
}

// renderStatBox renders a stat box
func (m *Model) renderStatBox(label, value string, color lipgloss.Color) string {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color).
		Padding(0, 2).
		Margin(0, 1).
		Render(
			lipgloss.JoinVertical(lipgloss.Center,
				lipgloss.NewStyle().Foreground(color).Bold(true).Render(value),
				SubtitleStyle.Render(label),
			),
		)
}

// renderProjectsList renders a list of projects
func (m *Model) renderProjectsList(projects []core.ProjectVM, width, height int, focused bool) string {
	title := PanelTitleStyle.Render("Projects")

	var rows []string
	for i, p := range projects {
		if i >= height-3 {
			rows = append(rows, SubtitleStyle.Render(fmt.Sprintf("  ... and %d more", len(projects)-i)))
			break
		}

		status := StatusIcon(m.getProjectStatus(p))
		git := ""
		if p.GitDirty {
			git = GitDirtyStyle.Render(" *")
		}

		row := fmt.Sprintf("%s %s%s", status, truncate(p.Name, width-10), git)

		if i == m.mainIndex && focused {
			row = TableRowSelectedStyle.Width(width - 4).Render(FocusIndicator + " " + row)
		} else if i == m.mainIndex {
			row = TableRowSelectedStyle.Width(width - 4).Render("› " + row)
		} else {
			row = "  " + row
		}
		rows = append(rows, row)
	}

	if len(rows) == 0 {
		rows = append(rows, SubtitleStyle.Render("  No projects"))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, rows...)

	var style lipgloss.Style
	if focused {
		style = FocusedBorderStyle
	} else {
		style = UnfocusedBorderStyle
	}

	return style.Width(width).Height(height).Render(
		lipgloss.JoinVertical(lipgloss.Left, title, content),
	)
}

// renderProcessesList renders a list of processes
func (m *Model) renderProcessesList(processes []core.ProcessVM, width, height int, focused bool) string {
	title := PanelTitleStyle.Render("Running Processes")

	var rows []string
	for i, p := range processes {
		if i >= height-3 {
			rows = append(rows, SubtitleStyle.Render(fmt.Sprintf("  ... and %d more", len(processes)-i)))
			break
		}

		row := fmt.Sprintf("%s %s/%s", StatusRunning.Render(IconRunning), truncate(p.ProjectName, 12), p.Component)
		rows = append(rows, "  "+row)
	}

	if len(rows) == 0 {
		rows = append(rows, SubtitleStyle.Render("  No running processes"))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, rows...)

	var style lipgloss.Style
	if focused {
		style = FocusedBorderStyle
	} else {
		style = UnfocusedBorderStyle
	}

	return style.Width(width).Height(height).Render(
		lipgloss.JoinVertical(lipgloss.Left, title, content),
	)
}

// renderProjects renders the projects view
func (m *Model) renderProjects(width, height int) string {
	vm := m.state.Projects
	if vm == nil {
		return m.renderLoading()
	}

	title := PanelTitleStyle.Render("Projects")

	// Table header
	header := TableHeaderStyle.Render(
		fmt.Sprintf("  %-20s %-12s %-8s %-20s", "Name", "Type", "Status", "Git"),
	)

	// Table rows
	var rows []string
	startIdx := m.mainScrollOffset
	endIdx := startIdx + m.visibleMainRows
	if endIdx > len(vm.Projects) {
		endIdx = len(vm.Projects)
	}

	for i := startIdx; i < endIdx; i++ {
		p := vm.Projects[i]
		status := m.getProjectStatus(p)
		statusStr := StatusIcon(status)

		gitInfo := fmt.Sprintf("%s %s", IconBranch, truncate(p.GitBranch, 10))
		if p.GitDirty {
			gitInfo += GitDirtyStyle.Render(" *")
		}
		if p.GitAhead > 0 {
			gitInfo += GitAheadStyle.Render(fmt.Sprintf(" ↑%d", p.GitAhead))
		}

		row := fmt.Sprintf("%-20s %-12s %s %-20s",
			truncate(p.Name, 20),
			truncate(string(p.Type), 12),
			statusStr,
			gitInfo,
		)

		if i == m.mainIndex && m.focusArea == FocusMain {
			row = TableRowSelectedStyle.Width(width - 6).Render(FocusIndicator + " " + row)
		} else if i == m.mainIndex {
			row = TableRowSelectedStyle.Width(width - 6).Render("› " + row)
		} else {
			row = "  " + row
		}
		rows = append(rows, row)
	}

	// Scroll indicator
	if len(vm.Projects) > m.visibleMainRows {
		scrollInfo := SubtitleStyle.Render(fmt.Sprintf("  [%d-%d of %d]", startIdx+1, endIdx, len(vm.Projects)))
		rows = append(rows, scrollInfo)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, rows...)

	var style lipgloss.Style
	if m.focusArea == FocusMain {
		style = FocusedBorderStyle
	} else {
		style = UnfocusedBorderStyle
	}

	mainPanel := style.Width(width - 2).Height(height - 2).Render(
		lipgloss.JoinVertical(lipgloss.Left, title, header, content),
	)

	return mainPanel
}

// renderBuild renders the build view
func (m *Model) renderBuild(width, height int) string {
	vm := m.state.Builds
	if vm == nil {
		return m.renderLoading()
	}

	title := PanelTitleStyle.Render("Build")

	// Profile selector bar
	profiles := []struct {
		key  string
		name string
		desc string
	}{
		{"dev", "DEV", "Debug symbols, verbose"},
		{"test", "TEST", "Race detection"},
		{"prod", "PROD", "Optimized, stripped"},
	}

	var profileButtons []string
	for _, p := range profiles {
		if m.currentBuildProfile == p.key {
			profileButtons = append(profileButtons, ButtonActiveStyle.Render(p.name))
		} else {
			profileButtons = append(profileButtons, ButtonStyle.Render(p.name))
		}
	}
	profileBar := lipgloss.JoinHorizontal(lipgloss.Center,
		SubtitleStyle.Render("Profile: "),
		strings.Join(profileButtons, " "),
		"  ",
		SubtitleStyle.Render(m.getProfileDescription()),
	)

	// Current build status
	var buildStatus string
	if vm.CurrentBuild != nil {
		b := vm.CurrentBuild
		progress := renderProgressBar(b.Progress, 20)
		buildStatus = fmt.Sprintf(
			"%s Building %s/%s [%s] %s\n",
			m.spinner.View(),
			b.ProjectName,
			b.Component,
			strings.ToUpper(m.currentBuildProfile),
			progress,
		)

		// Build output (last lines)
		outputLines := b.Output
		maxLines := height - 16
		if len(outputLines) > maxLines {
			outputLines = outputLines[len(outputLines)-maxLines:]
		}
		for _, line := range outputLines {
			buildStatus += LogInfoStyle.Render(truncate(line, width-10)) + "\n"
		}
	} else if vm.IsBuilding {
		buildStatus = m.spinner.View() + " Building..."
	} else {
		buildStatus = SubtitleStyle.Render("No active build. Press 'b' to build, 'B' for all.")
	}

	// Build history
	historyTitle := SubtitleStyle.Render("Recent Builds")
	var historyLines []string
	for i, b := range vm.BuildHistory {
		if i >= 5 {
			break
		}
		statusIcon := StatusSuccess.Render(IconSuccess)
		if string(b.Status) == "failed" {
			statusIcon = StatusError.Render(IconError)
		}
		historyLines = append(historyLines,
			fmt.Sprintf("  %s %s/%s %s",
				statusIcon, truncate(b.ProjectName, 10), b.Component, b.Duration))
	}
	if len(historyLines) == 0 {
		historyLines = append(historyLines, SubtitleStyle.Render("  No build history"))
	}

	var style lipgloss.Style
	if m.focusArea == FocusMain {
		style = FocusedBorderStyle
	} else {
		style = UnfocusedBorderStyle
	}

	return style.Width(width - 2).Height(height - 2).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			title,
			profileBar,
			"",
			buildStatus,
			"",
			historyTitle,
			strings.Join(historyLines, "\n"),
		),
	)
}

// getProfileDescription returns description for current build profile
func (m *Model) getProfileDescription() string {
	switch m.currentBuildProfile {
	case "dev":
		return "Debug symbols, verbose output"
	case "test":
		return "Race detection enabled"
	case "prod":
		return "Optimized, symbols stripped"
	default:
		return ""
	}
}

// renderProgressBar renders a progress bar
func renderProgressBar(percent, width int) string {
	filled := width * percent / 100
	empty := width - filled

	bar := strings.Repeat("█", filled) + strings.Repeat("░", empty)
	return fmt.Sprintf("[%s] %d%%", bar, percent)
}

// renderProcesses renders the processes view
func (m *Model) renderProcesses(width, height int) string {
	vm := m.state.Processes
	if vm == nil {
		return m.renderLoading()
	}

	title := PanelTitleStyle.Render("Processes")

	// Table header
	header := TableHeaderStyle.Render(
		fmt.Sprintf("  %-20s %-10s %-8s %-10s %-6s", "Project/Component", "State", "PID", "Uptime", "Restarts"),
	)

	// Table rows
	var rows []string
	for i, p := range vm.Processes {
		stateStr := StatusIcon(string(p.State))

		row := fmt.Sprintf("%-20s %s %-8d %-10s %-6d",
			fmt.Sprintf("%s/%s", truncate(p.ProjectName, 10), p.Component),
			stateStr,
			p.PID,
			truncate(p.Uptime, 10),
			p.Restarts,
		)

		if i == m.mainIndex && m.focusArea == FocusMain {
			row = TableRowSelectedStyle.Width(width - 6).Render(FocusIndicator + " " + row)
		} else if i == m.mainIndex {
			row = TableRowSelectedStyle.Width(width - 6).Render("› " + row)
		} else {
			row = "  " + row
		}
		rows = append(rows, row)
	}

	if len(rows) == 0 {
		rows = append(rows, SubtitleStyle.Render("  No processes"))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, rows...)

	var style lipgloss.Style
	if m.focusArea == FocusMain {
		style = FocusedBorderStyle
	} else {
		style = UnfocusedBorderStyle
	}

	return style.Width(width - 2).Height(height - 2).Render(
		lipgloss.JoinVertical(lipgloss.Left, title, header, content),
	)
}

// renderLogs renders the logs view with filtering
func (m *Model) renderLogs(width, height int) string {
	vm := m.state.Logs
	if vm == nil {
		return m.renderLoading()
	}

	title := PanelTitleStyle.Render("Logs")

	// Filter controls bar
	levelFilters := []string{"ALL", "ERR", "WRN", "INF", "DBG"}
	levelValues := []string{"", "error", "warn", "info", "debug"}
	var filterButtons []string
	for i, lbl := range levelFilters {
		if m.logLevelFilter == levelValues[i] {
			filterButtons = append(filterButtons, ButtonActiveStyle.Render(lbl))
		} else {
			filterButtons = append(filterButtons, ButtonStyle.Render(lbl))
		}
	}
	levelBar := strings.Join(filterButtons, " ")

	// Search box
	searchLabel := SubtitleStyle.Render("Search: ")
	var searchBox string
	if m.logSearchActive {
		searchBox = InputFocusedStyle.Width(20).Render(m.logSearchText + "█")
	} else if m.logSearchText != "" {
		searchBox = InputStyle.Width(20).Render(m.logSearchText)
	} else {
		searchBox = InputStyle.Width(20).Render(SubtitleStyle.Render("/ to search"))
	}

	filterBar := lipgloss.JoinHorizontal(lipgloss.Center,
		levelBar,
		"  ",
		searchLabel,
		searchBox,
	)

	// Filter log lines
	var filteredLines []core.LogLineVM
	for _, line := range vm.Lines {
		// Level filter
		if m.logLevelFilter != "" && line.Level != m.logLevelFilter {
			continue
		}
		// Text search filter
		if m.logSearchText != "" {
			searchLower := strings.ToLower(m.logSearchText)
			if !strings.Contains(strings.ToLower(line.Message), searchLower) &&
				!strings.Contains(strings.ToLower(line.Source), searchLower) {
				continue
			}
		}
		filteredLines = append(filteredLines, line)
	}

	// Display log lines
	var logLines []string
	maxLines := height - 8
	start := 0
	if len(filteredLines) > maxLines {
		start = len(filteredLines) - maxLines
	}

	for _, line := range filteredLines[start:] {
		timestamp := LogTimestampStyle.Render(line.TimeStr)
		source := LogSourceStyle.Render(fmt.Sprintf("[%-12s]", truncate(line.Source, 12)))

		var levelStyle lipgloss.Style
		var levelIcon string
		switch line.Level {
		case "error":
			levelStyle = LogErrorStyle
			levelIcon = "E"
		case "warn":
			levelStyle = LogWarnStyle
			levelIcon = "W"
		case "debug":
			levelStyle = LogDebugStyle
			levelIcon = "D"
		default:
			levelStyle = LogInfoStyle
			levelIcon = "I"
		}

		// Highlight search matches
		message := line.Message
		if m.logSearchText != "" {
			message = highlightMatch(message, m.logSearchText, width-40)
		} else {
			message = truncate(message, width-40)
		}

		logLine := fmt.Sprintf("%s %s %s %s",
			timestamp,
			levelStyle.Render(levelIcon),
			source,
			levelStyle.Render(message))
		logLines = append(logLines, logLine)
	}

	// Stats line
	statsLine := SubtitleStyle.Render(fmt.Sprintf(
		"Showing %d of %d lines │ Auto-scroll: %v",
		len(filteredLines), len(vm.Lines), vm.AutoScroll))

	if len(logLines) == 0 {
		logLines = append(logLines, SubtitleStyle.Render("No logs matching filters"))
	}

	content := strings.Join(logLines, "\n")

	var style lipgloss.Style
	if m.focusArea == FocusMain {
		style = FocusedBorderStyle
	} else {
		style = UnfocusedBorderStyle
	}

	return style.Width(width - 2).Height(height - 2).Render(
		lipgloss.JoinVertical(lipgloss.Left, title, filterBar, statsLine, "", content),
	)
}

// highlightMatch truncates and highlights matching text
func highlightMatch(text, search string, maxLen int) string {
	text = truncate(text, maxLen)
	if search == "" {
		return text
	}

	lower := strings.ToLower(text)
	searchLower := strings.ToLower(search)
	idx := strings.Index(lower, searchLower)
	if idx == -1 {
		return text
	}

	// Simple highlight by making the match bold/colored
	before := text[:idx]
	match := text[idx : idx+len(search)]
	after := text[idx+len(search):]

	highlighted := lipgloss.NewStyle().
		Background(ColorWarning).
		Foreground(ColorBg).
		Render(match)

	return before + highlighted + after
}

// renderGit renders the git view
func (m *Model) renderGit(width, height int) string {
	vm := m.state.Git
	if vm == nil {
		return m.renderLoading()
	}

	title := PanelTitleStyle.Render("Git Status")

	// Projects list (left panel)
	listWidth := width * 2 / 3
	var projectRows []string
	for i, p := range vm.Projects {
		status := GitStatusIcon(p.IsClean)
		branchInfo := truncate(p.Branch, 12)

		var syncInfo string
		if p.Ahead > 0 {
			syncInfo += GitAheadStyle.Render(fmt.Sprintf("↑%d", p.Ahead))
		}
		if p.Behind > 0 {
			syncInfo += GitBehindStyle.Render(fmt.Sprintf("↓%d", p.Behind))
		}

		row := fmt.Sprintf("%s %-18s %-12s %s",
			status,
			truncate(p.ProjectName, 18),
			branchInfo,
			syncInfo,
		)

		if i == m.mainIndex && m.focusArea == FocusMain {
			row = TableRowSelectedStyle.Width(listWidth - 6).Render(FocusIndicator + " " + row)
		} else if i == m.mainIndex {
			row = TableRowSelectedStyle.Width(listWidth - 6).Render("› " + row)
		} else {
			row = "  " + row
		}
		projectRows = append(projectRows, row)
	}

	if len(projectRows) == 0 {
		projectRows = append(projectRows, SubtitleStyle.Render("  No git repositories"))
	}

	// Detail panel (right)
	detailWidth := width - listWidth - 2
	var detailContent string
	if m.mainIndex >= 0 && m.mainIndex < len(vm.Projects) {
		p := vm.Projects[m.mainIndex]
		detailLines := []string{
			PanelTitleStyle.Render(p.ProjectName),
			fmt.Sprintf("Branch: %s", GitBranchStyle.Render(p.Branch)),
			"",
		}

		if len(p.Staged) > 0 {
			detailLines = append(detailLines, StatusSuccess.Render(fmt.Sprintf("Staged (%d):", len(p.Staged))))
			for _, f := range p.Staged[:min(3, len(p.Staged))] {
				detailLines = append(detailLines, "  "+truncate(f, detailWidth-4))
			}
		}
		if len(p.Modified) > 0 {
			detailLines = append(detailLines, StatusWarning.Render(fmt.Sprintf("Modified (%d):", len(p.Modified))))
			for _, f := range p.Modified[:min(3, len(p.Modified))] {
				detailLines = append(detailLines, "  "+truncate(f, detailWidth-4))
			}
		}
		if len(p.Untracked) > 0 {
			detailLines = append(detailLines, SubtitleStyle.Render(fmt.Sprintf("Untracked (%d):", len(p.Untracked))))
			for _, f := range p.Untracked[:min(3, len(p.Untracked))] {
				detailLines = append(detailLines, "  "+truncate(f, detailWidth-4))
			}
		}

		detailContent = strings.Join(detailLines, "\n")
	}

	// Build panels
	var listStyle, detailStyle lipgloss.Style
	if m.focusArea == FocusMain {
		listStyle = FocusedBorderStyle
	} else {
		listStyle = UnfocusedBorderStyle
	}
	if m.focusArea == FocusDetail {
		detailStyle = FocusedBorderStyle
	} else {
		detailStyle = UnfocusedBorderStyle
	}

	listPanel := listStyle.Width(listWidth).Height(height - 4).Render(
		lipgloss.JoinVertical(lipgloss.Left, title, strings.Join(projectRows, "\n")),
	)
	detailPanel := detailStyle.Width(detailWidth).Height(height - 4).Render(detailContent)

	return lipgloss.JoinHorizontal(lipgloss.Top, listPanel, detailPanel)
}

// renderConfig renders the config view with tabs
func (m *Model) renderConfig(width, height int) string {
	// Load browser entries if not loaded
	if m.configMode == "browser" && len(m.browserEntries) == 0 {
		m.loadBrowserEntries()
	}

	// Tab styles
	tabActive := lipgloss.NewStyle().
		Background(ColorPrimary).
		Foreground(lipgloss.Color("#000")).
		Padding(0, 2).
		Bold(true)
	tabInactive := lipgloss.NewStyle().
		Background(lipgloss.Color("#444")).
		Foreground(lipgloss.Color("#fff")).
		Padding(0, 2)

	// Render tabs
	var tabs []string
	modes := []struct {
		key  string
		name string
	}{
		{"projects", "[1] Projects"},
		{"browser", "[2] Browser"},
		{"settings", "[3] Settings"},
	}
	for _, mode := range modes {
		if m.configMode == mode.key {
			tabs = append(tabs, tabActive.Render(mode.name))
		} else {
			tabs = append(tabs, tabInactive.Render(mode.name))
		}
	}
	tabBar := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)

	// Render content based on mode
	var content string
	contentHeight := height - 6

	switch m.configMode {
	case "projects":
		content = m.renderConfigProjects(width-4, contentHeight)
	case "browser":
		content = m.renderConfigBrowser(width-4, contentHeight)
	case "settings":
		content = m.renderConfigSettings(width-4, contentHeight)
	default:
		content = m.renderConfigProjects(width-4, contentHeight)
	}

	var style lipgloss.Style
	if m.focusArea == FocusMain {
		style = FocusedBorderStyle
	} else {
		style = UnfocusedBorderStyle
	}

	return style.Width(width - 2).Height(height - 2).Render(
		lipgloss.JoinVertical(lipgloss.Left,
			tabBar,
			"",
			content,
		),
	)
}

// renderConfigProjects renders the projects list in config view
func (m *Model) renderConfigProjects(width, height int) string {
	cfg := config.GetGlobal()
	if cfg == nil || len(cfg.Projects) == 0 {
		return SubtitleStyle.Render("No projects configured.\nUse [2] Browser to add projects.")
	}

	title := PanelTitleStyle.Render(fmt.Sprintf("Configured Projects (%d)", len(cfg.Projects)))

	var rows []string
	for i, proj := range cfg.Projects {
		indicator := "  "
		style := lipgloss.NewStyle()
		if i == m.mainIndex && m.focusArea == FocusMain {
			indicator = "> "
			style = TableRowSelectedStyle
		}

		// Component badges
		var compBadges []string
		for compType := range proj.Components {
			compBadges = append(compBadges, string(compType))
		}
		comps := ""
		if len(compBadges) > 0 {
			comps = lipgloss.NewStyle().
				Foreground(ColorSecondary).
				Render(" [" + strings.Join(compBadges, ",") + "]")
		}

		row := fmt.Sprintf("%s%s%s", indicator, proj.Name, comps)
		rows = append(rows, style.Render(row))
	}

	// Show selected project details
	var details string
	if m.mainIndex >= 0 && m.mainIndex < len(cfg.Projects) {
		proj := cfg.Projects[m.mainIndex]
		details = "\n" + lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorMuted).
			Padding(0, 1).
			Render(
				lipgloss.JoinVertical(lipgloss.Left,
					PanelTitleStyle.Render(proj.Name),
					fmt.Sprintf("Path: %s", proj.Path),
					fmt.Sprintf("Type: %s", proj.Type),
					"",
					"[Enter] View details  [x] Remove from config",
				),
			)
	}

	m.maxMainItems = len(cfg.Projects)

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		"",
		strings.Join(rows, "\n"),
		details,
	)
}

// renderConfigBrowser renders the file browser in config view
func (m *Model) renderConfigBrowser(width, height int) string {
	// Path display
	pathStyle := lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true)
	pathDisplay := pathStyle.Render("📁 " + m.browserPath)

	// Detected project info
	var projectInfo string
	if m.detectedProject != nil {
		inConfig := m.isProjectInConfig(m.detectedProject.Path)
		var actionHint string
		if inConfig {
			actionHint = lipgloss.NewStyle().
				Foreground(ColorWarning).
				Render("[x] Remove from config")
		} else {
			actionHint = lipgloss.NewStyle().
				Foreground(ColorSuccess).
				Render("[a] Add to config")
		}

		projectInfo = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorSuccess).
			Padding(0, 1).
			Render(
				lipgloss.JoinVertical(lipgloss.Left,
					PanelTitleStyle.Render("✓ Project Detected: "+m.detectedProject.Name),
					fmt.Sprintf("Type: %s", m.detectedProject.Type),
					fmt.Sprintf("Components: %s", strings.Join(m.detectedProject.Components, ", ")),
					"",
					actionHint,
				),
			)
	}

	// Directory listing
	var rows []string
	visibleRows := height - 10
	if projectInfo != "" {
		visibleRows -= 6
	}

	startIdx := 0
	if m.mainIndex >= visibleRows {
		startIdx = m.mainIndex - visibleRows + 1
	}

	for i, entry := range m.browserEntries {
		if i < startIdx || i >= startIdx+visibleRows {
			continue
		}

		indicator := "  "
		style := lipgloss.NewStyle()
		if i == m.mainIndex && m.focusArea == FocusMain {
			indicator = "> "
			style = TableRowSelectedStyle
		}

		icon := "📁"
		if entry.Name == ".." {
			icon = "⬆️"
		}

		suffix := ""
		if entry.IsProject {
			if m.isProjectInConfig(entry.Path) {
				suffix = lipgloss.NewStyle().
					Foreground(ColorSuccess).
					Render(" ✓ configured")
			} else {
				suffix = lipgloss.NewStyle().
					Foreground(ColorWarning).
					Render(" ★ project")
			}
		}

		row := fmt.Sprintf("%s%s %s%s", indicator, icon, entry.Name, suffix)
		rows = append(rows, style.Render(row))
	}

	// Scroll indicator
	scrollInfo := ""
	if len(m.browserEntries) > visibleRows {
		scrollInfo = SubtitleStyle.Render(
			fmt.Sprintf(" [%d/%d]", m.mainIndex+1, len(m.browserEntries)))
	}

	m.maxMainItems = len(m.browserEntries)

	content := lipgloss.JoinVertical(lipgloss.Left,
		pathDisplay+scrollInfo,
		"",
		strings.Join(rows, "\n"),
	)

	if projectInfo != "" {
		content = lipgloss.JoinVertical(lipgloss.Left,
			content,
			"",
			projectInfo,
		)
	}

	return content
}

// renderConfigSettings renders the settings in config view
func (m *Model) renderConfigSettings(width, height int) string {
	vm := m.state.Config
	if vm == nil {
		return m.renderLoading()
	}

	title := PanelTitleStyle.Render("Settings")
	pathInfo := SubtitleStyle.Render(fmt.Sprintf("Config file: %s", vm.ConfigPath))

	// Settings with categories
	var sections []string

	// Build settings
	sections = append(sections, lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true).
		Render("Build"))
	if val, ok := vm.Settings["parallel_builds"]; ok {
		sections = append(sections, fmt.Sprintf("  Parallel builds: %v", val))
	}

	// Logging
	sections = append(sections, "", lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true).
		Render("Logging"))
	if val, ok := vm.Settings["log_buffer_size"]; ok {
		sections = append(sections, fmt.Sprintf("  Buffer size: %v", val))
	}
	if val, ok := vm.Settings["log_level"]; ok {
		sections = append(sections, fmt.Sprintf("  Log level: %v", val))
	}

	// Web server
	sections = append(sections, "", lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true).
		Render("Web Server"))
	if val, ok := vm.Settings["web_enabled"]; ok {
		sections = append(sections, fmt.Sprintf("  Enabled: %v", val))
	}
	if val, ok := vm.Settings["web_port"]; ok {
		sections = append(sections, fmt.Sprintf("  Port: %v", val))
	}

	// UI
	sections = append(sections, "", lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true).
		Render("UI"))
	if val, ok := vm.Settings["theme"]; ok {
		sections = append(sections, fmt.Sprintf("  Theme: %v", val))
	}
	if val, ok := vm.Settings["refresh_rate"]; ok {
		sections = append(sections, fmt.Sprintf("  Refresh rate: %vms", val))
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		pathInfo,
		"",
		strings.Join(sections, "\n"),
	)
}

// renderLoading renders a loading indicator
func (m *Model) renderLoading() string {
	return lipgloss.NewStyle().
		Padding(2).
		Render(m.spinner.View() + " Loading...")
}

// renderDialogOverlay renders a dialog overlay
func (m *Model) renderDialogOverlay(background string, width, height int) string {
	yesStyle := ButtonStyle
	noStyle := ButtonStyle
	if m.dialogConfirm {
		yesStyle = ButtonActiveStyle
	} else {
		noStyle = ButtonActiveStyle
	}

	dialog := DialogStyle.Render(
		lipgloss.JoinVertical(lipgloss.Center,
			DialogTitleStyle.Render("Confirm"),
			"",
			m.dialogMessage,
			"",
			lipgloss.JoinHorizontal(lipgloss.Center,
				yesStyle.Render(" Yes "),
				"  ",
				noStyle.Render(" No "),
			),
		),
	)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, dialog)
}

// renderHelpOverlay renders the help overlay
func (m *Model) renderHelpOverlay(background string, width, height int) string {
	helpContent := []string{
		DialogTitleStyle.Render("Keyboard Shortcuts"),
		"",
		HelpKeyStyle.Render("Navigation"),
		"  ↑/↓/←/→  Navigate within focused panel",
		"  Tab      Switch focus between panels",
		"  1-7      Jump to view",
		"  Enter    Select/Activate",
		"  PgUp/Dn  Page scroll",
		"  Home/End Go to start/end",
		"",
		HelpKeyStyle.Render("Actions"),
		"  b        Build selected project",
		"  B        Build all projects",
		"  r        Run/Start project",
		"  s        Stop project",
		"  R        Restart project",
		"  K        Kill project (force)",
		"  L        View logs",
		"",
		HelpKeyStyle.Render("Other"),
		"  /        Filter",
		"  Ctrl+R   Refresh",
		"  ?        Toggle help",
		"  q        Quit",
		"",
		SubtitleStyle.Render("Press any key to close"),
	}

	helpBox := DialogStyle.Width(50).Render(strings.Join(helpContent, "\n"))

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, helpBox)
}

// renderFilterOverlay renders the filter input overlay
func (m *Model) renderFilterOverlay(background string, width, height int) string {
	input := InputFocusedStyle.Width(30).Render(m.filterText + "█")
	filterBox := DialogStyle.Render(
		lipgloss.JoinVertical(lipgloss.Left,
			DialogTitleStyle.Render("Filter"),
			"",
			input,
			"",
			SubtitleStyle.Render("Enter to apply, Esc to cancel"),
		),
	)

	// Overlay at top
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Top,
		lipgloss.NewStyle().MarginTop(3).Render(filterBox),
	)
}

// Helper functions
func (m *Model) getProjectStatus(p core.ProjectVM) string {
	if p.RunningCount > 0 {
		return "running"
	}
	return "stopped"
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max <= 3 {
		return s[:max]
	}
	return s[:max-3] + "..."
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// renderScrollIndicator renders a vertical scroll bar
// total: total number of items, visible: visible items, offset: current scroll offset
func renderScrollIndicator(total, visible, offset, height int) string {
	if total <= visible || height < 3 {
		return strings.Repeat(" ", height)
	}

	// Calculate scroll bar position and size
	barHeight := height - 2 // space for arrows
	thumbSize := max(1, barHeight*visible/total)
	thumbPos := barHeight * offset / total

	var sb strings.Builder
	sb.WriteString(SubtitleStyle.Render("▲") + "\n") // Up arrow

	for i := 0; i < barHeight; i++ {
		if i >= thumbPos && i < thumbPos+thumbSize {
			sb.WriteString(lipgloss.NewStyle().Foreground(ColorPrimary).Render("█"))
		} else {
			sb.WriteString(lipgloss.NewStyle().Foreground(ColorBorder).Render("│"))
		}
		sb.WriteString("\n")
	}

	sb.WriteString(SubtitleStyle.Render("▼")) // Down arrow
	return sb.String()
}

// renderScrollInfo renders scroll position info like "[1-10 of 50]"
func renderScrollInfo(total, visible, offset int) string {
	if total <= visible {
		return ""
	}
	start := offset + 1
	end := min(offset+visible, total)
	return SubtitleStyle.Render(fmt.Sprintf("[%d-%d of %d]", start, end, total))
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// highlightShortcut highlights the [X] shortcut in a menu name
// e.g., "[D]ashboard" -> styled "[D]" + "ashboard"
func highlightShortcut(name string) string {
	// Find [X] pattern
	start := strings.Index(name, "[")
	end := strings.Index(name, "]")

	if start == -1 || end == -1 || end <= start {
		return name
	}

	before := name[:start]
	shortcut := name[start : end+1] // includes [ and ]
	after := name[end+1:]

	// Style the shortcut with accent color
	styledShortcut := lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true).
		Render(shortcut)

	return before + styledShortcut + after
}
