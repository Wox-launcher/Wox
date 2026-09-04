package window

import (
	"path/filepath"
	"strconv"
	"strings"
)

type explorerShellWindowCandidate struct {
	index        int
	hwnd         uintptr
	path         string
	locationName string
	z            int
}

// parseWindowID converts the decimal HWND or CGWindowID captured in QueryEnv.
func parseWindowID(windowId string) uintptr {
	value, err := strconv.ParseUint(strings.TrimSpace(windowId), 10, 64)
	if err != nil {
		return 0
	}
	return uintptr(value)
}

func scoreExplorerShellWindowCandidate(candidate explorerShellWindowCandidate, windowTitle string) int {
	score := 0

	titleLower := strings.ToLower(strings.TrimSpace(windowTitle))
	loc := strings.TrimSpace(candidate.locationName)
	if loc == "" {
		loc = filepath.Base(candidate.path)
	}
	locLower := strings.ToLower(loc)
	if titleLower != "" && locLower != "" {
		if titleLower == locLower {
			score += 100
		} else if strings.Contains(titleLower, locLower) || strings.Contains(locLower, titleLower) {
			score += 50
		}
	}

	if candidate.z < (1 << 30) {
		score += 10
	}

	return score
}

func selectBestExplorerShellWindowCandidate(candidates []explorerShellWindowCandidate, preferredHwnd uintptr, windowTitle string) int {
	if len(candidates) == 0 {
		return -1
	}

	bestIdx := 0
	bestScore := -1
	for i, candidate := range candidates {
		score := scoreExplorerShellWindowCandidate(candidate, windowTitle)
		if preferredHwnd != 0 && candidate.hwnd == preferredHwnd {
			score += 1000
		}

		if score > bestScore {
			bestScore = score
			bestIdx = i
			continue
		}

		if score == bestScore && candidate.z < candidates[bestIdx].z {
			bestIdx = i
		}
	}

	return bestIdx
}

// isExplorerCabinetWindowClass is true for real Explorer/Cabinet windows only.
// SHBrowseForFolder (Move Items) can appear in Shell.Application.Windows; querying
// Document on those entries crashes explorer.exe.
func isExplorerCabinetWindowClass(className string) bool {
	switch strings.ToLower(strings.TrimSpace(className)) {
	case "cabinetwclass", "explorewclass":
		return true
	default:
		return false
	}
}

// shouldQueryExplorerShellWindowPath is true when this ShellWindows entry is the
// captured Explorer HWND. Other cabinet windows in the same explorer.exe process
// must not be queried: one of them may own a modal SHBrowseForFolder dialog, and
// Document on that window crashes explorer.exe.
func shouldQueryExplorerShellWindowPath(hwnd, preferredHwnd uintptr) bool {
	if hwnd == 0 {
		return false
	}
	if preferredHwnd == 0 {
		return true
	}
	return hwnd == preferredHwnd
}
