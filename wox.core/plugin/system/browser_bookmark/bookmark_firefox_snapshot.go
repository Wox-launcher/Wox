package browserbookmark

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

// snapshotFirefoxPlacesFile isolates SQLite recovery from Firefox's exclusive lock.
func snapshotFirefoxPlacesFile(placesFile string) (string, func(), error) {
	tempDir, err := os.MkdirTemp("", "wox-firefox-bookmarks-*")
	if err != nil {
		return "", nil, err
	}
	cleanup := func() { _ = os.RemoveAll(tempDir) }

	// WAL contains committed bookmarks that may not yet be in the main database.
	// SHM is only an index; SQLite rebuilds it for the private copy.
	suffixes := []string{"", "-wal", "-journal"}
	for attempt := range 3 {
		before := make([]os.FileInfo, len(suffixes))
		for i, suffix := range suffixes {
			before[i], err = os.Stat(placesFile + suffix)
			if os.IsNotExist(err) && suffix != "" {
				continue
			}
			if err != nil {
				cleanup()
				return "", nil, err
			}
		}

		snapshotDir := filepath.Join(tempDir, strconv.Itoa(attempt))
		if err = os.Mkdir(snapshotDir, 0o700); err != nil {
			cleanup()
			return "", nil, err
		}
		snapshotFile := filepath.Join(snapshotDir, "places.sqlite")
		stable := true
		for i, suffix := range suffixes {
			if before[i] == nil {
				continue
			}
			if err = copyFirefoxPlacesFile(placesFile+suffix, snapshotFile+suffix); err != nil {
				if os.IsNotExist(err) {
					stable = false
					break
				}
				cleanup()
				return "", nil, err
			}
		}
		if !stable {
			continue
		}

		// Retry if Firefox wrote or checkpointed while the files were copied.
		for i, suffix := range suffixes {
			after, statErr := os.Stat(placesFile + suffix)
			if before[i] == nil && os.IsNotExist(statErr) {
				continue
			}
			if statErr != nil || before[i] == nil || !os.SameFile(before[i], after) ||
				before[i].Size() != after.Size() || !before[i].ModTime().Equal(after.ModTime()) {
				stable = false
				break
			}
		}
		if stable {
			return snapshotFile, cleanup, nil
		}
	}

	cleanup()
	return "", nil, errors.New("Firefox places files changed while creating bookmark snapshot")
}

// copyFirefoxPlacesFile streams large history databases without retaining them in memory.
func copyFirefoxPlacesFile(sourcePath, destinationPath string) error {
	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()
	destination, err := os.OpenFile(destinationPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(destination, source)
	closeErr := destination.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}
