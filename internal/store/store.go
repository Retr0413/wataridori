package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // pure-Go sqlite driver
)

// Action is the recorded operation kind.
type Action string

const (
	ActionApply    Action = "apply"
	ActionPromote  Action = "promote"
	ActionRollback Action = "rollback"
)

// Entry is one history record. See docs/spec/phase1-cli.md §3.
type Entry struct {
	ID      int64             `json:"id"`
	Time    time.Time         `json:"time"`
	Actor   string            `json:"actor"`
	Action  Action            `json:"action"`
	Env     string            `json:"env"`
	Service string            `json:"service"`
	Digest  string            `json:"digest"`
	Detail  map[string]string `json:"detail,omitempty"`
}

// Store persists operation history in SQLite.
type Store struct {
	db *sql.DB
}

// DefaultPath returns the history DB location: $WATARIDORI_DB if set,
// otherwise ~/.local/share/wataridori/history.db.
func DefaultPath() (string, error) {
	if p := os.Getenv("WATARIDORI_DB"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "wataridori", "history.db"), nil
}

// Open opens (creating directories and schema as needed) the DB at path.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrating history db: %w", err)
	}
	return &Store{db: db}, nil
}

const schema = `
CREATE TABLE IF NOT EXISTS history (
  id      INTEGER PRIMARY KEY AUTOINCREMENT,
  ts      TEXT NOT NULL,
  actor   TEXT NOT NULL,
  action  TEXT NOT NULL,
  env     TEXT NOT NULL,
  service TEXT NOT NULL,
  digest  TEXT NOT NULL,
  detail  TEXT
);
CREATE INDEX IF NOT EXISTS history_env ON history (env, id);
`

func (s *Store) Close() error { return s.db.Close() }

// Record appends an entry. A zero Time defaults to now.
func (s *Store) Record(ctx context.Context, e Entry) error {
	if e.Time.IsZero() {
		e.Time = time.Now()
	}
	detail := []byte("{}")
	if e.Detail != nil {
		var err error
		if detail, err = json.Marshal(e.Detail); err != nil {
			return err
		}
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO history (ts, actor, action, env, service, digest, detail) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		e.Time.UTC().Format(time.RFC3339), e.Actor, string(e.Action), e.Env, e.Service, e.Digest, string(detail))
	return err
}

// ListOptions filter List results.
type ListOptions struct {
	// Env filters by environment; empty means all.
	Env string
	// Limit caps the number of entries; 0 means the default of 20.
	Limit int
}

// List returns entries, newest first.
func (s *Store) List(ctx context.Context, opts ListOptions) ([]Entry, error) {
	limit := opts.Limit
	if limit <= 0 {
		limit = 20
	}
	query := `SELECT id, ts, actor, action, env, service, digest, detail FROM history`
	args := []any{}
	if opts.Env != "" {
		query += ` WHERE env = ?`
		args = append(args, opts.Env)
	}
	query += ` ORDER BY id DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var entries []Entry
	for rows.Next() {
		var e Entry
		var ts, action, detail string
		if err := rows.Scan(&e.ID, &ts, &e.Actor, &action, &e.Env, &e.Service, &e.Digest, &detail); err != nil {
			return nil, err
		}
		if e.Time, err = time.Parse(time.RFC3339, ts); err != nil {
			return nil, err
		}
		e.Action = Action(action)
		if detail != "" && detail != "{}" {
			if err := json.Unmarshal([]byte(detail), &e.Detail); err != nil {
				return nil, err
			}
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
