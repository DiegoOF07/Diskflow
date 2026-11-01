package desktop

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/wailsapp/wails/v2/pkg/options"
)

// The window paints this colour before the page loads; it must match the
// page's --paper token, or the window flashes another colour on start.
func TestBackgroundMatchesThePaperOfEachTheme(t *testing.T) {
	if got, want := *background(false), (options.RGBA{R: 0xee, G: 0xf2, B: 0xee, A: 255}); got != want {
		t.Errorf("light background = %+v, want %+v (#eef2ee)", got, want)
	}
	if got, want := *background(true), (options.RGBA{R: 0x13, G: 0x1b, B: 0x22, A: 255}); got != want {
		t.Errorf("dark background = %+v, want %+v (#131b22)", got, want)
	}
}

// The colours live twice, here and in the CSS tokens; this keeps them in step.
func TestBackgroundIsThePaperTokenOfTheCSS(t *testing.T) {
	css, err := os.ReadFile("../../frontend/src/style.css")
	if err != nil {
		t.Fatal(err)
	}
	for _, dark := range []bool{false, true} {
		c := background(dark)
		token := fmt.Sprintf("--paper: #%02x%02x%02x;", c.R, c.G, c.B)
		if !strings.Contains(string(css), token) {
			t.Errorf("style.css has no %q (dark = %v)", token, dark)
		}
	}
}

func TestPrefersDarkDoesNotFail(t *testing.T) {
	prefersDark() // reads the registry on Windows; false elsewhere
}
