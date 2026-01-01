// +build !windows

package supervisor

import (
	"os/exec"
	"syscall"
)

// setupProcessGroup sets up process group handling for Unix systems
func (m *Manager) setupProcessGroup(cmd *exec.Cmd) {
	// Create a new process group so we can kill all child processes
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

// signalProcess sends a signal to a process on Unix
func (m *Manager) signalProcess(cmd *exec.Cmd, sig syscall.Signal) error {
	if cmd.Process == nil {
		return nil
	}

	// Send signal to the process group
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err == nil {
		// Kill the entire process group
		return syscall.Kill(-pgid, sig)
	}

	// Fallback: kill just the process
	return cmd.Process.Signal(sig)
}

// killProcess forcefully kills a process on Unix
func (m *Manager) killProcess(cmd *exec.Cmd) error {
	return m.signalProcess(cmd, syscall.SIGKILL)
}
