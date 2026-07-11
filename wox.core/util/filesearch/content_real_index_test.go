//go:build filesearch_real_index

package filesearch

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
	"wox/util"
)

const (
	contentRealIndexTimeout       = 30 * time.Minute
	contentRealIndexSearchTimeout = time.Minute
	contentRealIndexSearchLimit   = 1000
	contentRealIndexPreviewLimit  = 20
)

var (
	contentRealIndexCaptureFlag      = flag.Bool("filesearch-content-real-index", false, "capture a real filesearch content index baseline")
	contentRealIndexArtifactPathFlag = flag.String("filesearch-content-real-index-artifact", "", "write the content index baseline artifact to this path")
	contentRealIndexRootPathFlag     = flag.String("filesearch-content-real-index-root", "", "root path to capture; defaults to the name-index real root")
	contentRealIndexKeywordFlag      = flag.String("filesearch-content-real-index-keyword", "", "content keyword to query after indexing")
	contentRealIndexMaxReadBytesFlag = flag.Int64("filesearch-content-real-index-max-read-bytes", ContentDefaultMaxReadBytes, "maximum extracted text bytes per file")
)

type contentRealIndexArtifact struct {
	CapturedAt       string                         `json:"captured_at"`
	GoGCFlags        string                         `json:"go_gcflags,omitempty"`
	RootPath         string                         `json:"root_path"`
	Keyword          string                         `json:"keyword"`
	Extensions       []string                       `json:"extensions"`
	MaxReadBytes     int64                          `json:"max_read_bytes"`
	InitialCrawl     contentRealIndexCrawlMetric    `json:"initial_crawl"`
	UnchangedRecrawl contentRealIndexCrawlMetric    `json:"unchanged_recrawl"`
	Search           contentRealIndexSearchMetric   `json:"search"`
	RgBaseline       realIndexToolBaseline          `json:"rg_baseline"`
	Database         contentRealIndexDatabaseMetric `json:"database"`
}

type contentRealIndexCrawlMetric struct {
	StartedAt          string  `json:"started_at"`
	ElapsedMillis      int64   `json:"elapsed_millis"`
	ProcessedFiles     int     `json:"processed_files"`
	StoredDocuments    int     `json:"stored_documents"`
	IndexedTextBytes   int64   `json:"indexed_text_bytes"`
	ProcessedTextBytes int64   `json:"processed_text_bytes"`
	DocumentsPerSecond float64 `json:"documents_per_second"`
	TextMiBPerSecond   float64 `json:"text_mib_per_second"`
	AllocatedBytes     uint64  `json:"allocated_bytes"`
	AllocationCount    uint64  `json:"allocation_count"`
	Complete           bool    `json:"complete"`
	Error              string  `json:"error,omitempty"`
}

type contentRealIndexSearchMetric struct {
	Keyword       string                         `json:"keyword"`
	Limit         int                            `json:"limit"`
	ElapsedMicros int64                          `json:"elapsed_micros"`
	ResultCount   int                            `json:"result_count"`
	Preview       []contentRealIndexSearchResult `json:"preview"`
	Error         string                         `json:"error,omitempty"`
}

type contentRealIndexSearchResult struct {
	Path  string `json:"path"`
	Score int64  `json:"score"`
}

type contentRealIndexDatabaseMetric struct {
	MainBytes  int64 `json:"main_bytes"`
	WALBytes   int64 `json:"wal_bytes"`
	SHMBytes   int64 `json:"shm_bytes"`
	TotalBytes int64 `json:"total_bytes"`
}

