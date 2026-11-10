//go:build linux && cgo

package clipboard

import (
	"bytes"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// TestDataControlSelectionTransfer exercises the same nonblocking descriptors Wayland passes to the source.
func TestDataControlSelectionTransfer(t *testing.T) {
	for _, stalled := range []bool{false, true} {
		name := "complete reader"
		if stalled {
			name = "stalled reader"
		}
		t.Run(name, func(t *testing.T) {
			reader, writer, err := os.Pipe()
			require.NoError(t, err)
			defer reader.Close()
			fd, err := unix.Dup(int(writer.Fd()))
			require.NoError(t, err)
			require.NoError(t, writer.Close())
			require.NoError(t, unix.SetNonblock(fd, true))
			payload := bytes.Repeat([]byte("Wox\x00世界\n"), 32768)
			done := make(chan struct{})
			go func() {
				serveDataControlSelection(fd, payload)
				close(done)
			}()
			if !stalled {
				actual, readErr := io.ReadAll(reader)
				require.NoError(t, readErr)
				require.Equal(t, payload, actual)
			}
			select {
			case <-done:
			case <-time.After(4 * time.Second):
				t.Fatal("clipboard transfer retained a stalled reader")
			}
		})
	}
}
