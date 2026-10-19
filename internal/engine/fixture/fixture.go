// Package fixture generates deterministic synthetic directory trees for tests
// and benchmarks. It is the only engine package that writes files, and only
// inside the output directory it is given.
package fixture

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
)

// Spec describes the tree to generate. Zero values take the defaults.
type Spec struct {
	Entries     int   // files + dirs to create, not counting the output dir
	FanOut      int   // subdirectories per directory (default 8)
	FilesPerDir int   // files per directory (default 20)
	MaxDepth    int   // deepest directory level below the output dir (default 6)
	FileSize    int64 // mean logical file size in bytes (default 4096)
	Seed        uint64
}

// Stats summarizes what Generate created.
type Stats struct {
	Files int
	Dirs  int
	Bytes int64
}

var ErrShapeTooSmall = errors.New("fixture: FanOut/FilesPerDir/MaxDepth cannot hold Entries")

func (s *Spec) applyDefaults() {
	if s.FanOut <= 0 {
		s.FanOut = 8
	}
	if s.FilesPerDir <= 0 {
		s.FilesPerDir = 20
	}
	if s.MaxDepth <= 0 {
		s.MaxDepth = 6
	}
	if s.FileSize <= 0 {
		s.FileSize = 4096
	}
}

// Generate fills dir breadth-first: each directory gets its files and then its
// subdirectories, level by level, until Entries items exist. The same Spec
// always produces the same tree. dir must not exist or be empty.
func Generate(dir string, spec Spec) (Stats, error) {
	spec.applyDefaults()
	var st Stats
	if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
		return st, fmt.Errorf("fixture: %s is not empty", dir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return st, err
	}

	rng := rand.New(rand.NewPCG(spec.Seed, 0))
	type pending struct {
		path  string
		depth int
	}
	queue := []pending{{dir, 0}}
	left := spec.Entries

	for len(queue) > 0 && left > 0 {
		cur := queue[0]
		queue = queue[1:]

		for i := 0; i < spec.FilesPerDir && left > 0; i++ {
			size := rng.Int64N(2 * spec.FileSize) // uniform in [0, 2*mean)
			if err := createFile(filepath.Join(cur.path, fmt.Sprintf("f%03d.bin", i)), size); err != nil {
				return st, err
			}
			st.Files++
			st.Bytes += size
			left--
		}
		if cur.depth == spec.MaxDepth {
			continue
		}
		for i := 0; i < spec.FanOut && left > 0; i++ {
			sub := filepath.Join(cur.path, fmt.Sprintf("d%02d", i))
			if err := os.Mkdir(sub, 0o755); err != nil {
				return st, err
			}
			st.Dirs++
			left--
			queue = append(queue, pending{sub, cur.depth + 1})
		}
	}
	if left > 0 {
		return st, ErrShapeTooSmall
	}
	return st, nil
}

// createFile makes a file of the given logical size without writing data:
// Truncate only moves the end-of-file marker.
func createFile(path string, size int64) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	if err := f.Truncate(size); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
