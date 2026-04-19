# HTMX + Templ Migration Design

## Goal

Replace full-page reloads with HTMX partial navigation and migrate from `html/template` to templ for type-safe, component-based rendering. Three outcomes:

1. **Snappy navigation** — clicking a note swaps only the content area and TOC, no page flash
2. **Reduced server work** — HTMX requests skip tree building, manifest serialization, and layout rendering
3. **Code quality** — typed templ components replace monolithic `pageData` + stringly-typed templates

## Current State

- 6 HTML templates (~200 lines total) rendered via `html/template`
- Every route renders the full `layout.html`, rebuilding sidebar tree (filesystem walk + DB lookup) and serializing a ~150KB manifest JSON
- `htmx.min.js` is bundled but unused
- Client-side filtering (sidebar + command palette) against `__ZK_MANIFEST` — fast, works well for ~900 notes
- Search endpoint already returns partials (no layout wrapper)

## Component Architecture

Templ components replace the monolithic `pageData` struct. Each takes only the props it needs:

```
Layout(title, manifestJSON, activePath)
├── Sidebar(tree, tags, activePath)
│   ├── Tree(nodes, activePath)          // recursive
│   └── TagCloud(tags)
├── ContentCol(...)                       // HTMX swap target: #content-col
│   ├── Breadcrumb(segments, currentTitle)
│   └── NoteArticle(note, html)           // or FolderListing / EmptyState
└── TOCPanel(headings, outgoing, backlinks)  // OOB swap target: #toc-panel
```

`ContentCol` and `TOCPanel` are renderable independently — they serve as both full-page parts and standalone HTMX partials. `Layout` and `Sidebar` only render on full page loads.

## HTMX Request/Response Flow

### Navigation Links

All navigable links (sidebar tree, backlinks, breadcrumbs) get HTMX attributes:

```html
<a href="/note/cooking/pasta"
   hx-get="/note/cooking/pasta"
   hx-target="#content-col"
   hx-push-url="true">pasta</a>
```

`href` is the fallback for first visit / JS-disabled.

### Handler Logic

Handlers check the `HX-Request` header to decide what to render:

```go
func (s *Server) handleNote(w http.ResponseWriter, r *http.Request) {
    // ... fetch note, render markdown, get links ...

    if r.Header.Get("HX-Request") != "" {
        // Partial: content + TOC as OOB
        views.ContentCol(breadcrumbs, note, html).Render(ctx, w)
        views.TOCPanel(headings, outgoing, backlinks).Render(ctx, w)
        return
    }
    // Full page: build tree, serialize manifest, render everything
    views.Layout(title, manifest, sidebar, content, toc).Render(ctx, w)
}
```

### Response Shape (HTMX)

```html
<!-- Primary swap into #content-col -->
<div id="content-col">
  <nav id="breadcrumb">...</nav>
  <div id="content-area"><article>...</article></div>
</div>

<!-- OOB swap replaces #toc-panel -->
<aside id="toc-panel" hx-swap-oob="true">
  ...headings, links, backlinks...
</aside>
```

### What the Server Skips on HTMX Requests

- Filesystem walk / tree build (`buildTree`)
- Manifest JSON serialization
- Sidebar rendering
- Full layout rendering

This is the main performance win.

## Client-Side After-Swap Hooks

After HTMX swaps content, several things need re-initialization via `htmx:afterSwap`:

### Tree Active State

Read the new URL, toggle classes on the existing sidebar tree DOM:

- Remove `.is-active` from old link
- Add `.is-active` to new link
- Ensure parent `<details>` elements are open

No server round-trip — pure DOM manipulation.

### Mermaid Diagrams

Re-run on new content:

```js
mermaid.run({ nodes: document.querySelectorAll('#content-area .mermaid') });
```

### TOC Scroll Observer

Tear down the old IntersectionObserver, set up a new one for the fresh heading elements. `toc.js` needs refactoring into `initTOC()` / `destroyTOC()` pair.

### Progress Bar

Reset scroll tracking for the new article.

### What Needs No Re-init

These persist across navigations without any work:

- Sidebar filter / tag state (stays in JS memory)
- Command palette (manifest stays loaded)
- Resize handles (attached to persistent DOM elements)
- Theme state

