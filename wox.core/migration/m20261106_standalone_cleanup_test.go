package migration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestCleanupDictationModels verifies both successful cleanup and preservation of conflicting copies.
func TestCleanupDictationModels(t *testing.T) {
	root := t.TempDir()
	source, destination := filepath.Join(root, "old"), filepath.Join(root, "new")
	write := func(path, value string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(value), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, base := range []string{source, destination} {
		write(filepath.Join(base, "same", "tokenizer", "vocab.json"), "vocab")
		write(filepath.Join(base, "same", "model.onnx"), "model")
	}
	write(filepath.Join(destination, "same", ".complete"), "ready")
	write(filepath.Join(source, "conflict", "model.onnx"), "old")
	write(filepath.Join(destination, "conflict", "model.onnx"), "new")
	write(filepath.Join(source, "missing", "model.onnx"), "only old")
	write(filepath.Join(source, "silero-vad", "silero_vad.onnx"), "vad")
	write(filepath.Join(destination, "same", "silero_vad.onnx"), "vad")
	for range 2 {
		if err := cleanupDictationModels(context.Background(), source, destination); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(source, "same")); !os.IsNotExist(err) {
		t.Fatalf("copied model not removed: %v", err)
	}
	for _, path := range []string{filepath.Join(source, "conflict", "model.onnx"), filepath.Join(source, "missing", "model.onnx"), filepath.Join(source, "silero-vad", "silero_vad.onnx"), filepath.Join(destination, "same", "model.onnx")} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
	}
	write(filepath.Join(destination, "conflict", "model.onnx"), "old")
	write(filepath.Join(destination, "missing", "model.onnx"), "only old")
	if err := cleanupDictationModels(context.Background(), source, destination); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("legacy directory not removed: %v", err)
	}
}

func TestCleanupDictationModelsPreservesVADWithoutCopy(t *testing.T) {
	root := t.TempDir()
	source, destination := filepath.Join(root, "old"), filepath.Join(root, "new")
	vad := filepath.Join(source, "silero-vad", "silero_vad.onnx")
	if err := os.MkdirAll(filepath.Dir(vad), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(vad, []byte("old"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := cleanupDictationModels(context.Background(), source, destination); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(vad); err != nil {
		t.Fatal(err)
	}
}

func TestCleanupDictationModelsAfterCopy(t *testing.T) {
	root := t.TempDir()
	source, destination := filepath.Join(root, "old"), filepath.Join(root, "new")
	name := "sherpa-onnx-sense-voice-zh-en-ja-ko-yue-int8-2024-07-17"
	model := filepath.Join(source, name)
	if err := os.MkdirAll(model, 0755); err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"model.int8.onnx", "tokens.txt"} {
		if err := os.WriteFile(filepath.Join(model, file), []byte(file), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := copyDictationModels(context.Background(), source, destination); err != nil {
		t.Fatal(err)
	}
	if err := cleanupDictationModels(context.Background(), source, destination); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(source); !os.IsNotExist(err) {
		t.Fatalf("legacy model not removed: %v", err)
	}
	for _, file := range []string{"model.int8.onnx", "tokens.txt", ".wox-migrated"} {
		if _, err := os.Stat(filepath.Join(destination, name, file)); err != nil {
			t.Fatal(err)
		}
	}
}
