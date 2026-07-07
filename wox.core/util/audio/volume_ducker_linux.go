package audio

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// getSystemVolume returns the current system output volume (0-100) on Linux
// using pactl (PulseAudio/PipeWire).
func getSystemVolume() (int, error) {
	out, err := exec.Command("pactl", "get-sink-volume", "@DEFAULT_SINK@").Output()
	if err != nil {
		return 0, fmt.Errorf("failed to get system volume: %w", err)
	}
	// Output looks like: "Volume: front-left: 65536 / 100% / 0.00 dB, front-right: ..."
	// Parse the first percentage.
	s := string(out)
	idx := strings.Index(s, "%")
	if idx < 0 {
		return 0, fmt.Errorf("could not parse volume from pactl output")
	}
	// Find the number before %
	start := idx - 1
	for start >= 0 && (s[start] >= '0' && s[start] <= '9') {
		start--
	}
	volStr := strings.TrimSpace(s[start+1 : idx])
	vol, err := strconv.Atoi(volStr)
	if err != nil {
		return 0, fmt.Errorf("could not parse volume percentage: %w", err)
	}
	return vol, nil
}

// setSystemVolume sets the system output volume (0-100) on Linux.
func setSystemVolume(volume int) error {
	return exec.Command("pactl", "set-sink-volume", "@DEFAULT_SINK@", fmt.Sprintf("%d%%", volume)).Run()
}
