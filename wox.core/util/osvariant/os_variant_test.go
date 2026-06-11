package osvariant

import "testing"

func TestWindowsPlatformVariantForBuildNumber(t *testing.T) {
	tests := []struct {
		name        string
		buildNumber uint32
		want        string
	}{
		{name: "older than Windows 10 is unknown", buildNumber: 9600, want: ""},
		{name: "Windows 10 first public build", buildNumber: 10240, want: "win10"},
		{name: "Windows 10 latest range before Windows 11", buildNumber: 21999, want: "win10"},
		{name: "Windows 11 first public build", buildNumber: 22000, want: "win11"},
		{name: "Windows 11 newer build", buildNumber: 26100, want: "win11"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := windowsPlatformVariantForBuildNumber(tt.buildNumber); got != tt.want {
				t.Fatalf("windowsPlatformVariantForBuildNumber(%d) = %q, want %q", tt.buildNumber, got, tt.want)
			}
		})
	}
}
