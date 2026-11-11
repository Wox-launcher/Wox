//go:build !windows

package screenshot

import "os"

func newRecordingEncoderPipe() (*os.File, *os.File, error) {
	return os.Pipe()
}
