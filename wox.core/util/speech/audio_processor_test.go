package speech

import (
	"math"
	"testing"
	"time"
)

const processorTestChunkSamples = audioSampleRate / 10

func TestAdaptiveAudioProcessorObserveDoesNotMutateSamples(t *testing.T) {
	processor := NewAdaptiveAudioProcessor()
	chunk := sineChunk(dbfsToAmplitude(-36), 800, processorTestChunkSamples, 0)
	original := append([]float32(nil), chunk...)

	processor.Observe(chunk)
	for i := range chunk {
		if chunk[i] != original[i] {
			t.Fatal("Observe mutated recognition samples")
		}
	}
}

func TestAdaptiveAudioProcessorTracksCandidates(t *testing.T) {
	processor := NewAdaptiveAudioProcessor()
	for range 4 {
		processor.Observe(sineChunk(dbfsToAmplitude(-40), 120, processorTestChunkSamples, 0))
	}
	if processor.Stats().CandidateDuration < 100*time.Millisecond {
		t.Fatalf("expected fallback candidates, got %s", processor.Stats().CandidateDuration)
	}
}

func TestApplyPreviewVolumeBoostsQuietSpeechWithoutMutatingInput(t *testing.T) {
	input := sineChunk(dbfsToAmplitude(-36), 800, processorTestChunkSamples*6, 0)
	original := append([]float32(nil), input...)

	out, gainDB := applyPreviewVolume(input)
	if len(out) != len(input) {
		t.Fatalf("length = %d, want %d", len(out), len(input))
	}
	for i := range input {
		if input[i] != original[i] {
			t.Fatal("applyPreviewVolume mutated the input waveform")
		}
	}
	if gainDB <= 0 {
		t.Fatalf("quiet speech preview gain=%.2f, want > 0", gainDB)
	}
	if math.Abs(chunkPeak(out)-previewPeakLimit) > 0.01 {
		t.Fatalf("preview peak=%.3f, want %.3f", chunkPeak(out), previewPeakLimit)
	}
}

func TestApplyPreviewVolumePreservesRelativeDynamics(t *testing.T) {
	quiet := float32(0.05)
	loud := float32(0.20)
	input := []float32{quiet, loud, -quiet, -loud}
	out, _ := applyPreviewVolume(input)
	if math.Abs(float64(out[1]/out[0])-float64(loud/quiet)) > 1e-5 {
		t.Fatalf("preview volume changed relative amplitudes: %.5f vs %.5f", out[1]/out[0], loud/quiet)
	}
}

func TestApplyPreviewVolumeDoesNotExceedPeakLimit(t *testing.T) {
	input := sineChunk(0.95, 800, processorTestChunkSamples, 0)
	out, _ := applyPreviewVolume(input)
	if chunkPeak(out) > previewPeakLimit+1e-6 {
		t.Fatalf("preview peak=%.3f exceeded limit %.3f", chunkPeak(out), previewPeakLimit)
	}
}

func TestPreviewVolumeStatsUsesConstantGain(t *testing.T) {
	raw := sineChunk(dbfsToAmplitude(-36), 800, processorTestChunkSamples, 0)
	processor := NewAdaptiveAudioProcessor()
	processor.Observe(raw)
	boosted, gainDB := applyPreviewVolume(raw)
	stats := previewVolumeStats(processor.Stats(), boosted, gainDB)
	if stats.AverageGainDB != gainDB || stats.MaximumGainDB != gainDB {
		t.Fatalf("preview stats average=%.2f max=%.2f, want %.2f", stats.AverageGainDB, stats.MaximumGainDB, gainDB)
	}
	if stats.OutputPeakDBFS <= stats.InputPeakDBFS {
		t.Fatalf("preview stats did not raise peak: in=%.1f out=%.1f", stats.InputPeakDBFS, stats.OutputPeakDBFS)
	}
}

func sineChunk(amplitude float64, freqHz float64, samples int, phase float64) []float32 {
	chunk := make([]float32, samples)
	step := 2 * math.Pi * freqHz / audioSampleRate
	for i := range chunk {
		chunk[i] = float32(amplitude * math.Sin(phase+float64(i)*step))
	}
	return chunk
}

func dbfsToAmplitude(dbfs float64) float64 {
	return math.Pow(10, dbfs/20)
}

func chunkPeak(samples []float32) float64 {
	var peak float64
	for _, sample := range samples {
		value := math.Abs(float64(sample))
		if value > peak {
			peak = value
		}
	}
	return peak
}
