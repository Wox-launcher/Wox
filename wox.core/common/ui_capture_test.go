package common

import "testing"

func TestDefaultCaptureScreenshotRequestDoesNotExposeVideoRecording(t *testing.T) {
	request := DefaultCaptureScreenshotRequest()
	if request.AllowVideoRecording {
		t.Fatal("third-party screenshot requests must remain image-only by default")
	}
}
