package system

import "golang.org/x/sys/unix"

// publishScreenshotStorageFile atomically publishes a completed copy without replacing another writer's file.
func publishScreenshotStorageFile(src, dst string) error {
	return unix.Renameat2(unix.AT_FDCWD, src, unix.AT_FDCWD, dst, unix.RENAME_NOREPLACE)
}
