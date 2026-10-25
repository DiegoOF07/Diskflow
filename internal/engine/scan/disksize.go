package scan

import "github.com/DiegoOF07/Diskflow/internal/engine/model"

// Windows file attributes that change how much space a file takes on disk.
// They live here, not in the Windows file, so the decision is testable everywhere.
const (
	attrSparse             = 0x200
	attrCompressed         = 0x800
	attrOffline            = 0x1000
	attrRecallOnDataAccess = 0x400000 // cloud placeholder: reading the data downloads it
)

// diskSizeFromAttrs decides the on-disk size of a file from its attributes.
// allocated asks the filesystem for the real figure; it is only called for
// sparse or compressed files, and never for cloud placeholders, so that
// measuring a file can't trigger its download.
func diskSizeFromAttrs(attrs uint32, size, cluster int64, allocated func() (int64, error)) (int64, model.Flags) {
	if attrs&(attrRecallOnDataAccess|attrOffline) != 0 {
		return 0, model.FlagCloud
	}
	var flags model.Flags
	if attrs&attrSparse != 0 {
		flags |= model.FlagSparse
	}
	if attrs&attrCompressed != 0 {
		flags |= model.FlagCompressed
	}
	if flags != 0 {
		if n, err := allocated(); err == nil {
			return n, flags
		}
	}
	return roundUp(size, cluster), flags
}

// roundUp rounds size up to a whole number of clusters. It overestimates tiny
// files that NTFS keeps inside the MFT record, which take no cluster at all.
func roundUp(size, cluster int64) int64 {
	if cluster <= 0 {
		return size
	}
	return (size + cluster - 1) / cluster * cluster
}
