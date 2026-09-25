package middleware

import (
	"net/http"
	"runtime/debug"

	"github.com/gin-gonic/gin"
	"github.com/wangn-tech/campus-hub/internal/httpx"
	"go.uber.org/zap"
)

func Recovery(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("panic recovered", zap.Any("panic", recovered), zap.ByteString("stack", debug.Stack()), zap.String("trace_id", httpx.TraceID(c)))
				if !c.Writer.Written() {
					httpx.Error(c, http.StatusInternalServerError, 100500, "internal server error")
				}
				c.Abort()
			}
		}()
		c.Next()
	}
}
