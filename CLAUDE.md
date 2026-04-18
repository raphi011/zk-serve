# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
make assets          # Download HTMX 2.0.4 + Mermaid 11 into internal/server/static/
make build           # Build binary → bin/zk-serve (runs assets first)
make test            # go test ./...

# Run a specific package's tests
go test ./internal/zk/...      -v
go test ./internal/render/...  -v
go test ./internal/server/...  -v

# Run the server
./bin/zk-serve --notebook ~/path/to/notes --addr :8080 --open
```

`htmx.min.js` and `mermaid.min.js` are downloaded at build time and must exist before the binary is built (they are embedded via `//go:embed`).

## Architecture

Three internal packages with a clear layered dependency: `server` → `render` + `zk`.

### `internal/zk` — notebook client
Shells out to the `zk` CLI (`exec.Command` in the notebook directory) and parses JSON output. Exposes `Client.List(query, tags)` and `Client.TagList()`. The server depends on `ClientInterface` (not the concrete type) so handlers can be tested with a stub.

### `internal/render` — Markdown pipeline
Single public function `Markdown(src []byte, lookup map[string]string) (string, error)`. The lookup map is `stem → absPath`, built from the full note list and used to resolve `[[wiki links]]`. The Goldmark pipeline runs these transforms in order:
1. Strip YAML frontmatter
2. Strip first `<h1>` (already in page header)
3. Convert `` ```mermaid `` fences → `<pre class="mermaid">` nodes (bypasses Chroma)
4. Render GFM, syntax highlighting (Chroma/Dracula), and wiki links

### `internal/server` — HTTP + templates
`Server` embeds templates and static assets from the filesystem at compile time. At startup it builds two Chroma stylesheets (dark/light) and merges them under `[data-theme="light"]` scoping, served as `/static/chroma.css`.

Routes:
- `GET /` — full page, all notes in sidebar
- `GET /search?q=&tags=` — HTMX partial replacing the note list
- `GET /note/{path...}` — full page with rendered note
- `GET /tags` — JSON for tag filter UI
- `GET /static/*` — embedded assets

### `cmd/zk-serve`
Cobra entry point. Validates the `zk` binary is on PATH before starting the server.

## Key conventions
- Templates are embedded (`//go:embed templates/* static/*`) and parsed once at startup.
- `pageData` is the single template context struct; `IsActiveTag` / `IsActiveNote` are helpers used in templates.
- Chroma CSS is generated programmatically at runtime (not a static file on disk).
- HTMX search is debounced 300 ms via `hx-trigger="input changed delay:300ms"`.
