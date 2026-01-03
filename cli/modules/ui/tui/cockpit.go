package tui

import (
	"fmt"
	"strings"

	"csd-devtrack/cli/modules/platform/config"
	"csd-devtrack/cli/modules/ui/core"

	"github.com/charmbracelet/lipgloss"
)

// WidgetLayout represents a calculated widget position and size
type WidgetLayout struct {
	Widget *config.WidgetConfig
	X      int // Left position
	Y      int // Top position
	Width  int
	Height int
}

// renderCockpit renders the configurable widgets view
func (m *Model) renderCockpit(width, height int) string {
	cfg := config.GetGlobal()
	if cfg == nil {
		return m.renderCockpitPlaceholder(width, height, "No configuration loaded")
	}

	// Check if there are any profiles
	if cfg.WidgetProfiles == nil || len(cfg.WidgetProfiles) == 0 {
		return m.renderCockpitEmpty(width, height)
	}

	// Get active profile
	profileName := m.getActiveCockpitProfile()
	profile := m.getCockpitProfile(profileName)
	if profile == nil {
		return m.renderCockpitEmpty(width, height)
	}

	// Render header
	header := m.renderCockpitHeader(width)
	headerHeight := 2 // header + gap

	// Calculate grid layout for remaining space
	gridHeight := height - headerHeight
	layouts := m.calculateWidgetLayouts(profile, width, gridHeight)
	if len(layouts) == 0 {
		return m.renderCockpitPlaceholder(width, height, "No widgets in profile")
	}

	// Render grid
	grid := m.renderWidgetGrid(layouts, profile, width, gridHeight)

	// Combine header and grid
	content := lipgloss.JoinVertical(lipgloss.Left, header, "", grid)

	// Add config overlay if in config mode
	if m.cockpitConfigMode {
		content = m.renderCockpitConfigOverlay(content, width, height)
	}

	return content
}

// renderCockpitEmpty renders the empty state when no profiles exist
func (m *Model) renderCockpitEmpty(width, height int) string {
	// Check for config overlay first (e.g., creating new profile)
	if m.cockpitConfigMode {
		// Render empty background with overlay
		emptyBg := lipgloss.NewStyle().
			Width(width).
			Height(height).
			Render("")
		return m.renderCockpitConfigOverlay(emptyBg, width, height)
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorSecondary)

	messageStyle := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Align(lipgloss.Center)

	keyStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary)

	content := lipgloss.JoinVertical(lipgloss.Center,
		titleStyle.Render("≡ COCKPIT"),
		"",
		"",
		messageStyle.Render("No widget profiles configured"),
		"",
		messageStyle.Render("Press "+keyStyle.Render("n")+" to create your first profile"),
	)

	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(content)
}

// getActiveCockpitProfile returns the active widget profile name
func (m *Model) getActiveCockpitProfile() string {
	cfg := config.GetGlobal()
	if cfg != nil && cfg.Settings != nil && cfg.Settings.ActiveWidgetProfile != "" {
		return cfg.Settings.ActiveWidgetProfile
	}
	return "default"
}

// getCockpitProfile returns the widget profile by name
func (m *Model) getCockpitProfile(name string) *config.WidgetProfile {
	cfg := config.GetGlobal()
	if cfg == nil || cfg.WidgetProfiles == nil {
		// Return default profile
		profiles := config.DefaultWidgetProfiles()
		if p, ok := profiles[name]; ok {
			return p
		}
		if p, ok := profiles["default"]; ok {
			return p
		}
		return nil
	}
	if p, ok := cfg.WidgetProfiles[name]; ok {
		return p
	}
	// Fallback to default
	if p, ok := cfg.WidgetProfiles["default"]; ok {
		return p
	}
	return nil
}

