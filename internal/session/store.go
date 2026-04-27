package session

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Store persists and retrieves Session records.
type Store interface {
	Upsert(s Session) error
	List() ([]Session, error)
	Close() error
}

const schema = `
CREATE TABLE IF NOT EXISTS sessions (
    id            TEXT PRIMARY KEY,
    agent_name    TEXT NOT NULL,
    status        TEXT NOT NULL,
    started_at    DATETIME NOT NULL,
    last_event_at DATETIME NOT NULL
);`

type sqliteStore struct {
	db *sql.DB
}

// NewSQLiteStore opens (or creates) a SQLite database at path and runs the
// schema migration. Use ":memory:" for an in-memory database.
func NewSQLiteStore(path string) (Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("session: open sqlite: %w", err)
	}

	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("session: migrate schema: %w", err)
	}

	return &sqliteStore{db: db}, nil
}

// Upsert inserts a new session or updates status and last_event_at if the
// session already exists. started_at is never overwritten on update.
func (s *sqliteStore) Upsert(sess Session) error {
	_, err := s.db.Exec(
		`INSERT OR IGNORE INTO sessions (id, agent_name, status, started_at, last_event_at)
		 VALUES (?, ?, ?, ?, ?)`,
		sess.ID, sess.AgentName, string(sess.Status),
		sess.StartedAt.UTC(), sess.LastEventAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("session: upsert insert: %w", err)
	}

	_, err = s.db.Exec(
		`UPDATE sessions
		 SET agent_name = ?, status = ?, last_event_at = ?
		 WHERE id = ?`,
		sess.AgentName, string(sess.Status), sess.LastEventAt.UTC(), sess.ID,
	)
	if err != nil {
		return fmt.Errorf("session: upsert update: %w", err)
	}

	return nil
}

// List returns all sessions ordered by last_event_at descending.
func (s *sqliteStore) List() ([]Session, error) {
	rows, err := s.db.Query(
		`SELECT id, agent_name, status, started_at, last_event_at
		 FROM sessions
		 ORDER BY last_event_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("session: list query: %w", err)
	}
	defer rows.Close()

	var sessions []Session
	for rows.Next() {
		var sess Session
		var status string
		if err := rows.Scan(
			&sess.ID, &sess.AgentName, &status,
			&sess.StartedAt, &sess.LastEventAt,
		); err != nil {
			return nil, fmt.Errorf("session: list scan: %w", err)
		}
		sess.Status = Status(status)
		sessions = append(sessions, sess)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("session: list rows: %w", err)
	}

	return sessions, nil
}

// Close releases the underlying database connection.
func (s *sqliteStore) Close() error {
	return s.db.Close()
}
