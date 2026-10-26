// Package store persists scans and their entries in a local SQLite database.
// It is the only package that writes SQL.
package store

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"time"

	"github.com/DiegoOF07/Diskflow/internal/engine/model"

	_ "modernc.org/sqlite" // registers the "sqlite" driver
)

//go:embed migrations/*.sql
var migrations embed.FS

// ErrNotFound is returned when a requested scan or entry does not exist.
var ErrNotFound = errors.New("store: not found")

type Store struct {
	db *sql.DB
}

// Open opens (or creates) the database at path, applies pending migrations and
// recovers scans interrupted by a previous process.
func Open(path string) (*Store, error) {
	dsn := "file:" + path +
		"?_pragma=journal_mode(WAL)" + // readers don't block the writer
		"&_pragma=busy_timeout(5000)" +
		"&_pragma=foreign_keys(1)" +
		// With WAL this only risks losing the last commits on power loss, never
		// corruption; the index can be rebuilt by scanning again.
		"&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	// SQLite allows a single writer; one connection avoids SQLITE_BUSY between our own goroutines.
	db.SetMaxOpenConns(1)

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: migrate: %w", err)
	}
	if err := s.recoverInterrupted(); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: recover: %w", err)
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

// migrate applies, in order, every migrations/NNN_*.sql whose number is
// greater than PRAGMA user_version, each one in its own transaction.
func (s *Store) migrate() error {
	var current int
	if err := s.db.QueryRow("PRAGMA user_version").Scan(&current); err != nil {
		return err
	}
	names, err := fs.Glob(migrations, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)
	for i, name := range names {
		version := i + 1
		if version <= current {
			continue
		}
		body, err := migrations.ReadFile(name)
		if err != nil {
			return err
		}
		if err := s.inTx(context.Background(), func(tx *sql.Tx) error {
			if _, err := tx.Exec(string(body)); err != nil {
				return fmt.Errorf("%s: %w", name, err)
			}
			// PRAGMA does not accept bound parameters.
			_, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", version))
			return err
		}); err != nil {
			return err
		}
	}
	return nil
}

// recoverInterrupted marks scans left "running" by a killed process as failed.
// Their partial entries are left for Prune.
func (s *Store) recoverInterrupted() error {
	_, err := s.db.Exec("UPDATE scan_runs SET status = ?, finished_at = ? WHERE status = ?",
		model.StatusFailed, time.Now().Unix(), model.StatusRunning)
	return err
}

// inTx runs fn in a transaction, committing on success and rolling back on error.
func (s *Store) inTx(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() // no-op after a successful Commit
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
