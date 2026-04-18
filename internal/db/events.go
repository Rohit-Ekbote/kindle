package db

import (
	"database/sql"
	"errors"
	"time"
)

var ErrNotFound = errors.New("event not found")

const (
	OutcomeInProgress = "in_progress"
	OutcomeSuccess    = "success"
	OutcomeFailure    = "failure"
)

type Event struct {
	ID          int64     `json:"id"`
	EnvName     string    `json:"env_name"`
	ActorEmail  string    `json:"actor_email"`
	ActionType  string    `json:"action_type"`
	Description string    `json:"description"`
	Outcome     string    `json:"outcome"`
	LogPath     string    `json:"log_path"`
	CreatedAt   time.Time `json:"created_at"`
}

func (d *DB) WriteEvent(e Event) error {
	_, err := d.conn.Exec(
		`INSERT INTO env_events (env_name, actor_email, action_type, description, outcome, log_path)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		e.EnvName, e.ActorEmail, e.ActionType, e.Description, e.Outcome, nullString(e.LogPath),
	)
	return err
}

func (d *DB) UpdateOutcome(envName, actionType, outcome string) error {
	result, err := d.conn.Exec(
		`UPDATE env_events SET outcome = ?
		 WHERE id = (
		   SELECT id FROM env_events WHERE env_name = ? AND action_type = ?
		   ORDER BY created_at DESC, id DESC LIMIT 1
		 )`,
		outcome, envName, actionType,
	)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (d *DB) ListEvents(envName string, limit, offset int) ([]Event, error) {
	rows, err := d.conn.Query(
		`SELECT id, env_name, actor_email, action_type, description, outcome, log_path, created_at
		 FROM env_events WHERE env_name = ?
		 ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		envName, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]Event, 0)
	for rows.Next() {
		var e Event
		var logPath sql.NullString
		if err := rows.Scan(&e.ID, &e.EnvName, &e.ActorEmail, &e.ActionType,
			&e.Description, &e.Outcome, &logPath, &e.CreatedAt); err != nil {
			return nil, err
		}
		e.LogPath = logPath.String
		events = append(events, e)
	}
	return events, rows.Err()
}

func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}
