package tui

import (
	"strings"
	"time"

	"csd-devtrack/cli/modules/ui/core"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// FocusArea represents which area has focus
type FocusArea int

const (
	FocusSidebar FocusArea = iota
	FocusMain
	FocusDetail
)

// Model is the main Bubble Tea model for the TUI
type Model struct {
	// Core
	presenter core.Presenter
	state     *core.AppState
	keys      KeyMap

	// UI state
	width       int
	height      int
	ready       bool
	currentView core.ViewModelType

	// Focus management
	focusArea     FocusArea
	sidebarIndex  int // Selected item in sidebar (0-6 for views)
	mainIndex     int // Selected item in main list
	detailIndex   int // Selected item in detail panel
	maxMainItems  int // Total items in main list
	maxDetailItems int

	// Scroll state
	mainScrollOffset   int
	detailScrollOffset int
	visibleMainRows    int
	visibleDetailRows  int

	// View-specific state
	filterText    string
	filterActive  bool
	showHelp      bool
	showDialog    bool
	dialogType    string
	dialogMessage string
	dialogConfirm bool

	// Log filtering
	logLevelFilter  string // "", "error", "warn", "info", "debug"
	logSearchText   string
	logSearchActive bool

	// Build profiles
	currentBuildProfile string // "dev", "test", "prod"

	// Components
	help     help.Model
	spinner  spinner.Model
	viewport viewport.Model

	// Notifications
	notifications []*core.Notification
	notifyTimer   *time.Timer

	// Errors
	lastError     string
	lastErrorTime time.Time
}

// NewModel creates a new TUI model
func NewModel(presenter core.Presenter) *Model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ColorPrimary)

	h := help.New()
	h.ShowAll = false
	h.Styles.ShortKey = HelpKeyStyle
	h.Styles.ShortDesc = HelpDescStyle
	h.Styles.ShortSeparator = HelpDescStyle

	return &Model{
		presenter:           presenter,
		state:               core.NewAppState(),
		keys:                DefaultKeyMap(),
		currentView:         core.VMDashboard,
		focusArea:           FocusSidebar,
		sidebarIndex:        0,
		help:                h,
		spinner:             s,
		notifications:       make([]*core.Notification, 0),
		visibleMainRows:     10,
		visibleDetailRows:   5,
		currentBuildProfile: "dev", // Default to dev profile
	}
}

// Init initializes the model
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.refreshData,
		tea.WindowSize(),
	)
}

