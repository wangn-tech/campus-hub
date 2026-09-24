package router

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/wangn-tech/campus-hub/internal/handler"
	"github.com/wangn-tech/campus-hub/internal/httpx"
)

func New() *gin.Engine {
	r := gin.New()
	r.GET("/health", func(c *gin.Context) { httpx.Success(c, gin.H{"status": "ok"}) })
	r.GET("/ready", func(c *gin.Context) { httpx.Success(c, gin.H{"status": "ready"}) })
	r.GET("/api/v1/openapi/status", func(c *gin.Context) {
		c.JSON(http.StatusNotImplemented, httpx.Response{Code: 100501, Message: "openapi generation pending", Data: nil, TraceID: httpx.TraceID(c)})
	})
	return r
}

func NewWithAuth(authHandler *handler.AuthHandler) *gin.Engine {
	r := New()
	auth := r.Group("/api/v1/auth")
	auth.POST("/register", authHandler.Register)
	auth.POST("/login", authHandler.Login)
	return r
}
