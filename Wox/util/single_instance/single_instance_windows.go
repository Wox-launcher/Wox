//go:build windows

package single_instance

import (
	"os"
	"wox/util"
)

func lock(content string) error {
	filename := util.GetLocation().GetAppLockFilePath()
	if _, err := os.Stat(filename); err == nil {
		// If the files exists, we first try to remove it
		if err = os.Remove(filename); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	file, err := os.OpenFile(filename, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}

	_, err = file.WriteString(content)
	if err != nil {
		return err
	}

	return nil
}
