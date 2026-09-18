package speech

import (
	"context"
	"testing"
)

func TestExpandOfflineSegmentKeepsLeadingAudioForFirstCut(t *testing.T) {
	audio := make([]float32, 10000)
	for i := range audio {
		audio[i] = float32(i)
	}
	seg := &SpeechSegment{Start: 3000, Samples: audio[3000:7000]}

	got, end := expandOfflineSegment(audio, seg, true, 0)
	if len(got) != 7000 || got[0] != 0 || got[6999] != 6999 || end != 7000 {
		t.Fatalf("first live segment should keep the onset, len=%d first=%.0f last=%.0f end=%d", len(got), got[0], got[len(got)-1], end)
	}
	if &got[0] != &audio[0] {
		t.Fatal("segment expansion should borrow audio until queueOfflineSegment copies it")
	}
}

func TestExpandOfflineSegmentPadsLaterCutsWithoutRewindingToZero(t *testing.T) {
	audio := make([]float32, 20000)
	for i := range audio {
		audio[i] = float32(i)
	}
	start := 12000
	seg := &SpeechSegment{Start: start, Samples: audio[start : start+4000]}

	got, end := expandOfflineSegment(audio, seg, false, 0)
	wantStart := start - offlineSegmentLeadInSamples
	if len(got) != 4000+offlineSegmentLeadInSamples || got[0] != float32(wantStart) || end != start+4000 {
		t.Fatalf("later segment lead-in = len %d first %.0f end %d, want start %d", len(got), got[0], end, wantStart)
	}
}

func TestExpandOfflineSegmentDoesNotSkipAudioBetweenCuts(t *testing.T) {
	audio := make([]float32, 20000)
	for i := range audio {
		audio[i] = float32(i)
	}
	start := 12000
	seg := &SpeechSegment{Start: start, Samples: audio[start : start+4000]}
	decodedThrough := 4000

	got, end := expandOfflineSegment(audio, seg, false, decodedThrough)
	if len(got) != start+4000-decodedThrough || got[0] != float32(decodedThrough) || end != start+4000 {
		t.Fatalf("gap after previous cut should stay in the next decode, len=%d first=%.0f end=%d", len(got), got[0], end)
	}
}

func TestExpandOfflineSegmentFallsBackWhenBufferMissing(t *testing.T) {
	seg := &SpeechSegment{Start: 100, Samples: []float32{1, 2, 3}}
	got, end := expandOfflineSegment(nil, seg, true, 0)
	if len(got) != 3 || got[0] != 1 || got[2] != 3 || end != 0 {
		t.Fatalf("missing buffer should keep the VAD samples: %v end=%d", got, end)
	}
}

func TestSessionShouldRecognizeOnlyCompletedStops(t *testing.T) {
	if !sessionShouldRecognize(SessionStopReasonCompleted) || !sessionShouldRecognize(SessionStopReasonTimeout) {
		t.Fatal("completed and timeout stops should recognize")
	}
	for _, reason := range []SessionStopReason{SessionStopReasonCancelled, SessionStopReasonStartupCancelled, SessionStopReasonPluginUnload} {
		if sessionShouldRecognize(reason) {
			t.Fatalf("reason %s should only release capture and pools", reason)
		}
	}
}

func TestOfflineFullDecodeAllowedStaysInsideVadCap(t *testing.T) {
	if offlineFullDecodeAllowed(0) || !offlineFullDecodeAllowed(1) || !offlineFullDecodeAllowed(maxOfflineFullDecodeSamples) {
		t.Fatal("short non-empty audio should be allowed")
	}
	if offlineFullDecodeAllowed(maxOfflineFullDecodeSamples + 1) {
		t.Fatal("audio longer than the VAD utterance cap must not take the live full-pass path")
	}
}

func TestStopCaptureIsIdempotent(t *testing.T) {
	session := &Session{ctx: context.Background(), state: SessionStateRecording, stopped: make(chan struct{})}
	if err := session.StopCapture(); err != nil {
		t.Fatalf("first StopCapture: %v", err)
	}
	if err := session.StopCapture(); err != nil {
		t.Fatalf("second StopCapture: %v", err)
	}
	if session.state != SessionStateStopping {
		t.Fatalf("state = %v, want stopping", session.state)
	}
}
