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
	"github.com/robfig/cron/v3"
	"github.com/wangn-tech/campus-hub/internal/config"
	"github.com/wangn-tech/campus-hub/internal/handler"
	"github.com/wangn-tech/campus-hub/internal/health"
	"github.com/wangn-tech/campus-hub/internal/platform/database"
	espkg "github.com/wangn-tech/campus-hub/internal/platform/elasticsearch"
	kafkapkg "github.com/wangn-tech/campus-hub/internal/platform/kafka"
	"github.com/wangn-tech/campus-hub/internal/platform/logging"
	mailpkg "github.com/wangn-tech/campus-hub/internal/platform/mail"
	redispkg "github.com/wangn-tech/campus-hub/internal/platform/redis"
	storagepkg "github.com/wangn-tech/campus-hub/internal/platform/storage"
	"github.com/wangn-tech/campus-hub/internal/repository"
	"github.com/wangn-tech/campus-hub/internal/router"
	"github.com/wangn-tech/campus-hub/internal/service"
	"github.com/wangn-tech/campus-hub/internal/token"
	"go.uber.org/zap"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "server:", err)
		os.Exit(1)
	}
}

func run() error {
	configPath := os.Getenv("CAMPUSHUB_CONFIG_FILE")
	if configPath == "" {
		configPath = "configs/config.dev.yaml"
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	logger, err := logging.New(cfg.Log, cfg.App)
	if err != nil {
		return fmt.Errorf("initialize logger: %w", err)
	}
	defer logger.Sync()
	db, err := database.Open(cfg.MySQL)
	if err != nil {
		return fmt.Errorf("open mysql: %w", err)
	}
	defer func() {
		if err := database.Close(db); err != nil {
			logger.Warn("close mysql", zap.Error(err))
		}
	}()
	redisClient, err := redispkg.Open(cfg.Redis)
	if err != nil {
		return fmt.Errorf("open redis: %w", err)
	}
	defer func() {
		if err := redisClient.Close(); err != nil {
			logger.Warn("close redis", zap.Error(err))
		}
	}()
	kafkaClient, err := kafkapkg.Open(cfg.Kafka)
	if err != nil {
		return fmt.Errorf("open kafka: %w", err)
	}
	defer kafkaClient.Close()
	esClient, err := espkg.Open(cfg.Elasticsearch)
	if err != nil {
		return fmt.Errorf("open elasticsearch: %w", err)
	}
	defer func() {
		shutdownContext, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
		defer cancel()
		if err := esClient.Close(shutdownContext); err != nil {
			logger.Warn("close elasticsearch", zap.Error(err))
		}
	}()
	storageClient, err := storagepkg.Open(cfg.Storage)
	if err != nil {
		return fmt.Errorf("open rustfs: %w", err)
	}

	if cfg.App.Env == "prod" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.TestMode)
	}
	userRepository := repository.NewUserRepository(db)
	fileRepository := repository.NewFileRepository(db)
	tagRepository := repository.NewTagRepository(db)
	categoryRepository := repository.NewCategoryRepository(db)
	activityRepository := repository.NewActivityRepository(db)
	registrationRepository := repository.NewRegistrationRepository(db)
	ticketRepository := repository.NewTicketRepository(db)
	checkInRepository := repository.NewCheckInRepository(db)
	outboxRepository := repository.NewOutboxRepository(db)
	verificationRepository := repository.NewVerificationRepository(db)
	authService := service.NewAuthService(userRepository, token.NewManager(cfg.JWT), redisClient, mailpkg.New(cfg.Mail))
	authHandler := handler.NewAuthHandler(authService)
	userService := service.NewUserService(userRepository, tagRepository, fileRepository)
	fileService := service.NewFileService(fileRepository, storageClient, cfg.Storage)
	activityService := service.NewActivityService(activityRepository, categoryRepository, tagRepository, userRepository, fileRepository, fileService)
	activityHandler := handler.NewActivityHandler(activityService, userService)
	registrationService := service.NewRegistrationService(registrationRepository, ticketRepository, activityRepository, userRepository, fileRepository, verificationRepository, fileService)
	registrationHandler := handler.NewRegistrationHandler(registrationService, userService)
	checkInService := service.NewCheckInService(checkInRepository, ticketRepository, activityRepository, userRepository)
	checkInHandler := handler.NewCheckInHandler(checkInService, userService)
	outboxRelay := service.NewOutboxRelay(outboxRepository, kafkaClient)
	verificationService, err := service.NewVerificationService(verificationRepository, fileRepository, cfg.Security)
	if err != nil {
		return fmt.Errorf("initialize verification service: %w", err)
	}
	scheduler, err := startActivityScheduler(activityService, registrationService, checkInService, outboxRelay, cfg.Activity.SchedulerInterval, logger)
	if err != nil {
		return err
	}
	defer scheduler.Stop()
	readiness := health.New(cfg.Observability.ReadinessTimeout,
		health.CheckFunc{CheckName: "mysql", Fn: func(ctx context.Context) error { return database.Check(ctx, db) }},
		health.CheckFunc{CheckName: "redis", Fn: func(ctx context.Context) error { return redispkg.Check(ctx, redisClient) }},
		health.CheckFunc{CheckName: "kafka", Fn: func(ctx context.Context) error { return kafkaClient.Check(ctx) }},
		health.CheckFunc{CheckName: "elasticsearch", Fn: func(ctx context.Context) error { return esClient.Check(ctx) }},
	)
	engine := router.New(router.Dependencies{
		AuthHandler:         authHandler,
		UserHandler:         handler.NewUserHandler(userService, fileService),
		FileHandler:         handler.NewFileHandler(fileService, userService),
		VerificationHandler: handler.NewVerificationHandler(verificationService, userService),
		ActivityHandler:     activityHandler,
		RegistrationHandler: registrationHandler,
		CheckInHandler:      checkInHandler,
		Authenticator:       authService,
		AdminChecker:        authService,
		Readiness:           readiness,
		Logger:              logger,
		AllowedOrigins:      cfg.HTTP.AllowedOrigins,
	})
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.HTTP.Host, cfg.HTTP.Port),
		Handler:      engine,
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
		IdleTimeout:  cfg.HTTP.IdleTimeout,
	}
	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("server started", zap.String("addr", server.Addr))
		serverErrors <- server.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case signalValue := <-stop:
		logger.Info("shutdown signal received", zap.String("signal", signalValue.String()))
	case serveErr := <-serverErrors:
		if !errors.Is(serveErr, http.ErrServerClosed) {
			return fmt.Errorf("server stopped unexpectedly: %w", serveErr)
		}
		return nil
	}
	signal.Stop(stop)
	ctx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}
	logger.Info("server stopped")
	return nil
}

