package server

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strings"

	"github.com/raphaelgruber/zk-serve/internal/render"
	"github.com/raphaelgruber/zk-serve/internal/zk"
)

type pageData struct {
	Title       string
	Query       string
	ActiveTag   string
	ActivePath  string
	Tags        []zk.Tag
	Notes       []zk.Note
	CurrentNote *zk.Note
	NoteHTML    template.HTML
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
	renderTemplate(w, s.tmpl, "layout.html", &pageData{Tags: tags, Notes: notes})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	activeTag := strings.TrimSpace(r.URL.Query().Get("tags"))
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
	rendered, err := render.Markdown(raw)
	if err != nil {
		http.Error(w, "failed to render note: "+err.Error(), http.StatusInternalServerError)
		return
	}
	tags, _ := s.zkClient.TagList()
	renderTemplate(w, s.tmpl, "layout.html", &pageData{
		Title:       note.Title,
		ActivePath:  notePath,
		Tags:        tags,
		Notes:       notes,
		CurrentNote: note,
		NoteHTML:    template.HTML(rendered),
	})
}

func renderTemplate(w http.ResponseWriter, tmpl *template.Template, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
	}
}
