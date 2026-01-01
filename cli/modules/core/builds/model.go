package builds

import (
	"time"

	"csd-devtrack/cli/modules/core/projects"
)

// BuildStatus represents the status of a build
type BuildStatus string

const (
	BuildStatusPending  BuildStatus = "pending"
	BuildStatusRunning  BuildStatus = "running"
	BuildStatusSuccess  BuildStatus = "success"
	BuildStatusFailed   BuildStatus = "failed"
	BuildStatusCanceled BuildStatus = "canceled"
)

// Build represents a build operation
type Build struct {
	ID          string                 `json:"id"`
	ProjectID   string                 `json:"project_id"`
	Component   projects.ComponentType `json:"component"`
	Status      BuildStatus            `json:"status"`
	StartedAt   time.Time              `json:"started_at"`
	FinishedAt  *time.Time             `json:"finished_at,omitempty"`
	Duration    time.Duration          `json:"duration"`
	Output      []string               `json:"output"`
	Errors      []string               `json:"errors"`
	Warnings    []string               `json:"warnings"`
	ExitCode    int                    `json:"exit_code"`
	Artifact    string                 `json:"artifact,omitempty"`
}

// NewBuild creates a new build
func NewBuild(projectID string, component projects.ComponentType) *Build {
	return &Build{
		ID:        generateBuildID(),
		ProjectID: projectID,
		Component: component,
		Status:    BuildStatusPending,
		Output:    []string{},
		Errors:    []string{},
		Warnings:  []string{},
	}
}

// Start marks the build as started
func (b *Build) Start() {
	b.Status = BuildStatusRunning
	b.StartedAt = time.Now()
}

// Finish marks the build as finished
func (b *Build) Finish(exitCode int) {
	now := time.Now()
	b.FinishedAt = &now
	b.ExitCode = exitCode
	b.Duration = now.Sub(b.StartedAt)

	if exitCode == 0 {
		b.Status = BuildStatusSuccess
	} else {
		b.Status = BuildStatusFailed
	}
}

// Cancel marks the build as canceled
func (b *Build) Cancel() {
	now := time.Now()
	b.FinishedAt = &now
	b.Status = BuildStatusCanceled
	if !b.StartedAt.IsZero() {
		b.Duration = now.Sub(b.StartedAt)
	}
}

// AddOutput adds output lines to the build
func (b *Build) AddOutput(lines ...string) {
	b.Output = append(b.Output, lines...)
}

// AddError adds error lines to the build
func (b *Build) AddError(lines ...string) {
	b.Errors = append(b.Errors, lines...)
}

// AddWarning adds warning lines to the build
func (b *Build) AddWarning(lines ...string) {
	b.Warnings = append(b.Warnings, lines...)
}

// IsComplete returns true if the build is complete
func (b *Build) IsComplete() bool {
	return b.Status == BuildStatusSuccess ||
		b.Status == BuildStatusFailed ||
		b.Status == BuildStatusCanceled
}

// IsSuccess returns true if the build succeeded
func (b *Build) IsSuccess() bool {
	return b.Status == BuildStatusSuccess
}

// BuildResult contains the result of a build operation
type BuildResult struct {
	Build    *Build
	Error    error
}

// BuildPlan represents a plan for building multiple components
type BuildPlan struct {
	ProjectID  string                   `json:"project_id"`
	Components []projects.ComponentType `json:"components"`
	Parallel   bool                     `json:"parallel"`
}

// BuildEvent represents an event during a build
type BuildEvent struct {
	Type      BuildEventType `json:"type"`
	BuildID   string         `json:"build_id"`
	ProjectID string         `json:"project_id"`
	Component string         `json:"component"`
	Message   string         `json:"message"`
	Timestamp time.Time      `json:"timestamp"`
}

// BuildEventType represents the type of build event
type BuildEventType string

const (
	BuildEventStarted  BuildEventType = "started"
	BuildEventOutput   BuildEventType = "output"
	BuildEventError    BuildEventType = "error"
	BuildEventWarning  BuildEventType = "warning"
	BuildEventFinished BuildEventType = "finished"
)

// generateBuildID generates a unique build ID
func generateBuildID() string {
	return time.Now().Format("20060102-150405") + "-" + randomString(6)
}

// randomString generates a random string of the given length
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
		time.Sleep(time.Nanosecond) // Ensure different values
	}
	return string(b)
}
