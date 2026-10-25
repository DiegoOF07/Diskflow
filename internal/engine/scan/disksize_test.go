package scan

import (
	"errors"
	"testing"

	"github.com/DiegoOF07/Diskflow/internal/engine/model"
)

func TestDiskSizeFromAttrs(t *testing.T) {
	const cluster = 4096
	allocated := func(n int64) func() (int64, error) {
		return func() (int64, error) { return n, nil }
	}
	failing := func() (int64, error) { return 0, errors.New("boom") }

	tests := []struct {
		name      string
		attrs     uint32
		size      int64
		cluster   int64
		allocated func() (int64, error) // nil: must not be called
		wantSize  int64
		wantFlags model.Flags
	}{
		{"plain rounds up to cluster", 0, 10_000, cluster, nil, 12_288, 0},
		{"plain exact multiple", 0, 8192, cluster, nil, 8192, 0},
		{"empty file", 0, 0, cluster, nil, 0, 0},
		{"unknown cluster keeps logical", 0, 10_000, 0, nil, 10_000, 0},
		{"sparse asks the filesystem", attrSparse, 512 << 30, cluster, allocated(3 << 30), 3 << 30, model.FlagSparse},
		{"compressed asks the filesystem", attrCompressed, 10_000, cluster, allocated(5000), 5000, model.FlagCompressed},
		{"sparse and compressed", attrSparse | attrCompressed, 10_000, cluster, allocated(1000), 1000, model.FlagSparse | model.FlagCompressed},
		{"failed query falls back to rounding", attrSparse, 10_000, cluster, failing, 12_288, model.FlagSparse},
		{"cloud placeholder counts zero", attrRecallOnDataAccess, 10_000, cluster, nil, 0, model.FlagCloud},
		{"offline counts zero", attrOffline, 10_000, cluster, nil, 0, model.FlagCloud},
		{"cloud wins over sparse", attrRecallOnDataAccess | attrSparse, 10_000, cluster, nil, 0, model.FlagCloud},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alloc := tt.allocated
			if alloc == nil {
				alloc = func() (int64, error) {
					t.Fatal("allocated size queried; it must not touch the file")
					return 0, nil
				}
			}
			size, flags := diskSizeFromAttrs(tt.attrs, tt.size, tt.cluster, alloc)
			if size != tt.wantSize || flags != tt.wantFlags {
				t.Fatalf("got (%d, %03b), want (%d, %03b)", size, flags, tt.wantSize, tt.wantFlags)
			}
		})
	}
}
