// +build windows

package supervisor

import (
	"os/exec"
	"syscall"
)

// setupProcessGroup sets up process handling for Windows
func (m *Manager) setupProcessGroup(cmd *exec.Cmd) {
	// On Windows, we use CREATE_NEW_PROCESS_GROUP
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// signalProcess sends a signal to a process on Windows
// Note: Windows doesn't support Unix signals, so we use different methods
func (m *Manager) signalProcess(cmd *exec.Cmd, sig syscall.Signal) error {
	if cmd.Process == nil {
		return nil
	}

	// On Windows, we can only really kill the process
	// SIGTERM is simulated as a kill
	return cmd.Process.Kill()
}

// killProcess forcefully kills a process on Windows
func (m *Manager) killProcess(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
