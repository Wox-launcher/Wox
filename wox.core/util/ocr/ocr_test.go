package ocr

import (
	"path/filepath"
	"reflect"
	"testing"
	"wox/util/speech"
)

func TestSortLinesByReadingOrder(t *testing.T) {
	lines := []Line{
		{Text: "title", Bounds: []Point{{X: 120, Y: 91}, {X: 500, Y: 91}, {X: 500, Y: 138}, {X: 120, Y: 138}}},
		{Text: "avatar artifact", Bounds: []Point{{X: 20, Y: 59}, {X: 100, Y: 59}, {X: 100, Y: 142}, {X: 20, Y: 142}}},
		{Text: "author", Bounds: []Point{{X: 120, Y: 50}, {X: 500, Y: 50}, {X: 500, Y: 93}, {X: 120, Y: 93}}},
		{Text: "bottom-right", Bounds: []Point{{X: 60, Y: 250}, {X: 100, Y: 250}, {X: 100, Y: 270}, {X: 60, Y: 270}}},
		{Text: "bottom-left", Bounds: []Point{{X: 10, Y: 250}, {X: 50, Y: 250}, {X: 50, Y: 270}, {X: 10, Y: 270}}},
	}

	sortLinesByReadingOrder(lines)

	got := make([]string, 0, len(lines))
	for _, line := range lines {
		got = append(got, line.Text)
	}
	want := []string{"author", "avatar artifact", "title", "bottom-left", "bottom-right"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("line order = %v, want %v", got, want)
	}
}

func TestPaddleEngineStatusReportsMissingRuntime(t *testing.T) {
	root := t.TempDir()
	runtimeManager, err := speech.NewNativeLibManager(filepath.Join(root, "sherpa"), filepath.Join(root, "onnx"), filepath.Join(root, "vad"))
	if err != nil {
		t.Fatal(err)
	}

	status := (&PaddleModelManager{runtimeManager: runtimeManager}).GetEngineStatus()
	if status.Ready {
		t.Fatal("expected a new OCR engine directory to be reported as unavailable")
	}
}
