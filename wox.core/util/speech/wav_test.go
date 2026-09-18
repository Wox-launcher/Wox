package speech

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestReadPCM16WAVRoundTrip(t *testing.T) {
	samples := []float32{0, 0.25, -0.5, 0.9, -0.9}
	path := filepath.Join(t.TempDir(), "clip.wav")
	if err := writePCM16WAV(path, samples); err != nil {
		t.Fatal(err)
	}

	got, err := ReadPCM16WAV(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(samples) {
		t.Fatalf("samples = %d, want %d", len(got), len(samples))
	}
	for i := range samples {
		if math.Abs(float64(got[i]-samples[i])) > 0.001 {
			t.Fatalf("sample[%d] = %f, want %f", i, got[i], samples[i])
		}
	}
}

func TestReadPCM16WAVRejectsShortFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "short.wav")
	if err := os.WriteFile(path, []byte("RIFF"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadPCM16WAV(path); err == nil {
		t.Fatal("expected short wav to fail")
	}
}
