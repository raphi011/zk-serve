package zk

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

// Note represents a single note from the zk notebook.
type Note struct {
	Filename     string         `json:"filename"`
	FilenameStem string         `json:"filenameStem"`
	Path         string         `json:"path"`
	AbsPath      string         `json:"absPath"`
	Title        string         `json:"title"`
	Lead         string         `json:"lead"`
	Snippet      string         `json:"snippet,omitempty"`
	Body         string         `json:"body"`
	Snippets     []string       `json:"snippets"`
	RawContent   string         `json:"rawContent"`
	WordCount    int            `json:"wordCount"`
	Tags         []string       `json:"tags"`
	Metadata     map[string]any `json:"metadata"`
	Created      time.Time      `json:"created"`
	Modified     time.Time      `json:"modified"`
	Checksum     string         `json:"checksum"`
}

// Tag represents a tag with its note count.
type Tag struct {
	ID        int    `json:"id"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	NoteCount int    `json:"noteCount"`
}

// DB provides read-only access to a zk notebook's SQLite database.
type DB struct {
	db           *sql.DB
	notebookPath string
}

// OpenDB opens the zk notebook database in read-only mode.
func OpenDB(dbPath, notebookPath string) (*DB, error) {
	dsn := fmt.Sprintf("file:%s?mode=ro&_pragma=busy_timeout(5000)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open notebook db: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping notebook db: %w", err)
	}
	return &DB{db: db, notebookPath: notebookPath}, nil
}

// Close closes the underlying database connection.
func (d *DB) Close() error {
	return d.db.Close()
}

// AllNotes returns every note ordered by sortable_path.
func (d *DB) AllNotes() ([]Note, error) {
	const query = `
		SELECT path, title, lead, metadata, COALESCE(tags, '')
		FROM notes_with_metadata
		ORDER BY sortable_path`

	rows, err := d.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query notes: %w", err)
	}
	defer rows.Close()

	var notes []Note
	for rows.Next() {
		var (
			n           Note
			metadataRaw string
			tagsRaw     string
		)
		if err := rows.Scan(&n.Path, &n.Title, &n.Lead, &metadataRaw, &tagsRaw); err != nil {
			return nil, fmt.Errorf("scan note: %w", err)
		}
		n.Filename = filepath.Base(n.Path)
		n.FilenameStem = strings.TrimSuffix(n.Filename, ".md")
		n.AbsPath = filepath.Join(d.notebookPath, n.Path)
		if tagsRaw != "" {
			n.Tags = strings.Split(tagsRaw, "\x01")
		}
		if metadataRaw != "" {
			_ = json.Unmarshal([]byte(metadataRaw), &n.Metadata)
		}
		notes = append(notes, n)
	}
	return notes, rows.Err()
}

// AllTags returns all tags sorted by name with their note counts.
func (d *DB) AllTags() ([]Tag, error) {
	const query = `
		SELECT c.name, COUNT(nc.note_id)
		FROM collections c
		LEFT JOIN notes_collections nc ON nc.collection_id = c.id
		WHERE c.kind = 'tag'
		GROUP BY c.id
		ORDER BY c.name`

	rows, err := d.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("query tags: %w", err)
	}
	defer rows.Close()

	var tags []Tag
	for rows.Next() {
		var t Tag
		if err := rows.Scan(&t.Name, &t.NoteCount); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		t.Kind = "tag"
		tags = append(tags, t)
	}
	return tags, rows.Err()
}
