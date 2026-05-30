package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/my-search-index/search-index-core/internal/httpapi"
	"github.com/my-search-index/search-index-core/internal/search"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := loadConfig()

	service, err := search.NewService(cfg.IndexPath)
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
		slog.Info("search index core listening", "addr", cfg.Addr, "index_path", cfg.IndexPath)
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

type config struct {
	Addr      string
	IndexPath string
}

func loadConfig() config {
	port := getenv("PORT", "8080")
	return config{
		Addr:      fmt.Sprintf(":%s", port),
		IndexPath: getenv("SEARCH_INDEX_PATH", "search.idx"),
	}
}

func getenv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
