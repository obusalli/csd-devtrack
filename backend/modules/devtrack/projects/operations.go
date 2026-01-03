package projects

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"csd-devtrack/backend/modules/platform/graphql"
	"csd-devtrack/cli/modules/core/projects"
)

var (
	projectService *projects.Service
	once           sync.Once
)

func init() {
	// Register queries
	graphql.RegisterQuery("projects", "List all projects", "", ListProjects)
	graphql.RegisterQuery("projectsCount", "Count projects", "", CountProjects)
	graphql.RegisterQuery("project", "Get a project by ID", "", GetProject)
}

// getProjectService returns the project service singleton
// Uses the CLI's project service directly
func getProjectService() *projects.Service {
	once.Do(func() {
		// Get config path - use standard csd-devtrack config location
		homeDir, _ := os.UserHomeDir()
		configPath := filepath.Join(homeDir, ".csd-devtrack", "csd-devtrack.yaml")

		repo := projects.NewRepository(configPath)
		repo.Load()
		projectService = projects.NewService(repo)
	})
	return projectService
}

// ListProjects handles the projects query
func ListProjects(ctx context.Context, w http.ResponseWriter, variables map[string]interface{}) {
	service := getProjectService()
	service.Load() // Reload from disk

	projectList := service.ListProjects()

	// Convert to summaries
	summaries := make([]*projects.ProjectSummary, len(projectList))
	for i, p := range projectList {
		summaries[i] = p.ToSummary()
	}

	graphql.SendDataMultiple(w, map[string]interface{}{
		"projects":      summaries,
		"projectsCount": len(summaries),
	})
}

// CountProjects handles the projectsCount query
func CountProjects(ctx context.Context, w http.ResponseWriter, variables map[string]interface{}) {
	service := getProjectService()
	service.Load()

	graphql.SendData(w, "projectsCount", len(service.ListProjects()))
}

// GetProject handles the project query
func GetProject(ctx context.Context, w http.ResponseWriter, variables map[string]interface{}) {
	id, ok := graphql.ParseString(variables, "id")
	if !ok || id == "" {
		graphql.SendError(w, nil, "id is required")
		return
	}

	service := getProjectService()
	service.Load()

	project, err := service.GetProject(id)
	if err != nil {
		graphql.SendError(w, err, "project")
		return
	}

	graphql.SendData(w, "project", project)
}
