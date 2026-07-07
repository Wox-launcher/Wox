package speech

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"wox/util"

	sherpa "github.com/k2-fsa/sherpa-onnx-go/sherpa_onnx"
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

// ---------------------------------------------------------------------------
// Streaming (online) recognizer — zipformer2 / paraformer
// ---------------------------------------------------------------------------

// sherpaOnlineRecognizer wraps sherpa-onnx OnlineRecognizer for streaming models.
type sherpaOnlineRecognizer struct {
	config     RecognizerConfig
	recognizer *sherpa.OnlineRecognizer
	stream     *sherpa.OnlineStream
}

func newOnlineRecognizer(ctx context.Context, config RecognizerConfig) (Recognizer, error) {
	t0 := time.Now()
	logger := util.GetLogger()

	sherpaConfig := sherpa.OnlineRecognizerConfig{
		FeatConfig: sherpa.FeatureConfig{
			SampleRate: 16000,
			FeatureDim: 80,
		},
		ModelConfig: sherpa.OnlineModelConfig{
			NumThreads: config.NumThreads,
			Debug:      0,
			Provider:   "cpu",
			ModelType:  config.ModelType,
		},
		DecodingMethod:          "greedy_search",
		MaxActivePaths:          4,
		EnableEndpoint:          1,
		Rule1MinTrailingSilence: 2.4,
		Rule2MinTrailingSilence: 1.2,
		Rule3MinUtteranceLength: 20,
	}

	switch config.ModelType {
	case "zipformer2":
		encoder, err := findModelFile(config.ModelPath, "encoder*.onnx")
		if err != nil {
			return nil, fmt.Errorf("encoder model not found in %s: %w", config.ModelPath, err)
		}
		decoder, err := findModelFile(config.ModelPath, "decoder*.onnx")
		if err != nil {
			return nil, fmt.Errorf("decoder model not found in %s: %w", config.ModelPath, err)
		}
		joiner, err := findModelFile(config.ModelPath, "joiner*.onnx")
		if err != nil {
			return nil, fmt.Errorf("joiner model not found in %s: %w", config.ModelPath, err)
		}
		sherpaConfig.ModelConfig.Transducer.Encoder = encoder
		sherpaConfig.ModelConfig.Transducer.Decoder = decoder
		sherpaConfig.ModelConfig.Transducer.Joiner = joiner
	case "paraformer":
		sherpaConfig.ModelConfig.Paraformer.Encoder = filepath.Join(config.ModelPath, "encoder.int8.onnx")
		sherpaConfig.ModelConfig.Paraformer.Decoder = filepath.Join(config.ModelPath, "decoder.int8.onnx")
	default:
		return nil, fmt.Errorf("unsupported streaming model type: %s", config.ModelType)
	}
	sherpaConfig.ModelConfig.Tokens = filepath.Join(config.ModelPath, "tokens.txt")

	logger.Info(ctx, fmt.Sprintf("dictation timing: recognizer.findModelFile cost=%dms", time.Since(t0).Milliseconds()))

	recognizer := sherpa.NewOnlineRecognizer(&sherpaConfig)
	if recognizer == nil {
		return nil, fmt.Errorf("failed to create sherpa recognizer (model path: %s)", config.ModelPath)
	}
	logger.Info(ctx, fmt.Sprintf("dictation timing: recognizer.NewOnlineRecognizer cost=%dms", time.Since(t0).Milliseconds()))

	stream := sherpa.NewOnlineStream(recognizer)
	if stream == nil {
		sherpa.DeleteOnlineRecognizer(recognizer)
		return nil, fmt.Errorf("failed to create sherpa stream")
	}
	logger.Info(ctx, fmt.Sprintf("dictation timing: recognizer.total cost=%dms", time.Since(t0).Milliseconds()))

	return &sherpaOnlineRecognizer{
		config:     config,
		recognizer: recognizer,
		stream:     stream,
	}, nil
}

func (r *sherpaOnlineRecognizer) IsStreaming() bool { return true }

func (r *sherpaOnlineRecognizer) AcceptWaveform(sampleRate int, samples []float32) {
	r.stream.AcceptWaveform(sampleRate, samples)
}

func (r *sherpaOnlineRecognizer) GetResult() PartialResult {
	return PartialResult{Text: r.recognizer.GetResult(r.stream).Text}
}

func (r *sherpaOnlineRecognizer) IsReady() bool {
	return r.recognizer.IsReady(r.stream)
}

func (r *sherpaOnlineRecognizer) Decode() {
	r.recognizer.Decode(r.stream)
}

