package middleware

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wangn-tech/campus-hub/internal/httpx"
)

// AdminChecker reports whether the authenticated user holds the admin role.
// The concrete implementation lives in the service layer.
type AdminChecker interface {
	IsAdmin(ctx context.Context, userUUID string) (bool, error)
}

// RequireAdmin rejects requests from users without the admin role. It must run
// after Auth so the user UUID is available.
func RequireAdmin(checker AdminChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		userUUID := UserUUID(c)
		if userUUID == "" {
			httpx.Error(c, http.StatusUnauthorized, 101401, "authentication required")
			c.Abort()
			return
		}
		isAdmin, err := checker.IsAdmin(c.Request.Context(), userUUID)
		if err != nil {
			httpx.Error(c, http.StatusServiceUnavailable, 100503, "authorization unavailable")
			c.Abort()
			return
		}
		if !isAdmin {
			httpx.Error(c, http.StatusForbidden, 100403, "administrator role required")
			c.Abort()
			return
		}
		c.Next()
	}
}
