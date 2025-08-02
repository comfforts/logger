package logger

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

const DEFAULT_LOG_FILE_PATH = "logs/app.log"
const NO_LOGGER_FOUND = "no logger found in context"

var ErrNoLoggerInContext = errors.New(NO_LOGGER_FOUND)

type LoggerContextKey string

const contextLoggerKey LoggerContextKey = "ContextLoggerKey"

type Logger interface {
	Debug(msg string, fields ...any)
	Info(msg string, fields ...any)
	Warn(msg string, fields ...any)
	Error(msg string, fields ...any)
}

// WithLogger returns a new context with the given logger.
func WithLogger(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, contextLoggerKey, logger)
}

// LoggerFromContext retrieves the logger from context.
// If none is found, returns a fallback logger.
func LoggerFromContext(ctx context.Context) (Logger, error) {
	logger, ok := ctx.Value(contextLoggerKey).(Logger)
	if !ok {
		return nil, ErrNoLoggerInContext
	}
	return logger, nil
}

func GetSlogLogger() *slog.Logger {
	// Initialize log level to Info
	logLevel := &slog.LevelVar{}
	logLevel.Set(slog.LevelInfo)

	// Set log level to Debug if running in local infrastructure
	if os.Getenv("INFRA") == "local" {
		logLevel.Set(slog.LevelDebug)
	}

	// Setup slog handler options. TODO update for log formatting
	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	// Using TextHandler. TODO use JsonHandler for structured logging
	handler := slog.NewTextHandler(os.Stdout, opts)

	l := slog.New(handler)
	slog.SetDefault(l)

	return l
}

func GetSlogMultiLogger(dir string) *slog.Logger {
	filePath := DEFAULT_LOG_FILE_PATH
	if dir != "" {
		filePath = filepath.Join(dir, filePath)
	}

	// Initialize log level to Info
	logLevel := &slog.LevelVar{}
	logLevel.Set(slog.LevelInfo)

	// Set log level to Debug if running in local infrastructure
	if os.Getenv("INFRA") == "local" {
		logLevel.Set(slog.LevelDebug)
	}

	// lumberjack writer for log rotation
	logWriter := &lumberjack.Logger{
		Filename:   filePath,
		MaxSize:    100, // megabytes
		MaxBackups: 5,
		MaxAge:     28,   // days
		Compress:   true, // compress rotated logs
	}

	// MultiWriter for logs in both file & console
	multiWriter := io.MultiWriter(os.Stdout, logWriter)

	// Setup slog handler options. TODO update for log formatting
	opts := &slog.HandlerOptions{
		Level: logLevel,
	}

	// Using TextHandler. TODO use JsonHandler for structured logging
	handler := slog.NewTextHandler(multiWriter, opts)

	l := slog.New(handler)
	slog.SetDefault(l)

	return l
}

func GetZapLogger(dir, namedAs string) *zap.Logger {
	filePath := DEFAULT_LOG_FILE_PATH
	if dir != "" {
		filePath = filepath.Join(dir, filePath)
	}

	logLevel := zapcore.InfoLevel
	cfg := zap.NewProductionEncoderConfig()
	if os.Getenv("INFRA") == "local" {
		logLevel = zapcore.DebugLevel
		cfg = zap.NewDevelopmentEncoderConfig()
	}
	cfg.EncodeTime = zapcore.ISO8601TimeEncoder

	fileEncoder := zapcore.NewJSONEncoder(cfg)
	consoleEncoder := zapcore.NewConsoleEncoder(cfg)

	writer := zapcore.AddSync(&lumberjack.Logger{
		Filename:   filePath,
		MaxSize:    10, // megabytes
		MaxBackups: 3,
		MaxAge:     28, // days
	})

	core := zapcore.NewTee(
		zapcore.NewCore(fileEncoder, writer, logLevel),
		zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), logLevel),
	)
	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel)).Named(namedAs)
	return logger
}
