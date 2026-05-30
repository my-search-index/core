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
	h := &handler{service: service}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Get("/healthz", h.health)

	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/documents", h.listDocuments)
		r.Post("/documents", h.addDocument)
		r.Delete("/documents", h.removeDocument)
		r.Get("/search", h.search)
	})

	return r
}
