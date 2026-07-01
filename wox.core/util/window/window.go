package window

import (
	"errors"
	"sort"
)

var (
	ErrWindowManagementUnsupported       = errors.New("window management is not supported on this platform")
	ErrWindowManagementPermissionDenied  = errors.New("window management permission denied")
	ErrWindowManagementWindowNotFound    = errors.New("window not found")
	ErrWindowManagementDisplayNotFound   = errors.New("display not found")
	ErrWindowManagementNoAdjacentDisplay = errors.New("no adjacent display")
)

type WindowRect struct {
	X      int
	Y      int
	Width  int
	Height int
}

type DisplayInfo struct {
	Id        string
	Bounds    WindowRect
	WorkArea  WindowRect
	IsPrimary bool
}

type ManagedWindow struct {
	Id          string
	Pid         int
	Title       string
	AppIdentity string
	Bounds      WindowRect
	Display     DisplayInfo
	IsMinimized bool
}

// SortDisplays keeps cross-display commands deterministic across platforms.
func SortDisplays(displays []DisplayInfo) {
	sort.SliceStable(displays, func(i, j int) bool {
		if displays[i].WorkArea.X == displays[j].WorkArea.X {
			return displays[i].WorkArea.Y < displays[j].WorkArea.Y
		}
		return displays[i].WorkArea.X < displays[j].WorkArea.X
	})
}
