package scan

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/DiegoOF07/Diskflow/internal/engine/fixture"
	"github.com/DiegoOF07/Diskflow/internal/engine/model"
)

// collect runs Walk to completion and returns the entries in arrival order.
func collect(t *testing.T, root string, opts Options) []model.Entry {
	t.Helper()
	out := make(chan model.Entry, 64)
	var got []model.Entry
	done := make(chan struct{})
	go func() {
		for e := range out {
			got = append(got, e)
		}
		close(done)
	}()
	err := Walk(context.Background(), root, opts, out)
	close(out)
	<-done
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	return got
}

func generate(t *testing.T, spec fixture.Spec) (string, fixture.Stats) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "tree")
	st, err := fixture.Generate(dir, spec)
	if err != nil {
		t.Fatal(err)
	}
	return dir, st
}

func byName(entries []model.Entry, name string) model.Entry {
	for _, e := range entries {
		if e.Name == name {
			return e
		}
	}
	return model.Entry{}
}

func patchFor(entries []model.Entry, id int64) model.Entry {
	for _, e := range entries {
		if e.ID == id && IsPatch(e) {
			return e
		}
	}
	return model.Entry{}
}

func TestWalkEmitsEveryEntryOnce(t *testing.T) {
	dir, st := generate(t, fixture.Spec{Entries: 2000, Seed: 1})
	got := collect(t, dir, Options{Workers: 4})

	if want := st.Files + st.Dirs + 1; len(got) != want { // +1: the root
		t.Fatalf("got %d entries, want %d", len(got), want)
	}
	byID := map[int64]model.Entry{}
	var bytes int64
	for _, e := range got {
		if _, dup := byID[e.ID]; dup {
			t.Fatalf("duplicate ID %d", e.ID)
		}
		byID[e.ID] = e
		bytes += e.Size
	}
	root := byID[1]
	if root.ParentID != 0 || root.Kind != model.KindDir || !filepath.IsAbs(root.Name) {
		t.Fatalf("root entry = %+v", root)
	}
	if bytes != st.Bytes {
		t.Fatalf("sum of sizes = %d, want %d", bytes, st.Bytes)
	}
}

// The scan facade aggregates sizes into ancestors as entries arrive, so it
// relies on every directory arriving before any of its children.
func TestWalkEmitsParentsBeforeChildren(t *testing.T) {
	dir, _ := generate(t, fixture.Spec{Entries: 3000, Seed: 3})
	seen := map[int64]bool{}
	for _, e := range collect(t, dir, Options{Workers: 8}) {
		if e.ID != 1 && !seen[e.ParentID] {
			t.Fatalf("entry %d (%s) arrived before its parent %d", e.ID, e.Name, e.ParentID)
		}
		seen[e.ID] = true
	}
}

func TestWalkSharedQueueIsBounded(t *testing.T) {
	// One wide level: 60 subdirectories under the root, each with files.
	dir, st := generate(t, fixture.Spec{Entries: 600, FanOut: 60, FilesPerDir: 5, MaxDepth: 2, Seed: 1})
	w := newWalker(context.Background(), Options{Workers: 2, MaxQueued: 4}, make(chan model.Entry, 1<<14))
	if err := w.run(dir); err != nil {
		t.Fatal(err)
	}
	if w.peakQueued > 4 {
		t.Fatalf("shared queue reached %d items, want <= 4", w.peakQueued)
	}
	if got := len(w.out); got != st.Files+st.Dirs+1 {
		t.Fatalf("emitted %d entries, want %d", got, st.Files+st.Dirs+1)
	}
}

func TestWalkRecordsSymlinksAsLinks(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if _, err := fixture.Generate(target, fixture.Spec{Entries: 5, FanOut: 1, Seed: 1}); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "link")); err != nil {
		t.Skipf("cannot create symlinks here: %v", err)
	}
	assertLinkNotFollowed(t, collect(t, dir, Options{}), "link")
}

func TestWalkRecordsJunctionsAsLinks(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("junctions only exist on Windows")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if _, err := fixture.Generate(target, fixture.Spec{Entries: 5, FanOut: 1, Seed: 1}); err != nil {
		t.Fatal(err)
	}
	junction := filepath.Join(dir, "junction")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", junction, target).CombinedOutput(); err != nil {
		t.Fatalf("mklink /J: %v: %s", err, out)
	}
	assertLinkNotFollowed(t, collect(t, dir, Options{}), "junction")
}

