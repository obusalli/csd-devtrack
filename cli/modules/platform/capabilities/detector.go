package capabilities

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// detect checks if a capability is available (auto-detection only)
func detect(cap Capability) *CapabilityInfo {
	return detectWithConfig(cap, "")
}

// detectWithConfig checks if a capability is available, using configured path if provided
func detectWithConfig(cap Capability, configuredPath string) *CapabilityInfo {
	config, ok := capabilityConfigs[cap]
	if !ok {
		return &CapabilityInfo{
			Name:      cap,
			Available: false,
			CheckedAt: time.Now(),
		}
	}

	info := &CapabilityInfo{
		Name:      cap,
		Available: false,
		CheckedAt: time.Now(),
	}

	// If a path is configured, try that first
	if configuredPath != "" {
		path := resolveConfiguredPath(configuredPath)
		if path != "" && isExecutable(path) {
			info.Path = path
			// Verify if needed
			if config.verify && config.versionArg != "" {
				version := getVersion(path, config.versionArg)
				if version != "" {
					info.Available = true
					info.Version = version
					return info
				}
			} else {
				info.Available = true
				return info
			}
		}
	}

	// Auto-detect: try to find the binary
	for _, binary := range config.binaries {
		path := findBinary(binary)
		if path == "" {
			continue
		}

		info.Path = path

		// Verify by running the binary if configured
		if config.verify && config.versionArg != "" {
			version := getVersion(path, config.versionArg)
			if version != "" {
				info.Available = true
				info.Version = version
				return info
			}
		} else {
			// Just check if the file exists and is executable
			info.Available = true
			return info
		}
	}

	return info
}

// resolveConfiguredPath resolves a configured path (can be just a name like "zsh" or full path "/usr/bin/zsh")
func resolveConfiguredPath(configured string) string {
	// If it's an absolute path, return as-is
	if strings.HasPrefix(configured, "/") || strings.HasPrefix(configured, "C:") || strings.HasPrefix(configured, "c:") {
		return configured
	}
	// Otherwise, try to find it in PATH
	return findBinary(configured)
}

// findBinary searches for a binary in common locations and PATH
func findBinary(name string) string {
	// First try exec.LookPath (searches PATH)
	if path, err := exec.LookPath(name); err == nil {
		return path
	}

	// On Windows, also try with .exe extension
	if runtime.GOOS == "windows" && !strings.HasSuffix(name, ".exe") {
		if path, err := exec.LookPath(name + ".exe"); err == nil {
			return path
		}
	}

	home, _ := os.UserHomeDir()
	var locations []string

	if runtime.GOOS == "windows" {
		// Windows-specific locations
		programFiles := os.Getenv("ProgramFiles")
		programFilesX86 := os.Getenv("ProgramFiles(x86)")
		localAppData := os.Getenv("LOCALAPPDATA")
		appData := os.Getenv("APPDATA")

		locations = []string{
			// User-specific locations
			filepath.Join(localAppData, "Programs"),
			filepath.Join(appData, "npm"),
			filepath.Join(home, "go", "bin"),
			filepath.Join(home, ".cargo", "bin"),
			filepath.Join(home, "scoop", "shims"),
			// System locations
			filepath.Join(programFiles, "Git", "bin"),
			filepath.Join(programFiles, "Git", "cmd"),
			filepath.Join(programFiles, "nodejs"),
			filepath.Join(programFiles, "Go", "bin"),
			filepath.Join(programFilesX86, "Git", "bin"),
			filepath.Join(programFilesX86, "Git", "cmd"),
			"C:\\Windows\\System32",
			"C:\\Windows\\System32\\WindowsPowerShell\\v1.0",
		}

		// Add NVM for Windows locations
		nvmDir := os.Getenv("NVM_HOME")
		if nvmDir == "" {
			nvmDir = filepath.Join(appData, "nvm")
		}
		if entries, err := os.ReadDir(nvmDir); err == nil {
			for _, entry := range entries {
				if entry.IsDir() && strings.HasPrefix(entry.Name(), "v") {
					locations = append(locations, filepath.Join(nvmDir, entry.Name()))
				}
			}
		}
	} else {
		// Unix-specific locations
		locations = []string{
			// User-specific locations
			filepath.Join(home, ".local", "bin"),
			filepath.Join(home, ".npm-global", "bin"),
			filepath.Join(home, "go", "bin"),
			filepath.Join(home, ".cargo", "bin"),
			// System locations
			"/usr/local/bin",
			"/usr/bin",
			"/bin",
			"/opt/homebrew/bin",
			"/snap/bin",
		}

		// Add NVM node bin directories (search all installed versions)
		nvmDir := filepath.Join(home, ".nvm", "versions", "node")
		if entries, err := os.ReadDir(nvmDir); err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					locations = append(locations, filepath.Join(nvmDir, entry.Name(), "bin"))
				}
			}
		}
	}

	for _, dir := range locations {
		path := filepath.Join(dir, name)
		if isExecutable(path) {
			return path
		}
		// On Windows, try with .exe extension
		if runtime.GOOS == "windows" && !strings.HasSuffix(name, ".exe") {
			pathExe := filepath.Join(dir, name+".exe")
			if isExecutable(pathExe) {
				return pathExe
			}
		}
	}

	return ""
}

// isExecutable checks if a file exists and is executable
func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	mode := info.Mode()
	if !mode.IsRegular() {
		return false
	}

	// On Windows, check for executable extensions
	if runtime.GOOS == "windows" {
		ext := strings.ToLower(filepath.Ext(path))
		return ext == ".exe" || ext == ".cmd" || ext == ".bat" || ext == ".com"
	}

	// On Unix, check execute permission
	return mode&0111 != 0
}

// getVersion runs the binary with version argument and returns the output
func getVersion(path, versionArg string) string {
	args := strings.Fields(versionArg)
	cmd := exec.Command(path, args...)
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	// Extract first line as version
	version := strings.TrimSpace(string(output))
	if idx := strings.Index(version, "\n"); idx > 0 {
		version = version[:idx]
	}

	return version
}
