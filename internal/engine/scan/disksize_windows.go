package scan

import (
	"io/fs"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/DiegoOF07/Diskflow/internal/engine/model"
)

// Neither function is wrapped by x/sys/windows, so they are loaded from
// kernel32 on first use.
var (
	kernel32                   = windows.NewLazySystemDLL("kernel32.dll")
	procGetCompressedFileSizeW = kernel32.NewProc("GetCompressedFileSizeW")
	procGetDiskFreeSpaceW      = kernel32.NewProc("GetDiskFreeSpaceW")
)

// diskSizer computes on-disk sizes on the volume of one walk. Mount points
// are never followed, so a walk never leaves its volume.
type diskSizer struct {
	cluster int64 // 0 if unknown: sizes are then left unrounded
}

func newDiskSizer(root string) diskSizer {
	return diskSizer{cluster: clusterSize(root)}
}

// size reads the attributes Go already fetched with the directory listing
// (FILE_ID_BOTH_DIR_INFO), so most files cost no extra system call.
func (s diskSizer) size(dir, name string, info fs.FileInfo) (int64, model.Flags) {
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return roundUp(info.Size(), s.cluster), 0
	}
	return diskSizeFromAttrs(data.FileAttributes, info.Size(), s.cluster, func() (int64, error) {
		return compressedFileSize(filepath.Join(dir, name))
	})
}

const invalidFileSize = 0xFFFFFFFF // INVALID_FILE_SIZE, missing from x/sys/windows

// compressedFileSize returns the bytes actually allocated to a sparse or
// compressed file.
func compressedFileSize(path string) (int64, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var high uint32
	low, _, callErr := procGetCompressedFileSizeW.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&high)))
	// 0xFFFFFFFF is also a valid low half, so only the last error tells them apart.
	if uint32(low) == invalidFileSize && callErr != windows.ERROR_SUCCESS {
		return 0, callErr
	}
	return int64(high)<<32 | int64(uint32(low)), nil
}

// clusterSize returns the allocation unit of the volume holding path, or 0.
func clusterSize(path string) int64 {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0
	}
	vol := make([]uint16, windows.MAX_PATH+1)
	if err := windows.GetVolumePathName(p, &vol[0], uint32(len(vol))); err != nil {
		return 0
	}
	var sectorsPerCluster, bytesPerSector, freeClusters, totalClusters uint32
	ok, _, _ := procGetDiskFreeSpaceW.Call(uintptr(unsafe.Pointer(&vol[0])),
		uintptr(unsafe.Pointer(&sectorsPerCluster)), uintptr(unsafe.Pointer(&bytesPerSector)),
		uintptr(unsafe.Pointer(&freeClusters)), uintptr(unsafe.Pointer(&totalClusters)))
	if ok == 0 {
		return 0
	}
	return int64(sectorsPerCluster) * int64(bytesPerSector)
}
