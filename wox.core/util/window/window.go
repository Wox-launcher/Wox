package window

import (
	"context"
	"errors"
	"sort"
	"time"
)

// FocusWindow returns only after the captured target is foreground. The caller
// supplies the deadline and must wait for any covering launcher to hide first.
func FocusWindow(ctx context.Context, id string, pid int) error {
	if id == "" && pid <= 0 {
		return ErrWindowManagementWindowNotFound
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return focusCapturedWindow(ctx, id, pid)
}

// waitForForeground allows an asynchronous activation to settle without a fixed delay.
func waitForForeground(ctx context.Context, matches func() bool) error {
	ticker := time.NewTicker(15 * time.Millisecond)
	defer ticker.Stop()
	for ctx.Err() == nil {
		if matches() {
			return ctx.Err()
		}
		select {
		case <-ctx.Done():
		case <-ticker.C:
		}
	}
	return ctx.Err()
}

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
