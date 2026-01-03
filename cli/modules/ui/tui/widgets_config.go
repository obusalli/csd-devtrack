package tui

import (
	"fmt"
	"strings"

	"csd-devtrack/cli/modules/platform/config"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Widget type options for the type menu
var widgetTypeOptions = []struct {
	Type  config.WidgetType
	Label string
	Desc  string
}{
	{config.WidgetDashStats, "Dashboard Stats", "Project counts and status summary"},
	{config.WidgetProcesses, "Processes", "Running processes list"},
	{config.WidgetLogs, "Logs", "Log output stream"},
	{config.WidgetBuildStatus, "Build Status", "Build progress and history"},
	{config.WidgetGitStatus, "Git Status", "Repository status"},
	{config.WidgetClaudeSessions, "Claude Sessions", "AI assistant sessions"},
}

// initWidgetsConfigMenus initializes the TreeMenus for widget configuration
func (m *Model) initWidgetsConfigMenus() {
	// Get current profile settings
	profile := m.getWidgetProfile(m.getActiveWidgetProfile())
	if profile != nil {
		m.widgetsConfigRows = profile.Rows
		m.widgetsConfigCols = profile.Cols
	} else {
		m.widgetsConfigRows = 2
		m.widgetsConfigCols = 2
	}

	// Initialize grid size menu
	gridItems := []TreeMenuItem{}
	for rows := 1; rows <= 4; rows++ {
		for cols := 1; cols <= 4; cols++ {
			label := fmt.Sprintf("%dx%d (%d rows, %d cols)", rows, cols, rows, cols)
			gridItems = append(gridItems, TreeMenuItem{
				Label: label,
				Data:  [2]int{rows, cols},
			})
		}
	}
	m.widgetsGridMenu = NewTreeMenu(gridItems)
	m.widgetsGridMenu.SetTitle("Grid Size")

	// Initialize widget type menu
	typeItems := []TreeMenuItem{}
	for _, opt := range widgetTypeOptions {
		typeItems = append(typeItems, TreeMenuItem{
			Label: fmt.Sprintf("%s - %s", opt.Label, opt.Desc),
			Data:  opt.Type,
		})
	}
	m.widgetsTypeMenu = NewTreeMenu(typeItems)
	m.widgetsTypeMenu.SetTitle("Widget Type")

	// Initialize profile menu
	m.initWidgetsProfileMenu()
}

// initWidgetsProfileMenu initializes the profile selection menu
func (m *Model) initWidgetsProfileMenu() {
	cfg := config.GetGlobal()
	items := []TreeMenuItem{}

	if cfg != nil && cfg.WidgetProfiles != nil {
		for name, profile := range cfg.WidgetProfiles {
			desc := profile.Description
			if desc == "" {
				desc = fmt.Sprintf("%dx%d grid, %d widgets", profile.Rows, profile.Cols, len(profile.Widgets))
			}
			items = append(items, TreeMenuItem{
				Label: fmt.Sprintf("%s (%s)", name, desc),
				Data:  name,
			})
		}
	}

	m.widgetsProfileMenu = NewTreeMenu(items)
	m.widgetsProfileMenu.SetTitle("Profiles")
}

// initWidgetsFilterMenu initializes the filter menu for current widget
func (m *Model) initWidgetsFilterMenu(widgetType config.WidgetType) {
	items := []TreeMenuItem{
		{Label: "All (no filter)", Data: ""},
	}

	// Add project-based filters
	if m.state.Projects != nil {
		for _, proj := range m.state.Projects.Projects {
			items = append(items, TreeMenuItem{
				Label: fmt.Sprintf("Project: %s", proj.Name),
				Data:  proj.ID,
			})
		}
	}

	// Add type-specific filters
	switch widgetType {
	case config.WidgetLogs:
		items = append(items,
			TreeMenuItem{Label: "─────────", Data: "", Disabled: true},
			TreeMenuItem{Label: "Errors only", Data: "level:error"},
			TreeMenuItem{Label: "Warnings and above", Data: "level:warn"},
			TreeMenuItem{Label: "Info and above", Data: "level:info"},
		)
	case config.WidgetClaudeSessions:
		// Add session selection
		if m.state.Claude != nil {
			items = append(items, TreeMenuItem{Label: "─────────", Data: "", Disabled: true})
			for _, sess := range m.state.Claude.Sessions {
				name := sess.Name
				if name == "" && len(sess.ID) >= 8 {
					name = sess.ID[:8]
				}
				items = append(items, TreeMenuItem{
					Label: fmt.Sprintf("Session: %s", name),
					Data:  "session:" + sess.ID,
				})
			}
		}
	}

	m.widgetsFilterMenu = NewTreeMenu(items)
	m.widgetsFilterMenu.SetTitle("Filters")
}

// handleWidgetsConfigEnter handles Enter key in config mode
func (m *Model) handleWidgetsConfigEnter() tea.Cmd {
	switch m.widgetsConfigStep {
	case "grid":
		// Grid size selected, get the selection
		if item := m.widgetsGridMenu.SelectedItem(); item != nil {
			if size, ok := item.Data.([2]int); ok {
				m.widgetsConfigRows = size[0]
				m.widgetsConfigCols = size[1]
			}
		}
		// Move to widget configuration
		m.widgetsConfigStep = "widgets"
		m.widgetsConfigCell = 0
		return nil

	case "widgets":
		// Widget type selected for current cell
		if item := m.widgetsTypeMenu.SelectedItem(); item != nil {
			if wtype, ok := item.Data.(config.WidgetType); ok {
				m.setWidgetTypeForCell(m.widgetsConfigCell, wtype)
				// Move to filter configuration for this widget
				m.initWidgetsFilterMenu(wtype)
				m.widgetsConfigStep = "filters"
			}
		}
		return nil

	case "filters":
		// Filter selected for current widget
		if item := m.widgetsFilterMenu.SelectedItem(); item != nil {
			if filter, ok := item.Data.(string); ok {
				m.setWidgetFilterForCell(m.widgetsConfigCell, filter)
			}
		}
		// Move to next cell or finish
		m.widgetsConfigCell++
		totalCells := m.widgetsConfigRows * m.widgetsConfigCols
		if m.widgetsConfigCell >= totalCells {
			// Done configuring, save and exit
			m.saveWidgetProfile()
			m.widgetsConfigMode = false
		} else {
			m.widgetsConfigStep = "widgets"
		}
		return nil

	case "profile_name":
		// Profile name entered
		if m.widgetsCreatingNew {
			m.createNewWidgetProfile(m.widgetsNewName)
			m.widgetsCreatingNew = false
		} else if m.widgetsRenaming {
			m.renameCurrentWidgetProfile(m.widgetsNewName)
			m.widgetsRenaming = false
		}
		m.widgetsConfigStep = "grid"
		m.widgetsNewName = ""
		return nil
	}
	return nil
}

// setWidgetTypeForCell sets the widget type for a specific cell
func (m *Model) setWidgetTypeForCell(cellIndex int, wtype config.WidgetType) {
	cfg := config.GetGlobal()
	if cfg == nil {
		return
	}

	profileName := m.getActiveWidgetProfile()
	profile := cfg.WidgetProfiles[profileName]
	if profile == nil {
		// Create new profile
		profile = &config.WidgetProfile{
			Name: profileName,
			Rows: m.widgetsConfigRows,
			Cols: m.widgetsConfigCols,
		}
		if cfg.WidgetProfiles == nil {
			cfg.WidgetProfiles = make(map[string]*config.WidgetProfile)
		}
		cfg.WidgetProfiles[profileName] = profile
	}

	// Update grid size
	profile.Rows = m.widgetsConfigRows
	profile.Cols = m.widgetsConfigCols

	// Calculate row/col from cell index
	row := cellIndex / m.widgetsConfigCols
	col := cellIndex % m.widgetsConfigCols

	// Find or create widget for this position
	found := false
	for i := range profile.Widgets {
		if profile.Widgets[i].Row == row && profile.Widgets[i].Col == col {
			profile.Widgets[i].Type = string(wtype)
			profile.Widgets[i].Title = ""
			found = true
			break
		}
	}
	if !found {
		profile.Widgets = append(profile.Widgets, config.WidgetConfig{
			ID:   fmt.Sprintf("w%d", cellIndex+1),
			Type: string(wtype),
			Row:  row,
			Col:  col,
		})
	}
}

// setWidgetFilterForCell sets the filter for a widget at a specific cell
func (m *Model) setWidgetFilterForCell(cellIndex int, filter string) {
	cfg := config.GetGlobal()
	if cfg == nil {
		return
	}

	profileName := m.getActiveWidgetProfile()
	profile := cfg.WidgetProfiles[profileName]
	if profile == nil {
		return
	}

	row := cellIndex / m.widgetsConfigCols
	col := cellIndex % m.widgetsConfigCols

	for i := range profile.Widgets {
		if profile.Widgets[i].Row == row && profile.Widgets[i].Col == col {
			// Parse filter string
			if strings.HasPrefix(filter, "level:") {
				profile.Widgets[i].LogLevelFilter = strings.TrimPrefix(filter, "level:")
			} else if strings.HasPrefix(filter, "session:") {
				profile.Widgets[i].SessionID = strings.TrimPrefix(filter, "session:")
			} else if filter != "" {
				profile.Widgets[i].ProjectFilter = filter
			} else {
				// Clear all filters
				profile.Widgets[i].ProjectFilter = ""
				profile.Widgets[i].LogLevelFilter = ""
				profile.Widgets[i].SessionID = ""
			}
			break
		}
	}
}

// saveWidgetProfile saves the current widget profile to config
func (m *Model) saveWidgetProfile() {
	_ = config.SaveGlobal()
}

// startNewWidgetProfile initiates new profile creation
func (m *Model) startNewWidgetProfile() {
	m.widgetsCreatingNew = true
	m.widgetsNewName = ""
	m.widgetsConfigStep = "profile_name"
	m.widgetsConfigMode = true
}

// createNewWidgetProfile creates a new widget profile
func (m *Model) createNewWidgetProfile(name string) {
	if name == "" {
		return
	}

	cfg := config.GetGlobal()
	if cfg == nil {
		return
	}

	if cfg.WidgetProfiles == nil {
		cfg.WidgetProfiles = make(map[string]*config.WidgetProfile)
	}

	// Create new profile with default 2x2 grid
	cfg.WidgetProfiles[name] = &config.WidgetProfile{
		Name: name,
		Rows: 2,
		Cols: 2,
		Widgets: []config.WidgetConfig{
			{ID: "w1", Type: string(config.WidgetDashStats), Row: 0, Col: 0, ColSpan: 2},
			{ID: "w2", Type: string(config.WidgetProcesses), Row: 1, Col: 0},
			{ID: "w3", Type: string(config.WidgetLogs), Row: 1, Col: 1},
		},
	}

	// Set as active
	if cfg.Settings != nil {
		cfg.Settings.ActiveWidgetProfile = name
	}

	_ = config.SaveGlobal()
	m.initWidgetsProfileMenu()
}

// deleteWidgetProfile deletes the current widget profile
func (m *Model) deleteWidgetProfile() {
	cfg := config.GetGlobal()
	if cfg == nil || cfg.WidgetProfiles == nil {
		return
	}

	profileName := m.getActiveWidgetProfile()

	// Don't delete if it's the only profile
	if len(cfg.WidgetProfiles) <= 1 {
		return
	}

	// Delete the profile
	delete(cfg.WidgetProfiles, profileName)

	// Switch to another profile
	for name := range cfg.WidgetProfiles {
		if cfg.Settings != nil {
			cfg.Settings.ActiveWidgetProfile = name
		}
		break
	}

	_ = config.SaveGlobal()
	m.initWidgetsProfileMenu()
}

// startRenameWidgetProfile initiates profile rename
func (m *Model) startRenameWidgetProfile() {
	m.widgetsRenaming = true
	m.widgetsNewName = m.getActiveWidgetProfile()
	m.widgetsConfigStep = "profile_name"
	m.widgetsConfigMode = true
}

// renameCurrentWidgetProfile renames the current profile
func (m *Model) renameCurrentWidgetProfile(newName string) {
	if newName == "" {
		return
	}

	cfg := config.GetGlobal()
	if cfg == nil || cfg.WidgetProfiles == nil {
		return
	}

	oldName := m.getActiveWidgetProfile()
	if oldName == newName {
		return
	}

	// Get the profile
	profile := cfg.WidgetProfiles[oldName]
	if profile == nil {
		return
	}

	// Rename
	profile.Name = newName
	delete(cfg.WidgetProfiles, oldName)
	cfg.WidgetProfiles[newName] = profile

	// Update active profile
	if cfg.Settings != nil {
		cfg.Settings.ActiveWidgetProfile = newName
	}

	_ = config.SaveGlobal()
	m.initWidgetsProfileMenu()
}

// renderWidgetsConfigOverlay renders the configuration overlay
func (m *Model) renderWidgetsConfigOverlay(baseContent string, width, height int) string {
	// Determine overlay content based on current step
	var title string
	var content string
	var menu *TreeMenu

	switch m.widgetsConfigStep {
	case "grid":
		title = "Select Grid Size"
		menu = m.widgetsGridMenu
	case "widgets":
		cellRow := m.widgetsConfigCell / m.widgetsConfigCols
		cellCol := m.widgetsConfigCell % m.widgetsConfigCols
		title = fmt.Sprintf("Widget for Cell [%d,%d]", cellRow+1, cellCol+1)
		menu = m.widgetsTypeMenu
	case "filters":
		title = "Select Filter"
		menu = m.widgetsFilterMenu
	case "profile_name":
		if m.widgetsCreatingNew {
			title = "New Profile Name"
		} else {
			title = "Rename Profile"
		}
		content = m.renderProfileNameInput()
	}

	// Render menu if available
	if menu != nil {
		menuWidth := 40
		menuHeight := 12
		menu.SetSize(menuWidth, menuHeight)
		menu.SetFocused(true)
		content = menu.Render()
	}

	// Create overlay box
	overlayWidth := 50
	overlayHeight := 16

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorSecondary).
		Padding(0, 1)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Padding(1, 2).
		Width(overlayWidth - 4).
		Height(overlayHeight - 4)

	// Footer hints
	footerStyle := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Padding(0, 1)

	var footerText string
	if m.widgetsConfigStep == "profile_name" {
		footerText = "Enter: confirm  Esc: cancel"
	} else {
		footerText = "↑↓: select  Enter: confirm  Esc: cancel"
	}

	overlay := lipgloss.JoinVertical(lipgloss.Left,
		titleStyle.Render("≡ "+title),
		content,
		footerStyle.Render(footerText),
	)

	overlay = boxStyle.Render(overlay)

	// Center overlay on screen
	overlayX := (width - overlayWidth) / 2
	overlayY := (height - overlayHeight) / 2

	// Overlay on base content
	return m.overlayBox(baseContent, overlay, overlayX, overlayY, width, height)
}

