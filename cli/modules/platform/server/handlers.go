package server

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"csd-devtrack/cli/modules/core/processes"
	"csd-devtrack/cli/modules/core/projects"
	"csd-devtrack/cli/modules/platform/builder"
	"csd-devtrack/cli/modules/platform/git"
	"csd-devtrack/cli/modules/platform/supervisor"
)

// APIHandler contains the dependencies for API handlers
type APIHandler struct {
	projectService *projects.Service
	processService *processes.Service
	processMgr     *supervisor.Manager
	buildOrch      *builder.Orchestrator
	gitService     *git.Service
}

// NewAPIHandler creates a new API handler
func NewAPIHandler(
	projectService *projects.Service,
	processService *processes.Service,
	processMgr *supervisor.Manager,
	buildOrch *builder.Orchestrator,
	gitService *git.Service,
) *APIHandler {
	return &APIHandler{
		projectService: projectService,
		processService: processService,
		processMgr:     processMgr,
		buildOrch:      buildOrch,
		gitService:     gitService,
	}
}

// Response helpers
func jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func errorResponse(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// ============================================
// Project handlers
// ============================================

// HandleListProjects returns all projects
func (h *APIHandler) HandleListProjects(w http.ResponseWriter, r *http.Request) {
	projectsList := h.projectService.ListProjects()
	jsonResponse(w, projectsList)
}

// HandleGetProject returns a specific project
func (h *APIHandler) HandleGetProject(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if projectID == "" {
		errorResponse(w, http.StatusBadRequest, "project ID required")
		return
	}

	project, err := h.projectService.GetProject(projectID)
	if err != nil {
		errorResponse(w, http.StatusNotFound, err.Error())
		return
	}

	jsonResponse(w, project)
}

// HandleAddProject adds a new project
func (h *APIHandler) HandleAddProject(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
		Name string `json:"name,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	var project *projects.Project
	var err error

	if req.Name != "" {
		project, err = h.projectService.AddProjectWithName(req.Path, req.Name)
	} else {
		project, err = h.projectService.AddProject(req.Path)
	}

	if err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	w.WriteHeader(http.StatusCreated)
	jsonResponse(w, project)
}

// HandleRemoveProject removes a project
func (h *APIHandler) HandleRemoveProject(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	if projectID == "" {
		errorResponse(w, http.StatusBadRequest, "project ID required")
		return
	}

	if err := h.projectService.RemoveProject(projectID); err != nil {
		errorResponse(w, http.StatusNotFound, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ============================================
// Process handlers
// ============================================

// HandleListProcesses returns all processes
func (h *APIHandler) HandleListProcesses(w http.ResponseWriter, r *http.Request) {
	allProcesses := h.processService.GetAllProcesses()
	jsonResponse(w, allProcesses)
}

// HandleStartProcess starts a process
func (h *APIHandler) HandleStartProcess(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("project")
	component := r.PathValue("component")

	if projectID == "" {
		errorResponse(w, http.StatusBadRequest, "project ID required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var err error
	if component != "" {
		err = h.processService.StartComponent(ctx, projectID, projects.ComponentType(component), h.processMgr)
	} else {
		project, projErr := h.projectService.GetProject(projectID)
		if projErr != nil {
			errorResponse(w, http.StatusNotFound, projErr.Error())
			return
		}

		for _, comp := range project.GetEnabledComponents() {
			if startErr := h.processService.StartComponent(ctx, projectID, comp.Type, h.processMgr); startErr != nil {
				err = startErr
			}
		}
	}

	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, map[string]string{"status": "started"})
}

// HandleStopProcess stops a process
func (h *APIHandler) HandleStopProcess(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("project")
	component := r.PathValue("component")

	if projectID == "" {
		errorResponse(w, http.StatusBadRequest, "project ID required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	var err error
	if component != "" {
		processID := projectID + "/" + component
		err = h.processService.StopProcess(ctx, processID, h.processMgr, false)
	} else {
		err = h.processService.StopProject(ctx, projectID, h.processMgr, false)
	}

	if err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, map[string]string{"status": "stopped"})
}

// HandleRestartProcess restarts a process
func (h *APIHandler) HandleRestartProcess(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("project")
	component := r.PathValue("component")

	if projectID == "" || component == "" {
		errorResponse(w, http.StatusBadRequest, "project ID and component required")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	processID := projectID + "/" + component
	if err := h.processService.RestartProcess(ctx, processID, h.processMgr); err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, map[string]string{"status": "restarted"})
}

// HandleKillProcess kills a process
func (h *APIHandler) HandleKillProcess(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("project")
	component := r.PathValue("component")

	if projectID == "" || component == "" {
		errorResponse(w, http.StatusBadRequest, "project ID and component required")
		return
	}

	processID := projectID + "/" + component
	if err := h.processService.KillProcess(processID, h.processMgr); err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, map[string]string{"status": "killed"})
}

// ============================================
// Build handlers
// ============================================

// HandleBuild starts a build
func (h *APIHandler) HandleBuild(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("project")
	component := r.PathValue("component")

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Minute)
	defer cancel()

	go func() {
		if component != "" {
			h.buildOrch.BuildComponent(ctx, projectID, projects.ComponentType(component))
		} else if projectID != "" {
			h.buildOrch.BuildProject(ctx, projectID)
		} else {
			h.buildOrch.BuildAll(ctx)
		}
	}()

	jsonResponse(w, map[string]string{"status": "building"})
}

// ============================================
// Git handlers
// ============================================

// HandleGitStatus returns git status
func (h *APIHandler) HandleGitStatus(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("project")

	if projectID == "" {
		// Get all projects status
		allStatus := h.gitService.GetAllStatus()
		jsonResponse(w, allStatus)
		return
	}

	status, err := h.gitService.GetStatus(projectID)
	if err != nil {
		errorResponse(w, http.StatusNotFound, err.Error())
		return
	}

	jsonResponse(w, status)
}

// HandleGitDiff returns git diff
func (h *APIHandler) HandleGitDiff(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("project")
	if projectID == "" {
		errorResponse(w, http.StatusBadRequest, "project ID required")
		return
	}

	opts := git.DefaultDiffOptions()
	if r.URL.Query().Get("staged") == "true" {
		opts.Staged = true
	}
	if path := r.URL.Query().Get("path"); path != "" {
		opts.Path = path
	}

	diff, err := h.gitService.GetDiff(projectID, opts)
	if err != nil {
		errorResponse(w, http.StatusNotFound, err.Error())
		return
	}

	jsonResponse(w, diff)
}

// HandleGitLog returns git log
func (h *APIHandler) HandleGitLog(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("project")
	if projectID == "" {
		errorResponse(w, http.StatusBadRequest, "project ID required")
		return
	}

	opts := git.DefaultLogOptions()

	commits, err := h.gitService.GetLog(projectID, opts)
	if err != nil {
		errorResponse(w, http.StatusNotFound, err.Error())
		return
	}

	jsonResponse(w, commits)
}

// ============================================
// Logs handlers
// ============================================

// HandleLogs returns logs for a process
func (h *APIHandler) HandleLogs(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("project")
	component := r.PathValue("component")

	if projectID == "" || component == "" {
		errorResponse(w, http.StatusBadRequest, "project ID and component required")
		return
	}

	proc := h.processService.GetProcessForComponent(projectID, projects.ComponentType(component))
	if proc == nil {
		errorResponse(w, http.StatusNotFound, "process not found")
		return
	}

	logs := proc.GetLogs(1000)
	jsonResponse(w, logs)
}

// ============================================
// Status handler
// ============================================

// StatusResponse represents the overall status
type StatusResponse struct {
	Projects  int  `json:"projects"`
	Running   int  `json:"running"`
	Building  int  `json:"building"`
	WebSocket int  `json:"websocket_clients"`
	Connected bool `json:"connected"`
}

// HandleStatus returns overall system status
func (h *APIHandler) HandleStatus(w http.ResponseWriter, r *http.Request) {
	projectsList := h.projectService.ListProjects()
	allProcesses := h.processService.GetAllProcesses()

	running := 0
	for _, p := range allProcesses {
		if p.IsRunning() {
			running++
		}
	}

	status := StatusResponse{
		Projects:  len(projectsList),
		Running:   running,
		Building:  0, // TODO: track active builds
		Connected: true,
	}

	jsonResponse(w, status)
}
