package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

type DB struct {
	conn *sql.DB
}

func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	d := &DB{conn: conn}
	if err := d.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return d, nil
}

func (d *DB) Close() error { return d.conn.Close() }

func (d *DB) migrate() error {
	_, err := d.conn.Exec(`
		CREATE TABLE IF NOT EXISTS env_events (
			id          INTEGER PRIMARY KEY AUTOINCREMENT,
			env_name    TEXT NOT NULL,
			actor_email TEXT NOT NULL,
			action_type TEXT NOT NULL,
			description TEXT NOT NULL,
			outcome     TEXT NOT NULL,
			log_path    TEXT,
			created_at  DATETIME NOT NULL DEFAULT (datetime('now'))
		);
		CREATE INDEX IF NOT EXISTS idx_env_events_env
			ON env_events(env_name, created_at DESC);
	`)
	return err
}
