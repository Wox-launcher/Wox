package speech

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"time"

	"wox/util"
)

// AudioDecodeResult is one offline pass over a complete diagnostic WAV.
type AudioDecodeResult struct {
	Path    string
	Text    string
	Elapsed time.Duration
}

// ponytail: serialize diagnostic model loads across windows; use a memory-budgeted
// scheduler only if parallel comparisons become necessary.
var audioDecodeSlot = make(chan struct{}, 1)

// DecodeAudioFiles loads one recognizer and decodes each WAV in order.
// Comparison uses the full file, not the live VAD-segmented path.
func DecodeAudioFiles(ctx context.Context, model LocalModel, paths []string) ([]AudioDecodeResult, error) {
	if IsStreamingModelType(model.ModelType) {
		return nil, fmt.Errorf("streaming model %s is not supported for file comparison", model.ID)
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no audio files to decode")
	}

	select {
	case audioDecodeSlot <- struct{}{}:
		defer func() { <-audioDecodeSlot }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	waveforms := make([][]float32, len(paths))
	for i, path := range paths {
		samples, err := ReadPCM16WAV(path)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", filepath.Base(path), err)
		}
		waveforms[i] = samples
	}

	threads := max(runtime.NumCPU()/2, 1)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rec, err := newRecognizer(ctx, RecognizerConfig{
		ModelPath:  model.Path,
		ModelType:  model.ModelType,
		NumThreads: threads,
	})
	if err != nil {
		return nil, err
	}
	defer rec.Close()

	results := make([]AudioDecodeResult, len(paths))
	for i, path := range paths {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		started := time.Now()
		text := cleanRecognizerText(rec.DecodeSamples(waveforms[i]))
		elapsed := time.Since(started)
		results[i] = AudioDecodeResult{Path: path, Text: text, Elapsed: elapsed}
		util.GetLogger().Info(ctx, fmt.Sprintf(
			"dictation compare: model=%s file=%s samples=%d elapsed=%dms empty=%t",
			model.ID, filepath.Base(path), len(waveforms[i]), elapsed.Milliseconds(), text == "",
		))
	}
	return results, nil
}
