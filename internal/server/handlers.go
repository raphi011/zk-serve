package server

import (
        "encoding/json"
        "fmt"
        "log"
        "net/http"
        "os"
        "path/filepath"
        "sort"
        "strings"
        "time"

        "github.com/raphaelgruber/zk-serve/internal/model"
        "github.com/raphaelgruber/zk-serve/internal/render"
        "github.com/raphaelgruber/zk-serve/internal/server/views"
        "github.com/raphaelgruber/zk-serve/internal/zk"
)

func isHTMX(r *http.Request) bool {
        return r.Header.Get("HX-Request") != ""
}

// noteCache holds data derived from AllNotes that is reused across requests.
// Computed once at startup since the notebook is read-only.
type noteCache struct {
        notes        []zk.Note
        tags         []zk.Tag
        manifestJSON string
        lookup       map[string]string // filenameStem|pathWithoutExt → path
        notesByPath  map[string]*zk.Note
}

func buildNoteCache(store Store) (*noteCache, error) {
        notes, err := store.AllNotes()
        if err != nil {
                return nil, fmt.Errorf("load notes: %w", err)
        }
        tags, err := store.AllTags()
        if err != nil {
                return nil, fmt.Errorf("load tags: %w", err)
        }

        lookup := make(map[string]string, len(notes)*2)
        byPath := make(map[string]*zk.Note, len(notes))
        for i, n := range notes {
                lookup[n.FilenameStem] = n.Path
                lookup[strings.TrimSuffix(n.Path, ".md")] = n.Path
                byPath[n.Path] = &notes[i]
        }

        return &noteCache{
                notes:        notes,
                tags:         tags,
                manifestJSON: buildManifestJSON(notes),
                lookup:       lookup,
                notesByPath:  byPath,
        }, nil
}

// buildBreadcrumbs splits a note path into clickable folder segments,
// excluding the filename itself.
func buildBreadcrumbs(notePath string) []model.BreadcrumbSegment {
        parts := strings.Split(notePath, "/")
        dirs := parts[:len(parts)-1]
        crumbs := make([]model.BreadcrumbSegment, len(dirs))
        for i, name := range dirs {
                crumbs[i] = model.BreadcrumbSegment{
                        Name:       name,
                        FolderPath: strings.Join(parts[:i+1], "/"),
                }
        }
        return crumbs
}

// buildTree constructs a sorted folder tree from the provided list of indexed notes.
// It uses the database as the source of truth, so any files or directories
// excluded via .zkignore will not appear in the sidebar.
func buildTree(notes []zk.Note, activePath string) []*model.FileNode {
        type treeEntry struct {
                node     *model.FileNode
                children map[string]*treeEntry
        }
        root := &treeEntry{children: map[string]*treeEntry{}}

        for _, n := range notes {
                parts := strings.Split(n.Path, "/")
                cur := root
                for i, part := range parts {
                        isLast := i == len(parts)-1
                        if _, exists := cur.children[part]; !exists {
                                var node *model.FileNode
                                if !isLast {
                                        node = &model.FileNode{Name: part, IsDir: true}
                                } else {
                                        node = &model.FileNode{
                                                Name:     n.Title,
                                                Path:     n.Path,
                                                IsActive: n.Path == activePath,
                                        }
                                }
                                cur.children[part] = &treeEntry{node: node, children: map[string]*treeEntry{}}
                        }
                        cur = cur.children[part]
                }
        }

        // flatten returns children and whether any descendant is active.
        var flatten func(*treeEntry) ([]*model.FileNode, bool)
        flatten = func(e *treeEntry) ([]*model.FileNode, bool) {
                var dirKeys, fileKeys []string
                for k, child := range e.children {
                        if child.node.IsDir {
                                dirKeys = append(dirKeys, k)
                        } else {
                                fileKeys = append(fileKeys, k)
                        }
                }
                sort.Strings(dirKeys)
                sort.Strings(fileKeys)

                anyActive := false
                nodes := make([]*model.FileNode, 0, len(e.children))
                for _, k := range dirKeys {
                        child := e.children[k]
                        child.node.Children, child.node.IsOpen = flatten(child)
                        if child.node.IsOpen {
                                anyActive = true
                        }
                        nodes = append(nodes, child.node)
                }
                for _, k := range fileKeys {
                        n := e.children[k].node
                        if n.IsActive {
                                anyActive = true
                        }
                        nodes = append(nodes, n)
                }
                return nodes, anyActive
        }

        nodes, _ := flatten(root)
        return nodes
}

func buildManifestJSON(notes []zk.Note) string {
        type entry struct {
                Title    string   `json:"title"`
                Path     string   `json:"path"`
                Tags     []string `json:"tags"`
                Modified int64    `json:"mod"`
        }
        entries := make([]entry, len(notes))
        for i, n := range notes {
                tags := n.Tags
                if tags == nil {
                        tags = []string{}
                }
                entries[i] = entry{Title: n.Title, Path: n.Path, Tags: tags, Modified: n.Modified.Unix()}
        }
        b, _ := json.Marshal(entries)
        return string(b)
}

