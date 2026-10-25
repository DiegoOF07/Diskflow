package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"slices"
	"testing"

	"github.com/DiegoOF07/Diskflow/internal/engine/model"
)

var ctx = context.Background()

func openTemp(t *testing.T) (*Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s, path
}

// completeScan stores root with two files (100 and 300 bytes; 4 and 8 KiB on
// disk) as a completed scan.
func completeScan(t *testing.T, s *Store, root string) int64 {
	t.Helper()
	id, err := s.BeginScan(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	entries := []model.Entry{
		{ID: 1, Name: root, Kind: model.KindDir},
		{ID: 2, ParentID: 1, Name: "small", Kind: model.KindFile, Size: 100, DiskSize: 4096},
		{ID: 3, ParentID: 1, Name: "big", Kind: model.KindFile, Size: 300, DiskSize: 8192},
	}
	if err := s.InsertEntries(ctx, id, entries); err != nil {
		t.Fatal(err)
	}
	if err := s.SetDirTotals(ctx, id, map[int64]DirTotal{1: {Size: 400, DiskSize: 12288, Files: 2}}); err != nil {
		t.Fatal(err)
	}
	if err := s.FinishScan(ctx, id, model.StatusCompleted, model.Progress{Files: 2, Dirs: 1, Bytes: 400, DiskBytes: 12288}); err != nil {
		t.Fatal(err)
	}
	return id
}

func countEntries(t *testing.T, s *Store, scanID int64) int {
	t.Helper()
	var n int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM entries WHERE scan_id = ?", scanID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func scanStatus(t *testing.T, s *Store, scanID int64) string {
	t.Helper()
	var status string
	if err := s.db.QueryRow("SELECT status FROM scan_runs WHERE id = ?", scanID).Scan(&status); err != nil {
		t.Fatal(err)
	}
	return status
}

func TestOpenEnablesWALAndMigratesOnce(t *testing.T) {
	s, path := openTemp(t)
	var mode string
	if err := s.db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", mode)
	}
	s.Close()

	s2, err := Open(path) // migrations must not run twice
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()
	var version int
	s2.db.QueryRow("PRAGMA user_version").Scan(&version)
	if version != 2 {
		t.Fatalf("user_version = %d, want 2", version)
	}
}

func TestOpenUsesNormalSynchronous(t *testing.T) {
	s, _ := openTemp(t)
	var level int
	if err := s.db.QueryRow("PRAGMA synchronous").Scan(&level); err != nil {
		t.Fatal(err)
	}
	if level != 1 { // 0 OFF, 1 NORMAL, 2 FULL
		t.Fatalf("synchronous = %d, want 1 (NORMAL)", level)
	}
}

func TestMigrationFillsDiskSizeOfOlderScans(t *testing.T) {
	// A database as version 1 left it: one completed scan, no disk_size column.
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	v1, _ := migrations.ReadFile("migrations/001_init.sql")
	for _, q := range []string{string(v1), "PRAGMA user_version = 1",
		"INSERT INTO scan_runs (id, root, status, started_at, bytes) VALUES (1, '/r', 'completed', 0, 400)",
		"INSERT INTO entries (scan_id, id, parent_id, name, kind, size, files, mtime) VALUES (1, 1, 0, '/r', 1, 400, 1, 0)",
	} {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%.40s: %v", q, err)
		}
	}
	db.Close()

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	root, err := s.Root(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if root.DiskSize != 400 || root.Flags != 0 {
		t.Fatalf("Root = %+v, want DiskSize copied from Size (400) and no flags", root)
	}
	var diskBytes int64
	s.db.QueryRow("SELECT disk_bytes FROM scan_runs WHERE id = 1").Scan(&diskBytes)
	if diskBytes != 400 {
		t.Fatalf("scan_runs.disk_bytes = %d, want 400", diskBytes)
	}
}

