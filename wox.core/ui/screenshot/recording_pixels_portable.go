//go:build !windows || !cgo

package screenshot

import "image"

func writeRecordingI420(frame *image.RGBA, output []byte) {
	writeRecordingI420Go(frame, output)
}
