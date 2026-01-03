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
	version := SubtitleStyle.Render("v" + modules.AppVersion + " (" + modules.BuildHash() + ")")

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

	// Git loading indicator
	gitStr := ""
	if m.state.GitLoading {
		gitStr = lipgloss.NewStyle().Foreground(ColorWarning).Render(" " + m.spinner.View() + "git")
	}

	// System metrics (CPU, RAM, Load)
	metricsStr := ""
	if m.metricsCollector != nil {
		metrics := m.metricsCollector.Get()
		// CPU with color based on usage
		cpuStyle := lipgloss.NewStyle().Foreground(ColorSuccess)
		if metrics.CPUPercent > 80 {
			cpuStyle = lipgloss.NewStyle().Foreground(ColorError)
		} else if metrics.CPUPercent > 50 {
			cpuStyle = lipgloss.NewStyle().Foreground(ColorWarning)
		}
		cpuStr := cpuStyle.Render(fmt.Sprintf("CPU:%.0f%%", metrics.CPUPercent))

		// RAM with color based on usage
		ramStyle := lipgloss.NewStyle().Foreground(ColorSuccess)
		if metrics.MemPercent > 80 {
			ramStyle = lipgloss.NewStyle().Foreground(ColorError)
		} else if metrics.MemPercent > 50 {
			ramStyle = lipgloss.NewStyle().Foreground(ColorWarning)
		}
		ramStr := ramStyle.Render(fmt.Sprintf("RAM:%.1f/%.0fG", metrics.MemUsedGB, metrics.MemTotalGB))

		// Load average with color based on load vs CPU count
		loadStyle := lipgloss.NewStyle().Foreground(ColorSuccess)
		if metrics.LoadAvg1 > float64(metrics.NumCPU) {
			loadStyle = lipgloss.NewStyle().Foreground(ColorError)
		} else if metrics.LoadAvg1 > float64(metrics.NumCPU)*0.7 {
			loadStyle = lipgloss.NewStyle().Foreground(ColorWarning)
		}
		loadStr := loadStyle.Render(fmt.Sprintf("Load:%.2f", metrics.LoadAvg1))

		metricsStr = fmt.Sprintf("%s %s %s", cpuStr, ramStr, loadStr)
	}

	// Current view indicator
	viewName := strings.ToUpper(string(m.currentView))

	// Claude usage info if in Claude view with active session
	usageStr := ""
	if m.currentView == core.VMClaude && m.state.Claude != nil && m.state.Claude.Usage != nil {
		usage := m.state.Claude.Usage
		usageStr = lipgloss.NewStyle().Foreground(ColorSecondary).Render(
			fmt.Sprintf(" │ %dk tokens", usage.TotalTokens/1000),
		)
		if usage.CostUSD > 0 {
			usageStr += lipgloss.NewStyle().Foreground(ColorMuted).Render(
				fmt.Sprintf(" ~$%.2f", usage.CostUSD),
			)
		}
	}

	left := fmt.Sprintf(" %s %s │ %s%s", title, version, viewName, usageStr)
	right := fmt.Sprintf("%s │ %s%s%s ", metricsStr, status, runningStr, gitStr)

	leftWidth := lipgloss.Width(left)
	rightWidth := lipgloss.Width(right)

	// Header events ticker (scrolling animation)
	centerContent := ""
	events := m.state.GetHeaderEvents()
	if len(events) > 0 {
		// Build ticker string from all events
		availableWidth := m.width - leftWidth - rightWidth - 4
		if availableWidth > 10 {
			centerContent = m.renderHeaderTicker(events, availableWidth)
		}
	}
	centerWidth := lipgloss.Width(centerContent)
	totalPadding := m.width - leftWidth - rightWidth - centerWidth
	if totalPadding < 0 {
		totalPadding = 0
	}

	leftPad := totalPadding / 2
	rightPad := totalPadding - leftPad

	header := lipgloss.JoinHorizontal(
		lipgloss.Center,
		left,
		strings.Repeat(" ", leftPad),
		centerContent,
		strings.Repeat(" ", rightPad),
		right,
	)

	return lipgloss.NewStyle().
		Background(ColorBgAlt).
		Width(m.width).
		Render(header)
}

// renderHeaderTicker renders a scrolling ticker of header events
func (m *Model) renderHeaderTicker(events []*core.HeaderEvent, maxWidth int) string {
	if len(events) == 0 || maxWidth <= 0 {
		return ""
	}

	// Build the full ticker text with separators
	separator := "  ◆  "
	var parts []string
	for _, event := range events {
		text := event.Icon + " " + event.Message
		parts = append(parts, text)
	}

	// Join all events
	fullText := strings.Join(parts, separator)

	// If only one event and it fits, just show it (no scrolling needed)
	if len(events) == 1 && lipgloss.Width(fullText) <= maxWidth {
		eventStyle := m.getEventStyle(events[0].Type)
		return eventStyle.Render(fullText)
	}

	// Add separator at end for continuous scroll effect
	fullText = fullText + separator

	// Calculate scroll position (2 chars per tick for smooth scrolling)
	textLen := len([]rune(fullText))
	scrollOffset := (m.tickerScrollPos * 2) % textLen

	// Create scrolling view by rotating the string
	runes := []rune(fullText)
	rotated := append(runes[scrollOffset:], runes[:scrollOffset]...)

	// Truncate to available width
	visible := string(rotated)
	if lipgloss.Width(visible) > maxWidth {
		// Truncate carefully to not break in middle of emoji
		visibleRunes := []rune(visible)
		width := 0
		cutPoint := 0
		for i, r := range visibleRunes {
			charWidth := lipgloss.Width(string(r))
			if width+charWidth > maxWidth {
				break
			}
			width += charWidth
			cutPoint = i + 1
		}
		visible = string(visibleRunes[:cutPoint])
	}

	// Apply color based on first event type (or blend if multiple)
	eventStyle := m.getEventStyle(events[0].Type)
	return eventStyle.Render(visible)
}

// getEventStyle returns the style for a header event type
func (m *Model) getEventStyle(eventType core.HeaderEventType) lipgloss.Style {
	switch eventType {
	case core.HeaderEventSuccess:
		return lipgloss.NewStyle().Foreground(ColorSuccess)
	case core.HeaderEventWarning:
		return lipgloss.NewStyle().Foreground(ColorWarning)
	case core.HeaderEventError:
		return lipgloss.NewStyle().Foreground(ColorError)
	default:
		return lipgloss.NewStyle().Foreground(ColorSecondary)
	}
}

// sidebarView represents a navigation menu item
type sidebarView struct {
	name  string // Name with [X] shortcut highlighted
	vtype core.ViewModelType
}

// baseSidebarViews defines the base navigation menu items (always shown)
var baseSidebarViews = []sidebarView{
	{"Dash[B]oard", core.VMDashboard},
	{"Coc[K]pit", core.VMCockpit},
	{"[P]rojects", core.VMProjects},
	{"B[U]ilds", core.VMBuild},
	{"Pr[O]cesses", core.VMProcesses},
	{"[L]ogs", core.VMLogs},
	{"[G]it", core.VMGit},
}

// getSidebarViews returns the sidebar views, filtered by available capabilities
func (m *Model) getSidebarViews() []sidebarView {
	views := make([]sidebarView, len(baseSidebarViews))
	copy(views, baseSidebarViews)

	// Add Claude Code view if capabilities available (tmux + claude)
	if m.state.Capabilities != nil && m.state.Capabilities.HasClaude() {
		views = append(views, sidebarView{"[C]laude Code", core.VMClaude})
	}

	// Add Codex view if capabilities available (tmux + codex)
	if m.state.Capabilities != nil && m.state.Capabilities.HasCodex() {
		views = append(views, sidebarView{"Code[X]", core.VMCodex})
	}

	// Add Database view if capabilities available (tmux + db client) and databases configured
	if m.state.Capabilities != nil && m.state.Capabilities.HasDatabase() &&
		m.state.Database != nil && len(m.state.Database.Databases) > 0 {
		views = append(views, sidebarView{"[D]atabases", core.VMDatabase})
	}

	// Add Shell view if capabilities available (tmux + shell)
	if m.state.Capabilities != nil && m.state.Capabilities.HasShell() {
		views = append(views, sidebarView{"[T]erminal", core.VMShell})
	}

	// Settings always last
	views = append(views, sidebarView{"[S]ettings", core.VMConfig})

	return views
}

