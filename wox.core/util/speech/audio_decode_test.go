package speech

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func TestDecodeAudioFilesRejectsMissingWav(t *testing.T) {
	_, err := DecodeAudioFiles(context.Background(), LocalModel{ID: "qwen3", ModelType: "qwen3_asr", Path: t.TempDir()}, []string{filepath.Join(t.TempDir(), "missing.wav")})
	if err == nil {
		t.Fatal("expected missing wav to fail before loading a recognizer")
	}
}

// TestDecodeAudioFilesWaitsForSlot verifies cancellation while another model owns memory.
func TestDecodeAudioFilesWaitsForSlot(t *testing.T) {
	audioDecodeSlot <- struct{}{}
	defer func() { <-audioDecodeSlot }()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := DecodeAudioFiles(ctx, LocalModel{ID: "qwen3", ModelType: "qwen3_asr"}, []string{"missing.wav"})
		done <- err
	}()
	select {
	case err := <-done:
		t.Fatalf("decode bypassed occupied slot: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v, want cancellation before loading a model", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("canceled decode did not leave the queue")
	}
}

func TestDecodeAudioFilesRejectsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := DecodeAudioFiles(ctx, LocalModel{ID: "qwen3", ModelType: "qwen3_asr"}, []string{"missing.wav"})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want cancellation before reading audio", err)
	}
}

func TestDecodeAudioFilesRejectsStreamingModel(t *testing.T) {
	_, err := DecodeAudioFiles(context.Background(), LocalModel{ID: "zipformer", ModelType: "zipformer2"}, []string{"raw.wav"})
	if err == nil {
		t.Fatal("expected streaming models to be rejected")
	}
}
