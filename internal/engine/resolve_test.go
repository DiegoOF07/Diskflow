package engine

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePath(t *testing.T) {
	e := newEngine(t)
	dir := t.TempDir()
	for _, f := range []string{"keep.txt", "gone.txt"} {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	scanToEnd(t, e, dir)
	root, _ := e.Root(dir)
	kids, _ := e.Children(dir, root.ID, 10)
	ids := map[string]int64{}
	for _, n := range kids {
		ids[n.Name] = n.ID
	}

	if got, err := e.ResolvePath(dir, ids["keep.txt"]); err != nil || got != filepath.Join(dir, "keep.txt") {
		t.Fatalf("ResolvePath(keep) = %q, %v", got, err)
	}
	if got, err := e.ResolvePath(dir, root.ID); err != nil || got != dir {
		t.Fatalf("ResolvePath(root) = %q, %v; want the folder itself", got, err)
	}

	if err := os.Remove(filepath.Join(dir, "gone.txt")); err != nil {
		t.Fatal(err)
	}
	if _, err := e.ResolvePath(dir, ids["gone.txt"]); !errors.Is(err, ErrNotFound) {
		t.Fatalf("entry deleted after the scan: err = %v, want ErrNotFound", err)
	}
	if _, err := e.ResolvePath(dir, 999_999); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown id: err = %v, want ErrNotFound", err)
	}
	if _, err := e.ResolvePath(t.TempDir(), ids["keep.txt"]); !errors.Is(err, ErrNoScan) {
		t.Fatalf("folder never scanned: err = %v, want ErrNoScan", err)
	}
}

func TestWithin(t *testing.T) {
	sep := string(filepath.Separator)
	dir := filepath.Join(sep+"data", "work")
	for path, want := range map[string]bool{
		dir:                                     true,
		filepath.Join(dir, "a", "b"):            true,
		filepath.Join(dir, "..a"):               true, // a name, not a parent reference
		filepath.Join(sep+"data", "work2"):      false,
		filepath.Join(sep + "data"):             false,
		filepath.Join(sep+"data", "other", "a"): false,
	} {
		if got := within(dir, path); got != want {
			t.Errorf("within(%q, %q) = %v, want %v", dir, path, got, want)
		}
	}
}
