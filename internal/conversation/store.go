package conversation

import (
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

// Store persists and retrieves Conversation records.
type Store interface {
	Upsert(c Conversation) error
	List() ([]Conversation, error)
	Close() error
}

const schema = `
CREATE TABLE IF NOT EXISTS conversations (
    id            TEXT PRIMARY KEY,
    project       TEXT NOT NULL,
    title         TEXT NOT NULL DEFAULT '',
    status        TEXT NOT NULL,
    source        TEXT NOT NULL DEFAULT 'host',
    started_at    DATETIME NOT NULL,
    last_event_at DATETIME NOT NULL,
    is_subagent   INTEGER NOT NULL DEFAULT 0,
    parent_id     TEXT NOT NULL DEFAULT ''
);`

const migration01 = `ALTER TABLE conversations ADD COLUMN source TEXT NOT NULL DEFAULT 'host';`

const migration02 = `ALTER TABLE conversations ADD COLUMN is_subagent INTEGER NOT NULL DEFAULT 0;
ALTER TABLE conversations ADD COLUMN parent_id TEXT NOT NULL DEFAULT '';`

type sqliteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens (or creates) a SQLite database at path and runs the schema migration.
func NewSQLiteStore(path string) (Store, error) {
	db, err := sql.Open("sqlite", path+"?_busy_timeout=5000&_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("conversation: open sqlite: %w", err)
	}
	db.SetMaxOpenConns(1)

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("conversation: migrate schema: %w", err)
	}

	// Add source column to existing databases that predate this field.
	_, _ = db.Exec(migration01)
	// Add is_subagent and parent_id columns to existing databases.
	for _, stmt := range strings.Split(migration02, "\n") {
		_, _ = db.Exec(stmt)
	}

	return &sqliteStore{db: db}, nil
}

// Upsert inserts or updates a conversation, preserving started_at on conflict.
func (s *sqliteStore) Upsert(c Conversation) error {
	isSubagent := 0
	if c.IsSubagent {
		isSubagent = 1
	}
	_, err := s.db.Exec(
		`INSERT INTO conversations (id, project, title, status, source, started_at, last_event_at, is_subagent, parent_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   project       = excluded.project,
		   title         = excluded.title,
		   status        = excluded.status,
		   source        = excluded.source,
		   last_event_at = excluded.last_event_at,
		   is_subagent   = excluded.is_subagent,
		   parent_id     = excluded.parent_id`,
		c.ID, c.Project, c.Title, string(c.Status), string(c.Source),
		c.StartedAt.UTC(), c.LastEventAt.UTC(), isSubagent, c.ParentID,
	)
	if err != nil {
		return fmt.Errorf("conversation: upsert: %w", err)
	}
	return nil
}

// List returns all conversations ordered by last_event_at descending.
func (s *sqliteStore) List() ([]Conversation, error) {
	rows, err := s.db.Query(
		`SELECT id, project, title, status, source, started_at, last_event_at, is_subagent, parent_id
		 FROM conversations
		 ORDER BY last_event_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("conversation: list query: %w", err)
	}
	defer rows.Close()

	var convs []Conversation
	for rows.Next() {
		var c Conversation
		var status, source string
		var isSubagent int
		if err := rows.Scan(
			&c.ID, &c.Project, &c.Title, &status, &source,
			&c.StartedAt, &c.LastEventAt, &isSubagent, &c.ParentID,
		); err != nil {
			return nil, fmt.Errorf("conversation: list scan: %w", err)
		}
		c.Status = Status(status)
		c.Source = Source(source)
		c.IsSubagent = isSubagent != 0
		convs = append(convs, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("conversation: list rows: %w", err)
	}

	return convs, nil
}

func (s *sqliteStore) Close() error {
	return s.db.Close()
}
