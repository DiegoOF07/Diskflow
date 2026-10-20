// Package engine is the entry point to the scanning engine: it runs scans and
// answers queries over the active index. Desktop adapters and the CLI use only
// this package; it knows nothing about Wails.
package engine

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/DiegoOF07/Diskflow/internal/engine/model"
	"github.com/DiegoOF07/Diskflow/internal/engine/scan"
	"github.com/DiegoOF07/Diskflow/internal/engine/store"
)

var (
	ErrScanRunning = errors.New("engine: a scan is already running")
	ErrNoScan      = errors.New("engine: no completed scan for this root")
	ErrClosed      = errors.New("engine: closed")
	ErrNotDir      = scan.ErrNotDir // re-exported so callers only import engine
)

// ProgressInterval is the minimum time between two OnProgress calls.
const ProgressInterval = 100 * time.Millisecond

// Options tunes the engine. Zero values take the defaults.
type Options struct {
	Workers int // walker goroutines (default: CPUs, at most 8)
}

// Callbacks receive scan events. They run on the engine's goroutine, so they
// must return quickly and must not call StartScan.
type Callbacks struct {
	OnProgress func(model.Progress) // at most every ProgressInterval, plus once at the end
	OnDone     func(model.Summary)  // exactly once, after the last OnProgress
}

type Engine struct {
	st    *store.Store
	opts  Options
	dbDir string // excluded from every scan, compared with foldCase applied

	mu     sync.Mutex
	cancel context.CancelFunc // non-nil while a scan runs
	done   chan struct{}      // closed when the current scan has finished
	closed bool
}

// New opens the engine with its database at dbPath, creating the directory if needed.
func New(dbPath string, opts Options) (*Engine, error) {
	dbPath, err := filepath.Abs(dbPath)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("engine: %w", err)
	}
	st, err := store.Open(dbPath)
	if err != nil {
		return nil, err
	}
	return &Engine{
		st:    st,
		opts:  opts,
		dbDir: foldCase(filepath.Dir(dbPath), caseInsensitiveFS),
	}, nil
}

// Close cancels a running scan, waits for it and closes the database.
func (e *Engine) Close() error {
	e.mu.Lock()
	e.closed = true
	e.mu.Unlock()
	e.Cancel()
	e.Wait()
	return e.st.Close()
}

// StartScan validates root and scans it in the background. It returns the new
// scan's id; results arrive through cb.
func (e *Engine) StartScan(root string, cb Callbacks) (int64, error) {
	key, err := rootKey(root, caseInsensitiveFS)
	if err != nil {
		return 0, err
	}
	info, err := os.Stat(root)
	if err != nil {
		return 0, fmt.Errorf("engine: %w", err)
	}
	if !info.IsDir() {
		return 0, fmt.Errorf("%w: %s", ErrNotDir, root)
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	switch {
	case e.closed:
		return 0, ErrClosed
	case e.cancel != nil:
		return 0, ErrScanRunning
	}
	id, err := e.st.BeginScan(context.Background(), key)
	if err != nil {
		return 0, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	e.done = make(chan struct{})
	go e.run(ctx, id, root, withDefaults(cb))
	return id, nil
}

func withDefaults(cb Callbacks) Callbacks {
	if cb.OnProgress == nil {
		cb.OnProgress = func(model.Progress) {}
	}
	if cb.OnDone == nil {
		cb.OnDone = func(model.Summary) {}
	}
	return cb
}

// Cancel stops the running scan, if any. The previous index stays active.
func (e *Engine) Cancel() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cancel != nil {
		e.cancel()
	}
}

// Wait blocks until the running scan, if any, has finished and OnDone has returned.
func (e *Engine) Wait() {
	e.mu.Lock()
	done := e.done
	e.mu.Unlock()
	if done != nil {
		<-done
	}
}

// Root returns the root node of the active index of root.
func (e *Engine) Root(root string) (model.Node, error) {
	id, err := e.activeScan(root)
	if err != nil {
		return model.Node{}, err
	}
	return e.st.Root(context.Background(), id)
}

// Children returns up to limit children of parentID in the active index of root, biggest first.
func (e *Engine) Children(root string, parentID int64, limit int) ([]model.Node, error) {
	id, err := e.activeScan(root)
	if err != nil {
		return nil, err
	}
	return e.st.Children(context.Background(), id, parentID, limit)
}

// TopDirs returns the biggest directories in the active index of root.
func (e *Engine) TopDirs(root string, limit int) ([]model.Node, error) {
	id, err := e.activeScan(root)
	if err != nil {
		return nil, err
	}
	return e.st.TopDirs(context.Background(), id, limit)
}

func (e *Engine) activeScan(root string) (int64, error) {
	key, err := rootKey(root, caseInsensitiveFS)
	if err != nil {
		return 0, err
	}
	id, ok, err := e.st.ActiveScan(context.Background(), key)
	if err != nil {
		return 0, err
	}
	if !ok {
		return 0, ErrNoScan
	}
	return id, nil
}