// getSidebarWidth returns a fixed width that fits all menu items
func getSidebarWidth() int {
	// Fixed wider width for context panel with logs
	// Minimum 32 chars to fit log lines
	minWidth := 32

	// Find longest name from base views
	maxLen := 0
	for _, v := range baseSidebarViews {
		if len(v.name) > maxLen {
			maxLen = len(v.name)
		}
	}
	// Also account for dynamic entries
	// "[C]laude Code" = 13 chars, "[S]ettings" = 10 chars
	if len("[C]laude Code") > maxLen {
		maxLen = len("[C]laude Code")
	}
	// Format: "> 1 [D]ashboard" = prefix(2) + key(1) + space(1) + name
	// + borders(2) + padding(4) + margin(2)
	width := maxLen + 12
	if width < minWidth {
		width = minWidth
	}
	return width
}

// renderSidebar renders the left navigation sidebar with context panel below
func (m *Model) renderSidebar() string {
	width := getSidebarWidth()
	totalHeight := m.height - 6

	// Calculate menu height based on items
	sidebarViews := m.getSidebarViews()
	menuHeight := len(sidebarViews) + 6 // items + header + separator + hints + borders

	// Configure and render TreeMenu for sidebar
	m.sidebarMenu.SetSize(width, menuHeight)
	m.sidebarMenu.SetFocused(m.focusArea == FocusSidebar)
	menuPanel := m.sidebarMenu.Render()

	// Context panel takes remaining space (separate panel with its own border)
	contextHeight := totalHeight - menuHeight - GapVertical - 2 // 2 for context border
	if contextHeight < 5 {
		contextHeight = 5
	}

	contextPanel := m.renderContextPanel(width, contextHeight)

	// Stack menu and context panels vertically
	return lipgloss.JoinVertical(lipgloss.Left,
		menuPanel,
		contextPanel,
	)
}

// renderContextPanel renders the context panel showing current project/git info
func (m *Model) renderContextPanel(width, height int) string {
	innerWidth := width - 4 // Account for border and padding
	var lines []string

	// Title (aligned with MENU)
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorSecondary).
		Padding(0, 2).
		Width(innerWidth)
	lines = append(lines, titleStyle.Render("≡ CONTEXT"))

	separator := lipgloss.NewStyle().
		Foreground(ColorBorder).
		Padding(0, 2).
		Width(innerWidth).
		Render(strings.Repeat("─", innerWidth-4))
	lines = append(lines, separator)

	// Section header style
	sectionStyle := lipgloss.NewStyle().Foreground(ColorMuted).Bold(true)
	labelStyle := lipgloss.NewStyle().Foreground(ColorMuted)

	// Get active project context based on current view
	activeProject := m.getActiveProjectContext()

	if activeProject != nil {
		// Project name (prominent)
		projectStyle := lipgloss.NewStyle().Foreground(ColorText).Bold(true).Padding(0, 2)
		lines = append(lines, projectStyle.Render(truncate(activeProject.Name, innerWidth-4)))

		// ─── Git Status ───
		lines = append(lines, "")
		lines = append(lines, sectionStyle.Padding(0, 2).Render("─ Git Status ─"))

		if activeProject.GitBranch != "" {
			// Branch
			branchLine := labelStyle.Render("  ⎇ ") + GitBranchStyle.Render(truncate(activeProject.GitBranch, innerWidth-6))
			lines = append(lines, branchLine)

			// Status: clean/dirty + sync
			var statusIcon, statusText string
			if activeProject.GitDirty {
				statusIcon = GitDirtyStyle.Render("  ● ")
				statusText = GitDirtyStyle.Render("modified")
			} else {
				statusIcon = StatusSuccess.Render("  ✓ ")
				statusText = StatusSuccess.Render("clean")
			}
			syncText := ""
			if activeProject.GitAhead > 0 {
				syncText += " " + GitAheadStyle.Render(fmt.Sprintf("↑%d", activeProject.GitAhead))
			}
			if activeProject.GitBehind > 0 {
				syncText += " " + GitBehindStyle.Render(fmt.Sprintf("↓%d", activeProject.GitBehind))
			}
			lines = append(lines, statusIcon+statusText+syncText)

			// Recent files (up to 5) - same format as Git view
			if m.state.Git != nil {
				for _, g := range m.state.Git.Projects {
					if g.ProjectID == activeProject.ID && !g.IsClean {
						// Collect all files with their status (same order as Git view)
						type fileEntry struct {
							path   string
							status string
							prefix string
						}
						var files []fileEntry
						for _, f := range g.Staged {
							files = append(files, fileEntry{f, "staged", "A"})
						}
						for _, f := range g.Modified {
							files = append(files, fileEntry{f, "modified", "M"})
						}
						for _, f := range g.Deleted {
							files = append(files, fileEntry{f, "deleted", "D"})
						}
						for _, f := range g.Untracked {
							files = append(files, fileEntry{f, "untracked", "?"})
						}

						// Show up to 5 files
						maxFiles := 5
						if len(files) < maxFiles {
							maxFiles = len(files)
						}
						for i := 0; i < maxFiles; i++ {
							f := files[i]
							// Get just filename, not full path
							name := f.path
							if idx := strings.LastIndex(name, "/"); idx >= 0 {
								name = name[idx+1:]
							}
							name = truncate(name, innerWidth-6)

							var statusStyle lipgloss.Style
							switch f.status {
							case "staged":
								statusStyle = StatusSuccess
							case "modified":
								statusStyle = StatusWarning
							case "deleted":
								statusStyle = StatusError
							default:
								statusStyle = SubtitleStyle
							}
							fileLine := fmt.Sprintf("  %s %s", statusStyle.Render(f.prefix), name)
							lines = append(lines, fileLine)
						}

						// Show remaining count if any
						remaining := len(files) - maxFiles
						if remaining > 0 {
							lines = append(lines, SubtitleStyle.Render(fmt.Sprintf("    +%d more", remaining)))
						}
						break
					}
				}
			}
		} else {
			lines = append(lines, labelStyle.Padding(0, 2).Render("No git info"))
		}

		// ─── Processes ───
		lines = append(lines, "")
		lines = append(lines, sectionStyle.Padding(0, 2).Render("─ Processes ─"))

		var projectProcesses []core.ProcessVM
		if m.state.Processes != nil {
			for _, p := range m.state.Processes.Processes {
				if p.ProjectID == activeProject.ID {
					projectProcesses = append(projectProcesses, p)
				}
			}
		}

		if len(projectProcesses) > 0 {
			// Show up to 3 most recent processes
			maxProcs := 3
			if len(projectProcesses) < maxProcs {
				maxProcs = len(projectProcesses)
			}
			for i := 0; i < maxProcs; i++ {
				p := projectProcesses[i]
				var icon string
				var style lipgloss.Style
				switch p.State {
				case "running":
					icon = "●"
					style = StatusSuccess
				case "stopped":
					icon = "○"
					style = SubtitleStyle
				default:
					icon = "✗"
					style = StatusError
				}
				name := truncate(string(p.Component), innerWidth-8)
				lines = append(lines, style.Render(fmt.Sprintf("  %s %s", icon, name)))
			}
			if len(projectProcesses) > maxProcs {
				lines = append(lines, SubtitleStyle.Render(fmt.Sprintf("    +%d more", len(projectProcesses)-maxProcs)))
			}
		} else {
			lines = append(lines, labelStyle.Render("  ○ none"))
		}

		// ─── Recent Logs ───
		if m.state.Logs != nil && len(m.state.Logs.Lines) > 0 {
			lines = append(lines, "")
			lines = append(lines, sectionStyle.Padding(0, 2).Render("─ Recent Logs ─"))

			// Get last 3 logs for this project
			var recentLogs []core.LogLineVM
			for i := len(m.state.Logs.Lines) - 1; i >= 0 && len(recentLogs) < 3; i-- {
				log := m.state.Logs.Lines[i]
				// Filter by project if we have one
				if activeProject != nil {
					if strings.HasPrefix(log.Source, activeProject.ID) || strings.HasPrefix(log.Source, activeProject.Name) {
						recentLogs = append(recentLogs, log)
					}
				} else {
					recentLogs = append(recentLogs, log)
				}
			}

			// Show logs in chronological order (oldest first)
			for i := len(recentLogs) - 1; i >= 0; i-- {
				log := recentLogs[i]
				// Level icon
				levelIcon := "·"
				levelStyle := labelStyle
				switch log.Level {
				case "error":
					levelIcon = "✗"
					levelStyle = StatusError
				case "warn":
					levelIcon = "!"
					levelStyle = StatusWarning
				}
				// Extract component type from Source (format: "project/component")
				compType := ""
				if parts := strings.Split(log.Source, "/"); len(parts) >= 2 {
					compType = parts[len(parts)-1]
				} else {
					compType = log.Source
				}
				// Short component label
				compLabel := ""
				switch compType {
				case "backend":
					compLabel = "[BE]"
				case "frontend":
					compLabel = "[FE]"
				case "agent":
					compLabel = "[AG]"
				case "cli":
					compLabel = "[CL]"
				default:
					if compType != "" {
						compLabel = "[" + truncate(compType, 2) + "]"
					}
				}
				// Truncate message to fit (account for label width)
				labelWidth := len(compLabel) + 1 // +1 for space
				msg := truncate(log.Message, innerWidth-4-labelWidth)
				compLabelStyled := lipgloss.NewStyle().Foreground(ColorSecondary).Render(compLabel)
				lines = append(lines, levelStyle.Render("  "+levelIcon+" ")+compLabelStyled+" "+labelStyle.Render(msg))
			}
		}

	} else {
		// No active project
		lines = append(lines, "")
		noProjectStyle := lipgloss.NewStyle().Foreground(ColorMuted).Italic(true).Padding(0, 2)
		lines = append(lines, noProjectStyle.Render("No project selected"))
	}

	// Join content
	content := lipgloss.JoinVertical(lipgloss.Left, lines...)

	// Apply border style (always unfocused since context is informational)
	return UnfocusedBorderStyle.
		Width(width).
		Height(height).
		Render(content)
}

