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
