package search

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	searchindex "github.com/my-search-index/search-index"
)

// Document describes a file stored in the search index.
type Document = searchindex.Document

// Result describes one ranked search hit returned by the index.
type Result = searchindex.Result

// Service coordinates access to the persisted search index.
//
// The underlying index is kept in memory for fast reads. Mutating operations
// update the in-memory index first and then save it back to disk so HTTP
// handlers can use this type without knowing the index persistence details.
type Service struct {
	mu        sync.RWMutex
	indexPath string
	uploadDir string
	idx       *searchindex.Index
}

// NewService loads the index stored at indexPath and returns a ready-to-use
// search service.
func NewService(indexPath, uploadDir string) (*Service, error) {
	idx, err := searchindex.Load(indexPath)
	if err != nil {
		return nil, fmt.Errorf("load index: %w", err)
	}
	slog.Info("search index loaded", "index_path", indexPath, "documents", len(idx.Docs), "terms", len(idx.Postings))
	return &Service{indexPath: indexPath, uploadDir: uploadDir, idx: idx}, nil
}

// Search runs a full-text query against the current index.
func (s *Service) Search(query string) []Result {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := s.idx.Search(query)
	if results == nil {
		results = []Result{}
	}
	missingSnippetCount := 0
	for _, result := range results {
		if len(result.Snippets) > 0 {
			continue
		}

		missingSnippetCount++
		if _, err := os.Stat(result.Doc.FilePath); err != nil {
			slog.Warn(
				"search result has no snippets because source file cannot be opened",
				"query", query,
				"doc_id", result.Doc.ID,
				"file_path", result.Doc.FilePath,
				"error", err,
			)
		}
	}

	slog.Debug(
		"search completed",
		"query", query,
		"results", len(results),
		"results_without_snippets", missingSnippetCount,
	)
	return results
}

// ListDocuments returns all indexed documents in deterministic order.
func (s *Service) ListDocuments() []Document {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.idx.ListDocuments()
}

// AddDocument indexes one document and persists the updated index.
func (s *Service) AddDocument(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.idx.AddDocument(path); err != nil {
		return fmt.Errorf("add document: %w", err)
	}
	return s.saveLocked()
}

// AddUploadedDocument stores an uploaded document on the backend host, indexes
// that stored copy, and persists the updated index.
func (s *Service) AddUploadedDocument(filename string, src io.Reader) (Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path, err := s.saveUpload(filename, src)
	if err != nil {
		return Document{}, err
	}

	if err := s.idx.AddDocument(path); err != nil {
		_ = os.Remove(path)
		return Document{}, fmt.Errorf("add uploaded document: %w", err)
	}
	if err := s.saveLocked(); err != nil {
		return Document{}, err
	}

	doc, ok := s.findDocumentByPathLocked(path)
	if !ok {
		return Document{}, fmt.Errorf("uploaded document was not indexed: %s", path)
	}
	slog.Info("uploaded document indexed", "file_path", path, "doc_id", doc.ID)
	return doc, nil
}

// AddDocuments indexes every supported document under root and persists the
// updated index.
func (s *Service) AddDocuments(root string, extensions []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.idx.AddDocuments(root, extensions); err != nil {
		return fmt.Errorf("add documents: %w", err)
	}
	return s.saveLocked()
}

// RemoveDocument deletes one document from the index and persists the updated
// index.
func (s *Service) RemoveDocument(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.idx.RemoveDocument(path); err != nil {
		return fmt.Errorf("remove document: %w", err)
	}
	return s.saveLocked()
}

// saveLocked writes the current in-memory index to disk.
//
// Callers must hold s.mu for writing before calling this method.
func (s *Service) saveLocked() error {
	if err := s.idx.Save(s.indexPath); err != nil {
		return fmt.Errorf("save index: %w", err)
	}
	return nil
}

// saveUpload copies an uploaded file into the configured upload directory.
//
// The search index needs a durable file path so snippets can reopen the source
// document after the HTTP request has finished.
func (s *Service) saveUpload(filename string, src io.Reader) (string, error) {
	if err := os.MkdirAll(s.uploadDir, 0o755); err != nil {
		return "", fmt.Errorf("create upload directory: %w", err)
	}

	name := safeUploadName(filename)
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	pattern := fmt.Sprintf("%s-%d-*%s", base, time.Now().UnixNano(), ext)

	file, err := os.CreateTemp(s.uploadDir, pattern)
	if err != nil {
		return "", fmt.Errorf("create uploaded file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, src); err != nil {
		_ = os.Remove(file.Name())
		return "", fmt.Errorf("store uploaded file: %w", err)
	}
	return file.Name(), nil
}

// findDocumentByPathLocked returns the indexed document for path.
//
// Callers must hold s.mu before calling this method.
func (s *Service) findDocumentByPathLocked(path string) (Document, bool) {
	for _, doc := range s.idx.Docs {
		if doc.FilePath == path {
			return doc, true
		}
	}
	return Document{}, false
}

// safeUploadName returns a filesystem-safe name derived from an uploaded file
// name.
func safeUploadName(filename string) string {
	name := filepath.Base(strings.TrimSpace(filename))
	if name == "." || name == string(filepath.Separator) || name == "" {
		return "document.txt"
	}

	var b strings.Builder
	for _, char := range name {
		switch {
		case char >= 'a' && char <= 'z':
			b.WriteRune(char)
		case char >= 'A' && char <= 'Z':
			b.WriteRune(char)
		case char >= '0' && char <= '9':
			b.WriteRune(char)
		case char == '.', char == '-', char == '_':
			b.WriteRune(char)
		default:
			b.WriteRune('-')
		}
	}

	safe := strings.Trim(b.String(), ".-")
	if safe == "" {
		return "document.txt"
	}
	return safe
}
