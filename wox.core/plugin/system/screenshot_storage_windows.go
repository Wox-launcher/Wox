package system

import "golang.org/x/sys/windows"

// publishScreenshotStorageFile moves a completed copy atomically without replacing an existing destination.
func publishScreenshotStorageFile(src, dst string) error {
	from, err := windows.UTF16PtrFromString(src)
	if err != nil {
		return err
	}
	to, err := windows.UTF16PtrFromString(dst)
	if err != nil {
		return err
	}
	return windows.MoveFile(from, to)
}
