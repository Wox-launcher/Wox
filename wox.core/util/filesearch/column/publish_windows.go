//go:build windows

package column

import "golang.org/x/sys/windows"

func replaceManifest(source, destination string) (bool, error) {
	from, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return false, err
	}
	to, err := windows.UTF16PtrFromString(destination)
	if err != nil {
		return false, err
	}
	err = windows.MoveFileEx(from, to, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
	return err == nil, err
}
