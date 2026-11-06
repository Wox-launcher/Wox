package woxview

import (
	"fmt"
	"strings"
	"testing"
)

// perfTree keeps five nodes per row; 800 rows stay within the protocol node budget.
func perfTree(rows int, version int) string {
	var b strings.Builder
	b.WriteString(`{"version":1,"root":{"type":"scroll","children":[`)
	for i := 0; i < rows; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		fmt.Fprintf(&b, `{"type":"row","key":"r%d","props":{"style":{"gap":8,"padding":4}},"children":[{"type":"text","children":[{"type":"span","props":{"text":"Row %d v%d"}}]},{"type":"checkbox","props":{"label":"c","value":true}},{"type":"button","props":{"label":"Go","onPress":{"$handler":"h%d"}}}]}`, i, i, version, i)
	}
	b.WriteString(`]}}`)
	return b.String()
}

// benchmarkFrames compares eager and virtualized layout with the same mixed controls.
func benchmarkFrames(b *testing.B, rows int, lazy bool) {
	view := NewView()
	data := perfTree(rows, 0)
	if lazy {
		data = strings.Replace(data, `"type":"scroll"`, `"type":"lazy-list","props":{"itemHeight":32}`, 1)
	}
	tree, err := Parse([]byte(data))
	if err != nil {
		b.Fatal(err)
	}
	view.SetTree(tree)
	var events []recordedEvent
	host := renderView(b, view, &events)
	b.Cleanup(host.Dispose)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		frame(host)
	}
}

// benchmarkUpdates includes parsing and retained-state reconciliation in each update.
func benchmarkUpdates(b *testing.B, rows int) {
	view := NewView()
	tree, err := Parse([]byte(perfTree(rows, 0)))
	if err != nil {
		b.Fatal(err)
	}
	view.SetTree(tree)
	var events []recordedEvent
	host := renderView(b, view, &events)
	b.Cleanup(host.Dispose)
	data := [][]byte{[]byte(perfTree(rows, 1)), []byte(perfTree(rows, 2))}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		next, err := Parse(data[i%2])
		if err != nil {
			b.Fatal(err)
		}
		view.SetTree(next)
		frame(host)
	}
}

// BenchmarkWoxViewFrame measures steady frames at representative document sizes.
func BenchmarkWoxViewFrame(b *testing.B) {
	for _, rows := range []int{100, 800} {
		b.Run(fmt.Sprintf("scroll/%d", rows), func(b *testing.B) { benchmarkFrames(b, rows, false) })
		b.Run(fmt.Sprintf("lazy/%d", rows), func(b *testing.B) { benchmarkFrames(b, rows, true) })
	}
}

// BenchmarkWoxViewUpdate measures complete producer updates at the same document sizes.
func BenchmarkWoxViewUpdate(b *testing.B) {
	for _, rows := range []int{100, 800} {
		b.Run(fmt.Sprint(rows), func(b *testing.B) { benchmarkUpdates(b, rows) })
	}
}
