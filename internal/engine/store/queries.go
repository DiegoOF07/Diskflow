package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/DiegoOF07/Diskflow/internal/engine/model"
)

// MaxPageSize caps every list query: the UI never receives the whole index.
const MaxPageSize = 500

// ErrBadSort is returned for a sort key outside the model.Sort constants.
var ErrBadSort = errors.New("store: unknown sort")

// sortColumns maps each sort key to its column. Only these strings ever reach
// ORDER BY, which cannot take bound parameters.
var sortColumns = map[string]string{
	model.SortSize:     "disk_size",
	model.SortName:     "name COLLATE NOCASE", // NOCASE folds ASCII letters only
	model.SortModified: "mtime",
}

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

// Children returns up to limit children of parentID, biggest on disk first.
func (s *Store) Children(ctx context.Context, scanID, parentID int64, limit int) ([]model.Node, error) {
	return s.queryNodes(ctx, "children",
		"SELECT "+nodeColumns+" FROM entries WHERE scan_id = ? AND parent_id = ? ORDER BY disk_size DESC LIMIT ?",
		scanID, parentID, clampLimit(limit))
}

// ListChildren returns one page of parentID's children in the order q asks
// for. Ties are broken by id, so consecutive pages neither skip nor repeat entries.
func (s *Store) ListChildren(ctx context.Context, scanID, parentID int64, q model.ListQuery) (model.Page, error) {
	col, ok := sortColumns[q.Sort]
	if !ok {
		return model.Page{}, fmt.Errorf("%w: %q", ErrBadSort, q.Sort)
	}
	dir := "ASC"
	if q.Desc {
		dir = "DESC"
	}
	items, err := s.queryNodes(ctx, "list children",
		"SELECT "+nodeColumns+" FROM entries WHERE scan_id = ? AND parent_id = ? ORDER BY "+col+" "+dir+", id LIMIT ? OFFSET ?",
		scanID, parentID, clampLimit(q.Limit), max(q.Offset, 0))
	if err != nil {
		return model.Page{}, err
	}
	page := model.Page{Items: items}
	if err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM entries WHERE scan_id = ? AND parent_id = ?", scanID, parentID).Scan(&page.Total); err != nil {
		return model.Page{}, fmt.Errorf("store: list children: %w", err)
	}
	return page, nil
}

// TopDirs returns the biggest directories on disk of a scan, excluding the root.
func (s *Store) TopDirs(ctx context.Context, scanID int64, limit int) ([]model.Node, error) {
	return s.queryNodes(ctx, "top dirs",
		"SELECT "+nodeColumns+" FROM entries WHERE scan_id = ? AND kind = ? AND id <> 1 ORDER BY disk_size DESC LIMIT ?",
		scanID, model.KindDir, clampLimit(limit))
}

// Search returns up to limit entries whose name contains q, biggest on disk
// first, without their Dir. The root itself is never a hit.
func (s *Store) Search(ctx context.Context, scanID int64, q string, limit int) (model.SearchResult, error) {
	res := model.SearchResult{Hits: []model.Hit{}}
	if q == "" {
		return res, nil
	}
	limit = clampLimit(limit)
	// One extra row tells whether there are more hits than the limit.
	rows, err := s.db.QueryContext(ctx,
		"SELECT "+nodeColumns+", parent_id FROM entries WHERE scan_id = ? AND id <> 1 AND name LIKE ? ESCAPE '\\' ORDER BY disk_size DESC, id LIMIT ?",
		scanID, "%"+likeEscaper.Replace(q)+"%", limit+1)
	if err != nil {
		return model.SearchResult{}, fmt.Errorf("store: search: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var h model.Hit
		if h.Node, err = scanNode(rows, &h.ParentID); err != nil {
			return model.SearchResult{}, fmt.Errorf("store: search: %w", err)
		}
		res.Hits = append(res.Hits, h)
	}
	if err := rows.Err(); err != nil {
		return model.SearchResult{}, fmt.Errorf("store: search: %w", err)
	}
	if len(res.Hits) > limit {
		res.Hits, res.More = res.Hits[:limit], true
	}
	return res, nil
}

// likeEscaper makes %, _ and the escape character itself match literally.
// LIKE already ignores case, for ASCII letters only.
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// Trail returns the entries from the root down to id.
func (s *Store) Trail(ctx context.Context, scanID, id int64) ([]model.Node, error) {
	nodes, err := s.queryNodes(ctx, "trail", `
		WITH RECURSIVE up(id, depth) AS (
			SELECT id, 0 FROM entries WHERE scan_id = ?1 AND id = ?2
			UNION ALL
			SELECT e.parent_id, up.depth + 1
			FROM entries e JOIN up ON e.scan_id = ?1 AND e.id = up.id
			WHERE e.parent_id <> 0
		)
		SELECT `+nodeColumns+` FROM up JOIN entries USING (id)
		WHERE entries.scan_id = ?1 ORDER BY depth DESC`, scanID, id)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, ErrNotFound
	}
	return nodes, nil
}

// Path returns the names from the root down to entry id. Joining them is left
// to the caller, which knows the OS path rules.
func (s *Store) Path(ctx context.Context, scanID, id int64) ([]string, error) {
	trail, err := s.Trail(ctx, scanID, id)
	if err != nil {
		return nil, err
	}
	names := make([]string, len(trail))
	for i, n := range trail {
		names[i] = n.Name
	}
	return names, nil
}

const nodeColumns = "id, name, kind, size, disk_size, flags, files, mtime, err"

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanNode reads nodeColumns, then any extra columns into extra.
func scanNode(r rowScanner, extra ...any) (model.Node, error) {
	var n model.Node
	var kind model.Kind
	dest := append([]any{&n.ID, &n.Name, &kind, &n.Size, &n.DiskSize, &n.Flags, &n.Files, &n.Modified, &n.Err}, extra...)
	if err := r.Scan(dest...); err != nil {
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