// Update handles messages
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		m.visibleMainRows = m.height - 10
		m.visibleDetailRows = m.height - 15

		headerHeight := 3
		footerHeight := 3
		sidebarWidth := getSidebarWidth()
		m.viewport = viewport.New(m.width-sidebarWidth-4, m.height-headerHeight-footerHeight)
		m.viewport.YPosition = headerHeight

	case tea.KeyMsg:
		if m.logSearchActive {
			cmd := m.handleLogSearchInput(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		} else if m.filterActive {
			cmd := m.handleFilterInput(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		} else if m.showDialog {
			cmd := m.handleDialogKey(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		} else if m.showHelp {
			m.showHelp = false
		} else {
			cmd := m.handleKeyPress(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case stateUpdateMsg:
		m.handleStateUpdate(msg.update)

	case notificationMsg:
		m.handleNotification(msg.notification)

	case refreshMsg:
		cmds = append(cmds, m.refreshData)

	case errMsg:
		m.lastError = msg.Error()
		m.lastErrorTime = time.Now()

	case tickMsg:
		cmds = append(cmds, m.refreshData, tickCmd())
	}

	return m, tea.Batch(cmds...)
}

// View renders the UI
func (m Model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	header := m.renderHeader()
	sidebar := m.renderSidebar()
	main := m.renderMainContent()
	footer := m.renderFooter()

	// Combine sidebar and main
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, main)

	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

// handleKeyPress processes keyboard input
func (m *Model) handleKeyPress(msg tea.KeyMsg) tea.Cmd {
	switch {
	// Quit
	case key.Matches(msg, m.keys.Quit):
		return tea.Quit

	// Help toggle
	case key.Matches(msg, m.keys.Help):
		m.showHelp = !m.showHelp
		return nil

	// Focus navigation (Tab/Shift+Tab)
	case key.Matches(msg, m.keys.Tab):
		m.cycleFocus(1)
		return nil
	case key.Matches(msg, m.keys.ShiftTab):
		m.cycleFocus(-1)
		return nil

	// View shortcuts (1-7)
	case key.Matches(msg, m.keys.View1):
		return m.selectView(0)
	case key.Matches(msg, m.keys.View2):
		return m.selectView(1)
	case key.Matches(msg, m.keys.View3):
		return m.selectView(2)
	case key.Matches(msg, m.keys.View4):
		return m.selectView(3)
	case key.Matches(msg, m.keys.View5):
		return m.selectView(4)
	case key.Matches(msg, m.keys.View6):
		return m.selectView(5)
	case key.Matches(msg, m.keys.View7):
		return m.selectView(6)

	// Directional navigation
	case key.Matches(msg, m.keys.Up):
		m.navigateUp()
		return nil
	case key.Matches(msg, m.keys.Down):
		m.navigateDown()
		return nil
	case key.Matches(msg, m.keys.Left):
		m.navigateLeft()
		return nil
	case key.Matches(msg, m.keys.Right):
		m.navigateRight()
		return nil

	// Page navigation
	case key.Matches(msg, m.keys.PageUp):
		m.pageUp()
		return nil
	case key.Matches(msg, m.keys.PageDown):
		m.pageDown()
		return nil
	case key.Matches(msg, m.keys.Home):
		m.goToStart()
		return nil
	case key.Matches(msg, m.keys.End):
		m.goToEnd()
		return nil

	// Enter - select/activate
	case key.Matches(msg, m.keys.Enter):
		return m.handleEnter()

	// Refresh
	case key.Matches(msg, m.keys.Refresh):
		return m.refreshData

	// Filter
	case key.Matches(msg, m.keys.Filter):
		m.filterActive = true
		m.filterText = ""
		return nil

	// Actions - context dependent
	default:
		return m.handleActionKey(msg)
	}
}

// cycleFocus cycles through focus areas
func (m *Model) cycleFocus(direction int) {
	numAreas := 2 // sidebar and main (detail only if visible)
	if m.hasDetailPanel() {
		numAreas = 3
	}

	newFocus := int(m.focusArea) + direction
	if newFocus < 0 {
		newFocus = numAreas - 1
	} else if newFocus >= numAreas {
		newFocus = 0
	}
	m.focusArea = FocusArea(newFocus)
}

// hasDetailPanel returns true if the current view has a detail panel
func (m *Model) hasDetailPanel() bool {
	switch m.currentView {
	case core.VMProjects, core.VMGit, core.VMProcesses:
		return true
	default:
		return false
	}
}

// navigateUp moves selection up in current focus area
func (m *Model) navigateUp() {
	switch m.focusArea {
	case FocusSidebar:
		if m.sidebarIndex > 0 {
			m.sidebarIndex--
		}
	case FocusMain:
		if m.mainIndex > 0 {
			m.mainIndex--
			m.ensureMainVisible()
		}
	case FocusDetail:
		if m.detailIndex > 0 {
			m.detailIndex--
			m.ensureDetailVisible()
		}
	}
}

// navigateDown moves selection down in current focus area
func (m *Model) navigateDown() {
	switch m.focusArea {
	case FocusSidebar:
		if m.sidebarIndex < 6 { // 7 views (0-6)
			m.sidebarIndex++
		}
	case FocusMain:
		if m.mainIndex < m.maxMainItems-1 {
			m.mainIndex++
			m.ensureMainVisible()
		}
	case FocusDetail:
		if m.detailIndex < m.maxDetailItems-1 {
			m.detailIndex++
			m.ensureDetailVisible()
		}
	}
}

// navigateLeft moves focus or selection left
func (m *Model) navigateLeft() {
	if m.focusArea > FocusSidebar {
		m.focusArea--
	}
}

// navigateRight moves focus or selection right
func (m *Model) navigateRight() {
	maxFocus := FocusMain
	if m.hasDetailPanel() {
		maxFocus = FocusDetail
	}
	if m.focusArea < maxFocus {
		m.focusArea++
	}
}

// pageUp scrolls up one page
func (m *Model) pageUp() {
	switch m.focusArea {
	case FocusMain:
		m.mainIndex -= m.visibleMainRows
		if m.mainIndex < 0 {
			m.mainIndex = 0
		}
		m.ensureMainVisible()
	case FocusDetail:
		m.detailIndex -= m.visibleDetailRows
		if m.detailIndex < 0 {
			m.detailIndex = 0
		}
		m.ensureDetailVisible()
	}
}

// pageDown scrolls down one page
func (m *Model) pageDown() {
	switch m.focusArea {
	case FocusMain:
		m.mainIndex += m.visibleMainRows
		if m.mainIndex >= m.maxMainItems {
			m.mainIndex = m.maxMainItems - 1
		}
		if m.mainIndex < 0 {
			m.mainIndex = 0
		}
		m.ensureMainVisible()
	case FocusDetail:
		m.detailIndex += m.visibleDetailRows
		if m.detailIndex >= m.maxDetailItems {
			m.detailIndex = m.maxDetailItems - 1
		}
		if m.detailIndex < 0 {
			m.detailIndex = 0
		}
		m.ensureDetailVisible()
	}
}

// goToStart goes to the start of the current list
func (m *Model) goToStart() {
	switch m.focusArea {
	case FocusSidebar:
		m.sidebarIndex = 0
	case FocusMain:
		m.mainIndex = 0
		m.mainScrollOffset = 0
	case FocusDetail:
		m.detailIndex = 0
		m.detailScrollOffset = 0
	}
}

// goToEnd goes to the end of the current list
func (m *Model) goToEnd() {
	switch m.focusArea {
	case FocusSidebar:
		m.sidebarIndex = 6
	case FocusMain:
		m.mainIndex = m.maxMainItems - 1
		if m.mainIndex < 0 {
			m.mainIndex = 0
		}
		m.ensureMainVisible()
	case FocusDetail:
		m.detailIndex = m.maxDetailItems - 1
		if m.detailIndex < 0 {
			m.detailIndex = 0
		}
		m.ensureDetailVisible()
	}
}

// ensureMainVisible adjusts scroll to keep selection visible
func (m *Model) ensureMainVisible() {
	if m.mainIndex < m.mainScrollOffset {
		m.mainScrollOffset = m.mainIndex
	} else if m.mainIndex >= m.mainScrollOffset+m.visibleMainRows {
		m.mainScrollOffset = m.mainIndex - m.visibleMainRows + 1
	}
}

// ensureDetailVisible adjusts scroll to keep selection visible
func (m *Model) ensureDetailVisible() {
	if m.detailIndex < m.detailScrollOffset {
		m.detailScrollOffset = m.detailIndex
	} else if m.detailIndex >= m.detailScrollOffset+m.visibleDetailRows {
		m.detailScrollOffset = m.detailIndex - m.visibleDetailRows + 1
	}
}

// selectView changes the current view
func (m *Model) selectView(index int) tea.Cmd {
	views := []core.ViewModelType{
		core.VMDashboard,
		core.VMProjects,
		core.VMBuild,
		core.VMProcesses,
		core.VMLogs,
		core.VMGit,
		core.VMConfig,
	}

	if index >= 0 && index < len(views) {
		m.sidebarIndex = index
		m.currentView = views[index]
		m.mainIndex = 0
		m.mainScrollOffset = 0
		m.detailIndex = 0
		m.detailScrollOffset = 0
		m.state.SetCurrentView(m.currentView)
		return m.sendEvent(core.NavigateEvent(m.currentView))
	}
	return nil
}

// handleEnter handles the Enter key based on focus
func (m *Model) handleEnter() tea.Cmd {
	switch m.focusArea {
	case FocusSidebar:
		return m.selectView(m.sidebarIndex)
	case FocusMain:
		// Depending on view, enter can mean different things
		switch m.currentView {
		case core.VMProjects:
			// Enter on a project -> go to build view for that project
			m.focusArea = FocusDetail
		case core.VMProcesses:
			// Enter on a process -> view logs
			return m.viewLogs()
		}
	}
	return nil
}

// handleActionKey handles action keys (b, r, s, etc.)
func (m *Model) handleActionKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()
	keyLower := strings.ToLower(key)

	// Quick view navigation shortcuts (work from anywhere, case insensitive)
	// These ALWAYS navigate to views - no exceptions
	switch keyLower {
	case "d":
		// d/D -> Dashboard
		return m.selectView(0)
	case "p":
		// p/P -> Projects
		return m.selectView(1)
	case "b":
		// b/B -> Build
		return m.selectView(2)
	case "o":
		// o/O -> prOcesses
		return m.selectView(3)
	case "l":
		// l/L -> Logs
		return m.selectView(4)
	case "g":
		// g/G -> Git
		return m.selectView(5)
	case "c":
		// c/C -> Config
		return m.selectView(6)
	}

	// Build view specific keys
	if m.currentView == core.VMBuild {
		switch key {
		case "1":
			m.currentBuildProfile = "dev"
			return nil
		case "2":
			m.currentBuildProfile = "test"
			return nil
		case "3":
			m.currentBuildProfile = "prod"
			return nil
		}
	}

	// Log view specific keys
	if m.currentView == core.VMLogs {
		switch key {
		case "/":
			m.logSearchActive = true
			return nil
		case "e":
			m.toggleLogLevel("error")
			return nil
		case "w":
			m.toggleLogLevel("warn")
			return nil
		case "i":
			m.toggleLogLevel("info")
			return nil
		case "a":
			m.logLevelFilter = "" // All
			return nil
		case "x":
			m.logSearchText = "" // Clear search
			return nil
		}
	}

	// Git view specific keys (use uppercase to avoid conflict with navigation)
	if m.currentView == core.VMGit {
		switch key {
		case "D": // Shift+D for diff
			return m.sendEvent(core.NewEvent(core.EventGitDiff).WithProject(m.getSelectedProjectID()))
		case "H": // H for history (log)
			return m.sendEvent(core.NewEvent(core.EventGitLog).WithProject(m.getSelectedProjectID()))
		}
	}

	// Action keys (use F-keys and special keys to avoid conflicts)
	switch key {
	case "f5":
		// F5 to build selected project
		return m.buildSelected()
	case "ctrl+b":
		// Ctrl+B to build all
		return m.buildAll()
	case "r":
		return m.runSelected()
	case "s":
		return m.stopSelected()
	case "ctrl+r":
		return m.restartSelected()
	case "k":
		m.dialogType = "kill"
		m.dialogMessage = "Kill the selected process?"
		m.showDialog = true
		return nil
	}
	return nil
}

// toggleLogLevel toggles or sets the log level filter
func (m *Model) toggleLogLevel(level string) {
	if m.logLevelFilter == level {
		m.logLevelFilter = "" // Toggle off
	} else {
		m.logLevelFilter = level
	}
}

// handleDialogKey handles keys when a dialog is open
func (m *Model) handleDialogKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "y", "Y", "enter":
		m.showDialog = false
		return m.handleDialogConfirm()
	case "n", "N", "esc":
		m.showDialog = false
	case "left", "right", "tab":
		m.dialogConfirm = !m.dialogConfirm
	}
	return nil
}

