package desktop

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/DiegoOF07/Diskflow/internal/engine"
	"github.com/DiegoOF07/Diskflow/internal/engine/fixture"
	"github.com/DiegoOF07/Diskflow/internal/engine/model"
)

type event struct {
	name string
	data any
}

// recorder stands in for the Wails event bus. The engine emits from its own
// goroutine, hence the mutex.
type recorder struct {
	mu     sync.Mutex
	events []event
}

func (r *recorder) emit(name string, data any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, event{name, data})
}

func newTestApp(t *testing.T) (*App, *recorder) {
	t.Helper()
	eng, err := engine.New(filepath.Join(t.TempDir(), "diskflow.db"), engine.Options{})
	if err != nil {
		t.Fatal(err)
	}
	rec := &recorder{}
	a := &App{eng: eng, emit: rec.emit}
	t.Cleanup(func() { eng.Close() })
	return a, rec
}

func tree(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "tree")
	if _, err := fixture.Generate(dir, fixture.Spec{Entries: 300, Seed: 1}); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestStartScanEmitsProgressThenDone(t *testing.T) {
	a, rec := newTestApp(t)
	if err := a.StartScan(tree(t)); err != nil {
		t.Fatal(err)
	}
	a.eng.Wait()

	n := len(rec.events)
	if n < 2 || rec.events[0].name != EventProgress || rec.events[n-1].name != EventDone {
		t.Fatalf("events = %v, want %s... then %s", rec.events, EventProgress, EventDone)
	}
	if sum, ok := rec.events[n-1].data.(model.Summary); !ok || sum.Status != model.StatusCompleted {
		t.Fatalf("done payload = %#v, want a completed model.Summary", rec.events[n-1].data)
	}
}

func TestRootIsNilBeforeTheFirstScan(t *testing.T) {
	a, _ := newTestApp(t)
	root, err := a.Root(t.TempDir())
	if err != nil || root != nil {
		t.Fatalf("Root = %v, %v; want nil, nil", root, err)
	}
}

func TestRootAndChildrenAfterScan(t *testing.T) {
	a, _ := newTestApp(t)
	dir := tree(t)
	a.StartScan(dir)
	a.eng.Wait()

	root, err := a.Root(dir)
	if err != nil || root == nil {
		t.Fatalf("Root = %v, %v", root, err)
	}
	kids, err := a.Children(dir, root.ID, 5)
	if err != nil || len(kids) != 5 {
		t.Fatalf("Children = %d nodes, %v; want 5", len(kids), err)
	}
}

func TestEngineThatFailedToOpen(t *testing.T) {
	cause := errors.New("disk full")
	a := &App{initErr: cause, emit: func(string, any) {}}

	if err := a.StartScan(t.TempDir()); !errors.Is(err, errNotReady) {
		t.Errorf("StartScan err = %v, want errNotReady", err)
	}
	if _, err := a.Root(t.TempDir()); !errors.Is(err, errNotReady) {
		t.Errorf("Root err = %v, want errNotReady", err)
	}
	if _, err := a.Children(t.TempDir(), 1, 10); !errors.Is(err, errNotReady) {
		t.Errorf("Children err = %v, want errNotReady", err)
	}
	a.CancelScan() // must not panic
	a.shutdown(context.Background())
}

func TestShutdownClosesTheEngine(t *testing.T) {
	a, _ := newTestApp(t)
	a.shutdown(context.Background())
	if err := a.StartScan(t.TempDir()); !errors.Is(err, engine.ErrClosed) {
		t.Fatalf("StartScan after shutdown: err = %v, want engine.ErrClosed", err)
	}
}
