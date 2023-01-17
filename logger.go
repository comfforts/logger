package logger

import (
	"os"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

const DEFAULT_LOG_FILE_PATH = "logs/app.log"
const DEFAULT_LOG_LEVEL = zapcore.DebugLevel

type AppLogger interface {
	Info(msg string, fields ...zapcore.Field)
	Error(msg string, fields ...zapcore.Field)
	Debug(msg string, fields ...zapcore.Field)
	Fatal(msg string, fields ...zapcore.Field)
}

type appLogger struct {
	*zap.Logger
	config *AppLoggerConfig
}

type AppLoggerConfig struct {
	FilePath string
	Name     string
	Level    zapcore.Level
}

func NewAppLogger(config *AppLoggerConfig) *appLogger {
	logLevel := DEFAULT_LOG_LEVEL
	filePath := DEFAULT_LOG_FILE_PATH

	if config != nil {
		if config.FilePath != "" {
			filePath = config.FilePath
		}

		logLevel = config.Level
	}

	cfg := zap.NewProductionEncoderConfig()
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
	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	if config.Name != "" {
		logger = logger.Named(config.Name)
	}

	return &appLogger{
		Logger: logger,
		config: config,
	}
}

func NewTestAppLogger(dir string) *appLogger {
	logLevel := DEFAULT_LOG_LEVEL
	filePath := DEFAULT_LOG_FILE_PATH

	if dir != "" {
		filePath = filepath.Join(dir, DEFAULT_LOG_FILE_PATH)
	}

	logCfg := AppLoggerConfig{
		Level:    logLevel,
		FilePath: filePath,
	}

	cfg := zap.NewDevelopmentEncoderConfig()
	cfg.EncodeTime = zapcore.ISO8601TimeEncoder

	fileEncoder := zapcore.NewJSONEncoder(cfg)
	consoleEncoder := zapcore.NewConsoleEncoder(cfg)

	writer := zapcore.AddSync(&lumberjack.Logger{
		Filename:   logCfg.FilePath,
		MaxSize:    10, // megabytes
		MaxBackups: 3,
		MaxAge:     28, // days
	})

	core := zapcore.NewTee(
		zapcore.NewCore(fileEncoder, writer, logCfg.Level),
		zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), logCfg.Level),
	)
	logger := zap.New(core, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel)).Named("test")
	return &appLogger{
		Logger: logger,
		config: &logCfg,
	}
}
