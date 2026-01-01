package selfmgmt

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"csd-devtrack/cli/modules/core/projects"
)

// SelfManager handles self-management operations for csd-devtrack
type SelfManager struct {
	projectService *projects.Service
	currentBinary  string
	projectID      string
}

// NewSelfManager creates a new self manager
func NewSelfManager(projectService *projects.Service) *SelfManager {
	// Get the current executable path
	execPath, err := os.Executable()
	if err != nil {
		execPath = os.Args[0]
	}

	return &SelfManager{
		projectService: projectService,
		currentBinary:  execPath,
		projectID:      "csd-devtrack", // Default self project ID
	}
}

// GetSelfProject returns the self-managed project
func (m *SelfManager) GetSelfProject() (*projects.Project, error) {
	// Find project with Self=true
	allProjects := m.projectService.ListProjects()
	for _, p := range allProjects {
		if p.Self {
			m.projectID = p.ID
			return p, nil
		}
	}

	// Try to find by ID
	project, err := m.projectService.GetProject(m.projectID)
	if err != nil {
		return nil, fmt.Errorf("self project not found: %w", err)
	}
	return project, nil
}

// BuildResult contains the result of a self-build
type BuildResult struct {
	Success     bool
	TempBinary  string
	Duration    time.Duration
	Output      []string
	Errors      []string
}

// BuildSelf builds a new version of csd-devtrack to a temporary location
func (m *SelfManager) BuildSelf(ctx context.Context) (*BuildResult, error) {
	project, err := m.GetSelfProject()
	if err != nil {
		return nil, err
	}

	result := &BuildResult{
		Output: make([]string, 0),
		Errors: make([]string, 0),
	}
	startTime := time.Now()

	// Get CLI component
	cliComp := project.GetComponent(projects.ComponentCLI)
	if cliComp == nil {
		return nil, fmt.Errorf("CLI component not found in self project")
	}

	// Create temp file for new binary
	tempDir := os.TempDir()
	tempBinary := filepath.Join(tempDir, "csd-devtrack-new")
	if runtime.GOOS == "windows" {
		tempBinary += ".exe"
	}

	// Build to temp location
	cliPath := filepath.Join(project.Path, cliComp.Path)
	entryPoint := cliComp.EntryPoint
	if entryPoint == "" {
		entryPoint = "csd-devtrack.go"
	}

	cmd := exec.CommandContext(ctx, "go", "build", "-o", tempBinary, filepath.Join(cliPath, entryPoint))
	cmd.Dir = cliPath

	output, err := cmd.CombinedOutput()
	result.Duration = time.Since(startTime)
	result.Output = append(result.Output, string(output))

	if err != nil {
		result.Success = false
		result.Errors = append(result.Errors, err.Error())
		return result, fmt.Errorf("build failed: %w", err)
	}

	// Verify the new binary works
	verifyCmd := exec.CommandContext(ctx, tempBinary, "version")
	verifyOutput, verifyErr := verifyCmd.CombinedOutput()
	if verifyErr != nil {
		result.Success = false
		result.Errors = append(result.Errors, "verification failed: "+string(verifyOutput))
		os.Remove(tempBinary)
		return result, fmt.Errorf("verification failed: %w", verifyErr)
	}

	result.Success = true
	result.TempBinary = tempBinary
	result.Output = append(result.Output, "Build verified: "+string(verifyOutput))

	return result, nil
}

// ApplyUpdate replaces the current binary with the new one
func (m *SelfManager) ApplyUpdate(tempBinary string) error {
	// Get current binary path
	currentBinary := m.currentBinary

	// On Windows, we need to rename the current binary first
	if runtime.GOOS == "windows" {
		oldBinary := currentBinary + ".old"
		if err := os.Rename(currentBinary, oldBinary); err != nil {
			return fmt.Errorf("failed to rename current binary: %w", err)
		}
		defer os.Remove(oldBinary)
	}

	// Copy new binary to current location
	input, err := os.ReadFile(tempBinary)
	if err != nil {
		return fmt.Errorf("failed to read new binary: %w", err)
	}

	// Get current binary permissions
	info, err := os.Stat(currentBinary)
	mode := os.FileMode(0755)
	if err == nil {
		mode = info.Mode()
	}

	if err := os.WriteFile(currentBinary, input, mode); err != nil {
		return fmt.Errorf("failed to write new binary: %w", err)
	}

	// Clean up temp binary
	os.Remove(tempBinary)

	return nil
}

// NeedsRestart returns true if the application should restart after an update
func (m *SelfManager) NeedsRestart() bool {
	return true
}

// GetCurrentVersion returns the current version information
func (m *SelfManager) GetCurrentVersion() (string, string, error) {
	cmd := exec.Command(m.currentBinary, "version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", err
	}
	return string(output), m.currentBinary, nil
}

// IsSelfProject checks if a project is the self-managed project
func (m *SelfManager) IsSelfProject(projectID string) bool {
	project, err := m.GetSelfProject()
	if err != nil {
		return false
	}
	return project.ID == projectID
}

// SafeBuild builds the self project safely
// It first builds to a temp location, verifies, then optionally applies
func (m *SelfManager) SafeBuild(ctx context.Context, apply bool) (*BuildResult, error) {
	result, err := m.BuildSelf(ctx)
	if err != nil {
		return result, err
	}

	if !result.Success {
		return result, fmt.Errorf("build verification failed")
	}

	if apply {
		if err := m.ApplyUpdate(result.TempBinary); err != nil {
			return result, fmt.Errorf("failed to apply update: %w", err)
		}
		result.Output = append(result.Output, "Update applied successfully")
	}

	return result, nil
}
