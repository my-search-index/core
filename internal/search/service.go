package search

import (
	"fmt"
	"sync"

	searchindex "github.com/my-search-index/search-index"
)

type Service struct {
	mu        sync.RWMutex
	indexPath string
	idx       *searchindex.Index
}

func NewService(indexPath string) (*Service, error) {
	idx, err := searchindex.Load(indexPath)
	if err != nil {
		return nil, fmt.Errorf("load index: %w", err)
	}
	return &Service{indexPath: indexPath, idx: idx}, nil
}

func (s *Service) Search(query string) []searchindex.Result {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.idx.Search(query)
}

func (s *Service) ListDocuments() []searchindex.Document {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.idx.ListDocuments()
}

func (s *Service) AddDocument(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.idx.AddDocument(path); err != nil {
		return fmt.Errorf("add document: %w", err)
	}
	return s.saveLocked()
}

func (s *Service) AddDocuments(root string, extensions []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.idx.AddDocuments(root, extensions); err != nil {
		return fmt.Errorf("add documents: %w", err)
	}
	return s.saveLocked()
}

func (s *Service) RemoveDocument(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.idx.RemoveDocument(path); err != nil {
		return fmt.Errorf("remove document: %w", err)
	}
	return s.saveLocked()
}

func (s *Service) saveLocked() error {
	if err := s.idx.Save(s.indexPath); err != nil {
		return fmt.Errorf("save index: %w", err)
	}
	return nil
}
