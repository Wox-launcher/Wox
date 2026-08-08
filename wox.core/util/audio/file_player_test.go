package audio

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadPCM16WAV(t *testing.T) {
	pcm := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	data := make([]byte, 44+len(pcm))
	copy(data[0:4], "RIFF")
	binary.LittleEndian.PutUint32(data[4:8], uint32(len(data)-8))
	copy(data[8:12], "WAVE")
	copy(data[12:16], "fmt ")
	binary.LittleEndian.PutUint32(data[16:20], 16)
	binary.LittleEndian.PutUint16(data[20:22], 1)
	binary.LittleEndian.PutUint16(data[22:24], 1)
	binary.LittleEndian.PutUint32(data[24:28], 16000)
	binary.LittleEndian.PutUint32(data[28:32], 32000)
	binary.LittleEndian.PutUint16(data[32:34], 2)
	binary.LittleEndian.PutUint16(data[34:36], 16)
	copy(data[36:40], "data")
	binary.LittleEndian.PutUint32(data[40:44], uint32(len(pcm)))
	copy(data[44:], pcm)
	path := filepath.Join(t.TempDir(), "test.wav")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	gotPCM, sampleRate, channels, err := readPCM16WAV(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotPCM) != string(pcm) || sampleRate != 16000 || channels != 1 {
		t.Fatalf("decoded WAV = %v, %d Hz, %d channels", gotPCM, sampleRate, channels)
	}
}

func TestFilePlayerSnapshotUsesConsumedPCMPosition(t *testing.T) {
	player := &FilePlayer{path: "test.wav", pcm: make([]byte, 32000), sampleRate: 16000, channels: 1, duration: time.Second}
	player.position.Store(16000)
	player.playing.Store(true)

	snapshot := player.Snapshot()
	if !snapshot.Playing || snapshot.Position != 500*time.Millisecond || snapshot.Duration != time.Second {
		t.Fatalf("snapshot = %+v, want playing at 500ms of 1s", snapshot)
	}
}
