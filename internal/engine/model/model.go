// Package model defines the types shared by the engine packages and the desktop adapter.
package model

// Kind is the type of a filesystem entry. It is stored as an integer in SQLite.
type Kind uint8

const (
	KindFile Kind = iota
	KindDir
	KindLink // symlinks and junctions; never followed
	KindOther
)

func (k Kind) String() string {
	switch k {
	case KindFile:
		return "file"
	case KindDir:
		return "dir"
	case KindLink:
		return "link"
	default:
		return "other"
	}
}

// Flags explain why an entry's on-disk size differs from its logical size.
// A directory carries the union of the flags found in its subtree.
type Flags uint8

const (
	FlagSparse     Flags = 1 << iota // has unallocated ranges (e.g. virtual disk images)
	FlagCompressed                   // NTFS compression
	FlagCloud                        // cloud placeholder whose data is not on disk (OneDrive)
)

// Error codes stored in Entry.Err. They are stable: the UI maps them to messages.
const (
	ErrAccessDenied = "access_denied"
	ErrNotFound     = "not_found"
	ErrOther        = "error"
)

// Scan statuses. Only a completed scan can become the active index.
const (
	StatusRunning   = "running"
	StatusCompleted = "completed"
	StatusCancelled = "cancelled"
	StatusFailed    = "failed"
)

// Entry is one filesystem item as produced by the walker.
type Entry struct {
	ID       int64  // unique within a scan; the root is always 1
	ParentID int64  // 0 for the root
	Name     string // base name; the root holds its normalized absolute path
	Kind     Kind
	Size     int64 // logical size for files; subtree total for dirs after aggregation
	DiskSize int64 // space allocated on disk; same aggregation as Size
	Flags    Flags
	ModTime  int64 // unix seconds
	Err      string
}

// Node is the read model sent to the UI.
type Node struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	Size     int64  `json:"size"`
	DiskSize int64  `json:"diskSize"`
	Flags    Flags  `json:"flags"`
	Files    int64  `json:"files"` // files in the subtree (1 for a file)
	Err      string `json:"err,omitempty"`
}

// Progress counts what a scan has seen so far.
type Progress struct {
	Files     int64 `json:"files"`
	Dirs      int64 `json:"dirs"`
	Bytes     int64 `json:"bytes"`
	DiskBytes int64 `json:"diskBytes"`
	Errors    int64 `json:"errors"`
}

// Summary describes a finished scan.
type Summary struct {
	ScanID int64  `json:"scanId"`
	Status string `json:"status"`
	Progress
	ElapsedMs int64  `json:"elapsedMs"`
	Error     string `json:"error,omitempty"`
}
