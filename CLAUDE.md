# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
make assets          # Download HTMX 2.0.4 + Mermaid 11 into internal/server/static/
make generate        # Run templ generate for .templ → .go
make build           # Build binary → bin/zk-serve (runs assets + generate first)
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

Four internal packages: `server` → `views` + `model` + `render` + `zk`.

### `internal/zk` — notebook data access
Queries zk's SQLite database directly via `modernc.org/sqlite` (pure-Go, no CGO). `DB` type wraps `*sql.DB` opened read-only against `notebook.db`. Exposes `DB.AllNotes()`, `DB.AllTags()`, and `DB.Search(q, tags)` (FTS5 full-text search with BM25 ranking). `ConvertQuery` transforms Google-like search syntax into FTS5 MATCH expressions. The server depends on the `Store` interface so handlers can be tested with a stub. The `zk` binary is only needed for indexing (`zk index`), not at runtime.

### `internal/render` — Markdown pipeline
Single public function `Markdown(src []byte, lookup map[string]string) (string, error)`. The lookup map is `stem → absPath`, built from the full note list and used to resolve `[[wiki links]]`. The Goldmark pipeline runs these transforms in order:
1. Strip YAML frontmatter
2. Strip first `<h1>` (already in page header)
3. Convert `` ```mermaid `` fences → `<pre class="mermaid">` nodes (bypasses Chroma)
4. Render GFM, syntax highlighting (Chroma/Dracula), and wiki links

### `internal/model` — shared types
`FileNode`, `BreadcrumbSegment`, `FolderEntry` — used by both `server` (handlers) and `views` (templ components). Extracted to break the circular dependency.

### `internal/server` — HTTP handlers
`Server` embeds static assets at compile time. At startup it builds two Chroma stylesheets (dark/light) scoped by `[data-theme]`, served as `/static/chroma.css`. Handlers detect `HX-Request` header: full requests render the complete layout, HTMX requests return only `#content-col` + `#toc-panel` (OOB swap).

Routes:
- `GET /` — full page (or HTMX partial: empty content + empty TOC)
- `GET /search?q=&tags=` — partial replacing the note list
- `GET /note/{path...}` — full page or HTMX partial (content + TOC OOB)
- `GET /folder/{path...}` — full page or HTMX partial (content + empty TOC OOB)
- `GET /tags` — JSON for tag filter UI
- `GET /static/*` — embedded assets

### `internal/server/views` — templ components
Type-safe templ components replacing `html/template`. Each component takes only the props it needs. Key components: `Layout`, `Sidebar`, `Tree`, `NoteContentCol`, `FolderContentCol`, `TOCPanel`. Run `templ generate ./internal/server/views/` (or `make generate`) after editing `.templ` files.

### `cmd/zk-serve`
Cobra entry point. Opens `notebook.db` via `zk.OpenDB` and passes it to the server.

## Key conventions
- HTMX powers partial navigation: note/folder clicks swap `#content-col` + `#toc-panel` (OOB) without full page reloads. Server detects `HX-Request` header.
- Templ components in `internal/server/views/` replace `html/template`. Run `templ generate` (or `make generate`) after editing `.templ` files. Generated `*_templ.go` files are committed.
- Static assets are embedded (`//go:embed static`).
- Chroma CSS is generated programmatically at runtime (not a static file on disk).
- Client-side JS handles: sidebar filtering (against `__ZK_MANIFEST`), command palette, tree active state updates after HTMX swap, TOC re-init, mermaid re-run.