// renderProfileNameInput renders the profile name input field
func (m *Model) renderProfileNameInput() string {
	inputStyle := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(ColorSecondary).
		Padding(0, 1).
		Width(30)

	cursorStyle := lipgloss.NewStyle().
		Background(ColorPrimary).
		Foreground(ColorText)

	// Show input with cursor
	display := m.widgetsNewName + cursorStyle.Render(" ")
	return inputStyle.Render(display)
}

// overlayBox places an overlay box on top of base content
func (m *Model) overlayBox(base, overlay string, x, y, width, height int) string {
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")

	// Ensure base has enough lines
	for len(baseLines) < height {
		baseLines = append(baseLines, strings.Repeat(" ", width))
	}

	// Overlay each line
	for i, overlayLine := range overlayLines {
		lineY := y + i
		if lineY >= 0 && lineY < len(baseLines) {
			baseLine := baseLines[lineY]
			baseRunes := []rune(baseLine)

			// Ensure base line is long enough
			for len(baseRunes) < width {
				baseRunes = append(baseRunes, ' ')
			}

			// Overlay the content
			overlayRunes := []rune(overlayLine)
			for j, r := range overlayRunes {
				posX := x + j
				if posX >= 0 && posX < len(baseRunes) {
					baseRunes[posX] = r
				}
			}

			baseLines[lineY] = string(baseRunes)
		}
	}

	return strings.Join(baseLines, "\n")
}

