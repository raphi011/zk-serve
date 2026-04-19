package server

import (
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

type pageData struct {
	Title       string
	Query       string
	ActiveTag   string
	ActivePath  string
	Tags        []zk.Tag
	Notes       []zk.Note
	Tree        []*FileNode
	CurrentNote *zk.Note
	NoteHTML    template.HTML
	Breadcrumbs []BreadcrumbSegment
}

func (d *pageData) IsActiveTag(name string) bool  { return d.ActiveTag == name }
func (d *pageData) IsActiveNote(path string) bool { return d.ActivePath == path }

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	tags, err := s.zkClient.TagList()
	if err != nil {
		http.Error(w, "failed to list tags: "+err.Error(), http.StatusInternalServerError)
		return
	}
	notes, err := s.zkClient.List("", nil)
	if err != nil {
		http.Error(w, "failed to list notes: "+err.Error(), http.StatusInternalServerError)
		return
	}
	renderTemplate(w, s.tmpl, "layout.html", &pageData{Tags: tags, Tree: buildTree(notes, "")})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	activeTag := strings.TrimSpace(r.URL.Query().Get("tags"))
	folder := strings.TrimSpace(r.URL.Query().Get("folder"))

	if q == "" && activeTag == "" {
		notes, err := s.zkClient.List("", nil)
		if err != nil {
			http.Error(w, "search failed: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if folder != "" {
			prefix := folder + "/"
			filtered := notes[:0]
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
	notes, err := s.zkClient.List(q, tagFilter)
	if err != nil {
		http.Error(w, "search failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	renderTemplate(w, s.tmpl, "list", &pageData{Query: q, ActiveTag: activeTag, Notes: notes})
}

func (s *Server) handleTags(w http.ResponseWriter, r *http.Request) {
	tags, err := s.zkClient.TagList()
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
	notes, err := s.zkClient.List("", nil)
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
	rendered, err := render.Markdown(raw, lookup)
	if err != nil {
		http.Error(w, "failed to render note: "+err.Error(), http.StatusInternalServerError)
		return
	}
	tags, _ := s.zkClient.TagList()
	renderTemplate(w, s.tmpl, "layout.html", &pageData{
		Title:       note.Title,
		ActivePath:  notePath,
		Tags:        tags,
		Tree:        buildTree(notes, notePath),
		CurrentNote: note,
		NoteHTML:    template.HTML(rendered),
		Breadcrumbs: buildBreadcrumbs(notePath),
	})
}

func renderTemplate(w http.ResponseWriter, tmpl *template.Template, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}
