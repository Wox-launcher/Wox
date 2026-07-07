package speech

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// RecognizerConfig configures a speech recognizer.
type RecognizerConfig struct {
	ModelPath  string
	ModelType  string
	NumThreads int
	Language   string
}

// PartialResult is a partial (interim) recognition result.
type PartialResult struct {
	Text string
}

// Recognizer is the platform-agnostic interface for speech recognition.
// It supports two modes:
//   - Streaming (online): feed audio continuously via AcceptWaveform, get
//     partial results via GetResult. Used by zipformer2/paraformer models.
//   - Offline: decode a complete audio segment via DecodeSamples. Used by
//     Qwen3-ASR.
type Recognizer interface {
	// IsStreaming reports whether this recognizer is a streaming (online)
	// recognizer that uses AcceptWaveform/GetResult/Decode/IsEndpoint/Reset.
	IsStreaming() bool
	// AcceptWaveform feeds audio samples into a streaming recognizer.
	AcceptWaveform(sampleRate int, samples []float32)
	// GetResult returns the current partial recognition text.
	GetResult() PartialResult
	// IsReady reports whether the recognizer has enough buffered audio to decode.
	IsReady() bool
	// Decode runs one decode pass on buffered audio.
	Decode()
	// IsEndpoint reports whether an endpoint (sentence boundary) was detected.
	IsEndpoint() bool
	// Reset clears the recognizer's internal state for a new segment.
	Reset()
	// DecodeSamples runs offline recognition on a complete audio segment and
	// returns the recognized text. Only used by non-streaming recognizers.
	DecodeSamples(samples []float32) string
	// Close releases the recognizer resources. Must be called exactly once.
	Close()
}

// IsStreamingModelType reports whether the model type uses streaming (online)
// recognition vs offline recognition with VAD.
func IsStreamingModelType(modelType string) bool {
	return modelType == "zipformer2" || modelType == "paraformer"
}

// newRecognizer creates the appropriate recognizer for the given config.
// Streaming models (zipformer2, paraformer) use OnlineRecognizer; offline
// models (qwen3_asr) use OfflineRecognizer.
func newRecognizer(ctx context.Context, config RecognizerConfig) (Recognizer, error) {
	if IsStreamingModelType(config.ModelType) {
		return newOnlineRecognizer(ctx, config)
	}
	return newOfflineRecognizer(ctx, config)
}

// findModelFile finds the first file matching the given glob pattern in the
// model directory. It prefers non-quantized (.onnx without int8) over
// quantized (.int8.onnx) files for better accuracy.
func findModelFile(modelDir, pattern string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(modelDir, pattern))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no file matching %s", pattern)
	}
	for _, m := range matches {
		base := filepath.Base(m)
		if !strings.Contains(base, "int8") {
			return m, nil
		}
	}
	return matches[0], nil
}
