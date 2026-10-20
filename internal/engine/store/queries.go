package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/DiegoOF07/Diskflow/internal/engine/model"
)

// MaxPageSize caps every list query: the UI never receives the whole index.
const MaxPageSize = 500

// ActiveScan returns the most recent completed scan of root.
func (s *Store) ActiveScan(ctx context.Context, root string) (id int64, ok bool, err error) {
	err = s.db.QueryRowContext(ctx,
		"SELECT id FROM scan_runs WHERE root = ? AND status = ? ORDER BY id DESC LIMIT 1",
		root, model.StatusCompleted).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("store: active scan: %w", err)
	}
	return id, true, nil
}

// Root returns the root entry (id 1) of a scan.
func (s *Store) Root(ctx context.Context, scanID int64) (model.Node, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT "+nodeColumns+" FROM entries WHERE scan_id = ? AND id = 1", scanID)
	n, err := scanNode(row)
	if errors.Is(err, sql.ErrNoRows) {
		return model.Node{}, ErrNotFound
	}
	if err != nil {
		return model.Node{}, fmt.Errorf("store: root: %w", err)
	}
	return n, nil
}

// Children returns up to limit children of parentID, biggest first.
func (s *Store) Children(ctx context.Context, scanID, parentID int64, limit int) ([]model.Node, error) {
	return s.queryNodes(ctx, "children",
		"SELECT "+nodeColumns+" FROM entries WHERE scan_id = ? AND parent_id = ? ORDER BY size DESC LIMIT ?",
		scanID, parentID, clampLimit(limit))
}

// TopDirs returns the biggest directories of a scan, excluding the root.
func (s *Store) TopDirs(ctx context.Context, scanID int64, limit int) ([]model.Node, error) {
	return s.queryNodes(ctx, "top dirs",
		"SELECT "+nodeColumns+" FROM entries WHERE scan_id = ? AND kind = ? AND id <> 1 ORDER BY size DESC LIMIT ?",
		scanID, model.KindDir, clampLimit(limit))
}

// Path returns the names from the root down to entry id. Joining them is left
// to the caller, which knows the OS path rules.
func (s *Store) Path(ctx context.Context, scanID, id int64) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		WITH RECURSIVE up(id, parent_id, name, depth) AS (
			SELECT id, parent_id, name, 0 FROM entries WHERE scan_id = ?1 AND id = ?2
			UNION ALL
			SELECT e.id, e.parent_id, e.name, up.depth + 1
			FROM entries e JOIN up ON e.scan_id = ?1 AND e.id = up.parent_id
		)
		SELECT name FROM up ORDER BY depth DESC`, scanID, id)
	if err != nil {
		return nil, fmt.Errorf("store: path: %w", err)
	}
	defer rows.Close()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("store: path: %w", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: path: %w", err)
	}
	if len(names) == 0 {
		return nil, ErrNotFound
	}
	return names, nil
}

const nodeColumns = "id, name, kind, size, files, err"

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanNode(r rowScanner) (model.Node, error) {
	var n model.Node
	var kind model.Kind
	if err := r.Scan(&n.ID, &n.Name, &kind, &n.Size, &n.Files, &n.Err); err != nil {
		return model.Node{}, err
	}
	n.Kind = kind.String()
	return n, nil
}

func (s *Store) queryNodes(ctx context.Context, what, query string, args ...any) ([]model.Node, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("store: %s: %w", what, err)
	}
	defer rows.Close()
	nodes := []model.Node{} // empty slice, not nil: serializes as [] for the UI
	for rows.Next() {
		n, err := scanNode(rows)
		if err != nil {
			return nil, fmt.Errorf("store: %s: %w", what, err)
		}
		nodes = append(nodes, n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: %s: %w", what, err)
	}
	return nodes, nil
}

func clampLimit(limit int) int {
	return min(max(limit, 1), MaxPageSize)
}
