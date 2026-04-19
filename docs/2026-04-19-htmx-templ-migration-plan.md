# HTMX + Templ Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace full-page reloads with HTMX partial navigation and migrate from `html/template` to templ components.

**Architecture:** Templ components replace 6 HTML templates and the monolithic `pageData` struct. Handlers detect `HX-Request` header — full requests render the complete layout, HTMX requests return only `#content-col` + `#toc-panel` (OOB swap). Sidebar persists across navigations; tree active state is updated client-side.

**Tech Stack:** Go, templ, HTMX 2.0.4, Goldmark, Chroma

**Design spec:** `docs/2026-04-19-htmx-templ-migration-design.md`

---

## File Map

### New files

| File | Responsibility |
|---|---|
| `internal/server/views/layout.templ` | Full page shell: DOCTYPE, head, topbar, sidebar, main wrapper, scripts, command palette |
| `internal/server/views/sidebar.templ` | Sidebar component, recursive TreeNode, TagCloud |
| `internal/server/views/content.templ` | ContentCol, Breadcrumb, NoteArticle, FolderListing, EmptyState |
| `internal/server/views/toc.templ` | TOCPanel (with OOB swap attr for HTMX) |
| `internal/server/views/search.templ` | SearchResults list partial |
| `internal/server/static/js/htmx-hooks.js` | After-swap hooks: tree active state, mermaid, TOC re-init, progress bar |

### Modified files

| File | What changes |
|---|---|
| `internal/server/server.go` | Remove template parsing, `templateFuncs`, `templateFS` embed, `tmpl` field. Keep `staticFS` embed, chroma CSS, routes. |
| `internal/server/handlers.go` | Replace `renderTemplate` calls with templ component renders. Add `HX-Request` detection. Remove `pageData` struct. Keep `buildTree`, `buildBreadcrumbs`, `buildManifest`, `FileNode`, `FolderEntry`, `BreadcrumbSegment`. |
| `internal/server/handlers_test.go` | Update tests to work with templ output. Add HTMX partial response tests. |
| `internal/server/static/js/app.js` | Import and init `htmx-hooks.js` |
| `internal/server/static/js/toc.js` | Refactor into `initToc()` / `destroyToc()` pair, export `destroyToc` |
| `internal/server/static/js/command-palette.js` | Use HTMX navigation instead of `window.location.href` |
| `Makefile` | Add `generate` target with `templ generate` |
| `go.mod` | Add `github.com/a-h/templ` dependency |

### Deleted files

| File | Reason |
|---|---|
| `internal/server/templates/layout.html` | Replaced by `views/layout.templ` |
| `internal/server/templates/note.html` | Replaced by `views/content.templ` |
| `internal/server/templates/toc.html` | Replaced by `views/toc.templ` |
| `internal/server/templates/tree.html` | Replaced by `views/sidebar.templ` |
| `internal/server/templates/folder.html` | Replaced by `views/content.templ` |
| `internal/server/templates/list.html` | Replaced by `views/search.templ` |

---

## Task 1: Add templ dependency and build target

**Files:**
- Modify: `go.mod`
- Modify: `Makefile`

- [ ] **Step 1: Install templ CLI**

```bash
go install github.com/a-h/templ/cmd/templ@latest
```

- [ ] **Step 2: Add templ module dependency**

```bash
go get github.com/a-h/templ@latest
```

- [ ] **Step 3: Create views directory**

```bash
mkdir -p internal/server/views
```

- [ ] **Step 4: Add generate target to Makefile**

Replace the current Makefile with:

```makefile
.PHONY: assets generate build test

HTMX_URL    := https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js
MERMAID_URL := https://cdn.jsdelivr.net/npm/mermaid@11/dist/mermaid.min.js
STATIC      := internal/server/static

assets:
	curl -fsSL $(HTMX_URL)    -o $(STATIC)/htmx.min.js
	curl -fsSL $(MERMAID_URL) -o $(STATIC)/mermaid.min.js

generate:
	templ generate ./internal/server/views/

build: assets generate
	go build -o bin/zk-serve ./cmd/zk-serve

test:
	go test ./...
```

- [ ] **Step 5: Create a minimal placeholder templ file to verify the toolchain**

Create `internal/server/views/layout.templ`:

```templ
package views

templ Placeholder() {
	<p>templ works</p>
}
```

- [ ] **Step 6: Run templ generate and verify it produces a .go file**

```bash
templ generate ./internal/server/views/
```

Expected: `internal/server/views/layout_templ.go` is created.

- [ ] **Step 7: Run `go build` to verify everything compiles**

```bash
make build
```

Expected: builds successfully.

- [ ] **Step 8: Commit**

```bash
git add go.mod go.sum Makefile internal/server/views/
git commit -m "build: add templ dependency and generate target"
```

---

## Task 2: Create sidebar templ components (Tree + TagCloud)

**Files:**
- Create: `internal/server/views/sidebar.templ`

These components translate `tree.html` (lines 1-19 of `internal/server/templates/tree.html`). The tree link `<a>` tags include HTMX attributes for partial navigation.

- [ ] **Step 1: Write sidebar.templ**

Create `internal/server/views/sidebar.templ`:

```templ
package views

import (
	"fmt"

	"github.com/raphaelgruber/zk-serve/internal/server"
	"github.com/raphaelgruber/zk-serve/internal/zk"
)

templ TreeNode(node *server.FileNode) {
	if node.IsDir {
		<details open?={ node.IsOpen }>
			<summary class="tree-folder"><span class="tree-chevron">▸</span>{ node.Name }</summary>
			<div class="tree-children">
				for _, child := range node.Children {
					@TreeNode(child)
				}
			</div>
		</details>
	} else {
		<a
			class={ "tree-item", templ.KV("active", node.IsActive) }
			href={ templ.SafeURL("/note/" + node.Path) }
			hx-get={ "/note/" + node.Path }
			hx-target="#content-col"
			hx-push-url="true"
			data-path={ node.Path }
		>
			<span class="tree-file-dot"></span>{ node.Name }
		</a>
	}
}

templ Tree(nodes []*server.FileNode) {
	<div class="sidebar-section-label">Notes</div>
	for _, node := range nodes {
		@TreeNode(node)
	}
}

templ TagCloud(tags []zk.Tag) {
	<div class="resize-handle-v server-tree" data-resize-target="next"></div>
	<details class="sidebar-tags-section server-tree" open>
		<summary class="sidebar-section-label">
			Tags <span class="sidebar-tag-count">{ fmt.Sprint(len(tags)) }</span>
		</summary>
		<div class="sidebar-tags-body">
			for _, tag := range tags {
				<span class="tag-pill" data-tag={ tag.Name }>
					{ tag.Name } <span class="tag-count">{ fmt.Sprint(tag.NoteCount) }</span>
				</span>
			}
		</div>
	</details>
}

templ Sidebar(nodes []*server.FileNode, tags []zk.Tag) {
	<div id="sidebar-search">
		<input type="text" id="sidebar-filter" placeholder="Filter notes…" autocomplete="off"/>
	</div>
	<div id="active-filters"></div>
	<div id="sidebar-inner">
		if len(nodes) > 0 {
			<div class="server-tree">
				@Tree(nodes)
			</div>
		}
	</div>
	if len(tags) > 0 {
		@TagCloud(tags)
	}
}
```

Note: `zk.Tag` is used directly — its `Name` and `NoteCount` fields are exactly what the template needs. `server.FileNode` is already exported from `internal/server/handlers.go:40`.

- [ ] **Step 2: Run templ generate**

```bash
templ generate ./internal/server/views/
```

Expected: `sidebar_templ.go` generated without errors.

- [ ] **Step 3: Run go build to verify compilation**

```bash
go build ./internal/server/...
```

Expected: compiles. The components aren't called yet, but the types must resolve.

- [ ] **Step 4: Commit**

```bash
git add internal/server/views/sidebar.templ internal/server/views/sidebar_templ.go
git commit -m "feat: add sidebar templ components (Tree, TagCloud)"
```

---

## Task 3: Create content templ components (ContentCol, NoteArticle, FolderListing, EmptyState)

**Files:**
- Create: `internal/server/views/content.templ`

These components translate `note.html` (lines 1-28), `folder.html` (lines 1-19), the breadcrumb nav (layout.html lines 42-48), and the content-area wrapper (layout.html lines 49-54).

- [ ] **Step 1: Write content.templ**

Create `internal/server/views/content.templ`:

```templ
package views

import (
	"fmt"
	"strings"

	"github.com/raphaelgruber/zk-serve/internal/render"
	"github.com/raphaelgruber/zk-serve/internal/server"
	"github.com/raphaelgruber/zk-serve/internal/zk"
)

templ Breadcrumb(segments []server.BreadcrumbSegment, currentTitle string) {
	<nav id="breadcrumb">
		<a class="bc-seg" href="/">~</a>
		for _, seg := range segments {
			<span class="bc-sep">/</span>
			<a
				class="bc-seg"
				href={ templ.SafeURL("/folder/" + seg.FolderPath) }
				hx-get={ "/folder/" + seg.FolderPath }
				hx-target="#content-col"
				hx-push-url="true"
			>{ seg.Name }</a>
		}
		<span class="bc-sep">/</span>
		<span class="bc-seg current">{ currentTitle }</span>
	</nav>
}

templ NoteArticle(note *zk.Note, noteHTML string, backlinks []zk.Link, headings []render.Heading) {
	<details id="mob-toc-details" class="mob-toc">
		<summary class="mob-toc-toggle">On this page</summary>
		<div class="mob-toc-body">
			for _, h := range headings {
				<a class={ "toc-item", templ.KV("h3", h.Level == 3) } href={ templ.SafeURL("#" + h.ID) }>{ h.Text }</a>
			}
		</div>
	</details>
	<article id="article">
		<h1 id="article-title">
			if note.Title != "" {
				{ note.Title }
			} else {
				{ note.FilenameStem }
			}
		</h1>
		<div class="article-meta">
			<span class="article-meta-text">
				Created: { note.Created.Format("2006-01-02") } · Modified: { note.Modified.Format("2006-01-02") } · { fmt.Sprint(note.WordCount) } words
			</span>
			for _, tag := range note.Tags {
				<a class="meta-tag" href={ templ.SafeURL("/?tags=" + tag) }>{ tag }</a>
			}
		</div>
		<hr class="article-divider"/>
		<div class="prose">
			@templ.Raw(noteHTML)
		</div>
		if len(backlinks) > 0 {
			<section id="backlinks-section">
				<h4>Referenced by</h4>
				for _, link := range backlinks {
					<a
						class="backlink-card"
						href={ templ.SafeURL("/note/" + link.SourcePath) }
						hx-get={ "/note/" + link.SourcePath }
						hx-target="#content-col"
						hx-push-url="true"
					>
						<span class="backlink-card-title">{ link.SourceTitle }</span>
						if len(link.SourceTags) > 0 {
							<span class="backlink-card-tags">
								for _, tag := range link.SourceTags {
									<span class="backlink-tag">{ tag }</span>
								}
							</span>
						}
					</a>
				}
			</section>
		}
	</article>
}

templ FolderListing(folderName string, entries []server.FolderEntry) {
	<article id="article">
		<h1>{ folderName }<span style="color:var(--text-faint)">/</span></h1>
		<hr class="article-divider"/>
		if len(entries) > 0 {
			<ul class="folder-list">
				for _, entry := range entries {
					<li class="folder-entry">
						if entry.IsDir {
							<a
								class="folder-link folder-link--dir"
								href={ templ.SafeURL("/folder/" + entry.Path) }
								hx-get={ "/folder/" + entry.Path }
								hx-target="#content-col"
								hx-push-url="true"
							><span class="folder-icon">▸</span>{ entry.Name }/</a>
						} else {
							<a
								class="folder-link"
								href={ templ.SafeURL("/note/" + entry.Path) }
								hx-get={ "/note/" + entry.Path }
								hx-target="#content-col"
								hx-push-url="true"
							>
								<span class="folder-icon">◦</span>
								if entry.Title != "" {
									{ entry.Title }
								} else {
									{ entry.Name }
								}
							</a>
						}
					</li>
				}
			</ul>
		} else {
			<p class="folder-empty">Empty folder.</p>
		}
	</article>
}

// NoteContentCol renders #content-col for a note page.
templ NoteContentCol(segments []server.BreadcrumbSegment, note *zk.Note, noteHTML string, backlinks []zk.Link, headings []render.Heading) {
	<div id="content-col">
		if note.Title != "" {
			@Breadcrumb(segments, note.Title)
		} else {
			@Breadcrumb(segments, note.FilenameStem)
		}
		<div id="content-area">
			@NoteArticle(note, noteHTML, backlinks, headings)
		</div>
	</div>
}

// FolderContentCol renders #content-col for a folder page.
templ FolderContentCol(segments []server.BreadcrumbSegment, folderName string, entries []server.FolderEntry) {
	<div id="content-col">
		@Breadcrumb(segments, folderName)
		<div id="content-area">
			@FolderListing(folderName, entries)
		</div>
	</div>
}

// EmptyContentCol renders #content-col for the index page with no note selected.
templ EmptyContentCol() {
	<div id="content-col">
		<div id="content-area">
			<div class="content-empty">Select a note to read it.</div>
		</div>
	</div>
}
```

