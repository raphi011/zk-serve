# zk-serve: replace CLI subprocess with SQLite snapshot + in-process FTS

**Status:** Design approved — pending implementation plan
**Date:** 2026-04-19
**Scope:** `~/Git/zk-serve` (main change) + `~/Git/turingpi-k8s/manifests/zk-serve/` (deployment)

## Problem

Each page load on https://zk.manx-turtle.ts.net/ takes 5–8 seconds. Measured breakdown per note request:

| Step | Cost | Location |
|---|---|---|
| `zk list` subprocess, full notebook → 4.5 MB JSON | ~2.7s | `internal/zk/client.go:46` via `handlers.go:175` |
| JSON unmarshal of 4.5 MB | ~0.5s | `client.go:67` |
| `zk tag list` subprocess | ~1.3s | `handlers.go:206` |
| `buildTree` over 877 notes | fast | `handlers.go:27` |
| Markdown + template render | fast | — |

Additionally, `zk.Client.run()` serializes all subprocess calls behind a `sync.Mutex` (`client.go:47`), so concurrent requests queue up.

The CPU limit (200m) on the ARM64 node compounds subprocess cost; raising limits is a band-aid, not a fix.

## Key architectural fact

The notebook lives in an `emptyDir` that is `git clone`d once at pod startup (`manifests/zk-serve/deployment.yaml:19`) and **never changes during the pod's lifetime**. This means a cache does not need invalidation. Populate once at boot, serve forever.

## Design decisions

Each decision below was ratified during brainstorming. The "alternatives considered" list is the record of what we chose against.

### D1 — Use a snapshot loaded once at boot

Serve all bulk-read endpoints (index, note, tree, tag list) from an immutable in-memory `Snapshot` built once during process startup. No mutex, no refresh, no invalidation.

**Alternatives considered:**
- TTL cache with background refresh — rejected; no value unless notebook changes, which it does not.
- Leave subprocess calls, tune resource limits / `zk --format` — rejected; band-aid; does not help mutex contention.

### D2 — Query zk's SQLite DB directly (read-only) instead of running `zk list`

The snapshot is built by opening `/notebook/.zk/notebook.db` read-only and running two SELECTs against zk's own schema (`notes_with_metadata` view + `collections` join). No JSON round-trip.

**Alternatives considered:**
- Keep `zk list` subprocess at boot — rejected; JSON serialization is the dominant cost and direct queries open the path to replacing FTS search too.
- Import zk as a Go library — **blocked**: all zk code is under `internal/`, forbidden by Go's visibility rules; and zk is GPL-3.0, which would force zk-serve to re-license.
- Reimplement markdown indexing ourselves, drop zk entirely — rejected as scope creep; good future optimization.

### D3 — Single init container for clone + index (Option X)

One init container using the zk-serve image does both `git clone` and `zk index` sequentially, in one script. The zk-serve Dockerfile already installs git, zk, and ca-certificates (`Dockerfile:34`), so no image changes are needed.

**Alternatives considered:**
- Two init containers (`git-clone` using alpine/git, then `zk-index` using zk-serve image) — rejected; extra YAML with no meaningful benefit since the runtime image already has both tools. Secret-scoping benefit of two containers is minor given the boot script reads the token once and moves on.
- Fold into main container startup — rejected; makes readiness probe semantics fuzzy and blurs the "one container, one responsibility" line.

### D4 — Replace `/search` subprocess with in-process FTS (Tier 2)

