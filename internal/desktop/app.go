// Package desktop is the only package that imports Wails. Keeping the
// dependency here limits a future Wails v3 migration to this package and main.go.
package desktop

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/DiegoOF07/Diskflow/internal/engine"
	"github.com/DiegoOF07/Diskflow/internal/engine/model"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Events emitted to the frontend. Keep in sync with frontend/src/backend/wails.ts.
const (
	EventProgress = "scan:progress" // model.Progress
	EventSaving   = "scan:saving"   // no data; the scan can no longer be cancelled
	EventDone     = "scan:done"     // model.Summary
)

var errNotReady = errors.New("desktop: the index database is not available")

// App is bound to the frontend: every exported method becomes a TS function.
// It only translates between Wails and the engine; the logic lives in the engine.
type App struct {
	ctx          context.Context // the Wails runtime's; nil until startup
	eng          *engine.Engine
	initErr      error                       // why eng is nil, if it is
	emit         func(name string, data any) // the Wails event bus; replaced in tests
	chooseFolder func() (string, error)      // the native folder dialog; replaced in tests
	showFile     func(path string) error     // selects path in Explorer; replaced in tests
	copyText     func(text string) error     // the clipboard; replaced in tests
	open         func() (*engine.Engine, error)
	ready        chan struct{} // closed when startup has finished; nil in tests that build App directly
}

func NewApp() *App {
	return &App{open: openEngine, ready: make(chan struct{})}
}

// startup runs in its own goroutine while the page loads (Wails on Windows),
// so bound methods call wait before touching what it sets.
func (a *App) startup(ctx context.Context) {
	defer close(a.ready)
	a.ctx = ctx
	a.emit = func(name string, data any) { runtime.EventsEmit(ctx, name, data) }
	a.chooseFolder = func() (string, error) {
		return runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{Title: "Carpeta a analizar"})
	}
	a.showFile = showInFileManager
	a.copyText = func(text string) error { return runtime.ClipboardSetText(ctx, text) }
	a.eng, a.initErr = a.open()
}

// wait blocks until startup has finished. Opening the database can take
// seconds when a migration runs, and the page calls in the meantime.
func (a *App) wait() {
	if a.ready != nil {
		<-a.ready
	}
}

// focus brings the window to the front, restoring it if it was minimised.
func (a *App) focus() {
	a.wait()
	if a.ctx == nil {
		return
	}
	runtime.WindowUnminimise(a.ctx)
	runtime.WindowShow(a.ctx)
}

// shutdown cancels a running scan and closes the database before the process exits.
func (a *App) shutdown(context.Context) {
	a.wait()
	if a.eng != nil {
		a.eng.Close()
	}
}

// openEngine opens the index in the user's local app data
// (%LocalAppData%\DiskFlow on Windows), which does not roam between machines.
func openEngine() (*engine.Engine, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	return engine.New(filepath.Join(dir, "DiskFlow", "diskflow.db"), engine.Options{})
}

func (a *App) engine() (*engine.Engine, error) {
	a.wait()
	if a.eng == nil {
		return nil, fmt.Errorf("%w: %v", errNotReady, a.initErr)
	}
	return a.eng, nil
}

