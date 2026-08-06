package woxui

import "testing"

func TestProtocolURLHandlerDrainsURLsReceivedBeforeStartup(t *testing.T) {
	SetProtocolURLHandler(nil)
	defer SetProtocolURLHandler(nil)

	dispatchProtocolURL("wox://query?q=queued")
	var received []string
	SetProtocolURLHandler(func(rawURL string) {
		received = append(received, rawURL)
	})
	dispatchProtocolURL("wox://query?q=ready")

	if len(received) != 2 {
		t.Fatalf("received %d protocol URLs, want 2", len(received))
	}
	if received[0] != "wox://query?q=queued" || received[1] != "wox://query?q=ready" {
		t.Fatalf("received protocol URLs = %v", received)
	}
}
