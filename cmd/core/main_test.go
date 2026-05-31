package main

import (
	"log/slog"
	"testing"
)

func TestLoadConfigUsesDefaults(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("SEARCH_INDEX_PATH", "")
	t.Setenv("UPLOAD_DIR", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("LOG_FORMAT", "")

	cfg := loadConfig()

	if cfg.Addr != ":8080" {
		t.Fatalf("expected default addr :8080, got %q", cfg.Addr)
	}
	if cfg.IndexPath != "search.idx" {
		t.Fatalf("expected default index path, got %q", cfg.IndexPath)
	}
	if cfg.UploadDir != "uploads" {
		t.Fatalf("expected default upload dir, got %q", cfg.UploadDir)
	}
	if cfg.LogLevel != "info" {
		t.Fatalf("expected default log level, got %q", cfg.LogLevel)
	}
	if cfg.LogFormat != "text" {
		t.Fatalf("expected default log format, got %q", cfg.LogFormat)
	}
}

func TestLoadConfigUsesEnvironment(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("SEARCH_INDEX_PATH", "/tmp/search.idx")
	t.Setenv("UPLOAD_DIR", "/tmp/uploads")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("LOG_FORMAT", "json")

	cfg := loadConfig()

	if cfg.Addr != ":9090" {
		t.Fatalf("expected env addr :9090, got %q", cfg.Addr)
	}
	if cfg.IndexPath != "/tmp/search.idx" {
		t.Fatalf("expected env index path, got %q", cfg.IndexPath)
	}
	if cfg.UploadDir != "/tmp/uploads" {
		t.Fatalf("expected env upload dir, got %q", cfg.UploadDir)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("expected env log level, got %q", cfg.LogLevel)
	}
	if cfg.LogFormat != "json" {
		t.Fatalf("expected env log format, got %q", cfg.LogFormat)
	}
}

func TestParseLogLevel(t *testing.T) {
	tests := map[string]slog.Level{
		"debug":   slog.LevelDebug,
		"info":    slog.LevelInfo,
		"warn":    slog.LevelWarn,
		"warning": slog.LevelWarn,
		"error":   slog.LevelError,
		"unknown": slog.LevelInfo,
		"":        slog.LevelInfo,
	}

	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			if got := parseLogLevel(input); got != want {
				t.Fatalf("parseLogLevel(%q) = %v, want %v", input, got, want)
			}
		})
	}
}