// TestCaptureFileSearchContentRealIndex captures an opt-in workstation-sized content indexing baseline.
func TestCaptureFileSearchContentRealIndex(t *testing.T) {
	if !*contentRealIndexCaptureFlag {
		t.Skip("run `make filesearch-content-real-index` from wox.core to capture the real content-index baseline")
	}

	rootPath := filepath.Clean(expandRealIndexRootPath(strings.TrimSpace(*contentRealIndexRootPathFlag)))
	if rootPath == "." || rootPath == "" {
		rootPath = realIndexRootPath()
	}
	rootInfo, err := os.Stat(rootPath)
	if err != nil {
		t.Skipf("skip content index capture because %q is unavailable: %v", rootPath, err)
	}
	if !rootInfo.IsDir() {
		t.Skipf("skip content index capture because %q is not a directory", rootPath)
	}
	benchmarkDataDir := t.TempDir()
	t.Setenv(util.TestWoxDataDirEnv, filepath.Join(benchmarkDataDir, "wox"))
	t.Setenv(util.TestUserDataDirEnv, filepath.Join(benchmarkDataDir, "user"))
	if err := util.GetLocation().Init(); err != nil {
		t.Fatalf("initialize isolated content benchmark location: %v", err)
	}

	maxReadBytes := *contentRealIndexMaxReadBytesFlag
	if maxReadBytes <= 0 {
		t.Fatalf("filesearch content real-index max read bytes must be positive, got %d", maxReadBytes)
	}

	db := newTestContentSearchDB(t)
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), contentRealIndexTimeout)
	defer cancel()

	root := RootRecord{ID: "content-real-index-root", Path: rootPath, Kind: RootKindUser, Status: RootStatusIdle}
	policy, _, _ := realIndexBenchmarkPolicy()
	extensionList := ContentDefaultExtensions()
	extensions := ContentExtensionsFromList(extensionList)

	initial := captureContentRealIndexCrawl(t, ctx, db, []RootRecord{root}, policy, extensions, maxReadBytes)
	unchanged := captureContentRealIndexCrawl(t, ctx, db, []RootRecord{root}, policy, extensions, maxReadBytes)
	search := captureContentRealIndexSearch(t, ctx, db, contentRealIndexKeyword())
	database := captureContentRealIndexDatabase(db.dbPath)
	rgBaseline := captureRealIndexToolBaseline(t, ctx, realIndexToolCapture{
		Tool:          "rg",
		BinaryNames:   []string{"rg"},
		EnvName:       actualIndexRgPathEnv,
		FlagValue:     actualIndexRgPathFlag,
		Args:          []string{"--files-with-matches", "--ignore-case", "--fixed-strings", "--no-messages", "--", contentRealIndexKeyword(), rootPath},
		RootPath:      rootPath,
		Mode:          "content-fixed-string",
		ResultKind:    "files",
		SearchKeyword: contentRealIndexKeyword(),
	})

	artifact := contentRealIndexArtifact{
		CapturedAt:       time.Now().UTC().Format(time.RFC3339),
		GoGCFlags:        strings.TrimSpace(os.Getenv(actualIndexGCFlagsEnv)),
		RootPath:         rootPath,
		Keyword:          contentRealIndexKeyword(),
		Extensions:       extensionList,
		MaxReadBytes:     maxReadBytes,
		InitialCrawl:     initial,
		UnchangedRecrawl: unchanged,
		Search:           search,
		RgBaseline:       rgBaseline,
		Database:         database,
	}
	writeContentRealIndexArtifact(t, artifact)
}

// captureContentRealIndexCrawl records one complete crawl without changing the crawler's production behavior.
func captureContentRealIndexCrawl(t *testing.T, ctx context.Context, db *ContentSearchDB, roots []RootRecord, policy Policy, extensions map[string]bool, maxReadBytes int64) contentRealIndexCrawlMetric {
	t.Helper()

	var before runtime.MemStats
	runtime.ReadMemStats(&before)
	metric := contentRealIndexCrawlMetric{StartedAt: time.Now().UTC().Format(time.RFC3339)}
	progress := ContentCrawlProgress{}
	crawler := NewContentCrawler(db, roots, policy, extensions, maxReadBytes, func(update ContentCrawlProgress) {
		progress = update
	})

	startedAt := time.Now()
	err := crawler.Run(ctx)
	elapsed := time.Since(startedAt)
	metric.ElapsedMillis = elapsed.Milliseconds()
	metric.ProcessedFiles = progress.FilesProcessed
	metric.Complete = progress.Complete
	if err != nil {
		metric.Error = err.Error()
	}

	stats, statsErr := db.ContentStats(ctx)
	if statsErr != nil {
		if metric.Error == "" {
			metric.Error = statsErr.Error()
		} else {
			metric.Error += "; stats: " + statsErr.Error()
		}
	}
	metric.StoredDocuments = stats.DocCount
	metric.IndexedTextBytes = stats.IndexedTextBytes
	metric.ProcessedTextBytes = progress.BytesProcessed
	seconds := elapsed.Seconds()
	if seconds > 0 {
		metric.DocumentsPerSecond = float64(metric.ProcessedFiles) / seconds
		metric.TextMiBPerSecond = float64(metric.ProcessedTextBytes) / (1024 * 1024) / seconds
	}

	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	metric.AllocatedBytes = after.TotalAlloc - before.TotalAlloc
	metric.AllocationCount = after.Mallocs - before.Mallocs

	t.Logf(
		"content crawl baseline: elapsed=%dms processed=%d stored=%d indexed_text_bytes=%d processed_text_bytes=%d docs_per_second=%.2f text_mib_per_second=%.2f allocated_bytes=%d allocations=%d complete=%t error=%q",
		metric.ElapsedMillis,
		metric.ProcessedFiles,
		metric.StoredDocuments,
		metric.IndexedTextBytes,
		metric.ProcessedTextBytes,
		metric.DocumentsPerSecond,
		metric.TextMiBPerSecond,
		metric.AllocatedBytes,
		metric.AllocationCount,
		metric.Complete,
		metric.Error,
	)
	return metric
}