// calculateWidgetLayouts calculates the position and size of each widget
func (m *Model) calculateWidgetLayouts(profile *config.WidgetProfile, width, height int) []WidgetLayout {
	if profile == nil || len(profile.Widgets) == 0 {
		return nil
	}

	gap := GapHorizontal
	rows := profile.Rows
	cols := profile.Cols
	if rows < 1 {
		rows = 1
	}
	if cols < 1 {
		cols = 1
	}

	// Calculate cell dimensions
	totalGapH := gap * (cols + 1)
	totalGapV := gap * (rows + 1)
	cellWidth := (width - totalGapH) / cols
	cellHeight := (height - totalGapV) / rows

	var layouts []WidgetLayout
	for i := range profile.Widgets {
		widget := &profile.Widgets[i]

		// Calculate position and size with spans
		rowSpan := widget.RowSpan
		colSpan := widget.ColSpan
		if rowSpan < 1 {
			rowSpan = 1
		}
		if colSpan < 1 {
			colSpan = 1
		}

		x := widget.Col*(cellWidth+gap) + gap
		y := widget.Row*(cellHeight+gap) + gap
		w := cellWidth*colSpan + (colSpan-1)*gap
		h := cellHeight*rowSpan + (rowSpan-1)*gap

		// Clamp to bounds
		if x+w > width {
			w = width - x
		}
		if y+h > height {
			h = height - y
		}
		if w < 10 {
			w = 10
		}
		if h < 3 {
			h = 3
		}

		layouts = append(layouts, WidgetLayout{
			Widget: widget,
			X:      x,
			Y:      y,
			Width:  w,
			Height: h,
		})
	}

	return layouts
}

// renderCockpitHeader renders the header showing active profile
func (m *Model) renderCockpitHeader(width int) string {
	profileName := m.getActiveCockpitProfile()
	profiles := m.getAvailableProfiles()

	// Find profile number for hint
	profileNum := 0
	for i, name := range profiles {
		if name == profileName {
			profileNum = i + 1
			break
		}
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorSecondary)

	profileStyle := lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true)

	hintStyle := lipgloss.NewStyle().
		Foreground(ColorMuted)

	header := titleStyle.Render("≡ COCKPIT") + "  " +
		profileStyle.Render(profileName)

	if profileNum > 0 && profileNum <= 9 {
		header += " " + hintStyle.Render(fmt.Sprintf("(%d/%d)", profileNum, len(profiles)))
	}

	return header
}

// renderWidgetGrid renders all widgets on a grid
func (m *Model) renderWidgetGrid(layouts []WidgetLayout, profile *config.WidgetProfile, width, height int) string {
	// Create a blank canvas
	canvas := make([][]rune, height)
	for i := range canvas {
		canvas[i] = make([]rune, width)
		for j := range canvas[i] {
			canvas[i][j] = ' '
		}
	}

	// Render each widget and place on canvas
	for i, layout := range layouts {
		focused := m.focusArea == FocusMain && m.cockpitFocusedIndex == i
		content := m.renderWidgetContent(layout, focused)

		// Place content on canvas (line by line)
		lines := strings.Split(content, "\n")
		for row, line := range lines {
			if layout.Y+row >= height {
				break
			}
			runes := []rune(line)
			for col, r := range runes {
				if layout.X+col >= width {
					break
				}
				if layout.Y+row >= 0 && layout.X+col >= 0 {
					canvas[layout.Y+row][layout.X+col] = r
				}
			}
		}
	}

	// Convert canvas to string
	var sb strings.Builder
	for i, row := range canvas {
		sb.WriteString(string(row))
		if i < len(canvas)-1 {
			sb.WriteRune('\n')
		}
	}

	return sb.String()
}

