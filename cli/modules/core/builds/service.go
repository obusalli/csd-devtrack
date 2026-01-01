package builds

import (
	"context"
	"fmt"
	"sync"
	"time"

	"csd-devtrack/cli/modules/core/projects"
)

// BuildHandler is a function that handles build events
type BuildHandler func(event BuildEvent)

// Service provides build orchestration
type Service struct {
	projectService *projects.Service
	builders       map[projects.ComponentType]Builder
	builds         map[string]*Build
	mu             sync.RWMutex
	eventHandler   BuildHandler
}

// Builder interface for component builders
type Builder interface {
	Build(ctx context.Context, project *projects.Project, component *projects.Component, build *Build) error
	CanBuild(component *projects.Component) bool
}

// NewService creates a new build service
func NewService(projectService *projects.Service) *Service {
	return &Service{
		projectService: projectService,
		builders:       make(map[projects.ComponentType]Builder),
		builds:         make(map[string]*Build),
	}
}

// RegisterBuilder registers a builder for a component type
func (s *Service) RegisterBuilder(ct projects.ComponentType, builder Builder) {
	s.builders[ct] = builder
}

// SetEventHandler sets the build event handler
func (s *Service) SetEventHandler(handler BuildHandler) {
	s.eventHandler = handler
}

// BuildProject builds all components of a project
func (s *Service) BuildProject(ctx context.Context, projectID string) ([]*BuildResult, error) {
	project, err := s.projectService.GetProject(projectID)
	if err != nil {
		return nil, fmt.Errorf("project not found: %s", projectID)
	}

	components := project.GetEnabledComponents()
	if len(components) == 0 {
		return nil, fmt.Errorf("no enabled components in project: %s", projectID)
	}

	var results []*BuildResult
	for _, comp := range components {
		result := s.BuildComponent(ctx, projectID, comp.Type)
		results = append(results, result)

		// Stop on first failure
		if result.Error != nil {
			break
		}
	}

	return results, nil
}

// BuildComponent builds a specific component
func (s *Service) BuildComponent(ctx context.Context, projectID string, componentType projects.ComponentType) *BuildResult {
	project, err := s.projectService.GetProject(projectID)
	if err != nil {
		return &BuildResult{Error: fmt.Errorf("project not found: %s", projectID)}
	}

	component := project.GetComponent(componentType)
	if component == nil {
		return &BuildResult{Error: fmt.Errorf("component not found: %s", componentType)}
	}

	if !component.Enabled {
		return &BuildResult{Error: fmt.Errorf("component is disabled: %s", componentType)}
	}

	builder := s.builders[componentType]
	if builder == nil {
		return &BuildResult{Error: fmt.Errorf("no builder registered for: %s", componentType)}
	}

	// Create build
	build := NewBuild(projectID, componentType)
	s.storeBuild(build)

	// Emit start event
	s.emitEvent(BuildEvent{
		Type:      BuildEventStarted,
		BuildID:   build.ID,
		ProjectID: projectID,
		Component: string(componentType),
		Message:   fmt.Sprintf("Starting build of %s/%s", projectID, componentType),
	})

	// Start build
	build.Start()

	// Run builder
	err = builder.Build(ctx, project, component, build)

	if err != nil {
		build.Finish(1)
		build.AddError(err.Error())

		s.emitEvent(BuildEvent{
			Type:      BuildEventError,
			BuildID:   build.ID,
			ProjectID: projectID,
			Component: string(componentType),
			Message:   err.Error(),
		})
	} else {
		build.Finish(0)
	}

	// Emit finish event
	s.emitEvent(BuildEvent{
		Type:      BuildEventFinished,
		BuildID:   build.ID,
		ProjectID: projectID,
		Component: string(componentType),
		Message:   fmt.Sprintf("Build finished with status: %s", build.Status),
	})

	return &BuildResult{Build: build, Error: err}
}

// BuildAll builds all projects
func (s *Service) BuildAll(ctx context.Context) (map[string][]*BuildResult, error) {
	allProjects := s.projectService.ListProjects()
	results := make(map[string][]*BuildResult)

	for _, project := range allProjects {
		projectResults, err := s.BuildProject(ctx, project.ID)
		if err != nil {
			return results, err
		}
		results[project.ID] = projectResults
	}

	return results, nil
}

// GetBuild returns a build by ID
func (s *Service) GetBuild(buildID string) *Build {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.builds[buildID]
}

// GetBuildsForProject returns all builds for a project
func (s *Service) GetBuildsForProject(projectID string) []*Build {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var builds []*Build
	for _, b := range s.builds {
		if b.ProjectID == projectID {
			builds = append(builds, b)
		}
	}
	return builds
}

// storeBuild stores a build
func (s *Service) storeBuild(build *Build) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.builds[build.ID] = build
}

// emitEvent emits a build event
func (s *Service) emitEvent(event BuildEvent) {
	if s.eventHandler != nil {
		// Add timestamp if not set
		if event.Timestamp.IsZero() {
			event.Timestamp = time.Now()
		}
		s.eventHandler(event)
	}
}
