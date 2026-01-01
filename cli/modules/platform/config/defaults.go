package config

import (
	"os"
	"path/filepath"
)

// GetDefaultConfigPath returns the default config file path
func GetDefaultConfigPath() string {
	// Try current directory first
	cwd, err := os.Getwd()
	if err == nil {
		return filepath.Join(cwd, DefaultConfigFileName)
	}

	return DefaultConfigFileName
}

// GetUserConfigDir returns the user's config directory for csd-devtrack
func GetUserConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, ".config", "csd-devtrack"), nil
}

// GetCacheDir returns the cache directory for csd-devtrack
func GetCacheDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, ".cache", "csd-devtrack"), nil
}

// GetDataDir returns the data directory for csd-devtrack
func GetDataDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, ".local", "share", "csd-devtrack"), nil
}

// EnsureDirectories creates all necessary directories
func EnsureDirectories() error {
	dirs := []func() (string, error){
		GetUserConfigDir,
		GetCacheDir,
		GetDataDir,
	}

	for _, dirFunc := range dirs {
		dir, err := dirFunc()
		if err != nil {
			continue // Skip if we can't get the directory path
		}

		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	return nil
}

// BuildEnvironment represents build environment settings
type BuildEnvironment struct {
	GOOS        string
	GOARCH      string
	CGO_ENABLED string
	GoPath      string
	NodePath    string
	NPMPath     string
}

// DetectBuildEnvironment detects the build environment
func DetectBuildEnvironment() *BuildEnvironment {
	env := &BuildEnvironment{
		GOOS:        os.Getenv("GOOS"),
		GOARCH:      os.Getenv("GOARCH"),
		CGO_ENABLED: os.Getenv("CGO_ENABLED"),
	}

	// Try to find Go
	if goPath, err := findExecutable("go"); err == nil {
		env.GoPath = goPath
	}

	// Try to find Node
	if nodePath, err := findExecutable("node"); err == nil {
		env.NodePath = nodePath
	}

	// Try to find NPM
	if npmPath, err := findExecutable("npm"); err == nil {
		env.NPMPath = npmPath
	}

	return env
}

// findExecutable finds an executable in PATH
func findExecutable(name string) (string, error) {
	// Check common locations
	paths := []string{
		"/usr/local/bin/" + name,
		"/usr/bin/" + name,
		"/opt/homebrew/bin/" + name,
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// Fall back to PATH lookup
	pathEnv := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(pathEnv) {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", os.ErrNotExist
}

// DefaultProjectPatterns returns default patterns for project detection
var DefaultProjectPatterns = struct {
	GoMain      []string
	Frontend    []string
	IgnoreDirs  []string
	IgnoreFiles []string
}{
	GoMain: []string{
		"main.go",
		"csd-*.go",
		"*ctl.go",
		"*d.go",
	},
	Frontend: []string{
		"package.json",
		"vite.config.ts",
		"vite.config.js",
		"next.config.js",
		"webpack.config.js",
	},
	IgnoreDirs: []string{
		"node_modules",
		"vendor",
		".git",
		".idea",
		".vscode",
		"build",
		"dist",
		"targets",
	},
	IgnoreFiles: []string{
		".gitignore",
		".dockerignore",
		"*.test.go",
		"*_test.go",
	},
}
