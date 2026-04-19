package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/raphaelgruber/zk-serve/internal/server"
	"github.com/raphaelgruber/zk-serve/internal/zk"
)

func main() {
	var (
		addr     string
		notebook string
		open     bool
	)

	root := &cobra.Command{
		Use:   "zk-serve",
		Short: "Read-only web viewer for a zk zettelkasten notebook",
		Long: `zk-serve starts an HTTP server that renders your zk notebook as a
dark-academic web viewer with live search, tag filtering, and Markdown rendering.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if notebook == "" {
				notebook = os.Getenv("ZK_NOTEBOOK_DIR")
			}
			if notebook == "" {
				return fmt.Errorf("notebook path required: use --notebook or set ZK_NOTEBOOK_DIR")
			}
			dbPath := filepath.Join(notebook, ".zk", "notebook.db")
			if _, err := os.Stat(dbPath); err != nil {
				return fmt.Errorf("notebook database not found at %s — run 'zk index' first: %w", dbPath, err)
			}
			store, err := zk.OpenDB(dbPath, notebook)
			if err != nil {
				return fmt.Errorf("open notebook db: %w", err)
			}
			defer store.Close()
			srv, err := server.New(store)
			if err != nil {
				return fmt.Errorf("init server: %w", err)
			}
			url := "http://localhost" + addr
			if addr[0] != ':' {
				url = "http://" + addr
			}
			fmt.Fprintf(os.Stderr, "zk-serve listening on %s\n", url)
			fmt.Fprintf(os.Stderr, "notebook: %s\n", notebook)
			if open {
				openBrowser(url)
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()
			return srv.ListenAndServe(ctx, addr)
		},
	}

	root.Flags().StringVar(&addr, "addr", ":8080", "bind address (e.g. :8080 or 0.0.0.0:9000)")
	root.Flags().StringVar(&notebook, "notebook", "", "path to zk notebook (default: $ZK_NOTEBOOK_DIR)")
	root.Flags().BoolVar(&open, "open", false, "open browser on start")

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func openBrowser(url string) {
	var cmd string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "linux":
		cmd = "xdg-open"
	default:
		return
	}
	_ = exec.Command(cmd, url).Start()
}
