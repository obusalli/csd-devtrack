package tui

import (
	"github.com/charmbracelet/bubbles/key"
)

// KeyMap defines all keyboard shortcuts
type KeyMap struct {
	// Navigation
	Up       key.Binding
	Down     key.Binding
	Left     key.Binding
	Right    key.Binding
	PageUp   key.Binding
	PageDown key.Binding
	Home     key.Binding
	End      key.Binding
	Tab      key.Binding
	ShiftTab key.Binding

	// Actions
	Enter   key.Binding
	Space   key.Binding
	Escape  key.Binding
	Delete  key.Binding
	Refresh key.Binding

	// Views (number keys)
	View1 key.Binding // Dashboard
	View2 key.Binding // Projects
	View3 key.Binding // Build
	View4 key.Binding // Processes
	View5 key.Binding // Logs
	View6 key.Binding // Git
	View7 key.Binding // Config

	// Project actions
	Build    key.Binding
	BuildAll key.Binding
	Run      key.Binding
	Stop     key.Binding
	Restart  key.Binding
	Kill     key.Binding
	Logs     key.Binding

	// Git actions
	GitStatus key.Binding
	GitDiff   key.Binding
	GitLog    key.Binding

	// Other
	Help   key.Binding
	Quit   key.Binding
	Filter key.Binding
}

// DefaultKeyMap returns the default key bindings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		// Navigation
		Up: key.NewBinding(
			key.WithKeys("up", "k"),
			key.WithHelp("↑/k", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left", "h"),
			key.WithHelp("←/h", "left"),
		),
		Right: key.NewBinding(
			key.WithKeys("right", "l"),
			key.WithHelp("→/l", "right"),
		),
		PageUp: key.NewBinding(
			key.WithKeys("pgup", "ctrl+u"),
			key.WithHelp("PgUp", "page up"),
		),
		PageDown: key.NewBinding(
			key.WithKeys("pgdown", "ctrl+d"),
			key.WithHelp("PgDn", "page down"),
		),
		Home: key.NewBinding(
			key.WithKeys("home", "g"),
			key.WithHelp("Home/g", "go to start"),
		),
		End: key.NewBinding(
			key.WithKeys("end", "G"),
			key.WithHelp("End/G", "go to end"),
		),
		Tab: key.NewBinding(
			key.WithKeys("tab"),
			key.WithHelp("Tab", "next panel"),
		),
		ShiftTab: key.NewBinding(
			key.WithKeys("shift+tab"),
			key.WithHelp("S-Tab", "prev panel"),
		),

		// Actions
		Enter: key.NewBinding(
			key.WithKeys("enter"),
			key.WithHelp("Enter", "select"),
		),
		Space: key.NewBinding(
			key.WithKeys(" "),
			key.WithHelp("Space", "toggle"),
		),
		Escape: key.NewBinding(
			key.WithKeys("esc"),
			key.WithHelp("Esc", "back/cancel"),
		),
		Delete: key.NewBinding(
			key.WithKeys("delete", "backspace"),
			key.WithHelp("Del", "delete"),
		),
		Refresh: key.NewBinding(
			key.WithKeys("ctrl+r"),
			key.WithHelp("C-r", "refresh"),
		),

		// Views
		View1: key.NewBinding(
			key.WithKeys("1"),
			key.WithHelp("1", "dashboard"),
		),
		View2: key.NewBinding(
			key.WithKeys("2"),
			key.WithHelp("2", "projects"),
		),
		View3: key.NewBinding(
			key.WithKeys("3"),
			key.WithHelp("3", "build"),
		),
		View4: key.NewBinding(
			key.WithKeys("4"),
			key.WithHelp("4", "processes"),
		),
		View5: key.NewBinding(
			key.WithKeys("5"),
			key.WithHelp("5", "logs"),
		),
		View6: key.NewBinding(
			key.WithKeys("6"),
			key.WithHelp("6", "git"),
		),
		View7: key.NewBinding(
			key.WithKeys("7"),
			key.WithHelp("7", "config"),
		),

		// Project actions
		Build: key.NewBinding(
			key.WithKeys("b"),
			key.WithHelp("b", "build"),
		),
		BuildAll: key.NewBinding(
			key.WithKeys("B"),
			key.WithHelp("B", "build all"),
		),
		Run: key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "run"),
		),
		Stop: key.NewBinding(
			key.WithKeys("s"),
			key.WithHelp("s", "stop"),
		),
		Restart: key.NewBinding(
			key.WithKeys("R"),
			key.WithHelp("R", "restart"),
		),
		Kill: key.NewBinding(
			key.WithKeys("K"),
			key.WithHelp("K", "kill"),
		),
		Logs: key.NewBinding(
			key.WithKeys("L"),
			key.WithHelp("L", "view logs"),
		),

		// Git
		GitStatus: key.NewBinding(
			key.WithKeys("g"),
			key.WithHelp("g", "git status"),
		),
		GitDiff: key.NewBinding(
			key.WithKeys("d"),
			key.WithHelp("d", "git diff"),
		),
		GitLog: key.NewBinding(
			key.WithKeys("c"),
			key.WithHelp("c", "git log"),
		),

		// Other
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q", "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
	}
}

// ShortHelp returns a brief help display
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Up, k.Down, k.Enter, k.Tab,
		k.Build, k.Run, k.Stop,
		k.Help, k.Quit,
	}
}

// FullHelp returns detailed help for all keys
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		// Navigation
		{k.Up, k.Down, k.Left, k.Right, k.PageUp, k.PageDown, k.Tab, k.ShiftTab},
		// Views
		{k.View1, k.View2, k.View3, k.View4, k.View5, k.View6, k.View7},
		// Actions
		{k.Enter, k.Space, k.Escape, k.Refresh},
		// Project
		{k.Build, k.BuildAll, k.Run, k.Stop, k.Restart, k.Kill, k.Logs},
		// Git
		{k.GitStatus, k.GitDiff, k.GitLog},
		// Other
		{k.Filter, k.Help, k.Quit},
	}
}
