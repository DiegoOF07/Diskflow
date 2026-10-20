package engine

import (
	"context"
	"time"

	"github.com/DiegoOF07/Diskflow/internal/engine/model"
	"github.com/DiegoOF07/Diskflow/internal/engine/scan"
	"github.com/DiegoOF07/Diskflow/internal/engine/store"
)

const (
	batchSize  = 2000 // entries per insert transaction
	walkBuffer = 1024 // entries buffered between walker and aggregator
)

// dirAgg accumulates a directory's subtree while the scan runs.
type dirAgg struct {
	parent int64
	size   int64
	files  int64
	err    string
}

// scanRun holds the state of one scan. Only the goroutine running
// (*Engine).run touches it, so it needs no locks.
type scanRun struct {
	id       int64
	st       *store.Store
	progress model.Progress
	dirs     map[int64]*dirAgg
	batch    []model.Entry
	storeErr error
}

// run walks root and stores the result. It is the single consumer of the
// walker's channel: it aggregates, batches inserts and reports progress.
func (e *Engine) run(ctx context.Context, id int64, root string, cb Callbacks) {
	start := time.Now()
	ctx, stopWalk := context.WithCancel(ctx)
	defer stopWalk()

	out := make(chan model.Entry, walkBuffer)
	walkErr := make(chan error, 1)
	go func() {
		err := scan.Walk(ctx, root, scan.Options{Workers: e.opts.Workers, Skip: e.isDBDir}, out)
		close(out) // the walker never closes out; its only producer does it here
		walkErr <- err
	}()

	r := &scanRun{id: id, st: e.st, dirs: map[int64]*dirAgg{}, batch: make([]model.Entry, 0, batchSize)}
	ticker := time.NewTicker(ProgressInterval)
	defer ticker.Stop()

consume:
	for {
		select {
		case entry, ok := <-out:
			if !ok {
				break consume
			}
			r.add(entry)
			if r.storeErr != nil {
				stopWalk() // keep draining until the walker notices
			}
		case <-ticker.C:
			cb.OnProgress(r.progress)
		}
	}

	sum := r.finish(ctx, <-walkErr)
	sum.ElapsedMs = time.Since(start).Milliseconds()
	cb.OnProgress(r.progress)
	cb.OnDone(sum)

	// Release the engine only after OnDone, so a new scan's events can never
	// arrive before this scan's OnDone.
	e.mu.Lock()
	e.cancel = nil
	done := e.done
	e.mu.Unlock()
	close(done)
}

// add folds one walker entry into the aggregates and the pending batch.
func (r *scanRun) add(entry model.Entry) {
	if r.storeErr != nil {
		return
	}
	if scan.IsPatch(entry) {
		if d := r.dirs[entry.ID]; d != nil {
			d.err = entry.Err
		}
		r.progress.Errors++
		return
	}
	if entry.Err != "" {
		r.progress.Errors++
	}
	switch entry.Kind {
	case model.KindDir:
		r.dirs[entry.ID] = &dirAgg{parent: entry.ParentID}
		r.progress.Dirs++
	case model.KindFile:
		r.progress.Files++
		r.progress.Bytes += entry.Size
		// The walker emits every directory before its children, so all
		// ancestors are already in r.dirs.
		for p := entry.ParentID; p != 0; {
			d := r.dirs[p]
			d.size += entry.Size
			d.files++
			p = d.parent
		}
	}
	r.batch = append(r.batch, entry)
	if len(r.batch) == batchSize {
		r.flush()
	}
}

// flush writes the pending batch. Store calls use a fresh context: cancelling
// the scan must not abort the bookkeeping that records the cancellation.
func (r *scanRun) flush() {
	if r.storeErr == nil && len(r.batch) > 0 {
		r.storeErr = r.st.InsertEntries(context.Background(), r.id, r.batch)
	}
	r.batch = r.batch[:0]
}

// finish decides the final status and closes the scan in the store.
func (r *scanRun) finish(ctx context.Context, walkErr error) model.Summary {
	sum := model.Summary{ScanID: r.id}
	switch {
	case r.storeErr != nil:
		sum.Status, sum.Error = model.StatusFailed, r.storeErr.Error()
	case ctx.Err() != nil:
		sum.Status = model.StatusCancelled
	case walkErr != nil:
		sum.Status, sum.Error = model.StatusFailed, walkErr.Error()
	default:
		r.flush()
		if r.storeErr == nil {
			r.storeErr = r.st.SetDirTotals(context.Background(), r.id, r.totals())
		}
		if r.storeErr != nil {
			sum.Status, sum.Error = model.StatusFailed, r.storeErr.Error()
		} else {
			sum.Status = model.StatusCompleted
		}
	}
	if err := r.st.FinishScan(context.Background(), r.id, sum.Status, r.progress); err != nil && sum.Error == "" {
		sum.Status, sum.Error = model.StatusFailed, err.Error()
	}
	sum.Progress = r.progress
	return sum
}

func (r *scanRun) totals() map[int64]store.DirTotal {
	t := make(map[int64]store.DirTotal, len(r.dirs))
	for id, d := range r.dirs {
		t[id] = store.DirTotal{Size: d.size, Files: d.files, Err: d.err}
	}
	return t
}

func (e *Engine) isDBDir(path string) bool {
	return foldCase(path, caseInsensitiveFS) == e.dbDir
}
