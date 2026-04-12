package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Session struct {
	ID          string    `json:"id"`
	DisplayName string    `json:"display_name"`
	Harness     string    `json:"harness"`
	State       string    `json:"state"`
	PID         int       `json:"pid,omitempty"`
	AgentID     string    `json:"agent_id,omitempty"`
	SpawnerID   string    `json:"spawner_id,omitempty"`
	ParentID    string    `json:"parent_id,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type Store struct {
	db *sql.DB
}

func New(dbPath string) (*Store, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}
	d, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	s := &Store{db: d}
	if err := s.migrate(); err != nil {
		d.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS sessions (
			id           TEXT PRIMARY KEY,
			display_name TEXT NOT NULL DEFAULT '',
			harness      TEXT NOT NULL,
			state        TEXT NOT NULL,
			pid          INTEGER NOT NULL DEFAULT 0,
			agent_id     TEXT NOT NULL DEFAULT '',
			spawner_id   TEXT NOT NULL DEFAULT '',
			parent_id    TEXT NOT NULL DEFAULT '',
			created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_sessions_state ON sessions(state);
		CREATE INDEX IF NOT EXISTS idx_sessions_harness ON sessions(harness);
	`)
	if err != nil {
		return err
	}
	// Migration for existing DBs
	s.db.Exec("ALTER TABLE sessions ADD COLUMN parent_id TEXT NOT NULL DEFAULT ''")
	return nil
}

func (s *Store) CreateSession(sess *Session) error {
	now := time.Now().UTC()
	sess.CreatedAt = now
	sess.UpdatedAt = now
	_, err := s.db.Exec(
		`INSERT INTO sessions (id, display_name, harness, state, pid, agent_id, spawner_id, parent_id, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?)`,
		sess.ID, sess.DisplayName, sess.Harness, sess.State, sess.PID, sess.AgentID, sess.SpawnerID, sess.ParentID, sess.CreatedAt, sess.UpdatedAt,
	)
	return err
}

func (s *Store) GetSession(id string) (*Session, error) {
	var sess Session
	err := s.db.QueryRow(
		`SELECT id, display_name, harness, state, pid, agent_id, spawner_id, parent_id, created_at, updated_at FROM sessions WHERE id=?`,
		id,
	).Scan(&sess.ID, &sess.DisplayName, &sess.Harness, &sess.State, &sess.PID, &sess.AgentID, &sess.SpawnerID, &sess.ParentID, &sess.CreatedAt, &sess.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &sess, nil
}

func (s *Store) ListSessions() ([]Session, error) {
	rows, err := s.db.Query(`SELECT id, display_name, harness, state, pid, agent_id, spawner_id, parent_id, created_at, updated_at FROM sessions ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sessions []Session
	for rows.Next() {
		var sess Session
		if err := rows.Scan(&sess.ID, &sess.DisplayName, &sess.Harness, &sess.State, &sess.PID, &sess.AgentID, &sess.SpawnerID, &sess.ParentID, &sess.CreatedAt, &sess.UpdatedAt); err != nil {
			return nil, err
		}
		sessions = append(sessions, sess)
	}
	return sessions, rows.Err()
}

func (s *Store) ListSessionsByState(state string) ([]Session, error) {
	rows, err := s.db.Query(`SELECT id, display_name, harness, state, pid, agent_id, spawner_id, parent_id, created_at, updated_at FROM sessions WHERE state=? ORDER BY created_at DESC`, state)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var sessions []Session
	for rows.Next() {
		var sess Session
		if err := rows.Scan(&sess.ID, &sess.DisplayName, &sess.Harness, &sess.State, &sess.PID, &sess.AgentID, &sess.SpawnerID, &sess.ParentID, &sess.CreatedAt, &sess.UpdatedAt); err != nil {
			return nil, err
		}
		sessions = append(sessions, sess)
	}
	return sessions, rows.Err()
}

func (s *Store) UpdateSessionState(id, state string) error {
	now := time.Now().UTC()
	res, err := s.db.Exec(`UPDATE sessions SET state=?, updated_at=? WHERE id=?`, state, now, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) UpdateSessionPID(id string, pid int) error {
	now := time.Now().UTC()
	res, err := s.db.Exec(`UPDATE sessions SET pid=?, updated_at=? WHERE id=?`, pid, now, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) DeleteSession(id string) error {
	res, err := s.db.Exec(`DELETE FROM sessions WHERE id=?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}
