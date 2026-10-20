package engine

import (
	"path/filepath"
	"runtime"
	"strings"
)

// NTFS is case-insensitive: C:\Work and c:\work are the same folder.
var caseInsensitiveFS = runtime.GOOS == "windows"

// rootKey turns a user-supplied path into the key a scan is stored under:
// absolute, cleaned and, on case-insensitive filesystems, lower-cased.
func rootKey(path string, caseInsensitive bool) (string, error) {
	abs, err := filepath.Abs(path) // Abs also cleans ("a/./b/" -> "a/b")
	if err != nil {
		return "", err
	}
	return foldCase(abs, caseInsensitive), nil
}

func foldCase(path string, caseInsensitive bool) string {
	if caseInsensitive {
		return strings.ToLower(path)
	}
	return path
}
