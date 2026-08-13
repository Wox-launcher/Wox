package tray

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"sync/atomic"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

func TestSNIIconPixmapSignature(t *testing.T) {
	variant := dbus.MakeVariant([]sniIconPixmap{{
		Width:  1,
		Height: 1,
		Pixels: []byte{0xff, 0x11, 0x22, 0x33},
	}})
	if got := variant.Signature().String(); got != "a(iiay)" {
		t.Fatalf("IconPixmap signature: got %s, want a(iiay)", got)
	}
}

func TestSNIToolTipSignature(t *testing.T) {
	variant := dbus.MakeVariant(sniToolTip{Title: "Wox"})
	if got := variant.Signature().String(); got != "(sa(iiay)ss)" {
		t.Fatalf("ToolTip signature: got %s, want (sa(iiay)ss)", got)
	}
}

func TestEncodeSNIIconPixelsNetworkARGB32(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.SetNRGBA(0, 0, color.NRGBA{R: 0x11, G: 0x22, B: 0x33, A: 0xff})

	pixels := encodeSNIIconPixels(img)
	want := []byte{0xff, 0x11, 0x22, 0x33}
	if !bytes.Equal(pixels, want) {
		t.Fatalf("ARGB32 pixels: got %v, want %v", pixels, want)
	}
}

func TestBuildSNIIconPixmapsMatchesRequestedPixelSizes(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			source.SetNRGBA(x, y, color.NRGBA{R: 0x40, G: 0x80, B: 0xc0, A: 0xff})
		}
	}

	pixmaps := buildSNIIconPixmaps(source)
	if len(pixmaps) != len(linuxTrayIconPixelSizes) {
		t.Fatalf("pixmap count: got %d, want %d", len(pixmaps), len(linuxTrayIconPixelSizes))
	}

	seen := map[int32]struct{}{}
	for _, pixmap := range pixmaps {
		if pixmap.Width != pixmap.Height {
			t.Fatalf("expected square pixmap, got %dx%d", pixmap.Width, pixmap.Height)
		}
		if int(pixmap.Width)*int(pixmap.Height)*4 != len(pixmap.Pixels) {
			t.Fatalf("pixel buffer length %d does not match %dx%d ARGB32", len(pixmap.Pixels), pixmap.Width, pixmap.Height)
		}
		seen[pixmap.Width] = struct{}{}
	}
	for _, size := range linuxTrayIconPixelSizes {
		if _, ok := seen[int32(size)]; !ok {
			t.Fatalf("missing pixmap size %d", size)
		}
	}
}

func TestDecodeTrayIconReadsPNGBytes(t *testing.T) {
	source := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	source.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, source); err != nil {
		t.Fatal(err)
	}

	decoded := decodeTrayIcon(encoded.Bytes())
	if decoded == nil {
		t.Fatal("expected PNG tray icon to decode")
	}
	if decoded.Bounds().Dx() != 2 || decoded.Bounds().Dy() != 2 {
		t.Fatalf("decoded size: got %dx%d, want 2x2", decoded.Bounds().Dx(), decoded.Bounds().Dy())
	}
}

func TestLinuxTrayMenuLayoutSignatureAndLabels(t *testing.T) {
	items := makeMenuEntries([]MenuItem{
		{Title: "Toggle Wox"},
		{Title: "Quit"},
	})
	layout, ok := buildMenuLayout(items, 0, -1, nil)
	if !ok {
		t.Fatal("expected root menu layout")
	}
	if layout.ID != 0 {
		t.Fatalf("root id: got %d, want 0", layout.ID)
	}
	if got := dbus.MakeVariant(layout).Signature().String(); got != "(ia{sv}av)" {
		t.Fatalf("menu layout signature: got %s, want (ia{sv}av)", got)
	}
	if len(layout.Children) != 2 {
		t.Fatalf("root children: got %d, want 2", len(layout.Children))
	}

	first, ok := layout.Children[0].Value().(menuLayout)
	if !ok {
		t.Fatalf("child type: got %T, want menuLayout", layout.Children[0].Value())
	}
	if first.ID != 1 {
		t.Fatalf("first item id: got %d, want 1", first.ID)
	}
	label, ok := first.Properties["label"]
	if !ok || label.Value() != "Toggle Wox" {
		t.Fatalf("first item label: got %#v", label)
	}
}

func TestLinuxTrayMenuLayoutRecursionDepthZeroOmitsChildren(t *testing.T) {
	items := makeMenuEntries([]MenuItem{{Title: "Quit"}})
	layout, ok := buildMenuLayout(items, 0, 0, nil)
	if !ok {
		t.Fatal("expected root menu layout")
	}
	if len(layout.Children) != 0 {
		t.Fatalf("root children: got %d, want 0", len(layout.Children))
	}
}

func TestLinuxTrayMenuClickedDispatchesCallback(t *testing.T) {
	done := make(chan struct{})
	host := &linuxTray{
		items: makeMenuEntries([]MenuItem{{
			Title: "Quit",
			Callback: func() {
				close(done)
			},
		}}),
	}
	menu := &dbusMenuServer{host: host}
	if err := menu.Event(1, "clicked", dbus.MakeVariant(""), 0); err != nil {
		t.Fatal(err)
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for tray callback")
	}
}

func TestLinuxTrayMenuHoveredDoesNotDispatchCallback(t *testing.T) {
	var called atomic.Bool
	host := &linuxTray{
		items: makeMenuEntries([]MenuItem{{
			Title: "Quit",
			Callback: func() {
				called.Store(true)
			},
		}}),
	}
	menu := &dbusMenuServer{host: host}
	if err := menu.Event(1, "hovered", dbus.MakeVariant(""), 0); err != nil {
		t.Fatal(err)
	}
	if called.Load() {
		t.Fatal("hovered event should not activate the menu item")
	}
}
