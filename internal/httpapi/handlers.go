package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/my-search-index/search-index-core/internal/search"
)

type handler struct {
	service *search.Service
}

type response struct {
	OK    bool        `json:"ok"`
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

type documentRequest struct {
	Path       string   `json:"path"`
	Directory  bool     `json:"directory"`
	Extensions []string `json:"extensions,omitempty"`
}

func (h *handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, response{
		OK:   true,
		Data: map[string]string{"status": "ok"},
	})
}

func (h *handler) listDocuments(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, response{
		OK:   true,
		Data: h.service.ListDocuments(),
	})
}

func (h *handler) addDocument(w http.ResponseWriter, r *http.Request) {
	var req documentRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	req.Path = strings.TrimSpace(req.Path)
	if req.Path == "" {
		writeError(w, http.StatusBadRequest, errors.New("path is required"))
		return
	}

	if req.Directory {
		extensions := req.Extensions
		if len(extensions) == 0 {
			extensions = []string{".txt", ".md"}
		}
		if err := h.service.AddDocuments(req.Path, extensions); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	} else {
		if err := h.service.AddDocument(req.Path); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}

	writeJSON(w, http.StatusCreated, response{OK: true})
}

func (h *handler) removeDocument(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSpace(r.URL.Query().Get("path"))
	if path == "" {
		writeError(w, http.StatusBadRequest, errors.New("path query parameter is required"))
		return
	}

	if err := h.service.RemoveDocument(path); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}

	writeJSON(w, http.StatusOK, response{OK: true})
}

func (h *handler) search(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeError(w, http.StatusBadRequest, errors.New("q query parameter is required"))
		return
	}

	writeJSON(w, http.StatusOK, response{
		OK:   true,
		Data: h.service.Search(query),
	})
}

func readJSON(r *http.Request, dst interface{}) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, payload response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, response{
		OK:    false,
		Error: err.Error(),
	})
}