// renderWidgetContent renders a single widget's content
func (m *Model) renderWidgetContent(layout WidgetLayout, focused bool) string {
	widget := layout.Widget
	w := layout.Width
	h := layout.Height

	// Title
	title := widget.Title
	if title == "" {
		title = widget.Type
	}

	// Border style based on focus
	var borderStyle lipgloss.Style
	if focused {
		borderStyle = FocusedBorderStyle.
			Width(w - 2).
			Height(h - 2)
	} else {
		borderStyle = UnfocusedBorderStyle.
			Width(w - 2).
			Height(h - 2)
	}

	// Render content based on type
	contentHeight := h - 4 // Account for border + title
	if contentHeight < 1 {
		contentHeight = 1
	}
	contentWidth := w - 4 // Account for border + padding
	if contentWidth < 1 {
		contentWidth = 1
	}

	var content string
	switch config.WidgetType(widget.Type) {
	case config.WidgetLogs:
		content = m.renderWidgetLogs(widget, contentWidth, contentHeight)
	case config.WidgetProcesses:
		content = m.renderWidgetProcesses(widget, contentWidth, contentHeight)
	case config.WidgetBuildStatus:
		content = m.renderWidgetBuild(widget, contentWidth, contentHeight)
	case config.WidgetGitStatus:
		content = m.renderWidgetGit(widget, contentWidth, contentHeight)
	case config.WidgetClaudeSessions:
		content = m.renderWidgetClaude(widget, contentWidth, contentHeight)
	case config.WidgetDatabaseSessions:
		content = m.renderWidgetDatabase(widget, contentWidth, contentHeight)
	case config.WidgetDashStats:
		content = m.renderWidgetDashStats(widget, contentWidth, contentHeight)
	default:
		content = lipgloss.NewStyle().
			Foreground(ColorMuted).
			Render(fmt.Sprintf("Unknown widget type: %s", widget.Type))
	}

	// Title style
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorSecondary)

	// Combine title and content
	fullContent := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render("≡ "+strings.ToUpper(title)),
		content,
	)

	return borderStyle.Render(fullContent)
}

// renderWidgetLogs renders log lines for a logs widget
func (m *Model) renderWidgetLogs(widget *config.WidgetConfig, width, height int) string {
	if m.state.Logs == nil || len(m.state.Logs.Lines) == 0 {
		return lipgloss.NewStyle().
			Foreground(ColorMuted).
			Render("No logs")
	}

	// Filter logs based on widget config
	var filtered []core.LogLineVM
	for _, line := range m.state.Logs.Lines {
		// Project filter
		if widget.ProjectFilter != "" && line.Source != widget.ProjectFilter {
			continue
		}
		// Level filter
		if widget.LogLevelFilter != "" && line.Level != widget.LogLevelFilter {
			continue
		}
		filtered = append(filtered, line)
	}

	if len(filtered) == 0 {
		return lipgloss.NewStyle().
			Foreground(ColorMuted).
			Render("No matching logs")
	}

	// Take last N lines that fit
	start := 0
	if len(filtered) > height {
		start = len(filtered) - height
	}

	var lines []string
	for i := start; i < len(filtered); i++ {
		line := filtered[i]
		// Format: [TIME] [LEVEL] message
		levelStyle := lipgloss.NewStyle()
		switch line.Level {
		case "error":
			levelStyle = levelStyle.Foreground(ColorError)
		case "warn":
			levelStyle = levelStyle.Foreground(ColorWarning)
		default:
			levelStyle = levelStyle.Foreground(ColorMuted)
		}

		text := fmt.Sprintf("%s %s",
			lipgloss.NewStyle().Foreground(ColorMuted).Render(line.TimeStr),
			levelStyle.Render(truncate(line.Message, width-10)),
		)
		lines = append(lines, text)
	}

	return strings.Join(lines, "\n")
}

