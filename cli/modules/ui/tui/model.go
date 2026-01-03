package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"csd-devtrack/cli/modules/core/projects"
	"csd-devtrack/cli/modules/platform/config"
	"csd-devtrack/cli/modules/platform/daemon"
	"csd-devtrack/cli/modules/platform/system"
	"csd-devtrack/cli/modules/ui/core"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
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
	dialogInput   textinput.Model // Text input for input dialogs
	dialogInputActive bool        // Whether the dialog has an input field

	// Pending new session creation
	pendingNewSessionProjectID string // Project ID for new session dialog

	// Header ticker animation
	tickerScrollPos int // Current scroll position for header event ticker

	// Context panel refresh tracking
	lastRefreshTime time.Time // Last time context was refreshed

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

	// Projects view state
	projectsMenu *TreeMenu // Tree menu for projects and components

	// Processes view state
	processesMenu *TreeMenu // Tree menu for processes

	// Git view state
	gitDiffContent       []string // Diff content lines
	gitDiffLoading       bool     // Loading diff content
	gitLastSelectedFile  string   // Last selected file ID (for auto-load detection)
	gitFiles             []GitFileEntry // Flat list of all files for current project
	gitFilesProjectID    string   // Project ID for which gitFiles was built
	gitMenu              *TreeMenu       // Tree menu for git projects and files

	// Claude view state
	claudeInstalled      bool              // Is Claude CLI installed
	claudeMode           string            // "sessions", "chat", "settings"
	claudeActiveSession  string            // Active session ID
	claudeSessionLoading bool              // Loading session data
	deletingSessions       map[string]bool // Sessions being deleted (for visual feedback)
	showAllClaudeSessions  bool            // Show all sessions (default: only 10 most recent per project)
	claudeInputText        string          // Current input text (deprecated, use claudeTextInput)
	claudeInputActive    bool            // User is typing
	claudeChatScroll     int             // Scroll offset for chat messages
	claudeSessionScroll  int             // Scroll offset for session list
	claudeRenameActive   bool            // Renaming a session
	claudeRenameText     string          // New name for session
	claudeFilterProject       string // Filter sessions by project ID
	claudeProjectSelectActive bool   // Project selection mode for new session
	claudeProjectSelectIndex  int    // Selected project index
	claudeTreeItemCount       int    // Total items in the tree (projects + sessions)
	claudeTreeItems           []claudeTreeItem // Flattened tree for navigation
	claudeTextInput      textinput.Model // Optimized text input component
	claudeLastEscTime    time.Time       // For double-ESC detection
	sessionsTreeMenu     *TreeMenu       // Tree menu for sessions panel
	sidebarMenu          *TreeMenu       // Tree menu for sidebar navigation

	// Terminal mode (embedded Claude terminal)
	terminalManager      *TerminalManager // Manages terminal sessions
	terminalMode         bool             // True when in terminal mode (keys go to terminal)
	terminalRefreshTick  <-chan time.Time // Ticker for terminal refresh

	// Cockpit view state
	cockpitFocusedIndex int  // Currently focused widget index
	cockpitWidgetActive bool // True when inside a widget (tmux session active)
	cockpitConfigMode   bool // True when configuring cockpit
	cockpitConfigStep   string // "grid", "widgets", "filters", "profile_name"
	cockpitConfigRows   int    // Grid rows being configured
	cockpitConfigCols   int    // Grid cols being configured
	cockpitConfigCell   int    // Current cell being configured
	cockpitGridMenu     *TreeMenu // Menu for grid size selection
	cockpitTypeMenu     *TreeMenu // Menu for widget type selection
	cockpitFilterMenu   *TreeMenu // Menu for filter selection
	cockpitProfileMenu  *TreeMenu // Menu for profile selection
	cockpitNewName      string    // New profile name being entered
	cockpitRenaming     bool      // True when renaming profile
	cockpitCreatingNew  bool      // True when creating new profile

	// Database view state
	databaseActiveSession string    // Active database session ID
	databaseTreeMenu      *TreeMenu // Tree menu for database/sessions panel
	databaseFilterProject string    // Filter by project ID

	// Codex view state
	codexActiveSession string    // Active Codex session ID
	codexTreeMenu      *TreeMenu // Tree menu for sessions panel
	codexFilterProject string    // Filter by project ID

	// Shell view state
	shellActiveSession string    // Active Shell session ID
	shellTreeMenu      *TreeMenu // Tree menu for sessions panel
	shellFilterProject string    // Filter by project ID

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

	// Daemon mode
	detachable bool // If true, can detach from TUI (daemon mode)
	detached   bool // Set to true when user detaches

	// Command mode (like screen/tmux - activated with Ctrl+Space)
	commandMode     bool      // True after Ctrl+Space, waiting for command key
	commandModeTime time.Time // When command mode was activated (for timeout)

	// State restoration callback (for daemon mode)
	onStateRestore func()

	// System metrics
	metricsCollector *system.MetricsCollector
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

	// Get initial browser path from config, or fall back to home directory
	homeDir, _ := os.UserHomeDir()
	if homeDir == "" {
		homeDir, _ = os.Getwd()
	}
	browserPath := homeDir
	if cfg := config.GetGlobal(); cfg != nil && cfg.Settings != nil && cfg.Settings.BrowserPath != "" {
		path := cfg.Settings.BrowserPath
		// Expand ~ to home directory
		if strings.HasPrefix(path, "~/") {
			path = filepath.Join(homeDir, path[2:])
		} else if path == "~" {
			path = homeDir
		}
		// Verify path exists and is accessible
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			browserPath = path
		}
	}

	// Create initial state and populate from presenter
	state := core.NewAppState()
	if presenter != nil {
		// Sync global state flags (including Initializing)
		if presenterState := presenter.GetState(); presenterState != nil {
			state.Initializing = presenterState.Initializing
		}

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
		if vm, err := presenter.GetViewModel(core.VMClaude); err == nil {
			if claude, ok := vm.(*core.ClaudeVM); ok {
				state.Claude = claude
			}
		}
		if vm, err := presenter.GetViewModel(core.VMDatabase); err == nil {
			if database, ok := vm.(*core.DatabaseVM); ok {
				state.Database = database
			}
		}
		// Sync capabilities from presenter state
		if presenterState := presenter.GetState(); presenterState != nil {
			state.Capabilities = presenterState.Capabilities
		}
	}

	// Create metrics collector (updates every 2 seconds)
	metricsCollector := system.NewMetricsCollector(2 * time.Second)
	metricsCollector.Start()

	// Create text input for Claude chat
	ti := textinput.New()
	ti.Placeholder = "Type a message..."
	ti.CharLimit = 4096
	ti.Width = 80

	// Create text input for dialogs
	dialogTi := textinput.New()
	dialogTi.Placeholder = "Enter name..."
	dialogTi.CharLimit = 100
	dialogTi.Width = 40

	// Get Claude path for terminal manager
	claudePath := "claude" // Default
	if state.Claude != nil && state.Claude.ClaudePath != "" {
		claudePath = state.Claude.ClaudePath
	}
	// Try to find full path if just "claude"
	if claudePath == "claude" {
		if fullPath, err := exec.LookPath("claude"); err == nil {
			claudePath = fullPath
		}
	}

	// Create sessions tree menu (right-side panel)
	sessionsMenu := NewTreeMenu(nil)
	sessionsMenu.SetTitle("Sessions")
	sessionsMenu.SetRightSidePanel(true)

	// Create sidebar menu
	sidebarMenu := NewTreeMenu(nil)
	sidebarMenu.SetTitle("≡ MENU")

	// Create git menu
	gitMenu := NewTreeMenu(nil)
	gitMenu.SetTitle("Git")

	// Create projects menu
	projectsMenu := NewTreeMenu(nil)
	projectsMenu.SetTitle("Projects")

	// Create processes menu
	processesMenu := NewTreeMenu(nil)
	processesMenu.SetTitle("Processes")

	// Create database tree menu
	databaseMenu := NewTreeMenu(nil)
	databaseMenu.SetTitle("Databases")
	databaseMenu.SetRightSidePanel(true)

	model := &Model{
		presenter:           presenter,
		state:               state,
		keys:                DefaultKeyMap(),
		currentView:         core.VMDashboard,
		focusArea:           FocusSidebar,
		sidebarIndex:        0,
		help:                h,
		spinner:             s,
		claudeTextInput:     ti,
		claudeMode:          ClaudeModeChat, // Initialize to avoid empty mode issues
		dialogInput:         dialogTi,
		deletingSessions:    make(map[string]bool),
		notifications:       make([]*core.Notification, 0),
		visibleMainRows:     10,
		visibleDetailRows:   5,
		currentBuildProfile: "dev",   // Default to dev profile
		configMode:          "projects", // Start with projects view
		browserPath:         browserPath,
		viewStates:          make(map[core.ViewModelType]*ViewState),
		logAutoScroll:       true, // Auto-scroll logs by default
		metricsCollector:    metricsCollector,
		terminalManager:     NewTerminalManager(claudePath),
		sessionsTreeMenu:    sessionsMenu,
		sidebarMenu:         sidebarMenu,
		gitMenu:             gitMenu,
		projectsMenu:        projectsMenu,
		processesMenu:       processesMenu,
		databaseTreeMenu:    databaseMenu,
	}

	// Initialize sidebar menu items
	model.updateSidebarMenu()

	return model
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
	// Clean up orphan tmux sessions from previous runs
	CleanupOrphanTmuxSessions()

	return tea.Batch(
		m.spinner.Tick,
		m.refreshData,
		tickCmd(), // Start the refresh tick cycle
		tea.WindowSize(),
	)
}

// Cleanup cleans up resources before shutdown
func (m *Model) Cleanup() {
	if m.terminalManager != nil {
		m.terminalManager.StopAll()
	}
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

		// Update textinput width for Claude chat
		inputWidth := m.width - sidebarWidth - 10
		if inputWidth < 40 {
			inputWidth = 40
		}
		m.claudeTextInput.Width = inputWidth

		// Resize active terminal if any
		if m.claudeActiveSession != "" {
			if t := m.terminalManager.Get(m.claudeActiveSession); t != nil {
				// Terminal width: main panel minus borders
				termWidth := m.width - sidebarWidth - 6
				termHeight := m.height - headerHeight - footerHeight - 2
				if termWidth > 20 && termHeight > 5 {
					t.SetSize(termWidth, termHeight)
				}
			}
		}

	case terminalRefreshMsg:
		// Terminal output refresh - just re-render
		return m, m.scheduleTerminalRefresh()

	case tea.KeyMsg:
		// Terminal mode - forward most keys to terminal
		// Works for Claude, Codex, and Database terminals
		activeTerminalSession := m.claudeActiveSession
		if activeTerminalSession == "" {
			activeTerminalSession = m.databaseActiveSession
		}

		if m.terminalMode && activeTerminalSession != "" {
			keyStr := msg.String()

			// ^G enters command mode (even in terminal mode)
			if key.Matches(msg, m.keys.CommandPrefix) {
				m.commandMode = true
				m.commandModeTime = time.Now()
				return m, nil
			}

			// If in command mode, handle command key
			if m.commandMode {
				if time.Since(m.commandModeTime) > 2*time.Second {
					m.commandMode = false
					// Fall through to send key to terminal
				} else {
					m.commandMode = false
					// Handle escape to exit terminal mode
					if keyStr == "esc" || keyStr == "escape" {
						m.terminalMode = false
						m.focusArea = FocusDetail
						m.claudeInputActive = false
						m.claudeRenameActive = false
						return m, nil
					}
					// Other commands work too (q for quit, d for detach)
					return m, m.handleCommandKey(msg)
				}
			}

			// Tab and Shift+Tab are handled by the global focus navigation
			// Don't consume them here - let them fall through
			if keyStr == "tab" || keyStr == "shift+tab" {
				// Will be handled by key.Matches below
			} else if t := m.terminalManager.Get(activeTerminalSession); t != nil {
				consumed, _ := t.HandleKey(keyStr)
				if consumed {
					return m, nil
				}
			}
		}

		// Claude input handling (chat or rename)
		if m.claudeInputActive {
			cmd := m.handleClaudeInput(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}
		if m.claudeRenameActive {
			cmd := m.handleClaudeRenameInput(msg)
			if cmd != nil {
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}

		if m.logSearchActive {
			// In search mode - handle typing
			if m.handleLogsSearchInput(msg) {
				return m, nil
			}
		} else if m.currentView == core.VMLogs && !m.showDialog && !m.showHelp {
			// In Logs view but not in search mode - handle shortcuts
			// Allow shortcuts regardless of focus area (s/t/e/w/i are filter shortcuts)
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
		// Only continue spinner when actually needed (loading states)
		needsSpinner := m.state.Initializing || m.state.GitLoading ||
			(m.state.Claude != nil && m.state.Claude.IsProcessing)
		if needsSpinner {
			cmds = append(cmds, cmd)
		}
		// Toggle blink state for TreeMenu animations (only when spinner is active)
		if needsSpinner && m.sessionsTreeMenu != nil {
			m.sessionsTreeMenu.ToggleBlink()
		}

	case stateUpdateMsg:
		needTerminalRefresh := m.handleStateUpdate(msg.update)
		// Force spinner tick when git is loading in background
		if m.state.GitLoading {
			cmds = append(cmds, m.spinner.Tick)
		}
		// Force spinner tick and schedule next refresh when Claude is processing
		if m.state.Claude != nil && m.state.Claude.IsProcessing {
			cmds = append(cmds, m.spinner.Tick, claudeRefreshCmd())
		}
		// Start terminal refresh loop if a new session was just created
		if needTerminalRefresh {
			cmds = append(cmds, m.scheduleTerminalRefresh())
		}

	case claudeRefreshMsg:
		// Periodic refresh during Claude processing for responsive streaming
		if m.state.Claude != nil && m.state.Claude.IsProcessing {
			cmds = append(cmds, m.spinner.Tick, claudeRefreshCmd())
		}

	case notificationMsg:
		m.handleNotification(msg.notification)

	case refreshMsg:
		// Update refresh timestamp on initial refresh too
		if m.lastRefreshTime.IsZero() {
			m.lastRefreshTime = time.Now()
		}
		cmds = append(cmds, m.refreshData)

	case errMsg:
		m.lastError = msg.Error()
		m.lastErrorTime = time.Now()

	case tickMsg:
		// Clear expired header events
		m.state.ClearExpiredHeaderEvents()

		// Advance ticker scroll position (for header event animation)
		m.tickerScrollPos++

		// Update refresh timestamp
		m.lastRefreshTime = time.Now()

		cmds = append(cmds, m.refreshData, tickCmd())

	case gitDiffMsg:
		m.gitDiffContent = msg.lines
		m.gitDiffLoading = false
		m.detailScrollOffset = 0

	case tuiStateRestoreMsg:
		m.ImportTUIState(msg.state)
	}

	return m, tea.Batch(cmds...)
}

// View renders the UI
func (m Model) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	// Show spinner while daemon is initializing (slow git operations)
	if m.state.Initializing {
		return m.renderInitializingView()
	}

	header := m.renderHeader()
	sidebar := m.renderSidebar()
	main := m.renderMainContent()
	footer := m.renderFooter()

	// Combine sidebar and main with gap
	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, " ", main)

	return lipgloss.JoinVertical(lipgloss.Left, header, body, footer)
}

