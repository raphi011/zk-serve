package zk_test

import (
	"strings"
	"testing"

	"github.com/raphaelgruber/zk-serve/internal/zk"
)

func openTestDB(t *testing.T) *zk.DB {
	t.Helper()
	skipIfNoDB(t)
	db, err := zk.OpenDB(testDBPath, testNotebookPath)
	if err != nil {
		t.Fatalf("OpenDB: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestSearchByQuery(t *testing.T) {
	db := openTestDB(t)

	notes, err := db.Search("goroutines", nil)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(notes) == 0 {
		t.Fatal("expected at least 1 result for 'goroutines'")
	}
	if notes[0].Path != "notes/go-concurrency.md" {
		t.Errorf("top result = %q, want notes/go-concurrency.md", notes[0].Path)
	}
}

func TestSearchByTag(t *testing.T) {
	db := openTestDB(t)

	notes, err := db.Search("", []string{"database"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("got %d results, want 1", len(notes))
	}
	if notes[0].Path != "notes/databases.md" {
		t.Errorf("result = %q", notes[0].Path)
	}
}

func TestSearchQueryAndTag(t *testing.T) {
	db := openTestDB(t)

	// "goroutines" matches go-concurrency, but filtering by "database" tag
	// should return nothing.
	notes, err := db.Search("goroutines", []string{"database"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(notes) != 0 {
		t.Errorf("expected 0 results, got %d", len(notes))
	}
}

func TestSearchSnippetMarkers(t *testing.T) {
	db := openTestDB(t)

	notes, err := db.Search("goroutines", nil)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(notes) == 0 {
		t.Fatal("expected results")
	}
	if !strings.Contains(notes[0].Snippet, "⟪MARK_START⟫") {
		t.Errorf("snippet missing MARK_START marker: %q", notes[0].Snippet)
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	db := openTestDB(t)

	// Empty query + empty tags → should return nothing.
	notes, err := db.Search("", nil)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(notes) != 0 {
		t.Errorf("expected 0 results for empty search, got %d", len(notes))
	}
}

func TestSearchTagGlob(t *testing.T) {
	db := openTestDB(t)

	// GLOB pattern: "machine*" should match "machine-learning" tag.
	notes, err := db.Search("", []string{"machine*"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(notes) != 1 {
		t.Fatalf("got %d results, want 1", len(notes))
	}
	if notes[0].Path != "notes/ai/machine-learning.md" {
		t.Errorf("result = %q", notes[0].Path)
	}
}

func TestSearchLimit(t *testing.T) {
	db := openTestDB(t)

	// Even a broad query should not exceed 200 results.
	notes, err := db.Search("the", nil)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(notes) > 200 {
		t.Errorf("got %d results, limit should cap at 200", len(notes))
	}
}
