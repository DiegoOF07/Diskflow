package scan

import (
	"os"
	"testing"

	"golang.org/x/sys/windows"
)

// makeSparse creates a file of the given size with no allocated clusters. NTFS
// only leaves holes in files marked sparse; otherwise Truncate zero-fills.
func makeSparse(t *testing.T, path string, size int64) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var n uint32
	if err := windows.DeviceIoControl(windows.Handle(f.Fd()), windows.FSCTL_SET_SPARSE, nil, 0, nil, 0, &n, nil); err != nil {
		t.Fatalf("FSCTL_SET_SPARSE: %v", err)
	}
	if err := f.Truncate(size); err != nil {
		t.Fatal(err)
	}
}
