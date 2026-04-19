package server

import (
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"sort"
	"strings"

	"github.com/raphaelgruber/zk-serve/internal/render"
	"github.com/raphaelgruber/zk-serve/internal/zk"
)

// BreadcrumbSegment is one folder step in a note's path.
type BreadcrumbSegment struct {
	Name       string
	FolderPath string // e.g. "notes" or "notes/ai"
}

// buildBreadcrumbs splits a note path into clickable folder segments,
// excluding the filename itself.
func buildBreadcrumbs(notePath string) []BreadcrumbSegment {
	parts := strings.Split(notePath, "/")
	dirs := parts[:len(parts)-1]
	crumbs := make([]BreadcrumbSegment, len(dirs))
	for i, name := range dirs {
		crumbs[i] = BreadcrumbSegment{
			Name:       name,
			FolderPath: strings.Join(parts[:i+1], "/"),
		}
	}
	return crumbs
}

// FileNode is one entry in the sidebar folder tree.
type FileNode struct {
	Name     string
	Path     string // non-empty for notes
	IsDir    bool
	IsActive bool
	IsOpen   bool // dir should render <details open>
	Children []*FileNode
}

// buildTree converts a flat note list into a sorted folder tree.
// Directories sort before files; both groups are sorted alphabetically.
func buildTree(notes []zk.Note, activePath string) []*FileNode {
	type treeEntry struct {
		node     *FileNode
		children map[string]*treeEntry
	}
	root := &treeEntry{children: map[string]*treeEntry{}}

	for _, note := range notes {
		parts := strings.Split(note.Path, "/")
		cur := root
		for i, part := range parts {
			if _, exists := cur.children[part]; !exists {
				var node *FileNode
				if i == len(parts)-1 {
					name := note.Title
					if name == "" {
						name = strings.TrimSuffix(part, ".md")
					}
					node = &FileNode{Name: name, Path: note.Path, IsActive: note.Path == activePath}
				} else {
					node = &FileNode{Name: part, IsDir: true}
				}
				cur.children[part] = &treeEntry{node: node, children: map[string]*treeEntry{}}
			}
			cur = cur.children[part]
		}
	}

	// flatten returns children and whether any descendant is active.
	var flatten func(*treeEntry) ([]*FileNode, bool)
	flatten = func(e *treeEntry) ([]*FileNode, bool) {
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
		nodes := make([]*FileNode, 0, len(e.children))
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

// FolderEntry is one item (file or subdirectory) in a folder listing.
type FolderEntry struct {
	Name  string
	Path  string
	Title string
	IsDir bool
}

type pageData struct {
	Title         string
	Query         string
	ActiveTag     string
	ActivePath    string
	Tags          []zk.Tag
	Notes         []zk.Note
	Tree          []*FileNode
	CurrentNote   *zk.Note
	NoteHTML      template.HTML
	Headings      []render.Heading
	OutgoingLinks []zk.Link
	Backlinks     []zk.Link
	Breadcrumbs   []BreadcrumbSegment
	FolderName    string
	FolderEntries []FolderEntry
	ManifestJSON  template.JS
}

type manifestEntry struct {
	Title string   `json:"title"`
	Path  string   `json:"path"`
	Tags  []string `json:"tags"`
}

func buildManifest(notes []zk.Note) template.JS {
	entries := make([]manifestEntry, len(notes))
	for i, n := range notes {
		tags := n.Tags
		if tags == nil {
			tags = []string{}
		}
		entries[i] = manifestEntry{Title: n.Title, Path: n.Path, Tags: tags}
	}
	b, _ := json.Marshal(entries)
	return template.JS(b)
}

func (d *pageData) IsActiveTag(name string) bool  { return d.ActiveTag == name }
func (d *pageData) IsActiveNote(path string) bool { return d.ActivePath == path }

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
	renderTemplate(w, s.tmpl, "layout.html", &pageData{Tags: tags, Tree: buildTree(notes, ""), ManifestJSON: buildManifest(notes)})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	activeTag := strings.TrimSpace(r.URL.Query().Get("tags"))
	folder := strings.TrimSpace(r.URL.Query().Get("folder"))

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
		renderTemplate(w, s.tmpl, "tree", &pageData{Tree: buildTree(notes, "")})
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
	renderTemplate(w, s.tmpl, "list", &pageData{Query: q, ActiveTag: activeTag, Notes: notes})
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
		http.NotFound(w, r)
		return
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
	tags, _ := s.store.AllTags()
	outLinks, _ := s.store.OutgoingLinks(notePath)
	backlinks, _ := s.store.Backlinks(notePath)
	renderTemplate(w, s.tmpl, "layout.html", &pageData{
		Title:         note.Title,
		ActivePath:    notePath,
		Tags:          tags,
		Tree:          buildTree(notes, notePath),
		CurrentNote:   note,
		NoteHTML:      template.HTML(result.HTML),
		Headings:      result.Headings,
		OutgoingLinks: outLinks,
		Backlinks:     backlinks,
		Breadcrumbs:   buildBreadcrumbs(notePath),
		ManifestJSON:  buildManifest(notes),
	})
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
			renderTemplate(w, s.tmpl, "layout.html", &pageData{
				Title:         note.Title,
				ActivePath:    note.Path,
				Tags:          tags,
				Tree:          buildTree(notes, note.Path),
				CurrentNote:   note,
				NoteHTML:      template.HTML(result.HTML),
				Headings:      result.Headings,
				OutgoingLinks: outLinks,
				Backlinks:     backlinks,
				Breadcrumbs:   buildBreadcrumbs(note.Path),
				ManifestJSON:  buildManifest(notes),
			})
			return
		}
	}

	// Build directory listing from notes whose path starts with folderPath/.
	prefix := folderPath + "/"
	seen := map[string]bool{}
	var entries []FolderEntry
	for _, n := range notes {
		if !strings.HasPrefix(n.Path, prefix) {
			continue
		}
		rest := strings.TrimPrefix(n.Path, prefix)
		parts := strings.SplitN(rest, "/", 2)
		if len(parts) == 1 {
			entries = append(entries, FolderEntry{Name: parts[0], Path: n.Path, Title: n.Title})
		} else if !seen[parts[0]] {
			seen[parts[0]] = true
			entries = append(entries, FolderEntry{Name: parts[0], Path: folderPath + "/" + parts[0], IsDir: true})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir != entries[j].IsDir {
			return entries[i].IsDir
		}
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})

	parts := strings.Split(folderPath, "/")
	folderName := parts[len(parts)-1]

	renderTemplate(w, s.tmpl, "layout.html", &pageData{
		Title:         folderName,
		Tags:          tags,
		Tree:          buildTree(notes, ""),
		Breadcrumbs:   buildBreadcrumbs(folderPath),
		FolderName:    folderName,
		FolderEntries: entries,
		ManifestJSON:  buildManifest(notes),
	})
}

func renderTemplate(w http.ResponseWriter, tmpl *template.Template, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}
