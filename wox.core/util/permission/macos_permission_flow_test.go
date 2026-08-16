package permission

import "testing"

func TestMacOSPermissionSettingsAnchor(t *testing.T) {
	if got := MacOSPermissionSettingsAnchor(MacOSPermissionAccessibility); got != "Privacy_Accessibility" {
		t.Fatalf("accessibility anchor = %q", got)
	}
	if got := MacOSPermissionSettingsAnchor(MacOSPermissionFullDiskAccess); got != "Privacy_AllFiles" {
		t.Fatalf("full disk access anchor = %q", got)
	}
	if got := MacOSPermissionSettingsAnchor("unknown"); got != "" {
		t.Fatalf("unknown anchor = %q, want empty", got)
	}
}
