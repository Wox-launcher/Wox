package filesearch

import (
	"path/filepath"
	"sort"
)

func buildDynamicRootExclusions(roots []RootRecord) map[string][]string {
	rootByID := make(map[string]RootRecord, len(roots))
	for _, root := range roots {
		rootByID[root.ID] = root
	}

	exclusions := map[string][]string{}
	for _, root := range roots {
		if root.Kind != RootKindDynamic {
			continue
		}

		parentID := root.DynamicParentRootID
		if parentID == "" {
			if parent, ok := findNearestNonDynamicParentRoot(roots, root.Path); ok {
				parentID = parent.ID
			}
		}
		parent, ok := rootByID[parentID]
		if !ok || parent.Kind == RootKindDynamic {
			continue
		}

		// Dynamic roots split ownership from exactly one parent. The exclusion map
		// is keyed by that parent root so planner and snapshot code can remain
		// stateless per job instead of carrying a mutable "current root" setting.
		exclusions[parent.ID] = append(exclusions[parent.ID], filepath.Clean(root.Path))
	}

	return copyRootExclusions(exclusions)
}

func copyRootExclusions(exclusions map[string][]string) map[string][]string {
	copied := make(map[string][]string, len(exclusions))
	for rootID, paths := range exclusions {
		for _, path := range paths {
			if path == "" {
				continue
			}
			copied[rootID] = append(copied[rootID], filepath.Clean(path))
		}
		sort.Slice(copied[rootID], func(left int, right int) bool {
			if len(copied[rootID][left]) == len(copied[rootID][right]) {
				return copied[rootID][left] < copied[rootID][right]
			}
			return len(copied[rootID][left]) > len(copied[rootID][right])
		})
	}
	return copied
}

func findNearestNonDynamicParentRoot(roots []RootRecord, path string) (RootRecord, bool) {
	cleanPath := filepath.Clean(path)
	bestIndex := -1
	bestLength := -1
	for index, root := range roots {
		if root.Kind == RootKindDynamic {
			continue
		}
		if !pathWithinScope(root.Path, cleanPath) {
			continue
		}
		if len(filepath.Clean(root.Path)) <= bestLength {
			continue
		}
		bestIndex = index
		bestLength = len(filepath.Clean(root.Path))
	}
	if bestIndex < 0 {
		return RootRecord{}, false
	}
	return roots[bestIndex], true
}
