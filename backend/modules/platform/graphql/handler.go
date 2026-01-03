package graphql

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	csdcore "csd-devtrack/backend/modules/platform/csd-core"
	"csd-devtrack/backend/modules/platform/middleware"
)

// Handler handles GraphQL requests
type Handler struct {
	csdCoreClient *csdcore.Client
}

// NewHandler creates a new GraphQL handler
func NewHandler(csdCoreClient *csdcore.Client) *Handler {
	return &Handler{
		csdCoreClient: csdCoreClient,
	}
}

// ServeHTTP handles GraphQL HTTP requests
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(NewErrorResponse("Method not allowed"))
		return
	}

	var req GraphQLRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(NewErrorResponse("Invalid request body"))
		return
	}

	// Parse the operation from the query
	opType, opName := parseOperation(req.Query, req.OperationName)

	// Try to handle locally first
	if handled := h.handleLocal(w, r, opType, opName, req.Variables); handled {
		return
	}

	// Forward to csd-core if not handled locally
	h.forwardToCSDCore(w, r, req)
}

// handleLocal tries to handle the operation locally
func (h *Handler) handleLocal(w http.ResponseWriter, r *http.Request, opType, opName string, variables map[string]interface{}) bool {
	var op *Operation
	var ok bool

	switch opType {
	case "query":
		op, ok = GetQuery(opName)
	case "mutation":
		op, ok = GetMutation(opName)
	default:
		return false
	}

	if !ok {
		return false
	}

	// Check permission if required
	if op.Permission != "" {
		token, hasToken := middleware.GetTokenFromContext(r.Context())
		if !hasToken {
			json.NewEncoder(w).Encode(NewErrorResponse("Unauthorized"))
			return true
		}

		if h.csdCoreClient != nil {
			// Check for system.admin first (grants all permissions)
			hasAdmin, _ := h.csdCoreClient.CheckPermission(r.Context(), token, "system.admin")

			if !hasAdmin {
				// Check for specific permission
				hasPermission, err := h.csdCoreClient.CheckPermission(r.Context(), token, op.Permission)
				if err != nil || !hasPermission {
					json.NewEncoder(w).Encode(NewErrorResponse("Permission denied"))
					return true
				}
			}
		}
	}

	// Execute handler
	op.Handler(r.Context(), w, variables)
	return true
}

// forwardToCSDCore forwards the request to csd-core
func (h *Handler) forwardToCSDCore(w http.ResponseWriter, r *http.Request, req GraphQLRequest) {
	if h.csdCoreClient == nil {
		json.NewEncoder(w).Encode(NewErrorResponse("Operation not found and csd-core not configured"))
		return
	}

	token, _ := middleware.GetTokenFromContext(r.Context())

	// Extract operation name if not provided in request
	operationName := req.OperationName
	if operationName == "" {
		operationName = extractOperationNameFromQuery(req.Query)
	}

	resp, err := h.csdCoreClient.ExecuteWithName(r.Context(), token, operationName, req.Query, req.Variables)
	if err != nil {
		json.NewEncoder(w).Encode(NewErrorResponse(err.Error()))
		return
	}

	// Forward the response as-is
	result := map[string]interface{}{}
	if err := json.Unmarshal(resp.Data, &result); err != nil {
		json.NewEncoder(w).Encode(GraphQLResponse{Data: resp.Data})
		return
	}
	json.NewEncoder(w).Encode(GraphQLResponse{Data: result})
}

// extractOperationNameFromQuery extracts the operation name from a GraphQL query
func extractOperationNameFromQuery(query string) string {
	query = strings.TrimSpace(query)
	pattern := regexp.MustCompile(`^(query|mutation|subscription)\s+(\w+)`)
	matches := pattern.FindStringSubmatch(query)
	if len(matches) > 2 {
		return matches[2]
	}
	return ""
}

// parseOperation extracts operation type and name from a GraphQL query
func parseOperation(query string, operationName string) (opType string, opName string) {
	query = strings.TrimSpace(query)

	typePattern := regexp.MustCompile(`^(query|mutation)\s*`)
	matches := typePattern.FindStringSubmatch(query)
	if len(matches) > 1 {
		opType = matches[1]
	} else {
		opType = "query"
	}

	// Try to find the first field name
	fieldPattern := regexp.MustCompile(`\{\s*(\w+)`)
	fieldMatches := fieldPattern.FindStringSubmatch(query)
	if len(fieldMatches) > 1 {
		opName = fieldMatches[1]
	}

	if operationName != "" && opName == "" {
		opName = operationName
	}

	return opType, opName
}
