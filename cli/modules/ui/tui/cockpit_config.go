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
	{config.WidgetDatabaseSessions, "Database Sessions", "Database connections and sessions"},
}

// initCockpitConfigMenus initializes the TreeMenus for widget configuration
func (m *Model) initCockpitConfigMenus() {
	// Get current profile settings
	profile := m.getCockpitProfile(m.getActiveCockpitProfile())
	if profile != nil {
		m.cockpitConfigRows = profile.Rows
		m.cockpitConfigCols = profile.Cols
	} else {
		m.cockpitConfigRows = 2
		m.cockpitConfigCols = 2
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
	m.cockpitGridMenu = NewTreeMenu(gridItems)
	m.cockpitGridMenu.SetTitle("Grid Size")

	// Initialize widget type menu
	typeItems := []TreeMenuItem{}
	for _, opt := range widgetTypeOptions {
		typeItems = append(typeItems, TreeMenuItem{
			Label: fmt.Sprintf("%s - %s", opt.Label, opt.Desc),
			Data:  opt.Type,
		})
	}
	m.cockpitTypeMenu = NewTreeMenu(typeItems)
	m.cockpitTypeMenu.SetTitle("Widget Type")

	// Initialize profile menu
	m.initCockpitProfileMenu()
}

// initCockpitProfileMenu initializes the profile selection menu
func (m *Model) initCockpitProfileMenu() {
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

	m.cockpitProfileMenu = NewTreeMenu(items)
	m.cockpitProfileMenu.SetTitle("Profiles")
}

// initCockpitFilterMenu initializes the filter menu for current widget
func (m *Model) initCockpitFilterMenu(widgetType config.WidgetType) {
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

	m.cockpitFilterMenu = NewTreeMenu(items)
	m.cockpitFilterMenu.SetTitle("Filters")
}

// handleCockpitConfigEnter handles Enter key in config mode
func (m *Model) handleCockpitConfigEnter() tea.Cmd {
	switch m.cockpitConfigStep {
	case "grid":
		// Grid size selected, get the selection
		if item := m.cockpitGridMenu.SelectedItem(); item != nil {
			if size, ok := item.Data.([2]int); ok {
				m.cockpitConfigRows = size[0]
				m.cockpitConfigCols = size[1]
			}
		}
		// Move to widget configuration
		m.cockpitConfigStep = "widgets"
		m.cockpitConfigCell = 0
		return nil

	case "widgets":
		// Widget type selected for current cell
		if item := m.cockpitTypeMenu.SelectedItem(); item != nil {
			if wtype, ok := item.Data.(config.WidgetType); ok {
				m.setWidgetTypeForCell(m.cockpitConfigCell, wtype)
				// Move to filter configuration for this widget
				m.initCockpitFilterMenu(wtype)
				m.cockpitConfigStep = "filters"
			}
		}
		return nil

	case "filters":
		// Filter selected for current widget
		if item := m.cockpitFilterMenu.SelectedItem(); item != nil {
			if filter, ok := item.Data.(string); ok {
				m.setWidgetFilterForCell(m.cockpitConfigCell, filter)
			}
		}
		// Move to next cell or finish
		m.cockpitConfigCell++
		totalCells := m.cockpitConfigRows * m.cockpitConfigCols
		if m.cockpitConfigCell >= totalCells {
			// Done configuring, save and exit
			m.saveCockpitProfile()
			m.cockpitConfigMode = false
		} else {
			m.cockpitConfigStep = "widgets"
		}
		return nil

	case "profile_name":
		// Profile name entered
		if m.cockpitCreatingNew {
			m.createNewCockpitProfile(m.cockpitNewName)
			m.cockpitCreatingNew = false
		} else if m.cockpitRenaming {
			m.renameCurrentCockpitProfile(m.cockpitNewName)
			m.cockpitRenaming = false
		}
		m.cockpitConfigStep = "grid"
		m.cockpitNewName = ""
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

	profileName := m.getActiveCockpitProfile()
	profile := cfg.WidgetProfiles[profileName]
	if profile == nil {
		// Create new profile
		profile = &config.WidgetProfile{
			Name: profileName,
			Rows: m.cockpitConfigRows,
			Cols: m.cockpitConfigCols,
		}
		if cfg.WidgetProfiles == nil {
			cfg.WidgetProfiles = make(map[string]*config.WidgetProfile)
		}
		cfg.WidgetProfiles[profileName] = profile
	}

	// Update grid size
	profile.Rows = m.cockpitConfigRows
	profile.Cols = m.cockpitConfigCols

	// Calculate row/col from cell index
	row := cellIndex / m.cockpitConfigCols
	col := cellIndex % m.cockpitConfigCols

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

	profileName := m.getActiveCockpitProfile()
	profile := cfg.WidgetProfiles[profileName]
	if profile == nil {
		return
	}

	row := cellIndex / m.cockpitConfigCols
	col := cellIndex % m.cockpitConfigCols

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

// saveCockpitProfile saves the current widget profile to config
func (m *Model) saveCockpitProfile() {
	_ = config.SaveGlobal()
}

// startNewCockpitProfile initiates new profile creation
func (m *Model) startNewCockpitProfile() {
	m.cockpitCreatingNew = true
	m.cockpitNewName = ""
	m.cockpitConfigStep = "profile_name"
	m.cockpitConfigMode = true
}

// createNewCockpitProfile creates a new widget profile
func (m *Model) createNewCockpitProfile(name string) {
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
	m.initCockpitProfileMenu()
}

// deleteCockpitProfile deletes the current widget profile
func (m *Model) deleteCockpitProfile() {
	cfg := config.GetGlobal()
	if cfg == nil || cfg.WidgetProfiles == nil {
		return
	}

	profileName := m.getActiveCockpitProfile()

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
	m.initCockpitProfileMenu()
}

// startRenameCockpitProfile initiates profile rename
func (m *Model) startRenameCockpitProfile() {
	m.cockpitRenaming = true
	m.cockpitNewName = m.getActiveCockpitProfile()
	m.cockpitConfigStep = "profile_name"
	m.cockpitConfigMode = true
}

// renameCurrentCockpitProfile renames the current profile
func (m *Model) renameCurrentCockpitProfile(newName string) {
	if newName == "" {
		return
	}

	cfg := config.GetGlobal()
	if cfg == nil || cfg.WidgetProfiles == nil {
		return
	}

	oldName := m.getActiveCockpitProfile()
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
	m.initCockpitProfileMenu()
}

// renderCockpitConfigOverlay renders the configuration overlay
func (m *Model) renderCockpitConfigOverlay(width, height int) string {
	// Determine overlay content based on current step
	var title string
	var content string
	var menu *TreeMenu

	switch m.cockpitConfigStep {
	case "grid":
		title = "Select Grid Size"
		menu = m.cockpitGridMenu
	case "widgets":
		cellRow := m.cockpitConfigCell / m.cockpitConfigCols
		cellCol := m.cockpitConfigCell % m.cockpitConfigCols
		title = fmt.Sprintf("Widget for Cell [%d,%d]", cellRow+1, cellCol+1)
		menu = m.cockpitTypeMenu
	case "filters":
		title = "Select Filter"
		menu = m.cockpitFilterMenu
	case "profile_name":
		if m.cockpitCreatingNew {
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

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorSecondary).
		Padding(0, 1)

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorPrimary).
		Background(ColorBgAlt).
		Padding(1, 2).
		Width(overlayWidth - 4)

	// Footer hints
	footerStyle := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Padding(0, 1)

	var footerText string
	if m.cockpitConfigStep == "profile_name" {
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

	// Center overlay on screen using lipgloss.Place
	return lipgloss.Place(
		width,
		height,
		lipgloss.Center,
		lipgloss.Center,
		overlay,
		lipgloss.WithWhitespaceBackground(ColorBg),
	)
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
	display := m.cockpitNewName + cursorStyle.Render(" ")
	return inputStyle.Render(display)
}

// handleCockpitConfigNavigation handles special keys in config mode (Enter, Esc, text input)
// Up/Down navigation is handled by getActiveTreeMenu() in the standard navigation flow
func (m *Model) handleCockpitConfigNavigation(msg tea.KeyMsg) bool {
	if !m.cockpitConfigMode {
		return false
	}

	key := msg.String()

	// Handle text input for profile name
	if m.cockpitConfigStep == "profile_name" {
		switch key {
		case "enter":
			// Confirm profile name
			if m.cockpitNewName != "" {
				if m.cockpitCreatingNew {
					m.createNewCockpitProfile(m.cockpitNewName)
					m.cockpitCreatingNew = false
				} else if m.cockpitRenaming {
					m.renameCurrentCockpitProfile(m.cockpitNewName)
					m.cockpitRenaming = false
				}
			}
			m.cockpitConfigMode = false
			m.cockpitConfigStep = ""
			m.cockpitNewName = ""
			return true
		case "backspace":
			if len(m.cockpitNewName) > 0 {
				m.cockpitNewName = m.cockpitNewName[:len(m.cockpitNewName)-1]
			}
			return true
		case "esc":
			m.cockpitConfigMode = false
			m.cockpitCreatingNew = false
			m.cockpitRenaming = false
			m.cockpitNewName = ""
			return true
		default:
			// Add character if printable
			if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
				m.cockpitNewName += key
				return true
			}
		}
		return false
	}

	// Handle Enter and Esc for menu navigation (Up/Down handled by getActiveTreeMenu)
	switch key {
	case "enter":
		// Handle menu selection
		m.handleCockpitConfigEnter()
		return true
	case "esc":
		// Go back or exit
		switch m.cockpitConfigStep {
		case "filters":
			m.cockpitConfigStep = "widgets"
		case "widgets":
			m.cockpitConfigStep = "grid"
		default:
			m.cockpitConfigMode = false
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
