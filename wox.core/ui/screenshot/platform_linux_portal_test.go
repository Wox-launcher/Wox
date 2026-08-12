//go:build linux

package screenshot

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
	utilscreen "wox/util/screen"

	"github.com/godbus/dbus/v5"
)

func TestLinuxScreenshotPortalSelectSourcesOptions(t *testing.T) {
	withoutRestore := linuxScreenshotPortalSelectSourcesOptions("select", "", false, 1)
	if _, ok := withoutRestore["restore_token"]; ok {
		t.Fatal("empty restore_token must not be sent")
	}
	if got := withoutRestore["multiple"].Value(); got != false {
		t.Fatalf("multiple = %#v", got)
	}
	if got := withoutRestore["cursor_mode"].Value(); got != uint32(1) {
		t.Fatalf("cursor_mode = %#v", got)
	}
	if got := withoutRestore["persist_mode"].Value(); got != uint32(linuxPortalPersistUntilRevoked) {
		t.Fatalf("persist_mode = %#v", got)
	}

	withRestore := linuxScreenshotPortalSelectSourcesOptions("select", "saved-token", true, 4)
	if got := withRestore["restore_token"].Value(); got != "saved-token" {
		t.Fatalf("restore_token = %#v", got)
	}
	if got := withRestore["multiple"].Value(); got != true {
		t.Fatalf("multiple = %#v", got)
	}
	if got := withRestore["cursor_mode"].Value(); got != uint32(4) {
		t.Fatalf("cursor_mode = %#v", got)
	}
}

func TestLinuxScreenshotPortalCleanRestoreStore(t *testing.T) {
	single := linuxPortalRestoreEntry{Token: "single-token", Monitors: []linuxPortalMonitor{{ID: "DP-1"}}}
	multiple := linuxPortalRestoreEntry{Token: "multiple-token", Monitors: []linuxPortalMonitor{{ID: "DP-1"}, {ID: "HDMI-A-1"}}}
	legacy := linuxPortalRestoreEntry{Token: "legacy-token"}
	store := linuxPortalRestoreStore{Version: 2, Entries: []linuxPortalRestoreEntry{single, multiple, legacy}}

	cleaned := linuxScreenshotPortalCleanRestoreStore(store)
	if len(cleaned.Entries) != 2 || cleaned.Entries[0].Token != "single-token" || cleaned.Entries[1].Token != "multiple-token" {
		t.Fatalf("multi-source restore store = %#v", cleaned)
	}
}

func TestLinuxScreenshotPortalRestoreTokenStorage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "portal", "restore-token")
	if store, err := loadLinuxScreenshotPortalRestoreStore(path); err != nil || len(store.Entries) != 0 {
		t.Fatalf("load missing token = %#v, %v", store, err)
	}
	entry := linuxPortalRestoreEntry{
		Token: "first-token",
		Monitors: []linuxPortalMonitor{
			{ID: "DP-1", Bounds: Rect{X: 0, Y: 0, Width: 1920, Height: 1080}},
		},
	}
	store := linuxPortalRestoreStore{Version: 2, Entries: []linuxPortalRestoreEntry{entry}}
	if err := saveLinuxScreenshotPortalRestoreStore(path, store); err != nil {
		t.Fatal(err)
	}
	if loaded, err := loadLinuxScreenshotPortalRestoreStore(path); err != nil || len(loaded.Entries) != 1 || loaded.Entries[0].Token != "first-token" || loaded.Entries[0].Monitors[0].ID != "DP-1" {
		t.Fatalf("load token = %#v, %v", loaded, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("restore token permissions = %o", info.Mode().Perm())
	}
	store.Entries[0].Token = "replacement-token"
	if err := saveLinuxScreenshotPortalRestoreStore(path, store); err != nil {
		t.Fatal(err)
	}
	if loaded, err := loadLinuxScreenshotPortalRestoreStore(path); err != nil || loaded.Entries[0].Token != "replacement-token" {
		t.Fatalf("load replacement token = %#v, %v", loaded, err)
	}
	if err := removeLinuxScreenshotPortalRestoreToken(path); err != nil {
		t.Fatal(err)
	}
	if loaded, err := loadLinuxScreenshotPortalRestoreStore(path); err != nil || len(loaded.Entries) != 0 {
		t.Fatalf("load removed token = %#v, %v", loaded, err)
	}
}

