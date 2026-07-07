package speech

import (
	"context"
	"fmt"
	"time"
	"wox/util"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
)

// VadConfig configures a VoiceActivityDetector for speech segmentation.
type VadConfig struct {
	// ModelPath is the path to silero_vad.onnx.
	ModelPath string
	// Threshold controls speech detection sensitivity (typical: 0.5).
	Threshold float32
	// MinSilenceDuration is the silence duration to trigger segment split (seconds).
	MinSilenceDuration float32
	// MinSpeechDuration is the minimum speech duration to keep a segment (seconds).
	MinSpeechDuration float32
	// WindowSize is the VAD analysis window size in samples (512 for 16kHz).
	WindowSize int
	// MaxSpeechDuration is the maximum segment length before forced split (seconds).
	MaxSpeechDuration float32
	// NumThreads controls inference threads.
	NumThreads int
}

// DefaultVadConfig returns the recommended VAD parameters for 16kHz dictation.
func DefaultVadConfig(modelPath string) VadConfig {
	return VadConfig{
		ModelPath:          modelPath,
		Threshold:          0.5,
		MinSilenceDuration: 0.5,
		MinSpeechDuration:  0.25,
		WindowSize:         512,
		MaxSpeechDuration:  5.0,
		NumThreads:         1,
	}
}

// VoiceActivityDetector wraps sherpa-onnx VAD for real-time speech
// segmentation. Audio samples are fed in continuously; the detector splits
// them into complete speech segments separated by silence.
type VoiceActivityDetector struct {
	config VadConfig
	vad    *sherpa.VoiceActivityDetector
}

// NewVoiceActivityDetector creates a VAD from the given config.
func NewVoiceActivityDetector(ctx context.Context, config VadConfig) (*VoiceActivityDetector, error) {
	t0 := time.Now()
	logger := util.GetLogger()

	sherpaConfig := sherpa.VadModelConfig{
		SileroVad: sherpa.SileroVadModelConfig{
			Model:              config.ModelPath,
			Threshold:          config.Threshold,
			MinSilenceDuration: config.MinSilenceDuration,
			MinSpeechDuration:  config.MinSpeechDuration,
			WindowSize:         config.WindowSize,
			MaxSpeechDuration:  config.MaxSpeechDuration,
		},
		SampleRate: 16000,
		NumThreads: config.NumThreads,
		Provider:   "cpu",
		Debug:      0,
	}

	vad := sherpa.NewVoiceActivityDetector(&sherpaConfig, 20.0)
	if vad == nil {
		return nil, fmt.Errorf("failed to create VAD (model: %s)", config.ModelPath)
	}
	logger.Info(ctx, fmt.Sprintf("dictation timing: vad.NewVoiceActivityDetector cost=%dms", time.Since(t0).Milliseconds()))

	return &VoiceActivityDetector{
		config: config,
		vad:    vad,
	}, nil
}

// AcceptWaveform feeds 16kHz mono float32 PCM samples into the VAD.
func (v *VoiceActivityDetector) AcceptWaveform(samples []float32) {
	v.vad.AcceptWaveform(samples)
}

// IsSpeech reports whether speech is currently being detected.
func (v *VoiceActivityDetector) IsSpeech() bool {
	return v.vad.IsSpeech()
}

// IsEmpty reports whether there are no completed speech segments available.
func (v *VoiceActivityDetector) IsEmpty() bool {
	return v.vad.IsEmpty()
}

// Front returns the first completed speech segment without removing it.
func (v *VoiceActivityDetector) Front() *sherpa.SpeechSegment {
	return v.vad.Front()
}

// Pop removes the first completed speech segment.
func (v *VoiceActivityDetector) Pop() {
	v.vad.Pop()
}

// Flush forces any buffered audio at the end of a session into a final segment.
func (v *VoiceActivityDetector) Flush() {
	v.vad.Flush()
}

// Clear resets the VAD internal buffer and state.
func (v *VoiceActivityDetector) Clear() {
	v.vad.Clear()
}

// Close releases the VAD resources. Must be called exactly once.
func (v *VoiceActivityDetector) Close() {
	if v.vad != nil {
		sherpa.DeleteVoiceActivityDetector(v.vad)
		v.vad = nil
	}
}
