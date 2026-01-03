package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"csd-devtrack/backend/modules/platform/config"
	csdcore "csd-devtrack/backend/modules/platform/csd-core"
	"csd-devtrack/backend/modules/platform/graphql"
	"csd-devtrack/backend/modules/platform/middleware"

	// Import terminal module to register HTTP handlers
	"csd-devtrack/backend/modules/devtrack/terminal"
)

// Server represents the csd-devtrack server
type Server struct {
	cfg           *config.Config
	httpServer    *http.Server
	csdCoreClient *csdcore.Client
}

// NewServer creates a new server instance
func NewServer(cfg *config.Config) (*Server, error) {
	// Create csd-core client
	csdCoreClient := csdcore.NewClient(&cfg.CSDCore)
	log.Printf("CSD-Core client configured: %s", cfg.CSDCore.URL)

	// Create GraphQL handler
	graphqlHandler := graphql.NewHandler(csdCoreClient)

	// Setup routes
	mux := http.NewServeMux()

	// API base path
	const apiBasePath = "/devtrack/api/latest"

	// Health check
	mux.HandleFunc(apiBasePath+"/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","timestamp":"` + time.Now().UTC().Format(time.RFC3339) + `"}`))
	})

	// GraphQL endpoint
	mux.Handle(apiBasePath+"/query", graphqlHandler)

	// Terminal token validation endpoint (called by csd-core)
	terminal.RegisterHTTPHandlers(mux)

	// Apply middleware chain: security headers -> CORS -> auth
	handler := securityHeadersMiddleware(corsMiddleware(cfg.CORS)(middleware.AuthMiddleware(mux)))

	server := &Server{
		cfg:           cfg,
		csdCoreClient: csdCoreClient,
		httpServer: &http.Server{
			Addr:    fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port),
			Handler: handler,
		},
	}

	return server, nil
}

// Start starts the server
func (s *Server) Start() error {
	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("Server starting on %s", s.httpServer.Addr)
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	<-stop
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown failed: %w", err)
	}

	log.Println("Server stopped")
	return nil
}

// securityHeadersMiddleware adds security headers to all responses
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")

		// Cache control for API responses
		if strings.HasPrefix(r.URL.Path, "/devtrack/api/") {
			w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, private")
			w.Header().Set("Pragma", "no-cache")
		}

		next.ServeHTTP(w, r)
	})
}

// corsMiddleware returns a CORS middleware
func corsMiddleware(cfg config.CORSConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Check if origin is allowed
			allowed := false
			isWildcard := false
			for _, allowedOrigin := range cfg.AllowedOrigins {
				if allowedOrigin == "*" {
					allowed = true
					isWildcard = true
					break
				}
				if allowedOrigin == origin {
					allowed = true
					break
				}
			}

			if allowed && origin != "" {
				if isWildcard {
					w.Header().Set("Access-Control-Allow-Origin", "*")
				} else {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
				w.Header().Set("Access-Control-Allow-Methods", strings.Join(cfg.AllowedMethods, ", "))
				w.Header().Set("Access-Control-Allow-Headers", strings.Join(cfg.AllowedHeaders, ", "))
				w.Header().Set("Access-Control-Max-Age", "86400")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