// handleDialogConfirm handles dialog confirmation
func (m *Model) handleDialogConfirm() tea.Cmd {
	switch m.dialogType {
	case "kill":
		return m.killSelected()
	}
	return nil
}

// handleLogSearchInput handles typing in log search mode
func (m *Model) handleLogSearchInput(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "enter":
		m.logSearchActive = false
		return nil
	case "esc":
		m.logSearchActive = false
		m.logSearchText = ""
		return nil
	case "backspace":
		if len(m.logSearchText) > 0 {
			m.logSearchText = m.logSearchText[:len(m.logSearchText)-1]
		}
	default:
		if len(msg.String()) == 1 {
			m.logSearchText += msg.String()
		}
	}
	return nil
}

// handleFilterInput handles typing in filter mode
func (m *Model) handleFilterInput(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "enter":
		m.filterActive = false
		// Apply filter
		return m.sendEvent(core.FilterEvent(m.filterText))
	case "esc":
		m.filterActive = false
		m.filterText = ""
		return nil
	case "backspace":
		if len(m.filterText) > 0 {
			m.filterText = m.filterText[:len(m.filterText)-1]
		}
	default:
		if len(msg.String()) == 1 {
			m.filterText += msg.String()
		}
	}
	return nil
}

// sendEvent sends an event to the presenter
func (m *Model) sendEvent(event *core.Event) tea.Cmd {
	return func() tea.Msg {
		if err := m.presenter.HandleEvent(event); err != nil {
			return errMsg{err}
		}
		return nil
	}
}

