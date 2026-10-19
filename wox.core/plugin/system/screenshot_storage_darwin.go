package system

import "golang.org/x/sys/unix"

// publishScreenshotStorageFile atomically publishes a completed copy without replacing another writer's file.
func publishScreenshotStorageFile(src, dst string) error {
	return unix.RenamexNp(src, dst, unix.RENAME_EXCL)
}
