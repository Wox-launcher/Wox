// Package sqlitememory reports the native heap SQLite holds for Wox's file index, content search
// and clipboard databases. Those allocations are page caches and FTS structures owned by the C
// library, so they never appear in Go runtime counters and would otherwise be indistinguishable
// from any other native allocation in the process.
package sqlitememory

// UsedBytes reports native bytes currently allocated by SQLite across every open connection.
// It reports zero when the build has no native SQLite or when SQLite's memory statistics are
// disabled.
func UsedBytes() uint64 {
	return usedBytes()
}

// PeakBytes reports the highest native SQLite allocation total reached since process start, which
// separates a database that is currently large from one that only spiked during indexing.
func PeakBytes() uint64 {
	return peakBytes()
}
