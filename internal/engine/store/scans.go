package store

import (
	"context"
	"database/sql"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/DiegoOF07/Diskflow/internal/engine/model"
)

// DirTotal holds the aggregated subtree values of a directory.
// An empty Err keeps the error already stored for that entry.
type DirTotal struct {
	Size     int64
	DiskSize int64
	Flags    model.Flags // union of the flags in the subtree
	Files    int64
	Err      string
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
			(scan_id, id, parent_id, name, kind, size, disk_size, flags, files, mtime, err)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
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
				scanID, e.ID, e.ParentID, e.Name, e.Kind, e.Size, e.DiskSize, e.Flags, files, e.ModTime, e.Err); err != nil {
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

// SetDirTotals writes the aggregated sizes, flags, file count and read error of directories.
func (s *Store) SetDirTotals(ctx context.Context, scanID int64, totals map[int64]DirTotal) error {
	err := s.inTx(ctx, func(tx *sql.Tx) error {
		stmt, err := tx.PrepareContext(ctx, `UPDATE entries
			SET size = ?, disk_size = ?, flags = ?, files = ?, err = CASE WHEN ? = '' THEN err ELSE ? END
			WHERE scan_id = ? AND id = ?`)
		if err != nil {
			return err
		}
		defer stmt.Close()
		// In key order the updates walk the B-tree sequentially; map order made
		// this step ~3.5x slower on a 500k-entry scan.
		for _, id := range slices.Sorted(maps.Keys(totals)) {
			t := totals[id]
			if _, err := stmt.ExecContext(ctx, t.Size, t.DiskSize, t.Flags, t.Files, t.Err, t.Err, scanID, id); err != nil {
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

// FinishScan closes a scan with its final status and counters. A completed
// scan becomes the active index of its root. The entries no longer needed (the
// older index, or this scan's own if it did not complete) are left for Prune:
// deleting millions of rows here would keep the user waiting for nothing.
func (s *Store) FinishScan(ctx context.Context, scanID int64, status string, p model.Progress) error {
	_, err := s.db.ExecContext(ctx, `UPDATE scan_runs
		SET status = ?, finished_at = ?, files = ?, dirs = ?, bytes = ?, disk_bytes = ?, errors = ?
		WHERE id = ?`,
		status, time.Now().Unix(), p.Files, p.Dirs, p.Bytes, p.DiskBytes, p.Errors, scanID)
	if err != nil {
		return fmt.Errorf("store: finish scan: %w", err)
	}
	return nil
}

// pruneChunk is how many entries one Prune statement deletes. Short writes let
// the UI's queries and a new scan's inserts take turns with the pruning.
var pruneChunk int64 = 10_000

// Prune deletes, in chunks, the entries nobody will read again: those of every
// finished scan except the latest completed one of each root, and those of
// removed roots. A scan never leaves that set once in it, so each chunk can run
// on its own. scan_runs rows stay as history, except a removed root's, which go
// once their entries are gone. Prune returns how many entries it deleted and
// stops between chunks when ctx is cancelled; the next call resumes.
func (s *Store) Prune(ctx context.Context) (int64, error) {
	var deleted int64
	for {
		if err := ctx.Err(); err != nil {
			return deleted, err
		}
		res, err := s.db.ExecContext(ctx, `DELETE FROM entries WHERE (scan_id, id) IN (
			SELECT scan_id, id FROM entries WHERE scan_id IN (
				SELECT id FROM scan_runs WHERE status <> ? AND (removed = 1 OR id NOT IN (
					SELECT MAX(id) FROM scan_runs WHERE status = ? AND removed = 0 GROUP BY root)))
			LIMIT ?)`,
			model.StatusRunning, model.StatusCompleted, pruneChunk)
		if err != nil {
			return deleted, fmt.Errorf("store: prune: %w", err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return deleted, fmt.Errorf("store: prune: %w", err)
		}
		deleted += n
		if n < pruneChunk {
			_, err := s.db.ExecContext(ctx, `DELETE FROM scan_runs WHERE removed = 1
				AND NOT EXISTS (SELECT 1 FROM entries WHERE scan_id = scan_runs.id)`)
			if err != nil {
				return deleted, fmt.Errorf("store: prune: %w", err)
			}
			return deleted, nil
		}
	}
}
