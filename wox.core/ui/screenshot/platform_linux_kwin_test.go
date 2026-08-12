//go:build linux

package screenshot

import (
	"bytes"
	"image/color"
	"os"
	"testing"
	utilscreen "wox/util/screen"

	"github.com/godbus/dbus/v5"
)

func TestParseLinuxKWinDisplayGeometries(t *testing.T) {
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
	displays := parseLinuxKWinDisplayGeometries(support)
	if len(displays) != 2 {
		t.Fatalf("KWin display count = %d", len(displays))
	}
	if displays[0].Name != "DP-1" || displays[0].Logical != (utilscreen.Rect{X: 0, Y: 800, Width: 1920, Height: 1080}) || displays[0].PixelWidth != 1920 || displays[0].PixelHeight != 1080 {
		t.Fatalf("first KWin display = %#v", displays[0])
	}
	if displays[1].Name != "HDMI-A-1" || displays[1].Logical != (utilscreen.Rect{X: 0, Y: 0, Width: 1280, Height: 800}) || displays[1].PixelWidth != 1920 || displays[1].PixelHeight != 1200 {
		t.Fatalf("scaled KWin display = %#v", displays[1])
	}
}

func TestLinuxKWinLiveActiveScreen(t *testing.T) {
	if os.Getenv("WOX_LIVE_KWIN_SCREEN") != "1" {
		t.Skip("set WOX_LIVE_KWIN_SCREEN=1 to query KWin")
	}
	display, err := linuxKWinActiveScreenIdentity()
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("active KWin display: %+v", display)
}

func TestReadLinuxKWinScreenshotARGB32Premultiplied(t *testing.T) {
	metadata := map[string]dbus.Variant{
		"type":   dbus.MakeVariant("raw"),
		"width":  dbus.MakeVariant(uint32(2)),
		"height": dbus.MakeVariant(uint32(1)),
		"stride": dbus.MakeVariant(uint32(12)),
		"format": dbus.MakeVariant(uint32(linuxKWinImageFormatARGB32Premul)),
		"scale":  dbus.MakeVariant(1.5),
		"screen": dbus.MakeVariant("HDMI-A-1"),
	}
	raw := []byte{
		0x30, 0x20, 0x10, 0xff,
		0x60, 0x50, 0x40, 0x80,
		0xaa, 0xbb, 0xcc, 0xdd,
	}

	result, details, err := readLinuxKWinScreenshot(bytes.NewReader(raw), metadata)
	if err != nil {
		t.Fatalf("read KWin screenshot: %v", err)
	}
	if details.width != 2 || details.height != 1 || details.stride != 12 || details.scale != 1.5 || details.screen != "HDMI-A-1" {
		t.Fatalf("unexpected screenshot details: %+v", details)
	}
	if got := color.RGBAModel.Convert(result.At(0, 0)).(color.RGBA); got != (color.RGBA{R: 0x10, G: 0x20, B: 0x30, A: 0xff}) {
		t.Fatalf("unexpected first pixel: %#v", got)
	}
	if got := color.RGBAModel.Convert(result.At(1, 0)).(color.RGBA); got != (color.RGBA{R: 0x40, G: 0x50, B: 0x60, A: 0x80}) {
		t.Fatalf("unexpected second pixel: %#v", got)
	}
}

func TestDecodeLinuxKWinRawImageRejectsUnsupportedFormat(t *testing.T) {
	if _, err := decodeLinuxKWinRawImage(make([]byte, 4), 1, 1, 4, 999); err == nil {
		t.Fatal("expected unsupported QImage format error")
	}
}

func TestReadLinuxKWinScreenshotRejectsInvalidStride(t *testing.T) {
	metadata := map[string]dbus.Variant{
		"type":   dbus.MakeVariant("raw"),
		"width":  dbus.MakeVariant(uint32(2)),
		"height": dbus.MakeVariant(uint32(1)),
		"stride": dbus.MakeVariant(uint32(4)),
		"format": dbus.MakeVariant(uint32(linuxKWinImageFormatARGB32Premul)),
	}
	if _, _, err := readLinuxKWinScreenshot(bytes.NewReader(make([]byte, 4)), metadata); err == nil {
		t.Fatal("expected invalid stride error")
	}
}
