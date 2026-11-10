//go:build linux

package screen

import (
	"bufio"
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

// KWinDisplay describes compositor logical geometry separately from native capture pixels.
type KWinDisplay struct {
	Name        string
	Logical     Rect
	PixelWidth  int
	PixelHeight int
}

// GetKWinActiveDisplay resolves the compositor's active output without consulting XWayland.
func GetKWinActiveDisplay() (KWinDisplay, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return KWinDisplay{}, err
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	object := conn.Object("org.kde.KWin", dbus.ObjectPath("/KWin"))
	var activeName string
	if err := object.CallWithContext(ctx, "org.kde.KWin.activeOutputName", 0).Store(&activeName); err != nil {
		return KWinDisplay{}, fmt.Errorf("query KWin active output: %w", err)
	}
	displays, err := readKWinDisplays(ctx, conn)
	if err != nil {
		return KWinDisplay{}, err
	}
	for _, display := range displays {
		if display.Name == activeName {
			return display, nil
		}
	}
	return KWinDisplay{}, fmt.Errorf("KWin active output %q has no geometry", activeName)
}

// GetKWinDisplays freezes every enabled output's logical geometry and native pixel size.
func GetKWinDisplays() ([]KWinDisplay, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	return readKWinDisplays(ctx, conn)
}

// readKWinDisplays shares output decoding between launcher placement and desktop capture.
func readKWinDisplays(ctx context.Context, conn *dbus.Conn) ([]KWinDisplay, error) {
	var support string
	if err := conn.Object("org.kde.KWin", "/KWin").CallWithContext(ctx, "org.kde.KWin.supportInformation", 0).Store(&support); err != nil {
		return nil, fmt.Errorf("query KWin output geometry: %w", err)
	}
	displays := parseKWinDisplays(support)
	if len(displays) == 0 {
		return nil, fmt.Errorf("KWin has no enabled outputs")
	}
	return displays, nil
}

// parseKWinDisplays reads logical geometry and fractional scale from KWin's output diagnostics.
func parseKWinDisplays(support string) []KWinDisplay {
	var displays []KWinDisplay
	var name string
	var logical Rect
	var scale float64
	enabled := true
	inScreen := false
	flush := func() {
		if !enabled || name == "" || logical.Width <= 0 || logical.Height <= 0 || scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
			return
		}
		displays = append(displays, KWinDisplay{
			Name: name, Logical: logical,
			PixelWidth: int(math.Round(float64(logical.Width) * scale)), PixelHeight: int(math.Round(float64(logical.Height) * scale)),
		})
	}
	scanner := bufio.NewScanner(strings.NewReader(support))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "Screen ") && strings.HasSuffix(line, ":") {
			flush()
			inScreen = true
			name = ""
			logical = Rect{}
			scale = 0
			enabled = true
			continue
		}
		if !inScreen {
			continue
		}
		if name == "" && strings.HasPrefix(line, "Name: ") {
			name = strings.TrimSpace(strings.TrimPrefix(line, "Name: "))
			continue
		}
		if strings.HasPrefix(line, "Enabled: ") {
			enabled = strings.TrimSpace(strings.TrimPrefix(line, "Enabled: ")) != "0"
			continue
		}
		if strings.HasPrefix(line, "Geometry: ") {
			_, _ = fmt.Sscanf(strings.TrimPrefix(line, "Geometry: "), "%d,%d,%dx%d", &logical.X, &logical.Y, &logical.Width, &logical.Height)
			continue
		}
		if strings.HasPrefix(line, "Scale: ") {
			_, _ = fmt.Sscanf(strings.TrimPrefix(line, "Scale: "), "%f", &scale)
			flush()
			inScreen = false
			name = ""
			logical = Rect{}
			scale = 0
		}
	}
	return displays
}
