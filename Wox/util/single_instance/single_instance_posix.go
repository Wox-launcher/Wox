//go:build !windows

package single_instance

import (
	"os"
	"syscall"
	"wox/util"
)

func lock(content string) error {
	filename := util.GetLocation().GetAppLockFilePath()
	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		return err
	}

	if err := file.Truncate(0); err != nil {
		return err
	}
	if _, err := file.WriteString(content); err != nil {
		return err
	}

	return nil
}
