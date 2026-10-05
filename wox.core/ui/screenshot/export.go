package screenshot

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"wox/util"
)

// reserveScreenshotExportFilePath allocates a collision-safe PNG only after image confirmation.
func reserveScreenshotExportFilePath() (string, error) {
	directory := filepath.Join(util.GetLocation().GetWoxDataDirectory(), "screenshots")
	if err := util.GetLocation().EnsureDirectoryExist(directory); err != nil {
		return "", fmt.Errorf("ensure screenshot directory: %w", err)
	}
	baseName := time.Now().Format("20060102_150405") + "_wox_snapshots"
	for suffix := 0; ; suffix++ {
		suffixText := ""
		if suffix > 0 {
			suffixText = fmt.Sprintf("_%02d", suffix)
		}
		candidate := filepath.Join(directory, baseName+suffixText+".png")
		file, err := os.OpenFile(candidate, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
		if err == nil {
			if closeErr := file.Close(); closeErr != nil {
				_ = os.Remove(candidate)
				return "", fmt.Errorf("close screenshot reservation: %w", closeErr)
			}
			return candidate, nil
		}
		if !os.IsExist(err) {
			return "", fmt.Errorf("reserve screenshot path: %w", err)
		}
	}
}