func TestLinuxScreenshotPortalRestoreMatchesCurrentMonitor(t *testing.T) {
	entry := linuxPortalRestoreEntry{
		Token: "token",
		Monitors: []linuxPortalMonitor{
			{ID: "DP-1", Bounds: Rect{X: 0, Y: 0, Width: 1920, Height: 1080}},
		},
	}
	if !linuxScreenshotPortalRestoreEntryMatchesScreen(entry, linuxPortalScreenIdentity{Logical: utilscreen.Size{X: 0, Y: 0, Width: 1920, Height: 1080}, PixelWidth: 1920, PixelHeight: 1080}) {
		t.Fatal("restore token should match its saved monitor")
	}
	if linuxScreenshotPortalRestoreEntryMatchesScreen(entry, linuxPortalScreenIdentity{Logical: utilscreen.Size{X: 1920, Y: 0, Width: 1280, Height: 800}, PixelWidth: 1920, PixelHeight: 1200}) {
		t.Fatal("restore token must not match a different monitor")
	}
	scaledEntry := linuxPortalRestoreEntry{
		Token: "scaled-token",
		Monitors: []linuxPortalMonitor{
			{ID: "portal-monitor-39", Bounds: Rect{X: 0, Y: 0, Width: 1920, Height: 1200}},
		},
	}
	if !linuxScreenshotPortalRestoreEntryMatchesScreen(scaledEntry, linuxPortalScreenIdentity{Logical: utilscreen.Size{X: 1920, Y: 0, Width: 1280, Height: 800}, PixelWidth: 1920, PixelHeight: 1200}) {
		t.Fatal("physical XDPH source should match the scaled logical monitor")
	}
	entry.Monitors = append(entry.Monitors, linuxPortalMonitor{ID: "HDMI-A-1", Bounds: Rect{X: 1920, Y: 0, Width: 1280, Height: 800}})
	if !linuxScreenshotPortalRestoreEntryMatchesScreen(entry, linuxPortalScreenIdentity{Logical: utilscreen.Size{X: 1920, Y: 0, Width: 1280, Height: 800}, PixelWidth: 1920, PixelHeight: 1200}) {
		t.Fatal("a restore token covering multiple monitors should be reusable from either monitor")
	}
}

