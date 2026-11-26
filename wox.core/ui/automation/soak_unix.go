//go:build wox_automation && (darwin || linux)

package automation

import (
	"os"
	"runtime"

	"golang.org/x/sys/unix"
)

func processSoakCounters() (int64, int, error) {
	var usage unix.Rusage
	if err := unix.Getrusage(unix.RUSAGE_SELF, &usage); err != nil {
		return 0, 0, err
	}
	path := "/proc/self/fd"
	if runtime.GOOS == "darwin" {
		path = "/dev/fd"
	}
	files, err := os.ReadDir(path)
	if err != nil {
		return 0, 0, err
	}
	return usage.Utime.Nano() + usage.Stime.Nano(), len(files) - 1, nil
}