// handleWidgetsConfigNavigation handles navigation in config mode
func (m *Model) handleWidgetsConfigNavigation(msg tea.KeyMsg) bool {
	if !m.widgetsConfigMode {
		return false
	}

	key := msg.String()

	// Handle text input for profile name
	if m.widgetsConfigStep == "profile_name" {
		switch key {
		case "backspace":
			if len(m.widgetsNewName) > 0 {
				m.widgetsNewName = m.widgetsNewName[:len(m.widgetsNewName)-1]
			}
			return true
		case "esc":
			m.widgetsConfigMode = false
			m.widgetsCreatingNew = false
			m.widgetsRenaming = false
			return true
		default:
			// Add character if printable
			if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
				m.widgetsNewName += key
				return true
			}
		}
		return false
	}

	// Handle menu navigation
	var menu *TreeMenu
	switch m.widgetsConfigStep {
	case "grid":
		menu = m.widgetsGridMenu
	case "widgets":
		menu = m.widgetsTypeMenu
	case "filters":
		menu = m.widgetsFilterMenu
	}

	if menu == nil {
		return false
	}

	switch key {
	case "up", "k":
		menu.MoveUp()
		return true
	case "down", "j":
		menu.MoveDown()
		return true
	case "esc":
		// Go back or exit
		switch m.widgetsConfigStep {
		case "filters":
			m.widgetsConfigStep = "widgets"
		case "widgets":
			m.widgetsConfigStep = "grid"
		default:
			m.widgetsConfigMode = false
		}
		return true
	}

	return false
}

// getAvailableProfiles returns a list of available profile names
func (m *Model) getAvailableProfiles() []string {
	cfg := config.GetGlobal()
	if cfg == nil || cfg.WidgetProfiles == nil {
		return []string{"default"}
	}

	var names []string
	for name := range cfg.WidgetProfiles {
		names = append(names, name)
	}

	// Sort for consistent ordering
	for i := 0; i < len(names)-1; i++ {
		for j := i + 1; j < len(names); j++ {
			if names[i] > names[j] {
				names[i], names[j] = names[j], names[i]
			}
		}
	}

	return names
}