// renderWidgetProcesses renders a compact process list
func (m *Model) renderWidgetProcesses(widget *config.WidgetConfig, width, height int) string {
	if m.state.Processes == nil || len(m.state.Processes.Processes) == 0 {
		return lipgloss.NewStyle().
			Foreground(ColorMuted).
			Render("No processes")
	}

	var lines []string
	for i, proc := range m.state.Processes.Processes {
		if i >= height {
			break
		}

		// Project filter
		if widget.ProjectFilter != "" && proc.ProjectID != widget.ProjectFilter {
			continue
		}

		// Status indicator
		var indicator string
		switch proc.State {
		case "running":
			indicator = lipgloss.NewStyle().Foreground(ColorSuccess).Render("●")
		case "stopped":
			indicator = lipgloss.NewStyle().Foreground(ColorMuted).Render("○")
		case "failed":
			indicator = lipgloss.NewStyle().Foreground(ColorError).Render("✗")
		default:
			indicator = lipgloss.NewStyle().Foreground(ColorMuted).Render("?")
		}

		name := truncate(proc.ProjectName, width-15)
		uptime := proc.Uptime
		if len(uptime) > 8 {
			uptime = uptime[:8]
		}

		line := fmt.Sprintf("%s %s %s", indicator, name,
			lipgloss.NewStyle().Foreground(ColorMuted).Render(uptime))
		lines = append(lines, line)
	}

	if len(lines) == 0 {
		return lipgloss.NewStyle().
			Foreground(ColorMuted).
			Render("No matching processes")
	}

	return strings.Join(lines, "\n")
}

// renderWidgetBuild renders build status
func (m *Model) renderWidgetBuild(widget *config.WidgetConfig, width, height int) string {
	if m.state.Builds == nil {
		return lipgloss.NewStyle().
			Foreground(ColorMuted).
			Render("No build data")
	}

	var lines []string

	// Current build if any
	if m.state.Builds.CurrentBuild != nil {
		b := m.state.Builds.CurrentBuild
		status := lipgloss.NewStyle().Foreground(ColorSecondary).Render("⟳ BUILDING")
		line := fmt.Sprintf("%s %s (%d%%)", status, b.ProjectName, b.Progress)
		lines = append(lines, line)
	}

	// Recent builds
	for i, build := range m.state.Builds.BuildHistory {
		if len(lines) >= height {
			break
		}
		if i >= 5 { // Limit history
			break
		}

		var status string
		switch build.Status {
		case "success":
			status = lipgloss.NewStyle().Foreground(ColorSuccess).Render("✓")
		case "failed":
			status = lipgloss.NewStyle().Foreground(ColorError).Render("✗")
		default:
			status = lipgloss.NewStyle().Foreground(ColorMuted).Render("○")
		}

		line := fmt.Sprintf("%s %s %s", status,
			truncate(build.ProjectName, width-15),
			lipgloss.NewStyle().Foreground(ColorMuted).Render(build.Duration))
		lines = append(lines, line)
	}

	if len(lines) == 0 {
		return lipgloss.NewStyle().
			Foreground(ColorMuted).
			Render("No build history")
	}

	return strings.Join(lines, "\n")
}

// renderWidgetGit renders git status
func (m *Model) renderWidgetGit(widget *config.WidgetConfig, width, height int) string {
	if m.state.Git == nil || len(m.state.Git.Projects) == 0 {
		return lipgloss.NewStyle().
			Foreground(ColorMuted).
			Render("No git data")
	}

	var lines []string
	for i, proj := range m.state.Git.Projects {
		if i >= height {
			break
		}

		// Project filter
		if widget.ProjectFilter != "" && proj.ProjectID != widget.ProjectFilter {
			continue
		}

		// Status indicators
		var status string
		if proj.IsClean {
			status = lipgloss.NewStyle().Foreground(ColorSuccess).Render("✓")
		} else {
			changes := len(proj.Modified) + len(proj.Staged) + len(proj.Untracked)
			status = lipgloss.NewStyle().Foreground(ColorWarning).Render(fmt.Sprintf("~%d", changes))
		}

		// Branch with ahead/behind
		branch := proj.Branch
		if proj.Ahead > 0 {
			branch += lipgloss.NewStyle().Foreground(ColorSuccess).Render(fmt.Sprintf("↑%d", proj.Ahead))
		}
		if proj.Behind > 0 {
			branch += lipgloss.NewStyle().Foreground(ColorWarning).Render(fmt.Sprintf("↓%d", proj.Behind))
		}

		line := fmt.Sprintf("%s %s %s", status,
			truncate(proj.ProjectName, width-20),
			lipgloss.NewStyle().Foreground(ColorSecondary).Render(branch))
		lines = append(lines, line)
	}

	if len(lines) == 0 {
		return lipgloss.NewStyle().
			Foreground(ColorMuted).
			Render("No matching repos")
	}

	return strings.Join(lines, "\n")
}

