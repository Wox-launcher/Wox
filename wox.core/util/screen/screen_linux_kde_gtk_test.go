//go:build linux && cgo

package screen

import (
	"os"
	"testing"

	"github.com/godbus/dbus/v5"
)

// TestKDELiveMouseScreen compares Wox's lookup with the real compositor and GTK workarea.
func TestKDELiveMouseScreen(t *testing.T) {
	if os.Getenv("WOX_LIVE_KWIN_SCREEN") != "1" {
		t.Skip("set WOX_LIVE_KWIN_SCREEN=1 in KDE Wayland")
	}
	display, err := GetKWinActiveDisplay()
	if err != nil {
		t.Fatal(err)
	}
	size := GetMouseScreen()
	logical := display.Logical
	if size.Width <= 0 || size.Height <= 0 || size.X < logical.X || size.Y < logical.Y || size.X+size.Width > logical.Right() || size.Y+size.Height > logical.Bottom() {
		t.Fatalf("mouse screen %+v outside active output %+v; %s", size, display, LastMouseScreenDebug())
	}
	t.Logf("active output=%+v Wox workarea=%+v debug=%s", display, size, LastMouseScreenDebug())
}

// TestKDELiveDisplayWorkareas checks every real output, including fractional scaling and desktop offsets.
func TestKDELiveDisplayWorkareas(t *testing.T) {
	if os.Getenv("WOX_LIVE_KWIN_SCREEN") != "1" {
		t.Skip("set WOX_LIVE_KWIN_SCREEN=1 in KDE Wayland")
	}
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	var support string
	if err := conn.Object("org.kde.KWin", "/KWin").Call("org.kde.KWin.supportInformation", 0).Store(&support); err != nil {
		t.Fatal(err)
	}
	displays := parseKWinDisplays(support)
	if len(displays) == 0 {
		t.Fatal("KWin has no outputs")
	}
	for _, display := range displays {
		t.Run(display.Name, func(t *testing.T) {
			logical := display.Logical
			area, err := getScreenGtkAtPoint(logical.X+logical.Width/2, logical.Y+logical.Height/2)
			if err != nil {
				t.Fatal(err)
			}
			if area.Width <= 0 || area.Height <= 0 || area.X < logical.X || area.Y < logical.Y || area.X+area.Width > logical.Right() || area.Y+area.Height > logical.Bottom() {
				t.Fatalf("GTK workarea %+v outside compositor output %+v", area, display)
			}
			t.Logf("output=%+v GTK workarea=%+v", display, area)
		})
	}
}
