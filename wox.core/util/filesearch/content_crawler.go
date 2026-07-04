package filesearch

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"wox/util"
)

const (
	contentCrawlYieldInterval = 10 * time.Millisecond
	contentCrawlFilesPerYield = 50
	contentCrawlReportEvery   = 2 * time.Second
)

// ContentCrawlProgress is reported periodically during content crawl.
type ContentCrawlProgress struct {
	FilesIndexed int
	CurrentRoot  string
	RootIndex    int
	RootTotal    int
	BytesIndexed int64
	Complete     bool
}

// ContentCrawler walks the effective roots and indexes file contents into the
// content_entries + entries_content_fts tables. It runs as a low-priority
// background goroutine, yielding frequently to avoid competing with the
// filesearch scanner or UI.
type ContentCrawler struct {
	db              *FileSearchDB
	extensions      map[string]bool
	maxReadBytes    int64
	roots           []RootRecord
	policy          Policy
	progressCB      func(ContentCrawlProgress)
}

// NewContentCrawler creates a crawler for the given DB, roots, and policy.
func NewContentCrawler(db *FileSearchDB, roots []RootRecord, policy Policy, extensions map[string]bool, maxReadBytes int64, progressCB func(ContentCrawlProgress)) *ContentCrawler {
	return &ContentCrawler{
		db:           db,
		extensions:   extensions,
		maxReadBytes: maxReadBytes,
		roots:        roots,
		policy:       policy,
		progressCB:   progressCB,
	}
}

// Run walks all roots and indexes eligible file contents. Sets crawl state
// to in_progress at start and complete at end.
func (c *ContentCrawler) Run(ctx context.Context) error {
	if c.db == nil {
		return fmt.Errorf("db not open")
	}

	if err := c.db.SetContentCrawlState(ctx, "in_progress"); err != nil {
		return fmt.Errorf("set crawl state: %w", err)
	}

	c.report(ContentCrawlProgress{RootTotal: len(c.roots)})

	fileCount := 0
	var lastYield time.Time
	lastReport := time.Now()
	rootTotal := len(c.roots)

	for rootIdx, root := range c.roots {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		c.report(ContentCrawlProgress{
			FilesIndexed: fileCount,
			CurrentRoot:  root.Path,
			RootIndex:    rootIdx,
			RootTotal:    rootTotal,
		})

		if err := c.crawlRoot(ctx, root, rootIdx, rootTotal, &fileCount, &lastYield, &lastReport); err != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("content crawl root %s failed: %v", root.Path, err))
		}
	}

	if err := c.db.SetContentCrawlState(ctx, "complete"); err != nil {
		return fmt.Errorf("set crawl complete: %w", err)
	}

	stats, _ := c.db.ContentStats(ctx)
	c.report(ContentCrawlProgress{
		FilesIndexed: fileCount,
		RootTotal:    rootTotal,
		BytesIndexed: stats.IndexedTextBytes,
		Complete:     true,
	})

	util.GetLogger().Info(ctx, fmt.Sprintf("content crawl complete: %d files indexed", fileCount))
	return nil
}

func (c *ContentCrawler) crawlRoot(ctx context.Context, root RootRecord, rootIdx, rootTotal int, fileCount *int, lastYield *time.Time, lastReport *time.Time) error {
	if c.policy.NewTraversalContext == nil {
		return fmt.Errorf("no traversal context in policy")
	}
	traversalCtx := c.policy.NewTraversalContext(root, root.Path)

	return filepath.WalkDir(root.Path, func(path string, d os.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			return nil
		}

		if !traversalCtx.ShouldIndexPath(path, d.IsDir()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			traversalCtx = traversalCtx.Descend(path)
			return nil
		}

		// Check extension whitelist.
		if !IsContentSearchableExtension(path, c.extensions) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		readBytes := c.maxReadBytes
		if info.Size() < readBytes {
			readBytes = info.Size()
		}

		text, err := readContentFile(path, readBytes)
		if err != nil {
			return nil
		}

		ext := contentNormalizeExtension(path)
		_, _ = c.db.IndexContent(ctx, path, info.ModTime().UnixMilli(), info.Size(), ext, text)
		*fileCount++

		// Report progress periodically.
		if time.Since(*lastReport) > contentCrawlReportEvery {
			stats, _ := c.db.ContentStats(ctx)
			c.report(ContentCrawlProgress{
				FilesIndexed: *fileCount,
				CurrentRoot:  root.Path,
				RootIndex:    rootIdx,
				RootTotal:    rootTotal,
				BytesIndexed: stats.IndexedTextBytes,
			})
			*lastReport = time.Now()
		}

		// Yield.
		if time.Since(*lastYield) > contentCrawlYieldInterval {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(contentCrawlYieldInterval):
			}
			*lastYield = time.Now()
		} else if *fileCount%contentCrawlFilesPerYield == 0 {
			runtime.Gosched()
		}

		return nil
	})
}

func (c *ContentCrawler) report(progress ContentCrawlProgress) {
	if c.progressCB != nil {
		c.progressCB(progress)
	}
}

func readContentFile(path string, maxBytes int64) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	limited := io.LimitReader(f, maxBytes)
	buf := make([]byte, maxBytes)
	n, err := io.ReadFull(limited, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return "", err
	}
	return string(buf[:n]), nil
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