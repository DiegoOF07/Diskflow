package desktop

import "github.com/wailsapp/wails/v2/pkg/options"

// background is the colour the window paints before the page loads: the
// --paper token of frontend/src/style.css for each theme.
func background(dark bool) *options.RGBA {
	if dark {
		return &options.RGBA{R: 0x13, G: 0x1b, B: 0x22, A: 255}
	}
	return &options.RGBA{R: 0xee, G: 0xf2, B: 0xee, A: 255}
}
