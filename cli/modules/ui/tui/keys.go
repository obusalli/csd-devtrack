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
	Cancel key.Binding // Ctrl+C to cancel current build/process
}

// DefaultKeyMap returns the default key bindings
func DefaultKeyMap() KeyMap {
	return KeyMap{
		// Navigation
		Up: key.NewBinding(
			key.WithKeys("up"),
			key.WithHelp("↑", "up"),
		),
		Down: key.NewBinding(
			key.WithKeys("down", "j"),
			key.WithHelp("↓/j", "down"),
		),
		Left: key.NewBinding(
			key.WithKeys("left"),
			key.WithHelp("←", "left"),
		),
		Right: key.NewBinding(
			key.WithKeys("right"),
			key.WithHelp("→", "right"),
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
			key.WithKeys("home"),
			key.WithHelp("Home", "go to start"),
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

		// Project actions
		Build: key.NewBinding(
			key.WithKeys("f5"),
			key.WithHelp("F5", "build"),
		),
		BuildAll: key.NewBinding(
			key.WithKeys("ctrl+b"),
			key.WithHelp("C-b", "build all"),
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
			key.WithKeys("ctrl+r"),
			key.WithHelp("C-r", "restart"),
		),
		Kill: key.NewBinding(
			key.WithKeys("k"),
			key.WithHelp("k", "kill"),
		),
		Logs: key.NewBinding(
			key.WithKeys("l"),
			key.WithHelp("l", "view logs"),
		),

		// Git (in Git view only)
		GitStatus: key.NewBinding(
			key.WithKeys("S"),
			key.WithHelp("S", "git status"),
		),
		GitDiff: key.NewBinding(
			key.WithKeys("D"),
			key.WithHelp("D", "git diff"),
		),
		GitLog: key.NewBinding(
			key.WithKeys("H"),
			key.WithHelp("H", "git history"),
		),

		// Other
		Help: key.NewBinding(
			key.WithKeys("?"),
			key.WithHelp("?", "help"),
		),
		Quit: key.NewBinding(
			key.WithKeys("q"),
			key.WithHelp("q", "quit"),
		),
		Filter: key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		),
		Cancel: key.NewBinding(
			key.WithKeys("ctrl+c"),
			key.WithHelp("C-c", "cancel build/process"),
		),
	}
}

// ShortHelp returns a brief help display
func (k KeyMap) ShortHelp() []key.Binding {
	return []key.Binding{
		k.Up, k.Down, k.Enter, k.Tab,
		k.Build, k.Run, k.Stop, k.Cancel,
		k.Help, k.Quit,
	}
}

// FullHelp returns detailed help for all keys
func (k KeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		// Navigation
		{k.Up, k.Down, k.Left, k.Right, k.PageUp, k.PageDown, k.Tab, k.ShiftTab},
		// Actions
		{k.Enter, k.Space, k.Escape, k.Refresh},
		// Project
		{k.Build, k.BuildAll, k.Run, k.Stop, k.Restart, k.Kill, k.Logs},
		// Git
		{k.GitStatus, k.GitDiff, k.GitLog},
		// Other
		{k.Filter, k.Cancel, k.Help, k.Quit},
	}
}
