# File Upload Handler Flow

This document explains how the core API receives uploaded files from a UI,
stores them on the backend host, and indexes the stored copies.

## Why Uploads Are Stored First

The search index library indexes files from disk. Snippet extraction also
reopens the original file later so it can extract matching text around a query.

Because the UI may run on a different host, the UI cannot safely send a local
path like `/Users/me/file.txt` and expect the backend to read it. Instead, the
backend receives the real file bytes, saves them under `UPLOAD_DIR`, and indexes
that backend-local copy.

## Endpoint

File uploads use the same endpoint as document indexing:

```text
POST /api/v1/documents
```

The handler chooses the upload path when the request `Content-Type` starts with:

```text
multipart/form-data
```

Relevant code:

- `internal/httpapi/handlers.go`
  - `addDocument`
  - `addUploadedDocuments`
- `internal/search/service.go`
  - `AddUploadedDocument`
  - `saveUpload`

## Request Shape

Upload one file with the `file` field:

```sh
curl -X POST http://localhost:8080/api/v1/documents \
  -F 'file=@/home/stac/dev-learning/data/web-crawler.txt'
```

Upload multiple files with repeated `files` fields:

```sh
curl -X POST http://localhost:8080/api/v1/documents \
  -F 'files=@/home/stac/dev-learning/data/web-crawler.txt' \
  -F 'files=@/home/stac/dev-learning/data/search-engines.txt'
```

The handler accepts both field names:

```go
files := r.MultipartForm.File["files"]
files = append(files, r.MultipartForm.File["file"]...)
```

This keeps the API convenient for both single-file and multi-file UI forms.

## Handler Flow

### 1. Detect Multipart Upload

`addDocument` is the entry point for `POST /api/v1/documents`.

It first checks the request content type:

```go
if strings.HasPrefix(strings.ToLower(r.Header.Get("Content-Type")), "multipart/form-data") {
    h.addUploadedDocuments(w, r)
    return
}
```

If the request is multipart, the upload handler runs. Otherwise, the existing
JSON path-based flow runs.

### 2. Limit and Parse the Request

`addUploadedDocuments` caps the request body:

```go
r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
```

Current limits:

```go
maxUploadMemory = 32 << 20  // 32 MB kept in memory before temp files
maxUploadSize   = 100 << 20 // 100 MB total request size
```

Then it parses the multipart form:

```go
if err := r.ParseMultipartForm(maxUploadMemory); err != nil {
    writeError(w, http.StatusBadRequest, fmt.Errorf("parse multipart form: %w", err))
    return
}
defer r.MultipartForm.RemoveAll()
```

`RemoveAll` cleans up temporary files created by Go while parsing large
multipart requests.

### 3. Collect Uploaded Files

The handler reads files from both supported field names:

```go
files := r.MultipartForm.File["files"]
files = append(files, r.MultipartForm.File["file"]...)
```

If neither field exists, the request fails:

```json
{
  "ok": false,
  "error": "multipart file field is required"
}
```

### 4. Store and Index Each File

For each uploaded file:

```go
file, err := header.Open()
doc, err := h.service.AddUploadedDocument(header.Filename, file)
```

The HTTP handler does not know how indexing works. It only opens the uploaded
file stream and passes it to the search service.

## Service Flow

`AddUploadedDocument` owns the backend storage and indexing sequence.

### 1. Lock the Index

The service uses a write lock:

```go
s.mu.Lock()
defer s.mu.Unlock()
```

This prevents concurrent writes from corrupting the in-memory index or the
persisted `search.idx` file.

### 2. Save the Upload

The upload is copied into `UPLOAD_DIR`:

```go
path, err := s.saveUpload(filename, src)
```

`saveUpload` creates the upload directory when needed:

```go
os.MkdirAll(s.uploadDir, 0o755)
```

It sanitizes the original filename with `safeUploadName`, then creates a unique
file:

```go
file, err := os.CreateTemp(s.uploadDir, pattern)
```

This avoids trusting user-provided paths and prevents filename collisions.

### 3. Index the Stored Copy

After the file exists on the backend host, the service indexes that stored path:

```go
if err := s.idx.AddDocument(path); err != nil {
    _ = os.Remove(path)
    return Document{}, fmt.Errorf("add uploaded document: %w", err)
}
```

If indexing fails, the stored upload is removed so the backend does not keep an
orphaned file.

### 4. Persist the Index

The service saves the updated index:

```go
if err := s.saveLocked(); err != nil {
    return Document{}, err
}
```

This writes the current in-memory index to `SEARCH_INDEX_PATH`.

### 5. Return the Indexed Document

Finally, the service finds the newly indexed document by its stored path and
returns it to the handler:

```go
doc, ok := s.findDocumentByPathLocked(path)
```

The handler returns the documents in the API response.

## Response Shape

Successful upload response:

```json
{
  "ok": true,
  "data": [
    {
      "ID": 1,
      "FilePath": "uploads/web-crawler-1780183276871776550-2245764232.txt",
      "Length": 3797
    }
  ]
}
```

`FilePath` is the backend-stored file path, not the user's original local path.
This is intentional: the index needs a path the backend can reopen later.

## Search After Upload

Once a file has been uploaded and indexed, search works the same way as before:

```sh
curl 'http://localhost:8080/api/v1/search?q=distributed'
```

The result can include snippets:

```json
{
  "Doc": {
    "ID": 1,
    "FilePath": "uploads/web-crawler-1780183276871776550-2245764232.txt",
    "Length": 3797
  },
  "Snippets": [
    {
      "Text": "a parallelization policy that states how to coordinate distributed web crawlers.",
      "Matches": [
        {
          "Start": 55,
          "End": 66,
          "Term": "distributed"
        }
      ]
    }
  ],
  "Score": 0.000747499540889426
}
```

The UI should render `Snippet.Text` and highlight each range in
`Snippet.Matches`.

## JSON Path Indexing Still Exists

The same endpoint still supports backend-local indexing with JSON:

```sh
curl -X POST http://localhost:8080/api/v1/documents \
  -H 'Content-Type: application/json' \
  -d '{"path":"../data/web-crawler.txt"}'
```

Use this only when the file already exists on the backend host. For UI uploads,
prefer multipart file upload.

## Configuration

Relevant environment variables:

```sh
SEARCH_INDEX_PATH=search.idx
UPLOAD_DIR=uploads
```

`SEARCH_INDEX_PATH` stores the persisted inverted index.

`UPLOAD_DIR` stores uploaded source files so snippets can reopen them later.

Both paths should point to durable storage in production.

## Important Notes

- Multipart uploads are capped at `100 MB` per request.
- The original uploaded filename is sanitized and made unique before storage.
- Uploaded files are not deleted when removing documents yet; removal currently
  deletes the index entry but does not clean up the stored source file.
- Snippets will be empty if the indexed file path is missing or unreadable.
- The old JSON path flow is useful for backend-local batch indexing, but it is
  not suitable for files that only exist on a UI user's machine.