## What Stays Client-Side

- **Sidebar filtering** — client-side against `__ZK_MANIFEST`. With HTMX partial navigation the page never fully reloads, so the manifest loads once and persists in memory. Instant filtering, no round-trips.
- **Command palette** — same manifest, same client-side search.
- **Theme toggle** — localStorage + `data-theme` attribute.
- **Resize handles** — DOM manipulation + localStorage persistence.

## Templ Migration Details

### File Structure

```
internal/server/
├── views/
│   ├── layout.templ      // Layout, head, topbar, scripts
│   ├── sidebar.templ     // Sidebar, Tree (recursive), TagCloud
│   ├── content.templ     // ContentCol, Breadcrumb, NoteArticle, FolderListing, EmptyState
│   ├── toc.templ         // TOCPanel (with hx-swap-oob attr)
│   └── search.templ      // SearchResults list
├── handlers.go
├── server.go
└── static/               // unchanged, still //go:embed
```

Generated `*_templ.go` files are committed to the repo so `go build` works without templ installed.

### Build Changes

```makefile
generate: assets
	templ generate ./internal/server/views/

build: generate
	go build -o bin/zk-serve ./cmd/zk-serve
```

### What Gets Removed

- `internal/server/templates/` directory (all 6 `.html` files)
- `//go:embed templates/*` in server.go
- `template.FuncMap` setup (`fmtDate`, `mul`, `safeSnippet`)
- The `pageData` struct
- `template.Must(template.ParseFS(...))` parsing at startup

### Template Function Equivalents

| html/template | templ |
|---|---|
| `fmtDate` | `{ note.Created.Format("2006-01-02") }` |
| `safeSnippet` | `@templ.Raw(snippet)` |
| `mul` | Inline Go expression |

### Recursive Tree

Templ supports recursion natively:

```
templ TreeNode(node *FileNode, activePath string) {
    if node.IsDir {
        <details open?={ node.IsOpen }>
            for _, child := range node.Children {
                @TreeNode(child, activePath)
            }
        </details>
    } else {
        <a class={ "tree-link", templ.KV("is-active", node.Path == activePath) }
           href={ templ.SafeURL("/note/" + node.Path) }
           hx-get={ "/note/" + node.Path }
           hx-target="#content-col"
           hx-push-url="true">{ node.Name }</a>
    }
}
```

## Edge Cases

### Back/Forward Buttons

HTMX handles popstate events automatically when `hx-push-url` is used. The browser back button triggers a re-fetch of the previous URL as a partial request. The `htmx:afterSwap` hooks fire on these too.

### Index Route (`/`)

- Full request: Layout with EmptyState + empty TOC
- HTMX request (unlikely — no link targets `/` with hx-get): return EmptyState + empty TOC OOB

### Folder Route (`/folder/{path}`)

Same HTMX pattern as notes. Returns ContentCol with FolderListing + empty TOC (no headings/links for folders). Folder links in the tree get the same `hx-get` / `hx-target` attributes.

### JS Disabled / First Visit

Every route still renders a full page via the non-HTMX path. Links have real `href` attributes. The app degrades to full-page navigation — exactly how it works today.

### Syntax Highlighting

Chroma CSS is a static stylesheet already loaded in `<head>`. Swapped-in code blocks are styled immediately with no extra work.

## Decision Summary

| Aspect | Decision |
|---|---|
| Template engine | Templ (full migration) |
| Partial navigation | HTMX `hx-get` + `hx-target="#content-col"` + `hx-push-url` |
| TOC updates | OOB swap (`hx-swap-oob="true"` on `#toc-panel`) |
| Sidebar tree | Persists across navigations, active state updated client-side |
| Sidebar filtering | Client-side (manifest loaded once on full page load) |
| Command palette | Client-side (unchanged) |
| Full vs partial | `HX-Request` header; first visit = full SSR, subsequent = partial |
| Back/forward | HTMX popstate (built-in) |
| After-swap hooks | Mermaid, TOC observer, progress bar, tree active state |
| Build | `templ generate` before `go build` |
| Generated code | Committed to repo |
| File structure | `internal/server/views/*.templ` replaces `templates/*.html` |
