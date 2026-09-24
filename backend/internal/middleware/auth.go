package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/wangn-tech/campus-hub/internal/httpx"
	"github.com/wangn-tech/campus-hub/internal/service"
)

const UserUUIDKey = "user_uuid"

func Auth(authService *service.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			httpx.Error(c, http.StatusUnauthorized, 101401, "authentication required")
			c.Abort()
			return
		}
		userUUID, err := authService.ParseAccessToken(parts[1])
		if err != nil {
			httpx.Error(c, http.StatusUnauthorized, 101401, "invalid access token")
			c.Abort()
			return
		}
		c.Set(UserUUIDKey, userUUID)
		c.Next()
	}
}