Note: the `strings` import may not be needed — remove if unused after generation.

- [ ] **Step 2: Run templ generate**

```bash
templ generate ./internal/server/views/
```

Expected: `content_templ.go` generated without errors.

- [ ] **Step 3: Run go build**

```bash
go build ./internal/server/...
```

Expected: compiles.

- [ ] **Step 4: Commit**

```bash
git add internal/server/views/content.templ internal/server/views/content_templ.go
git commit -m "feat: add content templ components (NoteArticle, FolderListing, Breadcrumb)"
```

---

## Task 4: Create TOC and search templ components

**Files:**
- Create: `internal/server/views/toc.templ`
- Create: `internal/server/views/search.templ`

TOC translates `toc.html` (lines 1-35). Search translates `list.html` (lines 1-14). The TOC component takes an `oob` boolean — when true it adds `hx-swap-oob="true"` for HTMX partial responses.

- [ ] **Step 1: Write toc.templ**

Create `internal/server/views/toc.templ`:

```templ
package views

import (
	"github.com/raphaelgruber/zk-serve/internal/render"
	"github.com/raphaelgruber/zk-serve/internal/zk"
)

// TOCPanel renders the right sidebar. When oob is true, adds hx-swap-oob
// so HTMX replaces the existing #toc-panel.
templ TOCPanel(headings []render.Heading, outgoing []zk.Link, backlinks []zk.Link, oob bool) {
	<aside
		id="toc-panel"
		if oob {
			hx-swap-oob="true"
		}
	>
		if len(headings) > 0 {
			<div id="toc-header"><span>On this page</span></div>
			<div id="toc-inner">
				for _, h := range headings {
					<a class={ "toc-item", templ.KV("h3", h.Level == 3) } href={ templ.SafeURL("#" + h.ID) }>{ h.Text }</a>
				}
			</div>
		}
		if len(outgoing) > 0 {
			<div class="resize-handle-v" data-resize-target="next"></div>
			<details class="toc-links-section" open>
				<summary class="toc-links-label">Links <span class="tl-count">{ lenStr(outgoing) }</span></summary>
				<div class="toc-links-body">
					for _, link := range outgoing {
						if link.IsExternal {
							<a class="toc-link-item toc-link-out" href={ templ.SafeURL(link.Href) } target="_blank" rel="noopener">
								if link.TargetTitle != "" {
									{ link.TargetTitle }
								} else {
									{ link.Href }
								}
							</a>
						} else if link.TargetPath != "" {
							<a
								class="toc-link-item toc-link-out"
								href={ templ.SafeURL("/note/" + link.TargetPath) }
								hx-get={ "/note/" + link.TargetPath }
								hx-target="#content-col"
								hx-push-url="true"
							>
								if link.TargetTitle != "" {
									{ link.TargetTitle }
								} else {
									{ link.Href }
								}
							</a>
						} else {
							<span class="toc-link-item toc-link-out">{ link.Href }</span>
						}
					}
				</div>
			</details>
		}
		if len(backlinks) > 0 {
			<div class="resize-handle-v" data-resize-target="next"></div>
			<details class="toc-links-section" open>
				<summary class="toc-links-label">Backlinks <span class="tl-count">{ lenStr(backlinks) }</span></summary>
				<div class="toc-links-body">
					for _, link := range backlinks {
						<a
							class="toc-link-item toc-link-in"
							href={ templ.SafeURL("/note/" + link.SourcePath) }
							hx-get={ "/note/" + link.SourcePath }
							hx-target="#content-col"
							hx-push-url="true"
						>{ link.SourceTitle }</a>
					}
				</div>
			</details>
		}
	</aside>
}
```

Add a helper at the top of the file (below imports):

```go
import "fmt"

func lenStr[T any](s []T) string {
	return fmt.Sprint(len(s))
}
```

Note: templ files support Go code blocks via `// ...` but helper functions should go in a separate `.go` file in the same package. Create `internal/server/views/helpers.go`:

```go
package views

import "fmt"

func lenStr[T any](s []T) string {
	return fmt.Sprint(len(s))
}
```

And use `{ lenStr(outgoing) }` in the templ file, importing from the same package (no import needed — same package).

