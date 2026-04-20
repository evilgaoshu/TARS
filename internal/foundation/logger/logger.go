package logger

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/natefinch/lumberjack.v2"
	"tars/internal/foundation/config"
	"tars/internal/foundation/metrics"
	"tars/internal/foundation/observability"
)

func New(level string) *slog.Logger {
	return NewWithOptions(config.Config{LogLevel: level}, nil, nil)
}

func NewWithOptions(cfg config.Config, store *observability.Store, registry *metrics.Registry) *slog.Logger {
	level := parseLevel(cfg.LogLevel)
	writer := io.Writer(os.Stdout)
	if fileWriter := buildRotatingFileWriter(cfg); fileWriter != nil {
		writer = io.MultiWriter(os.Stdout, fileWriter)
	}
	var handler slog.Handler = slog.NewJSONHandler(writer, &slog.HandlerOptions{Level: level})
	if store != nil {
		handler = observability.NewHandler(level, handler, store, registry)
	}
	return slog.New(handler)
}

func parseLevel(level string) slog.Level {
	switch level {
	case "DEBUG":
		return slog.LevelDebug
	case "WARN":
		return slog.LevelWarn
	case "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func buildRotatingFileWriter(cfg config.Config) *lumberjack.Logger {
	dataDir := strings.TrimSpace(cfg.Observability.DataDir)
	if dataDir == "" {
		return nil
	}
	logPath := filepath.Join(dataDir, "logs", "runtime.log")
	maxMB := int(cfg.Observability.Logs.MaxSizeBytes / (1024 * 1024))
	if maxMB <= 0 {
		maxMB = 10 * 1024
	}
	maxAgeDays := int(cfg.Observability.Logs.Retention.Hours() / 24)
	if maxAgeDays <= 0 {
		maxAgeDays = 7
	}
	return &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    maxMB,
		MaxAge:     maxAgeDays,
		MaxBackups: 3,
		Compress:   false,
	}
}
