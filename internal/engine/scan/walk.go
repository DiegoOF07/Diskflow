// Package scan enumerates a directory tree concurrently, read-only and
// without following links.
package scan

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/DiegoOF07/Diskflow/internal/engine/model"
)

// ErrNotDir is returned when the root is not a directory.
var ErrNotDir = errors.New("scan: root is not a directory")

// Options tunes a walk. Zero values take the defaults.
type Options struct {
	Workers   int // goroutines reading directories (default: CPUs, at most 8)
	MaxQueued int // capacity of the shared directory queue (default 1024)
	// Skip, if set, is called with the full path of each directory found below
	// the root; returning true omits that directory and everything inside it.
	Skip func(path string) bool
}

// beforeReadDir is a test hook called right before a directory is read.
var beforeReadDir func(path string)

// IsPatch reports whether e only carries a read error for a directory that was
// already emitted (same ID, empty Name) rather than being a new entry.
func IsPatch(e model.Entry) bool { return e.Name == "" }

// Walk emits the root (ID 1) and every entry below it on out, which it never
// closes. Each directory is emitted before any of its children. Problems with
// single entries are reported in Entry.Err and never stop the walk.
// Walk returns ctx.Err() if cancelled, or an error if root is not a readable directory.
func Walk(ctx context.Context, root string, opts Options, out chan<- model.Entry) error {
	return newWalker(ctx, opts, out).run(root)
}

type dirItem struct {
	id   int64
	path string
}

type walker struct {
	ctx    context.Context
	opts   Options
	out    chan<- model.Entry
	nextID atomic.Int64

	mu         sync.Mutex
	cond       *sync.Cond // signalled when queue gains items or pending reaches 0
	queue      []dirItem  // shared LIFO, at most opts.MaxQueued items
	pending    int        // dirs queued (shared or local) or being read
	peakQueued int        // high-water mark of queue, for tests
}

func newWalker(ctx context.Context, opts Options, out chan<- model.Entry) *walker {
	if opts.Workers <= 0 {
		opts.Workers = min(runtime.NumCPU(), 8)
	}
	if opts.MaxQueued <= 0 {
		opts.MaxQueued = 1024
	}
	w := &walker{ctx: ctx, opts: opts, out: out}
	w.cond = sync.NewCond(&w.mu)
	return w
}

func (w *walker) run(root string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	info, err := os.Lstat(root) // Lstat: a link given as root is not followed either
	if err != nil {
		return fmt.Errorf("scan: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: %s", ErrNotDir, root)
	}

	w.nextID.Store(1)
	if !w.send(model.Entry{ID: 1, Name: root, Kind: model.KindDir, ModTime: info.ModTime().Unix()}) {
		return w.ctx.Err()
	}
	w.queue = append(w.queue, dirItem{1, root})
	w.pending = 1

	// Wake workers sleeping in cond.Wait so they notice the cancellation.
	stop := context.AfterFunc(w.ctx, func() {
		w.mu.Lock()
		w.cond.Broadcast()
		w.mu.Unlock()
	})
	defer stop()

	var wg sync.WaitGroup
	for range w.opts.Workers {
		wg.Go(w.work)
	}
	wg.Wait()
	return w.ctx.Err()
}

// work processes directories until there are none left or the walk is cancelled.
// Directories that don't fit in the shared queue stay in the worker's local
// stack and are walked depth-first by that same worker.
func (w *walker) work() {
	var local []dirItem
	for {
		if w.ctx.Err() != nil {
			return
		}
		var d dirItem
		if n := len(local); n > 0 {
			d, local = local[n-1], local[:n-1]
		} else {
			w.mu.Lock()
			for len(w.queue) == 0 && w.pending > 0 && w.ctx.Err() == nil {
				w.cond.Wait()
			}
			if w.ctx.Err() != nil || w.pending == 0 {
				w.mu.Unlock()
				return
			}
			n := len(w.queue)
			d, w.queue = w.queue[n-1], w.queue[:n-1]
			w.mu.Unlock()
		}

		w.process(d, &local)

		w.mu.Lock()
		w.pending--
		if w.pending == 0 {
			w.cond.Broadcast() // let idle workers exit
		}
		w.mu.Unlock()
	}
}

// enqueue adds a directory to the shared queue, or to local when the queue is full.
func (w *walker) enqueue(d dirItem, local *[]dirItem) {
	w.mu.Lock()
	w.pending++
	if len(w.queue) < w.opts.MaxQueued {
		w.queue = append(w.queue, d)
		w.peakQueued = max(w.peakQueued, len(w.queue))
		w.cond.Signal()
		w.mu.Unlock()
		return
	}
	w.mu.Unlock()
	*local = append(*local, d)
}

// send delivers e unless the walk is cancelled first. A full channel blocks
// here, which is what slows the walker down to the consumer's pace.
func (w *walker) send(e model.Entry) bool {
	select {
	case w.out <- e:
		return true
	case <-w.ctx.Done():
		return false
	}
}

func (w *walker) process(d dirItem, local *[]dirItem) {
	if beforeReadDir != nil {
		beforeReadDir(d.path)
	}
	// On error ReadDir still returns the entries read so far; keep them.
	entries, readErr := os.ReadDir(d.path)
	for _, de := range entries {
		if de.IsDir() && w.opts.Skip != nil && w.opts.Skip(filepath.Join(d.path, de.Name())) {
			continue
		}
		e := model.Entry{ID: w.nextID.Add(1), ParentID: d.id, Name: de.Name(), Kind: kindOf(de.Type())}
		if info, err := de.Info(); err != nil {
			e.Err = errCode(err) // e.g. deleted between ReadDir and Info
		} else {
			e.ModTime = info.ModTime().Unix()
			if e.Kind == model.KindFile {
				e.Size = info.Size()
			}
		}
		if !w.send(e) {
			return
		}
		// Enqueue only after sending, so a directory always precedes its children.
		if e.Kind == model.KindDir && e.Err == "" {
			w.enqueue(dirItem{e.ID, filepath.Join(d.path, e.Name)}, local)
		}
	}
	if readErr != nil {
		w.send(model.Entry{ID: d.id, Err: errCode(readErr)})
	}
}

func kindOf(t fs.FileMode) model.Kind {
	switch {
	case t&fs.ModeSymlink != 0:
		return model.KindLink
	case t&fs.ModeIrregular != 0:
		// Go reports Windows junctions and mount points as irregular files.
		return model.KindLink
	case t.IsDir():
		return model.KindDir
	case t.IsRegular():
		return model.KindFile
	default:
		return model.KindOther
	}
}

func errCode(err error) string {
	switch {
	case errors.Is(err, fs.ErrPermission):
		return model.ErrAccessDenied
	case errors.Is(err, fs.ErrNotExist):
		return model.ErrNotFound
	default:
		return model.ErrOther
	}
}