- [ ] **Step 2: Write search.templ**

Create `internal/server/views/search.templ`:

```templ
package views

import (
	"github.com/raphaelgruber/zk-serve/internal/zk"
)

templ SearchResults(notes []zk.Note) {
	for _, note := range notes {
		<div class="result-item">
			<a class="result-link" href={ templ.SafeURL("/note/" + note.Path) }>
				<div class="result-title">
					if note.Title != "" {
						{ note.Title }
					} else {
						{ note.FilenameStem }
					}
				</div>
				<div class="result-meta">{ note.Modified.Format("2006-01-02") }</div>
				if note.Snippet != "" {
					<div class="result-snippet">
						@templ.Raw(safeSnippet(note.Snippet))
					</div>
				}
			</a>
			if len(note.Tags) > 0 {
				<div class="result-tags">
					for _, tag := range note.Tags {
						<span class="result-tag" data-tag={ tag }>{ tag }</span>
					}
				</div>
			}
		</div>
	} else {
		// templ doesn't have {{else}} for range — use an if check instead
	}
}

templ SearchEmpty() {
	<div class="sidebar-empty">No results</div>
}
```

Note: templ's `for` loop doesn't support `else`. Add the empty-state logic in the handler: if `len(notes) == 0`, render `SearchEmpty()` instead of `SearchResults()`.

Add `safeSnippet` to `internal/server/views/helpers.go`:

```go
package views

import (
	"fmt"
	"html"
	"strings"
)

func lenStr[T any](s []T) string {
	return fmt.Sprint(len(s))
}

func safeSnippet(s string) string {
	safe := html.EscapeString(s)
	safe = strings.ReplaceAll(safe, "⟪MARK_START⟫", "<mark>")
	safe = strings.ReplaceAll(safe, "⟪MARK_END⟫", "</mark>")
	return safe
}
```

- [ ] **Step 3: Run templ generate and go build**

```bash
templ generate ./internal/server/views/ && go build ./internal/server/...
```

Expected: compiles.

- [ ] **Step 4: Commit**

```bash
git add internal/server/views/
git commit -m "feat: add TOC and search templ components"
```

---

## Task 5: Create layout templ component

**Files:**
- Modify: `internal/server/views/layout.templ` (replace the placeholder from Task 1)

This translates `layout.html` (lines 1-79). The layout is only rendered on full page loads. It composes Sidebar, ContentCol, and TOCPanel.

- [ ] **Step 1: Write layout.templ**

Replace `internal/server/views/layout.templ` with:

```templ
package views

import (
	"github.com/raphaelgruber/zk-serve/internal/render"
	"github.com/raphaelgruber/zk-serve/internal/server"
	"github.com/raphaelgruber/zk-serve/internal/zk"
)

// LayoutParams holds all data needed for a full page render.
type LayoutParams struct {
	Title         string
	ManifestJSON  string
	Tree          []*server.FileNode
	Tags          []zk.Tag
	ContentCol    templ.Component
	Headings      []render.Heading
	OutgoingLinks []zk.Link
	Backlinks     []zk.Link
}

templ Layout(p LayoutParams) {
	<!DOCTYPE html>
	<html lang="en">
	<head>
		<meta charset="UTF-8"/>
		<meta name="viewport" content="width=device-width, initial-scale=1.0"/>
		<title>
			if p.Title != "" {
				{ p.Title } — zk
			} else {
				zk
			}
		</title>
		<link rel="stylesheet" href="/static/style.css"/>
		<link rel="stylesheet" href="/static/chroma.css"/>
		<script>
			(function(){var d=document.documentElement,s=d.style;var t=localStorage.getItem('zk-theme');d.setAttribute('data-theme',t||(window.matchMedia('(prefers-color-scheme:light)').matches?'light':'dark'));var sw=localStorage.getItem('zk-sidebar-width');if(sw)s.setProperty('--sidebar-width',sw+'px');var tw=localStorage.getItem('zk-toc-panel-width');if(tw)s.setProperty('--toc-width',tw+'px')})();
		</script>
	</head>
	<body>
		<div id="progress-bar"></div>
		<header id="topbar">
			<button id="mob-menu-btn" aria-label="Menu">☰</button>
			<a id="logo" href="/"><span class="logo-dot"></span><strong>zk</strong></a>
			<div id="topbar-spacer"></div>
			<button id="cmd-trigger" type="button"><span>Go to file, search…</span><kbd>⌘K</kbd></button>
			<button id="theme-toggle" type="button" aria-label="Toggle theme"><span id="theme-icon">☾</span></button>
		</header>
		<div id="sidebar-backdrop"></div>
		<div id="layout">
			<nav id="sidebar">
				@Sidebar(p.Tree, p.Tags)
			</nav>
			<div class="resize-handle" id="sidebar-resize"></div>
			<div id="main">
				@p.ContentCol
				<div class="resize-handle" id="toc-resize"></div>
				@TOCPanel(p.Headings, p.OutgoingLinks, p.Backlinks, false)
			</div>
		</div>
		<dialog id="cmd-dialog">
			<div id="cmd-box">
				<div id="cmd-input-row">
					<span class="cmd-icon-search">⌘</span>
					<input id="cmd-input" type="text" placeholder="Go to file, search notes…" autocomplete="off"/>
				</div>
				<div id="cmd-results"></div>
				<div id="cmd-footer">
					<span class="cmd-hint"><kbd>↑↓</kbd> navigate</span>
					<span class="cmd-hint"><kbd>↵</kbd> open</span>
					<span class="cmd-hint"><kbd>esc</kbd> close</span>
				</div>
			</div>
		</dialog>
		@templ.Raw("<script>window.__ZK_MANIFEST = " + p.ManifestJSON + ";</script>")
		<script src="/static/htmx.min.js"></script>
		<script type="module" src="/static/js/app.js"></script>
		<script src="/static/mermaid.min.js"></script>
		<script>
			if(window.mermaid){mermaid.initialize({startOnLoad:false,theme:'dark'});mermaid.run({nodes:document.querySelectorAll('.mermaid')});}
		</script>
	</body>
	</html>
}
```

