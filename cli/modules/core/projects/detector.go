package projects

import (
	"os"
	"path/filepath"
	"strings"
)

// Detector handles auto-detection of project structure
type Detector struct{}

// NewDetector creates a new project detector
func NewDetector() *Detector {
	return &Detector{}
}

// DetectProject auto-detects project structure from a path
func (d *Detector) DetectProject(projectPath string) (*Project, error) {
	absPath, err := filepath.Abs(projectPath)
	if err != nil {
		return nil, err
	}

	// Check if path exists
	info, err := os.Stat(absPath)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, &ProjectError{Message: "path is not a directory", Path: absPath}
	}

	// Extract project name and ID from directory name
	dirName := filepath.Base(absPath)
	projectID := strings.ToLower(strings.ReplaceAll(dirName, " ", "-"))
	projectName := dirName

	// Detect components
	components := make(map[ComponentType]*Component)

	// Check for agent/
	if comp := d.detectGoComponent(absPath, "agent", ComponentAgent); comp != nil {
		components[ComponentAgent] = comp
	}

	// Check for cli/
	if comp := d.detectGoComponent(absPath, "cli", ComponentCLI); comp != nil {
		components[ComponentCLI] = comp
	}

	// Check for backend/
	if comp := d.detectGoComponent(absPath, "backend", ComponentBackend); comp != nil {
		components[ComponentBackend] = comp
	}

	// Check for frontend/
	if comp := d.detectFrontendComponent(absPath, "frontend"); comp != nil {
		components[ComponentFrontend] = comp
	}

	// A project must have at least one of the 4 standard components
	// (agent/, cli/, backend/, frontend/)
	if len(components) == 0 {
		return nil, &ProjectError{Message: "no standard components found (agent/, cli/, backend/, frontend/)", Path: absPath}
	}

	// Determine project type
	projectType := d.determineProjectType(components)

	project := &Project{
		ID:         projectID,
		Name:       projectName,
		Path:       absPath,
		Type:       projectType,
		Components: components,
	}

	return project, nil
}

// detectGoComponent detects a Go component in a subdirectory
func (d *Detector) detectGoComponent(basePath, subdir string, ct ComponentType) *Component {
	compPath := filepath.Join(basePath, subdir)

	// Check if directory exists
	if _, err := os.Stat(compPath); os.IsNotExist(err) {
		return nil
	}

	// Check for go.mod
	goModPath := filepath.Join(compPath, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		return nil
	}

	// Find main Go file (entry point)
	entryPoint, binary := d.findGoEntryPoint(compPath)
	if entryPoint == "" {
		return nil
	}

	return &Component{
		Type:       ct,
		Path:       subdir + "/",
		EntryPoint: entryPoint,
		Binary:     binary,
		Enabled:    true,
	}
}

// findGoEntryPoint finds the main Go file in a directory
func (d *Detector) findGoEntryPoint(dirPath string) (entryPoint string, binary string) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return "", ""
	}

	// Look for common patterns
	patterns := []string{
		"main.go",
		"*.go", // Any .go file with main package
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".go") {
			continue
		}

		// Check if it's a main file
		if name == "main.go" {
			return name, strings.TrimSuffix(filepath.Base(dirPath), "/")
		}

		// Check for csd-* pattern (common in csd projects)
		if strings.HasPrefix(name, "csd-") && strings.HasSuffix(name, ".go") {
			binary := strings.TrimSuffix(name, ".go")
			return name, binary
		}
	}

	// Fallback: check for any .go file that might be main
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(filepath.Join(dirPath, pattern))
		for _, match := range matches {
			name := filepath.Base(match)
			if d.isMainGoFile(match) {
				binary := strings.TrimSuffix(name, ".go")
				if binary == "main" {
					binary = filepath.Base(dirPath)
				}
				return name, binary
			}
		}
	}

	return "", ""
}

// isMainGoFile checks if a Go file contains package main
func (d *Detector) isMainGoFile(filePath string) bool {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}

	// Simple check for package main
	return strings.Contains(string(content), "package main")
}

// detectFrontendComponent detects a frontend component
func (d *Detector) detectFrontendComponent(basePath, subdir string) *Component {
	compPath := filepath.Join(basePath, subdir)

	// Check if directory exists
	if _, err := os.Stat(compPath); os.IsNotExist(err) {
		return nil
	}

	// Check for package.json
	packageJSON := filepath.Join(compPath, "package.json")
	if _, err := os.Stat(packageJSON); os.IsNotExist(err) {
		return nil
	}

	// Detect default port from common configs
	port := d.detectFrontendPort(compPath)

	return &Component{
		Type:    ComponentFrontend,
		Path:    subdir + "/",
		Port:    port,
		Enabled: true,
	}
}

// detectFrontendPort tries to detect the frontend dev server port
func (d *Detector) detectFrontendPort(compPath string) int {
	// Default ports for common frameworks
	// Vite: 5173, Create React App: 3000, Next.js: 3000

	// Check for vite.config.ts/js
	for _, config := range []string{"vite.config.ts", "vite.config.js"} {
		if _, err := os.Stat(filepath.Join(compPath, config)); err == nil {
			return 5173 // Vite default
		}
	}

	// Check for next.config.js
	if _, err := os.Stat(filepath.Join(compPath, "next.config.js")); err == nil {
		return 3000 // Next.js default
	}

	return 3000 // Default fallback
}

// determineProjectType determines the project type based on detected components
func (d *Detector) determineProjectType(components map[ComponentType]*Component) ProjectType {
	hasAgent := components[ComponentAgent] != nil
	hasCLI := components[ComponentCLI] != nil
	hasBackend := components[ComponentBackend] != nil
	hasFrontend := components[ComponentFrontend] != nil

	switch {
	case hasAgent && hasCLI && hasBackend && hasFrontend:
		return ProjectTypeFullStack
	case (hasCLI || hasBackend) && hasFrontend:
		return ProjectTypeBackendOnly // CLI + Backend + Frontend or similar
	case hasFrontend && !hasCLI && !hasBackend && !hasAgent:
		return ProjectTypeFrontendOnly
	case (hasCLI || hasBackend) && !hasFrontend && !hasAgent:
		if len(components) == 1 {
			return ProjectTypeDaemon
		}
		return ProjectTypeBackendOnly
	default:
		return ProjectTypeCustom
	}
}

// ProjectError represents a project detection error
type ProjectError struct {
	Message string
	Path    string
}

func (e *ProjectError) Error() string {
	return e.Message + ": " + e.Path
}
