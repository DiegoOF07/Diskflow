package desktop

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/DiegoOF07/Diskflow/internal/engine"
	"github.com/DiegoOF07/Diskflow/internal/engine/model"
)

func code(err error) string {
	return toError(err).Code
}

func TestErrorsReachTheUIWithTheirCode(t *testing.T) {
	a, _ := newTestApp(t)
	dir := tree(t)
	missing := filepath.Join(t.TempDir(), "missing")
	r, err := a.eng.AddRoot(dir, "")
	if err != nil {
		t.Fatal(err)
	}

	_, noScan := a.ListChildren(dir, 1, model.ListQuery{Sort: model.SortSize, Limit: 10})
	unavailable := a.StartScan(missing)
	_, unknownRoot := a.GetRoot(r.ID + 1)
	if err := a.StartScan(dir); err != nil {
		t.Fatal(err)
	}
	a.eng.Wait()
	gone := a.CopyPath(dir, 1_000_000)
	_, notReady := (&App{initErr: errors.New("locked")}).ListRoots()

	for _, c := range []struct {
		name string
		err  error
		want string
	}{
		{"never scanned", noScan, CodeNoScan},
		{"missing folder", unavailable, CodeRootUnavailable},
		{"folder not in the catalog", unknownRoot, CodeNotFound},
		{"entry not in the index", gone, CodeNotFound},
		{"database not open", notReady, CodeDBUnavailable},
		{"scan running", fmt.Errorf("x: %w", engine.ErrScanRunning), CodeScanRunning},
		{"not a folder", fmt.Errorf("x: %w", engine.ErrNotDir), CodeRootUnavailable},
		{"permission", fmt.Errorf("x: %w", fs.ErrPermission), CodeAccessDenied},
		{"anything else", errors.New("boom"), CodeOther},
	} {
		if c.err == nil {
			t.Errorf("%s: no error", c.name)
			continue
		}
		if got := code(c.err); got != c.want {
			t.Errorf("%s: code = %q, want %q (err %v)", c.name, got, c.want, c.err)
		}
	}
}

// The runtime turns whatever the formatter returns into new Error(value): only
// text survives, so the code travels as JSON in the message.
func TestErrorIsSentAsJSONText(t *testing.T) {
	got, ok := formatError(fmt.Errorf("engine: %w", engine.ErrNoScan)).(string)
	want := `{"code":"no_scan","detail":"engine: engine: no completed scan for this root"}`
	if !ok || got != want {
		t.Fatalf("formatError = %#v, want the string %s", got, want)
	}
}

func TestRunFormatsErrorsWithTheirCode(t *testing.T) {
	opts := appOptions(NewApp(), nil)
	if opts.ErrorFormatter == nil {
		t.Fatal("ErrorFormatter not set: the UI would get bare strings")
	}
	var e Error
	text, _ := opts.ErrorFormatter(engine.ErrScanRunning).(string)
	if err := json.Unmarshal([]byte(text), &e); err != nil || e.Code != CodeScanRunning {
		t.Fatalf("ErrorFormatter = %#v, want JSON with code scan_running", text)
	}
	if opts.SingleInstanceLock == nil {
		t.Fatal("SingleInstanceLock not set: two windows would write the same index")
	}
}
