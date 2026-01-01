package tui

import (
	"fmt"
	"strings"
	"time"

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
	title := TitleStyle.Render("CSD DevTrack")
	version := SubtitleStyle.Render("v1.0.0")

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
	name  string
	icon  string
	vtype core.ViewModelType
}{
	{"1", "Dashboard", "◉", core.VMDashboard},
	{"2", "Projects", "◎", core.VMProjects},
	{"3", "Build", "⚙", core.VMBuild},
	{"4", "Processes", "▶", core.VMProcesses},
	{"5", "Logs", "☰", core.VMLogs},
	{"6", "Git", "⎇", core.VMGit},
	{"7", "Config", "⚙", core.VMConfig},
}

// getSidebarWidth calculates the optimal sidebar width based on menu items
func getSidebarWidth() int {
	maxLen := 0
	for _, v := range sidebarViews {
		// Format: "▶ X Name" = indicator(2) + key(1) + space(1) + name
		itemLen := 4 + len(v.name)
		if itemLen > maxLen {
			maxLen = itemLen
		}
	}
	// Add padding for borders (2 each side) + internal padding + margin
	// Borders: 2, internal padding: 4, safety margin: 2
	return maxLen + 10
}

// renderSidebar renders the left navigation sidebar
func (m *Model) renderSidebar() string {
	width := getSidebarWidth()

	var items []string
	for i, v := range sidebarViews {
		// Build the menu item text (without indicator)
		menuText := fmt.Sprintf("%s %s", v.key, v.name)

		// Selection indicator prefix (using fixed-width ASCII)
		var prefix string
		if i == m.sidebarIndex {
			if m.focusArea == FocusSidebar {
				prefix = "> " // Active focus
			} else {
				prefix = "* " // Selected but not focused
			}
		} else {
			prefix = "  " // Not selected
		}

		item := prefix + menuText

		// Apply consistent styling with same padding for all states
		itemWidth := width - 4
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
			HelpKeyStyle.Render("b")+HelpDescStyle.Render(" build  "),
			HelpKeyStyle.Render("B")+HelpDescStyle.Render(" build all  "),
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
			HelpKeyStyle.Render("/")+HelpDescStyle.Render(" filter  "),
			HelpKeyStyle.Render("G")+HelpDescStyle.Render(" bottom  "),
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

// renderDashboard renders the dashboard view
func (m *Model) renderDashboard(width, height int) string {
	vm := m.state.Dashboard
	if vm == nil {
		return m.renderLoading()
	}

	// Stats row
	stats := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.renderStatBox("Projects", fmt.Sprintf("%d", vm.ProjectCount), ColorSecondary),
		m.renderStatBox("Running", fmt.Sprintf("%d", vm.RunningCount), ColorSuccess),
		m.renderStatBox("Building", fmt.Sprintf("%d", vm.BuildingCount), ColorWarning),
		m.renderStatBox("Errors", fmt.Sprintf("%d", vm.ErrorCount), ColorError),
	)

	// Projects list
	projectsPanel := m.renderProjectsList(vm.Projects, width/2-2, height-8, m.focusArea == FocusMain)

	// Processes panel
	processesPanel := m.renderProcessesList(vm.RunningProcesses, width/2-2, height-8, false)

	panels := lipgloss.JoinHorizontal(lipgloss.Top, projectsPanel, processesPanel)

	return lipgloss.JoinVertical(lipgloss.Left,
		PanelTitleStyle.Render("Dashboard"),
		"",
		stats,
		"",
		panels,
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

	// Current build status
	var buildStatus string
	if vm.CurrentBuild != nil {
		b := vm.CurrentBuild
		progress := renderProgressBar(b.Progress, 20)
		buildStatus = fmt.Sprintf(
			"%s Building %s/%s %s\n",
			m.spinner.View(),
			b.ProjectName,
			b.Component,
			progress,
		)

		// Build output (last lines)
		outputLines := b.Output
		maxLines := height - 12
		if len(outputLines) > maxLines {
			outputLines = outputLines[len(outputLines)-maxLines:]
		}
		for _, line := range outputLines {
			buildStatus += LogInfoStyle.Render(truncate(line, width-10)) + "\n"
		}
	} else if vm.IsBuilding {
		buildStatus = m.spinner.View() + " Building..."
	} else {
		buildStatus = SubtitleStyle.Render("No active build. Press 'b' to build selected project.")
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
			"",
			buildStatus,
			"",
			historyTitle,
			strings.Join(historyLines, "\n"),
		),
	)
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

// renderLogs renders the logs view
func (m *Model) renderLogs(width, height int) string {
	vm := m.state.Logs
	if vm == nil {
		return m.renderLoading()
	}

	title := PanelTitleStyle.Render("Logs")

	// Filter info
	filterInfo := SubtitleStyle.Render(fmt.Sprintf(
		"Filter: %s  │  Auto-scroll: %v",
		orDefault(vm.FilterProject, "all"),
		vm.AutoScroll,
	))

	// Log lines
	var logLines []string
	maxLines := height - 6
	start := 0
	if len(vm.Lines) > maxLines {
		start = len(vm.Lines) - maxLines
	}

	for _, line := range vm.Lines[start:] {
		timestamp := LogTimestampStyle.Render(line.TimeStr)
		source := LogSourceStyle.Render(fmt.Sprintf("[%-15s]", truncate(line.Source, 15)))

		var levelStyle lipgloss.Style
		switch line.Level {
		case "error":
			levelStyle = LogErrorStyle
		case "warn":
			levelStyle = LogWarnStyle
		case "debug":
			levelStyle = LogDebugStyle
		default:
			levelStyle = LogInfoStyle
		}

		logLine := fmt.Sprintf("%s %s %s",
			timestamp, source, levelStyle.Render(truncate(line.Message, width-35)))
		logLines = append(logLines, logLine)
	}

	if len(logLines) == 0 {
		logLines = append(logLines, SubtitleStyle.Render("No logs"))
	}

	content := strings.Join(logLines, "\n")

	var style lipgloss.Style
	if m.focusArea == FocusMain {
		style = FocusedBorderStyle
	} else {
		style = UnfocusedBorderStyle
	}

	return style.Width(width - 2).Height(height - 2).Render(
		lipgloss.JoinVertical(lipgloss.Left, title, filterInfo, "", content),
	)
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

// renderConfig renders the config view
func (m *Model) renderConfig(width, height int) string {
	vm := m.state.Config
	if vm == nil {
		return m.renderLoading()
	}

	title := PanelTitleStyle.Render("Configuration")
	pathInfo := SubtitleStyle.Render(fmt.Sprintf("Config: %s", vm.ConfigPath))

	// Settings
	var settingsLines []string
	for key, value := range vm.Settings {
		settingsLines = append(settingsLines,
			fmt.Sprintf("  %s: %v", key, value))
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
			pathInfo,
			"",
			PanelTitleStyle.Render("Settings"),
			strings.Join(settingsLines, "\n"),
		),
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
