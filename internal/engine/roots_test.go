package engine

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DiegoOF07/Diskflow/internal/engine/model"
)

func TestAddRootValidatesTheFolder(t *testing.T) {
	e := newEngine(t)
	file := filepath.Join(t.TempDir(), "file.txt")
	os.WriteFile(file, []byte("x"), 0o644)
	if _, err := e.AddRoot(file, ""); !errors.Is(err, ErrNotDir) {
		t.Errorf("AddRoot(file) err = %v, want ErrNotDir", err)
	}
	if _, err := e.AddRoot(filepath.Join(t.TempDir(), "missing"), ""); !errors.Is(err, ErrRootUnavailable) {
		t.Errorf("AddRoot(missing) err = %v, want ErrRootUnavailable", err)
	}
}

func TestAddRootIsIdempotent(t *testing.T) {
	e := newEngine(t)
	dir := filepath.Join(t.TempDir(), "Work")
	os.Mkdir(dir, 0o755)
	first, err := e.AddRoot(dir, "Trabajo")
	if err != nil {
		t.Fatal(err)
	}
	again := dir + string(filepath.Separator) // same folder, written differently
	if caseInsensitiveFS {
		again = strings.ToUpper(dir)
	}
	second, err := e.AddRoot(again, "Otro nombre")
	if err != nil {
		t.Fatal(err)
	}
	if second.ID != first.ID || second.Name != "Trabajo" || second.Path != dir {
		t.Fatalf("second AddRoot = %+v, want the first one %+v unchanged", second, first)
	}
}

func TestRootNameDefaultsToTheFolderName(t *testing.T) {
	e := newEngine(t)
	dir := filepath.Join(t.TempDir(), "proyectos")
	os.Mkdir(dir, 0o755)
	r, _ := e.AddRoot(dir, "")
	if r.Name != "proyectos" {
		t.Fatalf("Name = %q, want the folder name", r.Name)
	}
	if err := e.RenameRoot(r.ID, "  Código  "); err != nil {
		t.Fatal(err)
	}
	if got, _ := e.GetRoot(r.ID); got.Name != "Código" {
		t.Fatalf("Name after rename = %q, want Código (trimmed)", got.Name)
	}
	e.RenameRoot(r.ID, " ")
	if got, _ := e.GetRoot(r.ID); got.Name != "proyectos" {
		t.Fatalf("Name after blank rename = %q, want the folder name back", got.Name)
	}
}

func TestListRootsSummarizesScans(t *testing.T) {
	e := newEngine(t)
	scanned, st := generate(t, 500)
	never := t.TempDir()
	e.AddRoot(scanned, "")
	e.AddRoot(never, "")
	scanToEnd(t, e, scanned)

	roots, err := e.ListRoots()
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 2 {
		t.Fatalf("ListRoots = %+v, want 2", roots)
	}
	r := roots[0]
	if r.Index == nil || r.Last == nil || r.Index.ID != r.Last.ID ||
		r.Index.Status != model.StatusCompleted || r.Index.Bytes != st.Bytes || r.Index.Files != int64(st.Files) {
		t.Fatalf("scanned root = %+v / index %+v, want its completed scan", r, r.Index)
	}
	if roots[1].Index != nil || roots[1].Last != nil {
		t.Fatalf("never scanned root = %+v, want no scans", roots[1])
	}
}

func TestRemoveRootKeepsTheFolder(t *testing.T) {
	e := newEngine(t)
	dir, _ := generate(t, 200)
	r, _ := e.AddRoot(dir, "")
	scanToEnd(t, e, dir)

	if err := e.RemoveRoot(r.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir)); err != nil {
		t.Fatalf("folder touched: %v", err)
	}
	if roots, _ := e.ListRoots(); len(roots) != 0 {
		t.Fatalf("ListRoots = %+v, want empty", roots)
	}
	if _, err := e.Root(dir); !errors.Is(err, ErrNoScan) {
		t.Fatalf("Root after remove: err = %v, want ErrNoScan", err)
	}
	if err := e.RemoveRoot(r.ID); err != nil {
		t.Fatalf("second RemoveRoot err = %v, want nil: removing is idempotent", err)
	}
}

func TestRemoveRootWhileItIsBeingScanned(t *testing.T) {
	e := newEngine(t)
	r, _ := e.AddRoot(bigTree, "")
	if _, err := e.StartScan(bigTree, Callbacks{}); err != nil {
		t.Fatal(err)
	}
	if err := e.RemoveRoot(r.ID); !errors.Is(err, ErrScanRunning) {
		t.Fatalf("RemoveRoot during its scan: err = %v, want ErrScanRunning", err)
	}
	e.Wait()
}

// A folder that was deleted, or whose drive is disconnected, stays in the
// catalog with its last scan; the UI offers to remove it.
func TestListRootsTellsWhichFoldersAreUnavailable(t *testing.T) {
	e := newEngine(t)
	gone := filepath.Join(t.TempDir(), "gone")
	os.Mkdir(gone, 0o755)
	here := t.TempDir()
	e.AddRoot(gone, "")
	e.AddRoot(here, "")
	os.Remove(gone)

	roots, err := e.ListRoots()
	if err != nil {
		t.Fatal(err)
	}
	if roots[0].Available || !roots[1].Available {
		t.Fatalf("available = %v, %v; want false for the deleted folder, true for the other",
			roots[0].Available, roots[1].Available)
	}
}
