package sessions

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"csd-devtrack/backend/modules/platform/graphql"
	"csd-devtrack/cli/modules/platform/claude"
)

var (
	claudeService *claude.Service
	once          sync.Once
)

func init() {
	// Register queries
	graphql.RegisterQuery("sessions", "List all Claude sessions", "", ListSessions)
	graphql.RegisterQuery("sessionsCount", "Count Claude sessions", "", CountSessions)
	graphql.RegisterQuery("session", "Get a Claude session by ID", "", GetSession)
}

// getClaudeService returns the Claude service singleton
// Uses the CLI's Claude service directly
func getClaudeService() *claude.Service {
	once.Do(func() {
		// Initialize with default data dir
		// The CLI service reads from ~/.claude/projects/
		homeDir, _ := os.UserHomeDir()
		dataDir := filepath.Join(homeDir, ".csd-devtrack")
		claudeService = claude.NewService(dataDir)
	})
	return claudeService
}

// ListSessions handles the sessions query
func ListSessions(ctx context.Context, w http.ResponseWriter, variables map[string]interface{}) {
	service := getClaudeService()
	service.RefreshSessions()

	limit, offset := graphql.ParsePagination(variables)

	// Get all sessions (empty string = no project filter)
	sessions := service.ListSessions("")

	// Sort by last active time (newest first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].LastActiveAt.After(sessions[j].LastActiveAt)
	})

	// Apply pagination
	total := len(sessions)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	paginatedSessions := sessions[offset:end]

	graphql.SendDataMultiple(w, map[string]interface{}{
		"sessions":      paginatedSessions,
		"sessionsCount": total,
	})
}

// CountSessions handles the sessionsCount query
func CountSessions(ctx context.Context, w http.ResponseWriter, variables map[string]interface{}) {
	service := getClaudeService()
	service.RefreshSessions()

	sessions := service.ListSessions("")
	graphql.SendData(w, "sessionsCount", len(sessions))
}

// GetSession handles the session query
func GetSession(ctx context.Context, w http.ResponseWriter, variables map[string]interface{}) {
	id, ok := graphql.ParseString(variables, "id")
	if !ok || id == "" {
		graphql.SendError(w, nil, "id is required")
		return
	}

	service := getClaudeService()
	service.RefreshSessions()

	session, err := service.GetSession(id)
	if err != nil {
		graphql.SendError(w, err, "session")
		return
	}

	graphql.SendData(w, "session", session)
}
