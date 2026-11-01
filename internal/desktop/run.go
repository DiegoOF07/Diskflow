package desktop

import (
	"io/fs"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

// Run opens the main window and blocks until it closes.
func Run(assets fs.FS) error {
	return wails.Run(appOptions(NewApp(), assets))
}

func appOptions(app *App, assets fs.FS) *options.App {
	return &options.App{
		Title:            "DiskFlow",
		Width:            prefWidth,
		Height:           prefHeight,
		MinWidth:         minWidth,
		MinHeight:        minHeight,
		StartHidden:      true, // shown by startup once it fits the screen
		AssetServer:      &assetserver.Options{Assets: assets},
		BackgroundColour: background(prefersDark()),
		Windows:          &windows.Options{Theme: windows.SystemDefault}, // the title bar follows the Windows theme
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		Bind:             []any{app},
		ErrorFormatter:   formatError,
		// A second window would write the same index; opening the app again
		// brings the running one to the front instead.
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId:               "com.github.diegoof07.diskflow",
			OnSecondInstanceLaunch: func(options.SecondInstanceData) { app.focus() },
		},
	}
}
