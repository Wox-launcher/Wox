package interactive

import (
	"os"
	"strconv"
)

// unixProcessIDs enumerates PID entries so teardown can find jobs in other process groups.
func unixProcessIDs() []int {
	entries, _ := os.ReadDir("/proc")
	pids := make([]int, 0, len(entries))
	for _, entry := range entries {
		if pid, err := strconv.Atoi(entry.Name()); err == nil && pid > 0 {
			pids = append(pids, pid)
		}
	}
	return pids
}
