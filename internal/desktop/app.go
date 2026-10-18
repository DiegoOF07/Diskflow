// Package desktop is the only package that imports Wails. Keeping the
// dependency here limits a future Wails v3 migration to this package and main.go.
package desktop

import "context"

// App is bound to the frontend: every exported method becomes a TS function.
type App struct {
	ctx context.Context
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}
