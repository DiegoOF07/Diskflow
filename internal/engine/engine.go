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
	// ErrRootUnavailable means the folder is gone or its drive is disconnected.
	ErrRootUnavailable = errors.New("engine: folder not available")
	ErrScanRunning     = errors.New("engine: a scan is already running")
	ErrNoScan          = errors.New("engine: no completed scan for this root")
	ErrClosed          = errors.New("engine: closed")
	ErrNotDir          = scan.ErrNotDir // re-exported so callers only import engine
	ErrBadSort         = store.ErrBadSort
)

// ProgressInterval is the minimum time between two OnProgress calls.
const ProgressInterval = 100 * time.Millisecond

// Options tunes the engine. Zero values take the defaults.
type Options struct {
	Workers int // walker goroutines (default: CPUs, at most 8)

	pruned func(deleted int64) // test hook, called after each pruning pass
}

// Callbacks receive scan events. They run on the engine's goroutine, so they
// must return quickly and must not call StartScan.
type Callbacks struct {
	OnProgress func(model.Progress) // at most every ProgressInterval, plus once when the walk ends
	// OnSaving announces that the walk is over and the results are being
	// saved, which can take a while on big trees. From then on the scan can no
	// longer be cancelled. Only scans about to complete get it.
	OnSaving func()
	OnDone   func(model.Summary) // exactly once, last
}

type Engine struct {
	st    *store.Store
	opts  Options
	dbDir string // excluded from every scan, compared with foldCase applied

	mu      sync.Mutex
	cancel  context.CancelFunc // non-nil while a scan runs
	running string             // root key of the running scan
	saving  bool               // the running scan is past OnSaving
	done    chan struct{}      // closed when the current scan has finished
	closed  bool

	prune      chan struct{} // asks the pruner for one more pass; capacity 1
	stopPrune  context.CancelFunc
	prunerDone chan struct{}

	searchMu     sync.Mutex
	cancelSearch context.CancelFunc // of the latest search
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
	ctx, stop := context.WithCancel(context.Background())
	e := &Engine{
		st:         st,
		opts:       opts,
		dbDir:      foldCase(filepath.Dir(dbPath), caseInsensitiveFS),
		prune:      make(chan struct{}, 1),
		stopPrune:  stop,
		prunerDone: make(chan struct{}),
	}
	go e.pruneLoop(ctx)
	e.requestPrune() // leftovers of a previous process
	return e, nil
}

// Close cancels a running scan and the pruning, waits for both and closes the database.
func (e *Engine) Close() error {
	e.mu.Lock()
	e.closed = true
	e.mu.Unlock()
	e.Cancel()
	e.Wait()
	e.stopPrune()
	<-e.prunerDone
	return e.st.Close()
}

// pruneLoop deletes the entries of obsolete scans in the background, so a scan
// ends without waiting for the old index to go. Requests that arrive during a
// pass collapse into one more pass.
func (e *Engine) pruneLoop(ctx context.Context) {
	defer close(e.prunerDone)
	for {
		select {
		case <-ctx.Done():
			return
		case <-e.prune:
		}
		// On failure the entries stay and the next request retries them.
		if n, err := e.st.Prune(ctx); err == nil && e.opts.pruned != nil {
			e.opts.pruned(n)
		}
	}
}

func (e *Engine) requestPrune() {
	select {
	case e.prune <- struct{}{}:
	default: // a pass is already pending
	}
}

