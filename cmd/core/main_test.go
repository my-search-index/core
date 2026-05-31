package main

import (
	"log/slog"
	"testing"
)

func TestLoadConfigUsesDefaults(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("SEARCH_INDEX_PATH", "")
	t.Setenv("UPLOAD_DIR", "")
	t.Setenv("CORS_ALLOWED_ORIGINS", "")
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
	if got, want := cfg.AllowedOrigins, []string{"http://localhost:5173", "http://127.0.0.1:5173"}; !equalStrings(got, want) {
		t.Fatalf("expected default CORS origins %v, got %v", want, got)
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
	t.Setenv("CORS_ALLOWED_ORIGINS", "http://localhost:5173, http://example.test")
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
	if got, want := cfg.AllowedOrigins, []string{"http://localhost:5173", "http://example.test"}; !equalStrings(got, want) {
		t.Fatalf("expected env CORS origins %v, got %v", want, got)
	}
	if cfg.LogLevel != "debug" {
		t.Fatalf("expected env log level, got %q", cfg.LogLevel)
	}
	if cfg.LogFormat != "json" {
		t.Fatalf("expected env log format, got %q", cfg.LogFormat)
	}
}

func TestSplitCSV(t *testing.T) {
	got := splitCSV(" http://localhost:5173, ,http://127.0.0.1:5173 ")
	want := []string{"http://localhost:5173", "http://127.0.0.1:5173"}

	if !equalStrings(got, want) {
		t.Fatalf("splitCSV returned %v, want %v", got, want)
	}
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
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
