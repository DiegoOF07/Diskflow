package engine

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/DiegoOF07/Diskflow/internal/engine/store"
)

var (
	// ErrNotFound means the entry is not in the index or no longer exists on disk.
	ErrNotFound = store.ErrNotFound
	// ErrOutsideRoot means an entry's path left its folder; it would take a corrupt index.
	ErrOutsideRoot = errors.New("engine: path outside the folder")
)

// ResolvePath returns the path of entry id for an action on it (show it,
// copy it). Unlike Path, it checks that the path is still inside root and
// still exists, since the disk may have changed after the scan.
func (e *Engine) ResolvePath(root string, id int64) (string, error) {
	path, err := e.Path(root, id)
	if err != nil {
		return "", err
	}
	key, err := rootKey(root, caseInsensitiveFS)
	if err != nil {
		return "", err
	}
	if !within(key, foldCase(filepath.Clean(path), caseInsensitiveFS)) {
		return "", fmt.Errorf("%w: %s", ErrOutsideRoot, path)
	}
	// Lstat reads only metadata: it neither follows links nor downloads cloud files.
	if _, err := os.Lstat(path); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("%w: %s no longer exists", ErrNotFound, path)
		}
		return "", fmt.Errorf("engine: %w", err)
	}
	return path, nil
}

// within reports whether path is dir or inside it. Both must be clean and
// case-folded alike.
func within(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
