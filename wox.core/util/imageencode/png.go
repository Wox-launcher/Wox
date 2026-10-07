// Package imageencode encodes screenshot rasters without searching every PNG filter for every row.
package imageencode

import (
	"bufio"
	"compress/zlib"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/png"
	"io"
)

// PNG uses the lossless Sub filter and fast deflate for packed screenshot pixels.
// The standard encoder scores five filters per row, which delays clipboard and history publication.
// Other image implementations retain the standard encoder's format and color conversion behavior.
func PNG(writer io.Writer, source image.Image) error {
	var pixels []byte
	var stride int
	bytesPerPixel := 4
	premultiplied := false
	switch raster := source.(type) {
	case *image.RGBA:
		pixels, stride, premultiplied = raster.Pix, raster.Stride, true
	case *image.NRGBA:
		pixels, stride = raster.Pix, raster.Stride
	case *image.NRGBA64:
		pixels, stride, bytesPerPixel = raster.Pix, raster.Stride, 8
	default:
		return png.Encode(writer, source)
	}
	bounds := source.Bounds()
	if bounds.Empty() || int64(bounds.Dx()) >= 1<<31 || int64(bounds.Dy()) >= 1<<31 {
		return png.FormatError("invalid image size")
	}
	output := bufio.NewWriter(writer)
	if _, err := output.WriteString("\x89PNG\r\n\x1a\n"); err != nil {
		return err
	}
	var header [13]byte
	binary.BigEndian.PutUint32(header[:4], uint32(bounds.Dx()))
	binary.BigEndian.PutUint32(header[4:8], uint32(bounds.Dy()))
	header[8], header[9] = byte(bytesPerPixel*2), 6 // RGBA, 8 or 16 bits per channel.
	if err := writePNGChunk(output, "IHDR", header[:]); err != nil {
		return err
	}
	chunks := &pngDataWriter{output: output}
	compressed, err := zlib.NewWriterLevel(chunks, zlib.BestSpeed)
	if err != nil {
		return err
	}
	row := make([]byte, bounds.Dx()*bytesPerPixel+1)
	row[0] = 1 // Sub predicts each byte from the same channel of its left neighbor.
	for y := 0; y < bounds.Dy(); y++ {
		filterPNGRow(row[1:], pixels[y*stride:y*stride+len(row)-1], premultiplied, bytesPerPixel)
		if _, err := compressed.Write(row); err != nil {
			_ = compressed.Close()
			return err
		}
	}
	if err := compressed.Close(); err != nil {
		return err
	}
	if err := chunks.flush(); err != nil {
		return err
	}
	if err := writePNGChunk(output, "IEND", nil); err != nil {
		return err
	}
	return output.Flush()
}

// pngDataWriter bounds compressed buffering independently of the image's dimensions.
type pngDataWriter struct {
	output io.Writer
	data   [32 * 1024]byte
	size   int
}

// Write splits one continuous zlib stream across bounded PNG IDAT chunks.
func (writer *pngDataWriter) Write(data []byte) (int, error) {
	written := 0
	for len(data) > 0 {
		copied := copy(writer.data[writer.size:], data)
		writer.size += copied
		written += copied
		data = data[copied:]
		if writer.size == len(writer.data) {
			if err := writer.flush(); err != nil {
				return written, err
			}
		}
	}
	return written, nil
}

// flush publishes the remaining compressed bytes as one complete IDAT chunk.
func (writer *pngDataWriter) flush() error {
	if writer.size == 0 {
		return nil
	}
	err := writePNGChunk(writer.output, "IDAT", writer.data[:writer.size])
	writer.size = 0
	return err
}

// writePNGChunk includes the type in the CRC, as required by the PNG datastream format.
func writePNGChunk(writer io.Writer, kind string, data []byte) error {
	var header [8]byte
	binary.BigEndian.PutUint32(header[:4], uint32(len(data)))
	copy(header[4:], kind)
	checksum := crc32.Update(crc32.ChecksumIEEE(header[4:]), crc32.IEEETable, data)
	var trailer [4]byte
	binary.BigEndian.PutUint32(trailer[:], checksum)
	for _, part := range [][]byte{header[:], data, trailer[:]} {
		if written, err := writer.Write(part); err != nil {
			return err
		} else if written != len(part) {
			return io.ErrShortWrite
		}
	}
	return nil
}
