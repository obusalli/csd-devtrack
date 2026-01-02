package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"
)

const (
	// EnvDaemonMode is set when running as daemon
	EnvDaemonMode = "CSD_DEVTRACK_DAEMON"
	// EnvDaemonConfig passes the config path to the daemon
	EnvDaemonConfig = "CSD_DEVTRACK_CONFIG"
	// EnvDaemonName passes the instance name to the daemon
	EnvDaemonName = "CSD_DEVTRACK_INSTANCE"
)

// IsDaemonMode returns true if running as daemon process
func IsDaemonMode() bool {
	return os.Getenv(EnvDaemonMode) == "1"
}

// StartDaemon starts the daemon process in background and returns
// Returns the PID of the daemon process
func StartDaemon() (int, error) {
	// Get the executable path
	executable, err := os.Executable()
	if err != nil {
		return 0, fmt.Errorf("failed to get executable path: %w", err)
	}

	// Build arguments - only pass global flags, not commands
	args := []string{executable}
	skipNext := false
	for i, arg := range os.Args[1:] {
		if skipNext {
			skipNext = false
			continue
		}

		// Skip commands (anything that doesn't start with -)
		if !strings.HasPrefix(arg, "-") {
			continue
		}

		// Skip flags that shouldn't be passed to daemon
		if arg == "--no-daemon" {
			continue
		}

		// Flags that take a value
		if arg == "--config" || arg == "-c" || arg == "--name" || arg == "-n" {
			if i+1 < len(os.Args[1:]) {
				args = append(args, arg, os.Args[i+2])
				skipNext = true
			}
			continue
		}

		// Flags with = syntax
		if strings.HasPrefix(arg, "--config=") || strings.HasPrefix(arg, "--name=") {
			args = append(args, arg)
			continue
		}

		// Other flags (--verbose, etc)
		args = append(args, arg)
	}

	// Create the daemon process
	cmd := exec.Command(args[0], args[1:]...)

	// Set environment to indicate daemon mode
	cmd.Env = append(os.Environ(), EnvDaemonMode+"=1")

	// Detach from terminal
	if runtime.GOOS != "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setsid: true, // Create new session
		}
	}

	// Redirect stdio to null
	cmd.Stdin = nil
	cmd.Stdout = nil
	cmd.Stderr = nil

	// Start the daemon
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start daemon: %w", err)
	}

	pid := cmd.Process.Pid

	// Detach - we don't wait for the daemon
	cmd.Process.Release()

	// Wait for daemon to initialize with retries
	// Daemon needs time to: load config, init presenter (git ops), create socket
	for i := 0; i < 30; i++ { // 3 second max
		time.Sleep(100 * time.Millisecond)
		if IsRunning() {
			return pid, nil
		}
	}

	return 0, fmt.Errorf("daemon failed to start (check logs)")
}

// StopDaemon stops the running daemon
func StopDaemon() error {
	pid := GetServerPID()
	if pid == 0 {
		return fmt.Errorf("daemon not running")
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find daemon process: %w", err)
	}

	// Send SIGTERM for graceful shutdown
	if runtime.GOOS == "windows" {
		// Windows: just kill it
		if err := process.Kill(); err != nil {
			return fmt.Errorf("failed to stop daemon: %w", err)
		}
	} else {
		// Unix: SIGTERM
		if err := process.Signal(syscall.SIGTERM); err != nil {
			return fmt.Errorf("failed to stop daemon: %w", err)
		}
	}

	// Wait for daemon to stop (with timeout)
	for i := 0; i < 50; i++ { // 5 second timeout
		time.Sleep(100 * time.Millisecond)
		if !IsRunning() {
			return nil
		}
	}

	// Force kill if still running
	process.Kill()

	// Final cleanup
	time.Sleep(100 * time.Millisecond)
	if IsRunning() {
		return fmt.Errorf("daemon did not stop")
	}

	return nil
}

// Status returns a human-readable status of the daemon
func Status() string {
	if !IsRunning() {
		return "Daemon is not running"
	}

	pid := GetServerPID()
	socketPath := GetSocketPath()

	return fmt.Sprintf("Daemon running (PID: %d, Socket: %s)", pid, socketPath)
}

// EnsureDaemon ensures the daemon is running, starting it if necessary
// Returns true if daemon was started, false if already running
func EnsureDaemon() (bool, error) {
	if IsRunning() {
		return false, nil
	}

	_, err := StartDaemon()
	if err != nil {
		return false, err
	}

	return true, nil
}

// ForceKillDaemon forcefully kills the daemon with SIGKILL
func ForceKillDaemon() error {
	pid := GetServerPID()
	if pid == 0 {
		return fmt.Errorf("daemon not running")
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find daemon process: %w", err)
	}

	// Send SIGKILL for immediate termination
	if err := process.Kill(); err != nil {
		return fmt.Errorf("failed to kill daemon: %w", err)
	}

	// Wait a moment for cleanup
	time.Sleep(100 * time.Millisecond)

	// Force cleanup files
	return Wipe()
}

// GetDaemonVersion returns the daemon's version and build hash
// Returns empty strings if daemon is not running or unreachable
func GetDaemonVersion() (version, hash string) {
	if !IsRunning() {
		return "", ""
	}

	client := NewClient()
	if err := client.Connect(); err != nil {
		return "", ""
	}
	defer client.Disconnect()

	// Wait a bit for handshake response
	time.Sleep(100 * time.Millisecond)

	return client.GetServerVersion()
}