// renderWidgetClaude renders Claude sessions
func (m *Model) renderWidgetClaude(widget *config.WidgetConfig, width, height int) string {
	if m.state.Claude == nil || !m.state.Claude.IsInstalled {
		return lipgloss.NewStyle().
			Foreground(ColorMuted).
			Render("Claude not installed")
	}

	if len(m.state.Claude.Sessions) == 0 {
		return lipgloss.NewStyle().
			Foreground(ColorMuted).
			Render("No sessions")
	}

	var lines []string
	for i, sess := range m.state.Claude.Sessions {
		if i >= height {
			break
		}

		// Filter by session ID if specified
		if widget.SessionID != "" && sess.ID != widget.SessionID {
			continue
		}
		// Filter by project
		if widget.ProjectFilter != "" && sess.ProjectID != widget.ProjectFilter {
			continue
		}

		// State indicator
		var indicator string
		switch sess.State {
		case "running":
			indicator = lipgloss.NewStyle().Foreground(ColorSuccess).Render("●")
		case "waiting":
			indicator = lipgloss.NewStyle().Foreground(ColorWarning).Render("◐")
		default:
			indicator = lipgloss.NewStyle().Foreground(ColorMuted).Render("○")
		}

		name := sess.Name
		if name == "" {
			name = sess.ID[:8]
		}

		line := fmt.Sprintf("%s %s", indicator, truncate(name, width-4))
		lines = append(lines, line)
	}

	if len(lines) == 0 {
		return lipgloss.NewStyle().
			Foreground(ColorMuted).
			Render("No matching sessions")
	}

	return strings.Join(lines, "\n")
}

// renderWidgetDatabase renders database sessions
func (m *Model) renderWidgetDatabase(widget *config.WidgetConfig, width, height int) string {
	if m.state.Database == nil || len(m.state.Database.Databases) == 0 {
		return lipgloss.NewStyle().
			Foreground(ColorMuted).
			Render("No databases found")
	}

	var lines []string

	// Show databases and sessions
	for i, db := range m.state.Database.Databases {
		if i >= height {
			break
		}

		// Filter by project
		if widget.ProjectFilter != "" && db.ProjectID != widget.ProjectFilter {
			continue
		}

		// Database type icon
		var icon string
		switch db.Type {
		case "postgres":
			icon = lipgloss.NewStyle().Foreground(ColorSecondary).Render("🐘")
		case "mysql":
			icon = lipgloss.NewStyle().Foreground(ColorSecondary).Render("🐬")
		case "sqlite":
			icon = lipgloss.NewStyle().Foreground(ColorSecondary).Render("📦")
		default:
			icon = lipgloss.NewStyle().Foreground(ColorMuted).Render("○")
		}

		line := fmt.Sprintf("%s %s (%s)",
			icon,
			truncate(db.DatabaseName, width-15),
			db.ProjectName)
		lines = append(lines, line)
	}

	// Add sessions
	for i, sess := range m.state.Database.Sessions {
		if len(lines)+i >= height {
			break
		}

		// Filter by project
		if widget.ProjectFilter != "" && sess.ProjectID != widget.ProjectFilter {
			continue
		}

		// State indicator
		var indicator string
		switch sess.State {
		case "running":
			indicator = lipgloss.NewStyle().Foreground(ColorSuccess).Render("●")
		default:
			indicator = lipgloss.NewStyle().Foreground(ColorMuted).Render("○")
		}

		line := fmt.Sprintf("  %s %s", indicator, truncate(sess.Name, width-6))
		lines = append(lines, line)
	}

	if len(lines) == 0 {
		return lipgloss.NewStyle().
			Foreground(ColorMuted).
			Render("No matching databases")
	}

	return strings.Join(lines, "\n")
}

