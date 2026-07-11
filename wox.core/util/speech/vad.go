package speech

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

// SpeechSegment contains one complete speech segment returned by the VAD.
type SpeechSegment struct {
	Start   int
	Samples []float32
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