// getActiveProjectContext returns the currently active/selected project based on view
func (m *Model) getActiveProjectContext() *core.ProjectVM {
	switch m.currentView {
	case core.VMProjects:
		// Use TreeMenu selection
		if selectedItem := m.projectsMenu.SelectedItem(); selectedItem != nil {
			if proj, ok := selectedItem.Data.(core.ProjectVM); ok {
				return &proj
			}
			// If it's a component, find the parent project
			if _, ok := selectedItem.Data.(core.ComponentVM); ok {
				drillPath := m.projectsMenu.DrillDownPath()
				if len(drillPath) > 0 && m.state.Projects != nil {
					for i := range m.state.Projects.Projects {
						if m.state.Projects.Projects[i].ID == drillPath[0] {
							return &m.state.Projects.Projects[i]
						}
					}
				}
			}
		}
	case core.VMGit:
		// Use TreeMenu selection
		if selectedItem := m.gitMenu.SelectedItem(); selectedItem != nil {
			if g, ok := selectedItem.Data.(core.GitStatusVM); ok {
				// Find corresponding ProjectVM
				if m.state.Projects != nil {
					for i := range m.state.Projects.Projects {
						if m.state.Projects.Projects[i].ID == g.ProjectID {
							return &m.state.Projects.Projects[i]
						}
					}
				}
				// Return a minimal project from git info
				return &core.ProjectVM{
					ID:        g.ProjectID,
					Name:      g.ProjectName,
					GitBranch: g.Branch,
					GitDirty:  !g.IsClean,
					GitAhead:  g.Ahead,
					GitBehind: g.Behind,
				}
			}
		}
	case core.VMProcesses:
		// Use TreeMenu selection
		if selectedItem := m.processesMenu.SelectedItem(); selectedItem != nil {
			if proc, ok := selectedItem.Data.(core.ProcessVM); ok {
				if m.state.Projects != nil {
					for i := range m.state.Projects.Projects {
						if m.state.Projects.Projects[i].ID == proc.ProjectID {
							return &m.state.Projects.Projects[i]
						}
					}
				}
			}
		}
	case core.VMClaude:
		// First try to get project from TreeMenu selection
		if m.sessionsTreeMenu != nil {
			// Check if we're at a project level (drilled into a project)
			drillPath := m.sessionsTreeMenu.DrillDownPath()
			if len(drillPath) > 0 && m.state.Projects != nil {
				// drillPath[0] is the project ID
				for i := range m.state.Projects.Projects {
					if m.state.Projects.Projects[i].ID == drillPath[0] {
						return &m.state.Projects.Projects[i]
					}
				}
			}
			// Or check selected item's data
			if selectedItem := m.sessionsTreeMenu.SelectedItem(); selectedItem != nil {
				if sess, ok := selectedItem.Data.(core.ClaudeSessionVM); ok {
					if sess.ProjectID != "" && m.state.Projects != nil {
						for i := range m.state.Projects.Projects {
							if m.state.Projects.Projects[i].ID == sess.ProjectID {
								return &m.state.Projects.Projects[i]
							}
						}
					}
				}
			}
		}
		// Fallback to active session from state
		if m.state.Claude != nil && m.state.Claude.ActiveSession != nil {
			projectID := m.state.Claude.ActiveSession.ProjectID
			if projectID != "" && m.state.Projects != nil {
				for i := range m.state.Projects.Projects {
					if m.state.Projects.Projects[i].ID == projectID {
						return &m.state.Projects.Projects[i]
					}
				}
			}
		}
	case core.VMDashboard, core.VMBuild, core.VMLogs, core.VMConfig:
		// For these views, show first project or none
		if m.state.Projects != nil && len(m.state.Projects.Projects) > 0 {
			return &m.state.Projects.Projects[0]
		}
	}
	return nil
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
	case core.VMClaude:
		content = m.renderClaude(width, height)
	case core.VMCodex:
		content = m.renderCodex(width, height)
	case core.VMCockpit:
		content = m.renderCockpit(width, height)
	case core.VMDatabase:
		content = m.renderDatabase(width, height)
	case core.VMShell:
		content = m.renderShell(width, height)
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
	// If in command mode, show command prompt
	if m.commandMode {
		cmdPrompt := StatusWarning.Render(" ^G... ") + HelpDescStyle.Render(" q=quit d=detach ?=help ")
		return lipgloss.NewStyle().Width(m.width).Background(ColorBgAlt).Render(cmdPrompt)
	}

	// Context-sensitive shortcuts based on current view and focus
	var shortcuts []string

	// Navigation hints
	navHint := HelpKeyStyle.Render("↑↓") + HelpDescStyle.Render(" nav  ")
	tabHint := HelpKeyStyle.Render("Tab") + HelpDescStyle.Render(" focus  ")

	shortcuts = append(shortcuts, navHint, tabHint)

	// Sidebar-specific shortcuts
	if m.focusArea == FocusSidebar {
		shortcuts = append(shortcuts,
			HelpKeyStyle.Render("Enter")+HelpDescStyle.Render(" select  "),
			HelpKeyStyle.Render("D P B")+HelpDescStyle.Render(" views  "),
		)
	} else {
		// View-specific shortcuts (only when not on sidebar)
		switch m.currentView {
		case core.VMDashboard:
			shortcuts = append(shortcuts,
				HelpKeyStyle.Render("b")+HelpDescStyle.Render(" build  "),
				HelpKeyStyle.Render("r")+HelpDescStyle.Render(" run  "),
				HelpKeyStyle.Render("s")+HelpDescStyle.Render(" stop  "),
				HelpKeyStyle.Render("p")+HelpDescStyle.Render(" pause  "),
				HelpKeyStyle.Render("k")+HelpDescStyle.Render(" kill  "),
				HelpKeyStyle.Render("l")+HelpDescStyle.Render(" logs  "),
			)
			// Show AI shortcut if Claude is installed
			if m.state.Claude != nil && m.state.Claude.IsInstalled {
				shortcuts = append(shortcuts,
					HelpKeyStyle.Render("a")+HelpDescStyle.Render(" ai  "),
				)
			}
		case core.VMProjects:
			// TreeMenu navigation hints
			if m.focusArea == FocusMain {
				if !m.projectsMenu.IsAtRoot() {
					shortcuts = append(shortcuts,
						HelpKeyStyle.Render("←")+HelpDescStyle.Render(" back  "),
					)
				}
				selectedItem := m.projectsMenu.SelectedItem()
				if selectedItem != nil && len(selectedItem.Children) > 0 {
					shortcuts = append(shortcuts,
						HelpKeyStyle.Render("→/Enter")+HelpDescStyle.Render(" components  "),
					)
				}
			}
			// Action shortcuts
			shortcuts = append(shortcuts,
				HelpKeyStyle.Render("b")+HelpDescStyle.Render(" build  "),
				HelpKeyStyle.Render("r")+HelpDescStyle.Render(" run  "),
				HelpKeyStyle.Render("s")+HelpDescStyle.Render(" stop  "),
			)
		case core.VMBuild:
			// Profile shortcuts
			shortcuts = append(shortcuts,
				HelpKeyStyle.Render("d")+HelpDescStyle.Render(" dev  "),
				HelpKeyStyle.Render("t")+HelpDescStyle.Render(" test  "),
				HelpKeyStyle.Render("p")+HelpDescStyle.Render(" prod  "),
				HelpKeyStyle.Render("←→")+HelpDescStyle.Render(" cycle  "),
			)
			if m.state.Builds != nil && m.state.Builds.IsBuilding {
				shortcuts = append(shortcuts,
					HelpKeyStyle.Render("CTRL+c")+HelpDescStyle.Render(" cancel  "),
				)
			} else {
				shortcuts = append(shortcuts,
					HelpKeyStyle.Render("b")+HelpDescStyle.Render(" build  "),
					HelpKeyStyle.Render("CTRL+b")+HelpDescStyle.Render(" all  "),
				)
			}
		case core.VMProcesses:
			// TreeMenu navigation hints
			if m.focusArea == FocusMain {
				if !m.processesMenu.IsAtRoot() {
					shortcuts = append(shortcuts,
						HelpKeyStyle.Render("←")+HelpDescStyle.Render(" back  "),
					)
				}
				selectedItem := m.processesMenu.SelectedItem()
				if selectedItem != nil && len(selectedItem.Children) > 0 {
					shortcuts = append(shortcuts,
						HelpKeyStyle.Render("→/Enter")+HelpDescStyle.Render(" processes  "),
					)
				}
			}
			// Action shortcuts
			shortcuts = append(shortcuts,
				HelpKeyStyle.Render("r")+HelpDescStyle.Render(" run  "),
				HelpKeyStyle.Render("s")+HelpDescStyle.Render(" stop  "),
				HelpKeyStyle.Render("k")+HelpDescStyle.Render(" kill  "),
				HelpKeyStyle.Render("l")+HelpDescStyle.Render(" logs  "),
			)
		case core.VMLogs:
			// Show cancel if a build is running
			if m.state.Builds != nil && m.state.Builds.IsBuilding {
				shortcuts = append(shortcuts,
					HelpKeyStyle.Render("CTRL+c")+HelpDescStyle.Render(" cancel  "),
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
					HelpKeyStyle.Render("↑↓")+HelpDescStyle.Render(" scroll  "),
					HelpKeyStyle.Render("S-↑↓")+HelpDescStyle.Render(" page  "),
					HelpKeyStyle.Render("End")+HelpDescStyle.Render(" bottom  "),
					HelpKeyStyle.Render("Space")+HelpDescStyle.Render(" pause  "),
					HelpKeyStyle.Render("s")+HelpDescStyle.Render(" source  "),
					HelpKeyStyle.Render("t")+HelpDescStyle.Render(" type  "),
					HelpKeyStyle.Render("/")+HelpDescStyle.Render(" search  "),
				)
			}
		case core.VMGit:
			if m.focusArea == FocusDetail {
				// Focused on diff panel - show scroll hints
				shortcuts = append(shortcuts,
					HelpKeyStyle.Render("↑↓")+HelpDescStyle.Render(" scroll  "),
					HelpKeyStyle.Render("S-↑↓")+HelpDescStyle.Render(" page  "),
					HelpKeyStyle.Render("Esc")+HelpDescStyle.Render(" back  "),
				)
			} else if m.focusArea == FocusMain {
				// TreeMenu navigation hints
				if !m.gitMenu.IsAtRoot() {
					// Drilled down into a project - show files
					selectedItem := m.gitMenu.SelectedItem()
					if selectedItem != nil {
						if _, isFile := selectedItem.Data.(GitFileEntry); isFile {
							shortcuts = append(shortcuts,
								HelpKeyStyle.Render("←")+HelpDescStyle.Render(" back  "),
								HelpKeyStyle.Render("Enter")+HelpDescStyle.Render(" focus diff  "),
							)
						} else {
							shortcuts = append(shortcuts,
								HelpKeyStyle.Render("←")+HelpDescStyle.Render(" back  "),
							)
						}
					}
				} else {
					// At project level
					selectedItem := m.gitMenu.SelectedItem()
					if selectedItem != nil && len(selectedItem.Children) > 0 {
						shortcuts = append(shortcuts,
							HelpKeyStyle.Render("→/Enter")+HelpDescStyle.Render(" files  "),
						)
					}
				}
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
		case core.VMClaude:
			// Terminal mode has its own shortcuts
			if m.terminalMode {
				shortcuts = append(shortcuts,
					HelpKeyStyle.Render("^G Esc")+HelpDescStyle.Render(" exit  "),
					HelpKeyStyle.Render("PgUp/Dn")+HelpDescStyle.Render(" scroll  "),
				)
			} else {
				// Sessions panel shortcuts (right side)
				if m.focusArea == FocusDetail {
					allLabel := "all"
					if m.showAllClaudeSessions {
						allLabel = "less"
					}
					shortcuts = append(shortcuts,
						HelpKeyStyle.Render("n")+HelpDescStyle.Render(" new  "),
						HelpKeyStyle.Render("Enter")+HelpDescStyle.Render(" open  "),
						HelpKeyStyle.Render("r")+HelpDescStyle.Render(" rename  "),
						HelpKeyStyle.Render("x")+HelpDescStyle.Render(" delete  "),
						HelpKeyStyle.Render("a")+HelpDescStyle.Render(" "+allLabel+"  "),
					)
				} else if m.claudeInputActive {
					shortcuts = append(shortcuts,
						HelpKeyStyle.Render("Enter")+HelpDescStyle.Render(" send  "),
						HelpKeyStyle.Render("Esc")+HelpDescStyle.Render(" cancel  "),
					)
				} else {
					shortcuts = append(shortcuts,
						HelpKeyStyle.Render("i")+HelpDescStyle.Render(" input  "),
						HelpKeyStyle.Render("Esc")+HelpDescStyle.Render(" back  "),
					)
				}
				if m.state.Claude != nil && m.state.Claude.IsProcessing {
					shortcuts = append(shortcuts,
						HelpKeyStyle.Render("CTRL+c")+HelpDescStyle.Render(" stop  "),
					)
				}
			}
		case core.VMCockpit:
			// Cockpit view shortcuts
			if m.cockpitConfigMode {
				shortcuts = append(shortcuts,
					HelpKeyStyle.Render("↑↓")+HelpDescStyle.Render(" select  "),
					HelpKeyStyle.Render("Enter")+HelpDescStyle.Render(" confirm  "),
					HelpKeyStyle.Render("Esc")+HelpDescStyle.Render(" back  "),
				)
			} else {
				shortcuts = append(shortcuts,
					HelpKeyStyle.Render("↑↓←→")+HelpDescStyle.Render(" navigate  "),
					HelpKeyStyle.Render("1-9")+HelpDescStyle.Render(" profile  "),
					HelpKeyStyle.Render("c")+HelpDescStyle.Render(" config  "),
					HelpKeyStyle.Render("n")+HelpDescStyle.Render(" new  "),
					HelpKeyStyle.Render("r")+HelpDescStyle.Render(" rename  "),
					HelpKeyStyle.Render("x")+HelpDescStyle.Render(" delete  "),
				)
			}
		case core.VMDatabase:
			// Database view shortcuts
			if m.terminalMode {
				shortcuts = append(shortcuts,
					HelpKeyStyle.Render("^G Esc")+HelpDescStyle.Render(" exit  "),
					HelpKeyStyle.Render("PgUp/Dn")+HelpDescStyle.Render(" scroll  "),
				)
			} else if m.focusArea == FocusDetail {
				shortcuts = append(shortcuts,
					HelpKeyStyle.Render("n")+HelpDescStyle.Render(" new  "),
					HelpKeyStyle.Render("Enter")+HelpDescStyle.Render(" connect  "),
					HelpKeyStyle.Render("r")+HelpDescStyle.Render(" rename  "),
					HelpKeyStyle.Render("x")+HelpDescStyle.Render(" delete  "),
					HelpKeyStyle.Render("s")+HelpDescStyle.Render(" stop  "),
				)
			} else {
				shortcuts = append(shortcuts,
					HelpKeyStyle.Render("Enter")+HelpDescStyle.Render(" terminal  "),
					HelpKeyStyle.Render("Tab")+HelpDescStyle.Render(" databases  "),
				)
			}
		}
	}

	// Show command mode hint (^G for commands)
	// In daemon mode, also hint at detach command
	if m.detachable {
		shortcuts = append(shortcuts,
			HelpKeyStyle.Render("^G d")+HelpDescStyle.Render(" detach  "),
		)
	}

	// Always show help and quit
	shortcuts = append(shortcuts,
		HelpKeyStyle.Render("?")+HelpDescStyle.Render(" help  "),
		HelpKeyStyle.Render("^G q")+HelpDescStyle.Render(" quit"),
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

	// Stats row (compact) - divide width by 4 for each box, accounting for gaps
	numStats := 4
	totalGaps := (numStats - 1) * GapHorizontal
	statBoxWidth := (width - totalGaps) / numStats
	gap := strings.Repeat(" ", GapHorizontal)
	stats := lipgloss.JoinHorizontal(
		lipgloss.Top,
		m.renderStatBox("Projects", fmt.Sprintf("%d", vm.ProjectCount), ColorSecondary, statBoxWidth),
		gap,
		m.renderStatBox("Running", fmt.Sprintf("%d", vm.RunningCount), ColorSuccess, statBoxWidth),
		gap,
		m.renderStatBox("Building", fmt.Sprintf("%d", vm.BuildingCount), ColorWarning, statBoxWidth),
		gap,
		m.renderStatBox("Errors", fmt.Sprintf("%d", vm.ErrorCount), ColorError, statBoxWidth),
	)

	// Calculate panel sizes
	// Left: Projects + Processes + Git stacked (narrow, 1/3)
	// Right: Logs (wide, 2/3)
	panelHeight := height - 8
	leftWidth := width / 3
	if leftWidth < 30 {
		leftWidth = 30
	}
	rightWidth := width - leftWidth - GapHorizontal

	// Left: 3 panels, distribute height with remainder to last panel
	thirdHeight := panelHeight / 3
	lastPanelHeight := panelHeight - (thirdHeight * 2)

	projectsPanel := m.renderProjectsList(vm.Projects, leftWidth, thirdHeight, m.focusArea == FocusMain)
	processesPanel := m.renderProcessesList(vm.RunningProcesses, leftWidth, thirdHeight, false)
	gitPanel := m.renderMiniGit(vm.GitSummary, leftWidth, lastPanelHeight)

	// Stack left panels
	leftPane := lipgloss.JoinVertical(lipgloss.Left, projectsPanel, processesPanel, gitPanel)

	// Right: Logs - use full panelHeight to align with left pane
	logsPanel := m.renderMiniLogs(rightWidth-5, panelHeight)

	// Combine with horizontal gap
	panels := lipgloss.JoinHorizontal(lipgloss.Top,
		leftPane,
		gap,
		logsPanel,
	)

	return lipgloss.JoinVertical(lipgloss.Left,
		stats,
		panels,
	)
}

// renderMiniGit renders a compact git status panel for dashboard
func (m *Model) renderMiniGit(gitSummary []core.GitStatusVM, width, height int) string {
	header := SubtitleStyle.Render("─ Git Changes ─")

	// Show loading state if git is loading in background
	if m.state.GitLoading {
		loadingMsg := lipgloss.NewStyle().Foreground(ColorWarning).Render(
			"  " + m.spinner.View() + " Loading git status...",
		)
		return UnfocusedBorderStyle.Width(width).Height(height).Render(
			lipgloss.JoinVertical(lipgloss.Left, header, loadingMsg),
		)
	}

	var rows []string
	for _, g := range gitSummary {
		if g.IsClean {
			continue // Skip clean repos
		}

		// Count changes
		changes := len(g.Modified) + len(g.Untracked) + len(g.Staged) + len(g.Deleted)
		if changes == 0 {
			continue
		}

		// Format: projectName M:x U:y S:z
		var parts []string
		if len(g.Modified) > 0 {
			parts = append(parts, StatusWarning.Render(fmt.Sprintf("M:%d", len(g.Modified))))
		}
		if len(g.Untracked) > 0 {
			parts = append(parts, SubtitleStyle.Render(fmt.Sprintf("?:%d", len(g.Untracked))))
		}
		if len(g.Staged) > 0 {
			parts = append(parts, StatusSuccess.Render(fmt.Sprintf("S:%d", len(g.Staged))))
		}
		if len(g.Deleted) > 0 {
			parts = append(parts, StatusError.Render(fmt.Sprintf("D:%d", len(g.Deleted))))
		}

		row := fmt.Sprintf("  %-12s %s", truncate(g.ProjectName, 12), strings.Join(parts, " "))
		rows = append(rows, row)

		if len(rows) >= height-3 {
			rows = append(rows, SubtitleStyle.Render(fmt.Sprintf("  ... and %d more", len(gitSummary)-len(rows))))
			break
		}
	}

	if len(rows) == 0 {
		rows = append(rows, SubtitleStyle.Render("  All clean ✓"))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, rows...)

	return UnfocusedBorderStyle.Width(width).Height(height).Render(
		lipgloss.JoinVertical(lipgloss.Left, header, content),
	)
}

// renderMiniLogs renders a compact logs panel for dashboard (run logs only)
func (m *Model) renderMiniLogs(width, height int) string {
	header := SubtitleStyle.Render("─ Run Logs ─")

	// Inner width accounting for border (2 chars total)
	innerWidth := width - 2

	var lines []string
	if m.state.Logs != nil && len(m.state.Logs.Lines) > 0 {
		maxLines := height - 4 // header + border

		// Filter to only show run logs (not build logs)
		// Build logs have source like "build:project/component"
		var runLogs []core.LogLineVM
		for _, line := range m.state.Logs.Lines {
			if !strings.HasPrefix(line.Source, "build:") {
				runLogs = append(runLogs, line)
			}
		}

		// Show last N lines that fit
		start := len(runLogs) - maxLines
		if start < 0 {
			start = 0
		}

		// Calculate max source width from visible logs
		maxSourceLen := 12 // minimum width
		for _, line := range runLogs[start:] {
			if len(line.Source) > maxSourceLen {
				maxSourceLen = len(line.Source)
			}
		}
		// Cap at reasonable max
		if maxSourceLen > 20 {
			maxSourceLen = 20
		}

		for _, line := range runLogs[start:] {
			// Compact format: [source] message - show full source name
			var levelStyle lipgloss.Style
			switch line.Level {
			case "error":
				levelStyle = LogErrorStyle
			case "warn":
				levelStyle = LogWarnStyle
			default:
				levelStyle = LogInfoStyle
			}
			sourceWidth := maxSourceLen + 2 // for brackets
			msgWidth := innerWidth - sourceWidth - 2
			if msgWidth < 20 {
				msgWidth = 20
			}
			logLine := fmt.Sprintf("%s %s",
				LogSourceStyle.Render(fmt.Sprintf("[%-*s]", maxSourceLen, line.Source)),
				levelStyle.Render(truncate(line.Message, msgWidth)))
			lines = append(lines, logLine)
		}
	}

	if len(lines) == 0 {
		lines = append(lines, SubtitleStyle.Render("  No recent logs"))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)

	return UnfocusedBorderStyle.Width(width).Height(height).Render(
		lipgloss.JoinVertical(lipgloss.Left, header, content),
	)
}

// renderStatBox renders a stat box
func (m *Model) renderStatBox(label, value string, color lipgloss.Color, boxWidth int) string {
	// Account for border (2) and padding (4) and margin (2)
	contentWidth := boxWidth - 8
	if contentWidth < 5 {
		contentWidth = 5
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(color).
		Padding(0, 2).
		Width(contentWidth).
		Render(
			lipgloss.JoinVertical(lipgloss.Center,
				lipgloss.NewStyle().Foreground(color).Bold(true).Width(contentWidth).Align(lipgloss.Center).Render(value),
				SubtitleStyle.Width(contentWidth).Align(lipgloss.Center).Render(label),
			),
		)
}

// renderProjectsList renders a list of projects
func (m *Model) renderProjectsList(projects []core.ProjectVM, width, height int, focused bool) string {
	header := SubtitleStyle.Render("─ Projects ─")

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

		// Check if path exists
		pathWarning := ""
		if _, err := os.Stat(p.Path); os.IsNotExist(err) {
			pathWarning = StatusError.Render(" " + IconWarning)
		}

		row := fmt.Sprintf("%s %s%s%s", status, truncate(p.Name, width-10), pathWarning, git)

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
		lipgloss.JoinVertical(lipgloss.Left, header, content),
	)
}

// renderProcessesList renders a list of processes
func (m *Model) renderProcessesList(processes []core.ProcessVM, width, height int, focused bool) string {
	header := SubtitleStyle.Render("─ Processes ─")

	var rows []string
	for i, p := range processes {
		if i >= height-3 {
			rows = append(rows, SubtitleStyle.Render(fmt.Sprintf("  ... and %d more", len(processes)-i)))
			break
		}

		// Show paused state
		var stateIcon string
		if p.State == "paused" {
			stateIcon = StatusWarning.Render("⏸")
		} else {
			stateIcon = StatusRunning.Render(IconRunning)
		}
		row := fmt.Sprintf("%s %s/%s", stateIcon, truncate(p.ProjectName, 12), p.Component)
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
		lipgloss.JoinVertical(lipgloss.Left, header, content),
	)
}

// ProjectComponentRow represents a row in the projects view (one per component)
type ProjectComponentRow struct {
	ProjectName string
	ProjectPath string
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

	// Left panel - TreeMenu with projects and components
	listWidth := m.projectsMenu.CalcWidth()
	if listWidth < 30 {
		listWidth = 30
	}
	if listWidth > width/2 {
		listWidth = width / 2
	}

	// Configure and render TreeMenu
	m.projectsMenu.SetSize(listWidth, height-4)
	m.projectsMenu.SetFocused(m.focusArea == FocusMain)
	listPanel := m.projectsMenu.Render()

	// Right panel - project/component details
	detailWidth := width - listWidth - GapHorizontal
	detailHeight := height - 6
	var detailContent string

	// Get selected item from TreeMenu
	selectedItem := m.projectsMenu.SelectedItem()
	if selectedItem != nil {
		// Check if it's a component or project
		if comp, ok := selectedItem.Data.(core.ComponentVM); ok {
			// Show component details
			detailLines := []string{
				PanelTitleStyle.Render(string(comp.Type)),
				"",
			}

			// Status
			if comp.IsRunning {
				detailLines = append(detailLines, StatusSuccess.Render("● Running"))
				if comp.PID > 0 {
					detailLines = append(detailLines, fmt.Sprintf("PID: %d", comp.PID))
				}
				if comp.Uptime != "" {
					detailLines = append(detailLines, fmt.Sprintf("Uptime: %s", comp.Uptime))
				}
			} else {
				detailLines = append(detailLines, StatusError.Render("○ Stopped"))
			}

			detailLines = append(detailLines, "")

			// Path
			if comp.Path != "" {
				detailLines = append(detailLines, fmt.Sprintf("Path: %s", comp.Path))
			}
			if comp.Binary != "" {
				detailLines = append(detailLines, fmt.Sprintf("Binary: %s", comp.Binary))
			}
			if comp.Port > 0 {
				detailLines = append(detailLines, fmt.Sprintf("Port: %d", comp.Port))
			}

			detailLines = append(detailLines, "")
			detailLines = append(detailLines, SubtitleStyle.Render("Actions:"))
			if comp.IsRunning {
				detailLines = append(detailLines, HelpKeyStyle.Render("s")+" stop  "+HelpKeyStyle.Render("r")+" restart")
			} else {
				detailLines = append(detailLines, HelpKeyStyle.Render("r")+" run  "+HelpKeyStyle.Render("b")+" build")
			}

			detailContent = strings.Join(detailLines, "\n")
		} else if project, ok := selectedItem.Data.(core.ProjectVM); ok {
			// Show project details
			detailLines := []string{
				PanelTitleStyle.Render(project.Name),
				"",
			}

			// Self indicator
			if project.IsSelf {
				detailLines = append(detailLines, lipgloss.NewStyle().Foreground(ColorSecondary).Render("* This is csd-devtrack"))
				detailLines = append(detailLines, "")
			}

			// Path
			detailLines = append(detailLines, fmt.Sprintf("Path: %s", project.Path))

			// Check if path exists
			if _, err := os.Stat(project.Path); os.IsNotExist(err) {
				detailLines = append(detailLines, StatusError.Render("⚠ Path does not exist"))
			}

			detailLines = append(detailLines, "")

			// Git info
			if project.GitBranch != "" {
				branchLine := fmt.Sprintf("Branch: %s", GitBranchStyle.Render(project.GitBranch))
				if project.GitDirty {
					branchLine += GitDirtyStyle.Render(" *")
				}
				detailLines = append(detailLines, branchLine)
				if project.GitAhead > 0 {
					detailLines = append(detailLines, GitAheadStyle.Render(fmt.Sprintf("  ↑%d ahead", project.GitAhead)))
				}
				if project.GitBehind > 0 {
					detailLines = append(detailLines, GitBehindStyle.Render(fmt.Sprintf("  ↓%d behind", project.GitBehind)))
				}
			} else if m.state.GitLoading {
				detailLines = append(detailLines, lipgloss.NewStyle().Foreground(ColorMuted).Render(m.spinner.View()+" Loading git..."))
			}

			detailLines = append(detailLines, "")

			// Components summary
			runningCount := 0
			for _, comp := range project.Components {
				if comp.IsRunning {
					runningCount++
				}
			}
			detailLines = append(detailLines, fmt.Sprintf("Components: %d (%d running)", len(project.Components), runningCount))

			if len(project.Components) > 0 {
				detailLines = append(detailLines, "")
				detailLines = append(detailLines, SubtitleStyle.Render("Press → or Enter to see components"))
			}

			detailContent = strings.Join(detailLines, "\n")
		}
	} else if len(vm.Projects) == 0 {
		detailContent = SubtitleStyle.Render("No projects configured")
	}

	// Build detail panel
	var detailStyle lipgloss.Style
	if m.focusArea == FocusDetail {
		detailStyle = FocusedBorderStyle
	} else {
		detailStyle = UnfocusedBorderStyle
	}
	detailPanel := detailStyle.Width(detailWidth - 5).Height(detailHeight).Render(detailContent)

	gap := strings.Repeat(" ", GapHorizontal)
	return lipgloss.JoinHorizontal(lipgloss.Top, listPanel, gap, detailPanel)
}

// renderBuild renders the build view
func (m *Model) renderBuild(width, height int) string {
	vm := m.state.Builds
	if vm == nil {
		return m.renderLoading()
	}

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

	return style.Width(width - 3).Height(height - 2).Render(
		lipgloss.JoinVertical(lipgloss.Left,
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

	// Left panel - TreeMenu with projects and processes
	listWidth := m.processesMenu.CalcWidth()
	if listWidth < 30 {
		listWidth = 30
	}
	if listWidth > width/2 {
		listWidth = width / 2
	}

	// Configure and render TreeMenu
	m.processesMenu.SetSize(listWidth, height-4)
	m.processesMenu.SetFocused(m.focusArea == FocusMain)
	listPanel := m.processesMenu.Render()

	// Right panel - process details
	detailWidth := width - listWidth - GapHorizontal
	detailHeight := height - 6
	var detailContent string

	// Get selected item from TreeMenu
	selectedItem := m.processesMenu.SelectedItem()
	if selectedItem != nil {
		if proc, ok := selectedItem.Data.(core.ProcessVM); ok {
			// Show process details
			detailLines := []string{
				PanelTitleStyle.Render(fmt.Sprintf("%s/%s", proc.ProjectName, proc.Component)),
				"",
			}

			// Self indicator
			if proc.IsSelf {
				detailLines = append(detailLines, lipgloss.NewStyle().Foreground(ColorSecondary).Render("* This is csd-devtrack"))
				detailLines = append(detailLines, "")
			}

			// State
			stateIcon := StatusIcon(string(proc.State))
			if proc.State == "running" {
				detailLines = append(detailLines, StatusSuccess.Render(stateIcon+" Running"))
			} else if proc.State == "stopped" {
				detailLines = append(detailLines, StatusError.Render(stateIcon+" Stopped"))
			} else {
				detailLines = append(detailLines, fmt.Sprintf("%s %s", stateIcon, proc.State))
			}

			detailLines = append(detailLines, "")

			// Process info
			if proc.PID > 0 {
				detailLines = append(detailLines, fmt.Sprintf("PID: %d", proc.PID))
			}
			if proc.Uptime != "" {
				detailLines = append(detailLines, fmt.Sprintf("Uptime: %s", proc.Uptime))
			}
			if proc.Restarts > 0 {
				detailLines = append(detailLines, fmt.Sprintf("Restarts: %d", proc.Restarts))
			}

			if proc.LastError != "" {
				detailLines = append(detailLines, "")
				detailLines = append(detailLines, StatusError.Render("Last error:"))
				detailLines = append(detailLines, truncate(proc.LastError, detailWidth-10))
			}

			detailLines = append(detailLines, "")
			detailLines = append(detailLines, SubtitleStyle.Render("Actions:"))
			if proc.State == "running" {
				detailLines = append(detailLines, HelpKeyStyle.Render("s")+" stop  "+HelpKeyStyle.Render("r")+" restart  "+HelpKeyStyle.Render("l")+" logs")
			} else {
				detailLines = append(detailLines, HelpKeyStyle.Render("r")+" run  "+HelpKeyStyle.Render("l")+" logs")
			}

			detailContent = strings.Join(detailLines, "\n")
		} else {
			// Selected a project group - show summary
			drillPath := m.processesMenu.DrillDownPath()
			if len(drillPath) == 0 && selectedItem.Label != "" {
				// At project level
				detailLines := []string{
					PanelTitleStyle.Render(selectedItem.Label),
					"",
					fmt.Sprintf("Processes: %d", selectedItem.Count),
					"",
					SubtitleStyle.Render("Press → or Enter to see processes"),
				}
				detailContent = strings.Join(detailLines, "\n")
			}
		}
	} else if len(vm.Processes) == 0 {
		detailContent = SubtitleStyle.Render("No processes running")
	}

	// Build detail panel
	var detailStyle lipgloss.Style
	if m.focusArea == FocusDetail {
		detailStyle = FocusedBorderStyle
	} else {
		detailStyle = UnfocusedBorderStyle
	}
	detailPanel := detailStyle.Width(detailWidth - 5).Height(detailHeight).Render(detailContent)

	gap := strings.Repeat(" ", GapHorizontal)
	return lipgloss.JoinHorizontal(lipgloss.Top, listPanel, gap, detailPanel)
}

// renderLogs renders the logs view with filtering
func (m *Model) renderLogs(width, height int) string {
	vm := m.state.Logs
	if vm == nil {
		return m.renderLoading()
	}

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

	// Display log lines with scroll support
	var logLines []string
	maxLines := height - 10 // Account for 2 filter rows + stats line
	if maxLines < 1 {
		maxLines = 1
	}

	// Calculate scroll position
	totalLines := len(filteredLines)

	// Clamp scroll offset
	maxOffset := totalLines - maxLines
	if maxOffset < 0 {
		maxOffset = 0
	}
	if m.logScrollOffset > maxOffset {
		m.logScrollOffset = maxOffset
	}

	// Calculate start position (from the end, offset by scroll)
	start := totalLines - maxLines - m.logScrollOffset
	if start < 0 {
		start = 0
	}
	end := start + maxLines
	if end > totalLines {
		end = totalLines
	}

	for _, line := range filteredLines[start:end] {
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

	// Stats line with scroll info
	var scrollInfo string
	if m.logPaused {
		scrollInfo = " │ " + StatusWarning.Render("⏸ PAUSED") + SubtitleStyle.Render(" (Space to resume)")
	} else if m.logScrollOffset > 0 {
		scrollInfo = fmt.Sprintf(" │ ↑%d lines (End to resume)", m.logScrollOffset)
	} else if m.logAutoScroll {
		scrollInfo = " │ Auto-scroll"
	}
	statsLine := SubtitleStyle.Render(fmt.Sprintf(
		"Lines %d-%d of %d",
		start+1, end, totalLines)) + scrollInfo

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

	return style.Width(width - 3).Height(height - 2).Render(
		lipgloss.JoinVertical(lipgloss.Left, filterBar1, filterBar2, statsLine, "", content),
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

// renderGit renders the git view using TreeMenu
func (m *Model) renderGit(width, height int) string {
	vm := m.state.Git
	if vm == nil {
		return m.renderLoading()
	}

	// Left panel - TreeMenu with projects and files
	listWidth := m.gitMenu.CalcWidth()
	if listWidth < 35 {
		listWidth = 35
	}
	if listWidth > width/2 {
		listWidth = width / 2
	}

	// Configure and render TreeMenu
	m.gitMenu.SetSize(listWidth, height-4)
	m.gitMenu.SetFocused(m.focusArea == FocusMain)
	listPanel := m.gitMenu.Render()

	// Right panel - diff/file preview
	detailWidth := width - listWidth - GapHorizontal
	detailHeight := height - 6
	var detailContent string

	// Get selected item from TreeMenu
	selectedItem := m.gitMenu.SelectedItem()
	if selectedItem != nil {
		// Check if it's a file (has GitFileEntry data) or project (has GitStatusVM data)
		if fileEntry, ok := selectedItem.Data.(GitFileEntry); ok {
			// Show diff content for file
			if m.gitDiffLoading {
				detailContent = lipgloss.NewStyle().Foreground(ColorWarning).Render(
					m.spinner.View() + " Loading diff...")
			} else if len(m.gitDiffContent) > 0 {
				// Render diff with scrolling
				contentHeight := detailHeight - 2
				m.visibleDetailRows = contentHeight

				// Calculate scroll bounds
				maxScroll := len(m.gitDiffContent) - m.visibleDetailRows
				if maxScroll < 0 {
					maxScroll = 0
				}
				if m.detailScrollOffset > maxScroll {
					m.detailScrollOffset = maxScroll
				}

				endIdx := m.detailScrollOffset + m.visibleDetailRows
				if endIdx > len(m.gitDiffContent) {
					endIdx = len(m.gitDiffContent)
				}

				var lines []string
				// Header with file path and status
				header := PanelTitleStyle.Render(fileEntry.Path) + " " +
					SubtitleStyle.Render("("+fileEntry.Status+")")
				lines = append(lines, header)

				for i := m.detailScrollOffset; i < endIdx; i++ {
					line := m.gitDiffContent[i]
					// Color diff lines
					if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
						line = lipgloss.NewStyle().Background(lipgloss.Color("#002200")).Foreground(lipgloss.Color("#00ff00")).Render(line)
					} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
						line = lipgloss.NewStyle().Background(lipgloss.Color("#220000")).Foreground(lipgloss.Color("#ff0000")).Render(line)
					} else if strings.HasPrefix(line, "@@") {
						line = lipgloss.NewStyle().Foreground(lipgloss.Color("#00ffff")).Render(line)
					} else if strings.HasPrefix(line, "diff ") || strings.HasPrefix(line, "index ") {
						line = lipgloss.NewStyle().Foreground(lipgloss.Color("#888888")).Render(line)
					}
					lines = append(lines, truncate(line, detailWidth-8))
				}

				// Scroll indicator
				if len(m.gitDiffContent) > m.visibleDetailRows {
					scrollInfo := SubtitleStyle.Render(fmt.Sprintf(
						" [%d-%d/%d lines]", m.detailScrollOffset+1, endIdx, len(m.gitDiffContent)))
					lines = append(lines, scrollInfo)
				}

				detailContent = strings.Join(lines, "\n")
			} else {
				// No diff content yet
				detailLines := []string{
					PanelTitleStyle.Render(fileEntry.Path),
					fmt.Sprintf("Status: %s", fileEntry.Status),
				}
				detailContent = strings.Join(detailLines, "\n")
			}
		} else if project, ok := selectedItem.Data.(core.GitStatusVM); ok {
			// Show project info
			branchDisplay := project.Branch
			if branchDisplay == "" && m.state.GitLoading {
				branchDisplay = m.spinner.View() + " loading..."
			} else if branchDisplay == "" {
				branchDisplay = "(unknown)"
			}

			var syncInfo string
			if project.Ahead > 0 {
				syncInfo += GitAheadStyle.Render(fmt.Sprintf("↑%d ahead", project.Ahead))
			}
			if project.Behind > 0 {
				if syncInfo != "" {
					syncInfo += " "
				}
				syncInfo += GitBehindStyle.Render(fmt.Sprintf("↓%d behind", project.Behind))
			}

			detailLines := []string{
				PanelTitleStyle.Render(project.ProjectName),
				fmt.Sprintf("Branch: %s", GitBranchStyle.Render(branchDisplay)),
			}
			if syncInfo != "" {
				detailLines = append(detailLines, syncInfo)
			}
			detailLines = append(detailLines, "")

			changeCount := len(project.Staged) + len(project.Modified) +
				len(project.Deleted) + len(project.Untracked)
			if changeCount == 0 {
				detailLines = append(detailLines, StatusSuccess.Render("✓ Working tree clean"))
			} else {
				detailLines = append(detailLines, fmt.Sprintf("%d changes", changeCount))
				if len(project.Staged) > 0 {
					detailLines = append(detailLines, StatusSuccess.Render(fmt.Sprintf("  %d staged", len(project.Staged))))
				}
				if len(project.Modified) > 0 {
					detailLines = append(detailLines, StatusWarning.Render(fmt.Sprintf("  %d modified", len(project.Modified))))
				}
				if len(project.Deleted) > 0 {
					detailLines = append(detailLines, StatusError.Render(fmt.Sprintf("  %d deleted", len(project.Deleted))))
				}
				if len(project.Untracked) > 0 {
					detailLines = append(detailLines, SubtitleStyle.Render(fmt.Sprintf("  %d untracked", len(project.Untracked))))
				}
				detailLines = append(detailLines, "")
				detailLines = append(detailLines, SubtitleStyle.Render("Press → or Enter to see files"))
			}
			detailContent = strings.Join(detailLines, "\n")
		}
	} else if m.state.GitLoading {
		detailContent = lipgloss.NewStyle().Foreground(ColorWarning).Render(
			m.spinner.View() + " Loading git status...")
	} else if len(vm.Projects) == 0 {
		detailContent = SubtitleStyle.Render("No git repositories")
	}

	// Build detail panel
	var detailStyle lipgloss.Style
	if m.focusArea == FocusDetail {
		detailStyle = FocusedBorderStyle
	} else {
		detailStyle = UnfocusedBorderStyle
	}
	detailPanel := detailStyle.Width(detailWidth - 5).Height(detailHeight).Render(detailContent)

	gap := strings.Repeat(" ", GapHorizontal)
	return lipgloss.JoinHorizontal(lipgloss.Top, listPanel, gap, detailPanel)
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

	return style.Width(width - 3).Height(height - 2).Render(
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

		// Check if path exists
		pathWarning := ""
		if _, err := os.Stat(proj.Path); os.IsNotExist(err) {
			pathWarning = StatusError.Render(" " + IconWarning)
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
		row := fmt.Sprintf("%s%-20s%s │ %s", indicator, truncate(proj.Name, 20), pathWarning, compsText)

		if isSelected {
			row = TableRowSelectedStyle.Width(width - 6).Render(row)
		}
		rows = append(rows, row)
	}

	// Show selected project details
	var details string
	if m.mainIndex >= 0 && m.mainIndex < len(cfg.Projects) {
		proj := cfg.Projects[m.mainIndex]

		// Check if path exists
		pathLine := fmt.Sprintf("Path: %s", proj.Path)
		if _, err := os.Stat(proj.Path); os.IsNotExist(err) {
			pathLine = StatusError.Render(fmt.Sprintf("%s Path: %s (not found)", IconWarning, proj.Path))
		}

		details = "\n" + lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(ColorMuted).
			Padding(0, 1).
			Render(
				lipgloss.JoinVertical(lipgloss.Left,
					PanelTitleStyle.Render(proj.Name),
					pathLine,
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
		rows = append(rows, style.Width(width-6).Render(row))
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
	// Input dialogs have different rendering
	if m.dialogInputActive {
		return m.renderInputDialogOverlay(width, height)
	}

	yesStyle := ButtonStyle
	noStyle := ButtonStyle
	if m.dialogConfirm {
		yesStyle = ButtonActiveStyle
	} else {
		noStyle = ButtonActiveStyle
	}

	// Calculate dialog width based on message length
	dialogWidth := len(m.dialogMessage) + 6
	if dialogWidth < 30 {
		dialogWidth = 30
	}

	// Style for content inside dialog (inherits background)
	contentStyle := lipgloss.NewStyle().
		Background(ColorBgAlt).
		Foreground(ColorText).
		Width(dialogWidth).
		Align(lipgloss.Center)

	// Button separator with background
	buttonSep := lipgloss.NewStyle().
		Background(ColorBgAlt).
		Render("  ")

	dialog := DialogStyle.Width(dialogWidth + 4).Render(
		lipgloss.JoinVertical(lipgloss.Center,
			contentStyle.Render(DialogTitleStyle.Render("Confirm")),
			contentStyle.Render(""),
			contentStyle.Render(m.dialogMessage),
			contentStyle.Render(""),
			contentStyle.Render(
				lipgloss.JoinHorizontal(lipgloss.Center,
					yesStyle.Render(" Yes "),
					buttonSep,
					noStyle.Render(" No "),
				),
			),
		),
	)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, dialog)
}

// renderInputDialogOverlay renders a dialog with text input
func (m *Model) renderInputDialogOverlay(width, height int) string {
	dialogWidth := 50

	// Style for content inside dialog
	contentStyle := lipgloss.NewStyle().
		Background(ColorBgAlt).
		Foreground(ColorText).
		Width(dialogWidth).
		Align(lipgloss.Center)

	// Input style
	inputStyle := lipgloss.NewStyle().
		Background(ColorBg).
		Foreground(ColorText).
		Padding(0, 1).
		Width(dialogWidth - 4)

	// Hint style
	hintStyle := lipgloss.NewStyle().
		Background(ColorBgAlt).
		Foreground(ColorMuted).
		Width(dialogWidth).
		Align(lipgloss.Center)

	dialog := DialogStyle.Width(dialogWidth + 4).Render(
		lipgloss.JoinVertical(lipgloss.Center,
			contentStyle.Render(DialogTitleStyle.Render("New Session")),
			contentStyle.Render(""),
			contentStyle.Render(m.dialogMessage),
			contentStyle.Render(""),
			contentStyle.Render(inputStyle.Render(m.dialogInput.View())),
			contentStyle.Render(""),
			hintStyle.Render("Enter to confirm, Esc to cancel"),
		),
	)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, dialog)
}

// renderHelpOverlay renders the help overlay in 2 columns
func (m *Model) renderHelpOverlay(background string, width, height int) string {
	// Style for content with background
	bgStyle := lipgloss.NewStyle().
		Background(ColorBgAlt).
		Foreground(ColorText)

	// Left column content
	leftCol := []string{
		HelpKeyStyle.Render("Navigation"),
		"  ↑/↓        Navigate items",
		"  Tab        Switch focus between panels",
		"  D P B O    Dashboard, Projects, Build, PrOcesses",
		"  L G C      Logs, Git, Config",
		"  PgUp/Dn    Page scroll",
		"  Esc        Back / Cancel",
		"",
		HelpKeyStyle.Render("Actions"),
		"  b          Build selected component",
		"  r          Run/Start component",
		"  s          Stop component",
		"  p          Pause/Resume (SIGSTOP/SIGCONT)",
		"  k          Kill (force stop)",
		"  l          View logs for component",
		"",
		HelpKeyStyle.Render("Build"),
		"  Ctrl+B     Build all projects",
		"  Ctrl+C     Cancel current build",
	}

	// Right column content
	rightCol := []string{
		HelpKeyStyle.Render("Logs"),
		"  ↑/↓ j/k    Scroll up/down one line",
		"  S-↑/↓      Page up/down",
		"  Home/End   Go to top/bottom",
		"  Space      Pause/Resume log display",
		"  s/←→       Cycle source filter",
		"  t          Cycle type (all/build/run)",
		"  e w i a    Filter: error/warn/info/all",
		"  /          Search, Esc to exit",
		"  c          Clear all filters",
		"",
		HelpKeyStyle.Render("Git"),
		"  Enter      Show files / Show diff",
		"  Esc        Back to project list",
		"",
		HelpKeyStyle.Render("Config"),
		"  ←→         Switch tabs",
		"  a          Add project (in browser)",
		"  x          Remove project",
	}

	// Pad columns to same height
	for len(leftCol) < len(rightCol) {
		leftCol = append(leftCol, "")
	}
	for len(rightCol) < len(leftCol) {
		rightCol = append(rightCol, "")
	}

	// Column widths
	colWidth := 54
	totalWidth := colWidth*2 + 3 // 2 columns + separator

	// Build left and right column strings with background
	leftContent := bgStyle.Width(colWidth).Render(strings.Join(leftCol, "\n"))
	rightContent := bgStyle.Width(colWidth).Render(strings.Join(rightCol, "\n"))

	// Join columns horizontally with separator
	columns := lipgloss.JoinHorizontal(lipgloss.Top,
		leftContent,
		bgStyle.Foreground(ColorBorder).Render(" │ "),
		rightContent,
	)

	// Footer with background
	footerLine := lipgloss.JoinHorizontal(lipgloss.Left,
		HelpKeyStyle.Render("Ctrl+R"),
		" Refresh  ",
		HelpKeyStyle.Render("Ctrl+D"),
		" Detach  ",
		HelpKeyStyle.Render("?"),
		" Help  ",
		HelpKeyStyle.Render("q"),
		" Quit",
	)

	footer := lipgloss.JoinVertical(lipgloss.Center,
		bgStyle.Width(totalWidth).Render(""),
		bgStyle.Width(totalWidth).Align(lipgloss.Center).Render(footerLine),
		bgStyle.Width(totalWidth).Render(""),
		bgStyle.Width(totalWidth).Align(lipgloss.Center).Render(SubtitleStyle.Render("Press any key to close")),
	)

	helpContent := lipgloss.JoinVertical(lipgloss.Center,
		bgStyle.Width(totalWidth).Align(lipgloss.Center).Render(DialogTitleStyle.Render("Keyboard Shortcuts")),
		bgStyle.Width(totalWidth).Render(""),
		columns,
		footer,
	)

	helpBox := DialogStyle.Width(totalWidth + 4).Render(helpContent)

	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, helpBox)
}

// renderFilterOverlay renders the filter input overlay
func (m *Model) renderFilterOverlay(background string, width, height int) string {
	dialogWidth := 40

	// Style for content with background
	bgStyle := lipgloss.NewStyle().
		Background(ColorBgAlt).
		Foreground(ColorText).
		Width(dialogWidth).
		Align(lipgloss.Center)

	input := InputFocusedStyle.Width(30).Render(m.filterText + "█")
	filterBox := DialogStyle.Width(dialogWidth + 4).Render(
		lipgloss.JoinVertical(lipgloss.Center,
			bgStyle.Render(DialogTitleStyle.Render("Filter")),
			bgStyle.Render(""),
			bgStyle.Render(input),
			bgStyle.Render(""),
			bgStyle.Render(SubtitleStyle.Render("Enter to apply, Esc to cancel")),
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

