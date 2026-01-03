package tui

import (
	"fmt"
	"sort"
	"strings"

	"csd-devtrack/cli/modules/ui/core"

	"github.com/charmbracelet/lipgloss"
)

// Database view modes
const (
	DatabaseModeSession = "sessions"
)

// databaseTreeItem represents an item in the database tree
type databaseTreeItem struct {
	IsProject  bool   // true = project, false = database
	ProjectID  string // Project ID
	DatabaseID string // Database ID (empty for projects)
}

// renderDatabase renders the Database view
// Layout: Terminal on left (70%), Sessions panel on right (30%)
func (m *Model) renderDatabase(width, height int) string {
	// Check if database service found any databases
	if m.state.Database == nil || len(m.state.Database.Databases) == 0 {
		return m.renderDatabaseNoDatabases(width, height)
	}

	// Layout: terminal (1 panel) + sessions (2 stacked panels)
	// Height: max(1×2, 2×2) = 4
	// Width: 2 panels × 2 = 4
	heightBorders := 4
	widthBorders := 4
	contentHeight := height - heightBorders
	availableWidth := width - widthBorders - GapHorizontal

	// Calculate sessions panel width using TreeMenu
	sessionsWidth := m.databaseTreeMenu.CalcWidth()
	termWidth := availableWidth - sessionsWidth

	// Session info takes some space at bottom
	infoHeight := 8
	treeHeight := contentHeight - infoHeight

	// Configure and render TreeMenu
	m.databaseTreeMenu.SetSize(sessionsWidth, treeHeight)
	m.databaseTreeMenu.SetFocused(m.focusArea == FocusDetail)

	// Terminal panel has only 1 panel (not 2 stacked like sessions), so add +2
	termPanel := m.renderDatabaseTerminalPanel(termWidth, contentHeight+2)
	treePanel := m.databaseTreeMenu.Render()
	infoPanel := m.renderDatabaseSessionInfo(sessionsWidth, infoHeight)
	sessionsPanel := lipgloss.JoinVertical(lipgloss.Left, treePanel, infoPanel)

	return lipgloss.JoinHorizontal(lipgloss.Top,
		termPanel,
		strings.Repeat(" ", GapHorizontal),
		sessionsPanel,
	)
}

// renderDatabaseSessionInfo renders selected session information
func (m *Model) renderDatabaseSessionInfo(width, height int) string {
	// Get selected item from TreeMenu
	var sess *core.DatabaseSessionVM
	var db *core.DatabaseInfoVM
	if m.databaseTreeMenu != nil {
		if item := m.databaseTreeMenu.SelectedItem(); item != nil {
			if s, ok := item.Data.(core.DatabaseSessionVM); ok {
				sess = &s
			} else if d, ok := item.Data.(core.DatabaseInfoVM); ok {
				db = &d
			}
		}
	}

	// Use same border style as TreeMenu for alignment
	borderStyle := UnfocusedBorderStyle

	// Adjust width for right-side panel border (1 panel = 1 × 2)
	renderWidth := width - 2
	if renderWidth < 20 {
		renderWidth = 20
	}

	if sess == nil && db == nil {
		// No item selected - show placeholder
		content := lipgloss.NewStyle().
			Foreground(ColorMuted).
			Render("Select a database")

		return borderStyle.
			Width(renderWidth).
			Height(height).
			Align(lipgloss.Center, lipgloss.Center).
			Render(content)
	}

	// Inner dimensions (inside border)
	innerWidth := renderWidth - 2
	innerHeight := height - 2
	contentWidth := innerWidth - 2 // padding

	mutedStyle := lipgloss.NewStyle().Foreground(ColorMuted)
	valueStyle := lipgloss.NewStyle().Foreground(ColorText)

	var lines []string

	if sess != nil {
		// Session selected - show session info
		relTime := formatRelativeTime(sess.LastActiveAt)
		infoLine := relTime + " · " + sess.DatabaseType
		lines = append(lines, mutedStyle.Render(infoLine))

		// Database name
		lines = append(lines, valueStyle.Render("DB: "+sess.DatabaseName))

		// Duration
		duration := formatDuration(sess.CreatedAt, sess.LastActiveAt)
		lines = append(lines, valueStyle.Render("Duration: "+duration))
	} else if db != nil {
		// Database selected - show database info
		lines = append(lines, valueStyle.Render(fmt.Sprintf("Type: %s", db.Type)))
		lines = append(lines, valueStyle.Render(fmt.Sprintf("DB: %s", db.DatabaseName)))
		lines = append(lines, valueStyle.Render(fmt.Sprintf("Host: %s:%d", db.Host, db.Port)))
		if db.User != "" {
			lines = append(lines, mutedStyle.Render(fmt.Sprintf("User: %s", db.User)))
		}
		lines = append(lines, mutedStyle.Render(fmt.Sprintf("Source: %s", db.Source)))
	}

	// Pad each line to exact width
	for i, line := range lines {
		lineWidth := lipgloss.Width(line)
		if lineWidth < contentWidth {
			lines[i] = line + strings.Repeat(" ", contentWidth-lineWidth)
		}
	}

	// Pad to exact height
	for len(lines) < innerHeight {
		lines = append(lines, strings.Repeat(" ", contentWidth))
	}

	content := lipgloss.JoinVertical(lipgloss.Left, lines...)

	return borderStyle.
		Width(renderWidth).
		Height(height).
		Padding(0, 1).
		Render(content)
}

