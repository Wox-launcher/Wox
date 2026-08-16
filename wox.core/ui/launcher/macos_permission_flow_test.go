package launcher

import (
	"testing"

	"wox/util/permission"
)

func TestMacOSPermissionFlowTitleKey(t *testing.T) {
	tests := []struct {
		permissionType string
		want           string
	}{
		{string(permission.MacOSPermissionAccessibility), "onboarding_permission_accessibility_title"},
		{string(permission.MacOSPermissionFullDiskAccess), "onboarding_permission_disk_title"},
		{"unknown", "onboarding_permissions_title"},
	}
	for _, test := range tests {
		if got := macOSPermissionFlowTitleKey(test.permissionType); got != test.want {
			t.Fatalf("title key for %q = %q, want %q", test.permissionType, got, test.want)
		}
	}
}

func TestOpenMacOSPermissionFlowRejectsInvalidType(t *testing.T) {
	app := &App{}
	if err := app.openMacOSPermissionFlow("camera"); err == nil {
		t.Fatal("invalid permission type must be rejected before opening System Settings")
	}
}
