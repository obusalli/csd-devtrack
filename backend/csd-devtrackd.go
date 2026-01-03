// Package main - CSD DevTrack Backend Server
//
// This is the main entry point for the csd-devtrack backend service.
// It provides a GraphQL API for managing development projects, Claude sessions,
// and development workflows.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"csd-devtrack/backend/modules/platform/config"
	csdcore "csd-devtrack/backend/modules/platform/csd-core"
	"csd-devtrack/backend/modules/platform/logger"
	"csd-devtrack/backend/modules/platform/server"

	// Import modules to register their GraphQL operations
	_ "csd-devtrack/backend/modules/devtrack/projects"
	_ "csd-devtrack/backend/modules/devtrack/sessions"
	_ "csd-devtrack/backend/modules/devtrack/terminal"
)

var Version = "1.0.0"

func main() {
	configPath := flag.String("config", "", "Path to configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger
	logLevel := logger.INFO
	if cfg.Logging.Level != "" {
		logLevel = logger.ParseLevel(cfg.Logging.Level)
	}
	outputs := []io.Writer{os.Stdout}
	appLogger := logger.NewLogger(logLevel, outputs)
	logger.SetGlobalLogger(appLogger)

	logger.Info("Configuration loaded (log level: %s)", cfg.Logging.Level)

	// Register with csd-core
	if cfg.CSDCore.ServiceToken != "" {
		log.Printf("Registering service with csd-core at %s%s...", cfg.CSDCore.URL, cfg.CSDCore.GraphQLEndpoint)
		if err := registerWithCore(cfg); err != nil {
			log.Printf("Warning: Failed to register with csd-core: %v", err)
		} else {
			log.Printf("Successfully registered as 'csd-devtrack' with csd-core")
		}
	} else {
		log.Printf("Warning: No service-token configured, skipping csd-core registration")
	}

	// Create and start server
	srv, err := server.NewServer(cfg)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	log.Printf("CSD-DevTrack server starting...")
	log.Printf("  Health:   http://%s:%s/devtrack/api/latest/health", cfg.Server.Host, cfg.Server.Port)
	log.Printf("  GraphQL:  http://%s:%s/devtrack/api/latest/query", cfg.Server.Host, cfg.Server.Port)
	log.Printf("  CSD-Core: %s", cfg.CSDCore.URL)

	if err := srv.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

func registerWithCore(cfg *config.Config) error {
	client := csdcore.NewClient(&cfg.CSDCore)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	serviceURL := fmt.Sprintf("http://%s:%s", cfg.Server.Host, cfg.Server.Port)
	if cfg.Server.Host == "0.0.0.0" {
		serviceURL = fmt.Sprintf("http://localhost:%s", cfg.Server.Port)
	}

	reg := &csdcore.ServiceRegistration{
		Name:        "CSD DevTrack",
		Slug:        "csd-devtrack",
		Version:     Version,
		BaseURL:     serviceURL,
		CallbackURL: serviceURL + "/devtrack/api/latest/query",
		Description: "Development project and Claude session management",
		// Frontend integration (Module Federation)
		FrontendURL:     cfg.Frontend.URL,
		RemoteEntryPath: cfg.Frontend.RemoteEntryPath,
		RoutePath:       cfg.Frontend.RoutePath,
		ExposedModules: map[string]string{
			"./Routes":       "./src/Routes.tsx",
			"./Translations": "./src/translations/index.ts",
			"./AppInfo":      "./src/appInfo.ts",
		},
	}

	return client.RegisterService(ctx, cfg.CSDCore.ServiceToken, reg)
}
