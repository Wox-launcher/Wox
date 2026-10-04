package screenshotedit

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"wox/util/clipboard"
)

type blockedSceneImage struct {
	image.Image
	once             sync.Once
	started, release chan struct{}
}

// At holds PNG encoding open so retention can race the final archive commit deterministically.
func (img *blockedSceneImage) At(x, y int) color.Color {
	img.once.Do(func() { close(img.started); <-img.release })
	return img.Image.At(x, y)
}

func TestSceneBackgroundCommitDoesNotResurrectDeletedScreenshot(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.jpg")
	pixels := image.NewRGBA(image.Rect(0, 0, 8, 8))
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, pixels, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	source := &blockedSceneImage{Image: pixels, started: make(chan struct{}), release: make(chan struct{})}
	done := make(chan error, 1)
	go func() { done <- Save(path, json.RawMessage(`{}`), source, nil, pixels) }()
	defer func() {
		select {
		case <-source.release:
		default:
			close(source.release)
		}
	}()
	select {
	case <-source.started:
	case <-time.After(5 * time.Second):
		t.Fatal("scene encoding did not start")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := Remove(path); err != nil {
		t.Fatal(err)
	}
	close(source.release)
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("scene committed after screenshot deletion")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("scene write did not finish")
	}
	if entries, err := os.ReadDir(filepath.Dir(path)); err != nil || len(entries) != 0 {
		t.Fatal("background write resurrected a scene or leaked its temporary file")
	}
}

func TestSceneRejectsCorruptAndUnsupportedArchives(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.jpg")
	if err := os.WriteFile(path, []byte("export"), 0600); err != nil {
		t.Fatal(err)
	}
	hash, err := fileHash(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name, source string
		version      int
	}{
		{"unsupported version", "", 2},
		{"missing source", "", 1},
		{"corrupt source", "not a PNG", 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			var buffer bytes.Buffer
			archive := zip.NewWriter(&buffer)
			entry, err := archive.Create("scene.json")
			if err != nil {
				t.Fatal(err)
			}
			if err := json.NewEncoder(entry).Encode(Metadata{Version: test.version, ExportHash: hash, State: json.RawMessage(`{}`), ImageHashes: []string{hash, hash}}); err != nil {
				t.Fatal(err)
			}
			if test.source != "" {
				entry, err = archive.Create("source.png")
				if err != nil {
					t.Fatal(err)
				}
				if _, err := entry.Write([]byte(test.source)); err != nil {
					t.Fatal(err)
				}
			}
			if err := archive.Close(); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path+Suffix, buffer.Bytes(), 0600); err != nil {
				t.Fatal(err)
			}
			if _, _, _, err := Load(path); err == nil {
				t.Fatal("invalid scene accepted")
			}
		})
	}
	if err := os.WriteFile(path+Suffix, []byte("broken ZIP"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := Load(path); err == nil {
		t.Fatal("invalid ZIP accepted")
	}
	if Resolve(strings.Repeat("0", 64)) != "" {
		t.Fatal("unrelated external image matched a scene")
	}
}

func TestSceneClipboardIndexAndLifecycle(t *testing.T) {
	source := image.NewRGBA(image.Rect(0, 0, 61, 39))
	for y := 0; y < 39; y++ {
		for x := 0; x < 61; x++ {
			source.SetRGBA(x, y, color.RGBA{R: uint8(x * 3), G: uint8(y * 5), B: uint8(x * y), A: 255})
		}
	}
	var encoded bytes.Buffer
	if err := jpeg.Encode(&encoded, source, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	exported, err := jpeg.Decode(bytes.NewReader(encoded.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	rawHash, jpegHash := clipboard.ImageHash(source), clipboard.ImageHash(exported)
	if rawHash == jpegHash {
		t.Fatal("fixture must cover lossy JPEG pixels")
	}
	path := filepath.Join(t.TempDir(), "capture.jpg")
	if err := os.WriteFile(path, encoded.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Remove(path) })
	if err := Save(path, json.RawMessage(`{"test":true}`), source, nil, source); err != nil {
		t.Fatal(err)
	}
	for _, hash := range []string{rawHash, jpegHash} {
		if Resolve(hash) != path {
			t.Fatal("clipboard copy variant lost scene association")
		}
	}
	metadata, loaded, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if clipboard.ImageHash(loaded) != rawHash {
		t.Fatal("lossless source changed")
	}

	// Restart rebuilds associations from JSON without decoding the source PNG.
	index.Lock()
	index.hashes = make(map[string]map[string]bool)
	index.Unlock()
	metadata, err = ReadMetadata(path)
	if err != nil {
		t.Fatal(err)
	}
	Register(path, metadata)
	if Resolve(rawHash) != path {
		t.Fatal("restart lost association")
	}
	other := filepath.Join(t.TempDir(), "other.jpg")
	if err := os.WriteFile(other, encoded.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Remove(other) })
	if err := Save(other, json.RawMessage(`{"test":false}`), source, nil, source); err != nil {
		t.Fatal(err)
	}
	if Resolve(rawHash) != "" {
		t.Fatal("ambiguous crops must not choose an arbitrary scene")
	}
	if err := Remove(other); err != nil {
		t.Fatal(err)
	}
	if Resolve(rawHash) != path {
		t.Fatal("expired candidate still blocks valid scene")
	}
	if err := os.WriteFile(path, []byte("replaced"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := Load(path); err == nil {
		t.Fatal("replaced export accepted")
	}
	if err := Remove(path); err != nil {
		t.Fatal(err)
	}
	if Resolve(rawHash) != "" || Available(path) {
		t.Fatal("expired scene still editable")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("scene cleanup deleted image")
	}
}

func TestSceneSaveFailureKeepsExport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.jpg")
	var encoded bytes.Buffer
	source := image.NewRGBA(image.Rect(0, 0, 3, 3))
	if err := jpeg.Encode(&encoded, source, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, encoded.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path+Suffix, 0700); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, json.RawMessage(`{}`), source, nil, source); err == nil {
		t.Fatal("expected scene commit failure")
	}
	actual, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(actual, encoded.Bytes()) {
		t.Fatal("failed scene save damaged export")
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil || len(entries) != 2 {
		t.Fatal("temporary scene leaked")
	}
}
