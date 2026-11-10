//go:build linux

package screenshot

import (
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"os"
	"sync"
	"time"
	"wox/util"
	utilscreen "wox/util/screen"

	"github.com/godbus/dbus/v5"
)

const (
	linuxKWinScreenshotBusName       = "org.kde.KWin.ScreenShot2"
	linuxKWinScreenshotObjectPath    = dbus.ObjectPath("/org/kde/KWin/ScreenShot2")
	linuxKWinScreenshotInterface     = "org.kde.KWin.ScreenShot2"
	linuxKWinScreenshotCallTimeout   = 4 * time.Second
	linuxKWinImageFormatRGB32        = 4
	linuxKWinImageFormatARGB32       = 5
	linuxKWinImageFormatARGB32Premul = 6
	linuxKWinImageFormatRGBX8888     = 16
	linuxKWinImageFormatRGBA8888     = 17
	linuxKWinImageFormatRGBA8888Prem = 18
)

// linuxKWinDesktopCapture freezes all output identities before any selection overlay appears.
type linuxKWinDesktopCapture struct {
	mu      sync.Mutex
	conn    *dbus.Conn
	screen  utilscreen.KWinDisplay
	screens []utilscreen.KWinDisplay
	closed  bool
}

func init() {
	registerLinuxWaylandCaptureBackend(linuxWaylandCaptureBackend{
		name:     "plasma-kwin",
		priority: 100,
		matches:  util.IsKDEWayland,
		open: func() (linuxDesktopCapture, error) {
			return newLinuxKWinDesktopCapture()
		},
	})
}

// newLinuxKWinDesktopCapture discovers every output without relying on the launcher's current screen.
func newLinuxKWinDesktopCapture() (*linuxKWinDesktopCapture, error) {
	screens, err := utilscreen.GetKWinDisplays()
	if err != nil {
		return nil, err
	}
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("connect to KWin screenshot service: %w", err)
	}
	if !conn.SupportsUnixFDs() {
		conn.Close()
		return nil, errors.New("KWin screenshot service requires D-Bus Unix FD passing")
	}
	return &linuxKWinDesktopCapture{conn: conn, screen: screens[0], screens: screens}, nil
}

// captureOutputs snapshots every monitor before mapping overlays and keeps recapture bound to its output.
func (capture *linuxKWinDesktopCapture) captureOutputs() ([]linuxDesktopOutput, error) {
	return captureLinuxKWinOutputs(capture.screens, capture.captureScreen)
}

// captureLinuxKWinOutputs separates the frozen output layout from the per-output native capture operation.
func captureLinuxKWinOutputs(screens []utilscreen.KWinDisplay, captureScreen func(utilscreen.KWinDisplay) (image.Image, error)) ([]linuxDesktopOutput, error) {
	outputs := make([]linuxDesktopOutput, 0, len(screens))
	for _, screen := range screens {
		source, err := captureScreen(screen)
		if err != nil {
			return nil, err
		}
		outputs = append(outputs, linuxDesktopOutput{
			source:  source,
			bounds:  Rect{X: float32(screen.Logical.X), Y: float32(screen.Logical.Y), Width: float32(screen.Logical.Width), Height: float32(screen.Logical.Height)},
			capture: func() (image.Image, error) { return captureScreen(screen) },
		})
	}
	return outputs, nil
}

// linuxKWinCaptureOptions keeps launcher hiding in Go. KWin's hide-caller-windows
// flag is process-wide and would also remove the independent settings window.
func linuxKWinCaptureOptions() map[string]dbus.Variant {
	return map[string]dbus.Variant{
		"native-resolution":   dbus.MakeVariant(true),
		"hide-caller-windows": dbus.MakeVariant(false),
	}
}

// capture requests native-resolution pixels for the fixed output and decodes KWin's raw QImage buffer.
func (capture *linuxKWinDesktopCapture) capture() (image.Image, error) {
	return capture.captureScreen(capture.screen)
}

