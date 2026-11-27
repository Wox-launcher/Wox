package manager

import (
	"testing"

	"wox/util/filesearchservice"
)

func TestNotInstalledStatusNeverHidesAQueryFailure(t *testing.T) {
	status := GetStatus()
	t.Logf("file index service status: state=%s detail=%q", status.State, status.Detail)
	if status.State == filesearchservice.StateNotInstalled && status.Detail != "" {
		t.Fatalf("SCM query failure was reported as not installed: %s", status.Detail)
	}
}
