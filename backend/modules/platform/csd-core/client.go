package csdcore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"csd-devtrack/backend/modules/platform/config"

	"github.com/google/uuid"
)

// Client is a GraphQL client for csd-core
type Client struct {
	httpClient *http.Client
	baseURL    string
	endpoint   string
}

// GraphQLRequest represents a GraphQL request
type GraphQLRequest struct {
	Query         string                 `json:"query"`
	OperationName string                 `json:"operationName,omitempty"`
	Variables     map[string]interface{} `json:"variables,omitempty"`
}

// GraphQLResponse represents a GraphQL response
type GraphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []GraphQLError  `json:"errors,omitempty"`
}

// GraphQLError represents a GraphQL error
type GraphQLError struct {
	Message string `json:"message"`
}

// UserInfo represents user information from csd-core
type UserInfo struct {
	ID            uuid.UUID `json:"id"`
	Email         string    `json:"email"`
	FirstName     string    `json:"firstName"`
	LastName      string    `json:"lastName"`
	TenantID      uuid.UUID `json:"tenantId"`
	Roles         []string  `json:"roles"`
	Permissions   []string  `json:"permissions"`
	IsActive      bool      `json:"isActive"`
	EmailVerified bool      `json:"emailVerified"`
}

// ServiceRegistration represents service registration data
type ServiceRegistration struct {
	Name            string            `json:"name"`
	Slug            string            `json:"slug"`
	Version         string            `json:"version"`
	BaseURL         string            `json:"baseUrl"`
	CallbackURL     string            `json:"callbackUrl"`
	Description     string            `json:"description"`
	FrontendURL     string            `json:"frontendUrl"`
	RemoteEntryPath string            `json:"remoteEntryPath"`
	RoutePath       string            `json:"routePath"`
	ExposedModules  map[string]string `json:"exposedModules"`
}

var globalClient *Client

// NewClient creates a new csd-core client
func NewClient(cfg *config.CSDCoreConfig) *Client {
	client := &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL:  cfg.URL,
		endpoint: cfg.GraphQLEndpoint,
	}
	globalClient = client
	return client
}

// GetClient returns the global client
func GetClient() *Client {
	return globalClient
}

// extractOperationName extracts the operation name from a GraphQL query string
func extractOperationName(query string) string {
	keywords := []string{"query ", "mutation ", "subscription "}
	for _, kw := range keywords {
		idx := strings.Index(query, kw)
		if idx != -1 {
			start := idx + len(kw)
			for start < len(query) && (query[start] == ' ' || query[start] == '\t' || query[start] == '\n') {
				start++
			}
			end := start
			for end < len(query) && (query[end] >= 'a' && query[end] <= 'z' ||
				query[end] >= 'A' && query[end] <= 'Z' ||
				query[end] >= '0' && query[end] <= '9' ||
				query[end] == '_') {
				end++
			}
			if end > start {
				return query[start:end]
			}
		}
	}
	return ""
}

// Execute executes a GraphQL query/mutation
func (c *Client) Execute(ctx context.Context, token string, query string, variables map[string]interface{}) (*GraphQLResponse, error) {
	operationName := extractOperationName(query)
	return c.ExecuteWithName(ctx, token, operationName, query, variables)
}

// ExecuteWithName executes a GraphQL query/mutation with an explicit operation name
func (c *Client) ExecuteWithName(ctx context.Context, token string, operationName string, query string, variables map[string]interface{}) (*GraphQLResponse, error) {
	reqBody := GraphQLRequest{
		Query:         query,
		OperationName: operationName,
		Variables:     variables,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+c.endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var gqlResp GraphQLResponse
	if err := json.Unmarshal(body, &gqlResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &gqlResp, nil
}

// ValidateToken validates a JWT token with csd-core
func (c *Client) ValidateToken(ctx context.Context, token string) (*UserInfo, error) {
	query := `query ValidateToken {
		me {
			id
			email
			firstName
			lastName
			tenantId
			isActive
			emailVerified
		}
	}`

	resp, err := c.Execute(ctx, token, query, nil)
	if err != nil {
		return nil, err
	}

	if len(resp.Errors) > 0 {
		return nil, fmt.Errorf("validation failed: %s", resp.Errors[0].Message)
	}

	var data struct {
		Me *UserInfo `json:"me"`
	}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return nil, fmt.Errorf("failed to parse user info: %w", err)
	}

	return data.Me, nil
}

// CheckPermission checks if the user has a specific permission
func (c *Client) CheckPermission(ctx context.Context, token string, permission string) (bool, error) {
	query := `query CheckPermission($permission: String!) {
		checkPermission(permission: $permission)
	}`

	variables := map[string]interface{}{
		"permission": permission,
	}

	resp, err := c.Execute(ctx, token, query, variables)
	if err != nil {
		return false, err
	}

	if len(resp.Errors) > 0 {
		return false, fmt.Errorf("permission check failed: %s", resp.Errors[0].Message)
	}

	var data struct {
		CheckPermission bool `json:"checkPermission"`
	}
	if err := json.Unmarshal(resp.Data, &data); err != nil {
		return false, fmt.Errorf("failed to parse response: %w", err)
	}

	return data.CheckPermission, nil
}

// RegisterService registers this service with csd-core
func (c *Client) RegisterService(ctx context.Context, serviceToken string, reg *ServiceRegistration) error {
	query := `mutation RegisterService($input: RegisterServiceInput!) {
		registerService(input: $input) {
			id
			name
			slug
		}
	}`

	exposedModulesJSON, _ := json.Marshal(reg.ExposedModules)

	variables := map[string]interface{}{
		"input": map[string]interface{}{
			"name":            reg.Name,
			"slug":            reg.Slug,
			"version":         reg.Version,
			"baseUrl":         reg.BaseURL,
			"callbackUrl":     reg.CallbackURL,
			"description":     reg.Description,
			"frontendUrl":     reg.FrontendURL,
			"remoteEntryPath": reg.RemoteEntryPath,
			"routePath":       reg.RoutePath,
			"exposedModules":  string(exposedModulesJSON),
		},
	}

	// Use service token for registration
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+c.endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Service-Token", serviceToken)

	resp, err := c.Execute(ctx, "", query, variables)
	if err != nil {
		return err
	}

	if len(resp.Errors) > 0 {
		return fmt.Errorf("registration failed: %s", resp.Errors[0].Message)
	}

	return nil
}
