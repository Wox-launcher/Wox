package audio

import (
	"context"
	"fmt"
	"os/exec"
)

// playFile tries PulseAudio paplay, then alsa aplay, then oss play. Returns an
// error if no player is available. Playback is dispatched in a goroutine so it
// never blocks the caller.
func playFile(ctx context.Context, name, path string) error {
	player, args := findLinuxPlayer()
	if player == "" {
		return fmt.Errorf("no audio player found (tried paplay, aplay, play)")
	}
	go func() {
		_ = exec.Command(player, append(args, path)...).Run()
	}()
	return nil
}

// findLinuxPlayer returns the first available player binary and its arg prefix.
func findLinuxPlayer() (string, []string) {
	for _, c := range []struct {
		bin  string
		args []string
	}{
		{"paplay", nil},
		{"aplay", {"-q"}},
		{"play", nil},
	} {
		if _, err := exec.LookPath(c.bin); err == nil {
			return c.bin, c.args
		}
	}
	return "", nil
}
