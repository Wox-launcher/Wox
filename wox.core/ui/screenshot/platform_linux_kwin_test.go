//go:build linux

package screenshot

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"testing"
	utilscreen "wox/util/screen"

	"github.com/godbus/dbus/v5"
)

// TestLinuxKWinCaptureOutputs keeps all native rasters and each recapture callback bound to the correct output.
func TestLinuxKWinCaptureOutputs(t *testing.T) {
	screens := []utilscreen.KWinDisplay{
		{Name: "DP-1", Logical: utilscreen.Rect{X: 0, Y: 80, Width: 192, Height: 108}, PixelWidth: 192, PixelHeight: 108},
		{Name: "HDMI-A-1", Logical: utilscreen.Rect{X: -128, Y: 0, Width: 128, Height: 80}, PixelWidth: 192, PixelHeight: 120},
	}
	var calls []string
	outputs, err := captureLinuxKWinOutputs(screens, func(screen utilscreen.KWinDisplay) (image.Image, error) {
		calls = append(calls, screen.Name)
		return image.NewRGBA(image.Rect(0, 0, screen.PixelWidth, screen.PixelHeight)), nil
	})
	if err != nil || len(outputs) != 2 || len(calls) != 2 || calls[0] != "DP-1" || calls[1] != "HDMI-A-1" {
		t.Fatalf("initial outputs=%+v calls=%v error=%v", outputs, calls, err)
	}
	if outputs[1].bounds != (Rect{X: -128, Width: 128, Height: 80}) || outputs[1].source.Bounds() != image.Rect(0, 0, 192, 120) {
		t.Fatalf("secondary logical/native geometry changed: %+v", outputs[1])
	}
	for index, output := range outputs {
		if _, err := output.capture(); err != nil || calls[len(calls)-1] != screens[index].Name {
			t.Fatalf("output %d recaptured %v, %v", index, calls, err)
		}
	}
	want := errors.New("output disconnected")
	outputs, err = captureLinuxKWinOutputs(screens, func(screen utilscreen.KWinDisplay) (image.Image, error) {
		if screen.Name == "HDMI-A-1" {
			return nil, want
		}
		return image.NewRGBA(image.Rect(0, 0, screen.PixelWidth, screen.PixelHeight)), nil
	})
	if !errors.Is(err, want) || outputs != nil {
		t.Fatalf("failed output left a partial selector: outputs=%v error=%v", outputs, err)
	}
}

func TestLinuxKWinCaptureKeepsCallerWindows(t *testing.T) {
	options := linuxKWinCaptureOptions()
	value, ok := options["hide-caller-windows"]
	if !ok {
		t.Fatal("KWin capture options must set hide-caller-windows")
	}
	hidden, ok := value.Value().(bool)
	if !ok || hidden {
		t.Fatalf("hide-caller-windows = %#v, want false so the settings window stays visible", value.Value())
	}
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