func TestCompletedScanBecomesActive(t *testing.T) {
	s, _ := openTemp(t)
	id := completeScan(t, s, `C:\work`)

	got, ok, err := s.ActiveScan(ctx, `C:\work`)
	if err != nil || !ok || got != id {
		t.Fatalf("ActiveScan = (%d, %v, %v), want (%d, true, nil)", got, ok, err, id)
	}
	root, err := s.Root(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	want := model.Node{ID: 1, Name: `C:\work`, Kind: "dir", Size: 400, DiskSize: 12288, Files: 2}
	if root != want {
		t.Fatalf("Root = %+v, want %+v", root, want)
	}
}

func TestActiveScanWithoutScans(t *testing.T) {
	s, _ := openTemp(t)
	if _, ok, err := s.ActiveScan(ctx, `C:\nothing`); ok || err != nil {
		t.Fatalf("ActiveScan = (ok=%v, err=%v), want (false, nil)", ok, err)
	}
}

func TestRootOfUnknownScan(t *testing.T) {
	s, _ := openTemp(t)
	if _, err := s.Root(ctx, 42); !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestChildrenOrderedBySizeDesc(t *testing.T) {
	s, _ := openTemp(t)
	id := completeScan(t, s, "/r")
	kids, err := s.Children(ctx, id, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(kids) != 2 || kids[0].Name != "big" || kids[1].Name != "small" {
		t.Fatalf("Children = %+v, want [big small]", kids)
	}
	if kids[0].Kind != "file" || kids[0].Files != 1 {
		t.Fatalf("file node = %+v, want Kind=file Files=1", kids[0])
	}
}

func TestChildrenOrderedByDiskSize(t *testing.T) {
	s, _ := openTemp(t)
	id, _ := s.BeginScan(ctx, "/r")
	s.InsertEntries(ctx, id, []model.Entry{
		{ID: 1, Name: "/r", Kind: model.KindDir},
		{ID: 2, ParentID: 1, Name: "disk.img", Kind: model.KindFile, Size: 512 << 30, DiskSize: 3 << 30, Flags: model.FlagSparse},
		{ID: 3, ParentID: 1, Name: "video.mp4", Kind: model.KindFile, Size: 4 << 30, DiskSize: 4 << 30},
	})
	kids, err := s.Children(ctx, id, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(kids) != 2 || kids[0].Name != "video.mp4" {
		t.Fatalf("Children = %+v, want video.mp4 first (bigger on disk)", kids)
	}
	if img := kids[1]; img.Size != 512<<30 || img.DiskSize != 3<<30 || img.Flags != model.FlagSparse {
		t.Fatalf("disk.img = %+v, want both sizes and FlagSparse", img)
	}
}

func TestChildrenLimitIsClamped(t *testing.T) {
	s, _ := openTemp(t)
	id, _ := s.BeginScan(ctx, "/r")
	entries := []model.Entry{{ID: 1, Name: "/r", Kind: model.KindDir}}
	for i := int64(2); i <= 601; i++ {
		entries = append(entries, model.Entry{ID: i, ParentID: 1, Name: "f", Kind: model.KindFile, Size: i})
	}
	if err := s.InsertEntries(ctx, id, entries); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct{ limit, want int }{{0, 1}, {10, 10}, {10_000, MaxPageSize}} {
		kids, err := s.Children(ctx, id, 1, tt.limit)
		if err != nil {
			t.Fatal(err)
		}
		if len(kids) != tt.want {
			t.Errorf("Children(limit=%d) returned %d, want %d", tt.limit, len(kids), tt.want)
		}
	}
}

func TestTopDirsExcludesRootAndFiles(t *testing.T) {
	s, _ := openTemp(t)
	id, _ := s.BeginScan(ctx, "/r")
	s.InsertEntries(ctx, id, []model.Entry{
		{ID: 1, Name: "/r", Kind: model.KindDir},
		{ID: 2, ParentID: 1, Name: "a", Kind: model.KindDir},
		{ID: 3, ParentID: 2, Name: "b", Kind: model.KindDir},
		{ID: 4, ParentID: 1, Name: "huge.iso", Kind: model.KindFile, Size: 9000},
	})
	s.SetDirTotals(ctx, id, map[int64]DirTotal{
		1: {Size: 9300, DiskSize: 9300},
		2: {Size: 1, DiskSize: 300, Flags: model.FlagCompressed},
		3: {Size: 9, DiskSize: 200},
	})
	top, err := s.TopDirs(ctx, id, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(top) != 2 || top[0].Name != "a" || top[1].Name != "b" {
		t.Fatalf("TopDirs = %+v, want [a b] (by disk size)", top)
	}
	if top[0].Flags != model.FlagCompressed {
		t.Fatalf("a.Flags = %03b, want the stored subtree flags", top[0].Flags)
	}
}

func TestNonCompletedScansKeepPreviousIndex(t *testing.T) {
	for _, status := range []string{model.StatusCancelled, model.StatusFailed} {
		t.Run(status, func(t *testing.T) {
			s, _ := openTemp(t)
			previous := completeScan(t, s, "/r")

			id, _ := s.BeginScan(ctx, "/r")
			s.InsertEntries(ctx, id, []model.Entry{{ID: 1, Name: "/r", Kind: model.KindDir}})
			if err := s.FinishScan(ctx, id, status, model.Progress{}); err != nil {
				t.Fatal(err)
			}

			if got, _, _ := s.ActiveScan(ctx, "/r"); got != previous {
				t.Fatalf("ActiveScan = %d, want previous %d", got, previous)
			}
			if n := countEntries(t, s, id); n != 0 {
				t.Fatalf("%s scan kept %d entries", status, n)
			}
			if n := countEntries(t, s, previous); n != 3 {
				t.Fatalf("previous index has %d entries, want 3", n)
			}
		})
	}
}

func TestCompletedScanPrunesOlderEntriesOfSameRoot(t *testing.T) {
	s, _ := openTemp(t)
	older := completeScan(t, s, "/r")
	other := completeScan(t, s, "/other")
	completeScan(t, s, "/r")

	if n := countEntries(t, s, older); n != 0 {
		t.Fatalf("older scan of the same root kept %d entries", n)
	}
	if n := countEntries(t, s, other); n != 3 {
		t.Fatalf("scan of another root has %d entries, want 3", n)
	}
	if st := scanStatus(t, s, older); st != model.StatusCompleted {
		t.Fatalf("older scan_runs row status = %q, want it kept as completed", st)
	}
}

func TestOpenRecoversInterruptedScan(t *testing.T) {
	s, path := openTemp(t)
	id, _ := s.BeginScan(ctx, "/r")
	s.InsertEntries(ctx, id, []model.Entry{{ID: 1, Name: "/r", Kind: model.KindDir}})
	s.Close() // simulates a killed process: FinishScan never runs

	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	if st := scanStatus(t, s2, id); st != model.StatusFailed {
		t.Fatalf("status = %q, want failed", st)
	}
	if n := countEntries(t, s2, id); n != 0 {
		t.Fatalf("interrupted scan kept %d entries", n)
	}
}

func TestSetDirTotalsKeepsErrWhenEmpty(t *testing.T) {
	s, _ := openTemp(t)
	id, _ := s.BeginScan(ctx, "/r")
	s.InsertEntries(ctx, id, []model.Entry{
		{ID: 1, Name: "/r", Kind: model.KindDir},
		{ID: 2, ParentID: 1, Name: "locked", Kind: model.KindDir, Err: model.ErrAccessDenied},
		{ID: 3, ParentID: 1, Name: "gone", Kind: model.KindDir},
	})
	err := s.SetDirTotals(ctx, id, map[int64]DirTotal{
		2: {},                       // empty Err must not erase the stored one
		3: {Err: model.ErrNotFound}, // a read error found during the walk
	})
	if err != nil {
		t.Fatal(err)
	}
	kids, _ := s.Children(ctx, id, 1, 10)
	got := map[string]string{}
	for _, k := range kids {
		got[k.Name] = k.Err
	}
	if got["locked"] != model.ErrAccessDenied || got["gone"] != model.ErrNotFound {
		t.Fatalf("errors = %v", got)
	}
}

func TestPathWalksUpToTheRoot(t *testing.T) {
	s, _ := openTemp(t)
	id, _ := s.BeginScan(ctx, "/r")
	s.InsertEntries(ctx, id, []model.Entry{
		{ID: 1, Name: "/r", Kind: model.KindDir},
		{ID: 2, ParentID: 1, Name: "a", Kind: model.KindDir},
		{ID: 3, ParentID: 2, Name: "b", Kind: model.KindDir},
		{ID: 4, ParentID: 3, Name: "c.txt", Kind: model.KindFile},
	})
	for _, tt := range []struct {
		id   int64
		want []string
	}{
		{1, []string{"/r"}},
		{4, []string{"/r", "a", "b", "c.txt"}},
	} {
		got, err := s.Path(ctx, id, tt.id)
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(got, tt.want) {
			t.Errorf("Path(%d) = %q, want %q", tt.id, got, tt.want)
		}
	}
	if _, err := s.Path(ctx, id, 99); !errors.Is(err, ErrNotFound) {
		t.Errorf("Path of unknown entry: err = %v, want ErrNotFound", err)
	}
}
