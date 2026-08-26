package launcher

import (
	"testing"

	"wox/ui/contract"
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

func TestMacOSPermissionGrantedMatchesTargetPermission(t *testing.T) {
	status := contract.MacOSPermissionStatus{Accessibility: "granted", FullDiskAccess: "notGranted"}
	if !macOSPermissionGranted(permission.MacOSPermissionAccessibility, status) {
		t.Fatal("granted Accessibility must complete the Accessibility flow")
	}
	if macOSPermissionGranted(permission.MacOSPermissionFullDiskAccess, status) {
		t.Fatal("granted Accessibility must not complete the Full Disk Access flow")
	}

	status = contract.MacOSPermissionStatus{Accessibility: "notGranted", FullDiskAccess: "granted"}
	if !macOSPermissionGranted(permission.MacOSPermissionFullDiskAccess, status) {
		t.Fatal("granted Full Disk Access must complete the Full Disk Access flow")
	}
	if macOSPermissionGranted(permission.MacOSPermissionAccessibility, status) {
		t.Fatal("granted Full Disk Access must not complete the Accessibility flow")
	}
}
