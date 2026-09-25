package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wangn-tech/campus-hub/internal/httpx"
)

// UserUUIDKey is the gin context key holding the authenticated user's UUID.
const UserUUIDKey = "user_uuid"

// Authenticator resolves a raw access token to a user UUID. The concrete
// implementation lives in the service layer; declaring the port here keeps the
// middleware free of any dependency on that package.
type Authenticator interface {
	Authenticate(ctx context.Context, raw string) (string, error)
}

// UserUUID returns the authenticated user's UUID stored by Auth, or an empty
// string when the context holds no value.
func UserUUID(c *gin.Context) string {
	value, _ := c.Get(UserUUIDKey)
	uuid, _ := value.(string)
	return uuid
}

// BearerToken extracts the raw token from the Authorization header.
func BearerToken(c *gin.Context) (string, bool) {
	parts := strings.SplitN(c.GetHeader("Authorization"), " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return "", false
	}
	return parts[1], true
}

// Auth rejects requests without a valid bearer access token and stores the
// authenticated user's UUID in the gin context for downstream handlers.
func Auth(authenticator Authenticator) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, ok := BearerToken(c)
		if !ok {
			httpx.Error(c, http.StatusUnauthorized, 101401, "authentication required")
			c.Abort()
			return
		}
		userUUID, err := authenticator.Authenticate(c.Request.Context(), raw)
		if err != nil {
			httpx.Error(c, http.StatusUnauthorized, 101401, "invalid access token")
			c.Abort()
			return
		}
		c.Set(UserUUIDKey, userUUID)
		c.Next()
	}
}

// OptionalAuth attaches the authenticated user's UUID when a valid bearer token
// is present and continues otherwise. It lets public endpoints personalize the
// response for their owner or an administrator without requiring a token.
func OptionalAuth(authenticator Authenticator) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, ok := BearerToken(c)
		if !ok {
			c.Next()
			return
		}
		userUUID, err := authenticator.Authenticate(c.Request.Context(), raw)
		if err != nil {
			c.Next()
			return
		}
		c.Set(UserUUIDKey, userUUID)
		c.Next()
	}
}