func TestLinuxScreenshotPortalPlainTokenMigrationRequiresFreshMonitorSelection(t *testing.T) {
	path := filepath.Join(t.TempDir(), "restore-token")
	if err := os.WriteFile(path, []byte("legacy-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := loadLinuxScreenshotPortalRestoreStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(store.Entries) != 1 || store.Entries[0].Token != "legacy-token" {
		t.Fatalf("legacy token store = %#v", store)
	}
	if linuxScreenshotPortalRestoreEntryMatchesScreen(store.Entries[0], linuxPortalScreenIdentity{Logical: utilscreen.Size{X: 0, Y: 0, Width: 1920, Height: 1080}, PixelWidth: 1920, PixelHeight: 1080}) {
		t.Fatal("legacy token without monitor metadata must require a fresh selection")
	}
}

func TestLinuxScreenshotPortalKeepsIndependentMonitorTokens(t *testing.T) {
	dp := linuxPortalRestoreEntry{Token: "dp-token", Monitors: []linuxPortalMonitor{{ID: "DP-1", Bounds: Rect{Width: 1920, Height: 1080}}}}
	hdmi := linuxPortalRestoreEntry{Token: "hdmi-token", Monitors: []linuxPortalMonitor{{ID: "HDMI-A-1", Bounds: Rect{X: 1920, Width: 1280, Height: 800}}}}
	store := linuxScreenshotPortalUpsertRestoreEntry(linuxPortalRestoreStore{Version: 2}, dp)
	store = linuxScreenshotPortalUpsertRestoreEntry(store, hdmi)
	if len(store.Entries) != 2 {
		t.Fatalf("restore entry count = %d", len(store.Entries))
	}
	index, selected := linuxScreenshotPortalRestoreEntryForScreen(store, linuxPortalScreenIdentity{Logical: utilscreen.Size{X: 1920, Width: 1280, Height: 800}, PixelWidth: 1920, PixelHeight: 1200})
	if index != 1 || selected.Token != "hdmi-token" {
		t.Fatalf("selected restore entry = %d, %#v", index, selected)
	}
	dp.Token = "dp-token-rotated"
	dp.Monitors[0].ID = "portal-monitor-99"
	dp.Monitors[0].NodeID = 99
	store = linuxScreenshotPortalUpsertRestoreEntry(store, dp)
	if len(store.Entries) != 2 || store.Entries[0].Token != "dp-token-rotated" {
		t.Fatalf("rotated restore store = %#v", store)
	}
}

func TestNormalizeLinuxPortalMonitorUsesPhysicalDisplaySize(t *testing.T) {
	displays := []linuxPortalDisplayGeometry{
		{Logical: utilscreen.Rect{X: 0, Y: 0, Width: 1920, Height: 1080}, PixelWidth: 1920, PixelHeight: 1080},
		{Logical: utilscreen.Rect{X: 1920, Y: 0, Width: 1280, Height: 800}, PixelWidth: 1920, PixelHeight: 1200},
	}
	current := linuxPortalScreenIdentity{Logical: utilscreen.Size{X: 1920, Y: 0, Width: 1280, Height: 800}, PixelWidth: 1920, PixelHeight: 1200}
	if got := linuxScreenshotPortalDisplayIndex(Rect{X: 0, Y: 0, Width: 1920, Height: 1200}, displays, map[int]bool{}, current); got != 1 {
		t.Fatalf("physical portal monitor mapped to display %d, want 1", got)
	}
}

func TestLinuxScreenshotPortalRestoreTokenResult(t *testing.T) {
	results := map[string]dbus.Variant{"restore_token": dbus.MakeVariant("next-token")}
	if got := linuxScreenshotPortalRestoreToken(results); got != "next-token" {
		t.Fatalf("restore token = %q", got)
	}
	if got := linuxScreenshotPortalRestoreToken(map[string]dbus.Variant{}); got != "" {
		t.Fatalf("missing restore token = %q", got)
	}
}

func TestLinuxPortalLiveCapture(t *testing.T) {
	if os.Getenv("WOX_LIVE_PORTAL_SCREENSHOT") != "1" {
		t.Skip("set WOX_LIVE_PORTAL_SCREENSHOT=1 to exercise the desktop portal")
	}
	capture, err := newLinuxWaylandDesktopCapture()
	if err != nil {
		t.Fatal(err)
	}
	defer capture.close()
	captureStartedAt := time.Now()
	frame, err := capture.capture()
	if err != nil {
		t.Fatal(err)
	}
	if outputPath := os.Getenv("WOX_LIVE_PORTAL_SCREENSHOT_OUTPUT"); outputPath != "" {
		file, createErr := os.Create(outputPath)
		if createErr != nil {
			t.Fatal(createErr)
		}
		if encodeErr := png.Encode(file, frame); encodeErr != nil {
			file.Close()
			t.Fatal(encodeErr)
		}
		if closeErr := file.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
	}
	t.Logf("captured portal frame=%v logical=%+v elapsed=%s", frame.Bounds(), capture.logicalBounds(), time.Since(captureStartedAt).Round(time.Millisecond))
}

func TestComposeLinuxPortalFramesNormalizesMixedDPI(t *testing.T) {
	monitors := []linuxPortalMonitor{
		{ID: "left", Bounds: Rect{X: -100, Y: 0, Width: 100, Height: 100}},
		{ID: "main", Bounds: Rect{X: 0, Y: 0, Width: 100, Height: 100}},
	}
	left := image.NewRGBA(image.Rect(0, 0, 100, 100))
	draw.Draw(left, left.Bounds(), image.NewUniform(color.RGBA{R: 255, A: 255}), image.Point{}, draw.Src)
	main := image.NewRGBA(image.Rect(0, 0, 200, 200))
	draw.Draw(main, main.Bounds(), image.NewUniform(color.RGBA{G: 255, A: 255}), image.Point{}, draw.Src)

	composed, err := composeLinuxPortalFrames(monitors, []*image.RGBA{left, main})
	if err != nil {
		t.Fatal(err)
	}
	if composed.Bounds() != image.Rect(0, 0, 400, 200) {
		t.Fatalf("composed bounds = %v, want (0,0)-(400,200)", composed.Bounds())
	}
	if got := color.RGBAModel.Convert(composed.At(100, 100)).(color.RGBA); got.R != 255 || got.G != 0 {
		t.Fatalf("left monitor pixel = %+v", got)
	}
	if got := color.RGBAModel.Convert(composed.At(300, 100)).(color.RGBA); got.R != 0 || got.G != 255 {
		t.Fatalf("main monitor pixel = %+v", got)
	}
}

func TestLinuxPortalMonitorUnionSupportsNegativeOrigins(t *testing.T) {
	monitors := []linuxPortalMonitor{
		{ID: "left", Bounds: Rect{X: -1920, Y: 0, Width: 1920, Height: 1080}},
		{ID: "main", Bounds: Rect{X: 0, Y: -200, Width: 2560, Height: 1440}},
	}
	bounds, err := linuxPortalMonitorUnion(monitors)
	if err != nil {
		t.Fatal(err)
	}
	want := Rect{X: -1920, Y: -200, Width: 4480, Height: 1440}
	if bounds != want {
		t.Fatalf("portal monitor union = %+v, want %+v", bounds, want)
	}
}

func TestParseLinuxPortalMonitors(t *testing.T) {
	results := map[string]dbus.Variant{
		"streams": dbus.MakeVariant([]linuxPortalStream{
			{
				NodeID: 42,
				Properties: map[string]dbus.Variant{
					"source_type": dbus.MakeVariant(uint32(1)),
					"position":    dbus.MakeVariant(linuxPortalIntPair{First: -1280, Second: 120}),
					"size":        dbus.MakeVariant(linuxPortalIntPair{First: 1280, Second: 720}),
					"id":          dbus.MakeVariant("DP-1"),
				},
			},
			{
				NodeID: 43,
				Properties: map[string]dbus.Variant{
					"source_type": dbus.MakeVariant(uint32(2)),
					"position":    dbus.MakeVariant(linuxPortalIntPair{}),
					"size":        dbus.MakeVariant(linuxPortalIntPair{First: 400, Second: 300}),
				},
			},
		}),
	}
	monitors, err := parseLinuxPortalMonitors(results)
	if err != nil {
		t.Fatal(err)
	}
	if len(monitors) != 1 {
		t.Fatalf("portal monitor count = %d, want 1", len(monitors))
	}
	if monitors[0].ID != "DP-1" || monitors[0].Bounds != (Rect{X: -1280, Y: 120, Width: 1280, Height: 720}) {
		t.Fatalf("portal monitor = %+v", monitors[0])
	}
}
