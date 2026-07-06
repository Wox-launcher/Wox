package audio

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"wox/resource"
	"wox/util"
)

// PlayEmbedded plays an embedded audio file (by name, e.g. "dictation_start.wav")
// to a temporary file and dispatches platform-native playback. It returns when
// playback has been kicked off (most platforms play asynchronously).
func PlayEmbedded(ctx context.Context, name string) error {
	data, err := resource.GetAudioFile(name)
	if err != nil {
		return fmt.Errorf("read embedded audio %s: %w", name, err)
	}

	tmpPath, err := writeTempAudio(name, data)
	if err != nil {
		return fmt.Errorf("write temp audio: %w", err)
	}

	return playFile(ctx, name, tmpPath)
}

// tempDirOnce ensures a single wox-audio temp subdir is created lazily.
var (
	tempDirOnce sync.Once
	tempDir     string
	tempDirErr  error
)

func getTempDir() (string, error) {
	tempDirOnce.Do(func() {
		dir, err := os.MkdirTemp("", "wox-audio")
		if err != nil {
			tempDirErr = err
			return
		}
		tempDir = dir
	})
	return tempDir, tempDirErr
}

// writeTempAudio writes the embedded bytes to <tmpdir>/<name>, returning the
// full path. The file is reused across calls so we don't churn the FS.
func writeTempAudio(name string, data []byte) (string, error) {
	dir, err := getTempDir()
	if err != nil {
		return "", err
	}
	full := filepath.Join(dir, name)
	// Skip rewriting if the file already exists with the same size.
	if fi, statErr := os.Stat(full); statErr == nil && int(fi.Size()) == len(data) {
		return full, nil
	}
	if err := os.WriteFile(full, data, 0644); err != nil {
		return "", err
	}
	return full, nil
}

// CleanupTempDir removes the temporary audio directory if it was created.
// Safe to call multiple times.
func CleanupTempDir() {
	if tempDir == "" {
		return
	}
	_ = os.RemoveAll(tempDir)
	tempDir = ""
}

// logErr centralizes non-fatal playback error logging.
func logErr(ctx context.Context, name string, err error) {
	util.GetLogger().Warn(ctx, fmt.Sprintf("audio play %s failed: %s", name, err.Error()))
}