// renderInitializingView renders a full-screen loading view with spinner
func (m Model) renderInitializingView() string {
	// Header
	header := m.renderHeader()

	// Centered spinner with message
	spinnerStyle := lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true)

	messageStyle := lipgloss.NewStyle().
		Foreground(ColorMuted)

	content := lipgloss.JoinVertical(lipgloss.Center,
		"",
		"",
		spinnerStyle.Render(m.spinner.View()+" Initializing..."),
		"",
		messageStyle.Render("Loading projects and git status"),
		"",
	)

	// Center in available space
	contentHeight := m.height - 6
	contentWidth := m.width - 4

	centered := lipgloss.Place(
		contentWidth,
		contentHeight,
		lipgloss.Center,
		lipgloss.Center,
		content,
	)

	// Simple footer
	footer := lipgloss.NewStyle().
		Width(m.width).
		Background(ColorBgAlt).
		Render(" Please wait...")

	return lipgloss.JoinVertical(lipgloss.Left, header, centered, footer)
}

// handleKeyPress processes keyboard input
func (m *Model) handleKeyPress(msg tea.KeyMsg) tea.Cmd {
	// Command mode handling (like screen/tmux)
	// Ctrl+G activates command mode, then next key is the command
	if key.Matches(msg, m.keys.CommandPrefix) {
		m.commandMode = true
		m.commandModeTime = time.Now()
		return nil
	}

	// If in command mode, process the command key
	if m.commandMode {
		// Timeout after 2 seconds
		if time.Since(m.commandModeTime) > 2*time.Second {
			m.commandMode = false
			// Fall through to normal key handling
		} else {
			m.commandMode = false
			return m.handleCommandKey(msg)
		}
	}

	// Handle Cockpit config mode navigation FIRST (before other handlers)
	if m.currentView == core.VMCockpit && m.cockpitConfigMode {
		if m.handleCockpitConfigNavigation(msg) {
			return nil
		}
	}

	// Handle Escape for context-specific exits
	if msg.String() == "esc" {
		// Focus detail -> back to main
		if m.focusArea == FocusDetail {
			m.focusArea = FocusMain
			return nil
		}
		return nil
	}

	// Handle Shift+Up/Down for page scrolling in git diff panel
	if m.currentView == core.VMGit && m.focusArea == FocusDetail {
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
	// Cancel current build/process (Ctrl+C)
	case key.Matches(msg, m.keys.Cancel):
		return m.cancelCurrent()

	// Help toggle
	case key.Matches(msg, m.keys.Help):
		m.showHelp = !m.showHelp
		return nil

	// Focus navigation - consistent across all views:
	// Shift+Tab: always go to sidebar (except Cockpit: previous widget)
	// Tab: cycle between other panels (Main <-> Detail) or widgets in Cockpit
	case key.Matches(msg, m.keys.ShiftTab):
		// Cockpit: Shift+Tab cycles to previous widget
		if m.currentView == core.VMCockpit && !m.cockpitConfigMode {
			m.navigateCockpitPrev()
			return nil
		}
		// Other views: Shift+Tab always goes to sidebar (from any panel)
		if m.focusArea != FocusSidebar {
			m.focusArea = FocusSidebar
			m.terminalMode = false
			m.claudeInputActive = false
		} else {
			// From sidebar, go to main panel
			m.focusArea = FocusMain
		}
		return nil
	case key.Matches(msg, m.keys.Tab):
		// Cockpit: Tab cycles between widgets
		if m.currentView == core.VMCockpit && !m.cockpitConfigMode {
			m.navigateCockpitNext()
			return nil
		}
		// Other views: Tab cycles between Main and Detail panels (skips sidebar)
		if m.focusArea == FocusSidebar {
			// From sidebar, go to main
			m.focusArea = FocusMain
		} else if m.focusArea == FocusMain {
			// Main -> Detail
			m.focusArea = FocusDetail
			m.terminalMode = false
			m.claudeInputActive = false
		} else {
			// Detail -> Main (and re-enter terminal if applicable)
			m.focusArea = FocusMain
			// Re-enter terminal mode if there's an active terminal
			if m.currentView == core.VMClaude && m.claudeActiveSession != "" {
				if t := m.terminalManager.Get(m.claudeActiveSession); t != nil && t.IsRunning() {
					m.terminalMode = true
					m.claudeInputActive = false
					m.claudeRenameActive = false
					m.commandMode = false
				}
			}
		}
		return nil

	// Directional navigation
	case key.Matches(msg, m.keys.Up):
		// Special: Claude chat scroll
		if m.currentView == core.VMClaude && m.claudeMode == ClaudeModeChat && m.focusArea == FocusMain && !m.claudeInputActive {
			m.claudeChatScroll++
			return nil
		}
		// TreeMenu navigation
		if tm := m.getActiveTreeMenu(); tm != nil {
			tm.MoveUp()
			if m.currentView == core.VMGit && m.focusArea == FocusMain {
				return m.loadGitDiffForSelection()
			}
			return nil
		}
		m.navigateUp()
		return nil

	case key.Matches(msg, m.keys.Down):
		// Special: Claude chat scroll
		if m.currentView == core.VMClaude && m.claudeMode == ClaudeModeChat && m.focusArea == FocusMain && !m.claudeInputActive {
			if m.claudeChatScroll > 0 {
				m.claudeChatScroll--
			}
			return nil
		}
		// TreeMenu navigation
		if tm := m.getActiveTreeMenu(); tm != nil {
			tm.MoveDown()
			if m.currentView == core.VMGit && m.focusArea == FocusMain {
				return m.loadGitDiffForSelection()
			}
			return nil
		}
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
		// Claude chat: page up scrolls chat
		if m.currentView == core.VMClaude && m.claudeMode == ClaudeModeChat && m.focusArea == FocusMain && !m.claudeInputActive {
			m.claudeChatScroll += 10
			return nil
		}
		m.pageUp()
		return nil
	case key.Matches(msg, m.keys.PageDown):
		// Claude chat: page down scrolls chat
		if m.currentView == core.VMClaude && m.claudeMode == ClaudeModeChat && m.focusArea == FocusMain && !m.claudeInputActive {
			m.claudeChatScroll -= 10
			if m.claudeChatScroll < 0 {
				m.claudeChatScroll = 0
			}
			return nil
		}
		m.pageDown()
		return nil
	case key.Matches(msg, m.keys.Home):
		// Claude chat: home goes to top
		if m.currentView == core.VMClaude && m.claudeMode == ClaudeModeChat && m.focusArea == FocusMain && !m.claudeInputActive {
			m.claudeChatScroll = 999999 // Will be clamped in render
			return nil
		}
		m.goToStart()
		return nil
	case key.Matches(msg, m.keys.End):
		// Claude chat: end goes to bottom (most recent)
		if m.currentView == core.VMClaude && m.claudeMode == ClaudeModeChat && m.focusArea == FocusMain && !m.claudeInputActive {
			m.claudeChatScroll = 0
			return nil
		}
		m.goToEnd()
		return nil

	// Enter - select/activate
	case key.Matches(msg, m.keys.Enter):
		// Claude sessions panel: use TreeMenu to select/drill-down
		if m.currentView == core.VMClaude && m.claudeMode == ClaudeModeChat && m.focusArea == FocusDetail {
			if item := m.sessionsTreeMenu.Select(); item != nil {
				// Verify it's actually a session (not a project without children)
				if _, isSession := item.Data.(core.ClaudeSessionVM); isSession {
					// Leaf item selected (session) - switch to it
					return m.switchToSessionByID(item.ID)
				}
				// It's a project with no sessions - do nothing
			}
			// If Select() returned nil, it drilled down - nothing more to do
			return nil
		}
		// Database panel: use TreeMenu to select/drill-down
		if m.currentView == core.VMDatabase && m.focusArea == FocusDetail {
			if item := m.databaseTreeMenu.Select(); item != nil {
				// Leaf item selected (database) - connect directly
				if _, isDB := item.Data.(core.DatabaseInfoVM); isDB {
					dbID := strings.TrimPrefix(item.ID, "db:")
					return m.connectToDatabase(dbID)
				}
			}
			// If Select() returned nil, it drilled down - nothing more to do
			return nil
		}
		// Shell sessions panel: use TreeMenu to select/drill-down
		if m.currentView == core.VMShell && m.focusArea == FocusDetail && m.shellTreeMenu != nil {
			if item := m.shellTreeMenu.Select(); item != nil {
				// Leaf item selected (session) - connect to it via ID
				if len(item.Children) == 0 && strings.HasPrefix(item.ID, "shell-") {
					m.shellActiveSession = item.ID
					return nil
				}
			}
			// If Select() returned nil, it drilled down/up - nothing more to do
			return nil
		}
		// Codex sessions panel: use TreeMenu to select/drill-down
		if m.currentView == core.VMCodex && m.focusArea == FocusDetail && m.codexTreeMenu != nil {
			if item := m.codexTreeMenu.Select(); item != nil {
				// Leaf item selected (session) - connect to it via ID
				if len(item.Children) == 0 && strings.HasPrefix(item.ID, "codex-") {
					m.codexActiveSession = item.ID
					return nil
				}
			}
			// If Select() returned nil, it drilled down/up - nothing more to do
			return nil
		}
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
	case core.VMProjects, core.VMGit, core.VMProcesses, core.VMDatabase:
		return true
	default:
		return false
	}
}

// getActiveTreeMenu returns the TreeMenu that should handle navigation for current view/focus
// Returns nil if no TreeMenu is active for the current context
func (m *Model) getActiveTreeMenu() *TreeMenu {
	// Cockpit config mode has priority - uses its own menus
	if m.currentView == core.VMCockpit && m.cockpitConfigMode {
		switch m.cockpitConfigStep {
		case "grid":
			return m.cockpitGridMenu
		case "widgets":
			return m.cockpitTypeMenu
		case "filters":
			return m.cockpitFilterMenu
		case "profile":
			return m.cockpitProfileMenu
		}
		return nil
	}

	switch m.focusArea {
	case FocusMain:
		switch m.currentView {
		case core.VMProjects:
			return m.projectsMenu
		case core.VMProcesses:
			return m.processesMenu
		case core.VMGit:
			return m.gitMenu
		}
	case FocusDetail:
		switch m.currentView {
		case core.VMClaude:
			if m.claudeMode == ClaudeModeChat && !m.claudeInputActive {
				return m.sessionsTreeMenu
			}
		case core.VMDatabase:
			return m.databaseTreeMenu
		case core.VMShell:
			return m.shellTreeMenu
		case core.VMCodex:
			return m.codexTreeMenu
		}
	}
	return nil
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
	switch m.focusArea {
	case FocusSidebar:
		m.sidebarMenu.MoveUp()
		m.sidebarIndex = m.sidebarMenu.SelectedIndex()
	case FocusMain:
		// Views using TreeMenu
		switch m.currentView {
		case core.VMGit:
			m.gitMenu.MoveUp()
		case core.VMProjects:
			m.projectsMenu.MoveUp()
		case core.VMProcesses:
			m.processesMenu.MoveUp()
		case core.VMCockpit:
			m.navigateCockpitUp()
		default:
			if m.mainIndex > 0 {
				m.mainIndex--
				m.ensureMainVisible()
			}
		}
	case FocusDetail:
		// Git view: scroll diff in detail panel
		if m.currentView == core.VMGit {
			if m.detailScrollOffset > 0 {
				m.detailScrollOffset--
			}
		} else if m.detailIndex > 0 {
			m.detailIndex--
			m.ensureDetailVisible()
		}
	}
}

