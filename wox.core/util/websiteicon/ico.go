package websiteicon

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"sort"
)

var pngSignature = []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}

type icoEntry struct {
	width    int
	height   int
	bitCount int
	size     int
	offset   int
}

// isICO reports whether data starts with a valid Windows ICO directory header.
func isICO(data []byte) bool {
	if len(data) < 6 {
		return false
	}
	reserved := binary.LittleEndian.Uint16(data[0:2])
	imgType := binary.LittleEndian.Uint16(data[2:4])
	count := int(binary.LittleEndian.Uint16(data[4:6]))
	return reserved == 0 && imgType == 1 && count > 0 && len(data) >= 6+count*16
}

// decodeICO decodes the highest-resolution valid icon image from an ICO container.
func decodeICO(data []byte) (image.Image, error) {
	if !isICO(data) {
		return nil, errors.New("invalid ICO header")
	}

	count := int(binary.LittleEndian.Uint16(data[4:6]))
	entries := make([]icoEntry, 0, count)
	for i := 0; i < count; i++ {
		off := 6 + i*16
		w := int(data[off])
		if w == 0 {
			w = 256
		}
		h := int(data[off+1])
		if h == 0 {
			h = 256
		}
		bitCount := int(binary.LittleEndian.Uint16(data[off+6 : off+8]))
		size := int(binary.LittleEndian.Uint32(data[off+8 : off+12]))
		imgOffset := int(binary.LittleEndian.Uint32(data[off+12 : off+16]))
		if size <= 0 || imgOffset < 6+count*16 || uint64(imgOffset)+uint64(size) > uint64(len(data)) {
			continue
		}
		entries = append(entries, icoEntry{
			width:    w,
			height:   h,
			bitCount: bitCount,
			size:     size,
			offset:   imgOffset,
		})
	}
	if len(entries) == 0 {
		return nil, errors.New("ICO contains no valid image entries")
	}

	sort.SliceStable(entries, func(i, j int) bool {
		areaI := entries[i].width * entries[i].height
		areaJ := entries[j].width * entries[j].height
		if areaI != areaJ {
			return areaI > areaJ
		}
		return entries[i].bitCount > entries[j].bitCount
	})

	var lastErr error
	for _, entry := range entries {
		payload := data[entry.offset : entry.offset+entry.size]
		img, err := decodeICOEntry(payload)
		if err == nil {
			return img, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("decode ICO entries: %w", lastErr)
}

func decodeICOEntry(payload []byte) (image.Image, error) {
	if bytes.HasPrefix(payload, pngSignature) {
		return png.Decode(bytes.NewReader(payload))
	}
	return decodeICOBitmap(payload)
}

func decodeICOBitmap(payload []byte) (image.Image, error) {
	if len(payload) < 40 {
		return nil, errors.New("ICO DIB header too short")
	}
	headerSize := int(binary.LittleEndian.Uint32(payload[0:4]))
	if headerSize < 40 || headerSize > len(payload) {
		return nil, fmt.Errorf("unsupported ICO DIB header size: %d", headerSize)
	}

	w := int(int32(binary.LittleEndian.Uint32(payload[4:8])))
	rawH := int(int32(binary.LittleEndian.Uint32(payload[8:12])))
	planes := binary.LittleEndian.Uint16(payload[12:14])
	bpp := int(binary.LittleEndian.Uint16(payload[14:16]))
	compression := binary.LittleEndian.Uint32(payload[16:20])
	colorsUsed := int(binary.LittleEndian.Uint32(payload[32:36]))

	if planes != 1 || compression != 0 {
		return nil, fmt.Errorf("unsupported ICO DIB format: planes=%d compression=%d", planes, compression)
	}
	topDown := rawH < 0
	if rawH < 0 {
		rawH = -rawH
	}
	// ICO DIB height doubles the icon height because XOR pixels are followed by the 1-bit AND mask.
	h := rawH / 2
	if w <= 0 || h <= 0 || w > 4096 || h > 4096 {
		return nil, fmt.Errorf("invalid ICO DIB dimensions: %dx%d", w, h)
	}

	var palette []color.NRGBA
	paletteBytes := 0
	switch bpp {
	case 1, 4, 8:
		numColors := colorsUsed
		if numColors <= 0 || numColors > (1<<bpp) {
			numColors = 1 << bpp
		}
		paletteBytes = numColors * 4
		if headerSize+paletteBytes > len(payload) {
			return nil, errors.New("ICO DIB palette truncated")
		}
		palette = make([]color.NRGBA, numColors)
		for i := 0; i < numColors; i++ {
			off := headerSize + i*4
			palette[i] = color.NRGBA{
				R: payload[off+2],
				G: payload[off+1],
				B: payload[off],
				A: 255,
			}
		}
	case 24, 32:
	default:
		return nil, fmt.Errorf("unsupported ICO DIB bit depth: %d", bpp)
	}

	xorOffset := headerSize + paletteBytes
	xorStride := ((w*bpp + 31) / 32) * 4
	xorSize := xorStride * h
	if xorOffset+xorSize > len(payload) {
		return nil, errors.New("ICO DIB pixel data truncated")
	}
	xorData := payload[xorOffset : xorOffset+xorSize]

	andOffset := xorOffset + xorSize
	andStride := ((w + 31) / 32) * 4
	var andData []byte
	if andOffset+andStride*h <= len(payload) {
		andData = payload[andOffset : andOffset+andStride*h]
	}

	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	hasNonZeroAlpha := false

	for row := 0; row < h; row++ {
		y := h - 1 - row
		if topDown {
			y = row
		}
		srcRow := xorData[row*xorStride : (row+1)*xorStride]
		for x := 0; x < w; x++ {
			var c color.NRGBA
			switch bpp {
			case 32:
				off := x * 4
				c = color.NRGBA{R: srcRow[off+2], G: srcRow[off+1], B: srcRow[off], A: srcRow[off+3]}
				if c.A != 0 {
					hasNonZeroAlpha = true
				}
			case 24:
				off := x * 3
				c = color.NRGBA{R: srcRow[off+2], G: srcRow[off+1], B: srcRow[off], A: 255}
			case 8:
				idx := int(srcRow[x])
				if idx < len(palette) {
					c = palette[idx]
				}
			case 4:
				b := srcRow[x/2]
				var idx int
				if x%2 == 0 {
					idx = int(b >> 4)
				} else {
					idx = int(b & 0x0f)
				}
				if idx < len(palette) {
					c = palette[idx]
				}
			case 1:
				idx := int((srcRow[x/8] >> (7 - uint(x%8))) & 1)
				if idx < len(palette) {
					c = palette[idx]
				}
			}

			if bpp != 32 && andData != nil {
				if andMaskBit(andData, andStride, row, x) {
					c.A = 0
				}
			}
			img.SetNRGBA(x, y, c)
		}
	}

	// Some 32-bit ICO DIBs leave the alpha channel zeroed and rely on the 1-bit AND mask.
	if bpp == 32 && !hasNonZeroAlpha {
		for row := 0; row < h; row++ {
			y := h - 1 - row
			if topDown {
				y = row
			}
			for x := 0; x < w; x++ {
				c := img.NRGBAAt(x, y)
				if andData != nil && andMaskBit(andData, andStride, row, x) {
					c.A = 0
				} else {
					c.A = 255
				}
				img.SetNRGBA(x, y, c)
			}
		}
	}

	return img, nil
}

func andMaskBit(andData []byte, andStride, row, x int) bool {
	b := andData[row*andStride+x/8]
	return (b>>(7-uint(x%8)))&1 != 0
}