func assertLinkNotFollowed(t *testing.T, got []model.Entry, name string) {
	t.Helper()
	link := byName(got, name)
	if link.Kind != model.KindLink {
		t.Fatalf("%s kind = %v, want link", name, link.Kind)
	}
	for _, e := range got {
		if e.ParentID == link.ID {
			t.Fatalf("walker followed %s into %s", name, e.Name)
		}
	}
}

func TestWalkRecordsAccessDenied(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs Unix permissions and a non-root user")
	}
	dir := t.TempDir()
	locked := filepath.Join(dir, "locked")
	if err := os.Mkdir(locked, 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(locked, "secret"), []byte("x"), 0o644)
	os.Chmod(locked, 0)
	t.Cleanup(func() { os.Chmod(locked, 0o755) })

	got := collect(t, dir, Options{})
	if p := patchFor(got, byName(got, "locked").ID); p.Err != model.ErrAccessDenied {
		t.Fatalf("patch for locked dir = %+v, want access_denied", p)
	}
}

func TestWalkRecordsVanishedDir(t *testing.T) {
	dir := t.TempDir()
	gone := filepath.Join(dir, "gone")
	if err := os.Mkdir(gone, 0o755); err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	beforeReadDir = func(path string) {
		if path == gone {
			once.Do(func() { os.RemoveAll(gone) })
		}
	}
	t.Cleanup(func() { beforeReadDir = nil })

	got := collect(t, dir, Options{})
	if p := patchFor(got, byName(got, "gone").ID); p.Err != model.ErrNotFound {
		t.Fatalf("patch for vanished dir = %+v, want not_found", p)
	}
}

func TestWalkHandlesLongPaths(t *testing.T) {
	dir := t.TempDir()
	deep := dir
	for len(deep) < 320 { // beyond Windows' classic MAX_PATH of 260
		deep = filepath.Join(deep, strings.Repeat("a", 50))
	}
	if err := os.MkdirAll(deep, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(deep, "deep.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	e := byName(collect(t, dir, Options{}), "deep.txt")
	if e.Err != "" || e.Size != 5 {
		t.Fatalf("deep file = %+v, want Size=5 and no error", e)
	}
}

func TestWalkStopsPromptlyOnCancel(t *testing.T) {
	dir, _ := generate(t, fixture.Spec{Entries: 5000, Seed: 1})
	ctx, cancel := context.WithCancel(context.Background())
	out := make(chan model.Entry, 16)
	errc := make(chan error, 1)
	go func() { errc <- Walk(ctx, dir, Options{Workers: 4}, out) }()

	for range 100 {
		<-out
	}
	// Nobody reads from now on: workers are blocked on a full channel.
	cancel()
	select {
	case err := <-errc:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Walk did not return within 2s of cancel")
	}
}

func TestWalkRejectsNonDirRoot(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file.txt")
	os.WriteFile(file, []byte("x"), 0o644)
	for _, root := range []string{file, filepath.Join(t.TempDir(), "missing")} {
		err := Walk(context.Background(), root, Options{}, make(chan model.Entry, 1))
		if err == nil {
			t.Errorf("Walk(%s) = nil, want an error", root)
		}
	}
}

func ExampleIsPatch() {
	fmt.Println(IsPatch(model.Entry{ID: 7, Err: model.ErrAccessDenied}))
	fmt.Println(IsPatch(model.Entry{ID: 7, Name: "docs", Kind: model.KindDir}))
	// Output:
	// true
	// false
}

func TestWalkSkipsExcludedDirs(t *testing.T) {
	dir, _ := generate(t, fixture.Spec{Entries: 200, FanOut: 3, Seed: 1})
	excluded := filepath.Join(dir, "d01")
	got := collect(t, dir, Options{Skip: func(path string) bool { return path == excluded }})

	topLevel := map[string]bool{} // names repeat at deeper levels, so look only under the root
	for _, e := range got {
		if e.ParentID == 1 {
			topLevel[e.Name] = true
		}
	}
	if topLevel["d01"] {
		t.Fatal("excluded dir was emitted")
	}
	if !topLevel["d00"] || !topLevel["d02"] {
		t.Fatal("sibling dirs must still be walked")
	}
}
