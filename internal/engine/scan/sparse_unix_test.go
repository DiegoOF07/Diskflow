//go:build unix

package scan

import (
	"os"
	"testing"
)

// makeSparse creates a file of the given size with no data blocks: Unix
// filesystems leave the range a Truncate extends as a hole.
func makeSparse(t *testing.T, path string, size int64) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := f.Truncate(size); err != nil {
		t.Fatal(err)
	}
}
