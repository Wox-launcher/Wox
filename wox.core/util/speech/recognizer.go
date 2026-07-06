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

// findModelFile finds the first file matching the given glob pattern in the
// model directory. It prefers non-quantized (.onnx without int8) over
// quantized (.int8.onnx) files for better accuracy. Returns the full path or
// an error if no match is found.
func findModelFile(modelDir, pattern string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(modelDir, pattern))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("no file matching %s", pattern)
	}
	// Prefer the non-int8 (full precision) file when both exist.
	for _, m := range matches {
		base := filepath.Base(m)
		if !strings.Contains(base, "int8") {
			return m, nil
		}
	}
	return matches[0], nil
}

// sherpaRecognizer wraps sherpa-onnx OnlineRecognizer to implement the
// Recognizer interface. The underlying C resources are managed through
// the sherpa Go bindings; Close must be called to free them.
//
// The recognizer (loaded model) and the stream (per-session decode state) are
// independent C objects. CloseStream releases only the stream, keeping the
// model in memory for reuse by the recognizer pool. Close releases both.
type sherpaRecognizer struct {
	config     RecognizerConfig
	recognizer *sherpa.OnlineRecognizer
	stream     *sherpa.OnlineStream
}

// newSherpaModel loads the ONNX model into memory and returns the
// recognizer. This is the expensive call (hundreds of ms) and is cached by
// the recognizer pool so it only happens once per model until idle eviction.
func newSherpaModel(ctx context.Context, config RecognizerConfig) (*sherpa.OnlineRecognizer, error) {
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
		return nil, fmt.Errorf("unsupported model type: %s", config.ModelType)
	}
	sherpaConfig.ModelConfig.Tokens = filepath.Join(config.ModelPath, "tokens.txt")

	recognizer := sherpa.NewOnlineRecognizer(&sherpaConfig)
	if recognizer == nil {
		return nil, fmt.Errorf("failed to create sherpa recognizer (model path: %s)", config.ModelPath)
	}
	return recognizer, nil
}

// newSherpaRecognizer creates a streaming recognizer with a fresh stream.
// Used as a fallback when the pool is not available.
func newSherpaRecognizer(ctx context.Context, config RecognizerConfig) (Recognizer, error) {
	t0 := time.Now()
	logger := util.GetLogger()

	recognizer, err := newSherpaModel(ctx, config)
	if err != nil {
		return nil, err
	}
	logger.Debug(ctx, fmt.Sprintf("dictation timing: recognizer.NewOnlineRecognizer cost=%dms", time.Since(t0).Milliseconds()))

	stream := sherpa.NewOnlineStream(recognizer)
	if stream == nil {
		sherpa.DeleteOnlineRecognizer(recognizer)
		return nil, fmt.Errorf("failed to create sherpa stream")
	}
	logger.Debug(ctx, fmt.Sprintf("dictation timing: recognizer.NewOnlineStream cost=%dms", time.Since(t0).Milliseconds()))
	logger.Debug(ctx, fmt.Sprintf("dictation timing: recognizer.total cost=%dms", time.Since(t0).Milliseconds()))

	return &sherpaRecognizer{
		config:     config,
		recognizer: recognizer,
		stream:     stream,
	}, nil
}

// wrapSherpaRecognizer creates a recognizer wrapper from an already-loaded
// model and a newly created stream. Used by the pool path.
func wrapSherpaRecognizer(ctx context.Context, config RecognizerConfig, recognizer *sherpa.OnlineRecognizer) (*sherpaRecognizer, error) {
	stream := sherpa.NewOnlineStream(recognizer)
	if stream == nil {
		return nil, fmt.Errorf("failed to create sherpa stream")
	}
	return &sherpaRecognizer{
		config:     config,
		recognizer: recognizer,
		stream:     stream,
	}, nil
}

func (r *sherpaRecognizer) AcceptWaveform(sampleRate int, samples []float32) {
	r.stream.AcceptWaveform(sampleRate, samples)
}

func (r *sherpaRecognizer) GetResult() PartialResult {
	return PartialResult{Text: r.recognizer.GetResult(r.stream).Text}
}

func (r *sherpaRecognizer) IsReady() bool {
	return r.recognizer.IsReady(r.stream)
}

func (r *sherpaRecognizer) Decode() {
	r.recognizer.Decode(r.stream)
}

func (r *sherpaRecognizer) IsEndpoint() bool {
	return r.recognizer.IsEndpoint(r.stream)
}

func (r *sherpaRecognizer) Reset() {
	r.recognizer.Reset(r.stream)
}

// CloseStream releases only the per-session stream, keeping the loaded model
// in memory for reuse. Call this when the recognizer came from a pool so the
// model can be returned to the pool.
func (r *sherpaRecognizer) CloseStream() {
	if r.stream != nil {
		sherpa.DeleteOnlineStream(r.stream)
		r.stream = nil
	}
}

// Close releases both the stream and the recognizer. Use this when the
// recognizer is NOT pooled (e.g. pool miss fallback or error cleanup).
func (r *sherpaRecognizer) Close() {
	r.CloseStream()
	if r.recognizer != nil {
		sherpa.DeleteOnlineRecognizer(r.recognizer)
		r.recognizer = nil
	}
}