Query SQLite's `notes_fts` virtual table directly. Port zk's Google-like query preprocessor (`fts5.ConvertQuery` in zk's source) as an independent reimplementation. Match zk's BM25 weights `(1000.0, 500.0, 1.0)` for path/title/body. This deletes `internal/zk/client.go` entirely — no runtime subprocess calls remain.

**Alternatives considered:**
- Tier 0 (defer): keep zk CLI for search — rejected; PR becomes coherent if the subprocess wrapper is fully deleted.
- Tier 1 (minimum viable): in-process SQL without porting `ConvertQuery` — rejected; loses phrases, prefix, NOT, OR syntax that would otherwise "just work".
- Add a third-party index (bleve) — rejected; zk already maintains the index, duplicating it is wasteful.

### D5 — Pure-Go SQLite driver

`modernc.org/sqlite`. No CGO, cross-compiles cleanly to ARM64, ~6MB binary bloat. Opened with `file:/notebook/.zk/notebook.db?mode=ro&immutable=1&_pragma=query_only(true)`. Single `*sql.DB` on the Server, shared across handlers.

**Alternatives considered:**
- `mattn/go-sqlite3` (CGO) — rejected; requires build toolchain in the Dockerfile builder stage and complicates cross-build.

## Architecture overview

```
[initContainer: prepare-notebook]         ← zk-serve image
   git clone second-brain → /notebook
   zk index               → /notebook/.zk/notebook.db
            ↓
[main container: zk-serve]
   LoadSnapshot(dbPath) reads 2 SELECTs, builds in-memory struct
   Open *sql.DB for search queries
   http.ListenAndServe
```

Both containers share the `notebook` emptyDir volume.

## Data structures

### `internal/zk/snapshot.go`

```go
type Snapshot struct {
    Notes       []Note            // ordered by sortable_path
    Tags        []Tag             // with note counts
    NotesByPath map[string]*Note  // O(1) lookup in handleNote
    WikiLookup  map[string]string // filenameStem + trimmed-path → canonical path
}

func LoadSnapshot(dbPath, notebookPath string) (*Snapshot, error)
```

`Note` is the existing `zk.Note` struct (kept for template compatibility), populated with the fields we have from SQLite: `Path`, `Filename`, `FilenameStem` (derived: `strings.TrimSuffix(Filename, ".md")`), `AbsPath` (derived: `filepath.Join(notebookPath, Path)`), `Title`, `Lead`, `Tags`, `Metadata`. Other fields (Body, RawContent, Snippets, Checksum, WordCount, Created, Modified) remain zero — no current handler reads them outside the note render path which uses `os.ReadFile`.

### SQLite queries at load

```sql
-- Q1: all notes with tags
SELECT path, filename, title, lead, metadata, COALESCE(tags, '')
FROM notes_with_metadata
ORDER BY sortable_path;

-- Q2: tag list with counts
SELECT c.name, COUNT(nc.note_id)
FROM collections c
LEFT JOIN notes_collections nc ON nc.collection_id = c.id
WHERE c.kind = 'tag'
GROUP BY c.id
ORDER BY c.name;
```

`notes_with_metadata` is a zk-provided view that GROUP_CONCATs tags with `\x01` separator (schema: `internal/adapter/sqlite/db.go:165` in zk). We split on `\x01` in Go to populate `Note.Tags`.

## Handler changes

| Handler | Before | After |
|---|---|---|
| `handleIndex` (`/`) | `TagList` + `List("", nil)` + buildTree | `s.snap.Tags` + `buildTree(s.snap.Notes, "")` |
| `handleNote` (`/note/:path`) | full List + linear scan + `TagList` + buildTree + build lookup | `s.snap.NotesByPath[path]`, `os.ReadFile`, `render.Markdown(raw, s.snap.WikiLookup)`, `buildTree(s.snap.Notes, path)`, `s.snap.Tags` |
| `handleSearch` empty q+tag | `List("", nil)` + buildTree | `buildTree(s.snap.Notes, "")` |
| `handleSearch` q and/or tag | `s.zkClient.List(q, tags)` | `zk.Search(s.db, q, tags)` |
| `handleTags` | `TagList` | `s.snap.Tags` |

`buildTree` itself is unchanged.

## In-process search

### `internal/zk/search.go`

```go
// Search returns notes matching the FTS query and/or tag filter,
// ordered by BM25 relevance when q is present, else by sortable_path.
// db must be opened in read-only mode.
func Search(db *sql.DB, q string, tags []string) ([]Note, error)
```

Dynamic SQL built from `q` and `tags`:

```sql
SELECT n.path, n.filename, n.title, n.lead,
       COALESCE(GROUP_CONCAT(c.name, char(1)), '') AS tags,
       {{ snippet }} AS snippet
FROM notes n
{{ fts_join }}
LEFT JOIN notes_collections nc ON nc.note_id = n.id
LEFT JOIN collections       c  ON c.id = nc.collection_id AND c.kind = 'tag'
{{ where }}
GROUP BY n.id
ORDER BY {{ order }}
LIMIT 200;
```

- When `q != ""`: `fts_join` = `JOIN notes_fts ON notes_fts.rowid = n.id`; add `notes_fts MATCH ?` to WHERE; `order` = `bm25(notes_fts, 1000.0, 500.0, 1.0)`; `snippet` = `snippet(notes_fts, 2, '⟪MARK_START⟫', '⟪MARK_END⟫', '…', 20)`.
- When `q == ""`: no FTS join; `snippet` = `''`; `order` = `n.sortable_path`.
- For each tag: add `AND n.id IN (SELECT note_id FROM notes_collections WHERE collection_id IN (SELECT id FROM collections WHERE kind='tag' AND name GLOB ?))`. Multiple tags are ANDed (must match all). Current handler only passes a single tag, so multi-tag semantics are a forward-compat detail.

`GLOB` (vs `=`) matches zk's tag behavior so `work/*` patterns work.

**`LIMIT 200`:** new behavior. Current `zk list` returns all matching notes (no limit); for a personal notebook this is fine but an unbounded FTS MATCH is a denial-of-service shape. 200 is well above any reasonable UI scroll depth and can be tuned if needed.

### Snippet HTML rendering

FTS5's `snippet()` operates on already-indexed text and cannot HTML-escape. To avoid injection from notebook content:

1. Use distinctive Unicode markers (`⟪MARK_START⟫`, `⟪MARK_END⟫`) unlikely to appear in user content.
2. In Go: `html.EscapeString(raw)` → then `strings.ReplaceAll(s, "⟪MARK_START⟫", "<mark>")` (same for end) → cast to `template.HTML`.
3. `list.html` template already renders snippets; add `safeHTML` pipeline.

### FTS query preprocessing — `internal/zk/ftsquery.go`

Independent reimplementation of the transformation zk does. Table of required behaviors:

| Input | Output |
|---|---|
| `foo` | `"foo"` |
| `foo bar` | `"foo" "bar"` |
| `"foo bar"` | `"foo bar"` |
| `foo*` | `"foo"*` |
| `-foo` | ` NOT "foo"` |
| `foo\|bar` | `"foo" OR "bar"` |
| `AND`, `OR`, `NOT` (unquoted) | passthrough as operators |
| `title:foo` | `title:"foo"` |
| `"`  (bare) | empty (stripped) |
| `foo "bar baz" qux` | `"foo" "bar baz" "qux"` |

Algorithm: state machine reading runes, tracking `inQuote` flag and current `term` buffer, flushing on separators (whitespace, parens). Implemented from scratch based on the table above — not a line-for-line copy of zk's implementation.

### Concurrency

Single `*sql.DB` opened in `main.go` after `LoadSnapshot`, stored on `Server`, closed on shutdown. `modernc.org/sqlite` supports concurrent queries on a shared handle. `immutable=1` flag means SQLite skips locking/journal files — safe under `readOnlyRootFilesystem: true` and eliminates all lock contention.

## Deployment

### `manifests/zk-serve/deployment.yaml`

Replace the current `git-clone` initContainer with one named `prepare-notebook` using the zk-serve image:

```yaml
initContainers:
  - name: prepare-notebook
    image: 10.108.73.22:5000/zk-serve:v0.2.0
    securityContext:
      runAsUser: 1000
      runAsNonRoot: true
      allowPrivilegeEscalation: false
      readOnlyRootFilesystem: true
    env:
      - { name: GIT_TERMINAL_PROMPT, value: "0" }
      - { name: HOME, value: /tmp }
    command: ["sh", "-c"]
    args:
      - |
        set -e
        TOKEN=$(cat /secrets/token)
        git clone --depth 1 --branch main \
          "https://x-oauth-basic:${TOKEN}@github.com/raphi011/second-brain.git" /notebook
        cd /notebook && zk index
    volumeMounts:
      - { name: notebook,     mountPath: /notebook }
      - { name: github-token, mountPath: /secrets, readOnly: true }
      - { name: tmp,          mountPath: /tmp }
```

Main container unchanged aside from `image: ...:v0.2.0`. Readiness probe `initialDelaySeconds: 5` is sufficient since the heavy work (zk indexing) happens in the init phase, not after main starts.

### Dockerfile — no changes

git, zk, ca-certificates already installed (`Dockerfile:34, 38`).

### `go.mod` — add one dependency

```
modernc.org/sqlite v1.x.x
```

## Testing

Fixture: `internal/zk/testdata/notebook/` — a few `.md` files with frontmatter tags, wiki-links, and varying titles. The test suite's setup hook runs `zk index` in this directory once to produce `testdata/notebook/.zk/notebook.db`. The `.zk/` dir is gitignored; CI or local test runs regenerate it. This is the only place the test suite invokes the `zk` binary.

| Test file | Coverage |
|---|---|
| `internal/zk/ftsquery_test.go` | Table-driven cases from the input/output table above; edge cases: empty input, stray quote, mixed quoted/unquoted terms |
| `internal/zk/snapshot_test.go` | `LoadSnapshot` against fixture DB: correct count, tags split correctly, `NotesByPath` map populated, `WikiLookup` covers filenameStem + path-without-suffix |
| `internal/zk/search_test.go` | FTS search returns expected note, BM25 ordering (title match above body match), tag-only filter, q+tag combo, GLOB tag matching, snippet markers present |
| `internal/server/handlers_test.go` (extend) | Construct `Server` with test Snapshot + fixture `*sql.DB`; verify handlers read from snapshot and call Search on non-empty q |

## Rollout

1. `cd ~/Git/zk-serve` — implement changes, pass tests.
2. Build and push: `docker build --platform linux/arm64 --provenance=false -t zot.manx-turtle.ts.net/zk-serve:v0.2.0 . && docker push zot.manx-turtle.ts.net/zk-serve:v0.2.0`.
3. Bump image tag in `manifests/zk-serve/deployment.yaml` to `v0.2.0`. Commit + push turingpi-k8s. ArgoCD auto-syncs.
4. Validate:
   - `kubectl get pods -n zk-serve` — pod Ready, 1 init completed.
   - `curl -s -o /dev/null -w 'ttfb=%{time_starttransfer}s\n' https://zk.manx-turtle.ts.net/note/<path>` — expect <100ms.
   - `curl 'https://zk.manx-turtle.ts.net/search?q=motorcycle'` — expect <100ms, results with `<mark>` highlights.
   - `kubectl logs -n zk-serve deploy/zk-serve` — no errors.

**Rollback:** revert image tag in `deployment.yaml`. ArgoCD syncs back. emptyDir means no state drift.

## Out of scope (explicitly deferred)

- Live-reload of notebook (fsnotify + re-index or pod restart on `git push`)
- Periodic git-pull to surface new notes without pod restart
- Dropping zk binary from image (requires reimplementing `zk index`)
- Fixing `handlers.go:220` "superfluous WriteHeader" log warnings
- Bumping CPU limits

## Success criteria

- Note page load: p50 < 150ms, p95 < 400ms (from current ~5s).
- Search query: p50 < 100ms (from current 1.2s).
- Concurrent requests no longer queue on a single mutex.
- No functional regression in tree, search, wiki-links, tag filtering.
- `internal/zk/client.go` deleted; no runtime subprocess calls remain.
