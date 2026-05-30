# Search Index Core

HTTP API for the `github.com/my-search-index/search-index` library.

## Run

```sh
go run ./cmd/search-index-core
```

Configuration:

```sh
PORT=8080
SEARCH_INDEX_PATH=search.idx
```

## Endpoints

```text
GET    /healthz
GET    /api/v1/documents
POST   /api/v1/documents
DELETE /api/v1/documents?path=<file>
GET    /api/v1/search?q=<query>
```

Add one file:

```sh
curl -X POST http://localhost:8080/api/v1/documents \
  -H 'Content-Type: application/json' \
  -d '{"path":"../data/web-crawler.txt"}'
```

Add a directory:

```sh
curl -X POST http://localhost:8080/api/v1/documents \
  -H 'Content-Type: application/json' \
  -d '{"path":"../data","directory":true,"extensions":[".txt",".md"]}'
```

Search:

```sh
curl 'http://localhost:8080/api/v1/search?q=distributed%20computing'
```