// captureScreen reads one output at native resolution, including fractional display scales.
func (capture *linuxKWinDesktopCapture) captureScreen(screen utilscreen.KWinDisplay) (image.Image, error) {
	capture.mu.Lock()
	defer capture.mu.Unlock()
	if capture.closed || capture.conn == nil {
		return nil, errors.New("KWin screenshot capture is closed")
	}

	reader, writer, err := os.Pipe()
	if err != nil {
		return nil, fmt.Errorf("create KWin screenshot pipe: %w", err)
	}
	defer reader.Close()

	options := linuxKWinCaptureOptions()
	callContext, cancel := context.WithTimeout(context.Background(), linuxKWinScreenshotCallTimeout)
	call := capture.conn.Object(linuxKWinScreenshotBusName, linuxKWinScreenshotObjectPath).CallWithContext(
		callContext,
		linuxKWinScreenshotInterface+".CaptureScreen",
		0,
		screen.Name,
		options,
		dbus.UnixFD(writer.Fd()),
	)
	cancel()
	if closeErr := writer.Close(); closeErr != nil && call.Err == nil {
		return nil, fmt.Errorf("close KWin screenshot pipe writer: %w", closeErr)
	}
	if call.Err != nil {
		return nil, fmt.Errorf("capture KWin screen %q: %w", screen.Name, call.Err)
	}

	var metadata map[string]dbus.Variant
	if err := call.Store(&metadata); err != nil {
		return nil, fmt.Errorf("decode KWin screenshot metadata: %w", err)
	}
	source, details, err := readLinuxKWinScreenshot(reader, metadata)
	if err != nil {
		return nil, err
	}
	if details.screen != "" && details.screen != screen.Name {
		return nil, fmt.Errorf("KWin captured screen %q instead of requested screen %q", details.screen, screen.Name)
	}
	util.GetLogger().Info(util.NewTraceContext(), fmt.Sprintf(
		"[screenshot] captured Plasma desktop source=kwin-screenshot2 screen=%s logical=%d,%d %dx%d pixels=%dx%d stride=%d format=%d scale=%.2f",
		screen.Name,
		screen.Logical.X,
		screen.Logical.Y,
		screen.Logical.Width,
		screen.Logical.Height,
		details.width,
		details.height,
		details.stride,
		details.format,
		details.scale,
	))
	return source, nil
}

func (capture *linuxKWinDesktopCapture) close() {
	if capture == nil {
		return
	}
	capture.mu.Lock()
	defer capture.mu.Unlock()
	if capture.closed {
		return
	}
	capture.closed = true
	if capture.conn != nil {
		capture.conn.Close()
		capture.conn = nil
	}
}

func (capture *linuxKWinDesktopCapture) logicalBounds() Rect {
	return Rect{
		X:      float32(capture.screen.Logical.X),
		Y:      float32(capture.screen.Logical.Y),
		Width:  float32(capture.screen.Logical.Width),
		Height: float32(capture.screen.Logical.Height),
	}
}

type linuxKWinScreenshotDetails struct {
	width  int
	height int
	stride int
	format uint32
	scale  float64
	screen string
}

// readLinuxKWinScreenshot validates the D-Bus metadata before allocating and reading the compositor buffer.
func readLinuxKWinScreenshot(reader io.Reader, metadata map[string]dbus.Variant) (image.Image, linuxKWinScreenshotDetails, error) {
	if imageType, err := linuxKWinScreenshotString(metadata, "type"); err != nil || imageType != "raw" {
		if err != nil {
			return nil, linuxKWinScreenshotDetails{}, err
		}
		return nil, linuxKWinScreenshotDetails{}, fmt.Errorf("unsupported KWin screenshot type %q", imageType)
	}
	width, err := linuxKWinScreenshotUint32(metadata, "width")
	if err != nil {
		return nil, linuxKWinScreenshotDetails{}, err
	}
	height, err := linuxKWinScreenshotUint32(metadata, "height")
	if err != nil {
		return nil, linuxKWinScreenshotDetails{}, err
	}
	stride, err := linuxKWinScreenshotUint32(metadata, "stride")
	if err != nil {
		return nil, linuxKWinScreenshotDetails{}, err
	}
	format, err := linuxKWinScreenshotUint32(metadata, "format")
	if err != nil {
		return nil, linuxKWinScreenshotDetails{}, err
	}
	if width == 0 || height == 0 || uint64(stride) < uint64(width)*4 || uint64(stride)*uint64(height) > uint64(^uint(0)>>1) {
		return nil, linuxKWinScreenshotDetails{}, fmt.Errorf("invalid KWin screenshot geometry width=%d height=%d stride=%d", width, height, stride)
	}

	raw := make([]byte, int(uint64(stride)*uint64(height)))
	if _, err := io.ReadFull(reader, raw); err != nil {
		return nil, linuxKWinScreenshotDetails{}, fmt.Errorf("read KWin screenshot pixels: %w", err)
	}
	source, err := decodeLinuxKWinRawImage(raw, int(width), int(height), int(stride), format)
	if err != nil {
		return nil, linuxKWinScreenshotDetails{}, err
	}
	details := linuxKWinScreenshotDetails{
		width: int(width), height: int(height), stride: int(stride), format: format,
	}
	if value, ok := metadata["scale"]; ok {
		details.scale, _ = value.Value().(float64)
	}
	if value, ok := metadata["screen"]; ok {
		details.screen, _ = value.Value().(string)
	}
	return source, details, nil
}