// navigateDown moves selection down in current focus area
func (m *Model) navigateDown() {
	switch m.focusArea {
	case FocusSidebar:
		m.sidebarMenu.MoveDown()
		m.sidebarIndex = m.sidebarMenu.SelectedIndex()
	case FocusMain:
		// Views using TreeMenu
		switch m.currentView {
		case core.VMGit:
			m.gitMenu.MoveDown()
		case core.VMProjects:
			m.projectsMenu.MoveDown()
		case core.VMProcesses:
			m.processesMenu.MoveDown()
		case core.VMCockpit:
			m.navigateCockpitDown()
		default:
			if m.mainIndex < m.maxMainItems-1 {
				m.mainIndex++
				m.ensureMainVisible()
			}
		}
	case FocusDetail:
		// Git view: scroll diff in detail panel
		if m.currentView == core.VMGit {
			maxScroll := len(m.gitDiffContent) - m.visibleDetailRows
			if maxScroll < 0 {
				maxScroll = 0
			}
			if m.detailScrollOffset < maxScroll {
				m.detailScrollOffset++
			}
		} else if m.detailIndex < m.maxDetailItems-1 {
			m.detailIndex++
			m.ensureDetailVisible()
		}
	}
}

// navigateLeft moves selection left (Config tabs or Widgets)
func (m *Model) navigateLeft() {
	switch m.currentView {
	case core.VMConfig:
		switch m.configMode {
		case "browser":
			m.configMode = "projects"
			m.mainIndex = 0
		case "settings":
			m.configMode = "browser"
			m.mainIndex = 0
			m.loadBrowserEntries()
		}
	case core.VMCockpit:
		m.navigateCockpitLeft()
	}
}

// navigateRight moves selection right (Config tabs or Widgets)
func (m *Model) navigateRight() {
	switch m.currentView {
	case core.VMConfig:
		switch m.configMode {
		case "projects":
			m.configMode = "browser"
			m.mainIndex = 0
			m.loadBrowserEntries()
		case "browser":
			m.configMode = "settings"
			m.mainIndex = 0
		}
	case core.VMCockpit:
		m.navigateCockpitRight()
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
		m.sidebarMenu.SetSelectedIndex(0)
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
		lastIndex := m.sidebarMenu.TotalVisibleCount() - 1
		m.sidebarMenu.SetSelectedIndex(lastIndex)
		m.sidebarIndex = lastIndex
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

// selectView changes the current view by index
func (m *Model) selectView(index int) tea.Cmd {
	views := m.getSidebarViews()

	if index >= 0 && index < len(views) {
		return m.selectViewByType(views[index].vtype)
	}
	return nil
}

// selectViewByType changes the current view by type
func (m *Model) selectViewByType(viewType core.ViewModelType) tea.Cmd {
	// Find index in sidebar
	views := m.getSidebarViews()
	index := -1
	for i, v := range views {
		if v.vtype == viewType {
			index = i
			break
		}
	}

	if index < 0 {
		return nil // View not available
	}

	// Save current view state before switching
	m.saveViewState()

	// Switch view
	m.sidebarIndex = index
	m.currentView = viewType

	// Restore saved state for new view
	m.restoreViewState(viewType)

	// Initialize Config view
	if m.currentView == core.VMConfig {
		if m.configMode == "" {
			m.configMode = "projects"
		}
		if m.configMode == "browser" {
			m.loadBrowserEntries()
		}
	}

	// Initialize Claude view
	if m.currentView == core.VMClaude {
		if m.claudeMode == "" {
			m.claudeMode = ClaudeModeChat
		}
		// Default focus to sessions panel so user can select/create a session
		if m.claudeActiveSession == "" {
			m.focusArea = FocusDetail
		}
	}

	// Initialize Database view
	if m.currentView == core.VMDatabase {
		// Update TreeMenu with current data
		m.updateDatabaseMenu()
		// Default focus to sessions panel so user can select/create a session
		if m.databaseActiveSession == "" {
			m.focusArea = FocusDetail
		}
	}

	// Reset widgets config mode when leaving/entering view
	if m.currentView != core.VMCockpit {
		m.cockpitConfigMode = false
	}

	m.state.SetCurrentView(m.currentView)
	return m.sendEvent(core.NavigateEvent(m.currentView))
}

// handleCommandKey handles command keys after Ctrl+Space prefix
// Commands: q=quit, d=detach, ?=help
func (m *Model) handleCommandKey(msg tea.KeyMsg) tea.Cmd {
	keyStr := msg.String()

	switch keyStr {
	case "q":
		// Quit DevTrack
		return tea.Quit

	case "d":
		// Detach from DevTrack (daemon mode only)
		if m.detachable {
			m.detached = true
			return tea.Quit
		}
		m.lastError = "Detach only available in daemon mode"
		m.lastErrorTime = time.Now()
		return nil

	case "?":
		// Show command help
		m.showHelp = true
		return nil

	case "escape", "esc":
		// Cancel command mode (already cancelled, just return)
		return nil

	default:
		// Unknown command
		m.lastError = fmt.Sprintf("Unknown command: ^G %s", keyStr)
		m.lastErrorTime = time.Now()
		return nil
	}
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
			// Projects view uses TreeMenu - Select() handles back item, drill-down, and leaf selection
			if item := m.projectsMenu.Select(); item != nil {
				// Leaf item selected (component) - focus detail panel
				if _, ok := item.Data.(core.ComponentVM); ok {
					m.focusArea = FocusDetail
				}
			}
		case core.VMProcesses:
			// Processes view uses TreeMenu - Select() handles back item, drill-down, and leaf selection
			if item := m.processesMenu.Select(); item != nil {
				// Leaf item selected (process) - focus detail panel
				if _, ok := item.Data.(core.ProcessVM); ok {
					m.focusArea = FocusDetail
				}
			}
		case core.VMGit:
			// Git view uses TreeMenu - Select() handles back item, drill-down, and leaf selection
			if item := m.gitMenu.Select(); item != nil {
				// Leaf item selected (file) - focus detail panel
				if _, ok := item.Data.(GitFileEntry); ok {
					m.focusArea = FocusDetail
				}
			}
			return m.loadGitDiffForSelection()
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
		case core.VMClaude:
			// Claude view: Enter in main panel is handled by text input
			// Sessions panel Enter is handled in handleKeyPress
		}
	case FocusDetail:
		// Detail panel Enter actions (none for Git - diff shown automatically)
	}
	return nil
}

