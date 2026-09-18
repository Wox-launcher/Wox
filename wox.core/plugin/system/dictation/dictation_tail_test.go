package dictation

import (
	"context"
	"testing"
	"time"
	"wox/util/speech"
)

// TestTakeRecordingForOutput exercises tail ownership without opening a microphone.
func TestTakeRecordingForOutput(t *testing.T) {
	session := &speech.Session{}
	p := &DictationPlugin{session: session, isRecording: true, activeAction: dictationAction{ID: "test"}}
	started := time.Now()
	got, action, _ := p.takeRecordingForOutput(context.Background(), 20*time.Millisecond)
	if got != session || action.ID != "test" || p.session != nil || p.isRecording || p.pendingOutput != nil {
		t.Fatal("tail completion must detach the recording exactly once")
	}
	if time.Since(started) < 20*time.Millisecond {
		t.Fatal("capture tail was skipped")
	}
	p.session, p.isRecording, p.pendingOutput = session, true, session
	if got, _, _ := p.takeRecordingForOutput(context.Background(), time.Hour); got != nil {
		t.Fatal("duplicate stop must not claim the recording")
	}
	p.pendingOutput = nil
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got, _, _ := p.takeRecordingForOutput(ctx, time.Hour); got != nil || p.session != session || p.pendingOutput != nil {
		t.Fatal("canceled wait must not output or detach the recording")
	}
}

// TestTakeRecordingForOutputDoesNotTakeReplacement covers cancellation followed by a new recording.
func TestTakeRecordingForOutputDoesNotTakeReplacement(t *testing.T) {
	p := &DictationPlugin{session: &speech.Session{}, isRecording: true}
	done := make(chan *speech.Session, 1)
	go func() {
		session, _, _ := p.takeRecordingForOutput(context.Background(), 100*time.Millisecond)
		done <- session
	}()
	deadline := time.After(time.Second)
	for {
		p.sessionMu.Lock()
		claimed := p.pendingOutput != nil
		if claimed {
			p.session = &speech.Session{}
		}
		p.sessionMu.Unlock()
		if claimed {
			break
		}
		select {
		case <-deadline:
			t.Fatal("tail was not claimed")
		default:
			time.Sleep(time.Millisecond)
		}
	}
	if got := <-done; got != nil || p.session == nil || !p.isRecording {
		t.Fatal("old tail must not detach or output the replacement recording")
	}
}
