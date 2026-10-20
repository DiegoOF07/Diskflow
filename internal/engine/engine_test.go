package engine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/DiegoOF07/Diskflow/internal/engine/fixture"
	"github.com/DiegoOF07/Diskflow/internal/engine/model"
)

func newEngine(t *testing.T) *Engine {
	t.Helper()
	e, err := New(filepath.Join(t.TempDir(), "data", "diskflow.db"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { e.Close() })
	return e
}

// bigTree is a 20k-entry tree shared, read-only, by the tests that need a
// scan long enough to cancel. Generating it once keeps the suite fast on NTFS.
var bigTree string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "diskflow-engine-test")
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	bigTree = filepath.Join(dir, "tree")
	if _, err := fixture.Generate(bigTree, fixture.Spec{Entries: 20_000, Seed: 1}); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

func generate(t *testing.T, entries int) (string, fixture.Stats) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "tree")
	st, err := fixture.Generate(dir, fixture.Spec{Entries: entries, Seed: 1})
	if err != nil {
		t.Fatal(err)
	}
	return dir, st
}

// scanToEnd starts a scan, waits for it and returns its summary.
func scanToEnd(t *testing.T, e *Engine, root string) model.Summary {
	t.Helper()
	var sum model.Summary
	if _, err := e.StartScan(root, Callbacks{OnDone: func(s model.Summary) { sum = s }}); err != nil {
		t.Fatal(err)
	}
	e.Wait()
	return sum
}

func TestScanTotalsMatchTheTree(t *testing.T) {
	e := newEngine(t)
	dir, st := generate(t, 5000)

	sum := scanToEnd(t, e, dir)
	if sum.Status != model.StatusCompleted {
		t.Fatalf("status = %q (%s)", sum.Status, sum.Error)
	}
	if sum.Files != int64(st.Files) || sum.Bytes != st.Bytes {
		t.Fatalf("summary = %+v, want Files=%d Bytes=%d", sum.Progress, st.Files, st.Bytes)
	}
	root, err := e.Root(dir)
	if err != nil {
		t.Fatal(err)
	}
	if root.Size != st.Bytes || root.Files != int64(st.Files) {
		t.Fatalf("Root = %+v, want Size=%d Files=%d", root, st.Bytes, st.Files)
	}
}

func TestDirectorySizesAreSubtreeTotals(t *testing.T) {
	e := newEngine(t)
	dir, _ := generate(t, 3000)
	scanToEnd(t, e, dir)

	// For every directory on the first two levels, its size equals the sum of its children.
	var check func(parent model.Node, depth int)
	check = func(parent model.Node, depth int) {
		kids, err := e.Children(dir, parent.ID, 500)
		if err != nil {
			t.Fatal(err)
		}
		var size, files int64
		for _, k := range kids {
			size += k.Size
			files += k.Files
			if k.Kind == "dir" && depth < 2 {
				check(k, depth+1)
			}
		}
		if parent.Size != size || parent.Files != files {
			t.Fatalf("%s: Size=%d Files=%d, children add up to %d and %d", parent.Name, parent.Size, parent.Files, size, files)
		}
	}
	root, err := e.Root(dir)
	if err != nil {
		t.Fatal(err)
	}
	check(root, 0)
}

func TestCancelKeepsPreviousIndex(t *testing.T) {
	e := newEngine(t)
	dir := bigTree
	scanToEnd(t, e, dir)
	before, _ := e.Root(dir)

	var sum model.Summary
	if _, err := e.StartScan(dir, Callbacks{OnDone: func(s model.Summary) { sum = s }}); err != nil {
		t.Fatal(err)
	}
	e.Cancel()
	e.Wait()

	if sum.Status != model.StatusCancelled {
		t.Fatalf("status = %q, want cancelled", sum.Status)
	}
	if after, err := e.Root(dir); err != nil || after != before {
		t.Fatalf("index changed after cancel: %+v -> %+v (%v)", before, after, err)
	}
}

