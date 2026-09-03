package shell

import (
	"os"
	"wox/util/processmemory"
)

func terminateProcessTree(pid int) {
	descendants, err := processmemory.ListDescendantProcesses(pid)
	if err == nil {
		// The walk is breadth-first, so reversing it terminates helpers before the process that
		// spawned them. That keeps a dying parent from reparenting a child we have not reached yet.
		for i := len(descendants) - 1; i >= 0; i-- {
			terminatePID(descendants[i].ProcessID)
		}
	}
	terminatePID(pid)
}

func terminatePID(pid int) {
	process, err := os.FindProcess(pid)
	if err != nil {
		return
	}
	_ = process.Kill()
}
