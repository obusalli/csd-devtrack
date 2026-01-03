package graphql

import (
	"context"
	"net/http"
	"sync"
)

// OperationType represents the type of GraphQL operation
type OperationType string

const (
	Query    OperationType = "query"
	Mutation OperationType = "mutation"
)

// HandlerFunc is the function signature for GraphQL operation handlers
type HandlerFunc func(ctx context.Context, w http.ResponseWriter, variables map[string]interface{})

// Operation represents a registered GraphQL operation
type Operation struct {
	Name        string
	Type        OperationType
	Description string
	Permission  string
	Handler     HandlerFunc
}

// Registry holds all registered GraphQL operations
type Registry struct {
	mu        sync.RWMutex
	queries   map[string]*Operation
	mutations map[string]*Operation
}

var globalRegistry = &Registry{
	queries:   make(map[string]*Operation),
	mutations: make(map[string]*Operation),
}

// RegisterQuery registers a query operation
func RegisterQuery(name string, description string, permission string, handler HandlerFunc) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()

	globalRegistry.queries[name] = &Operation{
		Name:        name,
		Type:        Query,
		Description: description,
		Permission:  permission,
		Handler:     handler,
	}
}

// RegisterMutation registers a mutation operation
func RegisterMutation(name string, description string, permission string, handler HandlerFunc) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()

	globalRegistry.mutations[name] = &Operation{
		Name:        name,
		Type:        Mutation,
		Description: description,
		Permission:  permission,
		Handler:     handler,
	}
}

// GetQuery returns a registered query by name
func GetQuery(name string) (*Operation, bool) {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	op, ok := globalRegistry.queries[name]
	return op, ok
}

// GetMutation returns a registered mutation by name
func GetMutation(name string) (*Operation, bool) {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	op, ok := globalRegistry.mutations[name]
	return op, ok
}

// GetAllQueries returns all registered queries
func GetAllQueries() map[string]*Operation {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	result := make(map[string]*Operation, len(globalRegistry.queries))
	for k, v := range globalRegistry.queries {
		result[k] = v
	}
	return result
}

// GetAllMutations returns all registered mutations
func GetAllMutations() map[string]*Operation {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	result := make(map[string]*Operation, len(globalRegistry.mutations))
	for k, v := range globalRegistry.mutations {
		result[k] = v
	}
	return result
}
