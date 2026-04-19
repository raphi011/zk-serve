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
	if gc.WordCount == 0 {
		t.Error("WordCount should be populated from DB")
	}
	if gc.Created.IsZero() {
		t.Error("Created should be populated from DB")
	}
	if gc.Modified.IsZero() {
		t.Error("Modified should be populated from DB")
	}
}

func TestOutgoingLinks(t *testing.T) {
	skipIfNoDB(t)
	db, err := zk.OpenDB(testDBPath, testNotebookPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	// machine-learning.md has a wiki-link to go-concurrency.
	links, err := db.OutgoingLinks("notes/ai/machine-learning.md")
	if err != nil {
		t.Fatalf("OutgoingLinks: %v", err)
	}
	if len(links) == 0 {
		t.Fatal("expected at least 1 outgoing link from machine-learning.md")
	}

	var found bool
	for _, l := range links {
		if l.TargetPath == "notes/go-concurrency.md" {
			found = true
			if l.IsExternal {
				t.Error("wiki-link should not be external")
			}
		}
	}
	if !found {
		t.Errorf("expected link to notes/go-concurrency.md, got %+v", links)
	}
}

func TestBacklinks(t *testing.T) {
	skipIfNoDB(t)
	db, err := zk.OpenDB(testDBPath, testNotebookPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	// go-concurrency.md is linked to by machine-learning.md.
	links, err := db.Backlinks("notes/go-concurrency.md")
	if err != nil {
		t.Fatalf("Backlinks: %v", err)
	}
	if len(links) == 0 {
		t.Fatal("expected at least 1 backlink to go-concurrency.md")
	}

	var found bool
	for _, l := range links {
		if l.SourcePath == "notes/ai/machine-learning.md" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected backlink from machine-learning.md, got %+v", links)
	}
}

func TestBacklinksEmpty(t *testing.T) {
	skipIfNoDB(t)
	db, err := zk.OpenDB(testDBPath, testNotebookPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	links, err := db.Backlinks("notes/databases.md")
	if err != nil {
		t.Fatalf("Backlinks: %v", err)
	}
	if len(links) != 0 {
		t.Errorf("expected 0 backlinks for databases.md, got %d", len(links))
	}
}

func TestActivityDays(t *testing.T) {
	skipIfNoDB(t)
	db, err := zk.OpenDB(testDBPath, testNotebookPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	// Get all notes to find a month with known activity.
	notes, err := db.AllNotes()
	if err != nil {
		t.Fatalf("AllNotes: %v", err)
	}
	if len(notes) == 0 {
		t.Fatal("expected at least 1 note")
	}

	// Use the created date of the first note to query.
	y, m, _ := notes[0].Created.Date()
	days, err := db.ActivityDays(y, int(m))
	if err != nil {
		t.Fatalf("ActivityDays: %v", err)
	}
	if len(days) == 0 {
		t.Errorf("expected at least 1 active day in %d-%02d", y, m)
	}

	// Every returned day should be 1-31.
	for d := range days {
		if d < 1 || d > 31 {
			t.Errorf("invalid day number: %d", d)
		}
	}
}

func TestActivityDaysEmptyMonth(t *testing.T) {
	skipIfNoDB(t)
	db, err := zk.OpenDB(testDBPath, testNotebookPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	// 1970-01 should have no notes.
	days, err := db.ActivityDays(1970, 1)
	if err != nil {
		t.Fatalf("ActivityDays: %v", err)
	}
	if len(days) != 0 {
		t.Errorf("expected 0 active days in 1970-01, got %d", len(days))
	}
}

func TestNotesByDate(t *testing.T) {
	skipIfNoDB(t)
	db, err := zk.OpenDB(testDBPath, testNotebookPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	// Get all notes to find a known date.
	allNotes, err := db.AllNotes()
	if err != nil {
		t.Fatalf("AllNotes: %v", err)
	}
	if len(allNotes) == 0 {
		t.Fatal("expected at least 1 note")
	}

	// Query by the created date of the first note.
	date := allNotes[0].Created.Format("2006-01-02")
	notes, err := db.NotesByDate(date)
	if err != nil {
		t.Fatalf("NotesByDate: %v", err)
	}
	if len(notes) == 0 {
		t.Errorf("expected at least 1 note for date %s", date)
	}

	// Verify the returned note's created or modified date matches.
	for _, n := range notes {
		createdDate := n.Created.Format("2006-01-02")
		modifiedDate := n.Modified.Format("2006-01-02")
		if createdDate != date && modifiedDate != date {
			t.Errorf("note %q: created=%s modified=%s, neither matches %s",
				n.Path, createdDate, modifiedDate, date)
		}
	}
}

func TestNotesByDateEmpty(t *testing.T) {
	skipIfNoDB(t)
	db, err := zk.OpenDB(testDBPath, testNotebookPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	defer db.Close()

	notes, err := db.NotesByDate("1970-01-01")
	if err != nil {
		t.Fatalf("NotesByDate: %v", err)
	}
	if len(notes) != 0 {
		t.Errorf("expected 0 notes for 1970-01-01, got %d", len(notes))
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
