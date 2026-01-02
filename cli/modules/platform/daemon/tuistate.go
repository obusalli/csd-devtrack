package daemon

import (
	"csd-devtrack/cli/modules/ui/core"
)

// TUIState captures the TUI-specific state for restore on reattach
type TUIState struct {
	// Current view
	CurrentView core.ViewModelType `json:"current_view"`

	// Focus state
	FocusArea    int `json:"focus_area"`    // 0=sidebar, 1=main, 2=detail
	SidebarIndex int `json:"sidebar_index"` // Selected sidebar item

	// Selection state per view
	MainIndex   int `json:"main_index"`   // Selected item in main panel
	DetailIndex int `json:"detail_index"` // Selected item in detail panel

	// Scroll offsets
	MainScrollOffset   int `json:"main_scroll_offset"`
	DetailScrollOffset int `json:"detail_scroll_offset"`

	// View-specific state
	ConfigMode string `json:"config_mode"` // "projects", "browser", "settings"
	BrowserPath string `json:"browser_path"`

	// Log view state
	LogLevelFilter  string `json:"log_level_filter"`
	LogSourceFilter string `json:"log_source_filter"`
	LogTypeFilter   string `json:"log_type_filter"`
	LogSearchText   string `json:"log_search_text"`
	LogScrollOffset int    `json:"log_scroll_offset"`
	LogAutoScroll   bool   `json:"log_auto_scroll"`

	// Git view state
	GitShowDiff bool `json:"git_show_diff"`

	// Build profile
	BuildProfile string `json:"build_profile"`
}

// TUIStatePayload wraps TUI state for transmission
type TUIStatePayload struct {
	TUIState *TUIState `json:"tui_state"`
}
