package httpapi

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	searchindex "github.com/my-search-index/search-index"

	"github.com/my-search-index/core/internal/search"
)

func TestAddDocumentUploadsFileAndSearchReturnsSnippets(t *testing.T) {
	router, uploadDir := newTestRouter(t)

	body, contentType := multipartBody(t, map[string]string{
		"file": "Distributed systems can coordinate crawler workers.\nThe crawler remains distributed.\n",
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/documents", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d: %s", http.StatusCreated, rec.Code, rec.Body.String())
	}

	var uploadResp apiResponse[[]search.Document]
	decodeJSON(t, rec.Body, &uploadResp)
	if !uploadResp.OK {
		t.Fatalf("expected ok upload response, got error %q", uploadResp.Error)
	}
	if len(uploadResp.Data) != 1 {
		t.Fatalf("expected 1 uploaded document, got %d", len(uploadResp.Data))
	}

	doc := uploadResp.Data[0]
	if !strings.HasPrefix(doc.FilePath, uploadDir) {
		t.Fatalf("expected uploaded file under %q, got %q", uploadDir, doc.FilePath)
	}
	if _, err := os.Stat(doc.FilePath); err != nil {
		t.Fatalf("expected uploaded file to exist: %v", err)
	}

	searchReq := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=distributed", nil)
	searchRec := httptest.NewRecorder()

	router.ServeHTTP(searchRec, searchReq)

	if searchRec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, searchRec.Code, searchRec.Body.String())
	}

	var searchResp apiResponse[[]searchindex.Result]
	decodeJSON(t, searchRec.Body, &searchResp)
	if !searchResp.OK {
		t.Fatalf("expected ok search response, got error %q", searchResp.Error)
	}
	if len(searchResp.Data) != 1 {
		t.Fatalf("expected 1 search result, got %d", len(searchResp.Data))
	}
	if len(searchResp.Data[0].Snippets) == 0 {
		t.Fatalf("expected uploaded document search result to include snippets")
	}
	if len(searchResp.Data[0].Snippets[0].Matches) == 0 {
		t.Fatalf("expected first snippet to include highlight matches")
	}
}

func TestAddDocumentMultipartRequiresFileField(t *testing.T) {
	router, _ := newTestRouter(t)

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	if err := writer.WriteField("title", "missing file"); err != nil {
		t.Fatalf("write multipart field: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/documents", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d: %s", http.StatusBadRequest, rec.Code, rec.Body.String())
	}

	var resp apiResponse[json.RawMessage]
	decodeJSON(t, rec.Body, &resp)
	if resp.OK {
		t.Fatalf("expected failed response")
	}
	if !strings.Contains(resp.Error, "multipart file field is required") {
		t.Fatalf("expected missing file error, got %q", resp.Error)
	}
}

func TestSearchReturnsEmptyArrayWhenNoDocumentsMatch(t *testing.T) {
	router, _ := newTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/search?q=missing", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp apiResponse[[]search.Result]
	decodeJSON(t, rec.Body, &resp)
	if !resp.OK {
		t.Fatalf("expected ok search response, got error %q", resp.Error)
	}
	if resp.Data == nil {
		t.Fatalf("expected empty search data array, got nil")
	}
	if len(resp.Data) != 0 {
		t.Fatalf("expected no search results, got %d", len(resp.Data))
	}
}

func TestRouterAllowsConfiguredCORSOrigin(t *testing.T) {
	router, _ := newTestRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/documents", nil)
	req.Header.Set("Origin", "http://127.0.0.1:5173")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d: %s", http.StatusOK, rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://127.0.0.1:5173" {
		t.Fatalf("expected CORS allow origin header, got %q", got)
	}
	if got := rec.Header().Get("Vary"); got != "Origin" {
		t.Fatalf("expected Vary Origin header, got %q", got)
	}
}

func TestRouterHandlesCORSPreflight(t *testing.T) {
	router, _ := newTestRouter(t)

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/documents", nil)
	req.Header.Set("Origin", "http://127.0.0.1:5173")
	req.Header.Set("Access-Control-Request-Method", "POST")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d: %s", http.StatusNoContent, rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "http://127.0.0.1:5173" {
		t.Fatalf("expected CORS allow origin header, got %q", got)
	}
}

type apiResponse[T any] struct {
	OK    bool   `json:"ok"`
	Data  T      `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

func newTestRouter(t *testing.T) (http.Handler, string) {
	t.Helper()

	root := t.TempDir()
	indexPath := filepath.Join(root, "search.idx")
	uploadDir := filepath.Join(root, "uploads")

	service, err := search.NewService(indexPath, uploadDir)
	if err != nil {
		t.Fatalf("new search service: %v", err)
	}

	return NewRouter(service), uploadDir
}

func multipartBody(t *testing.T, files map[string]string) (*bytes.Buffer, string) {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	for field, content := range files {
		part, err := writer.CreateFormFile(field, field+".txt")
		if err != nil {
			t.Fatalf("create multipart file: %v", err)
		}
		if _, err := part.Write([]byte(content)); err != nil {
			t.Fatalf("write multipart file: %v", err)
		}
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}
	return &body, writer.FormDataContentType()
}

func decodeJSON(t *testing.T, body *bytes.Buffer, dst interface{}) {
	t.Helper()

	if err := json.NewDecoder(body).Decode(dst); err != nil {
		t.Fatalf("decode JSON response: %v\nbody: %s", err, body.String())
	}
}
