package filesearch

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"wox/util"
)

const (
	contentCrawlBatchSize         = 128
	contentCrawlBatchMaxTextBytes = 16 * 1024 * 1024
	contentCrawlCandidatePage     = 512
	contentCrawlFilesPerYield     = 256
	contentCrawlReportEvery       = 2 * time.Second
)

// ContentCrawlProgress is reported periodically during content crawl.
type ContentCrawlProgress struct {
	FilesIndexed   int
	FilesUpdated   int
	FilesProcessed int
	FilesSkipped   int
	FilesFailed    int
	ElapsedMillis  int64
	CurrentRoot    string
	RootIndex      int
	RootTotal      int
	BytesIndexed   int64
	BytesProcessed int64
	Complete       bool
}

// ContentCrawler indexes name-index candidates into the content_entries and
// entries_content_fts tables. The filesystem walk remains only as a fallback
// for isolated callers that do not provide a name database.
type ContentCrawler struct {
	db           *ContentSearchDB
	nameDB       *FileSearchDB
	extensions   map[string]bool
	maxReadBytes int64
	roots        []RootRecord
	policy       Policy
	progressCB   func(ContentCrawlProgress)
}

// setNameDB makes the persisted name index authoritative for stale-content cleanup.
func (c *ContentCrawler) setNameDB(nameDB *FileSearchDB) {
	c.nameDB = nameDB
}

// NewContentCrawler creates a crawler for the given DB, roots, and policy.
func NewContentCrawler(db *ContentSearchDB, roots []RootRecord, policy Policy, extensions map[string]bool, maxReadBytes int64, progressCB func(ContentCrawlProgress)) *ContentCrawler {
	return &ContentCrawler{
		db:           db,
		extensions:   extensions,
		maxReadBytes: maxReadBytes,
		roots:        roots,
		policy:       policy,
		progressCB:   progressCB,
	}
}

// Run indexes eligible file contents and records the crawl lifecycle in the content database.
func (c *ContentCrawler) Run(ctx context.Context) error {
	if c.db == nil {
		return fmt.Errorf("db not open")
	}
	startedAt := time.Now()

	if err := c.db.SetContentCrawlState(ctx, "in_progress"); err != nil {
		return fmt.Errorf("set crawl state: %w", err)
	}

	c.report(ContentCrawlProgress{RootTotal: len(c.roots)})

	existing, err := c.db.ListContentEntryMetadata(ctx)
	if err != nil {
		return fmt.Errorf("load content metadata: %w", err)
	}

	counters := contentCrawlCounters{}
	seenPaths := make(map[string]struct{}, len(existing))
	lastReport := time.Now()
	rootTotal := len(c.roots)
	var firstErr error

	if c.nameDB != nil {
		firstErr = c.crawlNameIndex(ctx, existing, seenPaths, &counters, &lastReport)
	} else {
		for rootIdx, root := range c.roots {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			c.report(ContentCrawlProgress{
				FilesIndexed:   counters.indexed + counters.skipped,
				FilesUpdated:   counters.indexed,
				FilesProcessed: counters.extracted,
				FilesSkipped:   counters.skipped,
				FilesFailed:    counters.failed,
				CurrentRoot:    root.Path,
				RootIndex:      rootIdx,
				RootTotal:      rootTotal,
			})

			if err := c.crawlRoot(ctx, root, rootIdx, rootTotal, existing, seenPaths, &counters, &lastReport); err != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("content crawl root %s failed: %v", root.Path, err))
				if firstErr == nil {
					firstErr = err
				}
			}
		}
	}
	if firstErr != nil {
		return firstErr
	}
	if err := c.deleteMissingContent(ctx, existing, seenPaths); err != nil {
		return err
	}

	if err := c.db.SetContentCrawlState(ctx, "complete"); err != nil {
		return fmt.Errorf("set crawl complete: %w", err)
	}

	stats, _ := c.db.ContentStats(ctx)
	elapsedMillis := time.Since(startedAt).Milliseconds()
	c.report(ContentCrawlProgress{
		FilesIndexed:   stats.DocCount,
		FilesUpdated:   counters.indexed,
		FilesProcessed: counters.extracted,
		FilesSkipped:   counters.skipped,
		FilesFailed:    counters.failed,
		ElapsedMillis:  elapsedMillis,
		RootTotal:      rootTotal,
		BytesIndexed:   stats.IndexedTextBytes,
		BytesProcessed: counters.extractedBytes,
		Complete:       true,
	})

	filesPerSecond := float64(0)
	if elapsedMillis > 0 {
		filesPerSecond = float64(stats.DocCount) * 1000 / float64(elapsedMillis)
	}
	source := "filesystem"
	if c.nameDB != nil {
		source = "name_index"
	}
	util.GetLogger().Info(ctx, fmt.Sprintf(
		"content crawl complete: source=%s candidates=%d files=%d elapsed=%dms files_per_second=%.2f updated=%d skipped=%d failed=%d",
		source,
		counters.visited,
		stats.DocCount,
		elapsedMillis,
		filesPerSecond,
		counters.indexed,
		counters.skipped,
		counters.failed,
	))
	return nil
}

