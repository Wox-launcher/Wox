//go:build filesearch_diagnostic

package filesearch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"runtime/pprof"
	"sort"
	"strings"
	"testing"
	"time"
	"wox/util/filesearch/column"
	"wox/util/fuzzymatch"
)

// TestReadOnlyNativeQueryDiagnostic measures copied real snapshots and journals.
// Copying warms OS file caches; the first sample is process-cold, not disk-cold.
func TestReadOnlyNativeQueryDiagnostic(t *testing.T) {
	source := os.Getenv("WOX_FILESEARCH_REAL_INDEX")
	if source == "" {
		t.Skip("set WOX_FILESEARCH_REAL_INDEX to the native-columns directory")
	}
	paths, err := filepath.Glob(filepath.Join(source, "*", "current.json"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("native manifests: %v (%d)", err, len(paths))
	}
	var stores []*column.Store
	destination := t.TempDir()
	for _, manifestPath := range paths {
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			t.Fatal(err)
		}
		var manifest struct{ Generation string }
		if err := json.Unmarshal(data, &manifest); err != nil {
			t.Fatal(err)
		}
		token, err := hex.DecodeString(manifest.Generation)
		if err != nil || len(token) != 16 {
			t.Fatal("invalid generation")
		}
		folder := filepath.Join(destination, filepath.Base(filepath.Dir(manifestPath)))
		if err := os.MkdirAll(folder, 0700); err != nil {
			t.Fatal(err)
		}
		for _, suffix := range []string{".columns", ".log"} {
			input, err := os.Open(filepath.Join(filepath.Dir(manifestPath), manifest.Generation+suffix))
			if err != nil {
				t.Fatal(err)
			}
			info, err := input.Stat()
			if err != nil {
				input.Close()
				t.Fatal(err)
			}
			output, err := os.Create(filepath.Join(folder, manifest.Generation+suffix))
			if err != nil {
				input.Close()
				t.Fatal(err)
			}
			_, copyErr := io.CopyN(output, input, info.Size())
			input.Close()
			closeErr := output.Close()
			if copyErr != nil || closeErr != nil {
				t.Fatalf("copy: %v %v", copyErr, closeErr)
			}
		}
		if err := os.WriteFile(filepath.Join(folder, "current.json"), data, 0600); err != nil {
			t.Fatal(err)
		}
		store, err := column.OpenStore(folder, prepareColumnNames)
		if err != nil {
			t.Fatal(err)
		}
		defer store.Close()
		stores = append(stores, store)
		store.Read(func(v column.View) error {
			t.Logf("rows=%d names=%d delta=%d", v.Base.Len(), v.Base.Names(), len(v.Delta))
			return nil
		})
	}
	index := newDirectoryIndex("")
	for i, store := range stores {
		index.shards[fmt.Sprint(i)] = &directoryShard{store: store}
	}
	type sample struct {
		Query     string
		Millis    float64
		Results   int
		Signature string
		Allocated uint64
	}
	var samples []sample
	queries := []string{"qqyy", "qqyy", "qqyy", "woxzzzzmissing"}
	if raw := os.Getenv("WOX_QUERY_DIAGNOSTIC_QUERIES"); raw != "" {
		queries = strings.Split(raw, "|")
	}
	runtime.GC()
	if path := os.Getenv("WOX_QUERY_DIAGNOSTIC_PROFILE"); path != "" {
		file, err := os.Create(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := pprof.StartCPUProfile(file); err != nil {
			file.Close()
			t.Fatal(err)
		}
		defer func() { pprof.StopCPUProfile(); file.Close() }()
	}
	for _, q := range queries {
		if q == "<idle>" {
			if err := index.releaseIdleMemory(); err != nil {
				t.Fatal(err)
			}
			fuzzymatch.ReleaseIdleCaches()
			fuzzymatch.ReleasePinyinDictionary()
			debug.FreeOSMemory()
			continue
		}
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		started := time.Now()
		results, err := index.search(context.Background(), SearchQuery{Raw: q}, 100)
		if err != nil {
			t.Fatal(err)
		}

		elapsed := float64(time.Since(started)) / float64(time.Millisecond)
		if len(results) > 0 {
			t.Logf("score range %q: %d..%d", q, results[0].Score, results[len(results)-1].Score)
		}
		runtime.ReadMemStats(&after)
		keys := make([]string, len(results))
		for i, r := range results {
			keys[i] = fmt.Sprintf("%s:%d", r.Path, r.Score)
		}
		sort.Strings(keys)
		data, _ := json.Marshal(keys)
		hash := sha256.Sum256(data)
		s := sample{q, elapsed, len(results), hex.EncodeToString(hash[:]), after.TotalAlloc - before.TotalAlloc}
		samples = append(samples, s)
		t.Logf("query=%q elapsed=%.3fms results=%d allocated=%d signature=%s", q, elapsed, len(results), s.Allocated, s.Signature)
	}
	if path := os.Getenv("WOX_QUERY_DIAGNOSTIC_OUTPUT"); path != "" {
		data, _ := json.MarshalIndent(samples, "", "  ")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
}