// renderWidgetDashStats renders dashboard statistics
func (m *Model) renderWidgetDashStats(widget *config.WidgetConfig, width, height int) string {
	if m.state.Dashboard == nil {
		return lipgloss.NewStyle().
			Foreground(ColorMuted).
			Render("No dashboard data")
	}

	d := m.state.Dashboard

	// Create stat boxes
	projectsStat := fmt.Sprintf("Projects: %d", d.ProjectCount)
	runningStat := fmt.Sprintf("Running: %s%d%s",
		lipgloss.NewStyle().Foreground(ColorSuccess).Render(""),
		d.RunningCount,
		lipgloss.NewStyle().Foreground(ColorText).Render(""))
	buildingStat := fmt.Sprintf("Building: %d", d.BuildingCount)
	errorsStat := ""
	if d.ErrorCount > 0 {
		errorsStat = fmt.Sprintf("Errors: %s%d%s",
			lipgloss.NewStyle().Foreground(ColorError).Render(""),
			d.ErrorCount,
			lipgloss.NewStyle().Foreground(ColorText).Render(""))
	} else {
		errorsStat = "Errors: 0"
	}

	// Render based on available space
	if width >= 60 {
		// Horizontal layout
		return lipgloss.JoinHorizontal(lipgloss.Center,
			projectsStat, "  │  ", runningStat, "  │  ", buildingStat, "  │  ", errorsStat)
	}

	// Vertical layout
	return lipgloss.JoinVertical(lipgloss.Left,
		projectsStat,
		runningStat,
		buildingStat,
		errorsStat,
	)
}

// renderCockpitPlaceholder renders a placeholder when widgets can't be shown
func (m *Model) renderCockpitPlaceholder(width, height int, message string) string {
	style := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Align(lipgloss.Center, lipgloss.Center).
		Foreground(ColorMuted)

	content := fmt.Sprintf("%s\n\nPress 'c' to configure widgets", message)
	return style.Render(content)
}

// navigateCockpitUp moves to the widget above the current one
func (m *Model) navigateCockpitUp() {
	profile := m.getCockpitProfile(m.getActiveCockpitProfile())
	if profile == nil || len(profile.Widgets) == 0 {
		return
	}

	// Find current widget's row/col
	if m.cockpitFocusedIndex < 0 || m.cockpitFocusedIndex >= len(profile.Widgets) {
		m.cockpitFocusedIndex = 0
		return
	}

	currentWidget := profile.Widgets[m.cockpitFocusedIndex]
	currentRow := currentWidget.Row
	currentCol := currentWidget.Col

	// Find widget in the row above at same or nearest column
	targetRow := currentRow - 1
	if targetRow < 0 {
		return // Already at top
	}

	// Find best matching widget in target row
	bestIdx := -1
	bestColDiff := 999
	for i, w := range profile.Widgets {
		if w.Row == targetRow {
			colDiff := abs(w.Col - currentCol)
			if bestIdx == -1 || colDiff < bestColDiff {
				bestIdx = i
				bestColDiff = colDiff
			}
		}
	}

	if bestIdx >= 0 {
		m.cockpitFocusedIndex = bestIdx
	}
}