type contentCrawlCounters struct {
	visited        int
	indexed        int
	skipped        int
	failed         int
	extracted      int
	extractedBytes int64
}

// crawlNameIndex indexes only files already accepted by the authoritative name index.
func (c *ContentCrawler) crawlNameIndex(ctx context.Context, existing map[string]ContentEntryMetadata, seenPaths map[string]struct{}, counters *contentCrawlCounters, lastReport *time.Time) error {
	var afterEntryID int64
	batch := make([]ContentIndexCandidate, 0, contentCrawlBatchSize)
	batchReadBytes := int64(0)
	flushBatch := func() error {
		if len(batch) == 0 {
			return nil
		}
		documents, failed, extractedBytes, err := c.extractContentCandidateBatch(ctx, batch)
		if err != nil {
			return err
		}
		counters.failed += failed
		counters.extracted += len(documents)
		counters.extractedBytes += extractedBytes
		if len(documents) > 0 {
			updated, err := c.db.indexContentBatch(ctx, documents)
			if err != nil {
				return err
			}
			counters.indexed += updated
		}
		batch = batch[:0]
		batchReadBytes = 0
		return nil
	}

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		candidates, err := c.nameDB.ListContentIndexCandidates(ctx, afterEntryID, c.extensions, contentCrawlCandidatePage)
		if err != nil {
			return err
		}
		if len(candidates) == 0 {
			break
		}

		for _, candidate := range candidates {
			afterEntryID = candidate.EntryID
			counters.visited++
			seenPaths[candidate.Path] = struct{}{}
			if metadata, ok := existing[candidate.Path]; ok && metadata.Mtime == candidate.Mtime && metadata.Size == candidate.Size && metadata.Extension == candidate.Extension {
				counters.skipped++
				c.yieldAndReportPath("", 0, 0, counters, lastReport)
				continue
			}

			readBytes := contentExtractionMaxBytes(candidate.Path, candidate.Size, c.maxReadBytes)
			if len(batch) > 0 && (len(batch) >= contentCrawlBatchSize || batchReadBytes+readBytes > contentCrawlBatchMaxTextBytes) {
				if err := flushBatch(); err != nil {
					return err
				}
			}
			batch = append(batch, candidate)
			batchReadBytes += readBytes
			if len(batch) >= contentCrawlBatchSize || batchReadBytes >= contentCrawlBatchMaxTextBytes {
				if err := flushBatch(); err != nil {
					return err
				}
			}
			c.yieldAndReportPath("", 0, 0, counters, lastReport)
		}
	}
	return flushBatch()
}

type contentCandidateExtractionResult struct {
	text string
	err  error
}

