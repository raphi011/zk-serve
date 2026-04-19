package zk_test

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

var (
	testDBPath       string // empty if zk binary not available
	testNotebookPath string
)

func TestMain(m *testing.M) {
	if _, err := exec.LookPath("zk"); err != nil {
		fmt.Fprintln(os.Stderr, "zk binary not found — DB-dependent tests will be skipped")
		os.Exit(m.Run())
	}

	dir, err := os.MkdirTemp("", "zk-test-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to create temp dir:", err)
		os.Exit(1)
	}

	// Copy testdata/notebook/ into temp dir.
	src := os.DirFS("testdata/notebook")
	_ = fs.WalkDir(src, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		dst := filepath.Join(dir, path)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		data, err := fs.ReadFile(src, path)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0o644)
	})

	// zk init + index.
	init := exec.Command("zk", "init", "--no-input", dir)
	if out, err := init.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "zk init failed: %v\n%s\n", err, out)
		os.RemoveAll(dir)
		os.Exit(1)
	}

	idx := exec.Command("zk", "index", "--no-input")
	idx.Dir = dir
	if out, err := idx.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "zk index failed: %v\n%s\n", err, out)
		os.RemoveAll(dir)
		os.Exit(1)
	}

	testNotebookPath = dir
	testDBPath = filepath.Join(dir, ".zk", "notebook.db")

	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}
