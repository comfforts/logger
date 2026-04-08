package logger

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

const DEFAULT_LOG_FILE_PATH = "logs/app.log"
const NO_LOGGER_FOUND = "no logger found in context"

var ErrNoLoggerInContext = errors.New(NO_LOGGER_FOUND)

var (
	once    sync.Once
	base    *slog.Logger
	initErr error
)

type loggerContextKey string

const contextLoggerKey loggerContextKey = "ContextLoggerKey"

type Logger interface {
	Debug(msg string, fields ...any)
	Info(msg string, fields ...any)
	Warn(msg string, fields ...any)
	Error(msg string, fields ...any)
}

type Config struct {
	Level     slog.Leveler // slog.LevelInfo, slog.LevelDebug, ...
	Format    string       // "json" or "text"
	Writer    io.Writer    // defaults to os.Stdout
	AddSource bool
}

type Option func(*Config)

func WithLevel(level slog.Leveler) Option {
	return func(c *Config) { c.Level = level }
}

func WithFormat(format string) Option {
	return func(c *Config) { c.Format = format }
}

func WithWriter(w io.Writer) Option {
	return func(c *Config) { c.Writer = w }
}

func WithAddSource(add bool) Option {
	return func(c *Config) { c.AddSource = add }
}

func Init(opts ...Option) error {
	once.Do(func() {
		cfg := defaultConfig()
		for _, opt := range opts {
			opt(&cfg)
		}
		base, initErr = build(cfg)
	})
	return initErr
}

func build(c Config) (*slog.Logger, error) {
	opts := &slog.HandlerOptions{
		Level:     c.Level,
		AddSource: c.AddSource,
	}

	var h slog.Handler
	switch strings.ToLower(strings.TrimSpace(c.Format)) {
	case "json":
		h = slog.NewJSONHandler(c.Writer, opts)
	case "text", "console":
		h = slog.NewTextHandler(c.Writer, opts)
	default:
		h = slog.NewJSONHandler(c.Writer, opts)
	}

	return slog.New(h), nil
}

func defaultConfig() Config {
	env := strings.ToLower(strings.TrimSpace(os.Getenv("ENV")))
	goEnv := strings.ToLower(strings.TrimSpace(os.Getenv("GO_ENV")))
	infra := strings.ToLower(strings.TrimSpace(os.Getenv("INFRA")))

	logLevel := &slog.LevelVar{}
	logLevel.Set(slog.LevelInfo)

	dev := env == "dev" || env == "development" || goEnv == "dev" || goEnv == "development" || infra == "local"

	format := "json"
	if dev {
		// Use text format
		format = "text"
		// Set log level to Debug
		logLevel.Set(slog.LevelDebug)
	}

	return Config{
		Level:     logLevel,
		Format:    format,
		Writer:    os.Stdout,
		AddSource: true,
	}
}

// WithLogger returns a new context with the given logger.
func WithLogger(ctx context.Context, logger Logger) context.Context {
	return context.WithValue(ctx, contextLoggerKey, logger)
}

// SlogFromContext retrieves the slog.Logger from context.
// If none is found, returns error.
func SlogFromContext(ctx context.Context) (*slog.Logger, error) {
	l, ok := ctx.Value(contextLoggerKey).(*slog.Logger)
	if !ok || l == nil {
		return nil, ErrNoLoggerInContext
	}
	return l, nil
}

func WithAttrs(logger Logger, attrs ...any) Logger {
	if logger == nil {
		return GetSlogLogger().With(attrs...)
	}

	if l, ok := logger.(*slog.Logger); ok {
		return l.With(attrs...)
	}

	return GetSlogLogger().With(attrs...)
}

// WithContextAttrs retrieves the slog.Logger from context, adds the given attributes, and returns a new context with the updated logger.
func WithContextAttrs(ctx context.Context, attrs ...any) context.Context {
	l, err := SlogFromContext(ctx)
	if err != nil {
		l = GetSlogLogger()
	}
	return WithLogger(ctx, l.With(attrs...))
}

// WithTraceAttrs retrieves the current trace span from context and adds its trace_id and span_id as attributes to the logger in context.
func WithTraceAttrs(ctx context.Context) context.Context {
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return ctx
	}

	return WithContextAttrs(
		ctx,
		"trace_id", span.SpanContext().TraceID().String(),
		"span_id", span.SpanContext().SpanID().String(),
	)
}

// LoggerFromContext retrieves the logger from context.
// If none is found, returns a fallback logger.
func LoggerFromContext(ctx context.Context) (Logger, error) {
	if l, err := SlogFromContext(ctx); err == nil {
		return l, nil
	}

	logger, ok := ctx.Value(contextLoggerKey).(Logger)
	if !ok || logger == nil {
		return nil, ErrNoLoggerInContext
	}
	return logger, nil
}

// BaseWithAttrs returns a new slog.Logger with the given attributes, using the base logger as the source.
func BaseWithAttrs(attrs ...any) *slog.Logger {
	return GetSlogLogger().With(attrs...)
}

func GetSlogLogger() *slog.Logger {
	if base != nil {
		return base
	}

	// Initialize if not already done
	err := Init()
	if err != nil {
		// Defensive fallback; should not happen unless build failed unexpectedly.
		fmt.Println("######")
		fmt.Printf("Error occured, fallback logger initialized, logger will discard logs, error: %v\n", err)
		fmt.Println("######")
		fmt.Println()
		return slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	slog.SetDefault(base)
	return base
}

func GetSlogMultiLogger(dir string) *slog.Logger {
	if base != nil {
		return base
	}

	// lumberjack writer for log rotation
	logWriter := GetFileWriter(dir)

	// MultiWriter for logs in both file & console
	multiWriter := io.MultiWriter(os.Stdout, logWriter)

	// Initialize the logger with multiWriter
	err := Init(WithWriter(multiWriter))
	if err != nil {
		// Defensive fallback; should not happen unless build failed unexpectedly.
		fmt.Println("######")
		fmt.Printf("Error occured, fallback logger initialized, logger will discard logs, error: %v\n", err)
		fmt.Println("######")
		fmt.Println()
		return slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	slog.SetDefault(base)
	return base
}

func GetSlogFileLogger(dir string) *slog.Logger {
	if base != nil {
		return base
	}

	// lumberjack writer for log rotation
	fileWriter := GetFileWriter(dir)

	// Initialize the logger with fileWriter
	err := Init(WithWriter(fileWriter))
	if err != nil {
		// Defensive fallback; should not happen unless build failed unexpectedly.
		fmt.Println("######")
		fmt.Printf("Error occured, fallback logger initialized, logger will discard logs, error: %v\n", err)
		fmt.Println("######")
		fmt.Println()
		return slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	slog.SetDefault(base)
	return base
}

func GetFileWriter(dir string) io.Writer {
	filePath := DEFAULT_LOG_FILE_PATH
	if dir != "" {
		filePath = filepath.Join(dir, filePath)
	}

	// lumberjack writer for log rotation
	logWriter := &lumberjack.Logger{
		Filename:   filePath,
		MaxSize:    100, // megabytes
		MaxBackups: 5,
		MaxAge:     28,   // days
		Compress:   true, // compress rotated logs
	}

	return logWriter
}

func GetMultiWriter(dests ...io.Writer) io.Writer {
	return io.MultiWriter(dests...)
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
