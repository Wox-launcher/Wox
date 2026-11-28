//go:build windows

package filesearch

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"io"
	"os"
	"path/filepath"
	"strings"

	"wox/util/filesearch/column"
)

// Bump the version when the layout, ignore semantics or filename scores change.
func (f *nativeQueryVisibility) cacheKey(v string) []byte {
	return []byte("visibility-v2/" + v + "/" + f.rules.CacheKey())
}

func (f *nativeQueryVisibility) loadCache(v column.View) *nativeBaseVisibility {
	if f.cacheDir == "" {
		return nil
	}
	file, err := os.Open(filepath.Join(f.cacheDir, "visibility.cache"))
	if err != nil {
		return nil
	}
	defer file.Close()
	var want [sha256.Size]byte
	if _, err = io.ReadFull(file, want[:]); err != nil {
		return nil
	}
	h := sha256.New()
	h.Write(f.cacheKey(v.Generation))
	r := io.TeeReader(file, h)
	mask := &nativeBaseVisibility{flags: make([]byte, v.Base.Len()), names: make([]uint64, (v.Base.Names()+63)/64), frequentMatches: map[string][]column.ScoredName{}}
	if _, err = io.ReadFull(r, mask.flags); err != nil {
		return nil
	}
	if binary.Read(r, binary.LittleEndian, mask.names) != nil {
		return nil
	}
	var count uint32
	if binary.Read(r, binary.LittleEndian, &count) != nil || count > frequentTermLimit {
		return nil
	}
	remaining := uint32(frequentScoreBudget)
	for range count {
		var key [4]byte
		if _, err = io.ReadFull(r, key[:]); err != nil {
			return nil
		}
		var size uint32
		if binary.Read(r, binary.LittleEndian, &size) != nil || size > remaining {
			return nil
		}
		remaining -= size
		candidates := make([]column.ScoredName, size)
		if binary.Read(r, binary.LittleEndian, candidates) != nil {
			return nil
		}
		mask.frequentMatches[strings.TrimRight(string(key[:]), "\x00")] = candidates
	}
	if !bytes.Equal(want[:], h.Sum(nil)) {
		return nil
	}
	return mask
}

// Incomplete cache writes are disposable and never replace a usable mask.
func (f *nativeQueryVisibility) saveCache(generation string, mask *nativeBaseVisibility) {
	if f.cacheDir == "" {
		return
	}
	file, err := os.CreateTemp(f.cacheDir, ".visibility-*")
	if err != nil {
		return
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if _, err = file.Seek(sha256.Size, io.SeekStart); err != nil {
		return
	}
	h := sha256.New()
	h.Write(f.cacheKey(generation))
	w := io.MultiWriter(file, h)
	if _, err = w.Write(mask.flags); err != nil {
		return
	}
	if binary.Write(w, binary.LittleEndian, mask.names) != nil || binary.Write(w, binary.LittleEndian, uint32(len(mask.frequentMatches))) != nil {
		return
	}
	for name, candidates := range mask.frequentMatches {
		var key [4]byte
		copy(key[:], name)
		if _, err = w.Write(key[:]); err != nil {
			return
		}
		if binary.Write(w, binary.LittleEndian, uint32(len(candidates))) != nil || binary.Write(w, binary.LittleEndian, candidates) != nil {
			return
		}
	}
	if _, err = file.WriteAt(h.Sum(nil), 0); err == nil && file.Close() == nil {
		_ = os.Rename(file.Name(), filepath.Join(f.cacheDir, "visibility.cache"))
	}
}
