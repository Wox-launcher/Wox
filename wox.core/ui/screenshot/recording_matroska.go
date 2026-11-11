package screenshot

import (
	"encoding/binary"
	"io"
	"math/bits"
)

// recordingMatroskaHeader describes a single uncompressed I420 track on a non-seekable pipe.
func recordingMatroskaHeader(width, height, fps int) []byte {
	ebml := matroskaUint(nil, 0x4286, 1) // EBMLVersion
	ebml = matroskaUint(ebml, 0x42F7, 1) // EBMLReadVersion
	ebml = matroskaUint(ebml, 0x42F2, 4) // EBMLMaxIDLength
	ebml = matroskaUint(ebml, 0x42F3, 8) // EBMLMaxSizeLength
	ebml = matroskaElement(ebml, 0x4282, []byte("matroska"))
	ebml = matroskaUint(ebml, 0x4287, 4) // DocTypeVersion
	ebml = matroskaUint(ebml, 0x4285, 2) // DocTypeReadVersion
	header := matroskaElement(nil, 0x1A45DFA3, ebml)
	header = append(header, 0x18, 0x53, 0x80, 0x67, 0xFF) // Segment with unknown size

	info := matroskaUint(nil, 0x2AD7B1, 1000) // TimestampScale in nanoseconds
	info = matroskaElement(info, 0x4D80, []byte("Wox"))
	info = matroskaElement(info, 0x5741, []byte("Wox"))
	header = matroskaElement(header, 0x1549A966, info)

	video := matroskaUint(nil, 0xB0, uint64(width))
	video = matroskaUint(video, 0xBA, uint64(height))
	video = matroskaElement(video, 0x2EB524, []byte("I420"))
	track := matroskaUint(nil, 0xD7, 1)    // TrackNumber
	track = matroskaUint(track, 0x73C5, 1) // TrackUID
	track = matroskaUint(track, 0x83, 1)   // TrackType video
	track = matroskaUint(track, 0x9C, 0)   // FlagLacing
	track = matroskaElement(track, 0x86, []byte("V_UNCOMPRESSED"))
	track = matroskaUint(track, 0x23E383, 1_000_000_000/uint64(fps)) // DefaultDuration
	track = matroskaElement(track, 0xE0, video)
	return matroskaElement(header, 0x1654AE6B, matroskaElement(nil, 0xAE, track))
}

// writeRecordingMatroskaFrame preserves the capture timeline without sending duplicate frames to the live encoder.
func writeRecordingMatroskaFrame(writer io.Writer, pixels []byte, index int64, fps int) error {
	var timeBuffer [16]byte
	timestamp := matroskaUint(timeBuffer[:0], 0xE7, (uint64(index)*1_000_000+uint64(fps)/2)/uint64(fps))
	var blockBuffer [16]byte
	block := matroskaElementHeader(blockBuffer[:0], 0xA3, len(pixels)+4)
	block = append(block, 0x81, 0x00, 0x00, 0x80) // Track 1, relative timestamp 0, keyframe
	var clusterBuffer [48]byte
	cluster := matroskaElementHeader(clusterBuffer[:0], 0x1F43B675, len(timestamp)+len(block)+len(pixels))
	cluster = append(cluster, timestamp...)
	cluster = append(cluster, block...)
	if _, err := writer.Write(cluster); err != nil {
		return err
	}
	_, err := writer.Write(pixels)
	return err
}

func matroskaUint(dst []byte, id uint32, value uint64) []byte {
	var data [8]byte
	binary.BigEndian.PutUint64(data[:], value)
	return matroskaElement(dst, id, data[8-max(1, (bits.Len64(value)+7)/8):])
}

func matroskaElement(dst []byte, id uint32, data []byte) []byte {
	return append(matroskaElementHeader(dst, id, len(data)), data...)
}

// matroskaElementHeader writes an EBML ID and its variable-width size, reserving all-ones for unknown sizes.
func matroskaElementHeader(dst []byte, id uint32, size int) []byte {
	var elementID [4]byte
	binary.BigEndian.PutUint32(elementID[:], id)
	dst = append(dst, elementID[4-(bits.Len32(id)+7)/8:]...)
	var encodedSize [8]byte
	width := 1
	for uint64(size) >= (uint64(1)<<(7*width))-1 {
		width++
	}
	binary.BigEndian.PutUint64(encodedSize[:], (uint64(1)<<(7*width))|uint64(size))
	return append(dst, encodedSize[8-width:]...)
}
