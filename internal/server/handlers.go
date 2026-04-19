package server

import (
        "encoding/json"
        "fmt"
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

        w.Header().Set("Content-Type", "text/html; charset=utf-8")

        if isHTMX(r) {
                views.EmptyContentCol().Render(r.Context(), w)
                views.TOCPanel(nil, nil, nil, true, 0, 0, nil).Render(r.Context(), w)
                return
        }

        calYear, calMonth, activeDays := s.calendarData()
        views.Layout(views.LayoutParams{
                Tags:          tags,
                Tree:          buildTree(notes, ""),
                ManifestJSON:  buildManifestJSON(notes),
                ContentCol:    views.EmptyContentCol(),
                CalendarYear:  calYear,
                CalendarMonth: calMonth,
                ActiveDays:    activeDays,
        }).Render(r.Context(), w)
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
        q := strings.TrimSpace(r.URL.Query().Get("q"))
        activeTag := strings.TrimSpace(r.URL.Query().Get("tags"))
        folder := strings.TrimSpace(r.URL.Query().Get("folder"))
        date := strings.TrimSpace(r.URL.Query().Get("date"))

        w.Header().Set("Content-Type", "text/html; charset=utf-8")

        // Date filter takes priority — mutually exclusive with text/tag search.
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

func currentYearMonth() (int, int) {
        now := time.Now()
        return now.Year(), int(now.Month())
}

func (s *Server) calendarData() (int, int, map[int]bool) {
        year, month := currentYearMonth()
        days, _ := s.store.ActivityDays(year, month)
        if days == nil {
                days = map[int]bool{}
        }
        return year, month, days
}

func (s *Server) handleTags(w http.ResponseWriter, r *http.Request) {
        tags, err := s.store.AllTags()
        if err != nil {
                http.Error(w, "failed to list tags: "+err.Error(), http.StatusInternalServerError)
                return
        }
        w.Header().Set("Content-Type", "application/json")
        fmt.Fprintf(w, "[")
        for i, t := range tags {
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
                        Path:         notePath,
                        AbsPath:      absPath,
                        Filename:     filepath.Base(notePath),
                        FilenameStem: stem,
                        Title:        stem,
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
                views.TOCPanel(result.Headings, outLinks, backlinks, true, 0, 0, nil).Render(r.Context(), w)
                return
        }

        tags, _ := s.store.AllTags()
        calYear, calMonth, activeDays := s.calendarData()
        views.Layout(views.LayoutParams{
                Title:         note.Title,
                ManifestJSON:  buildManifestJSON(notes),
                Tree:          buildTree(notes, notePath),
                Tags:          tags,
                ContentCol:    views.NoteContentCol(breadcrumbs, note, result.HTML, backlinks, result.Headings),
                Headings:      result.Headings,
                OutgoingLinks: outLinks,
                Backlinks:     backlinks,
                CalendarYear:  calYear,
                CalendarMonth: calMonth,
                ActiveDays:    activeDays,
        }).Render(r.Context(), w)
}

func (s *Server) handleFolder(w http.ResponseWriter, r *http.Request) {
        folderPath := r.PathValue("path")
        if folderPath == "" {
                http.Redirect(w, r, "/", http.StatusFound)
                return
        }
        notes, err := s.store.AllNotes()
        if err != nil {
                http.Error(w, "failed to list notes: "+err.Error(), http.StatusInternalServerError)
                return
        }
        tags, _ := s.store.AllTags()

        // Serve index.md if it exists in this folder.
        indexPath := folderPath + "/index.md"
        for i := range notes {
                if notes[i].Path == indexPath {
                        note := &notes[i]
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
                        outLinks, _ := s.store.OutgoingLinks(note.Path)
                        backlinks, _ := s.store.Backlinks(note.Path)
                        breadcrumbs := buildBreadcrumbs(note.Path)

                        w.Header().Set("Content-Type", "text/html; charset=utf-8")

                        if isHTMX(r) {
                                views.NoteContentCol(breadcrumbs, note, result.HTML, backlinks, result.Headings).Render(r.Context(), w)
                                views.TOCPanel(result.Headings, outLinks, backlinks, true, 0, 0, nil).Render(r.Context(), w)
                                return
                        }

                        calYear, calMonth, activeDays := s.calendarData()
                        views.Layout(views.LayoutParams{
                                Title:         note.Title,
                                ManifestJSON:  buildManifestJSON(notes),
                                Tree:          buildTree(notes, note.Path),
                                Tags:          tags,
                                ContentCol:    views.NoteContentCol(breadcrumbs, note, result.HTML, backlinks, result.Headings),
                                Headings:      result.Headings,
                                OutgoingLinks: outLinks,
                                Backlinks:     backlinks,
                                CalendarYear:  calYear,
                                CalendarMonth: calMonth,
                                ActiveDays:    activeDays,
                        }).Render(r.Context(), w)
                        return
                }
        }

        // Build directory listing from notes whose path starts with folderPath/.
        prefix := folderPath + "/"
        seen := map[string]bool{}
        var entries []model.FolderEntry
        for _, n := range notes {
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
                views.TOCPanel(nil, nil, nil, true, 0, 0, nil).Render(r.Context(), w)
                return
        }

        calYear, calMonth, activeDays := s.calendarData()
        views.Layout(views.LayoutParams{
                Title:         folderName,
                ManifestJSON:  buildManifestJSON(notes),
                Tree:          buildTree(notes, ""),
                Tags:          tags,
                ContentCol:    views.FolderContentCol(breadcrumbs, folderName, entries),
                CalendarYear:  calYear,
                CalendarMonth: calMonth,
                ActiveDays:    activeDays,
        }).Render(r.Context(), w)
}
