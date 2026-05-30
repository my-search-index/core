package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/my-search-index/core/internal/search"
)

const (
	maxUploadMemory = 32 << 20
	maxUploadSize   = 100 << 20
)

// handler groups the HTTP handlers that share the same search service.
type handler struct {
	service *search.Service
}

// response is the common JSON envelope returned by every API endpoint.
type response struct {
	OK    bool        `json:"ok"`
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

// documentRequest is the JSON payload used to add either one file or every
// supported file under a directory.
type documentRequest struct {
	Path       string   `json:"path"`
	Directory  bool     `json:"directory"`
	Extensions []string `json:"extensions,omitempty"`
}

// health reports whether the API process is running.
func (h *handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, response{
		OK:   true,
		Data: map[string]string{"status": "ok"},
	})
}

// listDocuments returns the documents currently stored in the index.
func (h *handler) listDocuments(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, response{
		OK:   true,
		Data: h.service.ListDocuments(),
	})
}

// addDocument indexes a single file or a directory, depending on the request
// payload.
func (h *handler) addDocument(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/form-data") {
		h.addUploadedDocuments(w, r)
		return
	}

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

// addUploadedDocuments stores and indexes files sent as multipart form data.
func (h *handler) addUploadedDocuments(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadMemory); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("parse multipart form: %w", err))
		return
	}
	defer r.MultipartForm.RemoveAll()

	files := r.MultipartForm.File["files"]
	files = append(files, r.MultipartForm.File["file"]...)
	if len(files) == 0 {
		writeError(w, http.StatusBadRequest, errors.New("multipart file field is required"))
		return
	}

	documents := make([]search.Document, 0, len(files))
	for _, header := range files {
		file, err := header.Open()
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("open uploaded file: %w", err))
			return
		}

		doc, err := h.service.AddUploadedDocument(header.Filename, file)
		_ = file.Close()
		if err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
		documents = append(documents, doc)
	}

	writeJSON(w, http.StatusCreated, response{
		OK:   true,
		Data: documents,
	})
}

// removeDocument removes one indexed file identified by its path query
// parameter.
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

// search executes a query and returns ranked search results with snippets.
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

// readJSON decodes one JSON request body and rejects unknown fields so API
// clients get quick feedback for misspelled payload keys.
func readJSON(r *http.Request, dst interface{}) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	return nil
}

// writeJSON serializes a response envelope with the provided HTTP status code.
func writeJSON(w http.ResponseWriter, status int, payload response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

// writeError serializes an error using the common response envelope.
func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, response{
		OK:    false,
		Error: err.Error(),
	})
}
