package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"csd-devtrack/cli/modules/core/processes"
	"csd-devtrack/cli/modules/core/projects"
	"csd-devtrack/cli/modules/platform/builder"
	"csd-devtrack/cli/modules/platform/config"
	"csd-devtrack/cli/modules/platform/eventbus"
	"csd-devtrack/cli/modules/platform/git"
	"csd-devtrack/cli/modules/platform/supervisor"
)

// Server is the HTTP/WebSocket server for the web UI
type Server struct {
	mu sync.RWMutex

	// Configuration
	httpPort int
	wsPort   int
	enabled  bool

	// Services
	projectService *projects.Service
	processService *processes.Service
	processMgr     *supervisor.Manager
	buildOrch      *builder.Orchestrator
	gitService     *git.Service
	config         *config.Config

	// Server components
	httpServer *http.Server
	wsHub      *WSHub
	eventBus   *eventbus.Bus

	// State
	running bool
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewServer creates a new server
func NewServer(
	projectService *projects.Service,
	cfg *config.Config,
) *Server {
	httpPort := 9099
	wsPort := 9098
	enabled := true

	if cfg != nil && cfg.Settings != nil {
		if cfg.Settings.WebPort > 0 {
			httpPort = cfg.Settings.WebPort
		}
		if cfg.Settings.WebSocketPort > 0 {
			wsPort = cfg.Settings.WebSocketPort
		}
		enabled = cfg.Settings.WebEnabled
	}

	bus := eventbus.Global()

	return &Server{
		httpPort:       httpPort,
		wsPort:         wsPort,
		enabled:        enabled,
		projectService: projectService,
		config:         cfg,
		eventBus:       bus,
		wsHub:          NewWSHub(bus),
	}
}

// Initialize sets up the server
func (s *Server) Initialize() error {
	if !s.enabled {
		return nil
	}

	// Initialize services
	s.processService = processes.NewService(s.projectService)
	s.processMgr = supervisor.NewManager(s.processService)
	s.gitService = git.NewService(s.projectService)

	parallelBuilds := 4
	if s.config != nil && s.config.Settings != nil {
		parallelBuilds = s.config.Settings.ParallelBuilds
	}
	s.buildOrch = builder.NewOrchestrator(s.projectService, parallelBuilds)

	return nil
}

// Start starts the server
func (s *Server) Start(ctx context.Context) error {
	if !s.enabled {
		log.Println("Web server is disabled")
		return nil
	}

	s.ctx, s.cancel = context.WithCancel(ctx)

	// Start WebSocket hub
	go s.wsHub.Run()

	// Create HTTP handler
	handler := s.createHandler()

	// Create HTTP server
	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.httpPort),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	s.mu.Lock()
	s.running = true
	s.mu.Unlock()

	log.Printf("Web server starting on http://localhost:%d", s.httpPort)
	log.Printf("WebSocket available on ws://localhost:%d/ws", s.httpPort)

	// Start server
	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP server error: %v", err)
		}
	}()

	return nil
}

// Stop stops the server
func (s *Server) Stop() error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = false
	s.mu.Unlock()

	if s.cancel != nil {
		s.cancel()
	}

	if s.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.httpServer.Shutdown(ctx)
	}

	return nil
}

// IsRunning returns whether the server is running
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// createHandler creates the HTTP handler with all routes
func (s *Server) createHandler() http.Handler {
	mux := http.NewServeMux()

	apiHandler := NewAPIHandler(
		s.projectService,
		s.processService,
		s.processMgr,
		s.buildOrch,
		s.gitService,
	)

	// Static files (for embedded frontend)
	mux.HandleFunc("GET /", s.handleRoot)

	// Health check
	mux.HandleFunc("GET /health", s.handleHealth)

	// WebSocket
	mux.HandleFunc("GET /ws", s.wsHub.ServeWS)

	// API routes
	mux.HandleFunc("GET /api/status", apiHandler.HandleStatus)

	// Projects
	mux.HandleFunc("GET /api/projects", apiHandler.HandleListProjects)
	mux.HandleFunc("GET /api/projects/{id}", apiHandler.HandleGetProject)
	mux.HandleFunc("POST /api/projects", apiHandler.HandleAddProject)
	mux.HandleFunc("DELETE /api/projects/{id}", apiHandler.HandleRemoveProject)

	// Processes
	mux.HandleFunc("GET /api/processes", apiHandler.HandleListProcesses)
	mux.HandleFunc("POST /api/processes/{project}/start", apiHandler.HandleStartProcess)
	mux.HandleFunc("POST /api/processes/{project}/{component}/start", apiHandler.HandleStartProcess)
	mux.HandleFunc("POST /api/processes/{project}/stop", apiHandler.HandleStopProcess)
	mux.HandleFunc("POST /api/processes/{project}/{component}/stop", apiHandler.HandleStopProcess)
	mux.HandleFunc("POST /api/processes/{project}/{component}/restart", apiHandler.HandleRestartProcess)
	mux.HandleFunc("POST /api/processes/{project}/{component}/kill", apiHandler.HandleKillProcess)

	// Builds
	mux.HandleFunc("POST /api/build", apiHandler.HandleBuild)
	mux.HandleFunc("POST /api/build/{project}", apiHandler.HandleBuild)
	mux.HandleFunc("POST /api/build/{project}/{component}", apiHandler.HandleBuild)

	// Git
	mux.HandleFunc("GET /api/git/status", apiHandler.HandleGitStatus)
	mux.HandleFunc("GET /api/git/{project}/status", apiHandler.HandleGitStatus)
	mux.HandleFunc("GET /api/git/{project}/diff", apiHandler.HandleGitDiff)
	mux.HandleFunc("GET /api/git/{project}/log", apiHandler.HandleGitLog)

	// Logs
	mux.HandleFunc("GET /api/logs/{project}/{component}", apiHandler.HandleLogs)

	// Apply CORS middleware
	return corsMiddleware(mux)
}

// handleRoot serves the main page
func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
    <title>CSD DevTrack</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; margin: 40px; }
        h1 { color: #7C3AED; }
        .status { background: #f0f0f0; padding: 20px; border-radius: 8px; margin: 20px 0; }
        code { background: #e0e0e0; padding: 2px 6px; border-radius: 4px; }
    </style>
</head>
<body>
    <h1>CSD DevTrack</h1>
    <div class="status">
        <p>API Server is running</p>
        <p>WebSocket available at: <code>ws://localhost:` + fmt.Sprintf("%d", s.httpPort) + `/ws</code></p>
    </div>
    <h2>API Endpoints</h2>
    <ul>
        <li><code>GET /api/status</code> - System status</li>
        <li><code>GET /api/projects</code> - List projects</li>
        <li><code>GET /api/processes</code> - List processes</li>
        <li><code>GET /api/git/status</code> - Git status</li>
        <li><code>POST /api/build/{project}</code> - Start build</li>
        <li><code>POST /api/processes/{project}/start</code> - Start process</li>
        <li><code>POST /api/processes/{project}/stop</code> - Stop process</li>
    </ul>
    <p><em>For the full web UI, use Module Federation with csd-core.</em></p>
</body>
</html>`))
}

// handleHealth is a health check endpoint
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

// corsMiddleware adds CORS headers
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// GetEventBus returns the event bus
func (s *Server) GetEventBus() *eventbus.Bus {
	return s.eventBus
}

// GetWSHub returns the WebSocket hub
func (s *Server) GetWSHub() *WSHub {
	return s.wsHub
}

// GetPort returns the HTTP port
func (s *Server) GetPort() int {
	return s.httpPort
}
