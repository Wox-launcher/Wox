package screenshot

import (
	"os"

	"golang.org/x/sys/windows"
)

// newRecordingEncoderPipe reduces the number of handoffs for multi-megabyte frames.
func newRecordingEncoderPipe() (*os.File, *os.File, error) {
	var reader, writer windows.Handle
	if err := windows.CreatePipe(&reader, &writer, nil, 1<<20); err != nil {
		return os.Pipe()
	}
	return os.NewFile(uintptr(reader), "recording-input"), os.NewFile(uintptr(writer), "recording-output"), nil
}
