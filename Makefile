.PHONY: assets generate bundle build test

HTMX_URL    := https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js
MERMAID_URL := https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.min.js
STATIC      := internal/server/static

assets:
	curl -fsSL $(HTMX_URL)    -o $(STATIC)/htmx.min.js
	curl -fsSL $(MERMAID_URL) -o $(STATIC)/mermaid.min.js

generate:
	templ generate ./internal/server/views/

bundle:
	npx esbuild $(STATIC)/js/app.js --bundle --minify --format=iife --outfile=$(STATIC)/app.min.js

build: assets generate bundle
	go build -o bin/zk-serve ./cmd/zk-serve

test:
	go test ./...
