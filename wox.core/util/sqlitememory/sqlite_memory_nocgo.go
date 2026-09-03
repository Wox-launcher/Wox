//go:build !cgo

package sqlitememory

// Builds without cgo cannot contain native SQLite, so there is nothing to attribute.
func usedBytes() uint64 {
	return 0
}

func peakBytes() uint64 {
	return 0
}