// renderDatabaseNoDatabases shows message when no databases are found
func (m *Model) renderDatabaseNoDatabases(width, height int) string {
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
		Render("No Databases Found")

	msg := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Render("Add database configuration to your project YAML files:\ncommon.database.url or backend.database.url")

	content := lipgloss.JoinVertical(lipgloss.Center,
		icon,
		"",
		title,
		"",
		msg,
	)

	return style.Render(content)
}

// renderDatabaseTerminalPanel renders the main terminal area (terminal or placeholder)
func (m *Model) renderDatabaseTerminalPanel(width, height int) string {
	// Show terminal panel if there's an active session with a running terminal
	if m.databaseActiveSession != "" && m.terminalManager != nil {
		if t := m.terminalManager.Get(m.databaseActiveSession); t != nil && t.IsRunning() {
			return m.renderTerminalPanel(t, width, height)
		}
	}

	// No terminal running - show placeholder
	style := UnfocusedBorderStyle
	if m.focusArea == FocusMain {
		style = FocusedBorderStyle
	}

	var message string
	if m.databaseActiveSession == "" {
		message = "Select a database and press 'n' to create a session"
	} else {
		message = "Press Enter to connect to database"
	}

	content := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Align(lipgloss.Center).
		Width(width - 4).
		Render(message)

	return style.
		Width(width - 2).
		Height(height).
		Align(lipgloss.Center, lipgloss.Center).
		Render(content)
}

// buildDatabaseTreeItems builds the tree menu items for database view
func (m *Model) buildDatabaseTreeItems() []TreeMenuItem {
	if m.state.Database == nil {
		return nil
	}

	// Group databases by project
	projectDatabases := make(map[string][]core.DatabaseInfoVM)
	projectNames := make(map[string]string)

	for _, db := range m.state.Database.Databases {
		projectDatabases[db.ProjectID] = append(projectDatabases[db.ProjectID], db)
		projectNames[db.ProjectID] = db.ProjectName
	}

	// Get sorted project IDs (must sort to avoid flickering from non-deterministic map iteration)
	var projectIDs []string
	for pid := range projectDatabases {
		projectIDs = append(projectIDs, pid)
	}
	// Sort by project name (alphabetical, case-insensitive)
	sort.Slice(projectIDs, func(i, j int) bool {
		return strings.ToLower(projectNames[projectIDs[i]]) < strings.ToLower(projectNames[projectIDs[j]])
	})

	var items []TreeMenuItem

	for _, projectID := range projectIDs {
		projectName := projectNames[projectID]
		databases := projectDatabases[projectID]

		// Sort databases by name (alphabetical, case-insensitive)
		sort.Slice(databases, func(i, j int) bool {
			return strings.ToLower(databases[i].DatabaseName) < strings.ToLower(databases[j].DatabaseName)
		})

		// Project header
		projectIcon := "📁"

		projectItem := TreeMenuItem{
			ID:        "project:" + projectID,
			Label:     projectName,
			Icon:      projectIcon,
			IconColor: ColorSecondary,
			Count:     len(databases),
			Data: databaseTreeItem{
				IsProject: true,
				ProjectID: projectID,
			},
		}

		// Add databases as children
		for _, db := range databases {
			dbIcon := "○"
			iconColor := ColorMuted

			// Show active state if this database has running terminal
			if db.ID == m.databaseActiveSession {
				dbIcon = "●"
				iconColor = ColorSuccess
			}

			dbItem := TreeMenuItem{
				ID:        "db:" + db.ID,
				Label:     db.DatabaseName,
				Icon:      dbIcon,
				IconColor: iconColor,
				IsActive:  db.ID == m.databaseActiveSession,
				Data:      db,
			}
			projectItem.Children = append(projectItem.Children, dbItem)
		}

		items = append(items, projectItem)
	}

	return items
}
