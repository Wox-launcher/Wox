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

// logErr centralizes non-fatal playback error logging.
func logErr(ctx context.Context, path string, err error) {
	util.GetLogger().Warn(ctx, fmt.Sprintf("audio play %s failed: %s", path, err.Error()))
}
