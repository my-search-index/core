package httpapi

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/my-search-index/core/internal/search"
)

// NewRouter builds the HTTP API router for the search index core service.
//
// The router owns request middleware and versioned API routes. Business logic
// stays in the search service so handlers remain thin transport adapters.
func NewRouter(service *search.Service) http.Handler {
	handler := &handler{service: service}

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
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
