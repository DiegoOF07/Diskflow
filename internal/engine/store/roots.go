package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/DiegoOF07/Diskflow/internal/engine/model"
)

// RootRow is one folder of the catalog.
type RootRow struct {
	ID        int64
	Name      string // "" if the user never named it
	Path      string
	Key       string // what scan_runs.root holds for this folder
	CreatedAt int64
}

// AddRoot registers a folder and returns its id. Adding a key that is already
// registered returns the existing id and keeps its path and name.
func (s *Store) AddRoot(ctx context.Context, key, path, name string) (int64, error) {
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO roots (name, path, root_key, created_at) VALUES (?, ?, ?, ?) ON CONFLICT (root_key) DO NOTHING",
		name, path, key, time.Now().Unix())
	if err != nil {
		return 0, fmt.Errorf("store: add root: %w", err)
	}
	var id int64
	if err := s.db.QueryRowContext(ctx, "SELECT id FROM roots WHERE root_key = ?", key).Scan(&id); err != nil {
		return 0, fmt.Errorf("store: add root: %w", err)
	}
	return id, nil
}

const rootColumns = "id, name, path, root_key, created_at"

func scanRoot(r rowScanner) (RootRow, error) {
	var row RootRow
	err := r.Scan(&row.ID, &row.Name, &row.Path, &row.Key, &row.CreatedAt)
	return row, err
}

func (s *Store) GetRoot(ctx context.Context, id int64) (RootRow, error) {
	row, err := scanRoot(s.rd.QueryRowContext(ctx, "SELECT "+rootColumns+" FROM roots WHERE id = ?", id))
	if errors.Is(err, sql.ErrNoRows) {
		return RootRow{}, ErrNotFound
	}
	if err != nil {
		return RootRow{}, fmt.Errorf("store: get root: %w", err)
	}
	return row, nil
}

// ListRoots returns the catalog in the order folders were added.
func (s *Store) ListRoots(ctx context.Context) ([]RootRow, error) {
	rows, err := s.rd.QueryContext(ctx, "SELECT "+rootColumns+" FROM roots ORDER BY created_at, id")
	if err != nil {
		return nil, fmt.Errorf("store: list roots: %w", err)
	}
	defer rows.Close()
	var roots []RootRow
	for rows.Next() {
		row, err := scanRoot(rows)
		if err != nil {
			return nil, fmt.Errorf("store: list roots: %w", err)
		}
		roots = append(roots, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list roots: %w", err)
	}
	return roots, nil
}

func (s *Store) RenameRoot(ctx context.Context, id int64, name string) error {
	res, err := s.db.ExecContext(ctx, "UPDATE roots SET name = ? WHERE id = ?", name, id)
	if err != nil {
		return fmt.Errorf("store: rename root: %w", err)
	}
	return mustAffect(res)
}

// RemoveRoot drops a folder from the catalog and hides its scans at once;
// Prune deletes them later. It only touches the database, never the folder.
func (s *Store) RemoveRoot(ctx context.Context, id int64) error {
	return s.inTx(ctx, func(tx *sql.Tx) error {
		var key string
		err := tx.QueryRowContext(ctx, "SELECT root_key FROM roots WHERE id = ?", id).Scan(&key)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("store: remove root: %w", err)
		}
		for _, q := range []string{
			"UPDATE scan_runs SET removed = 1 WHERE root = ?",
			"DELETE FROM roots WHERE root_key = ?",
		} {
			if _, err := tx.ExecContext(ctx, q, key); err != nil {
				return fmt.Errorf("store: remove root: %w", err)
			}
		}
		return nil
	})
}

// LatestScan returns the newest scan of the folder with that key, only among
// completed ones if completedOnly is set, or nil if there is none.
func (s *Store) LatestScan(ctx context.Context, key string, completedOnly bool) (*model.ScanInfo, error) {
	query := `SELECT id, status, started_at, COALESCE(finished_at, 0), files, dirs, bytes, disk_bytes, errors
		FROM scan_runs WHERE root = ? AND removed = 0`
	args := []any{key}
	if completedOnly {
		query += " AND status = ?"
		args = append(args, model.StatusCompleted)
	}
	var si model.ScanInfo
	err := s.rd.QueryRowContext(ctx, query+" ORDER BY id DESC LIMIT 1", args...).Scan(
		&si.ID, &si.Status, &si.StartedAt, &si.FinishedAt,
		&si.Files, &si.Dirs, &si.Bytes, &si.DiskBytes, &si.Errors)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("store: latest scan: %w", err)
	}
	return &si, nil
}

func mustAffect(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
