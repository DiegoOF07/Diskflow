package store

import (
	"errors"
	"testing"

	"github.com/DiegoOF07/Diskflow/internal/engine/model"
)

func TestAddRootIsIdempotentByKey(t *testing.T) {
	s, _ := openTemp(t)
	id, err := s.AddRoot(ctx, `c:\work`, `C:\Work`, "Trabajo")
	if err != nil {
		t.Fatal(err)
	}
	again, err := s.AddRoot(ctx, `c:\work`, `c:\WORK`, "Otro")
	if err != nil || again != id {
		t.Fatalf("second AddRoot = %d, %v; want the same id %d", again, err, id)
	}
	r, err := s.GetRoot(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if r.Path != `C:\Work` || r.Key != `c:\work` || r.Name != "Trabajo" || r.CreatedAt == 0 {
		t.Fatalf("GetRoot = %+v, want the first path and name kept", r)
	}
}

func TestListRootsInCreationOrder(t *testing.T) {
	s, _ := openTemp(t)
	s.AddRoot(ctx, "/b", "/b", "")
	s.AddRoot(ctx, "/a", "/a", "")
	roots, err := s.ListRoots(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 2 || roots[0].Path != "/b" || roots[1].Path != "/a" {
		t.Fatalf("ListRoots = %+v, want [/b /a]", roots)
	}
}

func TestRenameRoot(t *testing.T) {
	s, _ := openTemp(t)
	id, _ := s.AddRoot(ctx, "/r", "/r", "")
	if err := s.RenameRoot(ctx, id, "Proyectos"); err != nil {
		t.Fatal(err)
	}
	if r, _ := s.GetRoot(ctx, id); r.Name != "Proyectos" {
		t.Fatalf("Name = %q, want Proyectos", r.Name)
	}
	if err := s.RenameRoot(ctx, 99, "x"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("RenameRoot(unknown) err = %v, want ErrNotFound", err)
	}
}

// RemoveRoot only marks the scans: deleting millions of entries would keep the
// user waiting. Prune deletes them later, scan_runs rows included.
func TestRemoveRootLeavesItsScansToPrune(t *testing.T) {
	s, _ := openTemp(t)
	gone := completeScan(t, s, "/r")
	kept := completeScan(t, s, "/other")
	id, _ := s.AddRoot(ctx, "/r", "/r", "")
	s.AddRoot(ctx, "/other", "/other", "")

	if err := s.RemoveRoot(ctx, id); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetRoot(ctx, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetRoot after remove: err = %v, want ErrNotFound", err)
	}
	if _, ok, _ := s.ActiveScan(ctx, "/r"); ok {
		t.Fatal("a removed root still has an active scan")
	}
	if got, _ := s.LatestScan(ctx, "/r", false); got != nil {
		t.Fatalf("LatestScan of a removed root = %+v, want nil", got)
	}
	if n := countEntries(t, s, gone); n != 3 {
		t.Fatalf("RemoveRoot deleted entries itself: %d left, want 3 until Prune", n)
	}

	if _, err := s.Prune(ctx); err != nil {
		t.Fatal(err)
	}
	if n := countEntries(t, s, gone); n != 0 {
		t.Fatalf("Prune kept %d entries of the removed root", n)
	}
	var runs int
	s.db.QueryRow("SELECT COUNT(*) FROM scan_runs WHERE root = '/r'").Scan(&runs)
	if runs != 0 {
		t.Fatalf("Prune kept %d scan_runs rows of the removed root", runs)
	}
	if n := countEntries(t, s, kept); n != 3 {
		t.Fatalf("other root has %d entries, want 3", n)
	}
	if err := s.RemoveRoot(ctx, id); !errors.Is(err, ErrNotFound) {
		t.Fatalf("second RemoveRoot err = %v, want ErrNotFound", err)
	}
}

func TestReaddedRootStartsWithoutItsRemovedScans(t *testing.T) {
	s, _ := openTemp(t)
	completeScan(t, s, "/r")
	id, _ := s.AddRoot(ctx, "/r", "/r", "")
	if err := s.RemoveRoot(ctx, id); err != nil {
		t.Fatal(err)
	}
	s.AddRoot(ctx, "/r", "/r", "") // before Prune ran
	if _, ok, _ := s.ActiveScan(ctx, "/r"); ok {
		t.Fatal("the removed index came back")
	}
	fresh := completeScan(t, s, "/r")
	if active, ok, _ := s.ActiveScan(ctx, "/r"); !ok || active != fresh {
		t.Fatalf("ActiveScan = %d, %v; want the new scan %d", active, ok, fresh)
	}
	s.Prune(ctx)
	if n := countEntries(t, s, fresh); n != 3 {
		t.Fatalf("Prune deleted the new scan: %d entries left", n)
	}
}

func TestLatestScan(t *testing.T) {
	s, _ := openTemp(t)
	if got, err := s.LatestScan(ctx, "/r", false); got != nil || err != nil {
		t.Fatalf("LatestScan of a never scanned root = %+v, %v; want nil, nil", got, err)
	}
	completed := completeScan(t, s, "/r")
	failed, _ := s.BeginScan(ctx, "/r")
	s.FinishScan(ctx, failed, model.StatusFailed, model.Progress{Files: 7})

	last, err := s.LatestScan(ctx, "/r", false)
	if err != nil || last.ID != failed || last.Status != model.StatusFailed || last.Files != 7 {
		t.Fatalf("LatestScan(any) = %+v, %v; want the failed scan %d", last, err, failed)
	}
	index, err := s.LatestScan(ctx, "/r", true)
	if err != nil || index.ID != completed || index.DiskBytes != 12288 || index.FinishedAt == 0 {
		t.Fatalf("LatestScan(completed) = %+v, %v; want scan %d with its totals", index, err, completed)
	}
}

func TestMigrationRegistersScannedRoots(t *testing.T) {
	path := openAtVersion(t, 2,
		// A completed scan: its root entry keeps the path's original case.
		"INSERT INTO scan_runs (id, root, status, started_at) VALUES (1, 'c:\\work', 'completed', 50)",
		"INSERT INTO scan_runs (id, root, status, started_at) VALUES (2, 'c:\\work', 'completed', 90)",
		"INSERT INTO entries (scan_id, id, parent_id, name, kind, size, files, mtime) VALUES (2, 1, 0, 'C:\\Work', 1, 0, 0, 0)",
		// Only a failed scan: no entries left, so the key is all there is.
		"INSERT INTO scan_runs (id, root, status, started_at) VALUES (3, '/gone', 'failed', 70)",
	)
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	roots, err := s.ListRoots(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 2 {
		t.Fatalf("ListRoots = %+v, want 2 roots", roots)
	}
	if r := roots[0]; r.Path != `C:\Work` || r.Key != `c:\work` || r.CreatedAt != 50 {
		t.Errorf("roots[0] = %+v, want path C:\\Work from its index, created at its first scan", r)
	}
	if r := roots[1]; r.Path != "/gone" || r.CreatedAt != 70 {
		t.Errorf("roots[1] = %+v, want path /gone", r)
	}
}
