package zk_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/raphaelgruber/zk-serve/internal/zk"
)

// fakeZKWithDir installs a fake zk binary that always prints output, and returns
// the notebook directory (to use as the client path) and a cleanup func.
func fakeZKWithDir(t *testing.T, output string) (notebookDir string, cleanup func()) {
	t.Helper()
	binDir := t.TempDir()
	notebookDir = t.TempDir()
	outFile := filepath.Join(binDir, "zk.out")
	if err := os.WriteFile(outFile, []byte(output), 0o644); err != nil {
		t.Fatal(err)
	}
	script := filepath.Join(binDir, "zk")
	if err := os.WriteFile(script, []byte("#!/bin/sh\ncat "+outFile), 0o755); err != nil {
		t.Fatal(err)
	}
	old := os.Getenv("PATH")
	os.Setenv("PATH", binDir+":"+old)
	return notebookDir, func() { os.Setenv("PATH", old) }
}

// fakeZK is a convenience wrapper for tests that don't need the notebook dir.
func fakeZK(t *testing.T, output string) func() {
	t.Helper()
	_, cleanup := fakeZKWithDir(t, output)
	return cleanup
}

func TestListReturnsNotes(t *testing.T) {
	notes := []map[string]any{
		{
			"filename": "go-concurrency.md", "filenameStem": "go-concurrency",
			"path": "notes/go-concurrency.md", "absPath": "/nb/notes/go-concurrency.md",
			"title": "Go Concurrency", "lead": "goroutines", "body": "goroutines",
			"snippets": []string{"goroutines"}, "rawContent": "# Go Concurrency\n",
			"wordCount": 3, "tags": []string{"go", "concurrency"},
			"metadata": map[string]any{},
			"created": "2024-01-15T00:00:00Z", "modified": "2024-06-01T10:00:00Z",
			"checksum": "abc123",
		},
	}
	raw, _ := json.Marshal(notes)
	notebookDir, cleanup := fakeZKWithDir(t, string(raw))
	defer cleanup()

	c := zk.NewClient(notebookDir)
	got, err := c.List("", nil)
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 note, got %d", len(got))
	}
	n := got[0]
	if n.Title != "Go Concurrency" {
		t.Errorf("Title = %q", n.Title)
	}
	if n.Path != "notes/go-concurrency.md" {
		t.Errorf("Path = %q", n.Path)
	}
	if n.AbsPath != "/nb/notes/go-concurrency.md" {
		t.Errorf("AbsPath = %q", n.AbsPath)
	}
	if len(n.Tags) != 2 || n.Tags[0] != "go" {
		t.Errorf("Tags = %v", n.Tags)
	}
	want := time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC)
	if !n.Created.Equal(want) {
		t.Errorf("Created = %v, want %v", n.Created, want)
	}
}

func TestListPassesQueryAndTags(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "zk")
	argsFile := filepath.Join(dir, "args.txt")
	pwdFile := filepath.Join(dir, "pwd.txt")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho \"$@\" > "+argsFile+"\npwd > "+pwdFile+"\necho '[]'"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := os.Getenv("PATH")
	os.Setenv("PATH", dir+":"+old)
	defer os.Setenv("PATH", old)

	notebook := t.TempDir()
	c := zk.NewClient(notebook)
	_, err := c.List("golang", []string{"go", "concurrency"})
	if err != nil {
		t.Fatalf("List() error: %v", err)
	}

	got, _ := os.ReadFile(argsFile)
	args := string(got)
	for _, want := range []string{"--match", "golang", "--tag", "go", "--tag", "concurrency", "--format", "json"} {
		if !containsStr(args, want) {
			t.Errorf("args %q missing %q", args, want)
		}
	}
	if containsStr(args, "--notebook") {
		t.Errorf("args must not contain --notebook flag, got: %s", args)
	}

	gotPWD, _ := os.ReadFile(pwdFile)
	wantPWD := strings.TrimSpace(notebook)
	gotPWDStr := strings.TrimSpace(string(gotPWD))
	if gotPWDStr != wantPWD {
		t.Errorf("cmd.Dir = %q, want %q", gotPWDStr, wantPWD)
	}
}

func TestListEmptyQueryOmitsMatchFlag(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "zk")
	argsFile := filepath.Join(dir, "args.txt")
	if err := os.WriteFile(script, []byte("#!/bin/sh\necho \"$@\" > "+argsFile+"\necho '[]'"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := os.Getenv("PATH")
	os.Setenv("PATH", dir+":"+old)
	defer os.Setenv("PATH", old)

	c := zk.NewClient("/nb")
	_, _ = c.List("", nil)

	got, _ := os.ReadFile(argsFile)
	if containsStr(string(got), "--match") {
		t.Errorf("expected --match omitted when query empty, got: %s", got)
	}
}

func TestTagList(t *testing.T) {
	tags := []map[string]any{
		{"id": 1, "kind": "tag", "name": "go", "noteCount": 31},
		{"id": 2, "kind": "tag", "name": "database", "noteCount": 18},
	}
	raw, _ := json.Marshal(tags)
	notebookDir, cleanup := fakeZKWithDir(t, string(raw))
	defer cleanup()

	c := zk.NewClient(notebookDir)
	got, err := c.TagList()
	if err != nil {
		t.Fatalf("TagList() error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(got))
	}
	if got[0].Name != "go" || got[0].NoteCount != 31 {
		t.Errorf("got[0] = %+v", got[0])
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
