package fixture

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// snapshot walks dir and returns "relative/path size" lines plus counted Stats.
func snapshot(t *testing.T, dir string) ([]string, Stats) {
	t.Helper()
	var lines []string
	var st Stats
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == dir {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		if d.IsDir() {
			st.Dirs++
			lines = append(lines, rel+"/")
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		st.Files++
		st.Bytes += info.Size()
		lines = append(lines, fmt.Sprintf("%s %d", rel, info.Size()))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return lines, st
}

func TestGenerateCreatesExactlyEntries(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tree")
	got, err := Generate(dir, Spec{Entries: 1000, Seed: 1})
	if err != nil {
		t.Fatal(err)
	}
	if got.Files+got.Dirs != 1000 {
		t.Fatalf("Files+Dirs = %d, want 1000", got.Files+got.Dirs)
	}
	if _, onDisk := snapshot(t, dir); onDisk != got {
		t.Fatalf("on disk %+v, Stats %+v", onDisk, got)
	}
}

func TestGenerateIsDeterministic(t *testing.T) {
	a, b := filepath.Join(t.TempDir(), "a"), filepath.Join(t.TempDir(), "b")
	if _, err := Generate(a, Spec{Entries: 300, Seed: 7}); err != nil {
		t.Fatal(err)
	}
	if _, err := Generate(b, Spec{Entries: 300, Seed: 7}); err != nil {
		t.Fatal(err)
	}
	treeA, _ := snapshot(t, a)
	treeB, _ := snapshot(t, b)
	if !slices.Equal(treeA, treeB) {
		t.Fatal("same seed produced different trees")
	}
}

func TestGenerateDifferentSeedsDiffer(t *testing.T) {
	a, _ := Generate(filepath.Join(t.TempDir(), "a"), Spec{Entries: 300, Seed: 1})
	b, _ := Generate(filepath.Join(t.TempDir(), "b"), Spec{Entries: 300, Seed: 2})
	if a.Bytes == b.Bytes {
		t.Fatal("different seeds produced the same total size")
	}
}

// 27 = 5 at the root + 2×5 at depth 1 + 4×3 files at depth 2: the shape is exactly full.
func TestGenerateRespectsShape(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "tree")
	if _, err := Generate(dir, Spec{Entries: 27, FanOut: 2, FilesPerDir: 3, MaxDepth: 2, FileSize: 10, Seed: 1}); err != nil {
		t.Fatal(err)
	}
	tree, _ := snapshot(t, dir)
	// Dirs live at depth 1..MaxDepth, so no path has more than MaxDepth separators.
	for _, line := range tree {
		path := strings.TrimSuffix(strings.Fields(line)[0], "/")
		if depth := strings.Count(filepath.ToSlash(path), "/"); depth > 2 {
			t.Fatalf("%s is deeper than MaxDepth", line)
		}
	}
	entries, _ := os.ReadDir(dir)
	var files, dirs int
	for _, e := range entries {
		if e.IsDir() {
			dirs++
		} else {
			files++
		}
	}
	if files != 3 || dirs != 2 {
		t.Fatalf("root has %d files and %d dirs, want 3 and 2", files, dirs)
	}
}

func TestGenerateFailsWhenShapeTooSmall(t *testing.T) {
	// MaxDepth 1, FanOut 1, 1 file per dir fits at most 3 entries.
	_, err := Generate(filepath.Join(t.TempDir(), "tree"), Spec{Entries: 10, FanOut: 1, FilesPerDir: 1, MaxDepth: 1})
	if err == nil {
		t.Fatal("expected an error when the shape cannot hold Entries")
	}
}

func TestGenerateRefusesNonEmptyDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Generate(dir, Spec{Entries: 10}); err == nil {
		t.Fatal("expected an error for a non-empty directory")
	}
	if _, err := os.Stat(filepath.Join(dir, "keep.txt")); err != nil {
		t.Fatal("existing file was touched")
	}
}
