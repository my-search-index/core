package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/my-search-index/core/internal/httpapi"
	"github.com/my-search-index/core/internal/search"
)

// main starts the search index core API process.
func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

// run wires configuration, services, and the HTTP server together.
//
// It blocks until the server fails or the process receives an interrupt signal.
func run() error {
	if err := loadDotEnv(); err != nil {
		return err
	}

	cfg := loadConfig()
	configureLogger(cfg)

	service, err := search.NewService(cfg.IndexPath, cfg.UploadDir)
	if err != nil {
		return err
	}

	server := &http.Server{
		Addr:         cfg.Addr,
		Handler:      httpapi.NewRouter(service),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("search index core listening", "addr", cfg.Addr, "index_path", cfg.IndexPath, "upload_dir", cfg.UploadDir)
		errCh <- server.ListenAndServe()
	}()

	shutdownCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	select {
	case <-shutdownCtx.Done():
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return server.Shutdown(ctx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

// loadDotEnv loads local development configuration from a .env file when one
// exists.
//
// Values already exported in the process environment take priority over .env
// values, which keeps production configuration explicit.
func loadDotEnv() error {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load .env: %w", err)
	}
	return nil
}

// config contains the runtime settings needed by the HTTP server.
type config struct {
	Addr      string
	IndexPath string
	UploadDir string
	LogLevel  string
	LogFormat string
}

// loadConfig reads runtime configuration from environment variables.
func loadConfig() config {
	port := getenv("PORT", "8080")
	return config{
		Addr:      fmt.Sprintf(":%s", port),
		IndexPath: getenv("SEARCH_INDEX_PATH", "search.idx"),
		UploadDir: getenv("UPLOAD_DIR", "uploads"),
		LogLevel:  getenv("LOG_LEVEL", "info"),
		LogFormat: getenv("LOG_FORMAT", "text"),
	}
}

// configureLogger sets the process-wide structured logger.
func configureLogger(cfg config) {
	opts := &slog.HandlerOptions{
		Level: parseLogLevel(cfg.LogLevel),
	}

	var handler slog.Handler
	if strings.EqualFold(cfg.LogFormat, "json") {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}
	slog.SetDefault(slog.New(handler))
}

// parseLogLevel converts a LOG_LEVEL string into a slog level.
func parseLogLevel(value string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// getenv returns the environment value for key, or fallback when key is empty.
func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
