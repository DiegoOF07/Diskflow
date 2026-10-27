package store

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/DiegoOF07/Diskflow/internal/engine/model"
)

// BenchmarkSearch measures a search over an index of 1M entries: 10k folders
// of 100 files each. LIKE '%q%' reads every name of the scan, so its cost grows
// with the index, not with the number of hits.
func BenchmarkSearch(b *testing.B) {
	s, err := Open(filepath.Join(b.TempDir(), "bench.db"))
	if err != nil {
		b.Fatal(err)
	}
	defer s.Close()
	id, err := s.BeginScan(ctx, "/r")
	if err != nil {
		b.Fatal(err)
	}
	const dirs, perDir = 10_000, 100
	batch := []model.Entry{{ID: 1, Name: "/r", Kind: model.KindDir}}
	next := int64(2)
	for d := range dirs {
		dirID := next
		next++
		batch = append(batch, model.Entry{ID: dirID, ParentID: 1, Name: fmt.Sprintf("folder-%05d", d), Kind: model.KindDir})
		for f := range perDir {
			batch = append(batch, model.Entry{
				ID: next, ParentID: dirID, Name: fmt.Sprintf("file-%05d-%03d.dat", d, f),
				Kind: model.KindFile, DiskSize: next * 7919 % 1_000_003,
			})
			next++
		}
		if len(batch) >= 50_000 {
			if err := s.InsertEntries(ctx, id, batch); err != nil {
				b.Fatal(err)
			}
			batch = batch[:0]
		}
	}
	if err := s.InsertEntries(ctx, id, batch); err != nil {
		b.Fatal(err)
	}

	for _, q := range []string{
		"file-00042-", // 100 hits
		"-042.dat",    // 10k hits, sorted to keep the biggest
		"nothing",     // no hits
	} {
		b.Run(q, func(b *testing.B) {
			for b.Loop() {
				if _, err := s.Search(ctx, id, q, 200); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
