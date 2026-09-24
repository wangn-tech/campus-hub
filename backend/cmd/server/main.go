package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wangn-tech/campus-hub/internal/config"
	"github.com/wangn-tech/campus-hub/internal/handler"
	"github.com/wangn-tech/campus-hub/internal/middleware"
	"github.com/wangn-tech/campus-hub/internal/platform/database"
	redispkg "github.com/wangn-tech/campus-hub/internal/platform/redis"
	"github.com/wangn-tech/campus-hub/internal/repository"
	"github.com/wangn-tech/campus-hub/internal/router"
	"github.com/wangn-tech/campus-hub/internal/service"
	"go.uber.org/zap"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()

	configPath := os.Getenv("CAMPUSHUB_CONFIG_FILE")
	if configPath == "" {
		configPath = "configs/config.dev.yaml"
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		logger.Fatal("load config", zap.Error(err))
	}
	db, err := database.Open(cfg.MySQL)
	if err != nil {
		logger.Fatal("open mysql", zap.Error(err))
	}
	redisClient, err := redispkg.Open(cfg.Redis)
	if err != nil {
		logger.Fatal("open redis", zap.Error(err))
	}
	defer redisClient.Close()

	gin.SetMode(gin.ReleaseMode)
	userRepository := repository.NewUserRepository(db)
	authService := service.NewAuthService(userRepository, cfg.JWT)
	authHandler := handler.NewAuthHandler(authService)
	engine := router.NewWithAuth(authHandler)
	engine.Use(gin.Recovery(), middleware.Request(logger), middleware.CORS(cfg.HTTP.AllowedOrigins))
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port),
		Handler:      engine,
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
		IdleTimeout:  cfg.HTTP.IdleTimeout,
	}
	startedAt := time.Now()

	go func() {
		logger.Info("server started", zap.String("addr", server.Addr))
		if serveErr := server.ListenAndServe(); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			logger.Fatal("server stopped unexpectedly", zap.Error(serveErr))
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("server shutdown", zap.Error(err))
	}
	logger.Info("server stopped", zap.Duration("uptime", time.Since(startedAt)))
}
