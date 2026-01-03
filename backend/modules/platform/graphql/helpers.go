package graphql

import (
	"context"
	"encoding/json"
	"net/http"

	"csd-devtrack/backend/modules/platform/middleware"

	"github.com/google/uuid"
)

// SendData sends a successful GraphQL response with a single data field
func SendData(w http.ResponseWriter, key string, data interface{}) {
	json.NewEncoder(w).Encode(GraphQLResponse{
		Data: map[string]interface{}{key: data},
	})
}

// SendDataMultiple sends a successful GraphQL response with multiple data fields
func SendDataMultiple(w http.ResponseWriter, data map[string]interface{}) {
	json.NewEncoder(w).Encode(GraphQLResponse{
		Data: data,
	})
}

// SendError sends an error GraphQL response
func SendError(w http.ResponseWriter, err error, operationType string) {
	json.NewEncoder(w).Encode(GraphQLResponse{
		Errors: []GraphQLError{{Message: SanitizeError(err, operationType)}},
	})
}

// RequireAuth checks if the request is authenticated and returns the tenant ID
// Returns (tenantID, ok). If not ok, an error response has already been sent.
func RequireAuth(ctx context.Context, w http.ResponseWriter) (uuid.UUID, bool) {
	claims, ok := middleware.GetUserFromContext(ctx)
	if !ok || claims == nil {
		json.NewEncoder(w).Encode(NewErrorResponse("Unauthorized"))
		return uuid.Nil, false
	}
	return claims.TenantID, true
}

// ParseUUID parses a UUID from variables
// Returns (uuid, ok). If not ok, an error response has already been sent.
func ParseUUID(w http.ResponseWriter, variables map[string]interface{}, key string) (uuid.UUID, bool) {
	idVal, ok := variables[key]
	if !ok {
		json.NewEncoder(w).Encode(NewErrorResponse(key + " is required"))
		return uuid.Nil, false
	}

	idStr, ok := idVal.(string)
	if !ok {
		json.NewEncoder(w).Encode(NewErrorResponse(key + " must be a string"))
		return uuid.Nil, false
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		json.NewEncoder(w).Encode(NewErrorResponse("invalid " + key))
		return uuid.Nil, false
	}

	return id, true
}

// ParseUUIDOptional parses an optional UUID from variables
func ParseUUIDOptional(w http.ResponseWriter, variables map[string]interface{}, key string) (*uuid.UUID, bool) {
	idVal, ok := variables[key]
	if !ok || idVal == nil {
		return nil, true
	}

	idStr, ok := idVal.(string)
	if !ok {
		json.NewEncoder(w).Encode(NewErrorResponse(key + " must be a string"))
		return nil, false
	}

	if idStr == "" {
		return nil, true
	}

	id, err := uuid.Parse(idStr)
	if err != nil {
		json.NewEncoder(w).Encode(NewErrorResponse("invalid " + key))
		return nil, false
	}

	return &id, true
}

// ParseString parses a string from variables
func ParseString(variables map[string]interface{}, key string) (string, bool) {
	val, ok := variables[key]
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

// ParseInt parses an int from variables (JSON numbers come as float64)
func ParseInt(variables map[string]interface{}, key string) (int, bool) {
	val, ok := variables[key]
	if !ok {
		return 0, false
	}
	switch v := val.(type) {
	case float64:
		return int(v), true
	case int:
		return v, true
	default:
		return 0, false
	}
}

// ParseBool parses a bool from variables
func ParseBool(variables map[string]interface{}, key string) (bool, bool) {
	val, ok := variables[key]
	if !ok {
		return false, false
	}
	b, ok := val.(bool)
	return b, ok
}

// ParsePagination extracts limit and offset from variables with defaults
func ParsePagination(variables map[string]interface{}) (limit int, offset int) {
	limit = 50 // Default limit
	offset = 0

	if l, ok := ParseInt(variables, "limit"); ok && l > 0 {
		limit = l
		if limit > 1000 {
			limit = 1000 // Max limit
		}
	}

	if o, ok := ParseInt(variables, "offset"); ok && o >= 0 {
		offset = o
	}

	return limit, offset
}