// PickFolder opens the native folder dialog and adds the chosen folder to the
// catalog. It returns nil if the user cancels.
func (a *App) PickFolder() (*model.RootInfo, error) {
	eng, err := a.engine()
	if err != nil {
		return nil, err
	}
	path, err := a.chooseFolder()
	if err != nil || path == "" {
		return nil, err
	}
	r, err := eng.AddRoot(path, "")
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// ListRoots returns the catalog with each folder's latest scans.
func (a *App) ListRoots() ([]model.RootInfo, error) {
	eng, err := a.engine()
	if err != nil {
		return nil, err
	}
	return eng.ListRoots()
}

func (a *App) GetRoot(id int64) (model.RootInfo, error) {
	eng, err := a.engine()
	if err != nil {
		return model.RootInfo{}, err
	}
	return eng.GetRoot(id)
}

func (a *App) RenameRoot(id int64, name string) error {
	eng, err := a.engine()
	if err != nil {
		return err
	}
	return eng.RenameRoot(id, name)
}

// RemoveRoot takes a folder out of the catalog. Its files are never touched.
func (a *App) RemoveRoot(id int64) error {
	eng, err := a.engine()
	if err != nil {
		return err
	}
	return eng.RemoveRoot(id)
}

// StartScan scans root in the background. Progress and the result arrive as
// EventProgress, EventSaving and EventDone.
func (a *App) StartScan(root string) error {
	eng, err := a.engine()
	if err != nil {
		return err
	}
	_, err = eng.StartScan(root, engine.Callbacks{
		OnProgress: func(p model.Progress) { a.emit(EventProgress, p) },
		OnSaving:   func() { a.emit(EventSaving, nil) },
		OnDone:     func(s model.Summary) { a.emit(EventDone, s) },
	})
	return err
}

// ScanState tells whether root is being scanned: "" (no), "running" or
// "saving". A view opened during a scan uses it to show the scan in progress.
func (a *App) ScanState(root string) (string, error) {
	eng, err := a.engine()
	if err != nil {
		return "", err
	}
	return eng.ScanState(root)
}

func (a *App) CancelScan() {
	a.wait()
	if a.eng != nil {
		a.eng.Cancel()
	}
}

// Root returns the root of the last completed scan of root, or nil if it was
// never scanned: that is a normal state for the UI, not an error.
func (a *App) Root(root string) (*model.Node, error) {
	eng, err := a.engine()
	if err != nil {
		return nil, err
	}
	n, err := eng.Root(root)
	if errors.Is(err, engine.ErrNoScan) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &n, nil
}

// ListChildren returns one page of parentID's children, sorted as q asks.
func (a *App) ListChildren(root string, parentID int64, q model.ListQuery) (model.Page, error) {
	eng, err := a.engine()
	if err != nil {
		return model.Page{}, err
	}
	return eng.ListChildren(root, parentID, q)
}

// Search returns up to limit entries whose name contains q, biggest first.
// A newer search cancels this one, which then rejects.
func (a *App) Search(root, q string, limit int) (model.SearchResult, error) {
	eng, err := a.engine()
	if err != nil {
		return model.SearchResult{}, err
	}
	return eng.Search(root, q, limit)
}

// ShowInExplorer opens Explorer with entry id selected. The UI only sends ids:
// the path is rebuilt from the index and checked here, never taken from the UI.
func (a *App) ShowInExplorer(root string, id int64) error {
	path, err := a.resolve(root, id)
	if err != nil {
		return err
	}
	return a.showFile(path)
}

// CopyPath puts the absolute path of entry id in the clipboard.
func (a *App) CopyPath(root string, id int64) error {
	path, err := a.resolve(root, id)
	if err != nil {
		return err
	}
	return a.copyText(path)
}

func (a *App) resolve(root string, id int64) (string, error) {
	eng, err := a.engine()
	if err != nil {
		return "", err
	}
	return eng.ResolvePath(root, id)
}

// CancelSearch stops the running search; the UI calls it when the search box is cleared.
func (a *App) CancelSearch() {
	a.wait()
	if a.eng != nil {
		a.eng.CancelSearch()
	}
}

// Trail returns the entries from the root down to id, to show where a search hit is.
func (a *App) Trail(root string, id int64) ([]model.Node, error) {
	eng, err := a.engine()
	if err != nil {
		return nil, err
	}
	return eng.Trail(root, id)
}

// Unreadable returns up to limit entries the last scan could not read, with
// their folders, for the partial coverage panel.
func (a *App) Unreadable(root string, limit int) (model.SearchResult, error) {
	eng, err := a.engine()
	if err != nil {
		return model.SearchResult{}, err
	}
	return eng.Unreadable(root, limit)
}

// Children returns up to limit children of parentID, biggest first.
func (a *App) Children(root string, parentID int64, limit int) ([]model.Node, error) {
	eng, err := a.engine()
	if err != nil {
		return nil, err
	}
	return eng.Children(root, parentID, limit)
}
