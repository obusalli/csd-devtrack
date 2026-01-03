package graphql

// GraphQLRequest represents an incoming GraphQL request
type GraphQLRequest struct {
	Query         string                 `json:"query"`
	OperationName string                 `json:"operationName"`
	Variables     map[string]interface{} `json:"variables"`
}

// GraphQLResponse represents a GraphQL response
type GraphQLResponse struct {
	Data   interface{}    `json:"data,omitempty"`
	Errors []GraphQLError `json:"errors,omitempty"`
}

// GraphQLError represents a GraphQL error
type GraphQLError struct {
	Message    string                 `json:"message"`
	Extensions map[string]interface{} `json:"extensions,omitempty"`
}

// NewErrorResponse creates an error response
func NewErrorResponse(message string) GraphQLResponse {
	return GraphQLResponse{
		Errors: []GraphQLError{{Message: message}},
	}
}

// NewDataResponse creates a data response
func NewDataResponse(data interface{}) GraphQLResponse {
	return GraphQLResponse{
		Data: data,
	}
}

// NewErrorResponseWithCode creates an error response with an error code
func NewErrorResponseWithCode(code, message string) GraphQLResponse {
	return GraphQLResponse{
		Errors: []GraphQLError{{
			Message: message,
			Extensions: map[string]interface{}{
				"code": code,
			},
		}},
	}
}

// SanitizeError converts internal errors to user-friendly messages
func SanitizeError(err error, operationType string) string {
	if err == nil {
		return "unknown error"
	}

	errStr := err.Error()

	switch {
	case strContains(errStr, "not found") || strContains(errStr, "no rows"):
		return operationType + " not found"
	case strContains(errStr, "already exists") || strContains(errStr, "duplicate"):
		return operationType + " already exists"
	case strContains(errStr, "permission") || strContains(errStr, "unauthorized"):
		return "permission denied"
	case strContains(errStr, "invalid"):
		return "invalid " + operationType
	case strContains(errStr, "connection refused") || strContains(errStr, "timeout"):
		return "service temporarily unavailable"
	default:
		return "failed to process " + operationType
	}
}

func strContains(s, substr string) bool {
	sLower := toLower(s)
	substrLower := toLower(substr)
	for i := 0; i <= len(sLower)-len(substrLower); i++ {
		if sLower[i:i+len(substrLower)] == substrLower {
			return true
		}
	}
	return false
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}
