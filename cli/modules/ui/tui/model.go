package tui

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"csd-devtrack/cli/modules/core/projects"
	"csd-devtrack/cli/modules/platform/config"
	"csd-devtrack/cli/modules/ui/core"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// BrowserEntry represents a directory entry in the file browser
type BrowserEntry struct {
	Name      string
	IsDir     bool
	IsProject bool // Has detectable project structure
	Path      string
}

// DetectedProjectInfo holds info about a detected project
type DetectedProjectInfo struct {
	Name       string
	Path       string
	Type       string   // full-stack, backend-only, etc.
	Components []string // agent, cli, backend, frontend
}

// GitFileEntry represents a file in git status
type GitFileEntry struct {
	Path   string // File path
	Status string // "staged", "modified", "untracked", "deleted"
}

// FocusArea represents which area has focus
type FocusArea int

const (
	FocusSidebar FocusArea = iota
	FocusMain
	FocusDetail
)

// ViewState holds the saved state for a view
type ViewState struct {
	FocusArea          FocusArea
	MainIndex          int
	DetailIndex        int
	MainScrollOffset   int
	DetailScrollOffset int
}

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

	// Per-view state preservation
	viewStates map[core.ViewModelType]*ViewState

	// View-specific state
	filterText    string
	filterActive  bool
	showHelp      bool
	showDialog    bool
	dialogType    string
	dialogMessage string
	dialogConfirm bool

	// Log filtering
	logLevelFilter   string // "", "error", "warn", "info", "debug"
	logSourceFilter  string // "", "project-id", "project-id/component"
	logTypeFilter    string // "", "build", "process"
	logSearchText    string
	logSearchActive  bool
	logSourceOptions []string // Available sources for selection
	logScrollOffset  int      // Scroll offset from bottom (0 = auto-scroll to bottom)
	logAutoScroll    bool     // Auto-scroll to bottom on new logs
	logPaused        bool     // Pause log display updates

	// Build profiles
	currentBuildProfile string // "dev", "test", "prod"

	// Config view - file browser state
	configMode      string   // "projects", "browser", "settings"
	browserPath     string   // Current directory path
	browserEntries  []BrowserEntry // Directory entries (uses mainIndex for selection)
	detectedProject *DetectedProjectInfo // Detected project in current dir

	// Pending actions (for dialogs)
	pendingRemovePath string // Path of project to remove (for confirmation dialog)

	// Git view state
	gitShowDiff       bool     // Showing diff view
	gitDiffContent    []string // Diff content lines
	gitFiles          []GitFileEntry // Flat list of all files for current project
	gitFilesProjectID string   // Project ID for which gitFiles was built

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

	// Get initial browser path (home directory)
	homeDir, _ := os.UserHomeDir()
	if homeDir == "" {
		homeDir, _ = os.Getwd()
	}

	// Create initial state and populate from presenter
	state := core.NewAppState()
	if presenter != nil {
		// Fetch initial state from presenter (already loaded)
		if vm, err := presenter.GetViewModel(core.VMDashboard); err == nil {
			if dashboard, ok := vm.(*core.DashboardVM); ok {
				state.Dashboard = dashboard
			}
		}
		if vm, err := presenter.GetViewModel(core.VMProjects); err == nil {
			if projects, ok := vm.(*core.ProjectsVM); ok {
				state.Projects = projects
			}
		}
		if vm, err := presenter.GetViewModel(core.VMProcesses); err == nil {
			if processes, ok := vm.(*core.ProcessesVM); ok {
				state.Processes = processes
			}
		}
		if vm, err := presenter.GetViewModel(core.VMLogs); err == nil {
			if logs, ok := vm.(*core.LogsVM); ok {
				state.Logs = logs
			}
		}
		if vm, err := presenter.GetViewModel(core.VMGit); err == nil {
			if git, ok := vm.(*core.GitVM); ok {
				state.Git = git
			}
		}
		if vm, err := presenter.GetViewModel(core.VMBuild); err == nil {
			if builds, ok := vm.(*core.BuildsVM); ok {
				state.Builds = builds
			}
		}
	}

	return &Model{
		presenter:           presenter,
		state:               state,
		keys:                DefaultKeyMap(),
		currentView:         core.VMDashboard,
		focusArea:           FocusSidebar,
		sidebarIndex:        0,
		help:                h,
		spinner:             s,
		notifications:       make([]*core.Notification, 0),
		visibleMainRows:     10,
		visibleDetailRows:   5,
		currentBuildProfile: "dev",   // Default to dev profile
		configMode:          "projects", // Start with projects view
		browserPath:         homeDir,
		viewStates:          make(map[core.ViewModelType]*ViewState),
		logAutoScroll:       true, // Auto-scroll logs by default
	}
}