// handleActionKey handles action keys (b, r, s, etc.)
func (m *Model) handleActionKey(msg tea.KeyMsg) tea.Cmd {
	key := msg.String()

	// Quick view navigation shortcuts (uppercase only)
	// These ALWAYS navigate to views - no exceptions
	switch key {
	case "B":
		return m.selectViewByType(core.VMDashboard)
	case "P":
		return m.selectViewByType(core.VMProjects)
	case "U":
		return m.selectViewByType(core.VMBuild)
	case "O":
		return m.selectViewByType(core.VMProcesses)
	case "L":
		return m.selectViewByType(core.VMLogs)
	case "G":
		return m.selectViewByType(core.VMGit)
	case "K":
		return m.selectViewByType(core.VMCockpit)
	case "C":
		// Claude Code view (requires tmux + claude)
		if m.state.Capabilities != nil && m.state.Capabilities.HasClaude() {
			return m.selectViewByType(core.VMClaude)
		}
		if m.state.Capabilities != nil && !m.state.Capabilities.Tmux.Available {
			m.lastError = "tmux required for Claude view"
			m.lastErrorTime = time.Now()
		} else if m.state.Capabilities != nil && !m.state.Capabilities.Claude.Available {
			m.lastError = "claude CLI not found"
			m.lastErrorTime = time.Now()
		}
		return nil
	case "X":
		// Codex view (requires tmux + codex)
		if m.state.Capabilities != nil && m.state.Capabilities.HasCodex() {
			return m.selectViewByType(core.VMCodex)
		}
		if m.state.Capabilities != nil && !m.state.Capabilities.Tmux.Available {
			m.lastError = "tmux required for Codex view"
			m.lastErrorTime = time.Now()
		} else if m.state.Capabilities != nil && !m.state.Capabilities.Codex.Available {
			m.lastError = "codex CLI not found"
			m.lastErrorTime = time.Now()
		}
		return nil
	case "D":
		// Database view (requires tmux + db client + databases configured)
		if m.state.Capabilities != nil && m.state.Capabilities.HasDatabase() &&
			m.state.Database != nil && len(m.state.Database.Databases) > 0 {
			return m.selectViewByType(core.VMDatabase)
		}
		if m.state.Capabilities != nil && !m.state.Capabilities.Tmux.Available {
			m.lastError = "tmux required for Database view"
			m.lastErrorTime = time.Now()
		} else if m.state.Capabilities != nil && !m.state.Capabilities.HasDatabase() {
			m.lastError = "No database client found (psql, mysql, sqlite3)"
			m.lastErrorTime = time.Now()
		} else if m.state.Database == nil || len(m.state.Database.Databases) == 0 {
			m.lastError = "No databases configured"
			m.lastErrorTime = time.Now()
		}
		return nil
	case "T":
		// Terminal/Shell view (requires tmux + shell)
		if m.state.Capabilities != nil && m.state.Capabilities.HasShell() {
			return m.selectViewByType(core.VMShell)
		}
		if m.state.Capabilities != nil && !m.state.Capabilities.Tmux.Available {
			m.lastError = "tmux required for Terminal view"
			m.lastErrorTime = time.Now()
		} else if m.state.Capabilities != nil && !m.state.Capabilities.Shell.Available {
			m.lastError = "shell (bash/sh) not found"
			m.lastErrorTime = time.Now()
		}
		return nil
	case "S":
		return m.selectViewByType(core.VMConfig)
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
		case "d", "1":
			m.currentBuildProfile = "dev"
			return nil
		case "t", "2":
			m.currentBuildProfile = "test"
			return nil
		case "p", "3":
			m.currentBuildProfile = "prod"
			return nil
		case "left":
			// Cycle profiles backward: dev <- test <- prod <- dev
			switch m.currentBuildProfile {
			case "dev":
				m.currentBuildProfile = "prod"
			case "test":
				m.currentBuildProfile = "dev"
			case "prod":
				m.currentBuildProfile = "test"
			}
			return nil
		case "right":
			// Cycle profiles forward: dev -> test -> prod -> dev
			switch m.currentBuildProfile {
			case "dev":
				m.currentBuildProfile = "test"
			case "test":
				m.currentBuildProfile = "prod"
			case "prod":
				m.currentBuildProfile = "dev"
			}
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

	// Widgets view specific keys
	if m.currentView == core.VMCockpit {
		switch key {
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			if !m.cockpitConfigMode {
				m.switchCockpitProfile(key)
			}
			return nil
		case "c":
			// Enter/exit config mode
			m.cockpitConfigMode = !m.cockpitConfigMode
			if m.cockpitConfigMode {
				m.cockpitConfigStep = "grid"
				m.initCockpitConfigMenus()
			}
			return nil
		case "n":
			// New profile
			if !m.cockpitConfigMode {
				m.startNewCockpitProfile()
			}
			return nil
		case "x":
			// Delete profile (with confirmation)
			if !m.cockpitConfigMode {
				cfg := config.GetGlobal()
				if cfg != nil && len(cfg.WidgetProfiles) > 1 {
					m.dialogType = "delete_cockpit_profile"
					m.dialogMessage = fmt.Sprintf("Delete profile '%s'?", m.getActiveCockpitProfile())
					m.showDialog = true
				} else {
					m.lastError = "Cannot delete the only profile"
					m.lastErrorTime = time.Now()
				}
			}
			return nil
		case "r":
			// Rename profile
			if !m.cockpitConfigMode {
				m.startRenameCockpitProfile()
			}
			return nil
		case "enter":
			if m.cockpitConfigMode {
				return m.handleCockpitConfigEnter()
			}
			return nil
		case "esc":
			if m.cockpitConfigMode {
				m.cockpitConfigMode = false
				return nil
			}
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

	// Claude view specific keys
	if m.currentView == core.VMClaude {
		// PRIORITY: Handle interactive responses first (when Claude is waiting for input)
		// Only handle y/n for interactive when NOT in the sessions panel
		if m.claudeMode == ClaudeModeChat && m.state.Claude != nil && m.state.Claude.WaitingForInput && m.focusArea != FocusDetail {
			switch key {
			case "y", "Y":
				// Approve permission/plan, then return to input mode
				var cmd tea.Cmd
				if m.state.Claude.Interactive != nil {
					switch m.state.Claude.Interactive.Type {
					case "permission":
						cmd = m.sendEvent(core.NewEvent(core.EventClaudeApprovePermission).
							WithData("session_id", m.claudeActiveSession))
					case "plan":
						cmd = m.sendEvent(core.NewEvent(core.EventClaudeApprovePlan).
							WithData("session_id", m.claudeActiveSession))
					}
				}
				if m.state.Claude.PlanPending {
					cmd = m.sendEvent(core.NewEvent(core.EventClaudeApprovePlan).
						WithData("session_id", m.claudeActiveSession))
				}
				// Return to input mode after response
				m.claudeInputActive = true
				m.claudeTextInput.Focus()
				return tea.Batch(cmd, m.claudeTextInput.Cursor.BlinkCmd(), claudeRefreshCmd())
			case "n", "N":
				// Deny permission/plan, then return to input mode
				var cmd tea.Cmd
				if m.state.Claude.Interactive != nil {
					switch m.state.Claude.Interactive.Type {
					case "permission":
						cmd = m.sendEvent(core.NewEvent(core.EventClaudeDenyPermission).
							WithData("session_id", m.claudeActiveSession))
					case "plan":
						cmd = m.sendEvent(core.NewEvent(core.EventClaudeRejectPlan).
							WithData("session_id", m.claudeActiveSession))
					}
				}
				if m.state.Claude.PlanPending {
					cmd = m.sendEvent(core.NewEvent(core.EventClaudeRejectPlan).
						WithData("session_id", m.claudeActiveSession))
				}
				// Return to input mode after response
				m.claudeInputActive = true
				m.claudeTextInput.Focus()
				return tea.Batch(cmd, m.claudeTextInput.Cursor.BlinkCmd(), claudeRefreshCmd())
			case "1", "2", "3", "4", "5", "6", "7", "8", "9":
				// Select option when Claude asks a question with options
				if m.state.Claude.Interactive != nil && m.state.Claude.Interactive.Type == "question" {
					optIdx := int(key[0] - '1')
					if optIdx >= 0 && optIdx < len(m.state.Claude.Interactive.Options) {
						answer := m.state.Claude.Interactive.Options[optIdx]
						cmd := m.sendEvent(core.NewEvent(core.EventClaudeAnswerQuestion).
							WithData("session_id", m.claudeActiveSession).
							WithData("answer", answer))
						// Return to input mode after answering
						m.claudeInputActive = true
						m.claudeTextInput.Focus()
						return tea.Batch(cmd, m.claudeTextInput.Cursor.BlinkCmd(), claudeRefreshCmd())
					}
				}
				return nil
			case "i":
				// Start input mode to type custom answer
				m.claudeInputActive = true
				m.claudeTextInput.Focus()
				return m.claudeTextInput.Cursor.BlinkCmd()
			}
		}

		// Sessions panel: Enter to select (up/down handled in handleKeyPress)
		if m.claudeMode == ClaudeModeChat && m.focusArea == FocusDetail && !m.claudeInputActive {
			if key == "enter" {
				return m.switchToSelectedSession()
			}
		}

		// Chat mode vim-style scroll controls (ctrl+u/d, g/G)
		// Note: pgup/pgdown/home/end/shift+up/shift+down handled in handleKeyPress
		if m.claudeMode == ClaudeModeChat && m.focusArea == FocusMain && !m.claudeInputActive {
			switch key {
			case "ctrl+u":
				// Half page up (vim style)
				m.claudeChatScroll += 10
				return nil
			case "ctrl+d":
				// Half page down (vim style)
				m.claudeChatScroll -= 10
				if m.claudeChatScroll < 0 {
					m.claudeChatScroll = 0
				}
				return nil
			case "g":
				// Go to top (vim style)
				m.claudeChatScroll = 999999
				return nil
			case "G":
				// Go to bottom (vim style)
				m.claudeChatScroll = 0
				return nil
			}
		}

		switch key {
		case "a":
			// Toggle show all sessions (vs 10 most recent per project)
			m.showAllClaudeSessions = !m.showAllClaudeSessions
			m.updateClaudeTree()
			return nil
		case "n":
			// New session - when focused on sessions panel and on/in a project
			if m.claudeMode == ClaudeModeChat && m.focusArea == FocusDetail {
				projectID, _, isProject, _ := m.getSelectedTreeItem()

				// If drilled down into a project, get project ID from drill path
				if !isProject && projectID == "" {
					drillPath := m.sessionsTreeMenu.DrillDownPath()
					if len(drillPath) > 0 {
						projectID = drillPath[0]
					}
				}

				// Create new session if we have a project (either selected or drilled into)
				if projectID != "" {
					// Generate default session name
					defaultName := m.generateDefaultSessionName(projectID)
					m.pendingNewSessionProjectID = projectID
					m.dialogType = "new_claude_session"
					m.dialogMessage = "New session name:"
					m.dialogInput.SetValue(defaultName)
					m.dialogInput.Focus()
					m.dialogInputActive = true
					m.showDialog = true
					return m.dialogInput.Cursor.BlinkCmd()
				}
			}
			return nil
		case "x":
			// Delete selected session (when focus is on sessions panel)
			if m.claudeMode == ClaudeModeChat && m.focusArea == FocusDetail {
				_, sessionID, isProject, _ := m.getSelectedTreeItem()
				if !isProject && sessionID != "" {
					// Get session name from selected item
					sessionName := sessionID
					if item := m.sessionsTreeMenu.SelectedItem(); item != nil {
						sessionName = item.Label
					}
					m.dialogType = "delete_claude_session"
					m.dialogMessage = fmt.Sprintf("Delete session \"%s\"?", sessionName)
					m.showDialog = true
				}
			}
			return nil
		case "r":
			// Rename selected session (when focus is on sessions panel)
			if m.claudeMode == ClaudeModeChat && m.focusArea == FocusDetail {
				_, sessionID, isProject, _ := m.getSelectedTreeItem()
				if !isProject && sessionID != "" {
					m.sessionsTreeMenu.SetRenameActive(true)
					m.claudeRenameActive = true
				}
			}
			return nil
		case "d":
			// Disconnect tmux session (when focus is on sessions panel and session has terminal)
			if m.claudeMode == ClaudeModeChat && m.focusArea == FocusDetail {
				_, sessionID, isProject, hasTerminal := m.getSelectedTreeItem()
				if !isProject && sessionID != "" && hasTerminal {
					return m.stopClaudeTerminal(sessionID)
				}
			}
			return nil
		case "i":
			// Start input mode (in chat mode) - only if a session is selected
			if m.claudeMode == ClaudeModeChat && m.claudeActiveSession != "" {
				m.claudeInputActive = true
				m.claudeTextInput.Focus()
				return m.claudeTextInput.Cursor.BlinkCmd()
			}
			return nil
		case "esc":
			// Exit input mode or switch focus
			if m.claudeInputActive {
				m.claudeInputActive = false
				return nil
			}
			// If in sessions panel, go back to chat
			if m.focusArea == FocusDetail {
				m.focusArea = FocusMain
				return nil
			}
			return nil
		case "c":
			// Clear filter
			m.claudeFilterProject = ""
			return nil
		}
	}

	// Database view specific keys
	if m.currentView == core.VMDatabase {
		switch key {
		case "d":
			// Disconnect database terminal
			if m.databaseActiveSession != "" {
				return m.stopDatabaseTerminal(m.databaseActiveSession)
			}
			return nil
		case "enter":
			// Enter terminal mode when focused on terminal panel
			if m.focusArea == FocusMain && m.databaseActiveSession != "" {
				if t := m.terminalManager.Get(m.databaseActiveSession); t != nil && t.IsRunning() {
					m.terminalMode = true
					m.claudeInputActive = false
					m.claudeRenameActive = false
					m.commandMode = false
				}
			}
			return nil
		case "esc":
			// Exit terminal mode or switch focus
			if m.terminalMode {
				m.terminalMode = false
				return nil
			}
			if m.focusArea == FocusDetail {
				m.focusArea = FocusMain
				return nil
			}
			return nil
		}
	}

	// Shell view specific keys
	if m.currentView == core.VMShell {
		switch key {
		case "n":
			// New project shell session - when focused on sessions panel and on a project
			if m.focusArea == FocusDetail && m.shellTreeMenu != nil {
				item := m.shellTreeMenu.SelectedItem()
				if item != nil && len(item.Children) > 0 {
					// Create new session for selected project (has children = category)
					return m.createShellSession("project", item.ID, item.Label)
				}
			}
			return nil
		case "h":
			// New home directory shell session
			return m.createShellSession("home", "", "")
		case "s":
			// New sudo root shell session (if sudo is available)
			if m.state.Capabilities != nil && m.state.Capabilities.HasSudo() {
				return m.createShellSession("sudo", "", "")
			} else {
				m.lastError = "sudo not available"
				m.lastErrorTime = time.Now()
			}
			return nil
		case "x":
			// Delete selected session
			if m.focusArea == FocusDetail && m.shellTreeMenu != nil {
				item := m.shellTreeMenu.SelectedItem()
				if item != nil && len(item.Children) == 0 && item.ID != "" {
					m.dialogType = "delete_shell_session"
					m.dialogMessage = fmt.Sprintf("Delete session \"%s\"?", item.Label)
					m.showDialog = true
				}
			}
			return nil
		case "d":
			// Disconnect shell terminal
			if m.shellActiveSession != "" {
				return m.stopShellTerminal(m.shellActiveSession)
			}
			return nil
		case "e":
			// Edit shell for selected session (cycle through available shells)
			// Only if there's no terminal running for this session
			if m.focusArea == FocusDetail && m.shellTreeMenu != nil {
				item := m.shellTreeMenu.SelectedItem()
				if item != nil && len(item.Children) == 0 && item.ID != "" {
					// Check if terminal is running
					if t := m.terminalManager.Get(item.ID); t != nil && t.IsRunning() {
						m.lastError = "Cannot change shell while terminal is running"
						m.lastErrorTime = time.Now()
						return nil
					}
					// Check if we have more than one shell available
					if m.state.Shell != nil && len(m.state.Shell.AvailableShells) <= 1 {
						m.lastError = "Only one shell available"
						m.lastErrorTime = time.Now()
						return nil
					}
					// Cycle shell via presenter event
					event := core.NewEvent(core.EventShellCycleShell).
						WithData("session_id", item.ID)
					return func() tea.Msg {
						m.presenter.HandleEvent(event)
						return refreshMsg{}
					}
				}
			}
			return nil
		case "enter":
			// Enter terminal mode when focused on terminal panel
			if m.focusArea == FocusMain && m.shellActiveSession != "" {
				if t := m.terminalManager.Get(m.shellActiveSession); t != nil && t.IsRunning() {
					m.terminalMode = true
					m.commandMode = false
				}
			}
			// Select session from tree menu
			if m.focusArea == FocusDetail && m.shellTreeMenu != nil {
				item := m.shellTreeMenu.SelectedItem()
				if item != nil && len(item.Children) == 0 && item.ID != "" {
					return m.switchToShellSession(item.ID)
				}
			}
			return nil
		case "esc":
			// Exit terminal mode or switch focus
			if m.terminalMode {
				m.terminalMode = false
				return nil
			}
			if m.focusArea == FocusDetail {
				m.focusArea = FocusMain
				return nil
			}
			return nil
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

// getSelectedTreeItem returns the selected item from the sessions TreeMenu
// Returns (projectID, sessionID, isProject, hasTerminal)
func (m *Model) getSelectedTreeItem() (string, string, bool, bool) {
	if m.sessionsTreeMenu == nil {
		return "", "", false, false
	}

	item := m.sessionsTreeMenu.SelectedItem()
	if item == nil {
		return "", "", false, false
	}

	// Check if it's a project (marked with "project" in Data, or has children)
	if item.Data == "project" || len(item.Children) > 0 {
		// It's a project
		return item.ID, "", true, false
	}

	// It's a session - check if it has terminal attached
	hasTerminal := false
	if m.terminalManager != nil {
		if t := m.terminalManager.Get(item.ID); t != nil && t.IsRunning() {
			hasTerminal = true
		}
	}

	// Get project ID from session data
	projectID := ""
	if sess, ok := item.Data.(core.ClaudeSessionVM); ok {
		projectID = sess.ProjectID
	}

	return projectID, item.ID, false, hasTerminal
}

// generateDefaultSessionName generates a default session name for a project
func (m *Model) generateDefaultSessionName(projectID string) string {
	// Count existing sessions for this project
	count := 0
	if m.state.Claude != nil {
		for _, sess := range m.state.Claude.Sessions {
			if sess.ProjectID == projectID {
				count++
			}
		}
	}
	return fmt.Sprintf("session-%d", count+1)
}

// createClaudeSessionWithName creates a new Claude session with a specific name
func (m *Model) createClaudeSessionWithName(projectID, name string) tea.Cmd {
	event := core.NewEvent(core.EventClaudeCreateSession).WithProject(projectID)
	event.Data["session_name"] = name
	return m.sendEvent(event)
}

// createClaudeSession creates a new Claude session for the selected project in tree
func (m *Model) createClaudeSession() tea.Cmd {
	projectID, _, isProject, _ := m.getSelectedTreeItem()
	if projectID != "" {
		return m.sendEvent(core.NewEvent(core.EventClaudeCreateSession).WithProject(projectID))
	}

	// Legacy fallback
	if m.mainIndex >= 0 && m.mainIndex < len(m.claudeTreeItems) {
		item := m.claudeTreeItems[m.mainIndex]
		return m.sendEvent(core.NewEvent(core.EventClaudeCreateSession).WithProject(item.ProjectID))
	}
	_ = isProject // unused

	// Fallback to filter project
	if m.claudeFilterProject != "" {
		return m.sendEvent(core.NewEvent(core.EventClaudeCreateSession).WithProject(m.claudeFilterProject))
	}

	// No project context - select first project if available
	if m.state.Projects != nil && len(m.state.Projects.Projects) > 0 {
		return m.sendEvent(core.NewEvent(core.EventClaudeCreateSession).WithProject(m.state.Projects.Projects[0].ID))
	}

	m.lastError = "No projects available"
	m.lastErrorTime = time.Now()
	return nil
}

// selectSessionInTree finds and selects a session in the sessions tree menu
func (m *Model) selectSessionInTree(sessionID string) {
	if m.sessionsTreeMenu == nil || sessionID == "" {
		return
	}

	// Find the session's project ID
	var projectID string
	if m.state.Claude != nil {
		for _, sess := range m.state.Claude.Sessions {
			if sess.ID == sessionID {
				projectID = sess.ProjectID
				break
			}
		}
	}

	if projectID == "" {
		return
	}

	m.selectSessionInTreeWithProject(sessionID, projectID)
}

// selectSessionInTreeWithProject finds and selects a session in the tree using known project ID
func (m *Model) selectSessionInTreeWithProject(sessionID, projectID string) {
	if m.sessionsTreeMenu == nil || sessionID == "" || projectID == "" {
		return
	}

	// Navigate to the session:
	// 1. First, go to root level
	m.sessionsTreeMenu.DrillUp()
	for len(m.sessionsTreeMenu.DrillDownPath()) > 0 {
		m.sessionsTreeMenu.DrillUp()
	}

	// 2. Find and select the project
	items := m.sessionsTreeMenu.Items()
	for i, item := range items {
		if item.ID == projectID {
			m.sessionsTreeMenu.SetSelectedIndex(i)
			// 3. Drill into the project
			m.sessionsTreeMenu.DrillDown()
			break
		}
	}

	// 4. Find and select the session within the project
	sessionItems := m.sessionsTreeMenu.VisibleItems()
	for i, item := range sessionItems {
		if item.ID == sessionID {
			// Account for back item (index 0)
			m.sessionsTreeMenu.SetSelectedIndex(i + 1)
			break
		}
	}
}

// openClaudeSession opens the selected session in chat mode
func (m *Model) openClaudeSession() tea.Cmd {
	if m.state.Claude == nil || len(m.state.Claude.Sessions) == 0 {
		return nil
	}
	if m.mainIndex >= 0 && m.mainIndex < len(m.state.Claude.Sessions) {
		sess := m.state.Claude.Sessions[m.mainIndex]
		m.claudeActiveSession = sess.ID
		m.claudeMode = ClaudeModeChat
		m.claudeSessionLoading = true
		m.claudeChatScroll = 0

		// Automatically activate input mode when opening a session
		m.claudeInputActive = true
		m.claudeTextInput.Focus()

		// Send select event, start cursor blink, and trigger spinner
		return tea.Batch(
			m.sendEvent(core.NewEvent(core.EventClaudeSelectSession).WithValue(sess.ID)),
			m.claudeTextInput.Cursor.BlinkCmd(),
			m.spinner.Tick,
		)
	}
	return nil
}

// switchToSelectedSession switches to the session selected in the tree
// Uses TreeMenu for navigation: Enter on project drills in, Enter on session opens it
func (m *Model) switchToSelectedSession() tea.Cmd {
	if m.sessionsTreeMenu == nil {
		return nil
	}

	// Check if back item is selected
	if m.sessionsTreeMenu.IsBackSelected() {
		m.sessionsTreeMenu.DrillUp()
		return nil
	}

	// Get selected item from TreeMenu
	treeItem := m.sessionsTreeMenu.SelectedItem()
	if treeItem == nil {
		return nil
	}

	// Check if it's a project (has "project" marker in Data or has children)
	isProject := false
	if dataStr, ok := treeItem.Data.(string); ok && dataStr == "project" {
		isProject = true
	}
	if isProject || len(treeItem.Children) > 0 {
		// Project selected - drill into it to show sessions
		// Only drill if there are children (sessions)
		if len(treeItem.Children) > 0 {
			m.sessionsTreeMenu.DrillDown()
		}
		// Either way, don't open a session
		return nil
	}

	// Verify it's actually a session (Data should be ClaudeSessionVM, not a string)
	if _, isSession := treeItem.Data.(core.ClaudeSessionVM); !isSession {
		// Not a session, do nothing
		return nil
	}

	// Session selected - switch to it and start terminal
	sessionID := treeItem.ID
	m.claudeActiveSession = sessionID
	m.claudeMode = ClaudeModeChat

	// Switch focus to terminal
	m.focusArea = FocusMain

	// Get or create terminal for this session
	workDir := ""
	claudeProjectDir := ""
	if m.state.Claude != nil {
		for _, sess := range m.state.Claude.Sessions {
			if sess.ID == sessionID {
				workDir = sess.WorkDir
				claudeProjectDir = sess.ClaudeProjectDir
				break
			}
		}
	}
	if workDir == "" {
		// Default to current dir
		workDir, _ = os.Getwd()
	}

	t := m.terminalManager.GetOrCreate(sessionID, workDir, claudeProjectDir)

	// Set terminal size
	headerHeight := 3
	footerHeight := 3
	sidebarWidth := getSidebarWidth()
	termWidth := m.width - sidebarWidth - 6
	termHeight := m.height - headerHeight - footerHeight - 2
	if termWidth > 20 && termHeight > 5 {
		t.SetSize(termWidth, termHeight)
	}

	// Start Claude if not already running
	if !t.IsRunning() {
		if err := t.Start(sessionID); err != nil {
			m.lastError = "Failed to start Claude: " + err.Error()
			m.lastErrorTime = time.Now()
			return nil
		}
	}

	// Enter terminal mode - reset conflicting input modes
	m.terminalMode = true
	m.claudeInputActive = false
	m.claudeRenameActive = false
	m.commandMode = false

	// Start terminal refresh loop
	return m.scheduleTerminalRefresh()
}

// switchToSessionByID switches to a specific session by ID (used by TreeMenu)
func (m *Model) switchToSessionByID(sessionID string) tea.Cmd {
	// Session selected - switch to it and start terminal
	m.claudeActiveSession = sessionID
	m.claudeMode = ClaudeModeChat

	// Switch focus to terminal
	m.focusArea = FocusMain

	// Get work directory and ClaudeProjectDir for this session
	workDir := ""
	claudeProjectDir := ""
	if m.state.Claude != nil {
		for _, sess := range m.state.Claude.Sessions {
			if sess.ID == sessionID {
				workDir = sess.WorkDir
				claudeProjectDir = sess.ClaudeProjectDir
				break
			}
		}
	}
	if workDir == "" {
		// Default to current dir
		workDir, _ = os.Getwd()
	}

	t := m.terminalManager.GetOrCreate(sessionID, workDir, claudeProjectDir)

	// Set terminal size
	headerHeight := 3
	footerHeight := 3
	sidebarWidth := getSidebarWidth()
	termWidth := m.width - sidebarWidth - 6
	termHeight := m.height - headerHeight - footerHeight - 2
	if termWidth > 20 && termHeight > 5 {
		t.SetSize(termWidth, termHeight)
	}

	// Start Claude if not already running
	if !t.IsRunning() {
		if err := t.Start(sessionID); err != nil {
			m.lastError = "Failed to start Claude: " + err.Error()
			m.lastErrorTime = time.Now()
			return nil
		}
	}

	// Enter terminal mode - reset conflicting input modes
	m.terminalMode = true
	m.claudeInputActive = false
	m.claudeRenameActive = false
	m.commandMode = false

	// Start terminal refresh loop
	return m.scheduleTerminalRefresh()
}

// stopClaudeTerminal stops the tmux terminal for a session
func (m *Model) stopClaudeTerminal(sessionID string) tea.Cmd {
	if m.terminalManager == nil {
		return nil
	}

	t := m.terminalManager.Get(sessionID)
	if t == nil {
		return nil
	}

	// Stop the terminal in goroutine to avoid blocking UI
	go t.Stop()

	// If this was the active terminal, exit terminal mode
	if m.claudeActiveSession == sessionID && m.terminalMode {
		m.terminalMode = false
	}

	// Show feedback
	m.lastError = "Terminal stopped"
	m.lastErrorTime = time.Now()

	return nil
}

// ============================================
// Database view helper functions
// ============================================

// connectToDatabase starts a terminal directly for a database config
func (m *Model) connectToDatabase(databaseID string) tea.Cmd {
	if m.terminalManager == nil || m.state.Database == nil {
		return nil
	}

	// Find the database info
	var db *core.DatabaseInfoVM
	for i := range m.state.Database.Databases {
		if m.state.Database.Databases[i].ID == databaseID {
			db = &m.state.Database.Databases[i]
			break
		}
	}

	if db == nil {
		m.lastError = "Database not found: " + databaseID
		m.lastErrorTime = time.Now()
		return nil
	}

	// Get the CLI command based on database type
	cliCmd, cliArgs := m.getDatabaseCLICommand(db)
	if cliCmd == "" {
		m.lastError = "Unknown database type: " + db.Type
		m.lastErrorTime = time.Now()
		return nil
	}

	// Use database ID as the terminal session ID
	m.databaseActiveSession = databaseID

	// Switch focus to terminal
	m.focusArea = FocusMain

	// Get or create terminal
	t := m.terminalManager.GetOrCreateWithCommand(databaseID, cliCmd, cliArgs)

	// Size the terminal appropriately
	headerHeight := 3
	footerHeight := 3
	sidebarWidth := getSidebarWidth()
	termWidth := m.width - sidebarWidth - 6
	termHeight := m.height - headerHeight - footerHeight - 2
	if termWidth > 20 && termHeight > 5 {
		t.SetSize(termWidth, termHeight)
	}

	// Start if not already running
	if !t.IsRunning() {
		if err := t.Start(databaseID); err != nil {
			m.lastError = "Failed to start database CLI: " + err.Error()
			m.lastErrorTime = time.Now()
			return nil
		}
	}

	// Enter terminal mode - reset conflicting input modes
	m.terminalMode = true
	m.claudeInputActive = false
	m.claudeRenameActive = false
	m.commandMode = false

	// Header event
	m.state.SetHeaderEvent(core.NewHeaderEvent(core.HeaderEventSuccess, "Connected to "+db.DatabaseName))

	return m.scheduleTerminalRefresh()
}

// stopDatabaseTerminal stops the database CLI terminal
func (m *Model) stopDatabaseTerminal(databaseID string) tea.Cmd {
	if m.terminalManager == nil {
		return nil
	}

	t := m.terminalManager.Get(databaseID)
	if t == nil {
		return nil
	}

	// Stop the terminal
	go t.Stop()

	// Exit terminal mode if this was active
	if m.databaseActiveSession == databaseID && m.terminalMode {
		m.terminalMode = false
	}

	// Clear active session
	if m.databaseActiveSession == databaseID {
		m.databaseActiveSession = ""
	}

	// Header event
	m.state.SetHeaderEvent(core.NewHeaderEvent(core.HeaderEventInfo, "Database disconnected"))

	return nil
}

// getDatabaseCLICommand returns the CLI command and args for a database type
func (m *Model) getDatabaseCLICommand(db *core.DatabaseInfoVM) (string, []string) {
	switch db.Type {
	case "postgres":
		// Build postgres URL from components
		url := fmt.Sprintf("postgres://%s@%s:%d/%s", db.User, db.Host, db.Port, db.DatabaseName)
		return "psql", []string{url}
	case "mysql":
		return "mysql", []string{
			"-h", db.Host,
			"-P", fmt.Sprintf("%d", db.Port),
			"-u", db.User,
			db.DatabaseName,
		}
	case "sqlite":
		return "sqlite3", []string{db.DatabaseName}
	default:
		return "", nil
	}
}

// ============================================
// Shell Session Functions
// ============================================

// createShellSession creates a new shell session
func (m *Model) createShellSession(sessionType, projectID, projectName string) tea.Cmd {
	if m.terminalManager == nil {
		return nil
	}

	// Get shell path
	shellPath := "/bin/bash"
	if m.state.Capabilities != nil && m.state.Capabilities.Shell.Path != "" {
		shellPath = m.state.Capabilities.Shell.Path
	}

	// Determine work directory
	var workDir string
	var sessionName string
	var args []string

	switch sessionType {
	case "home":
		workDir, _ = os.UserHomeDir()
		sessionName = "Home"
	case "sudo":
		workDir = "/root"
		sessionName = "Root (sudo)"
		// Use sudo -i to get root shell
		args = []string{"-i"}
		shellPath = "sudo"
	case "project":
		// Get project path from state
		if m.state.Projects != nil {
			for _, proj := range m.state.Projects.Projects {
				if proj.ID == projectID {
					workDir = proj.Path
					sessionName = proj.Name
					break
				}
			}
		}
		if workDir == "" {
			workDir, _ = os.UserHomeDir()
			sessionName = projectName
		}
	default:
		workDir, _ = os.UserHomeDir()
		sessionName = "Shell"
	}

	// Create session ID
	sessionID := fmt.Sprintf("shell-%s-%d", sessionType, time.Now().UnixNano())

	// Create terminal using TmuxPrefixShell prefix
	var t TerminalInterface
	if sessionType == "sudo" {
		t = m.terminalManager.GetOrCreateCommandWithPrefix(sessionID, shellPath, args, TmuxPrefixShell)
	} else {
		// For regular shell, use a command-based approach
		t = m.terminalManager.GetOrCreateCommandWithPrefix(sessionID, shellPath, []string{}, TmuxPrefixShell)
	}

	if t == nil {
		m.lastError = "Failed to create shell terminal"
		m.lastErrorTime = time.Now()
		return nil
	}

	// Start the terminal
	if err := t.Start(sessionID); err != nil {
		m.lastError = fmt.Sprintf("Failed to start shell: %v", err)
		m.lastErrorTime = time.Now()
		return nil
	}

	// Switch to this session
	m.shellActiveSession = sessionID
	m.focusArea = FocusMain

	// Header event
	m.state.SetHeaderEvent(core.NewHeaderEvent(core.HeaderEventSuccess, "Shell started: "+sessionName))

	// Update tree menu
	m.updateShellTree()

	return m.scheduleTerminalRefresh()
}

// stopShellTerminal stops the shell terminal
func (m *Model) stopShellTerminal(sessionID string) tea.Cmd {
	if m.terminalManager == nil {
		return nil
	}

	t := m.terminalManager.Get(sessionID)
	if t == nil {
		return nil
	}

	// Stop the terminal
	go t.Stop()

	// Exit terminal mode if this was active
	if m.shellActiveSession == sessionID && m.terminalMode {
		m.terminalMode = false
	}

	// Clear active session
	if m.shellActiveSession == sessionID {
		m.shellActiveSession = ""
	}

	// Header event
	m.state.SetHeaderEvent(core.NewHeaderEvent(core.HeaderEventInfo, "Shell disconnected"))

	// Update tree menu
	m.updateShellTree()

	return nil
}

// switchToShellSession switches to an existing shell session
func (m *Model) switchToShellSession(sessionID string) tea.Cmd {
	if m.terminalManager == nil {
		return nil
	}

	t := m.terminalManager.Get(sessionID)
	if t == nil || !t.IsRunning() {
		m.lastError = "Session not running"
		m.lastErrorTime = time.Now()
		return nil
	}

	m.shellActiveSession = sessionID
	m.focusArea = FocusMain
	m.updateShellTree()

	return m.scheduleTerminalRefresh()
}

// updateShellTree updates the shell sessions tree menu
func (m *Model) updateShellTree() {
	if m.state.Shell == nil {
		return
	}

	// Create root items for special sessions and projects
	var items []TreeMenuItem

	// Special sessions category (Home, Root)
	var specialItems []TreeMenuItem

	// Check for active home and sudo sessions in terminal manager
	if m.terminalManager != nil {
		for _, sessionID := range m.terminalManager.GetRunning() {
			if strings.HasPrefix(sessionID, "shell-home-") {
				specialItems = append(specialItems, TreeMenuItem{
					ID:       sessionID,
					Label:    "Home",
					Icon:     "~",
					IsActive: sessionID == m.shellActiveSession,
				})
			} else if strings.HasPrefix(sessionID, "shell-sudo-") {
				specialItems = append(specialItems, TreeMenuItem{
					ID:       sessionID,
					Label:    "Root (sudo)",
					Icon:     "#",
					IsActive: sessionID == m.shellActiveSession,
				})
			}
		}
	}

	if len(specialItems) > 0 {
		items = append(items, TreeMenuItem{
			ID:       "_special",
			Label:    "Special",
			Children: specialItems,
		})
	}

	// Project sessions
	projectSessionMap := make(map[string][]TreeMenuItem)

	if m.terminalManager != nil {
		for _, sessionID := range m.terminalManager.GetRunning() {
			if strings.HasPrefix(sessionID, "shell-project-") {
				// Extract project from session (need to track this better)
				parts := strings.Split(sessionID, "-")
				if len(parts) >= 3 {
					projectID := parts[2] // This is simplified, may need improvement
					projectSessionMap[projectID] = append(projectSessionMap[projectID], TreeMenuItem{
						ID:       sessionID,
						Label:    "Shell",
						Icon:     "$",
						IsActive: sessionID == m.shellActiveSession,
					})
				}
			}
		}
	}

	// Add projects with their sessions
	if m.state.Projects != nil {
		for _, proj := range m.state.Projects.Projects {
			projSessions := projectSessionMap[proj.ID]
			projItem := TreeMenuItem{
				ID:       proj.ID,
				Label:    proj.Name,
				Children: projSessions,
			}
			items = append(items, projItem)
		}
	}

	// Create or update tree menu
	if m.shellTreeMenu == nil {
		m.shellTreeMenu = NewTreeMenu(items)
		m.shellTreeMenu.SetSize(30, 20)
	} else {
		m.shellTreeMenu.SetItems(items)
	}
}

// handleClaudeInput handles text input in Claude chat mode
// Controls:
//   - Enter: send message, stay in input mode
//   - Escape: interrupt current Claude request (if processing)
//   - Double-Escape (within 500ms): exit input mode
func (m *Model) handleClaudeInput(msg tea.KeyMsg) tea.Cmd {
	keyStr := msg.String()

	// Ctrl+C: exit input mode (always)
	if keyStr == "ctrl+c" {
		m.claudeInputActive = false
		m.claudeTextInput.Blur()
		return nil
	}

	switch msg.Type {
	case tea.KeyEscape:
		now := time.Now()
		// Double-ESC detection: if last ESC was within 500ms, exit input mode
		if now.Sub(m.claudeLastEscTime) < 500*time.Millisecond {
			m.claudeInputActive = false
			m.claudeTextInput.Blur()
			m.focusArea = FocusDetail // Switch to sessions panel
			m.claudeLastEscTime = time.Time{} // Reset
			return nil
		}
		m.claudeLastEscTime = now

		// Single ESC: interrupt current request if processing
		if m.state.Claude != nil && m.state.Claude.IsProcessing {
			return m.sendEvent(core.NewEvent(core.EventClaudeStopSession).WithValue(m.claudeActiveSession))
		}
		// Not processing - wait for potential second ESC
		return nil
	case tea.KeyEnter:
		message := m.claudeTextInput.Value()
		if message == "" {
			return nil
		}
		// Clear input immediately for responsiveness
		m.claudeTextInput.Reset()

		// Add user message to UI state IMMEDIATELY (before event processing)
		// This gives instant visual feedback
		if m.state.Claude != nil {
			now := time.Now()
			userMsg := core.ClaudeMessageVM{
				ID:        "user-" + now.Format("20060102150405.000"),
				Role:      "user",
				Content:   message,
				Timestamp: now,
				TimeStr:   now.Format("060102 - 15:04:05"),
			}
			m.state.Claude.Messages = append(m.state.Claude.Messages, userMsg)

			// Add placeholder for assistant response
			assistantMsg := core.ClaudeMessageVM{
				ID:        "assistant-" + now.Format("20060102150405.000"),
				Role:      "assistant",
				Content:   "",
				Timestamp: now,
				TimeStr:   now.Format("060102 - 15:04:05"),
				IsPartial: true,
			}
			m.state.Claude.Messages = append(m.state.Claude.Messages, assistantMsg)
			m.state.Claude.IsProcessing = true

			// Reset scroll to bottom to show new messages
			m.claudeChatScroll = 0
		}

		// Send event to presenter (async processing) and start refresh loop
		return tea.Batch(
			m.sendEvent(core.NewEvent(core.EventClaudeSendMessage).
				WithData("session_id", m.claudeActiveSession).
				WithData("message", message)),
			claudeRefreshCmd(),
		)
	default:
		// Let textinput handle all other keys
		var cmd tea.Cmd
		m.claudeTextInput, cmd = m.claudeTextInput.Update(msg)
		return cmd
	}
}

// handleClaudeRenameInput handles text input for renaming Claude sessions
func (m *Model) handleClaudeRenameInput(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyEscape:
		m.claudeRenameActive = false
		m.sessionsTreeMenu.SetRenameActive(false)
		return nil
	case tea.KeyEnter:
		newName := m.sessionsTreeMenu.RenameText()
		if newName == "" {
			m.claudeRenameActive = false
			m.sessionsTreeMenu.SetRenameActive(false)
			return nil
		}
		// Rename session
		m.claudeRenameActive = false
		m.sessionsTreeMenu.SetRenameActive(false)
		// Get selected session ID from TreeMenu
		_, sessionID, isProject, _ := m.getSelectedTreeItem()
		if isProject || sessionID == "" {
			return nil
		}
		return m.sendEvent(core.NewEvent(core.EventClaudeRenameSession).
			WithData("session_id", sessionID).
			WithData("new_name", newName))
	case tea.KeyBackspace:
		m.sessionsTreeMenu.BackspaceRenameText()
		return nil
	case tea.KeySpace:
		m.sessionsTreeMenu.AppendRenameText(" ")
		return nil
	case tea.KeyRunes:
		m.sessionsTreeMenu.AppendRenameText(string(msg.Runes))
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
	// Input dialogs have special handling
	if m.dialogInputActive {
		switch msg.Type {
		case tea.KeyEnter:
			m.showDialog = false
			m.dialogInputActive = false
			m.dialogInput.Blur()
			return m.handleDialogConfirm()
		case tea.KeyEscape:
			m.showDialog = false
			m.dialogInputActive = false
			m.dialogInput.Blur()
			m.pendingNewSessionProjectID = ""
			return nil
		default:
			// Pass other keys to the input
			var cmd tea.Cmd
			m.dialogInput, cmd = m.dialogInput.Update(msg)
			return cmd
		}
	}

	// Regular confirmation dialogs
	switch msg.String() {
	case "y", "Y":
		// Direct yes - always confirm
		m.showDialog = false
		return m.handleDialogConfirm()
	case "enter":
		// Enter confirms only if Yes is selected (dialogConfirm = true)
		m.showDialog = false
		if m.dialogConfirm {
			return m.handleDialogConfirm()
		}
		return nil
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
	case "delete_claude_session":
		// Delete the selected Claude session
		_, sessionID, isProject, _ := m.getSelectedTreeItem()
		if !isProject && sessionID != "" {
			// Mark session as deleting for visual feedback
			m.deletingSessions[sessionID] = true

			// Update tree immediately so the Disabled flag is set
			m.updateClaudeTree()

			// Move selection away from deleting session
			if m.sessionsTreeMenu != nil {
				m.sessionsTreeMenu.MoveAwayFromDisabled()
			}

			// Reset active session if deleting it
			if m.claudeActiveSession == sessionID {
				m.claudeActiveSession = ""
			}
			// Stop terminal and kill tmux in goroutine to avoid blocking UI
			tm := m.terminalManager
			go func() {
				if tm != nil {
					if t := tm.Get(sessionID); t != nil {
						t.Stop()
					}
				}
				// Also kill any persistent tmux session
				KillTmuxSession(sessionID)
			}()
			return m.sendEvent(core.NewEvent(core.EventClaudeDeleteSession).WithValue(sessionID))
		}
		return nil
	case "new_claude_session":
		// Create a new Claude session with the entered name
		if m.pendingNewSessionProjectID != "" {
			sessionName := strings.TrimSpace(m.dialogInput.Value())
			if sessionName == "" {
				sessionName = m.generateDefaultSessionName(m.pendingNewSessionProjectID)
			}
			projectID := m.pendingNewSessionProjectID
			m.pendingNewSessionProjectID = ""
			return m.createClaudeSessionWithName(projectID, sessionName)
		}
		return nil
	case "delete_cockpit_profile":
		// Delete the current cockpit profile
		m.deleteCockpitProfile()
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
	}

	// Scroll controls - only when focus is on Main panel
	if m.focusArea != FocusMain {
		return false
	}

	switch key {
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
// Returns true if a terminal refresh loop should be started (new session created)
func (m *Model) handleStateUpdate(update core.StateUpdate) bool {
	m.state.UpdateViewModel(update.ViewModel)

	// Sync global state flags from presenter
	if presenterState := m.presenter.GetState(); presenterState != nil {
		m.state.Initializing = presenterState.Initializing
		m.state.GitLoading = presenterState.GitLoading
		m.state.Capabilities = presenterState.Capabilities

		// Sync header event
		if headerEvent := presenterState.GetHeaderEvent(); headerEvent != nil {
			m.state.SetHeaderEvent(headerEvent)
		}
	}

	// Always update log source options (logs can come from any view update)
	m.updateLogSourceOptions()

	// Update max items counts
	m.updateItemCounts()

	// Update Claude tree for navigation (must persist across Update calls)
	m.updateClaudeTree()

	// Handle newly created session - just select it, don't start terminal yet
	needTerminalRefresh := false
	if m.state.Claude != nil && m.state.Claude.NewlyCreatedSessionID != "" {
		sessionID := m.state.Claude.NewlyCreatedSessionID

		// Set as active session (shows in chat panel)
		m.claudeActiveSession = sessionID
		m.claudeMode = ClaudeModeChat

		// Don't start terminal automatically - user will start it when they want to interact
		// Terminal is started when user presses Enter or types in the session
	}

	// Update Git menu for navigation
	m.updateGitMenu()

	// Update Projects menu for navigation
	m.updateProjectsMenu()

	// Update Processes menu for navigation
	m.updateProcessesMenu()

	// Update Database menu for navigation
	m.updateDatabaseMenu()

	// Update sidebar menu
	m.updateSidebarMenu()

	// Clear session loading state when the requested session data is received
	if m.claudeSessionLoading && m.state.Claude != nil && m.state.Claude.ActiveSessionID == m.claudeActiveSession {
		m.claudeSessionLoading = false
	}

	// Auto-exit input mode when Claude is waiting for interactive response
	// This allows y/n/1-9 keys to work for permission/question/plan dialogs
	if m.state.Claude != nil && m.state.Claude.WaitingForInput && m.claudeInputActive {
		m.claudeInputActive = false
		m.claudeTextInput.Blur()
	}

	return needTerminalRefresh
}

// updateSidebarMenu updates the sidebar TreeMenu items
func (m *Model) updateSidebarMenu() {
	if m.sidebarMenu == nil {
		return
	}

	views := m.getSidebarViews()
	var items []TreeMenuItem

	for _, v := range views {
		items = append(items, TreeMenuItem{
			ID:       string(v.vtype),
			Label:    v.name,
			IsActive: m.currentView == v.vtype,
			Data:     v.vtype,
		})
	}

	m.sidebarMenu.SetItems(items)
}

// updateGitMenu updates the Git view TreeMenu with projects and their files
func (m *Model) updateGitMenu() {
	if m.gitMenu == nil || m.state.Git == nil {
		return
	}

	var items []TreeMenuItem

	for _, p := range m.state.Git.Projects {
		// Create children for each file status category
		var children []TreeMenuItem

		// Staged files
		for _, f := range p.Staged {
			children = append(children, TreeMenuItem{
				ID:    p.ProjectName + ":staged:" + f,
				Label: f,
				Icon:  "A",
				Data:  GitFileEntry{Path: f, Status: "staged"},
			})
		}

		// Modified files
		for _, f := range p.Modified {
			children = append(children, TreeMenuItem{
				ID:    p.ProjectName + ":modified:" + f,
				Label: f,
				Icon:  "M",
				Data:  GitFileEntry{Path: f, Status: "modified"},
			})
		}

		// Deleted files
		for _, f := range p.Deleted {
			children = append(children, TreeMenuItem{
				ID:    p.ProjectName + ":deleted:" + f,
				Label: f,
				Icon:  "D",
				Data:  GitFileEntry{Path: f, Status: "deleted"},
			})
		}

		// Untracked files
		for _, f := range p.Untracked {
			children = append(children, TreeMenuItem{
				ID:    p.ProjectName + ":untracked:" + f,
				Label: f,
				Icon:  "?",
				Data:  GitFileEntry{Path: f, Status: "untracked"},
			})
		}

		// Build project status indicator
		statusIcon := "●"
		if p.IsClean {
			statusIcon = "✓"
		}

		// Count of changes
		changeCount := len(p.Staged) + len(p.Modified) + len(p.Deleted) + len(p.Untracked)

		items = append(items, TreeMenuItem{
			ID:       p.ProjectName,
			Label:    p.ProjectName,
			Icon:     statusIcon,
			Children: children,
			Count:    changeCount,
			Data:     p,
		})
	}

	m.gitMenu.SetItems(items)
}

// updateProjectsMenu updates the projects TreeMenu with current project data
func (m *Model) updateProjectsMenu() {
	if m.projectsMenu == nil || m.state.Projects == nil {
		return
	}

	var items []TreeMenuItem

	for _, p := range m.state.Projects.Projects {
		// Create children for each component
		var children []TreeMenuItem

		for _, comp := range p.Components {
			// Status indicator
			statusIcon := ""
			if comp.IsRunning {
				statusIcon = "●"
			}

			children = append(children, TreeMenuItem{
				ID:           p.ID + ":" + string(comp.Type),
				Label:        string(comp.Type),
				TrailingIcon: statusIcon,
				Data:         comp,
			})
		}

		// Project status: count running components
		runningCount := 0
		for _, comp := range p.Components {
			if comp.IsRunning {
				runningCount++
			}
		}

		// Project icon based on status
		projectIcon := ""
		if p.IsSelf {
			projectIcon = "*"
		}

		// Trailing icon for running indicator
		trailingIcon := ""
		if runningCount > 0 {
			trailingIcon = "●"
		}

		items = append(items, TreeMenuItem{
			ID:           p.ID,
			Label:        p.Name,
			Icon:         projectIcon,
			TrailingIcon: trailingIcon,
			Children:     children,
			Count:        len(p.Components),
			Data:         p,
		})
	}

	m.projectsMenu.SetItems(items)
}

// updateProcessesMenu updates the processes TreeMenu with current process data
func (m *Model) updateProcessesMenu() {
	if m.processesMenu == nil || m.state.Processes == nil {
		return
	}

	var items []TreeMenuItem

	// Group processes by project
	projectProcesses := make(map[string][]core.ProcessVM)
	var projectOrder []string

	for _, proc := range m.state.Processes.Processes {
		if _, exists := projectProcesses[proc.ProjectName]; !exists {
			projectOrder = append(projectOrder, proc.ProjectName)
		}
		projectProcesses[proc.ProjectName] = append(projectProcesses[proc.ProjectName], proc)
	}

	for _, projectName := range projectOrder {
		procs := projectProcesses[projectName]
		var children []TreeMenuItem

		for _, proc := range procs {
			// Status indicator
			statusIcon := ""
			if proc.State == "running" {
				statusIcon = "●"
			}

			children = append(children, TreeMenuItem{
				ID:           proc.ID,
				Label:        string(proc.Component),
				TrailingIcon: statusIcon,
				Data:         proc,
			})
		}

		// Count running
		runningCount := 0
		for _, proc := range procs {
			if proc.State == "running" {
				runningCount++
			}
		}

		trailingIcon := ""
		if runningCount > 0 {
			trailingIcon = "●"
		}

		items = append(items, TreeMenuItem{
			ID:           projectName,
			Label:        projectName,
			TrailingIcon: trailingIcon,
			Children:     children,
			Count:        len(children),
		})
	}

	m.processesMenu.SetItems(items)
}

// updateDatabaseMenu updates the database TreeMenu with current database data
func (m *Model) updateDatabaseMenu() {
	if m.databaseTreeMenu == nil || m.state.Database == nil {
		return
	}

	items := m.buildDatabaseTreeItems()
	m.databaseTreeMenu.SetItems(items)
}

// updateClaudeTree builds the flattened tree structure for Claude sessions navigation
func (m *Model) updateClaudeTree() {
	// Clean up deletingSessions map: remove IDs that are no longer in the sessions list
	if m.state.Claude != nil && len(m.deletingSessions) > 0 {
		existingIDs := make(map[string]bool)
		for _, sess := range m.state.Claude.Sessions {
			existingIDs[sess.ID] = true
		}
		for id := range m.deletingSessions {
			if !existingIDs[id] {
				delete(m.deletingSessions, id)
			}
		}
	}

	// Build tree: group sessions by project
	// Show ALL registered projects, even those without sessions
	type projectNode struct {
		ID       string
		Name     string
		Path     string
		Sessions []core.ClaudeSessionVM
	}

	projectMap := make(map[string]*projectNode)
	var projectOrder []string

	// Add ALL registered projects to the tree
	if m.state.Projects != nil {
		for _, proj := range m.state.Projects.Projects {
			node := &projectNode{
				ID:       proj.ID,
				Name:     proj.Name,
				Path:     proj.Path,
				Sessions: []core.ClaudeSessionVM{},
			}
			projectMap[proj.ID] = node
			projectOrder = append(projectOrder, proj.ID)
		}
	}

	// Add sessions to their matching projects
	// Sessions in subdirectories should be matched to their parent project
	if m.state.Claude != nil {
		for _, sess := range m.state.Claude.Sessions {
			// Try to match session with a registered project (exact match only)
			matched := false
			var bestMatch *projectNode

			for _, node := range projectMap {
				// First, try exact ProjectID match (most reliable)
				if sess.ProjectID != "" && sess.ProjectID == node.ID {
					node.Sessions = append(node.Sessions, sess)
					matched = true
					break
				}

				// Primary: Use WorkDir (cwd from JSONL) for matching
				// This is the most reliable source as it's the actual directory where the session was used
				if sess.WorkDir != "" && node.Path != "" {
					// Resolve symlinks on both paths for comparison
					realWorkDir := sess.WorkDir
					if resolved, err := filepath.EvalSymlinks(sess.WorkDir); err == nil {
						realWorkDir = resolved
					}
					realNodePath := node.Path
					if resolved, err := filepath.EvalSymlinks(node.Path); err == nil {
						realNodePath = resolved
					}

					// Exact match takes priority
					if realWorkDir == realNodePath {
						bestMatch = node
						break // Exact match found, stop searching
					}

					// Also match if session is from a subdirectory of this project
					// Use the most specific (longest path) parent project
					if strings.HasPrefix(realWorkDir, realNodePath+"/") {
						if bestMatch == nil || len(realNodePath) > len(bestMatch.Path) {
							bestMatch = node
							// Don't break - continue searching for a more specific match
						}
					}
				}
			}

			// Use best path match if no exact ProjectID match
			if !matched && bestMatch != nil {
				bestMatch.Sessions = append(bestMatch.Sessions, sess)
				matched = true
			}

			// Fallback: try name-based matching
			if !matched {
				for _, node := range projectMap {
					// Match if project path ends with session's project name
					if sess.ProjectName != "" && strings.HasSuffix(node.Path, "/"+sess.ProjectName) {
						node.Sessions = append(node.Sessions, sess)
						matched = true
						break
					}
					// Match if project name equals session's project name
					if node.Name == sess.ProjectName {
						node.Sessions = append(node.Sessions, sess)
						matched = true
						break
					}
				}
			}

			// Sessions not matching any registered project are simply ignored
			// We only show sessions for known projects
		}
	}

	// Sort projects alphabetically by name
	sort.Slice(projectOrder, func(i, j int) bool {
		return strings.ToLower(projectMap[projectOrder[i]].Name) < strings.ToLower(projectMap[projectOrder[j]].Name)
	})

	// Sort sessions within each project alphabetically by name
	for _, node := range projectMap {
		sort.Slice(node.Sessions, func(i, j int) bool {
			// Sort by LastActiveAt descending (most recent first)
			return node.Sessions[i].LastActiveAt.After(node.Sessions[j].LastActiveAt)
		})
	}

	// Build flattened tree for navigation (legacy)
	m.claudeTreeItems = nil
	for _, projID := range projectOrder {
		node := projectMap[projID]

		// Add project to tree
		m.claudeTreeItems = append(m.claudeTreeItems, claudeTreeItem{
			IsProject: true,
			ProjectID: node.ID,
		})

		// Add sessions under project
		for _, sess := range node.Sessions {
			m.claudeTreeItems = append(m.claudeTreeItems, claudeTreeItem{
				IsProject: false,
				ProjectID: sess.ProjectID,
				SessionID: sess.ID,
			})
		}
	}
	m.claudeTreeItemCount = len(m.claudeTreeItems)

	// Build TreeMenu items
	// List all tmux sessions once for efficient lookup
	tmuxSessions := ListTmuxSessions()

	var treeItems []TreeMenuItem
	const maxSessionsPerProject = 10
	for _, projID := range projectOrder {
		node := projectMap[projID]

		// Limit sessions unless showAllClaudeSessions is enabled
		sessionsToShow := node.Sessions
		hiddenCount := 0
		if !m.showAllClaudeSessions && len(node.Sessions) > maxSessionsPerProject {
			sessionsToShow = node.Sessions[:maxSessionsPerProject]
			hiddenCount = len(node.Sessions) - maxSessionsPerProject
		}

		// Build session children for this project
		var sessionItems []TreeMenuItem
		for _, sess := range sessionsToShow {
			// Get display name (remove project prefix if present)
			displayName := sess.Name
			if idx := strings.Index(displayName, "-"); idx > 0 && strings.HasPrefix(displayName, sess.ProjectID) {
				displayName = displayName[idx+1:]
			}

			// Check if terminal is attached (in memory or persistent tmux)
			hasTmux := false
			if m.terminalManager != nil {
				if t := m.terminalManager.Get(sess.ID); t != nil && t.State() == TerminalRunning {
					hasTmux = true
				}
			}
			// Also check for persistent tmux sessions (using cached list)
			if !hasTmux {
				shortID := sess.ID
				if len(shortID) > 8 {
					shortID = shortID[:8]
				}
				hasTmux = tmuxSessions[shortID]
			}

			// Check if session is being deleted
			isDeleting := m.deletingSessions[sess.ID]

			// Use filled circle for tmux sessions, empty otherwise
			icon := "○"
			if isDeleting {
				icon = "●" // Red filled circle for deleting
			} else if hasTmux {
				icon = "●"
			}

			// Check if this is the active session
			isActive := sess.ID == m.claudeActiveSession

			item := TreeMenuItem{
				ID:       sess.ID,
				Label:    displayName,
				Icon:     icon,
				IsActive: isActive,
				Data:     sess, // Store the full session data
			}
			// Set icon color based on state
			if isDeleting {
				item.IconColor = ColorError
				item.Blink = true    // Enable blinking for deleting sessions
				item.Disabled = true // Can't select deleting sessions
			} else if hasTmux {
				item.IconColor = ColorSuccess
			}
			sessionItems = append(sessionItems, item)
		}

		// Add "show more" indicator if sessions are hidden
		if hiddenCount > 0 {
			sessionItems = append(sessionItems, TreeMenuItem{
				ID:       "more:" + node.ID,
				Label:    fmt.Sprintf("+%d more (press 'a' to show all)", hiddenCount),
				Icon:     "…",
				Disabled: true,
			})
		}

		// Add project with its sessions as children
		projIcon := "📁"
		if len(node.Sessions) > 0 {
			projIcon = "📂"
		}

		treeItems = append(treeItems, TreeMenuItem{
			ID:       node.ID,
			Label:    node.Name,
			Icon:     projIcon,
			Children: sessionItems,
			Count:    len(node.Sessions), // Total count, not just visible
			Data:     "project",          // Mark as project for identification
		})
	}

	// Update the TreeMenu
	if m.sessionsTreeMenu != nil {
		m.sessionsTreeMenu.SetItems(treeItems)
	}
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
	case core.VMClaude:
		// Claude view - count depends on current tab
		switch m.claudeMode {
		case ClaudeModeSession:
			if m.state.Claude != nil {
				// Count filtered sessions
				count := 0
				for _, sess := range m.state.Claude.Sessions {
					if m.claudeFilterProject == "" || sess.ProjectID == m.claudeFilterProject {
						count++
					}
				}
				m.maxMainItems = count
			}
		case ClaudeModeChat:
			m.maxMainItems = 0 // No list navigation in chat
		}
	case core.VMCockpit:
		// Widgets view - count widgets in active profile
		profile := m.getCockpitProfile(m.getActiveCockpitProfile())
		if profile != nil {
			m.maxMainItems = len(profile.Widgets)
		} else {
			m.maxMainItems = 0
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
		m.lastError = "No project selected"
		m.lastErrorTime = time.Now()
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
		m.lastError = "No project selected"
		m.lastErrorTime = time.Now()
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
		m.lastError = "No project selected"
		m.lastErrorTime = time.Now()
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
		// Projects view uses TreeMenu
		if selectedItem := m.projectsMenu.SelectedItem(); selectedItem != nil {
			// Check if it's a component (get project from drill-down path)
			if _, ok := selectedItem.Data.(core.ComponentVM); ok {
				drillPath := m.projectsMenu.DrillDownPath()
				if len(drillPath) > 0 {
					return drillPath[0] // Project ID is the first in drill path
				}
			}
			// Check if it's a project
			if proj, ok := selectedItem.Data.(core.ProjectVM); ok {
				return proj.ID
			}
		}
	case core.VMDashboard:
		projects := core.SelectProjects(m.state)
		if m.mainIndex >= 0 && m.mainIndex < len(projects) {
			return projects[m.mainIndex].ID
		}
	case core.VMProcesses:
		// Processes view uses TreeMenu
		if selectedItem := m.processesMenu.SelectedItem(); selectedItem != nil {
			if proc, ok := selectedItem.Data.(core.ProcessVM); ok {
				return proc.ProjectID
			}
		}
	case core.VMGit:
		// Git view uses TreeMenu
		if selectedItem := m.gitMenu.SelectedItem(); selectedItem != nil {
			if proj, ok := selectedItem.Data.(core.GitStatusVM); ok {
				return proj.ProjectID
			}
			// If it's a file, get project from drill-down path
			drillPath := m.gitMenu.DrillDownPath()
			if len(drillPath) > 0 {
				for _, p := range m.state.Git.Projects {
					if p.ProjectName == drillPath[0] {
						return p.ProjectID
					}
				}
			}
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
		// Projects view uses TreeMenu
		if selectedItem := m.projectsMenu.SelectedItem(); selectedItem != nil {
			if comp, ok := selectedItem.Data.(core.ComponentVM); ok {
				return comp.Type
			}
		}
	case core.VMProcesses:
		// Processes view uses TreeMenu
		if selectedItem := m.processesMenu.SelectedItem(); selectedItem != nil {
			if proc, ok := selectedItem.Data.(core.ProcessVM); ok {
				return proc.Component
			}
		}
	}
	return ""
}

// isSelectedProjectSelf returns true if the selected project is csd-devtrack itself
func (m *Model) isSelectedProjectSelf() bool {
	switch m.currentView {
	case core.VMProjects:
		// Projects view uses TreeMenu
		if selectedItem := m.projectsMenu.SelectedItem(); selectedItem != nil {
			if proj, ok := selectedItem.Data.(core.ProjectVM); ok {
				return proj.IsSelf
			}
			// If it's a component, check the parent project from drill-down path
			if _, ok := selectedItem.Data.(core.ComponentVM); ok {
				drillPath := m.projectsMenu.DrillDownPath()
				if len(drillPath) > 0 {
					for _, p := range m.state.Projects.Projects {
						if p.ID == drillPath[0] {
							return p.IsSelf
						}
					}
				}
			}
		}
	case core.VMDashboard:
		projects := core.SelectProjects(m.state)
		if m.mainIndex >= 0 && m.mainIndex < len(projects) {
			return projects[m.mainIndex].IsSelf
		}
	case core.VMProcesses:
		// Processes view uses TreeMenu
		if selectedItem := m.processesMenu.SelectedItem(); selectedItem != nil {
			if proc, ok := selectedItem.Data.(core.ProcessVM); ok {
				return proc.IsSelf
			}
		}
	}
	return false
}

// loadGitDiffForSelection loads the diff if the selection changed to a file
// Returns a command to load the diff, or nil if no change needed
func (m *Model) loadGitDiffForSelection() tea.Cmd {
	if m.gitMenu == nil || m.state.Git == nil {
		return nil
	}

	selectedItem := m.gitMenu.SelectedItem()
	if selectedItem == nil {
		// Clear diff if nothing selected
		if m.gitLastSelectedFile != "" {
			m.gitLastSelectedFile = ""
			m.gitDiffContent = nil
		}
		return nil
	}

	// Check if it's a file
	fileEntry, ok := selectedItem.Data.(GitFileEntry)
	if !ok {
		// Selected a project, not a file - clear diff
		if m.gitLastSelectedFile != "" {
			m.gitLastSelectedFile = ""
			m.gitDiffContent = nil
		}
		return nil
	}

	// Same file? No need to reload
	if selectedItem.ID == m.gitLastSelectedFile {
		return nil
	}

	// New file selected - load diff
	m.gitLastSelectedFile = selectedItem.ID
	m.gitDiffLoading = true
	m.detailScrollOffset = 0

	// Get project from drill-down path
	drillPath := m.gitMenu.DrillDownPath()
	if len(drillPath) == 0 {
		return nil
	}
	projectName := drillPath[0]

	var projectPath string
	cfg := config.GetGlobal()
	for _, p := range m.state.Git.Projects {
		if p.ProjectName == projectName {
			for _, proj := range cfg.Projects {
				if proj.ID == p.ProjectID {
					projectPath = proj.Path
					break
				}
			}
			break
		}
	}
	if projectPath == "" {
		return nil
	}

	f := fileEntry
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

// getRefreshRateMs returns the refresh rate in milliseconds from config (default 5000ms)
func getRefreshRateMs() int {
	if cfg := config.GetGlobal(); cfg != nil && cfg.Settings != nil && cfg.Settings.RefreshRate > 0 {
		return cfg.Settings.RefreshRate
	}
	return 5000
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Duration(getRefreshRateMs())*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// claudeRefreshMsg triggers UI refresh during Claude streaming
type claudeRefreshMsg struct{}

// claudeRefreshCmd schedules a fast refresh for responsive streaming (50ms)
func claudeRefreshCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*50, func(t time.Time) tea.Msg {
		return claudeRefreshMsg{}
	})
}

// terminalRefreshMsg triggers UI refresh for terminal output
type terminalRefreshMsg struct{}

// scheduleTerminalRefresh schedules a terminal output refresh (30ms for smooth display)
func (m *Model) scheduleTerminalRefresh() tea.Cmd {
	// Continue refreshing as long as there's an active running terminal
	if m.claudeActiveSession == "" || m.terminalManager == nil {
		return nil
	}
	if t := m.terminalManager.Get(m.claudeActiveSession); t == nil || !t.IsRunning() {
		return nil
	}
	return tea.Tick(time.Millisecond*30, func(t time.Time) tea.Msg {
		return terminalRefreshMsg{}
	})
}

// tuiStateRestoreMsg is sent when TUI state should be restored (reattach)
type tuiStateRestoreMsg struct {
	state *daemon.TUIState
}

// ============================================================================
// File Browser functions for Config view
// ============================================================================

// loadBrowserEntries loads directory entries for the file browser
func (m *Model) loadBrowserEntries() {
	m.browserEntries = make([]BrowserEntry, 0)

	// Always add parent directory entry first (so user can navigate back)
	if m.browserPath != "/" {
		m.browserEntries = append(m.browserEntries, BrowserEntry{
			Name:  "..",
			IsDir: true,
			Path:  filepath.Dir(m.browserPath),
		})
	}

	entries, err := os.ReadDir(m.browserPath)
	if err != nil {
		// Can't read directory, but we still have ".." to go back
		return
	}

	detector := projects.NewDetector()
	cfg := config.GetGlobal()

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

		// Check if we can read this directory (have permissions)
		// before attempting project detection
		if _, err := os.ReadDir(fullPath); err == nil {
			// Check if this is a detectable project
			if proj, err := detector.DetectProject(fullPath); err == nil && len(proj.Components) > 0 {
				browserEntry.IsProject = true
			}
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

// ============================================================================
// TUI State Export/Import for Daemon Mode
// ============================================================================

// ExportTUIState exports the current TUI state for daemon persistence
func (m *Model) ExportTUIState() *daemon.TUIState {
	return &daemon.TUIState{
		// Current view
		CurrentView: m.currentView,

		// Focus state
		FocusArea:    int(m.focusArea),
		SidebarIndex: m.sidebarIndex,

		// Selection state
		MainIndex:   m.mainIndex,
		DetailIndex: m.detailIndex,

		// Scroll offsets
		MainScrollOffset:   m.mainScrollOffset,
		DetailScrollOffset: m.detailScrollOffset,

		// Config view state
		ConfigMode:  m.configMode,
		BrowserPath: m.browserPath,

		// Log view state
		LogLevelFilter:  m.logLevelFilter,
		LogSourceFilter: m.logSourceFilter,
		LogTypeFilter:   m.logTypeFilter,
		LogSearchText:   m.logSearchText,
		LogScrollOffset: m.logScrollOffset,
		LogAutoScroll:   m.logAutoScroll,

		// Build profile
		BuildProfile: m.currentBuildProfile,
	}
}

// ImportTUIState restores TUI state from daemon persistence
func (m *Model) ImportTUIState(state *daemon.TUIState) {
	if state == nil {
		return
	}

	// Reset any modal states that could block input
	m.showDialog = false
	m.showHelp = false
	m.filterActive = false
	m.logSearchActive = false

	// Restore current view
	m.currentView = state.CurrentView
	m.sidebarIndex = state.SidebarIndex

	// Restore focus state
	m.focusArea = FocusArea(state.FocusArea)

	// Restore selection state
	m.mainIndex = state.MainIndex
	m.detailIndex = state.DetailIndex

	// Restore scroll offsets
	m.mainScrollOffset = state.MainScrollOffset
	m.detailScrollOffset = state.DetailScrollOffset

	// Restore Config view state
	m.configMode = state.ConfigMode
	if state.BrowserPath != "" {
		m.browserPath = state.BrowserPath
	}

	// Restore Log view state
	m.logLevelFilter = state.LogLevelFilter
	m.logSourceFilter = state.LogSourceFilter
	m.logTypeFilter = state.LogTypeFilter
	m.logSearchText = state.LogSearchText
	m.logScrollOffset = state.LogScrollOffset
	m.logAutoScroll = state.LogAutoScroll

	// Restore Build profile
	if state.BuildProfile != "" {
		m.currentBuildProfile = state.BuildProfile
	}

	// Reload browser entries if in config browser mode
	if m.currentView == core.VMConfig && m.configMode == "browser" {
		m.loadBrowserEntries()
	}

	// Update state to match restored view
	m.state.SetCurrentView(m.currentView)

	// Recalculate item counts for the restored view
	m.updateItemCounts()

	// Call restore callback if set
	if m.onStateRestore != nil {
		m.onStateRestore()
	}
}

// SetStateRestoreCallback sets a callback to be called after state is restored
func (m *Model) SetStateRestoreCallback(callback func()) {
	m.onStateRestore = callback
}
