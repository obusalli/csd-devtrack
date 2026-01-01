package builder

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"csd-devtrack/cli/modules/core/builds"
	"csd-devtrack/cli/modules/core/projects"
)

// GoBuilder builds Go components
type GoBuilder struct {
	goPath   string
	ldflags  string
	verbose  bool
}

// NewGoBuilder creates a new Go builder
func NewGoBuilder() *GoBuilder {
	goPath := findGo()
	return &GoBuilder{
		goPath:  goPath,
		ldflags: "-s -w", // Strip symbols by default
		verbose: false,
	}
}

// findGo finds the Go executable
func findGo() string {
	// Try PATH first
	if path, err := exec.LookPath("go"); err == nil {
		return path
	}

	// Common locations
	paths := []string{
		"/usr/local/go/bin/go",
		"/usr/local/bin/go",
		"/opt/homebrew/bin/go",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return "go" // Fallback
}

// Build builds a Go component
func (b *GoBuilder) Build(ctx context.Context, project *projects.Project, component *projects.Component, build *builds.Build) error {
	// Determine working directory
	workDir := filepath.Join(project.Path, component.Path)
	if component.Path == "" {
		workDir = project.Path
	}

	// Check if workDir exists
	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		return fmt.Errorf("component directory not found: %s", workDir)
	}

	// Determine output binary path
	outputDir := filepath.Join(project.Path, "targets")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	binaryName := component.Binary
	if binaryName == "" {
		binaryName = strings.TrimSuffix(component.EntryPoint, ".go")
	}

	// Add .exe on Windows
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}

	outputPath := filepath.Join(outputDir, binaryName)

	// Build command
	args := []string{"build"}

	if b.ldflags != "" {
		args = append(args, fmt.Sprintf("-ldflags=%s", b.ldflags))
	}

	args = append(args, "-o", outputPath)

	if component.EntryPoint != "" {
		args = append(args, "./"+component.EntryPoint)
	} else {
		args = append(args, ".")
	}

	// Use custom build command if specified
	if component.BuildCmd != "" {
		return b.runCustomBuildCommand(ctx, workDir, component.BuildCmd, build)
	}

	build.AddOutput(fmt.Sprintf("Building %s...", component.Binary))
	build.AddOutput(fmt.Sprintf("Command: go %s", strings.Join(args, " ")))
	build.AddOutput(fmt.Sprintf("Working directory: %s", workDir))

	// Create command
	cmd := exec.CommandContext(ctx, b.goPath, args...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=0", // Disable CGO for static builds
	)

	// Capture output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start command
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start build: %w", err)
	}

	// Read stdout
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			build.AddOutput(line)
		}
	}()

	// Read stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			// Check if it's a warning or error
			if strings.Contains(line, "warning:") {
				build.AddWarning(line)
			} else {
				build.AddError(line)
			}
		}
	}()

	// Wait for completion
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	// Verify output was created
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		return fmt.Errorf("build output not found: %s", outputPath)
	}

	build.Artifact = outputPath
	build.AddOutput(fmt.Sprintf("Build successful: %s", outputPath))

	return nil
}

// runCustomBuildCommand runs a custom build command
func (b *GoBuilder) runCustomBuildCommand(ctx context.Context, workDir, buildCmd string, build *builds.Build) error {
	build.AddOutput(fmt.Sprintf("Running custom build command: %s", buildCmd))

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/c", buildCmd)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", buildCmd)
	}

	cmd.Dir = workDir
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if err != nil {
		build.AddError(string(output))
		return fmt.Errorf("custom build command failed: %w", err)
	}

	build.AddOutput(string(output))
	return nil
}

// CanBuild checks if this builder can build the component
func (b *GoBuilder) CanBuild(component *projects.Component) bool {
	return projects.IsGoComponent(component.Type)
}

// SetLdflags sets the ldflags for Go builds
func (b *GoBuilder) SetLdflags(ldflags string) {
	b.ldflags = ldflags
}

// SetVerbose sets verbose mode
func (b *GoBuilder) SetVerbose(verbose bool) {
	b.verbose = verbose
}
