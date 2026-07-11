package audio

import (
	"context"
	"fmt"
	"wox/util"
)

// Play dispatches platform-native playback for an existing audio file path.
func Play(ctx context.Context, path string) error {
	return playFile(ctx, path)
}

// Prepare loads an audio file into the platform playback implementation so a
// later Play call can start without first-time file or decoder work.
func Prepare(ctx context.Context, path string) error {
	return prepareFile(ctx, path)
}

// logErr centralizes non-fatal playback error logging.
func logErr(ctx context.Context, path string, err error) {
	util.GetLogger().Warn(ctx, fmt.Sprintf("audio play %s failed: %s", path, err.Error()))
}
