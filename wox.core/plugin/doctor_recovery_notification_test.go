package plugin

import "testing"

func TestDatabaseRecoveryManualNotificationActionRequiresPath(t *testing.T) {
	withPath := databaseRecoveryManualNotify("Recovered", "recovered.db")
	if withPath.Action == nil || withPath.Action.Action == nil {
		t.Fatal("recovery path needs an action")
	}
	withoutPath := databaseRecoveryManualNotify("Recovery failed", "")
	if withoutPath.Action != nil {
		t.Fatal("empty recovery path must not expose an action")
	}
}
