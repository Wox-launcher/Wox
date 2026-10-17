package shell

import (
	"strings"
	"testing"
)

func TestControlPanelTaskCanBeResolved(t *testing.T) {
	const taskGUID = "{e2394c16-f45a-496f-83cc-49e163281662}"
	found := false
	err := VisitNamespaceChildren(controlPanelAllTasksNamespace, func(parsingName string, absolutePIDL uintptr) error {
		if absolutePIDL == 0 {
			t.Fatal("expected an absolute item ID list")
		}
		if strings.Contains(strings.ToLower(parsingName), taskGUID) {
			found = true
			return errStopNamespaceVisit
		}
		return nil
	})
	if err != nil && err != errStopNamespaceVisit {
		t.Fatalf("visit control panel tasks: %v", err)
	}
	if !found {
		t.Fatal("expected the system environment variables task")
	}
}
