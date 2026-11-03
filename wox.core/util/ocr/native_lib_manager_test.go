package ocr

import (
	"path/filepath"
	"sync"
	"testing"
)

// TestNativeLibManagerSharesConcurrentDownloads ensures OCR consumers coordinate on one runtime manager.
func TestNativeLibManagerSharesConcurrentDownloads(t *testing.T) {
	root := t.TempDir()
	managers := make([]*NativeLibManager, 16)
	errors := make([]error, len(managers))
	var group sync.WaitGroup
	for i := range managers {
		group.Add(1)
		go func() {
			defer group.Done()
			managers[i], errors[i] = NewNativeLibManager(filepath.Join(root, "sherpa"), filepath.Join(root, "onnx"))
		}()
	}
	group.Wait()
	for i := range managers {
		if errors[i] != nil || managers[i] != managers[0] {
			t.Fatalf("manager %d = %p, error %v; want shared manager %p", i, managers[i], errors[i], managers[0])
		}
	}
}
