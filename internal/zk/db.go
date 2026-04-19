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

// Link represents a connection between notes, from zk's links table.
type Link struct {
	SourcePath  string
	SourceTitle string
	TargetPath  string
	TargetTitle string
	Href        string
	IsExternal  bool
	SourceTags  []string // populated for backlinks (tags of the source note)
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

// OutgoingLinks returns all links originating from the note at path.
func (d *DB) OutgoingLinks(path string) ([]Link, error) {
	const query = `
		SELECT source_path, source_title,
		       COALESCE(target_path, '') AS target_path,
		       COALESCE(target_title, '') AS target_title,
		       href, external
		FROM resolved_links
		WHERE source_path = ?
		ORDER BY external, title`

	rows, err := d.db.Query(query, path)
	if err != nil {
		return nil, fmt.Errorf("query outgoing links: %w", err)
	}
	defer rows.Close()

	var links []Link
	for rows.Next() {
		var l Link
		var ext int
		if err := rows.Scan(&l.SourcePath, &l.SourceTitle, &l.TargetPath, &l.TargetTitle, &l.Href, &ext); err != nil {
			return nil, fmt.Errorf("scan outgoing link: %w", err)
		}
		l.IsExternal = ext != 0
		links = append(links, l)
	}
	return links, rows.Err()
}

// Backlinks returns all notes that link TO the note at path (internal links only).
// SourceTags is populated with the tags of each source note.
func (d *DB) Backlinks(path string) ([]Link, error) {
	const query = `
		SELECT rl.source_path, rl.source_title, rl.target_path, rl.target_title,
		       rl.href, rl.external,
		       COALESCE(GROUP_CONCAT(c.name, char(1)), '') AS source_tags
		FROM resolved_links rl
		LEFT JOIN notes n ON n.path = rl.source_path
		LEFT JOIN notes_collections nc ON nc.note_id = n.id
		LEFT JOIN collections c ON c.id = nc.collection_id AND c.kind = 'tag'
		WHERE rl.target_path = ? AND rl.external = 0
		GROUP BY rl.id
		ORDER BY rl.source_title`

	rows, err := d.db.Query(query, path)
	if err != nil {
		return nil, fmt.Errorf("query backlinks: %w", err)
	}
	defer rows.Close()

	var links []Link
	for rows.Next() {
		var (
			l       Link
			ext     int
			tagsRaw string
		)
		if err := rows.Scan(&l.SourcePath, &l.SourceTitle, &l.TargetPath, &l.TargetTitle, &l.Href, &ext, &tagsRaw); err != nil {
			return nil, fmt.Errorf("scan backlink: %w", err)
		}
		l.IsExternal = ext != 0
		if tagsRaw != "" {
			l.SourceTags = strings.Split(tagsRaw, "\x01")
		}
		links = append(links, l)
	}
	return links, rows.Err()
}