func (r *sherpaOnlineRecognizer) IsEndpoint() bool {
	return r.recognizer.IsEndpoint(r.stream)
}

func (r *sherpaOnlineRecognizer) Reset() {
	r.recognizer.Reset(r.stream)
}

func (r *sherpaOnlineRecognizer) DecodeSamples(samples []float32) string {
	// Not used for streaming recognizers.
	return ""
}

func (r *sherpaOnlineRecognizer) Close() {
	if r.stream != nil {
		sherpa.DeleteOnlineStream(r.stream)
		r.stream = nil
	}
	if r.recognizer != nil {
		sherpa.DeleteOnlineRecognizer(r.recognizer)
		r.recognizer = nil
	}
}

// ---------------------------------------------------------------------------
// Offline recognizer — Qwen3-ASR
// ---------------------------------------------------------------------------

// sherpaOfflineRecognizer wraps sherpa-onnx OfflineRecognizer for Qwen3-ASR.
type sherpaOfflineRecognizer struct {
	config     RecognizerConfig
	recognizer *sherpa.OfflineRecognizer
}

func newOfflineRecognizer(ctx context.Context, config RecognizerConfig) (Recognizer, error) {
	t0 := time.Now()
	logger := util.GetLogger()

	sherpaConfig := sherpa.OfflineRecognizerConfig{
		FeatConfig: sherpa.FeatureConfig{
			SampleRate: 16000,
			FeatureDim: 80,
		},
		ModelConfig: sherpa.OfflineModelConfig{
			NumThreads: config.NumThreads,
			Debug:      0,
			Provider:   "cpu",
			ModelType:  config.ModelType,
		},
		DecodingMethod: "greedy_search",
		MaxActivePaths: 4,
	}

	switch config.ModelType {
	case "qwen3_asr":
		sherpaConfig.ModelConfig.Qwen3ASR.ConvFrontend = filepath.Join(config.ModelPath, "conv_frontend.onnx")
		sherpaConfig.ModelConfig.Qwen3ASR.Encoder = filepath.Join(config.ModelPath, "encoder.int8.onnx")
		sherpaConfig.ModelConfig.Qwen3ASR.Decoder = filepath.Join(config.ModelPath, "decoder.int8.onnx")
		sherpaConfig.ModelConfig.Qwen3ASR.Tokenizer = filepath.Join(config.ModelPath, "tokenizer")
		sherpaConfig.ModelConfig.Qwen3ASR.Seed = 42
		sherpaConfig.ModelConfig.Tokens = ""
	default:
		return nil, fmt.Errorf("unsupported offline model type: %s", config.ModelType)
	}

	recognizer := sherpa.NewOfflineRecognizer(&sherpaConfig)
	if recognizer == nil {
		return nil, fmt.Errorf("failed to create offline recognizer (model path: %s)", config.ModelPath)
	}
	logger.Info(ctx, fmt.Sprintf("dictation timing: recognizer.NewOfflineRecognizer cost=%dms", time.Since(t0).Milliseconds()))
	logger.Info(ctx, fmt.Sprintf("dictation timing: recognizer.total cost=%dms", time.Since(t0).Milliseconds()))

	return &sherpaOfflineRecognizer{
		config:     config,
		recognizer: recognizer,
	}, nil
}

func (r *sherpaOfflineRecognizer) IsStreaming() bool { return false }

func (r *sherpaOfflineRecognizer) AcceptWaveform(sampleRate int, samples []float32) {}

func (r *sherpaOfflineRecognizer) GetResult() PartialResult { return PartialResult{} }

func (r *sherpaOfflineRecognizer) IsReady() bool { return false }

func (r *sherpaOfflineRecognizer) Decode() {}

func (r *sherpaOfflineRecognizer) IsEndpoint() bool { return false }

func (r *sherpaOfflineRecognizer) Reset() {}

func (r *sherpaOfflineRecognizer) DecodeSamples(samples []float32) string {
	stream := sherpa.NewOfflineStream(r.recognizer)
	if stream == nil {
		return ""
	}
	defer sherpa.DeleteOfflineStream(stream)

	stream.AcceptWaveform(16000, samples)
	r.recognizer.Decode(stream)
	result := stream.GetResult()
	if result == nil {
		return ""
	}
	return result.Text
}

func (r *sherpaOfflineRecognizer) Close() {
	if r.recognizer != nil {
		sherpa.DeleteOfflineRecognizer(r.recognizer)
		r.recognizer = nil
	}
}

// ---------------------------------------------------------------------------
// Factory
// ---------------------------------------------------------------------------

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
