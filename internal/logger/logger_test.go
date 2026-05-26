package logger

import (
	"os"
	"testing"
)

func TestNewLogger_Default(t *testing.T) {
	cfg := &Config{
		Level:      "info",
		FilePath:   os.TempDir() + "/test_default.log",
		MaxSize:    10,
		MaxBackups: 1,
		MaxAge:     1,
		Compress:   false,
	}
	defer os.Remove(cfg.FilePath)

	logger := NewLogger(cfg)
	if logger == nil {
		t.Fatal("NewLogger returned nil")
	}
}

func TestNewLogger_WithConfig(t *testing.T) {
	cfg := &Config{
		Level:      "debug",
		FilePath:   os.TempDir() + "/test_goagent.log",
		MaxSize:    10,
		MaxBackups: 3,
		MaxAge:     7,
		Compress:   false,
	}
	defer os.Remove(cfg.FilePath)

	logger := NewLogger(cfg)
	if logger == nil {
		t.Fatal("NewLogger(cfg) returned nil")
	}
}
