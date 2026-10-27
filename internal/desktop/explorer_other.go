//go:build !windows

package desktop

import "errors"

// DiskFlow targets Windows; other systems only run the tests and the dev build.
func showInFileManager(string) error {
	return errors.New("desktop: showing files is only supported on Windows")
}
