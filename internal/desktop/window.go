package desktop

import (
	"context"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Window sizes, in DPI-scaled units like runtime.Screen.Size.
const (
	prefWidth, prefHeight = 1200, 800
	minWidth, minHeight   = 640, 480
)

// fitSize is the preferred window size shrunk to 90% of the screen, so the
// title bar and the taskbar stay visible on small or scaled laptop screens.
func fitSize(screenW, screenH int) (w, h int) {
	w = max(min(prefWidth, screenW*9/10), minWidth)
	h = max(min(prefHeight, screenH*9/10), minHeight)
	return w, h
}

// currentScreen is the screen the window is on, or else the primary one.
// Wails leaves a zero size for a screen it could not read.
func currentScreen(screens []runtime.Screen) (runtime.Screen, bool) {
	var found *runtime.Screen
	for i, s := range screens {
		if s.IsCurrent {
			found = &screens[i]
			break
		}
		if s.IsPrimary && found == nil {
			found = &screens[i]
		}
	}
	if found == nil || found.Size.Width == 0 || found.Size.Height == 0 {
		return runtime.Screen{}, false
	}
	return *found, true
}

// showFitted sizes the window to its screen, centres it and shows it. The
// window starts hidden so it never appears at a size that doesn't fit.
func showFitted(ctx context.Context) {
	defer runtime.WindowShow(ctx)
	screens, _ := runtime.ScreenGetAll(ctx) // on a partial error, the screens read are still usable
	if s, ok := currentScreen(screens); ok {
		w, h := fitSize(s.Size.Width, s.Size.Height)
		runtime.WindowSetSize(ctx, w, h)
	}
	runtime.WindowCenter(ctx)
}