Important notes:
- The manifest JSON is injected as raw HTML via `templ.Raw()` since `ManifestJSON` is already a valid JSON string from `buildManifestJSON()`.
- `<script src="/static/htmx.min.js"></script>` is now loaded — it was bundled but never included before.
- `oob: false` is passed to TOCPanel for full page renders (OOB is only for HTMX partials).

- [ ] **Step 2: Run templ generate and go build**

```bash
templ generate ./internal/server/views/ && go build ./internal/server/...
```

Expected: compiles. If there are import issues with circular deps (`views` importing `server` for `FileNode`), see Task 6 for the fix.

- [ ] **Step 3: Commit**

```bash
git add internal/server/views/layout.templ internal/server/views/layout_templ.go
git commit -m "feat: add layout templ component"
```

---

## Task 6: Wire handlers to templ components (replace html/template)

**Files:**
- Modify: `internal/server/server.go`
- Modify: `internal/server/handlers.go`

This is the core switchover. Handlers call templ components instead of `renderTemplate`. The `pageData` struct and `renderTemplate` function are removed. `HX-Request` detection is added.

**Circular dependency note:** The `views` package imports `server.FileNode` and `server.BreadcrumbSegment`. This creates a circular dependency since `server` will also import `views`. Fix: move `FileNode`, `BreadcrumbSegment`, `FolderEntry`, `buildTree`, and `buildBreadcrumbs` to a shared package, or move them into `views`. The cleanest option is to keep them in `server` and have the handler code call templ components via their full path. Since `views` imports `server` types but `server` imports `views` components, this IS circular.

**Resolution:** Move the shared types to a new file `internal/server/types.go` in the same `server` package — no circular dep since `views` imports `server` and `server` imports `views` is the problem. 

Actually, the simplest fix: move the view types (`FileNode`, `BreadcrumbSegment`, `FolderEntry`) into the `views` package itself. Then `server/handlers.go` imports `views` (one direction only).

Alternative: extract types into `internal/model/types.go`. Both `server` and `views` import `model`.

**We'll use the model package approach** — it's clean and avoids putting handler logic in the views package.

- [ ] **Step 1: Create internal/model/types.go with shared types**

Create `internal/model/types.go`:

```go
package model

// FileNode is one entry in the sidebar folder tree.
type FileNode struct {
	Name     string
	Path     string // non-empty for notes
	IsDir    bool
	IsActive bool
	IsOpen   bool // dir should render <details open>
	Children []*FileNode
}

// BreadcrumbSegment is one folder step in a note's path.
type BreadcrumbSegment struct {
	Name       string
	FolderPath string // e.g. "notes" or "notes/ai"
}

// FolderEntry is one item (file or subdirectory) in a folder listing.
type FolderEntry struct {
	Name  string
	Path  string
	Title string
	IsDir bool
}
```

