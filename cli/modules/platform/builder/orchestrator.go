package builder

import (
	"context"
	"fmt"
	"sync"
	"time"

	"csd-devtrack/cli/modules/core/builds"
	"csd-devtrack/cli/modules/core/projects"
	"csd-devtrack/cli/modules/platform/selfmgmt"
)

// Orchestrator coordinates builds across multiple projects and components
type Orchestrator struct {
	buildService    *builds.Service
	goBuilder       *GoBuilder
	frontendBuilder *FrontendBuilder
	selfMgr         *selfmgmt.SelfManager
	projectService  *projects.Service
	maxParallel     int
	mu              sync.Mutex
}

// NewOrchestrator creates a new build orchestrator
func NewOrchestrator(projectService *projects.Service, maxParallel int) *Orchestrator {
	buildService := builds.NewService(projectService)
	goBuilder := NewGoBuilder()
	frontendBuilder := NewFrontendBuilder()

	// Register builders
	buildService.RegisterBuilder(projects.ComponentAgent, goBuilder)
	buildService.RegisterBuilder(projects.ComponentCLI, goBuilder)
	buildService.RegisterBuilder(projects.ComponentBackend, goBuilder)
	buildService.RegisterBuilder(projects.ComponentFrontend, frontendBuilder)

	return &Orchestrator{
		buildService:    buildService,
		goBuilder:       goBuilder,
		frontendBuilder: frontendBuilder,
		selfMgr:         selfmgmt.NewSelfManager(projectService),
		projectService:  projectService,
		maxParallel:     maxParallel,
	}
}

// SetEventHandler sets the build event handler
func (o *Orchestrator) SetEventHandler(handler builds.BuildHandler) {
	o.buildService.SetEventHandler(handler)
}

// BuildProject builds all components of a project
func (o *Orchestrator) BuildProject(ctx context.Context, projectID string) ([]*builds.BuildResult, error) {
	return o.buildService.BuildProject(ctx, projectID)
}

// BuildComponent builds a specific component
func (o *Orchestrator) BuildComponent(ctx context.Context, projectID string, component projects.ComponentType) *builds.BuildResult {
	return o.buildService.BuildComponent(ctx, projectID, component)
}

// BuildAll builds all projects
func (o *Orchestrator) BuildAll(ctx context.Context) (map[string][]*builds.BuildResult, error) {
	return o.buildService.BuildAll(ctx)
}

// BuildMultiple builds multiple projects in parallel
func (o *Orchestrator) BuildMultiple(ctx context.Context, projectIDs []string) (map[string][]*builds.BuildResult, error) {
	results := make(map[string][]*builds.BuildResult)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Create semaphore for limiting parallel builds
	sem := make(chan struct{}, o.maxParallel)

	for _, projectID := range projectIDs {
		wg.Add(1)
		go func(pid string) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			projectResults, err := o.buildService.BuildProject(ctx, pid)
			if err != nil {
				projectResults = []*builds.BuildResult{
					{Error: err},
				}
			}

			mu.Lock()
			results[pid] = projectResults
			mu.Unlock()
		}(projectID)
	}

	wg.Wait()
	return results, nil
}

// BuildSummary represents a summary of build results
type BuildSummary struct {
	TotalProjects   int           `json:"total_projects"`
	SuccessProjects int           `json:"success_projects"`
	FailedProjects  int           `json:"failed_projects"`
	TotalComponents int           `json:"total_components"`
	SuccessComponents int         `json:"success_components"`
	FailedComponents  int         `json:"failed_components"`
	TotalDuration   time.Duration `json:"total_duration"`
	Errors          []string      `json:"errors,omitempty"`
}

// Summarize summarizes build results
func (o *Orchestrator) Summarize(results map[string][]*builds.BuildResult) *BuildSummary {
	summary := &BuildSummary{
		Errors: []string{},
	}

	for projectID, projectResults := range results {
		summary.TotalProjects++
		projectSuccess := true

		for _, result := range projectResults {
			summary.TotalComponents++

			if result.Error != nil {
				summary.FailedComponents++
				projectSuccess = false
				summary.Errors = append(summary.Errors,
					fmt.Sprintf("%s: %v", projectID, result.Error))
			} else if result.Build != nil {
				summary.TotalDuration += result.Build.Duration

				if result.Build.IsSuccess() {
					summary.SuccessComponents++
				} else {
					summary.FailedComponents++
					projectSuccess = false
					for _, err := range result.Build.Errors {
						summary.Errors = append(summary.Errors,
							fmt.Sprintf("%s/%s: %s", projectID, result.Build.Component, err))
					}
				}
			}
		}

		if projectSuccess {
			summary.SuccessProjects++
		} else {
			summary.FailedProjects++
		}
	}

	return summary
}

// PrintSummary prints a build summary to stdout
func (o *Orchestrator) PrintSummary(summary *BuildSummary) {
	fmt.Println()
	fmt.Println("Build Summary:")
	fmt.Println("──────────────")
	fmt.Printf("Projects:   %d/%d successful\n", summary.SuccessProjects, summary.TotalProjects)
	fmt.Printf("Components: %d/%d successful\n", summary.SuccessComponents, summary.TotalComponents)
	fmt.Printf("Duration:   %s\n", summary.TotalDuration.Round(time.Millisecond))

	if len(summary.Errors) > 0 {
		fmt.Println()
		fmt.Println("Errors:")
		for _, err := range summary.Errors {
			fmt.Printf("  • %s\n", err)
		}
	}
}

// GetBuild returns a build by ID
func (o *Orchestrator) GetBuild(buildID string) *builds.Build {
	return o.buildService.GetBuild(buildID)
}

// GetBuildsForProject returns all builds for a project
func (o *Orchestrator) GetBuildsForProject(projectID string) []*builds.Build {
	return o.buildService.GetBuildsForProject(projectID)
}

// IsSelfProject checks if a project is the self-managed project
func (o *Orchestrator) IsSelfProject(projectID string) bool {
	return o.selfMgr.IsSelfProject(projectID)
}

// BuildSelf safely builds the self project
func (o *Orchestrator) BuildSelf(ctx context.Context, apply bool) (*selfmgmt.BuildResult, error) {
	return o.selfMgr.SafeBuild(ctx, apply)
}

// GetSelfManager returns the self manager
func (o *Orchestrator) GetSelfManager() *selfmgmt.SelfManager {
	return o.selfMgr
}
