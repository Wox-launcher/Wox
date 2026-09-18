package speech

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// ReadPCM16WAV loads a mono 16 kHz PCM16 WAV written by the diagnostic dumper.
func ReadPCM16WAV(path string) ([]float32, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(data) < 44 {
		return nil, fmt.Errorf("wav too short")
	}
	if string(data[0:4]) != "RIFF" || string(data[8:12]) != "WAVE" {
		return nil, fmt.Errorf("not a WAVE file")
	}

	offset := 12
	var audioFormat, channels, bitsPerSample uint16
	var sampleRate uint32
	var pcm []byte
	for offset+8 <= len(data) {
		chunkID := string(data[offset : offset+4])
		chunkSize := int(binary.LittleEndian.Uint32(data[offset+4 : offset+8]))
		offset += 8
		if chunkSize < 0 || offset+chunkSize > len(data) {
			return nil, fmt.Errorf("invalid %s chunk", chunkID)
		}
		switch chunkID {
		case "fmt ":
			if chunkSize < 16 {
				return nil, fmt.Errorf("fmt chunk too short")
			}
			audioFormat = binary.LittleEndian.Uint16(data[offset : offset+2])
			channels = binary.LittleEndian.Uint16(data[offset+2 : offset+4])
			sampleRate = binary.LittleEndian.Uint32(data[offset+4 : offset+8])
			bitsPerSample = binary.LittleEndian.Uint16(data[offset+14 : offset+16])
		case "data":
			pcm = data[offset : offset+chunkSize]
		}
		offset += chunkSize
		if chunkSize%2 == 1 {
			offset++
		}
	}
	if audioFormat != 1 || channels != 1 || bitsPerSample != 16 {
		return nil, fmt.Errorf("unsupported wav format format=%d channels=%d bits=%d", audioFormat, channels, bitsPerSample)
	}
	if sampleRate != audioSampleRate {
		return nil, fmt.Errorf("unsupported wav sample rate %d", sampleRate)
	}
	if len(pcm)%2 != 0 {
		return nil, io.ErrUnexpectedEOF
	}

	samples := make([]float32, len(pcm)/2)
	for i := range samples {
		value := int16(binary.LittleEndian.Uint16(pcm[i*2 : i*2+2]))
		samples[i] = float32(value) / 32767
	}
	return samples, nil
}
