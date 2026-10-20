package desktop

import (
	"io/fs"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

// Run opens the main window and blocks until it closes.
func Run(assets fs.FS) error {
	app := NewApp()
	return wails.Run(&options.App{
		Title:            "DiskFlow",
		Width:            1200,
		Height:           800,
		MinWidth:         800,
		MinHeight:        560,
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 1},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind:             []any{app},
	})
}
