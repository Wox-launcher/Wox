//go:build windows

package screen

import (
	"strconv"
	"testing"

	"github.com/lxn/win"
)

// TestWindowsScreenIdentity checks that native screen lookups use the enumeration's monitor IDs.
func TestWindowsScreenIdentity(t *testing.T) {
	displays, err := ListDisplays()
	if err != nil {
		t.Fatalf("list displays: %v", err)
	}
	if len(displays) == 0 {
		t.Skip("no Windows desktop displays")
	}
	for name, size := range map[string]Size{"mouse": GetMouseScreen(), "active": GetActiveScreen()} {
		found := false
		for _, display := range displays {
			if size.DisplayID == display.ID {
				found = true
				if size.X != display.WorkArea.X || size.Y != display.WorkArea.Y || size.Width != display.WorkArea.Width || size.Height != display.WorkArea.Height {
					t.Errorf("%s screen geometry %+v does not match display %+v", name, size, display)
				}
				break
			}
		}
		if !found {
			t.Errorf("%s screen ID %q missing from display snapshot", name, size.DisplayID)
		}
	}
}

// TestDisplayIDForWindow reads the monitor of an HWND without guessing from logical bounds.
func TestDisplayIDForWindow(t *testing.T) {
	if got := DisplayIDForWindow("invalid"); got != "" {
		t.Fatalf("invalid HWND resolved to %q", got)
	}
	displays, err := ListDisplays()
	if err != nil {
		t.Fatal(err)
	}
	if len(displays) == 0 {
		t.Skip("no Windows desktop displays")
	}
	id := DisplayIDForWindow(strconv.FormatUint(uint64(win.GetDesktopWindow()), 10))
	for _, display := range displays {
		if id == display.ID {
			return
		}
	}
	t.Fatalf("desktop HWND monitor %q missing from display snapshot", id)
}
