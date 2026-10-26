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
	eng          *engine.Engine
	initErr      error                       // why eng is nil, if it is
	emit         func(name string, data any) // the Wails event bus; replaced in tests
	chooseFolder func() (string, error)      // the native folder dialog; replaced in tests
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.emit = func(name string, data any) { runtime.EventsEmit(ctx, name, data) }
	a.chooseFolder = func() (string, error) {
		return runtime.OpenDirectoryDialog(ctx, runtime.OpenDialogOptions{Title: "Carpeta a analizar"})
	}
	a.eng, a.initErr = openEngine()
}

// shutdown cancels a running scan and closes the database before the process exits.
func (a *App) shutdown(context.Context) {
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

func (a *App) CancelScan() {
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

// Children returns up to limit children of parentID, biggest first.
func (a *App) Children(root string, parentID int64, limit int) ([]model.Node, error) {
	eng, err := a.engine()
	if err != nil {
		return nil, err
	}
	return eng.Children(root, parentID, limit)
}
