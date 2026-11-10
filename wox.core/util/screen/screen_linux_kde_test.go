//go:build linux

package screen

import (
	"os"
	"testing"
)

func TestParseKWinDisplays(t *testing.T) {
	support := `Screens
-------
Number of Screens: 2

Screen 0:
---------
Name: DP-1
Enabled: 1
Geometry: 0,800,1920x1080
Scale: 1
Screen 1:
---------
Name: HDMI-A-1
Enabled: 1
Geometry: 0,0,1280x800
Scale: 1.5
Compositing
-------`
	displays := parseKWinDisplays(support)
	if len(displays) != 2 {
		t.Fatalf("KWin display count = %d", len(displays))
	}
	if displays[0].Name != "DP-1" || displays[0].Logical != (Rect{X: 0, Y: 800, Width: 1920, Height: 1080}) || displays[0].PixelWidth != 1920 || displays[0].PixelHeight != 1080 {
		t.Fatalf("first KWin display = %#v", displays[0])
	}
	if displays[1].Name != "HDMI-A-1" || displays[1].Logical != (Rect{X: 0, Y: 0, Width: 1280, Height: 800}) || displays[1].PixelWidth != 1920 || displays[1].PixelHeight != 1200 {
		t.Fatalf("scaled KWin display = %#v", displays[1])
	}
}

func TestKWinLiveActiveDisplay(t *testing.T) {
	if os.Getenv("WOX_LIVE_KWIN_SCREEN") != "1" {
		t.Skip("set WOX_LIVE_KWIN_SCREEN=1 to query KWin")
	}
	display, err := GetKWinActiveDisplay()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("active KWin display: %+v", display)
}

// TestParseKWinDisplaysNegativeOrigin preserves desktop coordinates across mixed-DPI outputs.
func TestParseKWinDisplaysNegativeOrigin(t *testing.T) {
	displays := parseKWinDisplays(`Screen 0:
Name: DP-1
Geometry: -1536,-100,1536x864
Scale: 1.25
Screen 1:
Name: HDMI-A-1
Geometry: 0,0,1920x1080
Scale: 1
Screen 2:
Name: disabled
Enabled: 0
Geometry: 1920,0,1280x800
Scale: 1`)
	if len(displays) != 2 {
		t.Fatalf("valid KWin displays = %+v", displays)
	}
	if got := displays[0]; got.Logical != (Rect{X: -1536, Y: -100, Width: 1536, Height: 864}) || got.PixelWidth != 1920 || got.PixelHeight != 1080 {
		t.Fatalf("negative-origin scaled output = %+v", got)
	}
}
