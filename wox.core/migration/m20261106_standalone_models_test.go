package migration

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyDictationModels(t *testing.T) {
	root := t.TempDir()
	source, dest := filepath.Join(root, "old"), filepath.Join(root, "new")
	qwen := "sherpa-onnx-qwen3-asr-0.6B-int8-2026-03-25"
	sense := "sherpa-onnx-sense-voice-zh-en-ja-ko-yue-int8-2024-07-17"
	write := func(path, value string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(value), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, name := range []string{"conv_frontend.onnx", "encoder.int8.onnx", "decoder.int8.onnx", "tokenizer/vocab.json", "tokenizer/merges.txt"} {
		write(filepath.Join(source, qwen, name), name)
	}
	write(filepath.Join(source, sense, "model.int8.onnx"), "old")
	write(filepath.Join(dest, sense, "model.int8.onnx"), "new")
	write(filepath.Join(source, "custom", "model.onnx"), "custom")
	write(filepath.Join(source, ".downloading-123", "partial"), "partial")
	write(filepath.Join(source, "silero-vad", "silero_vad.onnx"), "invalid VAD")
	for range 2 {
		if err := copyDictationModels(context.Background(), source, dest); err != nil {
			t.Fatal(err)
		}
	}
	for path, want := range map[string]string{
		filepath.Join(source, qwen, "conv_frontend.onnx"):    "conv_frontend.onnx",
		filepath.Join(dest, qwen, "tokenizer", "vocab.json"): "tokenizer/vocab.json",
		filepath.Join(dest, qwen, ".wox-migrated"):           "legacy",
		filepath.Join(dest, sense, "model.int8.onnx"):        "new",
		filepath.Join(dest, "custom", "model.onnx"):          "custom",
	} {
		data, err := os.ReadFile(path)
		if err != nil || string(data) != want {
			t.Fatalf("%s: %q, %v", path, data, err)
		}
	}
	for _, path := range []string{filepath.Join(dest, qwen, ".complete"), filepath.Join(dest, qwen, "silero_vad.onnx"), filepath.Join(dest, "custom", ".wox-migrated"), filepath.Join(dest, "silero-vad")} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("unexpected path %s: %v", path, err)
		}
	}
	if err := copyDictationModels(context.Background(), filepath.Join(root, "absent"), dest); err != nil {
		t.Fatal(err)
	}
}

func TestCopyDictationModelsCancellation(t *testing.T) {
	root := t.TempDir()
	source := filepath.Join(root, "source", "model")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	dest := filepath.Join(root, "dest")
	if err := copyDictationModels(ctx, filepath.Dir(source), dest); err != context.Canceled {
		t.Fatalf("expected cancellation, got %v", err)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("unexpected destination: %v", err)
	}
}

// TestCopyDictationModelsWithVAD verifies reuse of an actual legacy VAD when supplied.
func TestCopyDictationModelsWithVAD(t *testing.T) {
	vad := os.Getenv("WOX_TEST_LEGACY_VAD")
	if vad == "" {
		t.Skip("set WOX_TEST_LEGACY_VAD to test with the real VAD file")
	}
	root := t.TempDir()
	source := filepath.Join(root, "source", "sherpa-onnx-sense-voice-zh-en-ja-ko-yue-int8-2024-07-17")
	if err := os.MkdirAll(source, 0755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"model.int8.onnx", "tokens.txt"} {
		if err := os.WriteFile(filepath.Join(source, name), []byte(name), 0644); err != nil {
			t.Fatal(err)
		}
	}
	target := filepath.Join(root, filepath.Base(source))
	if err := copyDictationModel(source, target, vad); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(target, ".complete")); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(filepath.Join(target, "silero_vad.onnx"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if n, err := io.Copy(io.Discard, f); err != nil || n == 0 {
		t.Fatalf("copied VAD: %d bytes, %v", n, err)
	}
}