// navigateCockpitDown moves to the widget below the current one
func (m *Model) navigateCockpitDown() {
	profile := m.getCockpitProfile(m.getActiveCockpitProfile())
	if profile == nil || len(profile.Widgets) == 0 {
		return
	}

	if m.cockpitFocusedIndex < 0 || m.cockpitFocusedIndex >= len(profile.Widgets) {
		m.cockpitFocusedIndex = 0
		return
	}

	currentWidget := profile.Widgets[m.cockpitFocusedIndex]
	currentRow := currentWidget.Row
	currentCol := currentWidget.Col

	// Find widget in the row below at same or nearest column
	targetRow := currentRow + 1

	// Find best matching widget in target row
	bestIdx := -1
	bestColDiff := 999
	for i, w := range profile.Widgets {
		if w.Row == targetRow {
			colDiff := abs(w.Col - currentCol)
			if bestIdx == -1 || colDiff < bestColDiff {
				bestIdx = i
				bestColDiff = colDiff
			}
		}
	}

	if bestIdx >= 0 {
		m.cockpitFocusedIndex = bestIdx
	}
}

// navigateCockpitLeft moves to the widget to the left of the current one
func (m *Model) navigateCockpitLeft() {
	profile := m.getCockpitProfile(m.getActiveCockpitProfile())
	if profile == nil || len(profile.Widgets) == 0 {
		return
	}

	if m.cockpitFocusedIndex < 0 || m.cockpitFocusedIndex >= len(profile.Widgets) {
		m.cockpitFocusedIndex = 0
		return
	}

	currentWidget := profile.Widgets[m.cockpitFocusedIndex]
	currentRow := currentWidget.Row
	currentCol := currentWidget.Col

	// Find widget in the same row to the left
	bestIdx := -1
	bestCol := -1
	for i, w := range profile.Widgets {
		if w.Row == currentRow && w.Col < currentCol {
			if bestIdx == -1 || w.Col > bestCol {
				bestIdx = i
				bestCol = w.Col
			}
		}
	}

	if bestIdx >= 0 {
		m.cockpitFocusedIndex = bestIdx
	}
}

// navigateCockpitRight moves to the widget to the right of the current one
func (m *Model) navigateCockpitRight() {
	profile := m.getCockpitProfile(m.getActiveCockpitProfile())
	if profile == nil || len(profile.Widgets) == 0 {
		return
	}

	if m.cockpitFocusedIndex < 0 || m.cockpitFocusedIndex >= len(profile.Widgets) {
		m.cockpitFocusedIndex = 0
		return
	}

	currentWidget := profile.Widgets[m.cockpitFocusedIndex]
	currentRow := currentWidget.Row
	currentCol := currentWidget.Col

	// Find widget in the same row to the right
	bestIdx := -1
	bestCol := 999
	for i, w := range profile.Widgets {
		if w.Row == currentRow && w.Col > currentCol {
			if bestIdx == -1 || w.Col < bestCol {
				bestIdx = i
				bestCol = w.Col
			}
		}
	}

	if bestIdx >= 0 {
		m.cockpitFocusedIndex = bestIdx
	}
}

// abs returns the absolute value of an integer
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// switchCockpitProfile switches to a widget profile by number key
func (m *Model) switchCockpitProfile(key string) {
	cfg := config.GetGlobal()
	if cfg == nil || cfg.WidgetProfiles == nil {
		return
	}

	// Get sorted list of profile names
	var profileNames []string
	for name := range cfg.WidgetProfiles {
		profileNames = append(profileNames, name)
	}

	// Sort for consistent ordering
	// (simple alphabetical sort)
	for i := 0; i < len(profileNames)-1; i++ {
		for j := i + 1; j < len(profileNames); j++ {
			if profileNames[i] > profileNames[j] {
				profileNames[i], profileNames[j] = profileNames[j], profileNames[i]
			}
		}
	}

	// Convert key to index (1-based)
	idx := int(key[0] - '1')
	if idx >= 0 && idx < len(profileNames) {
		// Update active profile in config
		if cfg.Settings != nil {
			cfg.Settings.ActiveWidgetProfile = profileNames[idx]
			// Reset focused index when switching profiles
			m.cockpitFocusedIndex = 0
			// Save config to persist the change
			_ = config.SaveGlobal()
		}
	}
}
