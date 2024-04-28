//go:build windows

package single_instance

import (
	"os"
	"syscall"
	"wox/util"
)

func lock(content string) error {
	filename := util.GetLocation().GetAppLockFilePath()
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	var overlapped syscall.Overlapped
	err = syscall.LockFileEx(syscall.Handle(file.Fd()), syscall.LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, &overlapped)
	if err != nil {
		return err
	}

	if _, err := file.WriteString(content); err != nil {
		return err
	}

	return nil
}
