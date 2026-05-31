package search

import (
	"strings"
	"testing"
)

func TestAddUploadedDocumentStoresIndexesAndSearchesFile(t *testing.T) {
	service := newTestService(t)

	doc, err := service.AddUploadedDocument("unsafe name../Distributed Notes.txt", strings.NewReader("Distributed search keeps crawler workers coordinated.\n"))
	if err != nil {
		t.Fatalf("add uploaded document: %v", err)
	}

	if doc.ID == 0 {
		t.Fatalf("expected document ID to be set")
	}
	if !strings.HasPrefix(doc.FilePath, service.uploadDir) {
		t.Fatalf("expected uploaded file under %q, got %q", service.uploadDir, doc.FilePath)
	}
	if strings.Contains(doc.FilePath, "..") {
		t.Fatalf("expected sanitized upload path, got %q", doc.FilePath)
	}

	results := service.Search("distributed")
	if len(results) != 1 {
		t.Fatalf("expected 1 search result, got %d", len(results))
	}
	if len(results[0].Snippets) == 0 {
		t.Fatalf("expected snippets for uploaded document")
	}
	if len(results[0].Snippets[0].Matches) == 0 {
		t.Fatalf("expected snippet matches for uploaded document")
	}
}

func TestListDocumentsReturnsUploadedDocuments(t *testing.T) {
	service := newTestService(t)

	if _, err := service.AddUploadedDocument("first.txt", strings.NewReader("alpha beta")); err != nil {
		t.Fatalf("add first upload: %v", err)
	}
	if _, err := service.AddUploadedDocument("second.txt", strings.NewReader("gamma delta")); err != nil {
		t.Fatalf("add second upload: %v", err)
	}

	docs := service.ListDocuments()
	if len(docs) != 2 {
		t.Fatalf("expected 2 documents, got %d", len(docs))
	}
	if docs[0].ID >= docs[1].ID {
		t.Fatalf("expected documents sorted by ID, got IDs %d then %d", docs[0].ID, docs[1].ID)
	}
}

func TestSafeUploadName(t *testing.T) {
	tests := map[string]string{
		"../notes.txt":         "notes.txt",
		"distributed notes.md": "distributed-notes.md",
		"":                     "document.txt",
		"***":                  "document.txt",
	}

	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			if got := safeUploadName(input); got != want {
				t.Fatalf("safeUploadName(%q) = %q, want %q", input, got, want)
			}
		})
	}
}

func newTestService(t *testing.T) *Service {
	t.Helper()

	root := t.TempDir()
	service, err := NewService(root+"/search.idx", root+"/uploads")
	if err != nil {
		t.Fatalf("new service: %v", err)
	}
	return service
}
