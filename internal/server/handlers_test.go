package server_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/raphaelgruber/zk-serve/internal/server"
	"github.com/raphaelgruber/zk-serve/internal/zk"
)

type stubStore struct {
	notes        []zk.Note
	tags         []zk.Tag
	outLinks     []zk.Link
	backLinks    []zk.Link
	notebookPath string
}

func (s *stubStore) AllNotes() ([]zk.Note, error)                      { return s.notes, nil }
func (s *stubStore) AllTags() ([]zk.Tag, error)                        { return s.tags, nil }
func (s *stubStore) Search(q string, tags []string) ([]zk.Note, error) { return s.notes, nil }
func (s *stubStore) OutgoingLinks(path string) ([]zk.Link, error)      { return s.outLinks, nil }
func (s *stubStore) Backlinks(path string) ([]zk.Link, error)          { return s.backLinks, nil }
func (s *stubStore) NotebookPath() string                              { return s.notebookPath }

var testNotes = []zk.Note{
	{
		Filename: "go-concurrency.md", FilenameStem: "go-concurrency",
		Path: "notes/go-concurrency.md", AbsPath: "/nb/notes/go-concurrency.md",
		Title: "Go Concurrency", Tags: []string{"go"},
		Modified:  time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
		Created:   time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		WordCount: 120,
	},
}
var testTags = []zk.Tag{
	{Name: "go", NoteCount: 31},
	{Name: "database", NoteCount: 18},
}

func newTestServer(t *testing.T, notes []zk.Note, tags []zk.Tag) http.Handler {
	t.Helper()
	stub := &stubStore{notes: notes, tags: tags, notebookPath: t.TempDir()}
	srv, err := server.New(stub)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return srv
}

func TestIndexRendersNoteList(t *testing.T) {
	h := newTestServer(t, testNotes, testTags)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Go Concurrency") {
		t.Errorf("expected note title in response")
	}
}

func TestIndexRendersTagList(t *testing.T) {
	h := newTestServer(t, testNotes, testTags)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if !strings.Contains(w.Body.String(), "database") {
		t.Errorf("expected tag 'database' in sidebar")
	}
}

func TestSearchReturnsHTMXPartial(t *testing.T) {
	h := newTestServer(t, testNotes, testTags)
	req := httptest.NewRequest("GET", "/search?q=go&tags=go", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "Go Concurrency") {
		t.Errorf("expected note title in search partial")
	}
	if strings.Contains(body, "<html") {
		t.Errorf("search partial must not include full page layout")
	}
}

func TestNoteHandlerRendersMarkdown(t *testing.T) {
	dir := t.TempDir()
	notePath := dir + "/test-note.md"
	if err := os.WriteFile(notePath, []byte("# Hello\n\nThis is **bold**.\n"), 0o644); err != nil {
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
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", w.Code)
	}
	if !strings.Contains(w.Body.String(), "<strong>bold</strong>") {
		t.Errorf("expected rendered markdown")
	}
}

func TestNoteHandlerNotFound(t *testing.T) {
	h := newTestServer(t, nil, nil)
	req := httptest.NewRequest("GET", "/note/nonexistent.md", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

func TestNoteHandlerIncludesManifest(t *testing.T) {
	dir := t.TempDir()
	notePath := dir + "/test-note.md"
	if err := os.WriteFile(notePath, []byte("## Section\n\nContent.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	notes := []zk.Note{
		{
			Filename: "test-note.md", FilenameStem: "test-note",
			Path: "test-note.md", AbsPath: notePath,
			Title: "Test Note", Tags: []string{"go"},
			Modified: time.Now(), Created: time.Now(),
		},
	}
	h := newTestServer(t, notes, testTags)
	req := httptest.NewRequest("GET", "/note/test-note.md", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	body := w.Body.String()
	if !strings.Contains(body, "__ZK_MANIFEST") {
		t.Error("expected __ZK_MANIFEST in page")
	}
	if !strings.Contains(body, "test-note.md") {
		t.Error("expected note path in manifest")
	}
}

func TestStaticAssetsServed(t *testing.T) {
	h := newTestServer(t, nil, nil)
	for _, path := range []string{"/static/style.css", "/static/htmx.min.js", "/static/mermaid.min.js"} {
		req := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("GET %s: status = %d, want 200", path, w.Code)
		}
	}
}
