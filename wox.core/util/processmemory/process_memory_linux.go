//go:build linux

package processmemory

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func getProcessMemoryBytes(pid int) (uint64, error) {
	resident, shared, err := readProcStatmPages(pid)
	if err != nil {
		return 0, err
	}
	// GNOME System Monitor's Memory column is resident minus shared pages from
	// /proc/<pid>/statm, not RSS. Shared library and file mappings stay in RAM
	// after the process exits, so counting them makes Wox look several times
	// larger than the platform monitor.
	if shared > resident {
		shared = resident
	}
	return (resident - shared) * uint64(os.Getpagesize()), nil
}

// readProcStatmPages returns the resident and shared page counts from /proc/<pid>/statm.
func readProcStatmPages(pid int) (resident, shared uint64, err error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/statm", pid))
	if err != nil {
		return 0, 0, fmt.Errorf("read statm for pid %d: %w", pid, err)
	}
	fields := strings.Fields(string(data))
	if len(fields) < 3 {
		return 0, 0, fmt.Errorf("invalid statm for pid %d: %q", pid, strings.TrimSpace(string(data)))
	}
	resident, err = strconv.ParseUint(fields[1], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid resident pages for pid %d: %w", pid, err)
	}
	shared, err = strconv.ParseUint(fields[2], 10, 64)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid shared pages for pid %d: %w", pid, err)
	}
	return resident, shared, nil
}

func getPrivateWorkingSetBreakdown(pid int) (PrivateWorkingSetBreakdown, error) {
	return PrivateWorkingSetBreakdown{}, nil
}

// Linux hosts WebKitGTK helper processes outside Wox's own subtree, so there is nothing to
// attribute per child here.
func listDescendantProcesses(pid int) ([]DescendantProcess, error) {
	return nil, nil
}
