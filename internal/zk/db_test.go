package zk_test

import (
	"testing"

	"github.com/raphaelgruber/zk-serve/internal/zk"
)

func skipIfNoDB(t *testing.T) {
	t.Helper()
	if testDBPath == "" {
		t.Skip("zk binary not available — skipping DB test")
	}
}

func TestAllNotes(t *testing.T) {
	skipIfNoDB(t)
	db, err := zk.OpenDB(testDBPath, testNotebookPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	notes, err := db.AllNotes()
	if err != nil {
		t.Fatalf("AllNotes: %v", err)
	}

	// Fixture has 4 notes.
	if len(notes) != 4 {
		t.Fatalf("got %d notes, want 4", len(notes))
	}

	// Verify sorted by sortable_path (index.md first, then notes/...).
	if notes[0].Path != "index.md" {
		t.Errorf("first note path = %q, want index.md", notes[0].Path)
	}

	// Spot-check a note with tags.
	var gc *zk.Note
	for i := range notes {
		if notes[i].Path == "notes/go-concurrency.md" {
			gc = &notes[i]
			break
		}
	}
	if gc == nil {
		t.Fatal("notes/go-concurrency.md not found")
	}
	if gc.Title != "Go Concurrency" {
		t.Errorf("Title = %q", gc.Title)
	}
	if gc.Filename != "go-concurrency.md" {
		t.Errorf("Filename = %q", gc.Filename)
	}
	if gc.FilenameStem != "go-concurrency" {
		t.Errorf("FilenameStem = %q", gc.FilenameStem)
	}
	if gc.AbsPath == "" {
		t.Error("AbsPath is empty")
	}
	if len(gc.Tags) < 2 {
		t.Errorf("Tags = %v, want at least [go, concurrency]", gc.Tags)
	}
}

func TestAllTags(t *testing.T) {
	skipIfNoDB(t)
	db, err := zk.OpenDB(testDBPath, testNotebookPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	tags, err := db.AllTags()
	if err != nil {
		t.Fatalf("AllTags: %v", err)
	}

	// Fixture has 6 distinct tags: ai, concurrency, database, go, machine-learning, meta.
	if len(tags) < 5 {
		t.Fatalf("got %d tags, want at least 5", len(tags))
	}

	// Tags should be sorted by name.
	for i := 1; i < len(tags); i++ {
		if tags[i].Name < tags[i-1].Name {
			t.Errorf("tags not sorted: %q before %q", tags[i-1].Name, tags[i].Name)
		}
	}

	// Each tag should have NoteCount >= 1.
	for _, tag := range tags {
		if tag.NoteCount < 1 {
			t.Errorf("tag %q has NoteCount %d", tag.Name, tag.NoteCount)
		}
	}
}