func currentYearMonth() (int, int) {
        now := time.Now()
        return now.Year(), int(now.Month())
}

func (s *Server) calendarData() (int, int, map[int]bool) {
        year, month := currentYearMonth()
        days, err := s.store.ActivityDays(year, month)
        if err != nil {
                log.Printf("calendar activity days: %v", err)
        }
        if days == nil {
                days = map[int]bool{}
        }
        return year, month, days
}

// renderTOC renders the TOC panel as an OOB swap, always including the calendar.
func (s *Server) renderTOC(w http.ResponseWriter, r *http.Request, headings []render.Heading, outLinks []zk.Link, backlinks []zk.Link) {
        calYear, calMonth, activeDays := s.calendarData()
        views.TOCPanel(headings, outLinks, backlinks, true, calYear, calMonth, activeDays).Render(r.Context(), w)
}

// renderFullPage renders a complete page layout with sidebar, TOC, and calendar.
func (s *Server) renderFullPage(w http.ResponseWriter, r *http.Request, p views.LayoutParams) {
        calYear, calMonth, activeDays := s.calendarData()
        p.Tags = s.cache.tags
        p.ManifestJSON = s.cache.manifestJSON
        p.CalendarYear = calYear
        p.CalendarMonth = calMonth
        p.ActiveDays = activeDays
        views.Layout(p).Render(r.Context(), w)
}

