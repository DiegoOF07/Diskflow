package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/DiegoOF07/Diskflow/internal/engine/model"
)

// DirTotal holds the aggregated subtree values of a directory.
// An empty Err keeps the error already stored for that entry.
type DirTotal struct {
	Size  int64
	Files int64
	Err   string
}

// BeginScan registers a new running scan of root and returns its id.
func (s *Store) BeginScan(ctx context.Context, root string) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		"INSERT INTO scan_runs (root, status, started_at) VALUES (?, ?, ?)",
		root, model.StatusRunning, time.Now().Unix())
	if err != nil {
		return 0, fmt.Errorf("store: begin scan: %w", err)
	}
	return res.LastInsertId()
}

// InsertEntries stores a batch of entries in one transaction.
func (s *Store) InsertEntries(ctx context.Context, scanID int64, entries []model.Entry) error {
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		stmt, err := tx.PrepareContext(ctx, `INSERT INTO entries
			(scan_id, id, parent_id, name, kind, size, files, mtime, err)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, e := range entries {
			var files int64
			if e.Kind == model.KindFile {
				files = 1
			}
			if _, err := stmt.ExecContext(ctx,
				scanID, e.ID, e.ParentID, e.Name, e.Kind, e.Size, files, e.ModTime, e.Err); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("store: insert entries: %w", err)
	}
	return nil
}

// SetDirTotals writes the aggregated size, file count and read error of directories.
func (s *Store) SetDirTotals(ctx context.Context, scanID int64, totals map[int64]DirTotal) error {
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		stmt, err := tx.PrepareContext(ctx, `UPDATE entries
			SET size = ?, files = ?, err = CASE WHEN ? = '' THEN err ELSE ? END
			WHERE scan_id = ? AND id = ?`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for id, t := range totals {
			if _, err := stmt.ExecContext(ctx, t.Size, t.Files, t.Err, t.Err, scanID, id); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("store: set dir totals: %w", err)
	}
	return nil
}

// FinishScan closes a scan with its final status and counters.
//   - completed: becomes the active index and older entries of the same root
//     are deleted (their scan_runs rows stay as history).
//   - cancelled/failed: its partial entries are deleted, so a partial index
//     is never presented as current.
func (s *Store) FinishScan(ctx context.Context, scanID int64, status string, p model.Progress) error {
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `UPDATE scan_runs
			SET status = ?, finished_at = ?, files = ?, dirs = ?, bytes = ?, errors = ?
			WHERE id = ?`,
			status, time.Now().Unix(), p.Files, p.Dirs, p.Bytes, p.Errors, scanID); err != nil {
			return err
		}
		if status != model.StatusCompleted {
			_, err := tx.ExecContext(ctx, "DELETE FROM entries WHERE scan_id = ?", scanID)
			return err
		}
		_, err := tx.ExecContext(ctx, `DELETE FROM entries WHERE scan_id IN (
			SELECT id FROM scan_runs
			WHERE root = (SELECT root FROM scan_runs WHERE id = ?) AND id < ?)`,
			scanID, scanID)
		return err
	})
	if err != nil {
		return fmt.Errorf("store: finish scan: %w", err)
	}
	return nil
}
