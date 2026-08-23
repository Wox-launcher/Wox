package updater

import (
	"strings"
	"testing"
)

func TestWindowsUpdateScriptRetriesBackupCleanupBeforeLaunchingUpdatedApp(t *testing.T) {
	cleanup := strings.Index(windowsUpdateScript, "del /f /q /a")
	launch := strings.Index(windowsUpdateScript, `start "" "%TARGET%" "--updated"`)
	if cleanup < 0 || launch < 0 || cleanup > launch {
		t.Fatal("Windows update script must retry backup cleanup before launching with --updated")
	}
	if !strings.Contains(windowsUpdateScript, "for /l %%I in (1,1,10)") || strings.Contains(windowsUpdateScript, "attrib -H") {
		t.Fatal("Windows update script must retry deletion without exposing a locked backup")
	}
}