// handleStateUpdate handles state updates from presenter
func (m *Model) handleStateUpdate(update core.StateUpdate) {
	m.state.UpdateViewModel(update.ViewModel)
	// Update max items counts
	m.updateItemCounts()
}

// updateItemCounts updates the max item counts for navigation
func (m *Model) updateItemCounts() {
	switch m.currentView {
	case core.VMProjects:
		if m.state.Projects != nil {
			m.maxMainItems = len(m.state.Projects.Projects)
		}
	case core.VMProcesses:
		if m.state.Processes != nil {
			m.maxMainItems = len(m.state.Processes.Processes)
		}
	case core.VMGit:
		if m.state.Git != nil {
			m.maxMainItems = len(m.state.Git.Projects)
		}
	case core.VMDashboard:
		if m.state.Dashboard != nil {
			m.maxMainItems = len(m.state.Dashboard.Projects)
		}
	case core.VMBuild:
		if m.state.Builds != nil {
			m.maxMainItems = len(m.state.Builds.BuildHistory)
		}
	}
}

// handleNotification handles notifications
func (m *Model) handleNotification(n *core.Notification) {
	m.notifications = append(m.notifications, n)
	// Keep only last 5
	if len(m.notifications) > 5 {
		m.notifications = m.notifications[1:]
	}
}

