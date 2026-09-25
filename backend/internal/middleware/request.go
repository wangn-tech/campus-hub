package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func Request(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		startedAt := time.Now()
		traceID := c.GetHeader("X-Request-ID")
		if traceID == "" {
			traceID = uuid.NewString()
		}
		c.Set("trace_id", traceID)
		c.Header("X-Request-ID", traceID)
		c.Next()
		fields := []zap.Field{
			zap.String("method", c.Request.Method),
			zap.String("path", c.Request.URL.Path),
			zap.Int("status", c.Writer.Status()),
			zap.Int64("latency_ms", time.Since(startedAt).Milliseconds()),
			zap.String("trace_id", traceID),
			zap.String("client_ip", c.ClientIP()),
			zap.String("user_agent", c.Request.UserAgent()),
		}
		if userUUID, ok := c.Get(UserUUIDKey); ok {
			if value, ok := userUUID.(string); ok {
				fields = append(fields, zap.String("user_id", value))
			}
		}
		logger.Info("http request", fields...)
	}
}