// saveViewState saves the current view state
func (m *Model) saveViewState() {
	m.viewStates[m.currentView] = &ViewState{
		FocusArea:          m.focusArea,
		MainIndex:          m.mainIndex,
		DetailIndex:        m.detailIndex,
		MainScrollOffset:   m.mainScrollOffset,
		DetailScrollOffset: m.detailScrollOffset,
	}
}

// restoreViewState restores the saved state for a view
func (m *Model) restoreViewState(view core.ViewModelType) {
	if state, ok := m.viewStates[view]; ok {
		m.focusArea = state.FocusArea
		m.mainIndex = state.MainIndex
		m.detailIndex = state.DetailIndex
		m.mainScrollOffset = state.MainScrollOffset
		m.detailScrollOffset = state.DetailScrollOffset
	} else {
		// Default state for new views
		m.focusArea = FocusMain
		m.mainIndex = 0
		m.detailIndex = 0
		m.mainScrollOffset = 0
		m.detailScrollOffset = 0
	}

	// When restoring to detail panel for Git view, rebuild file list for navigation
	if m.focusArea == FocusDetail && view == core.VMGit {
		if m.state.Git != nil && m.mainIndex >= 0 && m.mainIndex < len(m.state.Git.Projects) {
			p := m.state.Git.Projects[m.mainIndex]
			m.buildGitFileList(&p)
		}
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
			// In search mode - handle typing
			if m.handleLogsSearchInput(msg) {
				return m, nil
			}
		} else if m.currentView == core.VMLogs && m.focusArea == FocusMain && !m.showDialog && !m.showHelp {
			// In Logs view but not in search mode - handle shortcuts
			if m.handleLogsShortcuts(msg) {
				return m, nil
			}
		}

		if m.logSearchActive {
			// Legacy handler (shouldn't reach here now)
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

	case gitDiffMsg:
		m.gitDiffContent = msg.lines
		m.gitShowDiff = true
		m.detailScrollOffset = 0
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
	// Handle Escape for context-specific exits
	if msg.String() == "esc" {
		// Git diff view - go back to file list
		if m.currentView == core.VMGit && m.gitShowDiff {
			m.gitShowDiff = false
			m.gitDiffContent = nil
			m.detailScrollOffset = 0
			return nil
		}
		// Focus detail -> back to main
		if m.focusArea == FocusDetail {
			m.focusArea = FocusMain
			return nil
		}
		return nil
	}

	// Handle Shift+Up/Down for page scrolling in git diff view
	if m.currentView == core.VMGit && m.gitShowDiff {
		switch msg.String() {
		case "shift+up":
			m.gitDiffPageUp()
			return nil
		case "shift+down":
			m.gitDiffPageDown()
			return nil
		}
	}

	switch {
	// Quit
	case key.Matches(msg, m.keys.Quit):
		return tea.Quit

	// Cancel current build/process (Ctrl+C)
	case key.Matches(msg, m.keys.Cancel):
		return m.cancelCurrent()

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

	// When entering detail panel for Git view, build the file list for navigation
	if m.focusArea == FocusDetail && m.currentView == core.VMGit {
		if m.state.Git != nil && m.mainIndex >= 0 && m.mainIndex < len(m.state.Git.Projects) {
			p := m.state.Git.Projects[m.mainIndex]
			m.buildGitFileList(&p)
		}
	}
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

// gitDiffPageUp scrolls the git diff view up by a page
func (m *Model) gitDiffPageUp() {
	m.detailScrollOffset -= m.visibleDetailRows
	if m.detailScrollOffset < 0 {
		m.detailScrollOffset = 0
	}
}

// gitDiffPageDown scrolls the git diff view down by a page
func (m *Model) gitDiffPageDown() {
	maxScroll := len(m.gitDiffContent) - m.visibleDetailRows
	if maxScroll < 0 {
		maxScroll = 0
	}
	m.detailScrollOffset += m.visibleDetailRows
	if m.detailScrollOffset > maxScroll {
		m.detailScrollOffset = maxScroll
	}
}

// navigateUp moves selection up in current focus area
func (m *Model) navigateUp() {
	// Special handling for git diff view - scroll the diff
	if m.currentView == core.VMGit && m.gitShowDiff {
		if m.detailScrollOffset > 0 {
			m.detailScrollOffset--
		}
		return
	}

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
	// Special handling for git diff view - scroll the diff
	if m.currentView == core.VMGit && m.gitShowDiff {
		maxScroll := len(m.gitDiffContent) - m.visibleDetailRows
		if maxScroll < 0 {
			maxScroll = 0
		}
		if m.detailScrollOffset < maxScroll {
			m.detailScrollOffset++
		}
		return
	}

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

// navigateLeft moves selection left (Config tabs only)
func (m *Model) navigateLeft() {
	// Only used for Config tabs - use Tab for panel switching
	if m.currentView == core.VMConfig {
		switch m.configMode {
		case "browser":
			m.configMode = "projects"
			m.mainIndex = 0
		case "settings":
			m.configMode = "browser"
			m.mainIndex = 0
			m.loadBrowserEntries()
		}
	}
}

// navigateRight moves selection right (Config tabs only)
func (m *Model) navigateRight() {
	// Only used for Config tabs - use Tab for panel switching
	if m.currentView == core.VMConfig {
		switch m.configMode {
		case "projects":
			m.configMode = "browser"
			m.mainIndex = 0
			m.loadBrowserEntries()
		case "browser":
			m.configMode = "settings"
			m.mainIndex = 0
		}
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
		newView := views[index]

		// Save current view state before switching
		m.saveViewState()

		// Switch view
		m.sidebarIndex = index
		m.currentView = newView

		// Restore saved state for new view
		m.restoreViewState(newView)

		// Initialize Config view
		if m.currentView == core.VMConfig {
			if m.configMode == "" {
				m.configMode = "projects"
			}
			if m.configMode == "browser" {
				m.loadBrowserEntries()
			}
		}

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
		case core.VMGit:
			// Git view - move focus to file list
			if m.state.Git != nil && m.mainIndex >= 0 && m.mainIndex < len(m.state.Git.Projects) {
				// Build git files for the selected project
				p := m.state.Git.Projects[m.mainIndex]
				m.buildGitFileList(&p)
			}
			m.focusArea = FocusDetail
			m.detailIndex = 0
			m.detailScrollOffset = 0
		case core.VMConfig:
			// Config view - depends on current tab
			if m.configMode == "browser" {
				m.enterBrowserDirectory()
			} else if m.configMode == "projects" {
				// Navigate to project in browser
				cfg := config.GetGlobal()
				if cfg != nil && m.mainIndex >= 0 && m.mainIndex < len(cfg.Projects) {
					proj := cfg.Projects[m.mainIndex]
					m.browserPath = proj.Path
					m.configMode = "browser"
					m.mainIndex = 0
					m.loadBrowserEntries()
				}
			}
		}
	case FocusDetail:
		// Detail panel Enter actions
		switch m.currentView {
		case core.VMGit:
			// Show diff for selected file
			return m.showGitFileDiff()
		}
	}
	return nil
}

// handleActionKey handles action keys (b, r, s, etc.)
func (m *Model) handleActionKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()

	// Quick view navigation shortcuts (uppercase only)
	// These ALWAYS navigate to views - no exceptions
	switch key {
	case "D":
		return m.selectView(0) // Dashboard
	case "P":
		return m.selectView(1) // Projects
	case "B":
		return m.selectView(2) // Build
	case "O":
		return m.selectView(3) // PrOcesses
	case "L":
		return m.selectView(4) // Logs
	case "G":
		return m.selectView(5) // Git
	case "C":
		return m.selectView(6) // Config
	}

	// Projects/Processes view action keys (lowercase)
	if m.currentView == core.VMProjects || m.currentView == core.VMProcesses || m.currentView == core.VMDashboard {
		switch key {
		case "b":
			return m.buildSelected()
		case "r":
			return m.runSelected()
		case "s":
			return m.stopSelected()
		case "k":
			if m.isSelectedProjectSelf() {
				m.lastError = "Cannot kill self"
				m.lastErrorTime = time.Now()
				return nil
			}
			m.dialogType = "kill"
			m.dialogMessage = "Kill the selected process?"
			m.showDialog = true
			return nil
		case "p":
			if m.isSelectedProjectSelf() {
				m.lastError = "Cannot pause self"
				m.lastErrorTime = time.Now()
				return nil
			}
			return m.pauseResumeSelected()
		case "l":
			return m.viewLogsForSelected()
		}
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
		case "b":
			return m.buildSelected()
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

	// Config view specific keys
	if m.currentView == core.VMConfig {
		switch key {
		case "]", "n", "shift+right":
			// Switch to next tab (cycle)
			m.focusArea = FocusMain // Ensure focus is on main content
			switch m.configMode {
			case "projects":
				m.configMode = "browser"
				m.mainIndex = 0
				m.loadBrowserEntries()
			case "browser":
				m.configMode = "settings"
				m.mainIndex = 0
			case "settings":
				m.configMode = "projects"
				m.mainIndex = 0
			}
			return nil
		case "[", "N", "shift+left":
			// Switch to previous tab (cycle)
			m.focusArea = FocusMain // Ensure focus is on main content
			switch m.configMode {
			case "projects":
				m.configMode = "settings"
			case "browser":
				m.configMode = "projects"
				m.mainIndex = 0
			case "settings":
				m.configMode = "browser"
				m.mainIndex = 0
				m.loadBrowserEntries()
			}
			return nil
		case "backspace":
			if m.configMode == "browser" && m.browserPath != "/" {
				m.browserPath = filepath.Dir(m.browserPath)
				m.mainIndex = 0
				m.loadBrowserEntries()
				return nil
			}
		case "a", "A":
			// Add project to config
			if m.configMode == "browser" && m.detectedProject != nil {
				if !m.isProjectInConfig(m.detectedProject.Path) {
					if err := m.addProjectToConfig(); err == nil {
						m.loadBrowserEntries() // Refresh
					}
				}
				return nil
			}
		case "x", "X":
			// Remove project from config - ask for confirmation
			if m.configMode == "projects" {
				cfg := config.GetGlobal()
				if cfg != nil && m.mainIndex >= 0 && m.mainIndex < len(cfg.Projects) {
					proj := cfg.Projects[m.mainIndex]
					// Can't remove self project
					if proj.Self {
						m.lastError = "Cannot remove csd-devtrack (self)"
						m.lastErrorTime = time.Now()
						return nil
					}
					m.pendingRemovePath = proj.Path
					m.dialogType = "remove_project"
					m.dialogMessage = "Remove '" + proj.Name + "' from config?"
					m.dialogConfirm = false
					m.showDialog = true
				}
				return nil
			} else if m.configMode == "browser" && m.detectedProject != nil {
				if m.isProjectInConfig(m.detectedProject.Path) {
					// Check if it's the self project
					if m.isSelfProject(m.detectedProject.Path) {
						m.lastError = "Cannot remove csd-devtrack (self)"
						m.lastErrorTime = time.Now()
						return nil
					}
					m.pendingRemovePath = m.detectedProject.Path
					m.dialogType = "remove_project"
					m.dialogMessage = "Remove '" + m.detectedProject.Name + "' from config?"
					m.dialogConfirm = false
					m.showDialog = true
				}
				return nil
			}
		}
	}

	// Global action keys (F-keys and Ctrl shortcuts)
	switch key {
	case "f5":
		return m.buildSelected()
	case "ctrl+b":
		return m.buildAll()
	case "ctrl+r":
		return m.restartSelected()
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
	case "remove_project":
		if m.pendingRemovePath != "" {
			if err := m.removeProjectFromConfig(m.pendingRemovePath); err == nil {
				// Adjust index if needed
				cfg := config.GetGlobal()
				if cfg != nil && m.mainIndex >= len(cfg.Projects) {
					m.mainIndex = len(cfg.Projects) - 1
					if m.mainIndex < 0 {
						m.mainIndex = 0
					}
				}
				// Refresh browser if in browser mode
				if m.configMode == "browser" {
					m.loadBrowserEntries()
				}
			}
			m.pendingRemovePath = ""
		}
		return nil
	}
	return nil
}

// handleLogsShortcuts handles shortcuts in Logs view when NOT in search mode
// Returns true if the key was handled
func (m *Model) handleLogsShortcuts(msg tea.KeyMsg) bool {
	key := msg.String()

	switch key {
	case "/":
		// Enter search mode
		m.logSearchActive = true
		return true
	case "e":
		m.toggleLogLevel("error")
		return true
	case "w":
		m.toggleLogLevel("warn")
		return true
	case "i":
		m.toggleLogLevel("info")
		return true
	case "a":
		m.logLevelFilter = "" // All levels
		return true
	case "x":
		m.logSearchText = "" // Clear search
		return true
	case "s", "left", "right":
		// Cycle source filter
		m.cycleLogSource(key == "left")
		return true
	case "t":
		// Cycle type filter
		m.cycleLogType()
		return true
	case "c":
		// Clear all filters
		m.logSourceFilter = ""
		m.logTypeFilter = ""
		m.logLevelFilter = ""
		m.logSearchText = ""
		return true

	// Scroll controls
	case "up", "k":
		// Scroll up one line
		m.logScrollOffset++
		m.logAutoScroll = false
		return true
	case "down", "j":
		// Scroll down one line
		if m.logScrollOffset > 0 {
			m.logScrollOffset--
		}
		if m.logScrollOffset == 0 {
			m.logAutoScroll = true
		}
		return true
	case "shift+up", "pgup":
		// Page up
		m.logScrollOffset += 10
		m.logAutoScroll = false
		return true
	case "shift+down", "pgdown":
		// Page down
		m.logScrollOffset -= 10
		if m.logScrollOffset < 0 {
			m.logScrollOffset = 0
		}
		if m.logScrollOffset == 0 {
			m.logAutoScroll = true
		}
		return true
	case "home":
		// Go to top
		m.logScrollOffset = 999999 // Will be clamped in render
		m.logAutoScroll = false
		return true
	case "end":
		// Go to bottom (resume auto-scroll)
		m.logScrollOffset = 0
		m.logAutoScroll = true
		m.logPaused = false
		return true
	case " ":
		// Toggle pause
		m.logPaused = !m.logPaused
		if m.logPaused {
			// When pausing, disable auto-scroll
			m.logAutoScroll = false
		}
		return true
	}

	return false
}

// updateLogSourceOptions builds the list of available sources from log lines
func (m *Model) updateLogSourceOptions() {
	if m.state.Logs == nil {
		return
	}

	sources := make(map[string]bool)
	for _, line := range m.state.Logs.Lines {
		// Extract project/component from source
		source := line.Source
		// Remove "build:" prefix if present
		if strings.HasPrefix(source, "build:") {
			source = strings.TrimPrefix(source, "build:")
		}
		sources[source] = true
	}

	// Build sorted list
	m.logSourceOptions = []string{}
	for source := range sources {
		m.logSourceOptions = append(m.logSourceOptions, source)
	}
	sort.Strings(m.logSourceOptions)
}

// cycleLogSource cycles through source options
func (m *Model) cycleLogSource(reverse bool) {
	if len(m.logSourceOptions) == 0 {
		return
	}

	// Add "all" option at the beginning
	options := append([]string{""}, m.logSourceOptions...)

	// Find current index
	currentIdx := 0
	for i, opt := range options {
		if opt == m.logSourceFilter {
			currentIdx = i
			break
		}
	}

	// Cycle
	if reverse {
		currentIdx--
		if currentIdx < 0 {
			currentIdx = len(options) - 1
		}
	} else {
		currentIdx++
		if currentIdx >= len(options) {
			currentIdx = 0
		}
	}

	m.logSourceFilter = options[currentIdx]
}

// cycleLogType cycles through type options
func (m *Model) cycleLogType() {
	switch m.logTypeFilter {
	case "":
		m.logTypeFilter = "build"
	case "build":
		m.logTypeFilter = "process"
	case "process":
		m.logTypeFilter = ""
	}
}

// getSourceStatus returns the status of a source (running/building/stopped)
func (m *Model) getSourceStatus(source string) string {
	// Check if currently building
	if m.state.Builds != nil && m.state.Builds.IsBuilding && m.state.Builds.CurrentBuild != nil {
		buildSource := m.state.Builds.CurrentBuild.ProjectID + "/" + string(m.state.Builds.CurrentBuild.Component)
		if strings.HasPrefix(buildSource, source) || strings.HasPrefix(source, m.state.Builds.CurrentBuild.ProjectID) {
			return "building"
		}
	}

	// Check if running process
	if m.state.Processes != nil {
		for _, p := range m.state.Processes.Processes {
			if p.IsSelf {
				continue
			}
			procSource := p.ProjectID + "/" + string(p.Component)
			if strings.HasPrefix(procSource, source) || strings.HasPrefix(source, p.ProjectID) {
				if p.State == "running" {
					return "running"
				}
			}
		}
	}

	return "stopped"
}

// handleLogsSearchInput handles typing in Logs search mode
// Returns true if the key was handled
func (m *Model) handleLogsSearchInput(msg tea.KeyMsg) bool {
	key := msg.String()

	switch key {
	case "esc":
		// Exit search mode (keep text)
		m.logSearchActive = false
		return true
	case "enter":
		// Exit search mode (keep text)
		m.logSearchActive = false
		return true
	case "shift+backspace", "ctrl+u":
		// Clear all text
		m.logSearchText = ""
		return true
	case "backspace":
		// Delete last char
		if len(m.logSearchText) > 0 {
			m.logSearchText = m.logSearchText[:len(m.logSearchText)-1]
		}
		return true
	case "delete", "ctrl+k":
		// Clear all text
		m.logSearchText = ""
		return true
	}

	// Printable characters - add to search
	if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
		m.logSearchText += key
		return true
	}

	// Space
	if key == " " {
		m.logSearchText += " "
		return true
	}

	return false
}

// handleLogSearchInput handles typing in log search mode (legacy, for '/' activation)
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
			// Count total component rows (one line per component)
			count := 0
			for _, p := range m.state.Projects.Projects {
				if len(p.Components) == 0 {
					count++ // Project with no components still shows 1 row
				} else {
					count += len(p.Components)
				}
			}
			m.maxMainItems = count
		}
	case core.VMProcesses:
		if m.state.Processes != nil {
			m.maxMainItems = len(m.state.Processes.Processes)
		}
	case core.VMGit:
		if m.state.Git != nil {
			m.maxMainItems = len(m.state.Git.Projects)
			// Also update maxDetailItems if in detail panel
			if m.focusArea == FocusDetail && m.mainIndex >= 0 && m.mainIndex < len(m.state.Git.Projects) {
				p := m.state.Git.Projects[m.mainIndex]
				m.buildGitFileList(&p)
			}
		}
	case core.VMDashboard:
		if m.state.Dashboard != nil {
			m.maxMainItems = len(m.state.Dashboard.Projects)
		}
	case core.VMBuild:
		if m.state.Builds != nil {
			m.maxMainItems = len(m.state.Builds.BuildHistory)
		}
	case core.VMConfig:
		// Config view - count depends on current tab
		switch m.configMode {
		case "projects":
			cfg := config.GetGlobal()
			if cfg != nil {
				m.maxMainItems = len(cfg.Projects)
			}
		case "browser":
			m.maxMainItems = len(m.browserEntries)
		case "settings":
			m.maxMainItems = 0 // No navigation in settings
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

	// Get selected component BEFORE changing view
	component := m.getSelectedComponent()

	// Switch to Build view to show output
	m.currentView = core.VMBuild
	m.sidebarIndex = 2 // Build view index

	return m.sendEvent(core.NewEvent(core.EventStartBuild).WithProject(projectID).WithComponent(component))
}

func (m *Model) buildAll() tea.Cmd {
	// Switch to Build view to show output
	m.currentView = core.VMBuild
	m.sidebarIndex = 2 // Build view index
	return m.sendEvent(core.NewEvent(core.EventBuildAll))
}

func (m *Model) runSelected() tea.Cmd {
	projectID := m.getSelectedProjectID()
	if projectID == "" {
		return nil
	}
	// Cannot run self (csd-devtrack) - it's already running
	if m.isSelectedProjectSelf() {
		m.lastError = "Cannot run csd-devtrack (already running as self)"
		m.lastErrorTime = time.Now()
		return nil
	}
	component := m.getSelectedComponent()
	return m.sendEvent(core.NewEvent(core.EventStartProcess).WithProject(projectID).WithComponent(component))
}

func (m *Model) stopSelected() tea.Cmd {
	projectID := m.getSelectedProjectID()
	if projectID == "" {
		return nil
	}
	// Cannot stop self (csd-devtrack)
	if m.isSelectedProjectSelf() {
		m.lastError = "Cannot stop csd-devtrack (self)"
		m.lastErrorTime = time.Now()
		return nil
	}
	component := m.getSelectedComponent()
	return m.sendEvent(core.NewEvent(core.EventStopProcess).WithProject(projectID).WithComponent(component))
}

func (m *Model) restartSelected() tea.Cmd {
	projectID := m.getSelectedProjectID()
	if projectID == "" {
		return nil
	}
	// Cannot restart self (csd-devtrack)
	if m.isSelectedProjectSelf() {
		m.lastError = "Cannot restart csd-devtrack (self)"
		m.lastErrorTime = time.Now()
		return nil
	}
	component := m.getSelectedComponent()
	return m.sendEvent(core.NewEvent(core.EventRestartProcess).WithProject(projectID).WithComponent(component))
}

func (m *Model) killSelected() tea.Cmd {
	projectID := m.getSelectedProjectID()
	if projectID == "" {
		return nil
	}
	// Cannot kill self (csd-devtrack)
	if m.isSelectedProjectSelf() {
		m.lastError = "Cannot kill csd-devtrack (self)"
		m.lastErrorTime = time.Now()
		return nil
	}
	component := m.getSelectedComponent()
	return m.sendEvent(core.NewEvent(core.EventKillProcess).WithProject(projectID).WithComponent(component))
}

func (m *Model) pauseResumeSelected() tea.Cmd {
	projectID := m.getSelectedProjectID()
	if projectID == "" {
		return nil
	}
	component := m.getSelectedComponent()
	return m.sendEvent(core.NewEvent(core.EventPauseProcess).WithProject(projectID).WithComponent(component))
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

// viewLogsForSelected goes to logs view filtered by the selected component
func (m *Model) viewLogsForSelected() tea.Cmd {
	projectID := m.getSelectedProjectID()
	if projectID == "" {
		return nil
	}
	component := m.getSelectedComponent()

	// Switch to Logs view
	m.currentView = core.VMLogs
	m.sidebarIndex = 4 // Logs view index

	// Set source filter to show only this component's logs
	if component != "" {
		m.logSourceFilter = projectID + "/" + string(component)
	} else {
		m.logSourceFilter = projectID
	}
	// Clear other filters for a fresh view
	m.logSearchText = ""

	return m.sendEvent(core.NewEvent(core.EventViewLogs).WithProject(projectID).WithComponent(component))
}

// cancelCurrent cancels the current build or stops the current process
func (m *Model) cancelCurrent() tea.Cmd {
	// If we're building, cancel the build
	if m.state.Builds != nil && m.state.Builds.IsBuilding {
		// Cancel the current build
		return m.sendEvent(core.NewEvent(core.EventCancelBuild))
	}

	// If we have a selected running process, stop it
	projectID := m.getSelectedProjectID()
	if projectID != "" && !m.isSelectedProjectSelf() {
		component := m.getSelectedComponent()
		return m.sendEvent(core.NewEvent(core.EventStopProcess).WithProject(projectID).WithComponent(component))
	}

	// No action - just show a message
	m.lastError = "Nothing to cancel"
	m.lastErrorTime = time.Now()
	return nil
}

func (m *Model) getSelectedProjectID() string {
	switch m.currentView {
	case core.VMProjects:
		// mainIndex is now a component row index, need to find the project
		proj := m.getProjectForComponentRow(m.mainIndex)
		if proj != nil {
			return proj.ID
		}
	case core.VMDashboard:
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

// getProjectForComponentRow returns the project for a given component row index in Projects view
func (m *Model) getProjectForComponentRow(rowIndex int) *core.ProjectVM {
	if m.state.Projects == nil {
		return nil
	}
	currentRow := 0
	for i := range m.state.Projects.Projects {
		p := &m.state.Projects.Projects[i]
		compCount := len(p.Components)
		if compCount == 0 {
			compCount = 1
		}
		if rowIndex < currentRow+compCount {
			return p
		}
		currentRow += compCount
	}
	return nil
}

// getSelectedComponent returns the component type for the selected row
func (m *Model) getSelectedComponent() projects.ComponentType {
	switch m.currentView {
	case core.VMProjects:
		if m.state.Projects == nil {
			return ""
		}
		currentRow := 0
		for i := range m.state.Projects.Projects {
			p := &m.state.Projects.Projects[i]
			if len(p.Components) == 0 {
				if m.mainIndex == currentRow {
					return "" // Project with no components
				}
				currentRow++
			} else {
				for _, comp := range p.Components {
					if m.mainIndex == currentRow {
						return comp.Type
					}
					currentRow++
				}
			}
		}
	case core.VMProcesses:
		if m.state.Processes != nil && m.mainIndex >= 0 && m.mainIndex < len(m.state.Processes.Processes) {
			return m.state.Processes.Processes[m.mainIndex].Component
		}
	}
	return ""
}

// isSelectedProjectSelf returns true if the selected project is csd-devtrack itself
func (m *Model) isSelectedProjectSelf() bool {
	switch m.currentView {
	case core.VMProjects:
		proj := m.getProjectForComponentRow(m.mainIndex)
		if proj != nil {
			return proj.IsSelf
		}
	case core.VMDashboard:
		projects := core.SelectProjects(m.state)
		if m.mainIndex >= 0 && m.mainIndex < len(projects) {
			return projects[m.mainIndex].IsSelf
		}
	case core.VMProcesses:
		if m.state.Processes != nil && m.mainIndex >= 0 && m.mainIndex < len(m.state.Processes.Processes) {
			return m.state.Processes.Processes[m.mainIndex].IsSelf
		}
	}
	return false
}

// showGitFileDiff shows the diff for the selected file
func (m *Model) showGitFileDiff() tea.Cmd {
	if m.state.Git == nil || m.mainIndex < 0 || m.mainIndex >= len(m.state.Git.Projects) {
		return nil
	}
	if m.detailIndex < 0 || m.detailIndex >= len(m.gitFiles) {
		return nil
	}

	p := m.state.Git.Projects[m.mainIndex]
	f := m.gitFiles[m.detailIndex]

	// Get project path
	cfg := config.GetGlobal()
	var projectPath string
	for _, proj := range cfg.Projects {
		if proj.ID == p.ProjectID {
			projectPath = proj.Path
			break
		}
	}
	if projectPath == "" {
		return nil
	}

	// Get diff using git command
	return func() tea.Msg {
		var cmd *exec.Cmd
		if f.Status == "staged" {
			cmd = exec.Command("git", "diff", "--cached", "--", f.Path)
		} else if f.Status == "untracked" {
			// For untracked files, show file content
			cmd = exec.Command("cat", f.Path)
		} else {
			cmd = exec.Command("git", "diff", "--", f.Path)
		}
		cmd.Dir = projectPath

		output, err := cmd.Output()
		if err != nil {
			return gitDiffMsg{lines: []string{"Error getting diff: " + err.Error()}}
		}

		lines := strings.Split(string(output), "\n")
		if f.Status == "untracked" {
			// Add header for untracked files
			lines = append([]string{
				"New file: " + f.Path,
				"---",
			}, lines...)
		}
		return gitDiffMsg{lines: lines}
	}
}

// gitDiffMsg contains the diff result
type gitDiffMsg struct {
	lines []string
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

// ============================================================================
// File Browser functions for Config view
// ============================================================================

// loadBrowserEntries loads directory entries for the file browser
func (m *Model) loadBrowserEntries() {
	entries, err := os.ReadDir(m.browserPath)
	if err != nil {
		m.browserEntries = nil
		return
	}

	m.browserEntries = make([]BrowserEntry, 0)
	detector := projects.NewDetector()
	cfg := config.GetGlobal()

	// Add parent directory entry
	if m.browserPath != "/" {
		m.browserEntries = append(m.browserEntries, BrowserEntry{
			Name:  "..",
			IsDir: true,
			Path:  filepath.Dir(m.browserPath),
		})
	}

	// Process directory entries
	for _, entry := range entries {
		if !entry.IsDir() {
			continue // Only show directories
		}

		name := entry.Name()
		// Skip hidden directories
		if strings.HasPrefix(name, ".") {
			continue
		}

		fullPath := filepath.Join(m.browserPath, name)
		browserEntry := BrowserEntry{
			Name:  name,
			IsDir: true,
			Path:  fullPath,
		}

		// Check if this is a detectable project
		if proj, err := detector.DetectProject(fullPath); err == nil && len(proj.Components) > 0 {
			browserEntry.IsProject = true
		}

		// Check if already in config
		if cfg != nil {
			for _, p := range cfg.Projects {
				if p.Path == fullPath {
					browserEntry.IsProject = true
					break
				}
			}
		}

		m.browserEntries = append(m.browserEntries, browserEntry)
	}

	// Sort: directories first, then by name
	sort.Slice(m.browserEntries, func(i, j int) bool {
		if m.browserEntries[i].Name == ".." {
			return true
		}
		if m.browserEntries[j].Name == ".." {
			return false
		}
		return m.browserEntries[i].Name < m.browserEntries[j].Name
	})

	// Update max items for navigation
	m.maxMainItems = len(m.browserEntries)

	// Update detected project for current directory
	m.updateDetectedProject()
}

// updateDetectedProject checks if the current directory is a project
func (m *Model) updateDetectedProject() {
	detector := projects.NewDetector()
	proj, err := detector.DetectProject(m.browserPath)
	if err != nil || len(proj.Components) == 0 {
		m.detectedProject = nil
		return
	}

	// Build component list
	var components []string
	for compType := range proj.Components {
		components = append(components, string(compType))
	}

	m.detectedProject = &DetectedProjectInfo{
		Name:       proj.Name,
		Path:       proj.Path,
		Type:       string(proj.Type),
		Components: components,
	}
}

// isProjectInConfig checks if a path is already in config
func (m *Model) isProjectInConfig(path string) bool {
	cfg := config.GetGlobal()
	if cfg == nil {
		return false
	}
	for _, p := range cfg.Projects {
		if p.Path == path {
			return true
		}
	}
	return false
}

// isSelfProject checks if a path is the self project (csd-devtrack)
func (m *Model) isSelfProject(path string) bool {
	cfg := config.GetGlobal()
	if cfg == nil {
		return false
	}
	for _, p := range cfg.Projects {
		if p.Path == path && p.Self {
			return true
		}
	}
	return false
}

// getProjectFromConfig returns the project config for a path
func (m *Model) getProjectFromConfig(path string) *projects.Project {
	cfg := config.GetGlobal()
	if cfg == nil {
		return nil
	}
	for i, p := range cfg.Projects {
		if p.Path == path {
			return &cfg.Projects[i]
		}
	}
	return nil
}

// addProjectToConfig adds the detected project to config
func (m *Model) addProjectToConfig() error {
	if m.detectedProject == nil {
		return nil
	}

	detector := projects.NewDetector()
	proj, err := detector.DetectProject(m.detectedProject.Path)
	if err != nil {
		return err
	}

	cfg := config.GetGlobal()
	if cfg == nil {
		cfg = config.DefaultConfig()
	}

	// Check if already exists
	for _, p := range cfg.Projects {
		if p.Path == proj.Path {
			return nil // Already in config
		}
	}

	cfg.Projects = append(cfg.Projects, *proj)
	return config.SaveGlobal()
}

// removeProjectFromConfig removes a project from config by path
func (m *Model) removeProjectFromConfig(path string) error {
	cfg := config.GetGlobal()
	if cfg == nil {
		return nil
	}

	newProjects := make([]projects.Project, 0)
	for _, p := range cfg.Projects {
		// Never remove self project
		if p.Self {
			newProjects = append(newProjects, p)
			continue
		}
		if p.Path != path {
			newProjects = append(newProjects, p)
		}
	}
	cfg.Projects = newProjects
	return config.SaveGlobal()
}

// enterBrowserDirectory enters the selected directory
func (m *Model) enterBrowserDirectory() {
	if m.mainIndex < 0 || m.mainIndex >= len(m.browserEntries) {
		return
	}

	entry := m.browserEntries[m.mainIndex]
	if !entry.IsDir {
		return
	}

	m.browserPath = entry.Path
	m.mainIndex = 0
	m.loadBrowserEntries()
}
