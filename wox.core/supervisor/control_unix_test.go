//go:build !windows

package supervisor

import (
	"os"
	"testing"
)

func TestListenControlDoesNotReplaceExistingSocket(t *testing.T) {
	setupSupervisorTest(t)
	listener, err := listenControl()
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	before, err := os.Stat(controlAddress())
	if err != nil {
		t.Fatal(err)
	}
	other, err := listenControl()
	if err == nil {
		other.Close()
		t.Fatal("second listener replaced the existing socket")
	}
	after, err := os.Stat(controlAddress())
	if err != nil || !os.SameFile(before, after) {
		t.Fatalf("existing socket was changed: %v", err)
	}
}
