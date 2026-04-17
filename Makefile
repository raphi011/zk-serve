.PHONY: assets build test

HTMX_URL    := https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js
MERMAID_URL := https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.min.js
STATIC      := internal/server/static

assets:
	curl -fsSL $(HTMX_URL)    -o $(STATIC)/htmx.min.js
	curl -fsSL $(MERMAID_URL) -o $(STATIC)/mermaid.min.js

build: assets
	go build -o bin/zk-serve ./cmd/zk-serve

test:
	go test ./...
