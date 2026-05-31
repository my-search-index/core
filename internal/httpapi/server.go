package httpapi

import (
	"net/http"
	"slices"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/my-search-index/core/internal/search"
)

var defaultAllowedOrigins = []string{
	"http://localhost:5173",
	"http://127.0.0.1:5173",
}

// RouterConfig contains HTTP transport settings for the API router.
type RouterConfig struct {
	AllowedOrigins []string
}

// NewRouter builds the HTTP API router for the search index core service.
//
// The router owns request middleware and versioned API routes. Business logic
// stays in the search service so handlers remain thin transport adapters.
func NewRouter(service *search.Service) http.Handler {
	return NewRouterWithConfig(service, RouterConfig{
		AllowedOrigins: defaultAllowedOrigins,
	})
}

// NewRouterWithConfig builds the HTTP API router with explicit transport
// settings.
func NewRouterWithConfig(service *search.Service, config RouterConfig) http.Handler {
	handler := &handler{service: service}

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(corsMiddleware(config.AllowedOrigins))
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Use(middleware.Timeout(30 * time.Second))

	router.Get("/healthz", handler.health)

	router.Route("/api/v1", func(r chi.Router) {
		r.Get("/documents", handler.listDocuments)
		r.Post("/documents", handler.addDocument)
		r.Delete("/documents", handler.removeDocument)
		r.Get("/search", handler.search)
	})

	return router
}

func corsMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && slices.Contains(allowedOrigins, origin) {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