// extractContentCandidateBatch bounds parallel file parsing while preserving candidate order for SQLite writes.
func (c *ContentCrawler) extractContentCandidateBatch(ctx context.Context, candidates []ContentIndexCandidate) ([]contentIndexDocument, int, int64, error) {
	if len(candidates) == 0 {
		return nil, 0, 0, nil
	}

	workerCount := runtime.GOMAXPROCS(0)
	if workerCount > 4 {
		workerCount = 4
	}
	if workerCount > len(candidates) {
		workerCount = len(candidates)
	}

	results := make([]contentCandidateExtractionResult, len(candidates))
	jobs := make(chan int)
	var workers sync.WaitGroup
	workers.Add(workerCount)
	for worker := 0; worker < workerCount; worker++ {
		go func() {
			defer workers.Done()
			for index := range jobs {
				if ctx.Err() != nil {
					results[index].err = ctx.Err()
					continue
				}
				candidate := candidates[index]
				readBytes := contentExtractionMaxBytes(candidate.Path, candidate.Size, c.maxReadBytes)
				results[index].text, results[index].err = extractContentText(candidate.Path, readBytes)
			}
		}()
	}

sendLoop:
	for index := range candidates {
		select {
		case jobs <- index:
		case <-ctx.Done():
			break sendLoop
		}
	}
	close(jobs)
	workers.Wait()
	if ctx.Err() != nil {
		return nil, 0, 0, ctx.Err()
	}

	documents := make([]contentIndexDocument, 0, len(candidates))
	failed := 0
	var extractedBytes int64
	for index, candidate := range candidates {
		result := results[index]
		if result.err != nil {
			failed++
			continue
		}
		documents = append(documents, contentIndexDocument{Path: candidate.Path, Mtime: candidate.Mtime, Size: candidate.Size, Extension: candidate.Extension, Text: result.text})
		extractedBytes += int64(len(result.text))
	}
	return documents, failed, extractedBytes, nil
}

func (c *ContentCrawler) crawlRoot(ctx context.Context, root RootRecord, rootIdx, rootTotal int, existing map[string]ContentEntryMetadata, seenPaths map[string]struct{}, counters *contentCrawlCounters, lastReport *time.Time) error {
	if c.policy.NewTraversalContext == nil {
		return fmt.Errorf("no traversal context in policy")
	}
	rootPath := filepath.Clean(root.Path)
	type traversalFrame struct {
		path    string
		context TraversalPolicyContext
	}
	policyStack := []traversalFrame{{path: rootPath, context: c.policy.NewTraversalContext(root, rootPath)}}
	batch := make([]contentIndexDocument, 0, contentCrawlBatchSize)
	batchTextBytes := 0
	flushBatch := func() error {
		if len(batch) == 0 {
			return nil
		}
		updated, err := c.db.indexContentBatch(ctx, batch)
		if err != nil {
			return err
		}
		counters.indexed += updated
		batch = batch[:0]
		batchTextBytes = 0
		return nil
	}

	err := filepath.WalkDir(rootPath, func(path string, d os.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return nil
		}

		if path == rootPath {
			return nil
		}
		if shouldSkipSystemPathForRoot(root, path, d.IsDir()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		parentPath := filepath.Dir(path)
		for len(policyStack) > 0 && policyStack[len(policyStack)-1].path != parentPath {
			policyStack = policyStack[:len(policyStack)-1]
		}
		var parentContext TraversalPolicyContext
		if len(policyStack) > 0 {
			parentContext = policyStack[len(policyStack)-1].context
		} else {
			parentContext = c.policy.NewTraversalContext(root, filepath.Dir(path))
		}
		if !parentContext.ShouldIndexPath(path, d.IsDir()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			policyStack = append(policyStack, traversalFrame{path: path, context: parentContext.Descend(path)})
			return nil
		}
		counters.visited++

		// Check extension whitelist.
		if !IsContentSearchableExtension(path, c.extensions) {
			c.yieldAndReport(root, rootIdx, rootTotal, counters, lastReport)
			return nil
		}
		seenPaths[path] = struct{}{}

		info, err := d.Info()
		if err != nil {
			counters.failed++
			return nil
		}
		ext := contentNormalizeExtension(path)
		if metadata, ok := existing[path]; ok && metadata.Mtime == info.ModTime().UnixMilli() && metadata.Size == info.Size() && metadata.Extension == ext {
			counters.skipped++
			c.yieldAndReport(root, rootIdx, rootTotal, counters, lastReport)
			return nil
		}

		readBytes := contentExtractionMaxBytes(path, info.Size(), c.maxReadBytes)
		text, err := extractContentText(path, readBytes)
		if err != nil {
			counters.failed++
			return nil
		}

		batch = append(batch, contentIndexDocument{Path: path, Mtime: info.ModTime().UnixMilli(), Size: info.Size(), Extension: ext, Text: text})
		batchTextBytes += len(text)
		counters.extracted++
		counters.extractedBytes += int64(len(text))
		if len(batch) >= contentCrawlBatchSize || batchTextBytes >= contentCrawlBatchMaxTextBytes {
			if err := flushBatch(); err != nil {
				return err
			}
		}
		c.yieldAndReport(root, rootIdx, rootTotal, counters, lastReport)

		return nil
	})
	if err != nil {
		return err
	}
	return flushBatch()
}

