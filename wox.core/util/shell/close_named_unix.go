//go:build !windows

package shell

import (
	"errors"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
)

func namedProcessIDs(wanted map[string]struct{}) ([]int, error) {
	out, err := exec.Command("ps", "-ax", "-o", "pid=,comm=").Output()
	if err != nil {
		return nil, err
	}
	var ids []int
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil || !matchesProcessName(fields[1], wanted) {
			continue
		}
		ids = append(ids, pid)
	}
	return ids, nil
}

func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func requestProcessClose(pid int) {
	_ = syscall.Kill(pid, syscall.SIGTERM)
}

func forceProcessClose(pid int) {
	_ = syscall.Kill(pid, syscall.SIGKILL)
}
