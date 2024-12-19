package util

import (
	"os"
	"syscall"
	"time"
)

func GetFileCreatedAt(path string) string {
	stat, err := os.Stat(path)
	if err != nil {
		return "-"
	}

	statSys := stat.Sys().(*syscall.Win32FileAttributeData)
	creationTime := time.Unix(0, statSys.CreationTime.Nanoseconds())

	return FormatTime(creationTime)
}
