// Package filesearchbench supplies identical, deterministic inputs to index comparisons.
package filesearchbench

import "fmt"

type Entry struct {
	ID, ParentID uint64
	Name         string
	Directory    bool
}

// Entries includes repeated names, unique names, and Chinese names without disk I/O.
// Index-only timings must not be confused with the native smoke filesystem timings.
func Entries(count int) []Entry {
	entries := make([]Entry, 0, count+101)
	entries = append(entries, Entry{ID: 5, Name: "", Directory: true})
	for directory := 0; directory < 100; directory++ {
		entries = append(entries, Entry{ID: uint64(100 + directory), ParentID: 5, Name: fmt.Sprintf("project-%03d", directory), Directory: true})
	}
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("report-%04d.txt", i%1000)
		if i%4 == 0 {
			name = fmt.Sprintf("document-%06d.md", i)
		}
		if i%10 == 0 {
			name = fmt.Sprintf("项目报告-%04d.txt", i%1000)
		}
		entries = append(entries, Entry{ID: uint64(1000 + i), ParentID: uint64(100 + i/1000%100), Name: name})
	}
	return entries
}

var Queries = []string{"report", "document-000104", "rpt", "xiangmu", "xm", "项目"}