func (c *ContentCrawler) yieldAndReport(root RootRecord, rootIdx, rootTotal int, counters *contentCrawlCounters, lastReport *time.Time) {
	c.yieldAndReportPath(root.Path, rootIdx, rootTotal, counters, lastReport)
}

func (c *ContentCrawler) yieldAndReportPath(currentRoot string, rootIdx, rootTotal int, counters *contentCrawlCounters, lastReport *time.Time) {
	if counters.visited%contentCrawlFilesPerYield == 0 {
		runtime.Gosched()
	}
	if time.Since(*lastReport) < contentCrawlReportEvery {
		return
	}
	c.report(ContentCrawlProgress{
		FilesIndexed:   counters.indexed + counters.skipped,
		FilesUpdated:   counters.indexed,
		FilesProcessed: counters.extracted,
		FilesSkipped:   counters.skipped,
		FilesFailed:    counters.failed,
		CurrentRoot:    currentRoot,
		RootIndex:      rootIdx,
		RootTotal:      rootTotal,
		BytesIndexed:   counters.extractedBytes,
		BytesProcessed: counters.extractedBytes,
	})
	*lastReport = time.Now()
}

// deleteMissingContent prunes records for files that disappeared or no longer belong to the configured roots/extensions.
func (c *ContentCrawler) deleteMissingContent(ctx context.Context, existing map[string]ContentEntryMetadata, seenPaths map[string]struct{}) error {
	if c.nameDB != nil {
		stalePaths := make([]string, 0)
		for path := range existing {
			if _, seen := seenPaths[path]; !seen {
				stalePaths = append(stalePaths, path)
			}
		}
		return c.db.DeleteContentBatch(ctx, stalePaths)
	}

	stalePaths := make([]string, 0)
	for path := range existing {
		if _, seen := seenPaths[path]; seen {
			continue
		}
		withinRoot := false
		for _, root := range c.roots {
			if pathWithinScope(root.Path, path) {
				withinRoot = true
				break
			}
		}
		if !withinRoot || !IsContentSearchableExtension(path, c.extensions) {
			stalePaths = append(stalePaths, path)
			continue
		}
		if _, err := os.Lstat(path); os.IsNotExist(err) {
			stalePaths = append(stalePaths, path)
		}
	}
	return c.db.DeleteContentBatch(ctx, stalePaths)
}

func (c *ContentCrawler) report(progress ContentCrawlProgress) {
	if c.progressCB != nil {
		c.progressCB(progress)
	}
}

// ContentCrawlStatus returns a human-readable status string for the content index.
func ContentCrawlStatus(stats ContentStats) string {
	if stats.DocCount == 0 && !stats.CrawlComplete {
		return "not indexed"
	}
	if !stats.CrawlComplete {
		return fmt.Sprintf("indexing... %d files", stats.DocCount)
	}
	return fmt.Sprintf("ready: %d files, %s indexed", stats.DocCount, formatContentBytes(stats.IndexedTextBytes))
}

func formatContentBytes(b int64) string {
	switch {
	case b >= 1024*1024:
		return fmt.Sprintf("%.1fMB", float64(b)/(1024*1024))
	case b >= 1024:
		return fmt.Sprintf("%.1fKB", float64(b)/1024)
	default:
		return fmt.Sprintf("%dB", b)
	}
}

// ContentExtensionListFromSetting parses the table JSON setting value into
// an extension set. Falls back to defaults on parse failure.
func ContentExtensionListFromSetting(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ContentDefaultExtensions()
	}

	// The table setting is stored as JSON rows: [{"Extension":"txt"},...]
	// Parse generically.
	type extRow struct {
		Extension string `json:"Extension"`
	}
	var rows []extRow
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		return ContentDefaultExtensions()
	}

	exts := make([]string, 0, len(rows))
	for _, row := range rows {
		if e := strings.TrimSpace(row.Extension); e != "" {
			exts = append(exts, e)
		}
	}
	if len(exts) == 0 {
		return ContentDefaultExtensions()
	}
	return exts
}
