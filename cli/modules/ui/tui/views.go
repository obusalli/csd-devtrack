package tui

import (
	"fmt"
	"os"
	"sort"
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
	name  string // Name with [X] shortcut highlighted
	vtype core.ViewModelType
}{
	{"[D]ashboard", core.VMDashboard},
	{"[P]rojects", core.VMProjects},
	{"[B]uild", core.VMBuild},
	{"Pr[O]cesses", core.VMProcesses},
	{"[L]ogs", core.VMLogs},
	{"[G]it", core.VMGit},
	{"[C]onfig", core.VMConfig},
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
		// Selection indicator prefix (no chevron, use spaces)
		prefix := "  "

		// Apply consistent styling with same padding for all states
		var item string
		if m.currentView == v.vtype {
			// Current active view - use plain name (no shortcut highlighting)
			// to ensure background applies to all characters
			item = fmt.Sprintf("%s%s", prefix, v.name)
			item = NavItemActiveStyle.Width(itemWidth).Render(item)
		} else if i == m.sidebarIndex && m.focusArea == FocusSidebar {
			// Selected with focus (but not current view) - use plain name
			item = fmt.Sprintf("%s%s", prefix, v.name)
			item = lipgloss.NewStyle().
				Padding(0, 2). // Same padding as NavItemStyle
				Width(itemWidth).
				Background(ColorBgAlt).
				Foreground(ColorText).
				Render(item)
		} else {
			// Normal item - highlight shortcut
			displayName := highlightShortcut(v.name)
			item = fmt.Sprintf("%s%s", prefix, displayName)
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
	tabHint := HelpKeyStyle.Render("Tab") + HelpDescStyle.Render(" focus  ")

	shortcuts = append(shortcuts, navHint, tabHint)

	// View-specific shortcuts
	switch m.currentView {
	case core.VMDashboard, core.VMProjects:
		shortcuts = append(shortcuts,
			HelpKeyStyle.Render("b")+HelpDescStyle.Render(" build  "),
			HelpKeyStyle.Render("r")+HelpDescStyle.Render(" run  "),
			HelpKeyStyle.Render("s")+HelpDescStyle.Render(" stop  "),
			HelpKeyStyle.Render("k")+HelpDescStyle.Render(" kill  "),
			HelpKeyStyle.Render("l")+HelpDescStyle.Render(" logs  "),
		)
	case core.VMBuild:
		if m.state.Builds != nil && m.state.Builds.IsBuilding {
			shortcuts = append(shortcuts,
				HelpKeyStyle.Render("C-c")+HelpDescStyle.Render(" cancel  "),
			)
		} else {
			shortcuts = append(shortcuts,
				HelpKeyStyle.Render("b")+HelpDescStyle.Render(" build  "),
				HelpKeyStyle.Render("C-b")+HelpDescStyle.Render(" all  "),
			)
		}
	case core.VMProcesses:
		shortcuts = append(shortcuts,
			HelpKeyStyle.Render("b")+HelpDescStyle.Render(" build  "),
			HelpKeyStyle.Render("r")+HelpDescStyle.Render(" run  "),
			HelpKeyStyle.Render("s")+HelpDescStyle.Render(" stop  "),
			HelpKeyStyle.Render("k")+HelpDescStyle.Render(" kill  "),
			HelpKeyStyle.Render("l")+HelpDescStyle.Render(" logs  "),
		)
	case core.VMLogs:
		// Show cancel if a build is running
		if m.state.Builds != nil && m.state.Builds.IsBuilding {
			shortcuts = append(shortcuts,
				HelpKeyStyle.Render("C-c")+HelpDescStyle.Render(" cancel  "),
			)
		}
		if m.logSearchActive {
			// In search mode
			shortcuts = append(shortcuts,
				HelpKeyStyle.Render("Esc")+HelpDescStyle.Render(" exit  "),
				HelpKeyStyle.Render("Bksp")+HelpDescStyle.Render(" del  "),
			)
		} else {
			// Not in search mode
			shortcuts = append(shortcuts,
				HelpKeyStyle.Render("s/←→")+HelpDescStyle.Render(" source  "),
				HelpKeyStyle.Render("t")+HelpDescStyle.Render(" type  "),
				HelpKeyStyle.Render("e/w/a")+HelpDescStyle.Render(" level  "),
				HelpKeyStyle.Render("/")+HelpDescStyle.Render(" search  "),
				HelpKeyStyle.Render("c")+HelpDescStyle.Render(" clear all  "),
			)
		}
	case core.VMGit:
		if m.gitShowDiff {
			shortcuts = append(shortcuts,
				HelpKeyStyle.Render("↑↓")+HelpDescStyle.Render(" scroll  "),
				HelpKeyStyle.Render("S-↑↓")+HelpDescStyle.Render(" page  "),
				HelpKeyStyle.Render("Esc")+HelpDescStyle.Render(" back  "),
			)
		} else if m.focusArea == FocusDetail {
			shortcuts = append(shortcuts,
				HelpKeyStyle.Render("Enter")+HelpDescStyle.Render(" diff  "),
				HelpKeyStyle.Render("↑↓")+HelpDescStyle.Render(" select  "),
				HelpKeyStyle.Render("Esc")+HelpDescStyle.Render(" back  "),
			)
		} else {
			shortcuts = append(shortcuts,
				HelpKeyStyle.Render("Enter")+HelpDescStyle.Render(" files  "),
				HelpKeyStyle.Render("Tab")+HelpDescStyle.Render(" switch  "),
			)
		}
	case core.VMConfig:
		// Config-specific shortcuts based on current tab
		shortcuts = append(shortcuts,
			HelpKeyStyle.Render("←→")+HelpDescStyle.Render(" tabs  "),
		)
		switch m.configMode {
		case "projects":
			shortcuts = append(shortcuts,
				HelpKeyStyle.Render("Enter")+HelpDescStyle.Render(" browse  "),
				HelpKeyStyle.Render("x")+HelpDescStyle.Render(" remove  "),
			)
		case "browser":
			shortcuts = append(shortcuts,
				HelpKeyStyle.Render("Enter")+HelpDescStyle.Render(" open  "),
				HelpKeyStyle.Render("Bksp")+HelpDescStyle.Render(" back  "),
				HelpKeyStyle.Render("a")+HelpDescStyle.Render(" add  "),
				HelpKeyStyle.Render("x")+HelpDescStyle.Render(" remove  "),
			)
		case "settings":
			shortcuts = append(shortcuts,
				HelpKeyStyle.Render("↑↓")+HelpDescStyle.Render(" scroll  "),
			)
		}
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

// ProjectComponentRow represents a row in the projects view (one per component)
type ProjectComponentRow struct {
	ProjectName string
	ProjectIdx  int
	Component   core.ComponentVM
	IsFirst     bool // First component of the project (shows project name)
	GitBranch   string
	GitDirty    bool
	GitAhead    int
	IsSelf      bool
}

// renderProjects renders the projects view with one line per component
func (m *Model) renderProjects(width, height int) string {
	vm := m.state.Projects
	if vm == nil {
		return m.renderLoading()
	}

	title := PanelTitleStyle.Render("Projects")

	// Build flat list of component rows
	var componentRows []ProjectComponentRow
	for projIdx, p := range vm.Projects {
		if len(p.Components) == 0 {
			// Project with no components - show single row
			componentRows = append(componentRows, ProjectComponentRow{
				ProjectName: p.Name,
				ProjectIdx:  projIdx,
				IsFirst:     true,
				GitBranch:   p.GitBranch,
				GitDirty:    p.GitDirty,
				GitAhead:    p.GitAhead,
				IsSelf:      p.IsSelf,
			})
		} else {
			for i, comp := range p.Components {
				componentRows = append(componentRows, ProjectComponentRow{
					ProjectName: p.Name,
					ProjectIdx:  projIdx,
					Component:   comp,
					IsFirst:     i == 0,
					GitBranch:   p.GitBranch,
					GitDirty:    p.GitDirty,
					GitAhead:    p.GitAhead,
					IsSelf:      p.IsSelf,
				})
			}
		}
	}

	// Calculate dynamic column widths
	colProject := len("Project")
	colComponent := len("Component")
	colStatus := len("Status")
	colPort := len("Port")

	for _, r := range componentRows {
		if len(r.ProjectName) > colProject {
			colProject = len(r.ProjectName)
		}
		compLen := len(string(r.Component.Type))
		if compLen > colComponent {
			colComponent = compLen
		}
	}

	// Reasonable limits
	if colProject > 20 {
		colProject = 20
	}
	if colComponent > 10 {
		colComponent = 10
	}

	// Table header
	header := TableHeaderStyle.Render(fmt.Sprintf("  %-*s   %-*s   %-*s   %-*s   %s",
		colProject, "Project",
		colComponent, "Component",
		colStatus, "Status",
		colPort, "Port",
		"Git",
	))

	// Update maxMainItems for navigation
	m.maxMainItems = len(componentRows)

	// Table rows with scrolling
	var rows []string
	startIdx := m.mainScrollOffset
	endIdx := startIdx + m.visibleMainRows
	if endIdx > len(componentRows) {
		endIdx = len(componentRows)
	}

	for i := startIdx; i < endIdx; i++ {
		r := componentRows[i]

		// Project name: show only on first row, with self indicator
		projectDisplay := strings.Repeat(" ", colProject)
		if r.IsFirst {
			projectDisplay = fmt.Sprintf("%-*s", colProject, truncate(r.ProjectName, colProject))
			if r.IsSelf {
				projectDisplay = lipgloss.NewStyle().Foreground(ColorSecondary).Render("*") + projectDisplay[1:]
			}
		}

		// Component type
		compDisplay := fmt.Sprintf("%-*s", colComponent, string(r.Component.Type))

		// Status: running or stopped
		var statusDisplay string
		if r.Component.IsRunning {
			statusDisplay = StatusRunning.Render(IconRunning) + " " + fmt.Sprintf("%-*s", colStatus-2, "running")
		} else {
			statusDisplay = StatusStopped.Render(IconStopped) + " " + fmt.Sprintf("%-*s", colStatus-2, "stopped")
		}

		// Port
		portDisplay := fmt.Sprintf("%-*s", colPort, "")
		if r.Component.Port > 0 {
			portDisplay = fmt.Sprintf("%-*d", colPort, r.Component.Port)
		}

		// Git info: show only on first row
		gitDisplay := ""
		if r.IsFirst && r.GitBranch != "" {
			gitDisplay = fmt.Sprintf("%s %s", IconBranch, truncate(r.GitBranch, 10))
			if r.GitDirty {
				gitDisplay += GitDirtyStyle.Render(" *")
			}
			if r.GitAhead > 0 {
				gitDisplay += GitAheadStyle.Render(fmt.Sprintf(" ↑%d", r.GitAhead))
			}
		}

		row := fmt.Sprintf("%s   %s   %s   %s   %s",
			projectDisplay,
			compDisplay,
			statusDisplay,
			portDisplay,
			gitDisplay,
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
	if len(componentRows) > m.visibleMainRows {
		scrollInfo := SubtitleStyle.Render(fmt.Sprintf("  [%d-%d of %d]", startIdx+1, endIdx, len(componentRows)))
		rows = append(rows, scrollInfo)
	}

	if len(rows) == 0 {
		rows = append(rows, SubtitleStyle.Render("  No projects"))
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

	// Calculate dynamic column widths based on content
	colName := len("Project/Component") // minimum width
	colState := len("State")
	colPID := len("PID")
	colUptime := len("Uptime")
	colRestarts := len("Restarts")

	for _, p := range vm.Processes {
		nameLen := len(p.ProjectName) + 1 + len(string(p.Component)) // "name/component"
		if nameLen > colName {
			colName = nameLen
		}
		// State: icon (2 chars with space) + state text
		stateLen := 2 + len(string(p.State))
		if stateLen > colState {
			colState = stateLen
		}
		pidLen := len(fmt.Sprintf("%d", p.PID))
		if pidLen > colPID {
			colPID = pidLen
		}
		uptimeLen := len(p.Uptime)
		if uptimeLen > colUptime {
			colUptime = uptimeLen
		}
	}

	// Add padding
	colName += 2
	colState += 2
	colPID += 2
	colUptime += 2
	colRestarts += 2

	// Limit to available width (account for 4 separators of 3 spaces each)
	sepWidth := 3 * 4
	availableWidth := width - 10 - sepWidth // borders, padding, and separators
	totalWidth := colName + colState + colPID + colUptime + colRestarts
	if totalWidth > availableWidth {
		// Reduce name column first
		colName = availableWidth - colState - colPID - colUptime - colRestarts
		if colName < 15 {
			colName = 15
		}
	}

	// Table header (with 2-space prefix for alignment with row prefix)
	// State column needs +2 for the icon "● " prefix in rows
	header := TableHeaderStyle.Render(fmt.Sprintf("  %-*s   %-*s   %*s   %-*s   %*s",
		colName, "Project/Component",
		colState+2, "State", // +2 for icon prefix
		colPID, "PID",
		colUptime, "Uptime",
		colRestarts, "Restarts",
	))

	// Table rows
	var rows []string
	for i, p := range vm.Processes {
		// Build project/component name (plain text for proper alignment)
		projectComp := fmt.Sprintf("%s/%s", p.ProjectName, p.Component)

		// Pad to column width BEFORE adding styling
		projectCompPadded := fmt.Sprintf("%-*s", colName, projectComp)
		if p.IsSelf {
			// Add star prefix with color
			projectCompPadded = lipgloss.NewStyle().Foreground(ColorSecondary).Render("*") + projectCompPadded[1:]
		}

		// State: use text only, icon added with consistent width
		stateText := string(p.State)
		statePadded := fmt.Sprintf("%-*s", colState, stateText)
		stateIcon := StatusIcon(stateText)
		stateDisplay := stateIcon + " " + statePadded

		// Format row with proper column widths
		row := fmt.Sprintf("%s   %s   %*d   %-*s   %*d",
			projectCompPadded,
			stateDisplay,
			colPID, p.PID,
			colUptime, p.Uptime,
			colRestarts, p.Restarts,
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

	// Build source options from log lines
	m.updateLogSourceOptions()

	// Source filter (project/component) with status
	sourceLabel := SubtitleStyle.Render("Source:")
	var sourceValue string
	var sourceStatus string
	if m.logSourceFilter == "" {
		sourceValue = "ALL"
	} else {
		sourceValue = truncate(m.logSourceFilter, 15)
		// Get status for this source
		sourceStatus = m.getSourceStatus(m.logSourceFilter)
	}

	var sourceBox string
	if sourceStatus != "" {
		// Color based on status
		var statusStyle lipgloss.Style
		switch sourceStatus {
		case "running":
			statusStyle = StatusRunning
		case "building":
			statusStyle = StatusBuilding
		case "stopped":
			statusStyle = StatusStopped
		default:
			statusStyle = SubtitleStyle
		}
		sourceBox = ButtonActiveStyle.Render(" "+sourceValue+" ") + statusStyle.Render(sourceStatus) + ButtonActiveStyle.Render(" ◂▸")
	} else {
		sourceBox = ButtonActiveStyle.Render(" " + sourceValue + " ◂▸")
	}

	// Type filter (build/process)
	typeLabel := SubtitleStyle.Render("Type:")
	typeButtons := []string{}
	for _, t := range []struct{ lbl, val string }{{"ALL", ""}, {"BUILD", "build"}, {"RUN", "process"}} {
		if m.logTypeFilter == t.val {
			typeButtons = append(typeButtons, ButtonActiveStyle.Render(t.lbl))
		} else {
			typeButtons = append(typeButtons, ButtonStyle.Render(t.lbl))
		}
	}
	typeBar := strings.Join(typeButtons, " ")

	// Level filter
	levelLabel := SubtitleStyle.Render("Level:")
	levelButtons := []string{}
	for _, l := range []struct{ lbl, val string }{{"ALL", ""}, {"ERR", "error"}, {"WRN", "warn"}, {"INF", "info"}} {
		if m.logLevelFilter == l.val {
			levelButtons = append(levelButtons, ButtonActiveStyle.Render(l.lbl))
		} else {
			levelButtons = append(levelButtons, ButtonStyle.Render(l.lbl))
		}
	}
	levelBar := strings.Join(levelButtons, " ")

	// Search box
	searchLabel := SubtitleStyle.Render("Search:")
	var searchBox string
	if m.logSearchActive {
		searchBox = InputFocusedStyle.Width(20).Render(m.logSearchText + "█")
	} else if m.logSearchText != "" {
		searchBox = InputStyle.Width(20).Render(m.logSearchText)
	} else {
		searchBox = InputStyle.Width(20).Render(SubtitleStyle.Render("/ to search"))
	}

	// Filter bar row 1: Source and Type
	filterBar1 := lipgloss.JoinHorizontal(lipgloss.Center,
		sourceLabel, " ", sourceBox,
		"   ",
		typeLabel, " ", typeBar,
	)

	// Filter bar row 2: Level and Search
	filterBar2 := lipgloss.JoinHorizontal(lipgloss.Center,
		levelLabel, " ", levelBar,
		"   ",
		searchLabel, " ", searchBox,
	)

	// Filter log lines
	var filteredLines []core.LogLineVM
	for _, line := range vm.Lines {
		// Source filter
		if m.logSourceFilter != "" {
			if !strings.HasPrefix(line.Source, m.logSourceFilter) {
				continue
			}
		}
		// Type filter (build: starts with "build:", process: doesn't start with "build:")
		if m.logTypeFilter != "" {
			isBuild := strings.HasPrefix(line.Source, "build:")
			if m.logTypeFilter == "build" && !isBuild {
				continue
			}
			if m.logTypeFilter == "process" && isBuild {
				continue
			}
		}
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
	maxLines := height - 10 // Account for 2 filter rows + stats line
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
		"Showing %d of %d lines",
		len(filteredLines), len(vm.Lines)))

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
		lipgloss.JoinVertical(lipgloss.Left, title, filterBar1, filterBar2, statsLine, "", content),
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

	// If showing diff, render diff view
	if m.gitShowDiff {
		return m.renderGitDiff(width, height)
	}

	title := PanelTitleStyle.Render("Git Status")

	// Projects list (left panel) - narrower to give more space to details
	listWidth := width / 3
	if listWidth < 35 {
		listWidth = 35
	}
	var projectRows []string
	for i, p := range vm.Projects {
		status := GitStatusIcon(p.IsClean)
		branchInfo := truncate(p.Branch, 10)

		var syncInfo string
		if p.Ahead > 0 {
			syncInfo += GitAheadStyle.Render(fmt.Sprintf("↑%d", p.Ahead))
		}
		if p.Behind > 0 {
			syncInfo += GitBehindStyle.Render(fmt.Sprintf("↓%d", p.Behind))
		}

		row := fmt.Sprintf("%s %-12s %-10s %s",
			status,
			truncate(p.ProjectName, 12),
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

	// Detail panel (right) - file list with selection
	detailWidth := width - listWidth - 2
	detailHeight := height - 6
	var detailContent string

	if m.mainIndex >= 0 && m.mainIndex < len(vm.Projects) {
		p := vm.Projects[m.mainIndex]

		// Build flat file list (also sets maxDetailItems)
		m.buildGitFileList(&p)
		m.visibleDetailRows = detailHeight - 4

		detailLines := []string{
			PanelTitleStyle.Render(p.ProjectName),
			fmt.Sprintf("Branch: %s", GitBranchStyle.Render(p.Branch)),
			"",
		}

		if len(m.gitFiles) == 0 {
			detailLines = append(detailLines, StatusSuccess.Render("✓ Working tree clean"))
		} else {
			// Calculate scroll offset
			if m.detailIndex < m.detailScrollOffset {
				m.detailScrollOffset = m.detailIndex
			} else if m.detailIndex >= m.detailScrollOffset+m.visibleDetailRows {
				m.detailScrollOffset = m.detailIndex - m.visibleDetailRows + 1
			}

			endIdx := m.detailScrollOffset + m.visibleDetailRows
			if endIdx > len(m.gitFiles) {
				endIdx = len(m.gitFiles)
			}

			for i := m.detailScrollOffset; i < endIdx; i++ {
				f := m.gitFiles[i]
				var statusStyle lipgloss.Style
				var prefix string
				switch f.Status {
				case "staged":
					statusStyle = StatusSuccess
					prefix = "A"
				case "modified":
					statusStyle = StatusWarning
					prefix = "M"
				case "deleted":
					statusStyle = StatusError
					prefix = "D"
				case "untracked":
					statusStyle = SubtitleStyle
					prefix = "?"
				}

				row := fmt.Sprintf("%s %s", statusStyle.Render(prefix), truncate(f.Path, detailWidth-8))

				if i == m.detailIndex && m.focusArea == FocusDetail {
					row = TableRowSelectedStyle.Width(detailWidth - 6).Render(FocusIndicator + " " + row)
				} else if i == m.detailIndex {
					row = TableRowSelectedStyle.Width(detailWidth - 6).Render("  " + row)
				} else {
					row = "  " + row
				}
				detailLines = append(detailLines, row)
			}

			// Scroll indicator
			if len(m.gitFiles) > m.visibleDetailRows {
				scrollInfo := fmt.Sprintf(" [%d-%d/%d]", m.detailScrollOffset+1, endIdx, len(m.gitFiles))
				detailLines = append(detailLines, SubtitleStyle.Render(scrollInfo))
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

// buildGitFileList builds a flat list of all files from git status
// Only rebuilds if the project or file count changed
func (m *Model) buildGitFileList(p *core.GitStatusVM) {
	// Calculate expected file count
	expectedCount := len(p.Staged) + len(p.Modified) + len(p.Deleted) + len(p.Untracked)

	// Only rebuild if project changed or file count changed
	if m.gitFilesProjectID == p.ProjectID && len(m.gitFiles) == expectedCount {
		// Still update maxDetailItems for navigation
		m.maxDetailItems = len(m.gitFiles)
		return
	}

	m.gitFiles = make([]GitFileEntry, 0, expectedCount)
	m.gitFilesProjectID = p.ProjectID

	for _, f := range p.Staged {
		m.gitFiles = append(m.gitFiles, GitFileEntry{Path: f, Status: "staged"})
	}
	for _, f := range p.Modified {
		m.gitFiles = append(m.gitFiles, GitFileEntry{Path: f, Status: "modified"})
	}
	for _, f := range p.Deleted {
		m.gitFiles = append(m.gitFiles, GitFileEntry{Path: f, Status: "deleted"})
	}
	for _, f := range p.Untracked {
		m.gitFiles = append(m.gitFiles, GitFileEntry{Path: f, Status: "untracked"})
	}

	// Update maxDetailItems for navigation
	m.maxDetailItems = len(m.gitFiles)

	// Reset detail index only if out of bounds
	if m.detailIndex >= len(m.gitFiles) {
		m.detailIndex = 0
		m.detailScrollOffset = 0
	}
}

// renderGitDiff renders the diff view
func (m *Model) renderGitDiff(width, height int) string {
	title := PanelTitleStyle.Render("Git Diff")
	hint := SubtitleStyle.Render("Press Escape to go back")

	contentHeight := height - 6
	var lines []string

	// Calculate scroll
	m.visibleDetailRows = contentHeight - 2
	if m.detailScrollOffset < 0 {
		m.detailScrollOffset = 0
	}
	endIdx := m.detailScrollOffset + m.visibleDetailRows
	if endIdx > len(m.gitDiffContent) {
		endIdx = len(m.gitDiffContent)
	}

	for i := m.detailScrollOffset; i < endIdx; i++ {
		line := m.gitDiffContent[i]
		// Color diff lines
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			line = lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff00")).Render(line)
		} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
			line = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff0000")).Render(line)
		} else if strings.HasPrefix(line, "@@") {
			line = lipgloss.NewStyle().Foreground(lipgloss.Color("#00ffff")).Render(line)
		} else if strings.HasPrefix(line, "diff ") || strings.HasPrefix(line, "index ") {
			line = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Render(line)
		}
		lines = append(lines, truncate(line, width-6))
	}

	// Scroll indicator
	if len(m.gitDiffContent) > m.visibleDetailRows {
		scrollInfo := fmt.Sprintf(" [%d-%d/%d lines]", m.detailScrollOffset+1, endIdx, len(m.gitDiffContent))
		lines = append(lines, SubtitleStyle.Render(scrollInfo))
	}

	content := strings.Join(lines, "\n")

	panel := FocusedBorderStyle.Width(width - 2).Height(height - 4).Render(
		lipgloss.JoinVertical(lipgloss.Left, title, hint, "", content),
	)

	return panel
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
		{"projects", "Projects"},
		{"browser", "Browser"},
		{"settings", "Settings"},
	}
	for _, mode := range modes {
		if m.configMode == mode.key {
			tabs = append(tabs, tabActive.Render(mode.name))
		} else {
			tabs = append(tabs, tabInactive.Render(mode.name))
		}
	}
	tabBar := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
	tabHint := SubtitleStyle.Render("  ←/→")
	tabBar = lipgloss.JoinHorizontal(lipgloss.Center, tabBar, tabHint)

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
		isSelected := i == m.mainIndex && m.focusArea == FocusMain

		indicator := "  "
		if isSelected {
			indicator = "> "
		}

		// Component badges - sort for consistent order
		var compBadges []string
		for compType := range proj.Components {
			compBadges = append(compBadges, string(compType))
		}
		sort.Strings(compBadges)
		compsText := ""
		if len(compBadges) > 0 {
			compsText = strings.Join(compBadges, ", ")
		}

		// Build the row with fixed columns
		row := fmt.Sprintf("%s%-20s │ %s", indicator, truncate(proj.Name, 20), compsText)

		if isSelected {
			row = TableRowSelectedStyle.Render(row)
		}
		rows = append(rows, row)
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

// renderConfigSettings renders the settings in config view (raw YAML file)
func (m *Model) renderConfigSettings(width, height int) string {
	configPath := config.GetGlobalPath()
	if configPath == "" {
		return SubtitleStyle.Render("No config file loaded")
	}

	title := PanelTitleStyle.Render("Config File (read-only)")
	pathInfo := SubtitleStyle.Render(fmt.Sprintf("📄 %s", configPath))

	// Read the raw config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return lipgloss.JoinVertical(lipgloss.Left,
			title,
			pathInfo,
			"",
			StatusError.Render(fmt.Sprintf("Error reading file: %v", err)),
		)
	}

	// Split into lines
	lines := strings.Split(string(data), "\n")
	totalLines := len(lines)
	visibleLines := height - 8
	if visibleLines < 5 {
		visibleLines = 5
	}

	// Update maxMainItems for scroll navigation
	m.maxMainItems = totalLines

	// Calculate scroll offset based on mainIndex
	scrollOffset := m.mainIndex
	if scrollOffset > totalLines-visibleLines {
		scrollOffset = totalLines - visibleLines
	}
	if scrollOffset < 0 {
		scrollOffset = 0
	}

	// Display lines with scroll
	var displayLines []string
	endIdx := scrollOffset + visibleLines
	if endIdx > totalLines {
		endIdx = totalLines
	}

	for i := scrollOffset; i < endIdx; i++ {
		lineNum := lipgloss.NewStyle().Foreground(ColorMuted).Render(fmt.Sprintf("%3d │ ", i+1))
		displayLines = append(displayLines, lineNum+lines[i])
	}

	// Scroll indicator
	scrollInfo := ""
	if totalLines > visibleLines {
		scrollInfo = SubtitleStyle.Render(fmt.Sprintf("  [%d-%d of %d lines] ↑↓ to scroll", scrollOffset+1, endIdx, totalLines))
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		title,
		pathInfo,
		scrollInfo,
		"",
		strings.Join(displayLines, "\n"),
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
		HelpKeyStyle.Render("Global Navigation"),
		"  ↑/↓       Navigate items",
		"  Tab       Switch focus between panels",
		"  D P B O   Dashboard, Projects, Build, PrOcesses",
		"  L G C     Logs, Git, Config",
		"  PgUp/Dn   Page scroll",
		"  Esc       Back / Cancel",
		"",
		HelpKeyStyle.Render("Actions (Projects/Processes/Dashboard)"),
		"  b         Build selected component",
		"  r         Run/Start component",
		"  s         Stop component",
		"  k         Kill (force stop)",
		"  l         View logs for component",
		"",
		HelpKeyStyle.Render("Build"),
		"  Ctrl+B    Build all projects",
		"  Ctrl+C    Cancel current build",
		"",
		HelpKeyStyle.Render("Logs"),
		"  s/←→      Cycle source (project/component)",
		"  t         Cycle type (all/build/run)",
		"  e w i a   Filter level: error/warn/info/all",
		"  /         Enter search mode, Esc to exit",
		"  c         Clear all filters",
		"",
		HelpKeyStyle.Render("Git"),
		"  Enter     Show files / Show diff",
		"  Esc       Back to project list",
		"",
		HelpKeyStyle.Render("Config"),
		"  ←→        Switch tabs",
		"  a         Add project (in browser)",
		"  x         Remove project",
		"",
		HelpKeyStyle.Render("Other"),
		"  Ctrl+R    Refresh data",
		"  ?         Toggle this help",
		"  q         Quit",
		"",
		SubtitleStyle.Render("Press any key to close"),
	}

	helpBox := DialogStyle.Width(55).Render(strings.Join(helpContent, "\n"))

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
