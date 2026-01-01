package builder

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"csd-devtrack/cli/modules/core/builds"
	"csd-devtrack/cli/modules/core/projects"
)

// FrontendBuilder builds frontend components
type FrontendBuilder struct {
	npmPath  string
	nodePath string
	verbose  bool
}

// NewFrontendBuilder creates a new frontend builder
func NewFrontendBuilder() *FrontendBuilder {
	return &FrontendBuilder{
		npmPath:  findNpm(),
		nodePath: findNode(),
		verbose:  false,
	}
}

// findNpm finds the npm executable
func findNpm() string {
	if path, err := exec.LookPath("npm"); err == nil {
		return path
	}

	paths := []string{
		"/usr/local/bin/npm",
		"/opt/homebrew/bin/npm",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return "npm"
}

// findNode finds the node executable
func findNode() string {
	if path, err := exec.LookPath("node"); err == nil {
		return path
	}

	paths := []string{
		"/usr/local/bin/node",
		"/opt/homebrew/bin/node",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return "node"
}

// Build builds a frontend component
func (b *FrontendBuilder) Build(ctx context.Context, project *projects.Project, component *projects.Component, build *builds.Build) error {
	workDir := filepath.Join(project.Path, component.Path)

	// Check if workDir exists
	if _, err := os.Stat(workDir); os.IsNotExist(err) {
		return fmt.Errorf("frontend directory not found: %s", workDir)
	}

	// Check for package.json
	packageJSONPath := filepath.Join(workDir, "package.json")
	if _, err := os.Stat(packageJSONPath); os.IsNotExist(err) {
		return fmt.Errorf("package.json not found in: %s", workDir)
	}

	// Use custom build command if specified
	if component.BuildCmd != "" {
		return b.runCustomBuildCommand(ctx, workDir, component.BuildCmd, build)
	}

	// Detect build tool and command
	buildScript := b.detectBuildScript(workDir)

	build.AddOutput(fmt.Sprintf("Building frontend in %s...", workDir))

	// First, check if node_modules exists, if not run npm install
	nodeModulesPath := filepath.Join(workDir, "node_modules")
	if _, err := os.Stat(nodeModulesPath); os.IsNotExist(err) {
		build.AddOutput("Installing dependencies (npm install)...")
		if err := b.runNpmCommand(ctx, workDir, []string{"install"}, build); err != nil {
			return fmt.Errorf("npm install failed: %w", err)
		}
	}

	// Run build
	build.AddOutput(fmt.Sprintf("Running build script: %s", buildScript))
	if err := b.runNpmCommand(ctx, workDir, []string{"run", buildScript}, build); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}

	// Check for build output
	buildOutputDir := b.detectBuildOutput(workDir)
	if buildOutputDir != "" {
		build.Artifact = buildOutputDir
		build.AddOutput(fmt.Sprintf("Build output: %s", buildOutputDir))
	}

	build.AddOutput("Frontend build successful")
	return nil
}

// detectBuildScript detects the appropriate build script
func (b *FrontendBuilder) detectBuildScript(workDir string) string {
	packageJSONPath := filepath.Join(workDir, "package.json")
	data, err := os.ReadFile(packageJSONPath)
	if err != nil {
		return "build"
	}

	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}

	if err := json.Unmarshal(data, &pkg); err != nil {
		return "build"
	}

	// Check for common build scripts in order of preference
	preferredScripts := []string{"build", "build:prod", "build:production"}
	for _, script := range preferredScripts {
		if _, ok := pkg.Scripts[script]; ok {
			return script
		}
	}

	return "build"
}

// detectBuildOutput detects the build output directory
func (b *FrontendBuilder) detectBuildOutput(workDir string) string {
	// Common build output directories
	outputs := []string{
		"dist",
		"build",
		"out",
		".next",
	}

	for _, output := range outputs {
		outputPath := filepath.Join(workDir, output)
		if _, err := os.Stat(outputPath); err == nil {
			return outputPath
		}
	}

	return ""
}

// runNpmCommand runs an npm command
func (b *FrontendBuilder) runNpmCommand(ctx context.Context, workDir string, args []string, build *builds.Build) error {
	cmd := exec.CommandContext(ctx, b.npmPath, args...)
	cmd.Dir = workDir
	cmd.Env = os.Environ()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	// Read stdout
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			build.AddOutput(scanner.Text())
		}
	}()

	// Read stderr
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			// npm outputs progress to stderr, not all is errors
			if strings.Contains(strings.ToLower(line), "error") {
				build.AddError(line)
			} else if strings.Contains(strings.ToLower(line), "warn") {
				build.AddWarning(line)
			} else {
				build.AddOutput(line)
			}
		}
	}()

	return cmd.Wait()
}

// runCustomBuildCommand runs a custom build command
func (b *FrontendBuilder) runCustomBuildCommand(ctx context.Context, workDir, buildCmd string, build *builds.Build) error {
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
func (b *FrontendBuilder) CanBuild(component *projects.Component) bool {
	return projects.IsFrontendComponent(component.Type)
}

// SetVerbose sets verbose mode
func (b *FrontendBuilder) SetVerbose(verbose bool) {
	b.verbose = verbose
}
