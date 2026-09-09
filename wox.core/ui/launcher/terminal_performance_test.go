package launcher

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	woxui "wox/ui/runtime"
	"wox/ui/widget"
)

// TestTerminalLayoutReuse compares incremental chunks and invalidation with a fresh layout.
func TestTerminalLayoutReuse(t *testing.T) {
	var cache textLayoutCache
	key := textLayoutKey{session: "test", width: 560, scale: 1, lineHeight: 18, style: woxui.TextStyle{Size: 12}}
	values := []string{"", "one", "one\r", "one\r\n", "one\r\n中文", "one\r\n中文\n\nlast", "one\r\n中文\n\nreplacement", "history\none\r\n中文\n\nreplacement", "trimmed\n", "x\r\ry", "x\r\ry\r", "x\r\ry\r\nend"}
	for _, width := range []float32{560, 0, 320} {
		key.width = width
		key.scale += 0.5
		for _, value := range values {
			old := append([]string(nil), cache.layout.Lines...)
			previous := cache.layout.Lines
			got := cache.measure(value, key)
			want := widget.LayoutTextBlock(nil, value, key.style, key.width, 0, key.lineHeight)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("width %v value %q: got %#v, want %#v", width, value, got, want)
			}
			if !reflect.DeepEqual(previous, old) {
				t.Fatal("incremental layout mutated a retained frame")
			}
			if reused := cache.measure(value, key); &reused.Lines[0] != &got.Lines[0] {
				t.Fatal("unchanged output was laid out again")
			}
		}
	}
}

// BenchmarkTerminalPreparation includes layout and search preparation before clipped painting.
func BenchmarkTerminalPreparation(b *testing.B) {
	for _, mode := range []string{"scroll", "stream", "search"} {
		b.Run(mode, func(b *testing.B) {
			app := New(false, nil)
			defer app.cancel()
			line := "perffind: a long Shell output line with numbers 123456789 and Unicode 中文.\n"
			value := strings.Repeat(line, 10000)
			snapshot := terminalPreviewSnapshot{SessionID: "perf", Text: value}
			if mode == "search" {
				for i := range 10000 {
					snapshot.Matches = append(snapshot.Matches, terminalMatch{start: i * len(line), end: i*len(line) + 8})
				}
			}
			palette := defaultPalette()
			app.buildTerminalPreview(snapshot, palette, 560, 500, 1, nil)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if mode == "stream" {
					snapshot.Text = value + fmt.Sprint(i)
				}
				app.buildTerminalPreview(snapshot, palette, 560, 500, 1, nil)
			}
		})
	}
}
