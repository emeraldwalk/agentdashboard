package conversation

import (
	"database/sql"
	"fmt"

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
    started_at    DATETIME NOT NULL,
    last_event_at DATETIME NOT NULL
);`

type sqliteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens (or creates) a SQLite database at path and runs the schema migration.
func NewSQLiteStore(path string) (Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("conversation: open sqlite: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("conversation: migrate schema: %w", err)
	}

	return &sqliteStore{db: db}, nil
}

// Upsert inserts a new conversation or updates project, title, status, last_event_at.
// started_at is set only on first insert.
func (s *sqliteStore) Upsert(c Conversation) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO conversations (id, project, title, status, started_at, last_event_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		c.ID, c.Project, c.Title, string(c.Status),
		c.StartedAt.UTC(), c.LastEventAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("conversation: upsert insert: %w", err)
	}

	_, err = s.db.Exec(
		`UPDATE conversations
		 SET project = ?, title = ?, status = ?, last_event_at = ?
		 WHERE id = ?`,
		c.Project, c.Title, string(c.Status), c.LastEventAt.UTC(), c.ID,
	)
	if err != nil {
		return fmt.Errorf("conversation: upsert update: %w", err)
	}

	return nil
}

// List returns all conversations ordered by last_event_at descending.
func (s *sqliteStore) List() ([]Conversation, error) {
	rows, err := s.db.Query(
		`SELECT id, project, title, status, started_at, last_event_at
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
		var status string
		if err := rows.Scan(
			&c.ID, &c.Project, &c.Title, &status,
			&c.StartedAt, &c.LastEventAt,
		); err != nil {
			return nil, fmt.Errorf("conversation: list scan: %w", err)
		}
		c.Status = Status(status)
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