- [ ] **Step 2: Update views/*.templ to import from model instead of server**

In all `.templ` files, replace:
- `"github.com/raphaelgruber/zk-serve/internal/server"` → `"github.com/raphaelgruber/zk-serve/internal/model"`
- `server.FileNode` → `model.FileNode`
- `server.BreadcrumbSegment` → `model.BreadcrumbSegment`
- `server.FolderEntry` → `model.FolderEntry`

- [ ] **Step 3: Update handlers.go to use model types and remove duplicates**

In `internal/server/handlers.go`:
- Remove the `BreadcrumbSegment`, `FileNode`, `FolderEntry` type definitions
- Add import: `"github.com/raphaelgruber/zk-serve/internal/model"`
- Change `buildTree` return type to `[]*model.FileNode`
- Change `buildBreadcrumbs` return type to `[]model.BreadcrumbSegment`
- Remove the `pageData` struct
- Remove the `renderTemplate` function
- Remove the `manifestEntry` type and `buildManifest` function (move to a simpler JSON helper)

- [ ] **Step 4: Update server.go — remove template machinery**

In `internal/server/server.go`:
- Remove `//go:embed templates` and `var templateFS embed.FS`
- Remove `var templateFuncs = template.FuncMap{...}`
- Remove the `tmpl *template.Template` field from `Server` struct
- Remove the template parsing block in `New()` (lines 65-70)
- Remove imports: `"html/template"`, `"html"`, `"time"`, `"strings"` (if no longer needed)
- Keep: `staticFS` embed, chroma CSS, routes, `Store` interface

The `Server` struct becomes:

```go
type Server struct {
	mux         *http.ServeMux
	store       Store
	chromaDark  []byte
	chromaLight []byte
}
```

`New()` becomes:

```go
func New(store Store) (*Server, error) {
	dark, err := buildChromaCSS("dracula")
	if err != nil {
		return nil, fmt.Errorf("chroma dark css: %w", err)
	}
	light, err := buildChromaCSS("github")
	if err != nil {
		return nil, fmt.Errorf("chroma light css: %w", err)
	}
	s := &Server{
		mux:         http.NewServeMux(),
		store:       store,
		chromaDark:  dark,
		chromaLight: light,
	}
	s.registerRoutes()
	return s, nil
}
```

- [ ] **Step 5: Rewrite handleIndex**

```go
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	tags, err := s.store.AllTags()
	if err != nil {
		http.Error(w, "failed to list tags: "+err.Error(), http.StatusInternalServerError)
		return
	}
	notes, err := s.store.AllNotes()
	if err != nil {
		http.Error(w, "failed to list notes: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if isHTMX(r) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		views.EmptyContentCol().Render(r.Context(), w)
		views.TOCPanel(nil, nil, nil, true).Render(r.Context(), w)
		return
	}

	manifest := buildManifestJSON(notes)
	views.Layout(views.LayoutParams{
		Tags:         tags,
		Tree:         buildTree(s.store.NotebookPath(), notes, ""),
		ManifestJSON: manifest,
		ContentCol:   views.EmptyContentCol(),
	}).Render(r.Context(), w)
}
```

- [ ] **Step 6: Rewrite handleNote with HX-Request detection**

```go
func (s *Server) handleNote(w http.ResponseWriter, r *http.Request) {
	notePath := r.PathValue("path")
	if notePath == "" {
		http.NotFound(w, r)
		return
	}
	notes, err := s.store.AllNotes()
	if err != nil {
		http.Error(w, "failed to list notes: "+err.Error(), http.StatusInternalServerError)
		return
	}
	var note *zk.Note
	for i := range notes {
		if notes[i].Path == notePath {
			note = &notes[i]
			break
		}
	}
	if note == nil {
		absPath := filepath.Join(s.store.NotebookPath(), notePath)
		if _, err := os.Stat(absPath); err != nil {
			http.NotFound(w, r)
			return
		}
		stem := strings.TrimSuffix(filepath.Base(notePath), ".md")
		note = &zk.Note{
			Path: notePath, AbsPath: absPath,
			Filename: filepath.Base(notePath), FilenameStem: stem, Title: stem,
		}
	}
	raw, err := os.ReadFile(note.AbsPath)
	if err != nil {
		http.Error(w, "failed to read note: "+err.Error(), http.StatusInternalServerError)
		return
	}
	lookup := make(map[string]string, len(notes)*2)
	for _, n := range notes {
		lookup[n.FilenameStem] = n.Path
		lookup[strings.TrimSuffix(n.Path, ".md")] = n.Path
	}
	result, err := render.Markdown(raw, lookup)
	if err != nil {
		http.Error(w, "failed to render note: "+err.Error(), http.StatusInternalServerError)
		return
	}
	outLinks, _ := s.store.OutgoingLinks(notePath)
	backlinks, _ := s.store.Backlinks(notePath)
	breadcrumbs := buildBreadcrumbs(notePath)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if isHTMX(r) {
		views.NoteContentCol(breadcrumbs, note, result.HTML, backlinks, result.Headings).Render(r.Context(), w)
		views.TOCPanel(result.Headings, outLinks, backlinks, true).Render(r.Context(), w)
		return
	}

	tags, _ := s.store.AllTags()
	manifest := buildManifestJSON(notes)
	views.Layout(views.LayoutParams{
		Title:         note.Title,
		ManifestJSON:  manifest,
		Tree:          buildTree(s.store.NotebookPath(), notes, notePath),
		Tags:          tags,
		ContentCol:    views.NoteContentCol(breadcrumbs, note, result.HTML, backlinks, result.Headings),
		Headings:      result.Headings,
		OutgoingLinks: outLinks,
		Backlinks:     backlinks,
	}).Render(r.Context(), w)
}
```

- [ ] **Step 7: Rewrite handleFolder with HX-Request detection**

Same pattern as handleNote. For the HTMX path:

```go
if isHTMX(r) {
	views.FolderContentCol(breadcrumbs, folderName, entries).Render(r.Context(), w)
	views.TOCPanel(nil, nil, nil, true).Render(r.Context(), w)
	return
}
```

For the folder-with-index.md case, render the note content instead.

- [ ] **Step 8: Rewrite handleSearch**

```go
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	activeTag := strings.TrimSpace(r.URL.Query().Get("tags"))
	folder := strings.TrimSpace(r.URL.Query().Get("folder"))

	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	if q == "" && activeTag == "" {
		notes, err := s.store.AllNotes()
		if err != nil {
			http.Error(w, "search failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if folder != "" {
			prefix := folder + "/"
			filtered := notes[:0:0]
			for _, n := range notes {
				if strings.HasPrefix(n.Path, prefix) {
					filtered = append(filtered, n)
				}
			}
			notes = filtered
		}
		views.Tree(buildTree(s.store.NotebookPath(), notes, "")).Render(r.Context(), w)
		return
	}

	var tagFilter []string
	if activeTag != "" {
		tagFilter = []string{activeTag}
	}
	notes, err := s.store.Search(q, tagFilter)
	if err != nil {
		http.Error(w, "search failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if len(notes) == 0 {
		views.SearchEmpty().Render(r.Context(), w)
	} else {
		views.SearchResults(notes).Render(r.Context(), w)
	}
}
```

- [ ] **Step 9: Add isHTMX helper and buildManifestJSON**

Add to `handlers.go`:

```go
func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") != ""
}

func buildManifestJSON(notes []zk.Note) string {
	type entry struct {
		Title string   `json:"title"`
		Path  string   `json:"path"`
		Tags  []string `json:"tags"`
	}
	entries := make([]entry, len(notes))
	for i, n := range notes {
		tags := n.Tags
		if tags == nil {
			tags = []string{}
		}
		entries[i] = entry{Title: n.Title, Path: n.Path, Tags: tags}
	}
	b, _ := json.Marshal(entries)
	return string(b)
}
```

- [ ] **Step 10: Delete old template files**

```bash
rm -rf internal/server/templates/
```

- [ ] **Step 11: Run templ generate, build, and test**

```bash
templ generate ./internal/server/views/ && make build && go test ./...
```

Tests will likely fail because they assert on html/template output. Fix in next task.

- [ ] **Step 12: Commit**

```bash
git add internal/model/ internal/server/ 
git commit -m "feat: wire handlers to templ components, remove html/template"
```

---

## Task 7: Update tests for templ output

**Files:**
- Modify: `internal/server/handlers_test.go`

The existing tests use `newTestServer` which calls `server.New()`. Since `New()` no longer parses templates, it should still work. The test assertions check for string content in HTML output — these should still pass since templ renders the same HTML structure. But we need to:

1. Verify existing tests pass as-is (they may, since we preserved the same HTML structure)
2. Add tests for HTMX partial responses

- [ ] **Step 1: Run existing tests to see what passes**

```bash
go test ./internal/server/... -v
```

Check which tests fail and why.

- [ ] **Step 2: Fix any failing tests**

Common issues:
- Whitespace differences in templ output vs html/template
- Missing content due to component wiring issues

Update assertions to match the new output.

- [ ] **Step 3: Add test for HTMX partial note response**

```go
func TestNoteHTMXReturnsPartial(t *testing.T) {
	dir := t.TempDir()
	notePath := dir + "/test-note.md"
	if err := os.WriteFile(notePath, []byte("# Hello\n\nBold **text**.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	notes := []zk.Note{
		{
			Filename: "test-note.md", FilenameStem: "test-note",
			Path: "test-note.md", AbsPath: notePath,
			Title: "Hello", Modified: time.Now(), Created: time.Now(),
		},
	}
	h := newTestServer(t, notes, nil)
	req := httptest.NewRequest("GET", "/note/test-note.md", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	body := w.Body.String()
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	// Must contain content-col (primary swap target)
	if !strings.Contains(body, `id="content-col"`) {
		t.Error("expected #content-col in partial response")
	}
	// Must contain OOB toc-panel
	if !strings.Contains(body, `hx-swap-oob="true"`) {
		t.Error("expected hx-swap-oob in partial response")
	}
	// Must NOT contain full layout
	if strings.Contains(body, "<html") {
		t.Error("HTMX partial must not include full page layout")
	}
	// Must NOT contain sidebar
	if strings.Contains(body, `id="sidebar"`) {
		t.Error("HTMX partial must not include sidebar")
	}
}
```

- [ ] **Step 4: Add test for HTMX partial folder response**

```go
func TestFolderHTMXReturnsPartial(t *testing.T) {
	notes := []zk.Note{
		{
			Filename: "note.md", FilenameStem: "note",
			Path: "recipes/note.md", AbsPath: "/nb/recipes/note.md",
			Title: "My Recipe",
		},
	}
	h := newTestServer(t, notes, nil)
	req := httptest.NewRequest("GET", "/folder/recipes", nil)
	req.Header.Set("HX-Request", "true")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	body := w.Body.String()
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(body, `id="content-col"`) {
		t.Error("expected #content-col in partial response")
	}
	if strings.Contains(body, "<html") {
		t.Error("HTMX partial must not include full page layout")
	}
}
```

- [ ] **Step 5: Run all tests**

```bash
go test ./... -v
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add internal/server/handlers_test.go
git commit -m "test: update tests for templ output, add HTMX partial tests"
```

---

## Task 8: Refactor toc.js into init/destroy pair

**Files:**
- Modify: `internal/server/static/js/toc.js`

The TOC IntersectionObserver and progress bar listener need to be torn down and re-created after each HTMX swap. Currently `initToc()` is fire-once. Refactor into `initToc()` / `destroyToc()`.

- [ ] **Step 1: Rewrite toc.js**

Replace `internal/server/static/js/toc.js` with:

```js
let observer = null;
let scrollHandler = null;

export function destroyToc() {
  if (observer) {
    observer.disconnect();
    observer = null;
  }
  const contentArea = document.getElementById('content-area');
  if (scrollHandler && contentArea) {
    contentArea.removeEventListener('scroll', scrollHandler);
    scrollHandler = null;
  }
  const progressBar = document.getElementById('progress-bar');
  if (progressBar) progressBar.style.width = '0%';
}

export function initToc() {
  destroyToc();

  const contentArea = document.getElementById('content-area');
  const tocItems = document.querySelectorAll('#toc-inner .toc-item, .mob-toc-body .toc-item');
  const progressBar = document.getElementById('progress-bar');
  if (!contentArea) return;

  const headingIds = [...new Set([...tocItems].map(a => {
    const href = a.getAttribute('href');
    return href ? href.replace('#', '') : null;
  }).filter(Boolean))];

  const headingEls = headingIds.map(id => document.getElementById(id)).filter(Boolean);

  if (headingEls.length > 0) {
    let activeId = headingIds[0];

    observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            activeId = entry.target.id;
          }
        }
        tocItems.forEach(item => {
          const href = item.getAttribute('href');
          const id = href ? href.replace('#', '') : '';
          item.classList.toggle('active', id === activeId);
        });
      },
      { root: contentArea, rootMargin: '-10% 0px -80% 0px', threshold: 0 }
    );

    headingEls.forEach(el => observer.observe(el));
  }

  if (progressBar) {
    scrollHandler = () => {
      const max = contentArea.scrollHeight - contentArea.clientHeight;
      const pct = max > 0 ? Math.round((contentArea.scrollTop / max) * 100) : 0;
      progressBar.style.width = pct + '%';
    };
    contentArea.addEventListener('scroll', scrollHandler, { passive: true });
  }

  const mobDetails = document.getElementById('mob-toc-details');
  if (mobDetails) {
    mobDetails.addEventListener('click', (e) => {
      const link = e.target.closest('.toc-item');
      if (link) {
        e.preventDefault();
        const id = link.getAttribute('href')?.replace('#', '');
        const target = id ? document.getElementById(id) : null;
        if (target) contentArea.scrollTop = target.offsetTop - 20;
        mobDetails.open = false;
      }
    });
  }
}
```

- [ ] **Step 2: Verify build still works**

```bash
make build
```

- [ ] **Step 3: Commit**

```bash
git add internal/server/static/js/toc.js
git commit -m "refactor: toc.js init/destroy pair for HTMX re-init"
```

---

## Task 9: Add HTMX after-swap hooks

**Files:**
- Create: `internal/server/static/js/htmx-hooks.js`
- Modify: `internal/server/static/js/app.js`
- Modify: `internal/server/static/js/command-palette.js`

- [ ] **Step 1: Create htmx-hooks.js**

Create `internal/server/static/js/htmx-hooks.js`:

```js
import { initToc } from './toc.js';
import { initResize } from './resize.js';

export function initHTMXHooks() {
  document.body.addEventListener('htmx:afterSwap', (e) => {
    if (e.detail.target.id !== 'content-col') return;

    // 1. Update tree active state.
    updateTreeActive();

    // 2. Re-init TOC observer + progress bar.
    initToc();

    // 3. Re-init vertical resize handles (new TOC panel DOM).
    initResize();

    // 4. Re-run mermaid on new content.
    if (window.mermaid) {
      mermaid.run({ nodes: document.querySelectorAll('#content-area .mermaid') });
    }

    // 5. Scroll content area to top.
    const contentArea = document.getElementById('content-area');
    if (contentArea) contentArea.scrollTop = 0;
  });
}

function updateTreeActive() {
  const path = decodeURIComponent(location.pathname).replace(/^\/note\//, '').replace(/^\/folder\//, '');

  // Remove old active.
  document.querySelectorAll('.tree-item.active').forEach(el => el.classList.remove('active'));

  // Set new active.
  const link = document.querySelector(`.tree-item[data-path="${CSS.escape(path)}"]`);
  if (link) {
    link.classList.add('active');
    // Expand parent <details> elements.
    let parent = link.parentElement;
    while (parent) {
      if (parent.tagName === 'DETAILS') parent.open = true;
      parent = parent.parentElement;
    }
  }
}
```

Note: the `data-path` attribute was added to tree links in the sidebar templ component (Task 2). `CSS.escape()` handles paths with special characters.

- [ ] **Step 2: Update app.js to import htmx-hooks**

Replace `internal/server/static/js/app.js`:

```js
import { initTheme } from './theme.js';
import { initResize } from './resize.js';
import { initToc } from './toc.js';
import { initSidebar } from './sidebar.js';
import { initCommandPalette } from './command-palette.js';
import { initHTMXHooks } from './htmx-hooks.js';

initTheme();
initResize();
initToc();
initSidebar();
initCommandPalette();
initHTMXHooks();
```

- [ ] **Step 3: Update command-palette.js to use HTMX navigation**

In `internal/server/static/js/command-palette.js`, replace both `window.location.href = ...` occurrences with HTMX-aware navigation.

Replace lines 44-47 (Enter key handler):

```js
    } else if (e.key === 'Enter') {
      e.preventDefault();
      const focused = els[focusIdx];
      if (focused?.dataset.href) {
        dialog.close();
        htmx.ajax('GET', focused.dataset.href, { target: '#content-col', swap: 'outerHTML' });
        history.pushState({}, '', focused.dataset.href);
      }
    }
```

Replace lines 52-55 (click handler):

```js
  results.addEventListener('click', (e) => {
    const item = e.target.closest('.cmd-item');
    if (item?.dataset.href) {
      dialog.close();
      htmx.ajax('GET', item.dataset.href, { target: '#content-col', swap: 'outerHTML' });
      history.pushState({}, '', item.dataset.href);
    }
  });
```

Note: `htmx.ajax()` sends the `HX-Request` header automatically, so the server returns partials. The OOB swap for TOC is handled by HTMX automatically when the response contains `hx-swap-oob`. After the swap completes, the `htmx:afterSwap` event fires and our hooks run.

- [ ] **Step 4: Verify build**

```bash
make build
```

- [ ] **Step 5: Commit**

```bash
git add internal/server/static/js/
git commit -m "feat: add HTMX after-swap hooks and command palette integration"
```

---

## Task 10: End-to-end verification and cleanup

**Files:**
- Modify: `CLAUDE.md` (update stale HTMX reference)

- [ ] **Step 1: Run full test suite**

```bash
go test ./... -v
```

Expected: all pass.

- [ ] **Step 2: Build and start the server**

```bash
make build && ./bin/zk-serve --notebook ~/Git/second-brain --addr :8080 --open
```

- [ ] **Step 3: Manual verification checklist**

Test in browser:

1. **First visit to `/`** — full page loads, sidebar tree visible, tags visible, empty content area
2. **Click a note in sidebar** — content swaps without full page reload, URL updates, TOC appears
3. **Click another note** — sidebar persists, content swaps, tree active state updates
4. **Click a backlink in content** — navigates to linked note via HTMX
5. **Click a link in TOC panel** — navigates via HTMX
6. **Click a breadcrumb** — navigates to folder via HTMX
7. **Browser back button** — returns to previous note, content + TOC update
8. **Browser forward button** — returns to next note
9. **Cmd+K command palette** — opens, search works, selecting a note navigates via HTMX
10. **Direct URL visit** (paste `/note/some-note.md` in address bar) — full page renders
11. **Mermaid diagrams** — render correctly after HTMX swap
12. **Syntax highlighting** — code blocks styled correctly after swap
13. **Sidebar filter** — still works client-side, no regression
14. **Tag clicks** — still filter client-side
15. **Resize handles** — sidebar width, TOC width, vertical handles all work
16. **Theme toggle** — persists across HTMX navigations
17. **Mobile responsive** — sidebar hamburger menu still works

- [ ] **Step 4: Update CLAUDE.md**

Remove the stale reference to "HTMX search is debounced 300 ms via `hx-trigger`" from the Key conventions section (it was already inaccurate). Add:

Under Key conventions, replace the HTMX line with:
```
- HTMX powers partial navigation: note/folder clicks swap `#content-col` + `#toc-panel` (OOB) without full page reloads. Server detects `HX-Request` header.
- Templ components in `internal/server/views/` replace `html/template`. Run `templ generate` (or `make generate`) after editing `.templ` files.
```

- [ ] **Step 5: Run tests one final time**

```bash
go test ./... -v
```

- [ ] **Step 6: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md for templ + HTMX architecture"
```
