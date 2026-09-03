//go:build cgo

package sqlitememory

/*
#include <stdint.h>

// These are exported by the SQLite amalgamation that github.com/mattn/go-sqlite3 compiles into
// the binary. Declaring them here keeps this package independent of that driver's internals while
// still reading SQLite's own allocation counters, which are exact rather than sampled.
extern int64_t sqlite3_memory_used(void);
extern int64_t sqlite3_memory_highwater(int resetFlag);
*/
import "C"

// The blank import guarantees the native SQLite objects are linked into any binary that reads
// these counters, including per-package test binaries that do not otherwise open a database.
import _ "github.com/mattn/go-sqlite3"

func usedBytes() uint64 {
	return nonNegative(int64(C.sqlite3_memory_used()))
}

func peakBytes() uint64 {
	return nonNegative(int64(C.sqlite3_memory_highwater(0)))
}

func nonNegative(value int64) uint64 {
	if value < 0 {
		return 0
	}
	return uint64(value)
}
