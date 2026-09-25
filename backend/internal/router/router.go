package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wangn-tech/campus-hub/internal/handler"
	"github.com/wangn-tech/campus-hub/internal/health"
	"github.com/wangn-tech/campus-hub/internal/httpx"
	"github.com/wangn-tech/campus-hub/internal/middleware"
	"go.uber.org/zap"
)

type Dependencies struct {
	AuthHandler    *handler.AuthHandler
	Readiness      *health.Service
	Logger         *zap.Logger
	AllowedOrigins []string
}

func New(dependencies Dependencies) *gin.Engine {
	r := gin.New()
	r.Use(middleware.Request(dependencies.Logger), middleware.Recovery(dependencies.Logger), middleware.CORS(dependencies.AllowedOrigins))
	r.GET("/health", func(c *gin.Context) { httpx.Success(c, gin.H{"status": "ok"}) })
	r.GET("/ready", func(c *gin.Context) {
		status := dependencies.Readiness.Check(c.Request.Context())
		for name, err := range status.Errors {
			dependencies.Logger.Warn("dependency not ready", zap.String("dependency", name), zap.Error(err), zap.String("trace_id", httpx.TraceID(c)))
		}
		data := gin.H{"status": "not_ready", "dependencies": status.Dependencies}
		if status.Ready {
			data["status"] = "ready"
			httpx.Success(c, data)
			return
		}
		c.JSON(http.StatusServiceUnavailable, httpx.Response{Code: 100503, Message: "service not ready", Data: data, TraceID: httpx.TraceID(c)})
	})
	if dependencies.AuthHandler == nil {
		return r
	}
	auth := r.Group("/api/v1/auth")
	auth.POST("/register", dependencies.AuthHandler.Register)
	auth.POST("/login", dependencies.AuthHandler.Login)
	return r
}
