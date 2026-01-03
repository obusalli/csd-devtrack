package terminal

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sync"
	"time"
)

// TerminalToken represents a token for terminal session access
// The token is opaque to the frontend - it never sees the actual session name or prefix
type TerminalToken struct {
	TokenID     string    `json:"tid"`         // Unique token ID
	SessionID   string    `json:"sid"`         // Claude session ID
	SessionType string    `json:"stype"`       // Type: claude, codex, database, shell
	UserID      string    `json:"uid"`         // User who requested the token
	ExpiresAt   time.Time `json:"exp"`         // Expiration time
	CreatedAt   time.Time `json:"iat"`         // Creation time
}

// TerminalTokenClaims is the validated result returned to csd-core
type TerminalTokenClaims struct {
	SessionName string `json:"sessionName"` // Full tmux session name with prefix
	Prefix      string `json:"prefix"`      // The prefix used
	SessionID   string `json:"sessionId"`   // Original session ID
	SessionType string `json:"sessionType"` // Type
	UserID      string `json:"userId"`      // User ID
	Valid       bool   `json:"valid"`       // Whether token is valid
	Error       string `json:"error,omitempty"`
}

var (
	// tokenStore stores active tokens (in production, use Redis or similar)
	tokenStore   = make(map[string]*TerminalToken)
	tokenStoreMu sync.RWMutex

	// tokenSecret is used to sign tokens (in production, load from config)
	tokenSecret []byte

	// Token expiration time
	tokenExpiration = 5 * time.Minute
)

func init() {
	// Generate a random secret on startup
	// In production, this should be loaded from config/environment
	tokenSecret = make([]byte, 32)
	rand.Read(tokenSecret)
}

// SetTokenSecret sets the secret used for signing tokens
// Call this during initialization with a secret from config
func SetTokenSecret(secret []byte) {
	tokenSecret = secret
}

// GenerateToken creates a new terminal token for a session
// The token is opaque - the frontend never sees the session name or prefix
func GenerateToken(sessionID, sessionType, userID string) (string, error) {
	if sessionID == "" {
		return "", errors.New("sessionId is required")
	}
	if sessionType == "" {
		sessionType = "claude"
	}

	// Generate unique token ID
	tokenIDBytes := make([]byte, 16)
	if _, err := rand.Read(tokenIDBytes); err != nil {
		return "", err
	}
	tokenID := base64.RawURLEncoding.EncodeToString(tokenIDBytes)

	token := &TerminalToken{
		TokenID:     tokenID,
		SessionID:   sessionID,
		SessionType: sessionType,
		UserID:      userID,
		ExpiresAt:   time.Now().Add(tokenExpiration),
		CreatedAt:   time.Now(),
	}

	// Store token
	tokenStoreMu.Lock()
	tokenStore[tokenID] = token
	tokenStoreMu.Unlock()

	// Create signed token string
	tokenData, err := json.Marshal(token)
	if err != nil {
		return "", err
	}

	// Sign the token
	h := hmac.New(sha256.New, tokenSecret)
	h.Write(tokenData)
	signature := h.Sum(nil)

	// Combine data and signature
	signedToken := base64.RawURLEncoding.EncodeToString(tokenData) + "." +
		base64.RawURLEncoding.EncodeToString(signature)

	return signedToken, nil
}

// ValidateToken validates a terminal token and returns the session info
// This is called by csd-core to get the actual session name with prefix
func ValidateToken(tokenString string) *TerminalTokenClaims {
	claims := &TerminalTokenClaims{Valid: false}

	// Split token into data and signature
	parts := splitToken(tokenString)
	if len(parts) != 2 {
		claims.Error = "invalid token format"
		return claims
	}

	// Decode data
	tokenData, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		claims.Error = "invalid token encoding"
		return claims
	}

	// Verify signature
	h := hmac.New(sha256.New, tokenSecret)
	h.Write(tokenData)
	expectedSig := h.Sum(nil)

	actualSig, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		claims.Error = "invalid signature encoding"
		return claims
	}

	if !hmac.Equal(expectedSig, actualSig) {
		claims.Error = "invalid signature"
		return claims
	}

	// Parse token
	var token TerminalToken
	if err := json.Unmarshal(tokenData, &token); err != nil {
		claims.Error = "invalid token data"
		return claims
	}

	// Check expiration
	if time.Now().After(token.ExpiresAt) {
		claims.Error = "token expired"
		// Clean up expired token
		tokenStoreMu.Lock()
		delete(tokenStore, token.TokenID)
		tokenStoreMu.Unlock()
		return claims
	}

	// Verify token exists in store (prevents replay after revocation)
	tokenStoreMu.RLock()
	storedToken, exists := tokenStore[token.TokenID]
	tokenStoreMu.RUnlock()

	if !exists {
		claims.Error = "token not found or revoked"
		return claims
	}

	// Verify token data matches
	if storedToken.SessionID != token.SessionID || storedToken.UserID != token.UserID {
		claims.Error = "token mismatch"
		return claims
	}

	// Build session name with correct prefix
	prefix := prefixForType(token.SessionType)
	shortID := token.SessionID
	if len(shortID) > 8 {
		shortID = shortID[:8]
	}
	sessionName := prefix + shortID

	// Return validated claims with session info
	claims.Valid = true
	claims.SessionName = sessionName
	claims.Prefix = prefix
	claims.SessionID = token.SessionID
	claims.SessionType = token.SessionType
	claims.UserID = token.UserID

	return claims
}

// RevokeToken revokes a terminal token
func RevokeToken(tokenString string) bool {
	parts := splitToken(tokenString)
	if len(parts) != 2 {
		return false
	}

	tokenData, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return false
	}

	var token TerminalToken
	if err := json.Unmarshal(tokenData, &token); err != nil {
		return false
	}

	tokenStoreMu.Lock()
	delete(tokenStore, token.TokenID)
	tokenStoreMu.Unlock()

	return true
}

// CleanupExpiredTokens removes expired tokens from the store
// Should be called periodically
func CleanupExpiredTokens() int {
	now := time.Now()
	count := 0

	tokenStoreMu.Lock()
	defer tokenStoreMu.Unlock()

	for id, token := range tokenStore {
		if now.After(token.ExpiresAt) {
			delete(tokenStore, id)
			count++
		}
	}

	return count
}

// splitToken splits a token string by the last dot
func splitToken(token string) []string {
	for i := len(token) - 1; i >= 0; i-- {
		if token[i] == '.' {
			return []string{token[:i], token[i+1:]}
		}
	}
	return nil
}
