package speech

// RecognizerConfig configures a streaming speech recognizer.
type RecognizerConfig struct {
	// ModelPath is the directory containing the model files (encoder.onnx,
	// decoder.onnx, joiner.onnx, tokens.txt, etc.).
	ModelPath string
	// ModelType identifies which model architecture to load. Supported
	// values: "zipformer2" (streaming transducer), "paraformer" (streaming
	// paraformer).
	ModelType string
	// NumThreads controls the number of inference threads. 1 is a safe
	// default for real-time dictation.
	NumThreads int
	// Language tells the recognizer which language to expect. Used for
	// models that support multiple languages.
	Language string
}

// PartialResult is a partial (interim) recognition result.
type PartialResult struct {
	Text string
}

// FinalResult is the final recognition result after endpoint detection.
type FinalResult struct {
	Text string
}

// Recognizer is the platform-agnostic interface for streaming speech
// recognition. The concrete implementation wraps sherpa-onnx's
// OnlineRecognizer.
type Recognizer interface {
	// AcceptWaveform feeds audio samples into the recognizer. Samples
	// must be 16kHz mono float32 PCM.
	AcceptWaveform(sampleRate int, samples []float32)
	// GetResult returns the current partial recognition text.
	GetResult() PartialResult
	// IsReady reports whether the recognizer has enough buffered audio
	// to run another decode pass.
	IsReady() bool
	// Decode runs one decode pass on the buffered audio. Call this in a
	// loop after feeding audio until IsReady returns false.
	Decode()
	// IsEndpoint reports whether an endpoint (sentence boundary) was
	// detected. When true, the caller should retrieve the final result
	// and call Reset to start a new segment.
	IsEndpoint() bool
	// Reset clears the recognizer's internal state for a new segment.
	Reset()
	// Close releases the recognizer resources. Must be called exactly once.
	Close()
}