// captureContentRealIndexSearch measures FTS query latency and preserves a small result preview for accuracy comparisons.
func captureContentRealIndexSearch(t *testing.T, parentCtx context.Context, db *ContentSearchDB, keyword string) contentRealIndexSearchMetric {
	t.Helper()

	metric := contentRealIndexSearchMetric{Keyword: keyword, Limit: contentRealIndexSearchLimit}
	ctx, cancel := context.WithTimeout(parentCtx, contentRealIndexSearchTimeout)
	defer cancel()
	startedAt := time.Now()
	results, err := db.SearchContent(ctx, keyword, contentRealIndexSearchLimit)
	metric.ElapsedMicros = time.Since(startedAt).Microseconds()
	if err != nil {
		metric.Error = err.Error()
	}
	metric.ResultCount = len(results)
	previewLimit := contentRealIndexPreviewLimit
	if len(results) < previewLimit {
		previewLimit = len(results)
	}
	metric.Preview = make([]contentRealIndexSearchResult, 0, previewLimit)
	for _, result := range results[:previewLimit] {
		metric.Preview = append(metric.Preview, contentRealIndexSearchResult{Path: result.Path, Score: result.Score})
	}
	t.Logf("content search baseline: keyword=%q results=%d limit=%d elapsed=%dus error=%q", keyword, metric.ResultCount, metric.Limit, metric.ElapsedMicros, metric.Error)
	return metric
}

// captureContentRealIndexDatabase records the main database and WAL sidecar footprint.
func captureContentRealIndexDatabase(dbPath string) contentRealIndexDatabaseMetric {
	metric := contentRealIndexDatabaseMetric{
		MainBytes: contentRealIndexFileSize(dbPath),
		WALBytes:  contentRealIndexFileSize(dbPath + "-wal"),
		SHMBytes:  contentRealIndexFileSize(dbPath + "-shm"),
	}
	metric.TotalBytes = metric.MainBytes + metric.WALBytes + metric.SHMBytes
	return metric
}

func contentRealIndexFileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func contentRealIndexKeyword() string {
	if keyword := strings.TrimSpace(*contentRealIndexKeywordFlag); keyword != "" {
		return keyword
	}
	return realIndexSearchKeyword()
}

// writeContentRealIndexArtifact logs the artifact and optionally persists it when an output path is requested.
func writeContentRealIndexArtifact(t *testing.T, artifact contentRealIndexArtifact) {
	t.Helper()
	payload, err := json.MarshalIndent(artifact, "", "  ")
	if err != nil {
		t.Fatalf("marshal content real-index artifact: %v", err)
	}
	artifactPath := strings.TrimSpace(*contentRealIndexArtifactPathFlag)
	if artifactPath != "" {
		if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
			t.Fatalf("create content real-index artifact directory: %v", err)
		}
		if err := os.WriteFile(artifactPath, payload, 0o644); err != nil {
			t.Fatalf("write content real-index artifact %q: %v", artifactPath, err)
		}
		t.Logf("content real-index artifact written to %s", artifactPath)
	}
	t.Log(string(payload))
}
