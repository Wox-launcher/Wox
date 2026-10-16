//go:build windows && filesearch_diagnostic

package filesearch

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sync"
	"syscall"
	"testing"
	"time"
	"unsafe"
	"wox/util/filesearch/column"
	"wox/util/filesearchservice"
	"wox/util/filesearchservice/protocol"
)

// TestNativeBuildMemoryDiagnostic records real bounded fact frames once, then
// replays the identical enumeration and catch-up through the production builder.
// Every invocation uses a fresh store; production indexes are never opened.
func TestNativeBuildMemoryDiagnostic(t *testing.T) {
	dir := os.Getenv("WOX_BUILD_DIAGNOSTIC")
	if dir == "" {
		t.Skip("set WOX_BUILD_DIAGNOSTIC")
	}
	root := os.Getenv("WOX_BUILD_VOLUME")
	if root == "" {
		root = `C:\`
	}
	capture := os.Getenv("WOX_BUILD_CAPTURE") == "1"
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	store, err := column.OpenStore(t.TempDir(), prepareColumnNames)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	request := 0
	stream := func(ctx context.Context, kind protocol.Kind, boundary protocol.Boundary, emit filesearchservice.FactSink) error {
		if os.Getenv("WOX_BUILD_LIVE") == "1" {
			return filesearchservice.StreamFacts(ctx, kind, boundary, emit)
		}
		path := filepath.Join(dir, fmt.Sprintf("request-%03d.bin", request))
		request++
		if capture {
			file, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
			if err != nil {
				return err
			}
			defer file.Close()
			return filesearchservice.StreamFacts(ctx, kind, boundary, func(k protocol.Kind, payload []byte) error {
				var header [8]byte
				binary.LittleEndian.PutUint32(header[:], uint32(k))
				binary.LittleEndian.PutUint32(header[4:], uint32(len(payload)))
				if _, err := file.Write(header[:]); err != nil {
					return err
				}
				if _, err := file.Write(payload); err != nil {
					return err
				}
				return emit(k, payload)
			})
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		buffer := make([]byte, protocol.MaxPayload)
		for {
			var header [8]byte
			if _, err := io.ReadFull(file, header[:]); err == io.EOF {
				return nil
			} else if err != nil {
				return err
			}
			length := binary.LittleEndian.Uint32(header[4:])
			if length > uint32(len(buffer)) {
				return fmt.Errorf("oversize frame")
			}
			payload := buffer[:length]
			if _, err := io.ReadFull(file, payload); err != nil {
				return err
			}
			if err := emit(protocol.Kind(binary.LittleEndian.Uint32(header[:])), payload); err != nil {
				return err
			}
		}
	}
	type memory struct{ Heap, WorkingSet, PrivateCommit uint64 }
	proc := syscall.NewLazyDLL("psapi.dll").NewProc("GetProcessMemoryInfo")
	sample := func() memory {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		// PROCESS_MEMORY_COUNTERS_EX uses pointer-width SIZE_T fields after its
		// two DWORDs. Private commit and resident working set measure different costs.
		var counters struct {
			Size, Faults                                                                          uint32
			PeakWS, WS, PeakPaged, Paged, PeakNonpaged, Nonpaged, Pagefile, PeakPagefile, Private uintptr
		}
		counters.Size = uint32(unsafe.Sizeof(counters))
		ok, _, callErr := proc.Call(^uintptr(0), uintptr(unsafe.Pointer(&counters)), uintptr(counters.Size))
		if ok == 0 {
			panic(callErr)
		}
		return memory{m.HeapAlloc, uint64(counters.WS), uint64(counters.Private)}
	}
	debug.FreeOSMemory()
	baseline := sample()
	peak := baseline
	var peakMu sync.Mutex
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			m := sample()
			peakMu.Lock()
			peak.Heap = max(peak.Heap, m.Heap)
			peak.WorkingSet = max(peak.WorkingSet, m.WorkingSet)
			peak.PrivateCommit = max(peak.PrivateCommit, m.PrivateCommit)
			peakMu.Unlock()
			select {
			case <-stop:
				return
			case <-ticker.C:
			}
		}
	}()
	stopSampling := sync.OnceFunc(func() { close(stop); wg.Wait() })
	defer stopSampling()
	started := time.Now()
	var ingestMillis float64
	source := ntfsSource{root: root, store: store, stream: stream, progress: func(p nativeBuildProgress) {
		if p.finalizing {
			ingestMillis = float64(time.Since(started)) / float64(time.Millisecond)
		}
	}}
	err = source.build(context.Background())
	elapsed := float64(time.Since(started)) / float64(time.Millisecond)
	postBuild := sample()
	peakMu.Lock()
	buildPeak := memory{max(peak.Heap, postBuild.Heap), max(peak.WorkingSet, postBuild.WorkingSet), max(peak.PrivateCommit, postBuild.PrivateCommit)}
	peak = postBuild
	peakMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	// Query immediately after publication, before hashing the snapshot touches
	// its mapped pages. Include this phase so a lower build peak cannot hide a
	// deferred page-fault/memory cost in the first user searches.
	index := newDirectoryIndex("")
	index.shards[root] = &directoryShard{store: store}
	queries := []string{"readme", "wox", "qqyy", "微信", "wx", "main.go", "test", "文件", "src/main", "read me", ".json", "woxzzzzmissing"}
	signatures := map[string]string{}
	queryMillis := map[string]float64{}
	for _, q := range queries {
		queryStarted := time.Now()
		results, err := index.search(context.Background(), SearchQuery{Raw: q}, 100)
		queryMillis[q] = float64(time.Since(queryStarted)) / float64(time.Millisecond)
		if err != nil {
			t.Fatal(err)
		}
		keys := make([]string, len(results))
		for i, r := range results {
			keys[i] = fmt.Sprintf("%s:%d", r.Path, r.Score)
		}
		data, _ := json.Marshal(keys)
		hash := sha256.Sum256(data)
		signatures[q] = hex.EncodeToString(hash[:])
	}
	stopSampling()
	queryPeak := peak
	var rows, names int
	var snapshotHash string
	err = store.Read(func(v column.View) error {
		rows, names = v.Base.Len(), v.Base.Names()
		// Write a second copy only after measurement, to compare every durable byte.
		path := filepath.Join(t.TempDir(), "verify.columns")
		if err := column.Write(path, v.Base); err != nil {
			return err
		}
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()
		h := sha256.New()
		if _, err := io.Copy(h, file); err != nil {
			return err
		}
		snapshotHash = hex.EncodeToString(h.Sum(nil))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	result := struct {
		Root                 string
		Rows, Names          int
		Millis, IngestMillis float64
		Baseline, Peak       memory
		SnapshotSHA256       string
		Queries              map[string]string
		PostBuild, QueryPeak memory
		QueryMillis          map[string]float64
	}{root, rows, names, elapsed, ingestMillis, baseline, buildPeak, snapshotHash, signatures, postBuild, queryPeak, queryMillis}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if path := os.Getenv("WOX_BUILD_OUTPUT"); path != "" {
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Log(string(data))
}
