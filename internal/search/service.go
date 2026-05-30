package search

import (
	"fmt"
	"log/slog"
	"os"
	"sync"

	searchindex "github.com/my-search-index/search-index"
)

// Service coordinates access to the persisted search index.
//
// The underlying index is kept in memory for fast reads. Mutating operations
// update the in-memory index first and then save it back to disk so HTTP
// handlers can use this type without knowing the index persistence details.
type Service struct {
	mu        sync.RWMutex
	indexPath string
	idx       *searchindex.Index
}

// NewService loads the index stored at indexPath and returns a ready-to-use
// search service.
func NewService(indexPath string) (*Service, error) {
	idx, err := searchindex.Load(indexPath)
	if err != nil {
		return nil, fmt.Errorf("load index: %w", err)
	}
	slog.Info("search index loaded", "index_path", indexPath, "documents", len(idx.Docs), "terms", len(idx.Postings))
	return &Service{indexPath: indexPath, idx: idx}, nil
}

// Search runs a full-text query against the current index.
func (s *Service) Search(query string) []searchindex.Result {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := s.idx.Search(query)
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
func (s *Service) ListDocuments() []searchindex.Document {
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
