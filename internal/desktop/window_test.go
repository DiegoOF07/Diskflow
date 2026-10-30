package desktop

import (
	"testing"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func TestFitSizeKeepsTheWindowOnScreen(t *testing.T) {
	for _, c := range []struct {
		name          string
		screenW, scrH int
		wantW, wantH  int
	}{
		{"big screen keeps the preferred size", 1920, 1080, prefWidth, prefHeight},
		{"laptop 1366x768 at 125%", 1093, 614, 983, 552},
		{"1080p at 150%", 1280, 720, 1152, 648},
		{"tiny screen gets the minimum", 600, 400, minWidth, minHeight},
	} {
		w, h := fitSize(c.screenW, c.scrH)
		if w != c.wantW || h != c.wantH {
			t.Errorf("%s: fitSize(%d, %d) = %dx%d, want %dx%d", c.name, c.screenW, c.scrH, w, h, c.wantW, c.wantH)
		}
	}
}

func TestCurrentScreen(t *testing.T) {
	screen := func(w int, current, primary bool) runtime.Screen {
		s := runtime.Screen{IsCurrent: current, IsPrimary: primary}
		s.Size.Width, s.Size.Height = w, w
		return s
	}
	if s, ok := currentScreen([]runtime.Screen{screen(1, false, true), screen(2, true, false)}); !ok || s.Size.Width != 2 {
		t.Errorf("got %+v, want the screen the window is on", s)
	}
	if s, ok := currentScreen([]runtime.Screen{screen(1, false, false), screen(2, false, true)}); !ok || s.Size.Width != 2 {
		t.Errorf("got %+v, want the primary screen when none is current", s)
	}
	if _, ok := currentScreen(nil); ok {
		t.Error("no screens: ok = true")
	}
	if _, ok := currentScreen([]runtime.Screen{screen(0, true, true)}); ok {
		t.Error("a screen of size 0 (Wails failed to read it): ok = true")
	}
}
