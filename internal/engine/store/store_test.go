package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
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
	if version != 3 {
		t.Fatalf("user_version = %d, want 3", version)
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

// openAtVersion creates a database as migration version left it, runs the
// given statements on it, and returns its path.
func openAtVersion(t *testing.T, version int, stmts ...string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	names, _ := fs.Glob(migrations, "migrations/*.sql") // sorted, like migrate
	var all []string
	for _, name := range names[:version] {
		body, _ := migrations.ReadFile(name)
		all = append(all, string(body))
	}
	all = append(all, fmt.Sprintf("PRAGMA user_version = %d", version))
	for _, q := range append(all, stmts...) {
		if _, err := db.Exec(q); err != nil {
			t.Fatalf("%.40s: %v", q, err)
		}
	}
	return path
}

func TestMigrationFillsDiskSizeOfOlderScans(t *testing.T) {
	path := openAtVersion(t, 1,
		"INSERT INTO scan_runs (id, root, status, started_at, bytes) VALUES (1, '/r', 'completed', 0, 400)",
		"INSERT INTO entries (scan_id, id, parent_id, name, kind, size, files, mtime) VALUES (1, 1, 0, '/r', 1, 400, 1, 0)",
	)

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

// listScan stores a folder with five children whose names, sizes and dates
// each give a different order. "b" and "B" tie on size and date.
func listScan(t *testing.T, s *Store) int64 {
	t.Helper()
	id, _ := s.BeginScan(ctx, "/r")
	err := s.InsertEntries(ctx, id, []model.Entry{
		{ID: 1, Name: "/r", Kind: model.KindDir},
		{ID: 2, ParentID: 1, Name: "c", Kind: model.KindFile, DiskSize: 300, ModTime: 10},
		{ID: 3, ParentID: 1, Name: "a", Kind: model.KindFile, DiskSize: 100, ModTime: 30},
		{ID: 4, ParentID: 1, Name: "b", Kind: model.KindDir, DiskSize: 200, ModTime: 20},
		{ID: 5, ParentID: 1, Name: "B", Kind: model.KindFile, DiskSize: 200, ModTime: 20},
		{ID: 6, ParentID: 1, Name: "d", Kind: model.KindFile, DiskSize: 50, ModTime: 40},
		{ID: 7, ParentID: 4, Name: "inside", Kind: model.KindFile, DiskSize: 200},
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func names(nodes []model.Node) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = n.Name
	}
	return out
}

func TestListChildrenSorts(t *testing.T) {
	s, _ := openTemp(t)
	id := listScan(t, s)
	for _, tt := range []struct {
		sort string
		desc bool
		want []string
	}{
		{model.SortSize, true, []string{"c", "b", "B", "a", "d"}},
		{model.SortSize, false, []string{"d", "a", "b", "B", "c"}},
		{model.SortName, false, []string{"a", "b", "B", "c", "d"}}, // case-insensitive; ties by id
		{model.SortName, true, []string{"d", "c", "b", "B", "a"}},
		{model.SortModified, true, []string{"d", "a", "b", "B", "c"}},
		{model.SortModified, false, []string{"c", "b", "B", "a", "d"}},
	} {
		page, err := s.ListChildren(ctx, id, 1, model.ListQuery{Sort: tt.sort, Desc: tt.desc, Limit: 10})
		if err != nil {
			t.Fatal(err)
		}
		if got := names(page.Items); !slices.Equal(got, tt.want) || page.Total != 5 {
			t.Errorf("sort=%s desc=%v: %v (total %d), want %v (total 5)", tt.sort, tt.desc, got, page.Total, tt.want)
		}
	}
}

func TestListChildrenPagesWithoutGapsOrRepeats(t *testing.T) {
	s, _ := openTemp(t)
	id := listScan(t, s)
	var got []string
	for offset := 0; offset < 5; offset += 2 {
		page, err := s.ListChildren(ctx, id, 1, model.ListQuery{Sort: model.SortName, Offset: offset, Limit: 2})
		if err != nil {
			t.Fatal(err)
		}
		if page.Total != 5 {
			t.Fatalf("offset %d: total = %d, want 5", offset, page.Total)
		}
		got = append(got, names(page.Items)...)
	}
	if want := []string{"a", "b", "B", "c", "d"}; !slices.Equal(got, want) {
		t.Fatalf("pages = %v, want %v", got, want)
	}
}

func TestListChildrenRejectsUnknownSort(t *testing.T) {
	s, _ := openTemp(t)
	id := listScan(t, s)
	// The sort key picks an ORDER BY from a fixed list: it never reaches the SQL.
	if _, err := s.ListChildren(ctx, id, 1, model.ListQuery{Sort: "name; DROP TABLE entries", Limit: 10}); !errors.Is(err, ErrBadSort) {
		t.Fatalf("err = %v, want ErrBadSort", err)
	}
}

func TestListChildrenClampsOffsetAndLimit(t *testing.T) {
	s, _ := openTemp(t)
	id := listScan(t, s)
	page, err := s.ListChildren(ctx, id, 1, model.ListQuery{Sort: model.SortSize, Desc: true, Offset: -3, Limit: 0})
	if err != nil {
		t.Fatal(err)
	}
	if got := names(page.Items); !slices.Equal(got, []string{"c"}) {
		t.Fatalf("items = %v, want [c]: offset clamped to 0 and limit to 1", got)
	}
}

func TestNodesCarryModificationTime(t *testing.T) {
	s, _ := openTemp(t)
	id := listScan(t, s)
	kids, err := s.Children(ctx, id, 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if kids[0].Name != "c" || kids[0].Modified != 10 {
		t.Fatalf("first child = %+v, want c with Modified=10", kids[0])
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
			if n, err := s.Prune(ctx); err != nil || n != 1 {
				t.Fatalf("Prune = %d, %v; want the partial entry deleted", n, err)
			}
			if n := countEntries(t, s, previous); n != 3 {
				t.Fatalf("previous index has %d entries, want 3", n)
			}
		})
	}
}

func TestFinishScanLeavesOlderEntriesToPrune(t *testing.T) {
	s, _ := openTemp(t)
	older := completeScan(t, s, "/r")
	other := completeScan(t, s, "/other")
	completeScan(t, s, "/r")

	// Deleting a big index takes long, so finishing a scan never does it.
	if n := countEntries(t, s, older); n != 3 {
		t.Fatalf("FinishScan deleted the older index: %d entries left, want 3", n)
	}
	if n, err := s.Prune(ctx); err != nil || n != 3 {
		t.Fatalf("Prune = %d, %v; want the 3 older entries deleted", n, err)
	}
	if n := countEntries(t, s, other); n != 3 {
		t.Fatalf("scan of another root has %d entries, want 3", n)
	}
	if st := scanStatus(t, s, older); st != model.StatusCompleted {
		t.Fatalf("older scan_runs row status = %q, want it kept as completed", st)
	}
	if n, err := s.Prune(ctx); err != nil || n != 0 {
		t.Fatalf("second Prune = %d, %v; want nothing left to delete", n, err)
	}
}

func TestPruneKeepsRunningScan(t *testing.T) {
	s, _ := openTemp(t)
	id, _ := s.BeginScan(ctx, "/r")
	s.InsertEntries(ctx, id, []model.Entry{{ID: 1, Name: "/r", Kind: model.KindDir}})

	if n, err := s.Prune(ctx); err != nil || n != 0 {
		t.Fatalf("Prune = %d, %v; want the running scan untouched", n, err)
	}
	if n := countEntries(t, s, id); n != 1 {
		t.Fatalf("running scan has %d entries, want 1", n)
	}
}

func TestPruneDeletesInChunksAndStopsOnCancel(t *testing.T) {
	defer func(n int64) { pruneChunk = n }(pruneChunk)
	pruneChunk = 2

	s, _ := openTemp(t)
	id, _ := s.BeginScan(ctx, "/r")
	var entries []model.Entry
	for i := int64(1); i <= 5; i++ {
		entries = append(entries, model.Entry{ID: i, ParentID: i - 1, Name: fmt.Sprint(i), Kind: model.KindDir})
	}
	s.InsertEntries(ctx, id, entries)
	s.FinishScan(ctx, id, model.StatusCancelled, model.Progress{})

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := s.Prune(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("Prune with a cancelled context: err = %v, want context.Canceled", err)
	}
	if n, err := s.Prune(ctx); err != nil || n != 5 {
		t.Fatalf("Prune = %d, %v; want all 5 entries deleted in chunks of 2", n, err)
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
	if n, err := s2.Prune(ctx); err != nil || n != 1 {
		t.Fatalf("Prune = %d, %v; want the interrupted scan's entry deleted", n, err)
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
