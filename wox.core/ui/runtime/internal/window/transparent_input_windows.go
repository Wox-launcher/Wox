//go:build windows

package window

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"image"
	"syscall"
	"unsafe"

	"github.com/lxn/win"
)

var createInputRegion = syscall.NewLazyDLL("gdi32.dll").NewProc("ExtCreateRegion")

// transparentRegionData describes the painted pixels in physical window coordinates.
// Region rectangles are sorted by scanline, as required by ExtCreateRegion.
func transparentRegionData(pixels *image.RGBA) []byte {
	width, height := pixels.Bounds().Dx(), pixels.Bounds().Dy()
	data := make([]byte, 32, 32+height*16)
	count := uint32(0)
	for y := 0; y < height; y++ {
		for x := 0; x < width; {
			for x < width && pixels.Pix[y*pixels.Stride+x*4+3] == 0 {
				x++
			}
			left := x
			for x < width && pixels.Pix[y*pixels.Stride+x*4+3] != 0 {
				x++
			}
			if left == x {
				continue
			}
			var rect [16]byte
			binary.LittleEndian.PutUint32(rect[0:4], uint32(left))
			binary.LittleEndian.PutUint32(rect[4:8], uint32(y))
			binary.LittleEndian.PutUint32(rect[8:12], uint32(x))
			binary.LittleEndian.PutUint32(rect[12:16], uint32(y+1))
			data = append(data, rect[:]...)
			count++
		}
	}
	if count == 0 {
		// A completely empty native region cannot receive WM_PAINT to advance an entrance animation.
		data = append(data, []byte{0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 0, 0, 1, 0, 0, 0}...)
		count = 1
	}
	binary.LittleEndian.PutUint32(data[0:4], 32)
	binary.LittleEndian.PutUint32(data[4:8], 1)
	binary.LittleEndian.PutUint32(data[8:12], count)
	binary.LittleEndian.PutUint32(data[12:16], count*16)
	binary.LittleEndian.PutUint32(data[24:28], uint32(width))
	binary.LittleEndian.PutUint32(data[28:32], uint32(height))
	return data
}

// updateTransparentInputRegion uses the portable scene to keep the HWND's input region in sync with visible content.
func (w *platformWindow) updateTransparentInputRegion(list *DisplayList, size PixelSize, scale float32) error {
	if size.Width <= 0 || size.Height <= 0 || int64(size.Width)*int64(size.Height) > 4_194_304 {
		return fmt.Errorf("unsupported transparent input surface size %dx%d", size.Width, size.Height)
	}
	if w.inputMask == nil || w.inputMask.pixels.Bounds().Dx() != size.Width || w.inputMask.pixels.Bounds().Dy() != size.Height {
		var err error
		w.inputMask, err = NewSoftwareRenderer(size.Width, size.Height)
		if err != nil {
			return err
		}
		w.inputRegionValid = false
	}
	physical := *list
	physical.commands = append([]displayCommand(nil), list.commands...)
	physical.floatingMaterials = append([]Rect(nil), list.floatingMaterials...)
	physical.inputRegions = append([]displayClip(nil), list.inputRegions...)
	physical.TransformScene(scale, Point{})
	if err := w.inputMask.Render(&physical); err != nil {
		return err
	}
	for _, region := range physical.inputRegions {
		if len(region.points) == 0 {
			w.inputMask.fill(region.rect, Color{A: 255}, nil)
		} else {
			bounds := Rect{Width: float32(size.Width), Height: float32(size.Height)}
			w.inputMask.fillConvexPolygon(region.points, region.rect, Color{A: 255}, bounds, nil)
		}
	}
	data := transparentRegionData(w.inputMask.pixels)
	digest := sha256.Sum256(data)
	if w.inputRegionValid && w.inputRegionHash == digest {
		return nil
	}
	region, _, err := createInputRegion.Call(0, uintptr(len(data)), uintptr(unsafe.Pointer(&data[0])))
	if region == 0 {
		return fmt.Errorf("create native input region: %w", err)
	}
	previous, valid := w.inputRegionHash, w.inputRegionValid
	w.inputRegionHash, w.inputRegionValid = digest, true
	result, _, err := cornerSetWindowRgn.Call(uintptr(w.hwnd), region, 1)
	if result == 0 {
		w.inputRegionHash, w.inputRegionValid = previous, valid
		win.DeleteObject(win.HGDIOBJ(region))
		return fmt.Errorf("set native input region: %w", err)
	}
	return nil
}
