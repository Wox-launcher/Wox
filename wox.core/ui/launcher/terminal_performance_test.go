package launcher

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"wox/plugin/system/shell/interactive"
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

// TestShellOutputLayoutReusesRows verifies incremental wrapping agrees with a fresh layout.
func TestShellOutputLayoutReusesRows(t *testing.T) {
	var cached shellOutputLayout
	for _, columns := range []int{1, 4, 8} {
		for _, text := range []string{"", "one\r", "one\r\n", "one\r\n中文", "one\r\n中文\tlast", "one\r\n中文\tlast\n", "earlier\none\r\n中文\tlast\n", "short"} {
			cached.prepare("go", text, "", columns)
			var fresh shellOutputLayout
			fresh.prepare("go", text, "", columns)
			if !reflect.DeepEqual(cached.rows, fresh.rows) {
				t.Fatalf("%d columns, %q: incremental rows %v, fresh %v", columns, text, cached.rows, fresh.rows)
			}
		}
	}
	cached.prepare("", "abcd\tZ", "", 4)
	if want := []int{0, 4, 5}; !reflect.DeepEqual(cached.rows, want) {
		t.Fatalf("tab after a full row: got %v, want %v", cached.rows, want)
	}
}

// TestShellOutputFrameShowsCommandAndWideText checks the grid and original output offsets.
func TestShellOutputFrameShowsCommandAndWideText(t *testing.T) {
	var layout shellOutputLayout
	output := terminalPreviewSnapshot{Command: "go", Text: "中x\nlast"}
	layout.prepare(output.Command, output.Text, "", 4)
	frame, offsets := layout.frame(interactive.Size{Columns: 4, Rows: 3}, 0, output, defaultPalette().componentTheme())
	if frame.Cells[0].Text != ">" || frame.Cells[2].Text != "g" ||
		frame.Cells[4].Text != "中" || !frame.Cells[4].Wide || !frame.Cells[5].Continuation ||
		frame.Cells[6].Text != "x" || offsets[6] != len("中") || offsets[8] != len("中x\n") {
		t.Fatalf("grid cells %+v, byte offsets %v", frame.Cells, offsets)
	}
}

// BenchmarkTerminalPreparation measures the visible grid with loaded output and search matches.
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
			theme := defaultPalette().componentTheme()
			app.buildShellOutputPreview(snapshot, theme, 560, 500, 1)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if mode == "stream" {
					snapshot.Text = value + fmt.Sprint(i)
				}
				app.buildShellOutputPreview(snapshot, theme, 560, 500, 1)
			}
		})
	}
}
