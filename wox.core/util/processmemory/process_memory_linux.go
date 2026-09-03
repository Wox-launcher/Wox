//go:build linux

package processmemory

func getProcessMemoryBytes(pid int) (uint64, error) {
	// Feature split: Windows now needs a Task Manager-compatible private
	// working-set implementation. Linux keeps the existing RSS path because
	// pidusage already reads it directly from /proc without spawning tools.
	return getProcessRSSBytes(pid)
}

func getPrivateWorkingSetBreakdown(pid int) (PrivateWorkingSetBreakdown, error) {
	return PrivateWorkingSetBreakdown{}, nil
}

// Linux hosts WebKitGTK helper processes outside Wox's own subtree, so there is nothing to
// attribute per child here.
func listDescendantProcesses(pid int) ([]DescendantProcess, error) {
	return nil, nil
}