// renderNote renders a note as content + TOC for both HTMX and full-page.
func (s *Server) renderNote(w http.ResponseWriter, r *http.Request, note *zk.Note) {
        raw, err := os.ReadFile(note.AbsPath)
        if err != nil {
                http.Error(w, "failed to read note: "+err.Error(), http.StatusInternalServerError)
                return
        }
        result, err := render.Markdown(raw, s.cache.lookup)
        if err != nil {
                http.Error(w, "failed to render note: "+err.Error(), http.StatusInternalServerError)
                return
        }
        outLinks, err := s.store.OutgoingLinks(note.Path)
        if err != nil {
                log.Printf("outgoing links for %s: %v", note.Path, err)
        }
        backlinks, err := s.store.Backlinks(note.Path)
        if err != nil {
                log.Printf("backlinks for %s: %v", note.Path, err)
        }
        breadcrumbs := buildBreadcrumbs(note.Path)

        w.Header().Set("Content-Type", "text/html; charset=utf-8")

        if isHTMX(r) {
                views.NoteContentCol(breadcrumbs, note, result.HTML, backlinks, result.Headings).Render(r.Context(), w)
                s.renderTOC(w, r, result.Headings, outLinks, backlinks)
                return
        }

        s.renderFullPage(w, r, views.LayoutParams{
                Title:         note.Title,
                Tree:          buildTree(s.cache.notes, note.Path),
                ContentCol:    views.NoteContentCol(breadcrumbs, note, result.HTML, backlinks, result.Headings),
                Headings:      result.Headings,
                OutgoingLinks: outLinks,
                Backlinks:     backlinks,
        })
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
        if r.URL.Path != "/" {
                http.NotFound(w, r)
                return
        }

        // Serve index.md if it exists at the notebook root.
        if note := s.cache.notesByPath["index.md"]; note != nil {
                s.renderNote(w, r, note)
                return
        }

        // Build root directory listing from cached notes.
        seen := map[string]bool{}
        var entries []model.FolderEntry
        for _, n := range s.cache.notes {
                parts := strings.SplitN(n.Path, "/", 2)
                if len(parts) == 1 {
                        entries = append(entries, model.FolderEntry{Name: parts[0], Path: n.Path, Title: n.Title})
                } else if !seen[parts[0]] {
                        seen[parts[0]] = true
                        entries = append(entries, model.FolderEntry{Name: parts[0], Path: parts[0], IsDir: true})
                }
        }
        sort.Slice(entries, func(i, j int) bool {
                if entries[i].IsDir != entries[j].IsDir {
                        return entries[i].IsDir
                }
                return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
        })

        contentCol := views.FolderContentCol(nil, "Notebook", entries)
        w.Header().Set("Content-Type", "text/html; charset=utf-8")

        if isHTMX(r) {
                contentCol.Render(r.Context(), w)
                s.renderTOC(w, r, nil, nil, nil)
                return
        }

        s.renderFullPage(w, r, views.LayoutParams{
                Title:      "Notebook",
                Tree:       buildTree(s.cache.notes, ""),
                ContentCol: contentCol,
        })
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
        q := strings.TrimSpace(r.URL.Query().Get("q"))
        activeTag := strings.TrimSpace(r.URL.Query().Get("tags"))
        folder := strings.TrimSpace(r.URL.Query().Get("folder"))
        date := strings.TrimSpace(r.URL.Query().Get("date"))

        w.Header().Set("Content-Type", "text/html; charset=utf-8")

        if date != "" {
                notes, err := s.store.NotesByDate(date)
                if err != nil {
                        http.Error(w, "search failed: "+err.Error(), http.StatusInternalServerError)
                        return
                }
                if len(notes) == 0 {
                        views.SearchEmpty().Render(r.Context(), w)
                } else {
                        views.SearchResults(notes).Render(r.Context(), w)
                }
                return
        }

        if q == "" && activeTag == "" {
                notes := s.cache.notes
                if folder != "" {
                        prefix := folder + "/"
                        filtered := make([]zk.Note, 0)
                        for _, n := range notes {
                                if strings.HasPrefix(n.Path, prefix) {
                                        filtered = append(filtered, n)
                                }
                        }
                        notes = filtered
                }
                views.Tree(buildTree(notes, "")).Render(r.Context(), w)
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

func (s *Server) handleCalendar(w http.ResponseWriter, r *http.Request) {
        year, month := currentYearMonth()
        if v := r.URL.Query().Get("year"); v != "" {
                fmt.Sscan(v, &year)
        }
        if v := r.URL.Query().Get("month"); v != "" {
                fmt.Sscan(v, &month)
        }

        days, err := s.store.ActivityDays(year, month)
        if err != nil {
                http.Error(w, "calendar failed: "+err.Error(), http.StatusInternalServerError)
                return
        }

        w.Header().Set("Content-Type", "text/html; charset=utf-8")
        views.Calendar(year, month, days, 0).Render(r.Context(), w)
}

func (s *Server) handleTags(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        fmt.Fprintf(w, "[")
        for i, t := range s.cache.tags {
                if i > 0 {
                        fmt.Fprintf(w, ",")
                }
                fmt.Fprintf(w, `{"name":%q,"noteCount":%d}`, t.Name, t.NoteCount)
        }
        fmt.Fprintf(w, "]")
}

func (s *Server) handleNote(w http.ResponseWriter, r *http.Request) {
        notePath := r.PathValue("path")
        if notePath == "" {
                http.NotFound(w, r)
                return
        }

        note := s.cache.notesByPath[notePath]
        if note == nil {
                // Fallback: file exists on disk but not in the index.
                absPath := filepath.Join(s.store.NotebookPath(), notePath)
                if _, err := os.Stat(absPath); err != nil {
                        http.NotFound(w, r)
                        return
                }
                stem := strings.TrimSuffix(filepath.Base(notePath), ".md")
                note = &zk.Note{
                        Path:         notePath,
                        AbsPath:      absPath,
                        Filename:     filepath.Base(notePath),
                        FilenameStem: stem,
                        Title:        stem,
                }
        }

        s.renderNote(w, r, note)
}

func (s *Server) handleFolder(w http.ResponseWriter, r *http.Request) {
        folderPath := r.PathValue("path")
        if folderPath == "" {
                http.Redirect(w, r, "/", http.StatusFound)
                return
        }

        // Serve index.md if it exists in this folder.
        if note := s.cache.notesByPath[folderPath+"/index.md"]; note != nil {
                s.renderNote(w, r, note)
                return
        }

        // Build directory listing from cached notes.
        prefix := folderPath + "/"
        seen := map[string]bool{}
        var entries []model.FolderEntry
        for _, n := range s.cache.notes {
                if !strings.HasPrefix(n.Path, prefix) {
                        continue
                }
                rest := strings.TrimPrefix(n.Path, prefix)
                parts := strings.SplitN(rest, "/", 2)
                if len(parts) == 1 {
                        entries = append(entries, model.FolderEntry{Name: parts[0], Path: n.Path, Title: n.Title})
                } else if !seen[parts[0]] {
                        seen[parts[0]] = true
                        entries = append(entries, model.FolderEntry{Name: parts[0], Path: folderPath + "/" + parts[0], IsDir: true})
                }
        }
        sort.Slice(entries, func(i, j int) bool {
                if entries[i].IsDir != entries[j].IsDir {
                        return entries[i].IsDir
                }
                return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
        })

        folderParts := strings.Split(folderPath, "/")
        folderName := folderParts[len(folderParts)-1]
        breadcrumbs := buildBreadcrumbs(folderPath)

        w.Header().Set("Content-Type", "text/html; charset=utf-8")

        if isHTMX(r) {
                views.FolderContentCol(breadcrumbs, folderName, entries).Render(r.Context(), w)
                s.renderTOC(w, r, nil, nil, nil)
                return
        }

        s.renderFullPage(w, r, views.LayoutParams{
                Title:      folderName,
                Tree:       buildTree(s.cache.notes, ""),
                ContentCol: views.FolderContentCol(breadcrumbs, folderName, entries),
        })
}
