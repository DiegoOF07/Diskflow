package engine

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
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
	if err := checkDir(abs); err != nil {
		return model.RootInfo{}, err
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

// RemoveRoot takes a folder out of the catalog at once; its scans are deleted
// from the database in the background. The folder itself is never touched.
// Removing a folder that is already gone succeeds: the user gets what they asked.
func (e *Engine) RemoveRoot(id int64) error {
	row, err := e.st.GetRoot(context.Background(), id)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	// Holding mu also keeps a new scan of this folder from starting meanwhile.
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cancel != nil && e.running == row.Key {
		return ErrScanRunning
	}
	err = e.st.RemoveRoot(context.Background(), id)
	if errors.Is(err, store.ErrNotFound) {
		return nil // removed by a concurrent call
	}
	if err != nil {
		return err
	}
	e.requestPrune()
	return nil
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
		Available: available(row.Path),
		Index:     index,
		Last:      last,
	}, nil
}

// checkDir tells why path cannot be scanned: ErrRootUnavailable if it is gone
// or its drive is disconnected, a permission error, or ErrNotDir.
func checkDir(path string) error {
	info, err := os.Stat(path)
	switch {
	case errors.Is(err, fs.ErrPermission):
		return fmt.Errorf("engine: %w", err)
	case err != nil:
		return fmt.Errorf("%w: %w", ErrRootUnavailable, err)
	case !info.IsDir():
		return fmt.Errorf("%w: %s", ErrNotDir, path)
	}
	return nil
}

// available reports whether a catalog folder is there to scan or act on.
// A folder that exists but denies access is still there.
func available(path string) bool {
	err := checkDir(path)
	return err == nil || errors.Is(err, fs.ErrPermission)
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