// refreshData fetches fresh data
func (m *Model) refreshData() tea.Msg {
	if m.presenter != nil {
		_ = m.presenter.Refresh()
	}
	return refreshMsg{}
}

// Action helpers
func (m *Model) buildSelected() tea.Cmd {
	projectID := m.getSelectedProjectID()
	if projectID == "" {
		return nil
	}
	return m.sendEvent(core.NewEvent(core.EventStartBuild).WithProject(projectID))
}

func (m *Model) buildAll() tea.Cmd {
	return m.sendEvent(core.NewEvent(core.EventBuildAll))
}

func (m *Model) runSelected() tea.Cmd {
	projectID := m.getSelectedProjectID()
	if projectID == "" {
		return nil
	}
	return m.sendEvent(core.NewEvent(core.EventStartProcess).WithProject(projectID))
}

func (m *Model) stopSelected() tea.Cmd {
	projectID := m.getSelectedProjectID()
	if projectID == "" {
		return nil
	}
	return m.sendEvent(core.NewEvent(core.EventStopProcess).WithProject(projectID))
}

func (m *Model) restartSelected() tea.Cmd {
	projectID := m.getSelectedProjectID()
	if projectID == "" {
		return nil
	}
	return m.sendEvent(core.NewEvent(core.EventRestartProcess).WithProject(projectID))
}

func (m *Model) killSelected() tea.Cmd {
	projectID := m.getSelectedProjectID()
	if projectID == "" {
		return nil
	}
	return m.sendEvent(core.NewEvent(core.EventKillProcess).WithProject(projectID))
}

func (m *Model) viewLogs() tea.Cmd {
	projectID := m.getSelectedProjectID()
	if projectID == "" {
		return nil
	}
	m.currentView = core.VMLogs
	m.sidebarIndex = 4 // Logs view index
	return m.sendEvent(core.NewEvent(core.EventViewLogs).WithProject(projectID))
}

func (m *Model) getSelectedProjectID() string {
	switch m.currentView {
	case core.VMProjects, core.VMDashboard:
		projects := core.SelectProjects(m.state)
		if m.mainIndex >= 0 && m.mainIndex < len(projects) {
			return projects[m.mainIndex].ID
		}
	case core.VMProcesses:
		if m.state.Processes != nil && m.mainIndex >= 0 && m.mainIndex < len(m.state.Processes.Processes) {
			return m.state.Processes.Processes[m.mainIndex].ProjectID
		}
	case core.VMGit:
		if m.state.Git != nil && m.mainIndex >= 0 && m.mainIndex < len(m.state.Git.Projects) {
			return m.state.Git.Projects[m.mainIndex].ProjectID
		}
	}
	return ""
}

// Message types
type stateUpdateMsg struct {
	update core.StateUpdate
}

type notificationMsg struct {
	notification *core.Notification
}

type refreshMsg struct{}

type errMsg struct {
	error
}

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second*2, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
