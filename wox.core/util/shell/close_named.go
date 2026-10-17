package shell

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"
)

const closeNamedSettle = 300 * time.Millisecond

// CloseNamedProcesses asks every process with one of these executable names to exit.
// A nil error means none of them are still running. Names are matched case-insensitively
// against the file name, such as Raycast.exe.
func CloseNamedProcesses(ctx context.Context, names ...string) error {
	wanted := map[string]struct{}{}
	for _, name := range names {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "" {
			continue
		}
		wanted[name] = struct{}{}
	}
	if len(wanted) == 0 {
		return nil
	}
	pids, err := namedProcessIDs(wanted)
	if err != nil {
		return err
	}
	return closeProcessIDs(ctx, pids)
}

func matchesProcessName(fileName string, wanted map[string]struct{}) bool {
	fileName = strings.ToLower(strings.TrimSpace(fileName))
	if fileName == "" {
		return false
	}
	_, ok := wanted[fileName]
	return ok
}

// closeProcessIDs tries a normal quit, then terminates whatever is still alive.
// The short settle gives the OS time to drop global hotkeys the process owned.
func closeProcessIDs(ctx context.Context, pids []int) error {
	pids = liveProcessIDs(pids)
	if len(pids) == 0 {
		return nil
	}
	for _, pid := range pids {
		requestProcessClose(pid)
	}
	if err := waitProcessesGone(ctx, pids, 3*time.Second); err != nil {
		return err
	}
	if !anyProcessAlive(pids) {
		return settleClosedProcess(ctx)
	}
	for _, pid := range pids {
		if processAlive(pid) {
			forceProcessClose(pid)
		}
	}
	if err := waitProcessesGone(ctx, pids, 3*time.Second); err != nil {
		return err
	}
	if anyProcessAlive(pids) {
		return errors.New("launcher process is still running")
	}
	return settleClosedProcess(ctx)
}

func liveProcessIDs(pids []int) []int {
	self := os.Getpid()
	seen := map[int]struct{}{}
	live := make([]int, 0, len(pids))
	for _, pid := range pids {
		if pid <= 0 || pid == self {
			continue
		}
		if _, ok := seen[pid]; ok || !processAlive(pid) {
			continue
		}
		seen[pid] = struct{}{}
		live = append(live, pid)
	}
	return live
}

func anyProcessAlive(pids []int) bool {
	for _, pid := range pids {
		if processAlive(pid) {
			return true
		}
	}
	return false
}

func waitProcessesGone(ctx context.Context, pids []int, limit time.Duration) error {
	deadline := time.Now().Add(limit)
	for anyProcessAlive(pids) {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if time.Now().After(deadline) {
			return nil
		}
		timer := time.NewTimer(100 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return nil
}

func settleClosedProcess(ctx context.Context) error {
	timer := time.NewTimer(closeNamedSettle)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
