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
	AuthHandler         *handler.AuthHandler
	UserHandler         *handler.UserHandler
	FileHandler         *handler.FileHandler
	VerificationHandler *handler.VerificationHandler
	ActivityHandler     *handler.ActivityHandler
	RegistrationHandler *handler.RegistrationHandler
	Authenticator       middleware.Authenticator
	AdminChecker        middleware.AdminChecker
	Readiness           *health.Service
	Logger              *zap.Logger
	AllowedOrigins      []string
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
	auth.POST("/refresh", dependencies.AuthHandler.Refresh)
	auth.POST("/password/reset", dependencies.AuthHandler.ResetPassword)
	r.POST("/api/v1/email-codes", dependencies.AuthHandler.EmailCode)
	if dependencies.ActivityHandler != nil {
		r.GET("/api/v1/categories", dependencies.ActivityHandler.Categories)
		r.GET("/api/v1/tags", dependencies.ActivityHandler.Tags)
		r.GET("/api/v1/activities/search", dependencies.ActivityHandler.Search)
		r.GET("/api/v1/activities", dependencies.ActivityHandler.List)
		detail := r.Group("/api/v1")
		if dependencies.Authenticator != nil {
			detail.Use(middleware.OptionalAuth(dependencies.Authenticator))
		}
		detail.GET("/activities/:id", dependencies.ActivityHandler.Detail)
	}
	if dependencies.Authenticator == nil || dependencies.UserHandler == nil || dependencies.FileHandler == nil || dependencies.VerificationHandler == nil || dependencies.ActivityHandler == nil || dependencies.RegistrationHandler == nil {
		return r
	}
	protected := r.Group("/api/v1")
	protected.Use(middleware.Auth(dependencies.Authenticator))
	protected.POST("/auth/logout", dependencies.AuthHandler.Logout)
	protected.POST("/auth/logoff", dependencies.AuthHandler.Logoff)
	protected.PUT("/users/me/password", dependencies.AuthHandler.ChangePassword)
	protected.GET("/users/me", dependencies.UserHandler.Me)
	protected.PUT("/users/me", dependencies.UserHandler.Update)
	protected.PUT("/users/me/interests", dependencies.UserHandler.Interests)
	protected.POST("/files/images", dependencies.FileHandler.Upload)
	protected.GET("/files/:id", dependencies.FileHandler.Get)
	protected.DELETE("/files/:id", dependencies.FileHandler.Delete)
	protected.GET("/student-verifications/current", dependencies.VerificationHandler.Current)
	protected.POST("/student-verifications", dependencies.VerificationHandler.Submit)
	protected.POST("/student-verifications/:id/confirm", dependencies.VerificationHandler.Confirm)
	protected.POST("/student-verifications/:id/cancel", dependencies.VerificationHandler.Cancel)
	protected.GET("/users/me/activities/created", dependencies.ActivityHandler.MyCreated)
	protected.POST("/activities", dependencies.ActivityHandler.Create)
	protected.PUT("/activities/:id", dependencies.ActivityHandler.Update)
	protected.POST("/activities/:id/submit", dependencies.ActivityHandler.Submit)
	protected.POST("/activities/:id/cancel", dependencies.ActivityHandler.Cancel)
	protected.POST("/activities/:id/registrations", dependencies.RegistrationHandler.Register)
	protected.GET("/activities/:id/registrations", dependencies.RegistrationHandler.ListByActivity)
	protected.GET("/registrations/:id", dependencies.RegistrationHandler.Detail)
	protected.DELETE("/registrations/:id", dependencies.RegistrationHandler.Cancel)
	protected.POST("/registrations/:id/approve", dependencies.RegistrationHandler.Approve)
	protected.POST("/registrations/:id/reject", dependencies.RegistrationHandler.Reject)
	protected.GET("/users/me/activities/registered", dependencies.RegistrationHandler.MyRegistrations)
	protected.GET("/tickets", dependencies.RegistrationHandler.MyTickets)
	protected.GET("/tickets/:id", dependencies.RegistrationHandler.Ticket)
	if dependencies.AdminChecker != nil {
		admin := protected.Group("")
		admin.Use(middleware.RequireAdmin(dependencies.AdminChecker))
		admin.POST("/activities/:id/approve", dependencies.ActivityHandler.Approve)
		admin.POST("/activities/:id/reject", dependencies.ActivityHandler.Reject)
		admin.POST("/activities/:id/send-back", dependencies.ActivityHandler.SendBack)
	}
	return r
}
