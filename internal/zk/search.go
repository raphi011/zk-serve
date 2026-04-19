package zk

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Search returns notes matching the FTS query and/or tag filter, ordered by
// BM25 relevance when q is present, else by sortable_path. Returns an empty
// slice when both q and tags are empty.
func (d *DB) Search(q string, tags []string) ([]Note, error) {
	if q == "" && len(tags) == 0 {
		return nil, nil
	}

	if q != "" {
		return d.searchFTS(q, tags)
	}
	return d.searchByTags(tags)
}

// searchFTS runs an FTS5 MATCH query, optionally filtered by tags. Uses a
// subquery so that bm25/snippet auxiliary functions work (they require the FTS
// table to be the primary FROM without GROUP BY).
func (d *DB) searchFTS(q string, tags []string) ([]Note, error) {
	fts := ConvertQuery(q)
	if fts == "" {
		return nil, nil
	}

	var (
		innerClauses []string
		args         []any
	)

	innerClauses = append(innerClauses, "notes_fts MATCH ?")
	args = append(args, fts)

	for _, tag := range tags {
		innerClauses = append(innerClauses, `n.id IN (
			SELECT note_id FROM notes_collections
			WHERE collection_id IN (
				SELECT id FROM collections WHERE kind = 'tag' AND name GLOB ?
			))`)
		args = append(args, tag)
	}

	innerWhere := strings.Join(innerClauses, " AND ")

	query := fmt.Sprintf(`
		SELECT r.path, r.title, r.lead,
		       COALESCE(GROUP_CONCAT(c.name, char(1)), '') AS tags,
		       r.snippet
		FROM (
			SELECT n.id, n.path, n.title, n.lead,
			       snippet(notes_fts, 2, '⟪MARK_START⟫', '⟪MARK_END⟫', '…', 20) AS snippet,
			       bm25(notes_fts, 1000.0, 500.0, 1.0) AS rank
			FROM notes_fts
			JOIN notes n ON n.id = notes_fts.rowid
			WHERE %s
			ORDER BY rank
			LIMIT 200
		) r
		LEFT JOIN notes_collections nc ON nc.note_id = r.id
		LEFT JOIN collections       c  ON c.id = nc.collection_id AND c.kind = 'tag'
		GROUP BY r.id
		ORDER BY r.rank`, innerWhere)

	return d.execSearch(query, args)
}

// searchByTags returns notes matching all given tags, ordered by sortable_path.
func (d *DB) searchByTags(tags []string) ([]Note, error) {
	var (
		clauses []string
		args    []any
	)

	for _, tag := range tags {
		clauses = append(clauses, `n.id IN (
			SELECT note_id FROM notes_collections
			WHERE collection_id IN (
				SELECT id FROM collections WHERE kind = 'tag' AND name GLOB ?
			))`)
		args = append(args, tag)
	}

	query := fmt.Sprintf(`
		SELECT n.path, n.title, n.lead,
		       COALESCE(GROUP_CONCAT(c.name, char(1)), '') AS tags,
		       '' AS snippet
		FROM notes n
		LEFT JOIN notes_collections nc ON nc.note_id = n.id
		LEFT JOIN collections       c  ON c.id = nc.collection_id AND c.kind = 'tag'
		WHERE %s
		GROUP BY n.id
		ORDER BY n.sortable_path
		LIMIT 200`, strings.Join(clauses, " AND "))

	return d.execSearch(query, args)
}

// execSearch runs the given query and scans the result into a []Note.
func (d *DB) execSearch(query string, args []any) ([]Note, error) {
	rows, err := d.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer rows.Close()

	var notes []Note
	for rows.Next() {
		var (
			n       Note
			tagsRaw string
		)
		if err := rows.Scan(&n.Path, &n.Title, &n.Lead, &tagsRaw, &n.Snippet); err != nil {
			return nil, fmt.Errorf("scan search result: %w", err)
		}
		n.Filename = filepath.Base(n.Path)
		n.FilenameStem = strings.TrimSuffix(n.Filename, ".md")
		n.AbsPath = filepath.Join(d.notebookPath, n.Path)
		if tagsRaw != "" {
			n.Tags = strings.Split(tagsRaw, "\x01")
		}
		notes = append(notes, n)
	}
	return notes, rows.Err()
}
