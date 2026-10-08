package clipboard

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	"image/png"
	"os"
	"sync"
)

type clipboardImage struct {
	width         int
	height        int
	stride        int
	pixels        []byte
	png           []byte
	premultiplied bool
}

// PreparedImage owns clipboard formats independently of the window that dispatches publication.
// Publish and Close share a lock so teardown cannot free native handles during an in-flight commit.
type PreparedImage struct {
	mu      sync.Mutex
	payload *clipboardImage
	native  *preparedImageNative
	size    image.Point
}

// PrepareImage reuses PNG bytes and packed pixels. Borrowed inputs stay immutable until publication returns.
func PrepareImage(source image.Image, encodedPNG []byte) (*PreparedImage, error) {
	payload, err := newClipboardImage(source, encodedPNG)
	if err != nil {
		return nil, err
	}
	return prepareImagePayload(payload)
}

// PrepareImageFile preserves original PNG bytes instead of encoding the decoded raster again.
func PrepareImageFile(path string) (*PreparedImage, error) {
	if path == "" {
		return nil, errors.New("clipboard image file path is empty")
	}
	payload, err := loadClipboardImage(path)
	if err != nil {
		return nil, err
	}
	return prepareImagePayload(payload)
}

// prepareImagePayload creates native formats before UI-thread dispatch or clipboard locking.
func prepareImagePayload(payload *clipboardImage) (*PreparedImage, error) {
	if payload == nil {
		return nil, errors.New("clipboard image is empty")
	}
	native, err := prepareNativeImage(payload)
	if err != nil {
		return nil, err
	}
	return &PreparedImage{payload: payload, native: native, size: image.Pt(payload.width, payload.height)}, nil
}

func (prepared *PreparedImage) Size() image.Point { return prepared.size }

// Publish transfers formats on the owner's UI thread: HWND on Windows, GdkDisplay on Linux, system pasteboard on macOS.
// High-level Write reaches clipboard history after its settle window. This entry does not open that window.
func (prepared *PreparedImage) Publish(owner uintptr) error {
	if prepared == nil {
		return errors.New("clipboard image is empty")
	}
	prepared.mu.Lock()
	defer prepared.mu.Unlock()
	if prepared.payload == nil {
		return errors.New("clipboard image is closed")
	}
	return publishNativeImage(owner, prepared.payload, prepared.native)
}

// Close releases untransferred native handles and borrowed Go buffers, including repeated teardown.
func (prepared *PreparedImage) Close() {
	if prepared == nil {
		return
	}
	prepared.mu.Lock()
	defer prepared.mu.Unlock()
	if prepared.native != nil {
		releaseNativeImage(prepared.native)
	}
	prepared.native, prepared.payload = nil, nil
}

// PublishText commits text on the owner's UI thread without opening the high-level settle window.
func PublishText(owner uintptr, text string) error { return publishNativeText(owner, text) }

// Flush materializes promised formats before exit; the caller supplies only generic UI-thread dispatch.
func Flush(dispatch func(func()) error) error {
	if !clipboardUsesEncodedPNG {
		return nil
	}
	var flushErr error
	if err := dispatch(func() { flushErr = flushNativeImage() }); err != nil {
		return err
	}
	return flushErr
}

// loadClipboardImage preserves PNG bytes when already available and otherwise publishes pixels only.
func loadClipboardImage(filePath string) (*clipboardImage, error) {
	encoded, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read clipboard image: %w", err)
	}
	source, format, err := image.Decode(bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("decode clipboard image: %w", err)
	}
	if format != "png" {
		encoded = nil
	}
	return newClipboardImage(source, encoded)
}

// newClipboardImage prepares encoded PNG or packed pixels with explicit alpha semantics for the platform clipboard.
func newClipboardImage(source image.Image, encodedPNG []byte) (*clipboardImage, error) {
	if source == nil {
		return nil, errors.New("clipboard image is empty")
	}
	bounds := source.Bounds()
	if bounds.Empty() || bounds.Dx() > 16384 || bounds.Dy() > 16384 {
		return nil, fmt.Errorf("clipboard image dimensions are invalid: %dx%d", bounds.Dx(), bounds.Dy())
	}
	if clipboardUsesEncodedPNG && len(encodedPNG) > 0 {
		// Encoded native publication needs no additional normalized full-size raster.
		return &clipboardImage{width: bounds.Dx(), height: bounds.Dy(), png: encodedPNG}, nil
	}
	clipboard := &clipboardImage{width: bounds.Dx(), height: bounds.Dy()}
	if clipboardAcceptsPackedRGBA {
		// Windows prepares its native DIB directly from these immutable rows before publication.
		// Avoid an image/draw conversion and a second full-size raster on the confirmation path.
		switch raster := source.(type) {
		case *image.RGBA:
			clipboard.pixels, clipboard.stride, clipboard.premultiplied = raster.Pix, raster.Stride, true
		case *image.NRGBA:
			clipboard.pixels, clipboard.stride = raster.Pix, raster.Stride
		}
	}
	if len(clipboard.pixels) == 0 {
		normalized := image.NewNRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
		draw.Draw(normalized, normalized.Bounds(), source, bounds.Min, draw.Src)
		clipboard.pixels, clipboard.stride = normalized.Pix, normalized.Stride
		source = normalized
	}
	// Windows' compatibility DIB format is often read without alpha. Publish PNG too for transparent captures.
	if len(encodedPNG) == 0 && !source.(interface{ Opaque() bool }).Opaque() {
		var encoded bytes.Buffer
		if err := png.Encode(&encoded, source); err != nil {
			return nil, fmt.Errorf("encode transparent clipboard image: %w", err)
		}
		encodedPNG = encoded.Bytes()
	}
	clipboard.png = encodedPNG
	return clipboard, nil
}
