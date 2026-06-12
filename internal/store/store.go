// Package store — метадані знімків і подій у SQLite (pure-Go, без CGO).
package store

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

// Snapshot — рядок таблиці знімків.
type Snapshot struct {
	ID      int64     `json:"id"`
	Path    string    `json:"path"`
	TakenAt time.Time `json:"taken_at"`
	Size    int64     `json:"size"`
	Trigger string    `json:"trigger"` // "schedule" | "manual" | "motion"
}

// Event — подія (рух, запис відео тощо).
type Event struct {
	ID        int64     `json:"id"`
	Type      string    `json:"type"` // "motion" | "recording"
	Path      string    `json:"path"` // шлях до кліпу/знімка, якщо є
	Note      string    `json:"note"`
	CreatedAt time.Time `json:"created_at"`
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // SQLite: уникаємо конкурентних записів
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, fmt.Errorf("міграція: %w", err)
	}
	return s, nil
}

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS snapshots (
	id       INTEGER PRIMARY KEY AUTOINCREMENT,
	path     TEXT NOT NULL,
	taken_at DATETIME NOT NULL,
	size     INTEGER NOT NULL,
	trigger  TEXT NOT NULL DEFAULT 'schedule'
);
CREATE INDEX IF NOT EXISTS idx_snapshots_taken_at ON snapshots(taken_at);

CREATE TABLE IF NOT EXISTS events (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	type       TEXT NOT NULL,
	path       TEXT NOT NULL DEFAULT '',
	note       TEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_events_created_at ON events(created_at);
`)
	return err
}

func (s *Store) AddSnapshot(snap Snapshot) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO snapshots(path, taken_at, size, trigger) VALUES (?, ?, ?, ?)`,
		snap.Path, snap.TakenAt, snap.Size, snap.Trigger,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) ListSnapshots(limit int) ([]Snapshot, error) {
	rows, err := s.db.Query(
		`SELECT id, path, taken_at, size, trigger FROM snapshots ORDER BY taken_at DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Snapshot
	for rows.Next() {
		var s Snapshot
		if err := rows.Scan(&s.ID, &s.Path, &s.TakenAt, &s.Size, &s.Trigger); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ListSnapshotsBetween повертає знімки у діапазоні [from,to] (нульові межі = без обмеження).
func (s *Store) ListSnapshotsBetween(from, to time.Time, limit int) ([]Snapshot, error) {
	rows, err := s.db.Query(
		`SELECT id, path, taken_at, size, trigger FROM snapshots
		 WHERE (?1 = 0 OR taken_at >= ?2) AND (?3 = 0 OR taken_at <= ?4)
		 ORDER BY taken_at DESC LIMIT ?5`,
		boolInt(!from.IsZero()), from, boolInt(!to.IsZero()), to, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Snapshot
	for rows.Next() {
		var s Snapshot
		if err := rows.Scan(&s.ID, &s.Path, &s.TakenAt, &s.Size, &s.Trigger); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ListRecordingsBetween повертає відеозаписи (events type=recording) у діапазоні.
func (s *Store) ListRecordingsBetween(from, to time.Time, limit int) ([]Event, error) {
	rows, err := s.db.Query(
		`SELECT id, type, path, note, created_at FROM events
		 WHERE type = 'recording' AND path <> ''
		   AND (?1 = 0 OR created_at >= ?2) AND (?3 = 0 OR created_at <= ?4)
		 ORDER BY created_at DESC LIMIT ?5`,
		boolInt(!from.IsZero()), from, boolInt(!to.IsZero()), to, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.Type, &e.Path, &e.Note, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// SnapshotByID повертає знімок за id.
func (s *Store) SnapshotByID(id int64) (Snapshot, error) {
	var sn Snapshot
	err := s.db.QueryRow(
		`SELECT id, path, taken_at, size, trigger FROM snapshots WHERE id = ?`, id,
	).Scan(&sn.ID, &sn.Path, &sn.TakenAt, &sn.Size, &sn.Trigger)
	return sn, err
}

// EventByID повертає подію за id.
func (s *Store) EventByID(id int64) (Event, error) {
	var e Event
	err := s.db.QueryRow(
		`SELECT id, type, path, note, created_at FROM events WHERE id = ?`, id,
	).Scan(&e.ID, &e.Type, &e.Path, &e.Note, &e.CreatedAt)
	return e, err
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (s *Store) AddEvent(e Event) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO events(type, path, note, created_at) VALUES (?, ?, ?, ?)`,
		e.Type, e.Path, e.Note, e.CreatedAt,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) ListEvents(limit int) ([]Event, error) {
	rows, err := s.db.Query(
		`SELECT id, type, path, note, created_at FROM events ORDER BY created_at DESC LIMIT ?`, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Event
	for rows.Next() {
		var e Event
		if err := rows.Scan(&e.ID, &e.Type, &e.Path, &e.Note, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// DeleteSnapshotByPath видаляє рядок знімка за шляхом до файлу.
func (s *Store) DeleteSnapshotByPath(path string) error {
	_, err := s.db.Exec(`DELETE FROM snapshots WHERE path = ?`, path)
	return err
}

// DeleteEventByPath видаляє рядок події за шляхом до файлу (кліпу).
func (s *Store) DeleteEventByPath(path string) error {
	_, err := s.db.Exec(`DELETE FROM events WHERE path = ?`, path)
	return err
}

// DeleteEventsWithoutFileOlderThan видаляє події без файлу (напр. motion) старші за cutoff.
func (s *Store) DeleteEventsWithoutFileOlderThan(cutoff time.Time) (int64, error) {
	res, err := s.db.Exec(`DELETE FROM events WHERE path = '' AND created_at < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (s *Store) Close() error { return s.db.Close() }
