package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/DiegoOF07/Diskflow/internal/engine/model"
	"github.com/DiegoOF07/Diskflow/internal/engine/store"
)

var ErrRootNotFound = errors.New("engine: folder not in the catalog")

// AddRoot puts a folder in the catalog. Adding a folder that is already there,
// however its path is written, returns the existing entry unchanged.
func (e *Engine) AddRoot(path, name string) (model.RootInfo, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return model.RootInfo{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return model.RootInfo{}, fmt.Errorf("engine: %w", err)
	}
	if !info.IsDir() {
		return model.RootInfo{}, fmt.Errorf("%w: %s", ErrNotDir, abs)
	}
	id, err := e.st.AddRoot(context.Background(), foldCase(abs, caseInsensitiveFS), abs, strings.TrimSpace(name))
	if err != nil {
		return model.RootInfo{}, err
	}
	return e.GetRoot(id)
}

func (e *Engine) GetRoot(id int64) (model.RootInfo, error) {
	row, err := e.st.GetRoot(context.Background(), id)
	if err != nil {
		return model.RootInfo{}, rootErr(err)
	}
	return e.rootInfo(row)
}

// ListRoots returns the catalog in the order folders were added, each with
// its latest scans.
func (e *Engine) ListRoots() ([]model.RootInfo, error) {
	rows, err := e.st.ListRoots(context.Background())
	if err != nil {
		return nil, err
	}
	roots := make([]model.RootInfo, 0, len(rows))
	for _, row := range rows {
		r, err := e.rootInfo(row)
		if err != nil {
			return nil, err
		}
		roots = append(roots, r)
	}
	return roots, nil
}

// RenameRoot sets the name shown for a folder; a blank name restores the
// folder's own name.
func (e *Engine) RenameRoot(id int64, name string) error {
	return rootErr(e.st.RenameRoot(context.Background(), id, strings.TrimSpace(name)))
}

// RemoveRoot takes a folder out of the catalog and deletes its scans from
// the database. The folder itself is never touched.
func (e *Engine) RemoveRoot(id int64) error {
	row, err := e.st.GetRoot(context.Background(), id)
	if err != nil {
		return rootErr(err)
	}
	// Holding mu also keeps a new scan of this folder from starting meanwhile.
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cancel != nil && e.running == row.Key {
		return ErrScanRunning
	}
	return rootErr(e.st.RemoveRoot(context.Background(), id))
}

func (e *Engine) rootInfo(row store.RootRow) (model.RootInfo, error) {
	ctx := context.Background()
	index, err := e.st.LatestScan(ctx, row.Key, true)
	if err != nil {
		return model.RootInfo{}, err
	}
	last, err := e.st.LatestScan(ctx, row.Key, false)
	if err != nil {
		return model.RootInfo{}, err
	}
	return model.RootInfo{
		ID:        row.ID,
		Name:      displayName(row.Name, row.Path),
		Path:      row.Path,
		CreatedAt: row.CreatedAt,
		Index:     index,
		Last:      last,
	}, nil
}

// displayName is the user's name for a folder or, without one, the folder's
// own name. A volume root such as C:\ has none, so its path is used.
func displayName(name, path string) string {
	if name != "" {
		return name
	}
	base := filepath.Base(path)
	if base == string(filepath.Separator) || base == "." || strings.HasSuffix(base, ":") {
		return path
	}
	return base
}

func rootErr(err error) error {
	if errors.Is(err, store.ErrNotFound) {
		return ErrRootNotFound
	}
	return err
}
