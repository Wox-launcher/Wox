package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
	"wox/util"
)

// reserveScreenshotExportFilePath keeps screenshot export naming in the Go layer so every caller
// shares the same woxDataDirectory policy instead of letting Flutter guess where screenshots belong.
func reserveScreenshotExportFilePath() (string, error) {
	screenshotDirectory := filepath.Join(util.GetLocation().GetWoxDataDirectory(), "screenshots")
	if err := util.GetLocation().EnsureDirectoryExist(screenshotDirectory); err != nil {
		return "", fmt.Errorf("failed to ensure screenshot directory: %w", err)
	}

	baseName := time.Now().Format("20060102_150405") + "_wox_snapshots"
	for suffix := 0; ; suffix++ {
		suffixText := ""
		if suffix > 0 {
			suffixText = fmt.Sprintf("_%02d", suffix)
		}

		candidate := filepath.Join(screenshotDirectory, baseName+suffixText+".png")
		// Pre-creating the file with O_EXCL reserves the path for this screenshot session so a
		// concurrent capture cannot race Flutter into the same timestamp-based filename.
		file, err := os.OpenFile(candidate, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			if closeErr := file.Close(); closeErr != nil {
				return "", fmt.Errorf("failed to finalize screenshot export reservation: %w", closeErr)
			}
			return candidate, nil
		}

		if os.IsExist(err) {
			continue
		}

		return "", fmt.Errorf("failed to reserve screenshot export file: %w", err)
	}
}
