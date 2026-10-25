//go:build unix

package scan

import (
	"io/fs"
	"syscall"

	"github.com/DiegoOF07/Diskflow/internal/engine/model"
)

// diskSizer computes on-disk sizes. Unix reports them exactly, per file.
type diskSizer struct{}

func newDiskSizer(root string) diskSizer { return diskSizer{} }

// size uses st_blocks, always counted in 512-byte units. Any shortfall from
// the logical size is reported as sparse, although on filesystems such as
// btrfs it may also come from compression.
func (diskSizer) size(dir, name string, info fs.FileInfo) (int64, model.Flags) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return info.Size(), 0
	}
	n := int64(st.Blocks) * 512
	if n < info.Size() {
		return n, model.FlagSparse
	}
	return n, 0
}
