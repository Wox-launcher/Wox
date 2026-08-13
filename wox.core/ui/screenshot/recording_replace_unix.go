//go:build !windows

package screenshot

import "os"

func replaceRecordingFile(source, target string) error {
	return os.Rename(source, target)
}
