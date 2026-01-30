package logger_test

import (
	"testing"

	"github.com/comfforts/logger"
)

func TestDefaultSlogLogger(t *testing.T) {
	l := logger.GetSlogLogger()
	l.Info("This is a Default Slog logger test log message")
}

func TestFileLogger(t *testing.T) {
	l := logger.GetSlogFileLogger("")
	l.Info("This is a File Slog logger test log message")
}

func TestSlogMultiLogger(t *testing.T) {
	l := logger.GetSlogMultiLogger("")
	l.Info("This is a Multi Slog logger test log message")
}
