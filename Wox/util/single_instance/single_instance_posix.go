//go:build !windows

package single_instance

import (
	"os"
	"syscall"
	"wox/util"
)

// CreateLockFile tries to create a file with given name and acquire an
// exclusive lock on it. If the file already exists AND is still locked, it will
// fail.
func lock(content string) error {
	filename := util.GetLocation().GetAppLockFilePath()
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}

	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		file.Close()
		return err
	}

	if err := file.Truncate(0); err != nil {
		file.Close()
		return err
	}
	if _, err := file.WriteString(content); err != nil {
		file.Close()
		return err
	}

	return nil
}
