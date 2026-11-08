package interactive

import "golang.org/x/sys/unix"

// unixProcessIDs enumerates PID entries so teardown can find jobs in other process groups.
func unixProcessIDs() []int {
	processes, _ := unix.SysctlKinfoProcSlice("kern.proc.all")
	pids := make([]int, 0, len(processes))
	for _, process := range processes {
		if pid := int(process.Proc.P_pid); pid > 0 {
			pids = append(pids, pid)
		}
	}
	return pids
}
