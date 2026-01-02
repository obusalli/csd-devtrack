package commands

import (
	uicore "csd-devtrack/cli/modules/ui/core"
)

var daemonMode bool

// SetDaemonMode sets whether the UI should run in daemon mode
func SetDaemonMode(enabled bool) {
	daemonMode = enabled
}

// IsDaemonMode returns whether daemon mode is enabled
func IsDaemonMode() bool {
	return daemonMode
}

// CreatePresenter creates a presenter for the daemon
// This initializes the full presenter with all services
func CreatePresenter(appCtx *AppContext) uicore.Presenter {
	return uicore.NewPresenter(appCtx.ProjectService, appCtx.Config)
}
