package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DiegoOF07/Diskflow/internal/engine"
)

// cli runs one command and returns its stdout.
func cli(t *testing.T, ctx context.Context, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := run(ctx, args, &out, io.Discard)
	return out.String(), err
}

// fields parses a "key=value key=value" line.
func fields(line string) map[string]string {
	m := map[string]string{}
	for _, f := range strings.Fields(line) {
		if k, v, ok := strings.Cut(f, "="); ok {
			m[k] = v
		}
	}
	return m
}

func TestUsageErrors(t *testing.T) {
	for _, args := range [][]string{
		{},
		{"nope"},
		{"gen"},                        // --out is required
		{"scan"},                       // ROOT is required
		{"scan", "--bogus", "x"},       // unknown flag
		{"top", "--limit", "abc", "x"}, // bad flag value
	} {
		if _, err := cli(t, context.Background(), args...); !errors.Is(err, errUsage) {
			t.Errorf("%q: err = %v, want errUsage", args, err)
		}
	}
}

func TestHelpIsNotAnError(t *testing.T) {
	if _, err := cli(t, context.Background(), "scan", "-h"); err != nil {
		t.Fatalf("scan -h: %v", err)
	}
}

func TestGenScanTop(t *testing.T) {
	ctx := context.Background()
	tree := filepath.Join(t.TempDir(), "tree")
	db := filepath.Join(t.TempDir(), "dev.db")

	out, err := cli(t, ctx, "gen", "--out", tree, "--entries", "2000")
	if err != nil {
		t.Fatal(err)
	}
	gen := fields(out)

	out, err = cli(t, ctx, "scan", "--db", db, tree)
	if err != nil {
		t.Fatal(err)
	}
	scan := fields(out)
	if scan["status"] != "completed" || scan["files"] != gen["files"] || scan["bytes"] != gen["bytes"] {
		t.Fatalf("scan = %q, gen = %q", out, gen)
	}

	out, err = cli(t, ctx, "top", "--db", db, "--limit", "3", tree)
	if err != nil {
		t.Fatal(err)
	}
	// The listed paths are the engine's three biggest directories, in order.
	e, err := engine.New(db, engine.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	top, _ := e.TopDirs(tree, 3)
	lines := strings.Split(strings.TrimSpace(out), "\n")
	rows := lines[len(lines)-3:]
	for i, d := range top {
		want, _ := e.Path(tree, d.ID)
		if !strings.HasSuffix(strings.TrimSpace(rows[i]), want) {
			t.Errorf("row %d = %q, want path %q", i, rows[i], want)
		}
	}
}

func TestScanInterruptedIsCancelled(t *testing.T) {
	tree := filepath.Join(t.TempDir(), "tree")
	if _, err := cli(t, context.Background(), "gen", "--out", tree, "--entries", "5000"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // as if Ctrl+C arrived right away

	out, err := cli(t, ctx, "scan", "--db", filepath.Join(t.TempDir(), "dev.db"), tree)
	if err == nil {
		t.Fatal("interrupted scan returned no error")
	}
	if f := fields(out); f["status"] != "cancelled" || f["cancel_to_exit_ms"] == "" {
		t.Fatalf("output = %q, want status=cancelled with cancel_to_exit_ms", out)
	}
}

func TestTopWithoutScan(t *testing.T) {
	db := filepath.Join(t.TempDir(), "dev.db")
	if _, err := cli(t, context.Background(), "top", "--db", db, t.TempDir()); !errors.Is(err, engine.ErrNoScan) {
		t.Fatalf("err = %v, want engine.ErrNoScan", err)
	}
}

func TestHumanBytes(t *testing.T) {
	for _, tt := range []struct {
		n    int64
		want string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1.0 KiB"},
		{1536, "1.5 KiB"},
		{5 << 30, "5.0 GiB"},
		{3 << 40, "3.0 TiB"},
	} {
		if got := humanBytes(tt.n); got != tt.want {
			t.Errorf("humanBytes(%d) = %q, want %q", tt.n, got, tt.want)
		}
	}
}
