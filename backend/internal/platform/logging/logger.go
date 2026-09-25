package logging

import (
	"fmt"
	"time"

	"github.com/wangn-tech/campus-hub/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func New(cfg config.LogConfig, app config.AppConfig) (*zap.Logger, error) {
	level := zap.NewAtomicLevel()
	if err := level.UnmarshalText([]byte(cfg.Level)); err != nil {
		return nil, fmt.Errorf("parse log level: %w", err)
	}
	loggerConfig := zap.NewProductionConfig()
	loggerConfig.Level = level
	loggerConfig.Encoding = cfg.Encoding
	loggerConfig.OutputPaths = []string{cfg.Output}
	loggerConfig.ErrorOutputPaths = []string{cfg.Output}
	loggerConfig.EncoderConfig.EncodeTime = func(t time.Time, encoder zapcore.PrimitiveArrayEncoder) {
		encoder.AppendInt64(t.UnixMilli())
	}
	logger, err := loggerConfig.Build()
	if err != nil {
		return nil, fmt.Errorf("build logger: %w", err)
	}
	return logger.With(zap.String("service", app.Name), zap.String("env", app.Env), zap.String("instance_id", app.InstanceID)), nil
}