func linuxKWinScreenshotUint32(metadata map[string]dbus.Variant, key string) (uint32, error) {
	value, ok := metadata[key]
	if !ok {
		return 0, fmt.Errorf("KWin screenshot metadata is missing %q", key)
	}
	number, ok := value.Value().(uint32)
	if !ok {
		return 0, fmt.Errorf("KWin screenshot metadata %q has type %T", key, value.Value())
	}
	return number, nil
}

func linuxKWinScreenshotString(metadata map[string]dbus.Variant, key string) (string, error) {
	value, ok := metadata[key]
	if !ok {
		return "", fmt.Errorf("KWin screenshot metadata is missing %q", key)
	}
	text, ok := value.Value().(string)
	if !ok {
		return "", fmt.Errorf("KWin screenshot metadata %q has type %T", key, value.Value())
	}
	return text, nil
}

// decodeLinuxKWinRawImage converts the byte layouts used by QImage into Go image layouts.
func decodeLinuxKWinRawImage(raw []byte, width int, height int, stride int, format uint32) (image.Image, error) {
	if width <= 0 || height <= 0 || stride < width*4 || len(raw) < stride*height {
		return nil, errors.New("invalid KWin raw screenshot buffer")
	}
	redIndex := 0
	blueIndex := 2
	alphaIndex := 3
	premultiplied := false
	switch format {
	case linuxKWinImageFormatRGB32:
		redIndex, blueIndex, alphaIndex, premultiplied = 2, 0, -1, true
	case linuxKWinImageFormatARGB32:
		redIndex, blueIndex = 2, 0
	case linuxKWinImageFormatARGB32Premul:
		redIndex, blueIndex, premultiplied = 2, 0, true
	case linuxKWinImageFormatRGBX8888:
		alphaIndex = -1
	case linuxKWinImageFormatRGBA8888:
	case linuxKWinImageFormatRGBA8888Prem:
		premultiplied = true
	default:
		return nil, fmt.Errorf("unsupported KWin QImage format %d", format)
	}

	var pixels []byte
	var destinationStride int
	var result image.Image
	if premultiplied {
		destination := image.NewRGBA(image.Rect(0, 0, width, height))
		pixels, destinationStride, result = destination.Pix, destination.Stride, destination
	} else {
		destination := image.NewNRGBA(image.Rect(0, 0, width, height))
		pixels, destinationStride, result = destination.Pix, destination.Stride, destination
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			sourceOffset := y*stride + x*4
			destinationOffset := y*destinationStride + x*4
			pixels[destinationOffset] = raw[sourceOffset+redIndex]
			pixels[destinationOffset+1] = raw[sourceOffset+1]
			pixels[destinationOffset+2] = raw[sourceOffset+blueIndex]
			pixels[destinationOffset+3] = 0xff
			if alphaIndex >= 0 {
				pixels[destinationOffset+3] = raw[sourceOffset+alphaIndex]
			}
		}
	}
	return result, nil
}
