package woxview

import (
	"fmt"
	"strings"
	"testing"
)

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

func benchmarkFrames(b *testing.B, rows int) {
	view := NewView()
	tree, err := Parse([]byte(perfTree(rows, 0)))
	if err != nil {
		b.Fatal(err)
	}
	view.SetTree(tree)
	var events []recordedEvent
	host := renderView(&testing.T{}, view, &events)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		frame(host)
	}
}

func benchmarkUpdates(b *testing.B, rows int) {
	view := NewView()
	tree, _ := Parse([]byte(perfTree(rows, 0)))
	view.SetTree(tree)
	var events []recordedEvent
	host := renderView(&testing.T{}, view, &events)
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

func BenchmarkTmpFrame100(b *testing.B)   { benchmarkFrames(b, 100) }
func BenchmarkTmpFrame1000(b *testing.B)  { benchmarkFrames(b, 1000) }
func BenchmarkTmpUpdate100(b *testing.B)  { benchmarkUpdates(b, 100) }
func BenchmarkTmpUpdate1000(b *testing.B) { benchmarkUpdates(b, 1000) }
