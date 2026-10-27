package desktop

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// showInFileManager opens Explorer with path selected. No shell is involved:
// Explorer is started directly, with path as data.
func showInFileManager(path string) error {
	cmd, err := explorerCommand(path)
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("desktop: explorer: %w", err)
	}
	go cmd.Wait() // Explorer exits with 1 even when it worked; just reap the process
	return nil
}

// explorerCommand builds the command line by hand: Go would quote the whole
// "/select,<path>" argument when the path has spaces, and Explorer then ignores
// /select. Quoting only the path is safe because Windows paths cannot contain '"'.
func explorerCommand(path string) (*exec.Cmd, error) {
	if strings.ContainsRune(path, '"') || !filepath.IsAbs(path) {
		return nil, fmt.Errorf("desktop: refusing to show %q", path)
	}
	exe := "explorer.exe"
	if dir := os.Getenv("SystemRoot"); dir != "" {
		exe = filepath.Join(dir, "explorer.exe") // not whatever explorer.exe comes first in PATH
	}
	cmd := exec.Command(exe)
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: `explorer.exe /select,"` + path + `"`}
	return cmd, nil
}