func TestOnlyOneScanAtATime(t *testing.T) {
	e := newEngine(t)
	dir := bigTree
	if _, err := e.StartScan(dir, Callbacks{}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.StartScan(dir, Callbacks{}); !errors.Is(err, ErrScanRunning) {
		t.Fatalf("second StartScan err = %v, want ErrScanRunning", err)
	}
	e.Wait()
	if _, err := e.StartScan(dir, Callbacks{}); err != nil {
		t.Fatalf("StartScan after the first finished: %v", err)
	}
	e.Wait()
}

func TestWaitReturnsAfterOnDone(t *testing.T) {
	e := newEngine(t)
	dir, _ := generate(t, 500)
	var doneFinished atomic.Bool
	_, err := e.StartScan(dir, Callbacks{OnDone: func(model.Summary) {
		time.Sleep(50 * time.Millisecond)
		doneFinished.Store(true)
	}})
	if err != nil {
		t.Fatal(err)
	}
	e.Wait()
	if !doneFinished.Load() {
		t.Fatal("Wait returned before OnDone finished")
	}
}

func TestEmptyDirectory(t *testing.T) {
	e := newEngine(t)
	dir := t.TempDir()
	if s := scanToEnd(t, e, dir); s.Status != model.StatusCompleted {
		t.Fatalf("status = %q", s.Status)
	}
	root, _ := e.Root(dir)
	kids, _ := e.Children(dir, root.ID, 10)
	if root.Size != 0 || root.Files != 0 || len(kids) != 0 {
		t.Fatalf("root = %+v, children = %v", root, kids)
	}
}

func TestInvalidRoots(t *testing.T) {
	e := newEngine(t)
	file := filepath.Join(t.TempDir(), "file.txt")
	os.WriteFile(file, []byte("x"), 0o644)
	missing := filepath.Join(t.TempDir(), "missing")

	if _, err := e.StartScan(file, Callbacks{}); !errors.Is(err, ErrNotDir) {
		t.Errorf("file root: err = %v, want ErrNotDir", err)
	}
	if _, err := e.StartScan(missing, Callbacks{}); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("missing root: err = %v, want os.ErrNotExist", err)
	}
	if _, err := e.Root(file); !errors.Is(err, ErrNoScan) {
		t.Errorf("Root of never-scanned path: err = %v, want ErrNoScan", err)
	}
}

func TestProgressIsThrottled(t *testing.T) {
	e := newEngine(t)
	dir := bigTree
	var calls atomic.Int64
	start := time.Now()
	if _, err := e.StartScan(dir, Callbacks{OnProgress: func(model.Progress) { calls.Add(1) }}); err != nil {
		t.Fatal(err)
	}
	e.Wait()
	limit := int64(time.Since(start)/ProgressInterval) + 2 // ticks plus the final report
	if n := calls.Load(); n == 0 || n > limit {
		t.Fatalf("OnProgress called %d times, want 1..%d", n, limit)
	}
}

func TestReadErrorsDoNotStopTheScan(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs Unix permissions and a non-root user")
	}
	e := newEngine(t)
	dir, _ := generate(t, 300)
	locked := filepath.Join(dir, "d00")
	os.Chmod(locked, 0)
	t.Cleanup(func() { os.Chmod(locked, 0o755) })

	sum := scanToEnd(t, e, dir)
	if sum.Status != model.StatusCompleted || sum.Errors == 0 {
		t.Fatalf("summary = %+v, want completed with errors", sum)
	}
	root, _ := e.Root(dir)
	kids, _ := e.Children(dir, root.ID, 50)
	for _, k := range kids {
		if k.Name == "d00" && k.Err != model.ErrAccessDenied {
			t.Fatalf("d00 = %+v, want err access_denied", k)
		}
	}
}

func TestDatabaseDirectoryIsNotScanned(t *testing.T) {
	root := t.TempDir()
	if _, err := fixture.Generate(filepath.Join(root, "project"), fixture.Spec{Entries: 50, Seed: 1}); err != nil {
		t.Fatal(err)
	}
	e, err := New(filepath.Join(root, "appdata", "diskflow.db"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()

	scanToEnd(t, e, root)
	r, _ := e.Root(root)
	kids, _ := e.Children(root, r.ID, 10)
	for _, k := range kids {
		if k.Name == "appdata" {
			t.Fatal("the database directory was scanned")
		}
	}
	if len(kids) != 1 || kids[0].Name != "project" {
		t.Fatalf("children = %+v, want only project", kids)
	}
}

func TestCloseCancelsRunningScan(t *testing.T) {
	e, err := New(filepath.Join(t.TempDir(), "diskflow.db"), Options{})
	if err != nil {
		t.Fatal(err)
	}
	dir := bigTree
	var sum model.Summary
	e.StartScan(dir, Callbacks{OnDone: func(s model.Summary) { sum = s }})

	closed := make(chan error, 1)
	go func() { closed <- e.Close() }()
	select {
	case err := <-closed:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not return within 5s")
	}
	if sum.Status != model.StatusCancelled && sum.Status != model.StatusCompleted {
		t.Fatalf("status after Close = %q", sum.Status)
	}
}

func TestPathPointsToTheRealDirectory(t *testing.T) {
	e := newEngine(t)
	dir, _ := generate(t, 500)
	scanToEnd(t, e, dir)

	top, err := e.TopDirs(dir, 1)
	if err != nil {
		t.Fatal(err)
	}
	path, err := e.Path(dir, top[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() || filepath.Base(path) != top[0].Name {
		t.Fatalf("Path = %q (stat err %v), want an existing dir named %q", path, err, top[0].Name)
	}
}
