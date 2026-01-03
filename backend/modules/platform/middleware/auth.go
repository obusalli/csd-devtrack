package middleware

import (
	"context"
	"net/http"
	"strings"

	"csd-devtrack/backend/modules/platform/config"
	csdcore "csd-devtrack/backend/modules/platform/csd-core"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type contextKey string

const (
	UserContextKey   contextKey = "user"
	TokenContextKey  contextKey = "token"
	TenantContextKey contextKey = "tenantId"
)

// UserClaims represents JWT claims
type UserClaims struct {
	UserID   uuid.UUID `json:"userId"`
	Email    string    `json:"email"`
	TenantID uuid.UUID `json:"tenantId"`
	jwt.RegisteredClaims
}

// AuthMiddleware validates JWT tokens via csd-core
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract token from Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			next.ServeHTTP(w, r)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			next.ServeHTTP(w, r)
			return
		}
		tokenString := parts[1]

		// Parse JWT locally (faster than calling csd-core for every request)
		cfg := config.GetConfig()
		if cfg.JWT.Secret != "" {
			claims := &UserClaims{}
			token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
				return []byte(cfg.JWT.Secret), nil
			})

			if err == nil && token.Valid {
				ctx := r.Context()
				ctx = context.WithValue(ctx, UserContextKey, claims)
				ctx = context.WithValue(ctx, TokenContextKey, tokenString)
				ctx = context.WithValue(ctx, TenantContextKey, claims.TenantID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		// Fallback: Validate via csd-core if local validation fails
		client := csdcore.GetClient()
		if client != nil {
			userInfo, err := client.ValidateToken(r.Context(), tokenString)
			if err == nil && userInfo != nil {
				claims := &UserClaims{
					UserID:   userInfo.ID,
					Email:    userInfo.Email,
					TenantID: userInfo.TenantID,
				}
				ctx := r.Context()
				ctx = context.WithValue(ctx, UserContextKey, claims)
				ctx = context.WithValue(ctx, TokenContextKey, tokenString)
				ctx = context.WithValue(ctx, TenantContextKey, userInfo.TenantID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		next.ServeHTTP(w, r)
	})
}

// GetUserFromContext retrieves user claims from context
func GetUserFromContext(ctx context.Context) (*UserClaims, bool) {
	claims, ok := ctx.Value(UserContextKey).(*UserClaims)
	return claims, ok
}

// GetTokenFromContext retrieves the JWT token from context
func GetTokenFromContext(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(TokenContextKey).(string)
	return token, ok
}

// GetTenantIDFromContext retrieves tenant ID from context
func GetTenantIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	tenantID, ok := ctx.Value(TenantContextKey).(uuid.UUID)
	return tenantID, ok
}

// GetUserIDFromContext retrieves user ID from context
func GetUserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	claims, ok := ctx.Value(UserContextKey).(*UserClaims)
	if !ok || claims == nil {
		return uuid.Nil, false
	}
	return claims.UserID, true
}

// RequireAuth is a middleware that requires authentication
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, ok := GetUserFromContext(r.Context())
		if !ok {
			http.Error(w, `{"errors":[{"message":"Unauthorized"}]}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
