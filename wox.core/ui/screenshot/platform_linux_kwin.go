//go:build linux

package screenshot

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"image"
	"io"
	"math"
	"os"
	"strings"
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
	linuxKWinBusName                 = "org.kde.KWin"
	linuxKWinObjectPath              = dbus.ObjectPath("/KWin")
	linuxKWinInterface               = "org.kde.KWin"
	linuxKWinImageFormatRGB32        = 4
	linuxKWinImageFormatARGB32       = 5
	linuxKWinImageFormatARGB32Premul = 6
	linuxKWinImageFormatRGBX8888     = 16
	linuxKWinImageFormatRGBA8888     = 17
	linuxKWinImageFormatRGBA8888Prem = 18
)

// linuxKWinDesktopCapture keeps one compositor output stable across scrolling recaptures.
type linuxKWinDesktopCapture struct {
	mu     sync.Mutex
	conn   *dbus.Conn
	screen linuxKWinDisplayGeometry
	closed bool
}

type linuxKWinDisplayGeometry struct {
	Name        string
	Logical     utilscreen.Rect
	PixelWidth  int
	PixelHeight int
}

// newLinuxKWinDesktopCapture fixes the output at the compositor's active screen before capture begins.
func newLinuxKWinDesktopCapture() (*linuxKWinDesktopCapture, error) {
	screen, err := linuxKWinActiveScreenIdentity()
	if err != nil {
		return nil, err
	}
	util.GetLogger().Info(util.NewTraceContext(), fmt.Sprintf(
		"[screenshot] active Plasma output source=kwin name=%s logical=%d,%d %dx%d pixels=%dx%d",
		screen.Name,
		screen.Logical.X,
		screen.Logical.Y,
		screen.Logical.Width,
		screen.Logical.Height,
		screen.PixelWidth,
		screen.PixelHeight,
	))
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("connect to KWin screenshot service: %w", err)
	}
	if !conn.SupportsUnixFDs() {
		conn.Close()
		return nil, errors.New("KWin screenshot service requires D-Bus Unix FD passing")
	}
	return &linuxKWinDesktopCapture{conn: conn, screen: screen}, nil
}

// linuxKWinActiveScreenIdentity resolves KWin's active output to logical and native-pixel geometry.
func linuxKWinActiveScreenIdentity() (linuxKWinDisplayGeometry, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return linuxKWinDisplayGeometry{}, err
	}
	defer conn.Close()
	object := conn.Object(linuxKWinBusName, linuxKWinObjectPath)
	var activeName string
	if err := object.Call(linuxKWinInterface+".activeOutputName", 0).Store(&activeName); err != nil {
		return linuxKWinDisplayGeometry{}, fmt.Errorf("query KWin active output: %w", err)
	}
	var support string
	if err := object.Call(linuxKWinInterface+".supportInformation", 0).Store(&support); err != nil {
		return linuxKWinDisplayGeometry{}, fmt.Errorf("query KWin output geometry: %w", err)
	}
	for _, display := range parseLinuxKWinDisplayGeometries(support) {
		if display.Name == activeName {
			return display, nil
		}
	}
	return linuxKWinDisplayGeometry{}, fmt.Errorf("KWin active output %q has no geometry", activeName)
}

// parseLinuxKWinDisplayGeometries reads KWin's per-output logical geometry and fractional scale diagnostics.
func parseLinuxKWinDisplayGeometries(support string) []linuxKWinDisplayGeometry {
	var displays []linuxKWinDisplayGeometry
	var name string
	var logical utilscreen.Rect
	var scale float64
	inScreen := false
	flush := func() {
		if name == "" || logical.Width <= 0 || logical.Height <= 0 || scale <= 0 {
			return
		}
		displays = append(displays, linuxKWinDisplayGeometry{
			Name: name, Logical: logical,
			PixelWidth: int(math.Round(float64(logical.Width) * scale)), PixelHeight: int(math.Round(float64(logical.Height) * scale)),
		})
	}
	scanner := bufio.NewScanner(strings.NewReader(support))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "Screen ") && strings.HasSuffix(line, ":") {
			flush()
			inScreen = true
			name = ""
			logical = utilscreen.Rect{}
			scale = 0
			continue
		}
		if !inScreen {
			continue
		}
		if name == "" && strings.HasPrefix(line, "Name: ") {
			name = strings.TrimSpace(strings.TrimPrefix(line, "Name: "))
			continue
		}
		if strings.HasPrefix(line, "Geometry: ") {
			_, _ = fmt.Sscanf(strings.TrimPrefix(line, "Geometry: "), "%d,%d,%dx%d", &logical.X, &logical.Y, &logical.Width, &logical.Height)
			continue
		}
		if strings.HasPrefix(line, "Scale: ") {
			_, _ = fmt.Sscanf(strings.TrimPrefix(line, "Scale: "), "%f", &scale)
			flush()
			inScreen = false
			name = ""
			logical = utilscreen.Rect{}
			scale = 0
		}
	}
	return displays
}

// capture requests native-resolution pixels for the fixed output and decodes KWin's raw QImage buffer.
func (capture *linuxKWinDesktopCapture) capture() (image.Image, error) {
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

	options := map[string]dbus.Variant{
		"native-resolution":   dbus.MakeVariant(true),
		"hide-caller-windows": dbus.MakeVariant(true),
	}
	callContext, cancel := context.WithTimeout(context.Background(), linuxKWinScreenshotCallTimeout)
	call := capture.conn.Object(linuxKWinScreenshotBusName, linuxKWinScreenshotObjectPath).CallWithContext(
		callContext,
		linuxKWinScreenshotInterface+".CaptureScreen",
		0,
		capture.screen.Name,
		options,
		dbus.UnixFD(writer.Fd()),
	)
	cancel()
	if closeErr := writer.Close(); closeErr != nil && call.Err == nil {
		return nil, fmt.Errorf("close KWin screenshot pipe writer: %w", closeErr)
	}
	if call.Err != nil {
		return nil, fmt.Errorf("capture KWin screen %q: %w", capture.screen.Name, call.Err)
	}

	var metadata map[string]dbus.Variant
	if err := call.Store(&metadata); err != nil {
		return nil, fmt.Errorf("decode KWin screenshot metadata: %w", err)
	}
	source, details, err := readLinuxKWinScreenshot(reader, metadata)
	if err != nil {
		return nil, err
	}
	if details.screen != "" && details.screen != capture.screen.Name {
		return nil, fmt.Errorf("KWin captured screen %q instead of requested screen %q", details.screen, capture.screen.Name)
	}
	util.GetLogger().Info(util.NewTraceContext(), fmt.Sprintf(
		"[screenshot] captured Plasma desktop source=kwin-screenshot2 screen=%s logical=%d,%d %dx%d pixels=%dx%d stride=%d format=%d scale=%.2f",
		capture.screen.Name,
		capture.screen.Logical.X,
		capture.screen.Logical.Y,
		capture.screen.Logical.Width,
		capture.screen.Logical.Height,
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
