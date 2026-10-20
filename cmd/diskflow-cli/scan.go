package main

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/DiegoOF07/Diskflow/internal/engine"
	"github.com/DiegoOF07/Diskflow/internal/engine/model"
)

func runScan(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("scan", stderr)
	db := fs.String("db", "diskflow.db", "database file")
	workers := fs.Int("workers", 0, "walker goroutines (0 = auto)")
	if ok, err := parse(fs, args, 1); !ok {
		return err
	}

	eng, err := engine.New(*db, engine.Options{Workers: *workers})
	if err != nil {
		return err
	}
	defer eng.Close()

	peak := samplePeakHeap()
	var sum model.Summary
	_, err = eng.StartScan(fs.Arg(0), engine.Callbacks{
		OnProgress: func(p model.Progress) {
			fmt.Fprintf(stderr, "\rfiles=%d dirs=%d %s", p.Files, p.Dirs, humanBytes(p.Bytes))
		},
		OnDone: func(s model.Summary) { sum = s },
	})
	if err != nil {
		peak()
		return err
	}

	var cancelAt atomic.Int64 // unix nanos; written by the AfterFunc goroutine
	stopWatch := context.AfterFunc(ctx, func() {
		cancelAt.Store(time.Now().UnixNano())
		eng.Cancel()
	})
	eng.Wait()
	stopWatch()
	fmt.Fprintln(stderr)

	entries := sum.Files + sum.Dirs
	rate := float64(entries) / max(float64(sum.ElapsedMs)/1000, 0.001)
	fmt.Fprintf(stdout, "status=%s files=%d dirs=%d bytes=%d errors=%d elapsed_ms=%d entries_per_s=%.0f peak_heap_mb=%.1f",
		sum.Status, sum.Files, sum.Dirs, sum.Bytes, sum.Errors, sum.ElapsedMs, rate, float64(peak())/(1<<20))
	if at := cancelAt.Load(); at != 0 {
		fmt.Fprintf(stdout, " cancel_to_exit_ms=%d", time.Since(time.Unix(0, at)).Milliseconds())
	}
	fmt.Fprintln(stdout)

	switch sum.Status {
	case model.StatusCompleted:
		return nil
	case model.StatusFailed:
		return fmt.Errorf("scan failed: %s", sum.Error)
	default:
		return fmt.Errorf("scan %s", sum.Status)
	}
}

// samplePeakHeap polls the heap in use every 100 ms until the returned
// function is called, which stops it and reports the peak in bytes.
func samplePeakHeap() (stop func() uint64) {
	var peak uint64
	done := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		t := time.NewTicker(100 * time.Millisecond)
		defer t.Stop()
		var ms runtime.MemStats
		for {
			runtime.ReadMemStats(&ms)
			peak = max(peak, ms.HeapInuse)
			select {
			case <-done:
				return
			case <-t.C:
			}
		}
	}()
	return func() uint64 {
		close(done)
		<-finished // after this, the sampler no longer touches peak
		return peak
	}
}