// startActivityScheduler runs the periodic maintenance tasks: the time based
// activity status flow, pending registration expiry, ticket expiry and outbox
// delivery.
func startActivityScheduler(activities *service.ActivityService, registrations *service.RegistrationService, checkIns *service.CheckInService, relay *service.OutboxRelay, interval time.Duration, logger *zap.Logger) (*cron.Cron, error) {
	runner := cron.New()
	if _, err := runner.AddFunc("@every "+interval.String(), func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		changed, err := activities.SyncStatuses(ctx)
		if err != nil {
			logger.Warn("activity status sync failed", zap.Error(err))
		} else if changed > 0 {
			logger.Info("activity status synced", zap.Int64("changed", changed))
		}
		expired, err := registrations.ExpirePending(ctx)
		if err != nil {
			logger.Warn("registration expiry failed", zap.Error(err))
		} else if expired > 0 {
			logger.Info("registrations expired", zap.Int64("expired", expired))
		}
		tickets, err := checkIns.ExpireTickets(ctx)
		if err != nil {
			logger.Warn("ticket expiry failed", zap.Error(err))
		} else if tickets > 0 {
			logger.Info("tickets expired", zap.Int64("expired", tickets))
		}
		delivered, failed, err := relay.Run(ctx)
		if err != nil {
			logger.Warn("outbox relay failed", zap.Error(err))
		} else if delivered > 0 || failed > 0 {
			logger.Info("outbox relayed", zap.Int("delivered", delivered), zap.Int("failed", failed))
		}
	}); err != nil {
		return nil, fmt.Errorf("schedule activity maintenance: %w", err)
	}
	runner.Start()
	return runner, nil
}