// StartScan validates root and scans it in the background. It returns the new
// scan's id; results arrive through cb.
func (e *Engine) StartScan(root string, cb Callbacks) (int64, error) {
	key, err := rootKey(root, caseInsensitiveFS)
	if err != nil {
		return 0, err
	}
	if err := checkDir(root); err != nil {
		return 0, err
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
	e.running = key
	e.saving = false
	e.done = make(chan struct{})
	go e.run(ctx, id, root, withDefaults(cb))
	return id, nil
}

func withDefaults(cb Callbacks) Callbacks {
	if cb.OnProgress == nil {
		cb.OnProgress = func(model.Progress) {}
	}
	if cb.OnSaving == nil {
		cb.OnSaving = func() {}
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

// Scan states reported by ScanState.
const (
	ScanIdle    = ""
	ScanRunning = "running"
	ScanSaving  = "saving" // the walk is over; the scan can no longer be cancelled
)

// ScanState tells whether root is being scanned, so a view opened during the
// scan can show it instead of offering to start another one.
func (e *Engine) ScanState(root string) (string, error) {
	key, err := rootKey(root, caseInsensitiveFS)
	if err != nil {
		return "", err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	switch {
	case e.cancel == nil || e.running != key:
		return ScanIdle, nil
	case e.saving:
		return ScanSaving, nil
	default:
		return ScanRunning, nil
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

// ListChildren returns one page of parentID's children in the active index of root.
func (e *Engine) ListChildren(root string, parentID int64, q model.ListQuery) (model.Page, error) {
	id, err := e.activeScan(root)
	if err != nil {
		return model.Page{}, err
	}
	return e.st.ListChildren(context.Background(), id, parentID, q)
}

// TopDirs returns the biggest directories in the active index of root.
func (e *Engine) TopDirs(root string, limit int) ([]model.Node, error) {
	id, err := e.activeScan(root)
	if err != nil {
		return nil, err
	}
	return e.st.TopDirs(context.Background(), id, limit)
}

// Path returns the absolute path of entry id in the active index of root.
func (e *Engine) Path(root string, id int64) (string, error) {
	scanID, err := e.activeScan(root)
	if err != nil {
		return "", err
	}
	names, err := e.st.Path(context.Background(), scanID, id)
	if err != nil {
		return "", err
	}
	return filepath.Join(names...), nil
}

// Search returns the entries of the active index of root whose name contains
// q, biggest first, each with the path of its folder. A search takes seconds on
// huge indexes, so starting a new one cancels the previous one, which then
// fails with context.Canceled instead of holding the database for nothing.
func (e *Engine) Search(root, q string, limit int) (model.SearchResult, error) {
	ctx, done := e.searchContext()
	defer done()
	scanID, err := e.activeScan(root)
	if err != nil {
		return model.SearchResult{}, err
	}
	res, err := e.st.Search(ctx, scanID, q, limit)
	if err != nil {
		return model.SearchResult{}, err
	}
	return e.withDirs(ctx, scanID, res)
}

// Unreadable returns up to limit entries of the active index of root that the
// scan could not read, each with the path of its folder.
func (e *Engine) Unreadable(root string, limit int) (model.SearchResult, error) {
	ctx := context.Background()
	scanID, err := e.activeScan(root)
	if err != nil {
		return model.SearchResult{}, err
	}
	res, err := e.st.Unreadable(ctx, scanID, limit)
	if err != nil {
		return model.SearchResult{}, err
	}
	return e.withDirs(ctx, scanID, res)
}

// withDirs fills the Dir of each hit.
func (e *Engine) withDirs(ctx context.Context, scanID int64, res model.SearchResult) (model.SearchResult, error) {
	dirs := map[int64]string{} // hits often share a folder
	for i, h := range res.Hits {
		dir, ok := dirs[h.ParentID]
		if !ok {
			names, err := e.st.Path(ctx, scanID, h.ParentID)
			if err != nil {
				return model.SearchResult{}, err
			}
			dir = filepath.Join(names...)
			dirs[h.ParentID] = dir
		}
		res.Hits[i].Dir = dir
	}
	return res, nil
}

// searchContext cancels the previous search and returns the context of a new one.
func (e *Engine) searchContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	e.searchMu.Lock()
	if e.cancelSearch != nil {
		e.cancelSearch()
	}
	e.cancelSearch = cancel
	e.searchMu.Unlock()
	return ctx, cancel
}

// CancelSearch stops the running search, if any, so it frees the database.
func (e *Engine) CancelSearch() {
	e.searchMu.Lock()
	defer e.searchMu.Unlock()
	if e.cancelSearch != nil {
		e.cancelSearch()
	}
}

// Trail returns the entries from the root of the active index down to id.
func (e *Engine) Trail(root string, id int64) ([]model.Node, error) {
	scanID, err := e.activeScan(root)
	if err != nil {
		return nil, err
	}
	return e.st.Trail(context.Background(), scanID, id)
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
