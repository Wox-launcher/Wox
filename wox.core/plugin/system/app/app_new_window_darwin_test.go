package app

import (
	"reflect"
	"testing"
)

func TestMacNewWindowArguments(t *testing.T) {
	for _, tc := range []struct {
		id   string
		want []string
	}{
		{"org.mozilla.firefox", []string{"-n", "-a", "/Apps/Renamed.app", "--args", "--new-window", "about:blank"}},
		{"com.microsoft.VSCode", []string{"-n", "-a", "/Apps/Renamed.app", "--args", "--new-window"}},
		{"org.example.app", []string{"-n", "-a", "/Apps/Renamed.app"}},
	} {
		if got := macNewWindowArguments(appInfo{Path: "/Apps/Renamed.app", Identity: tc.id}); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: arguments=%q, want %q", tc.id, got, tc.want)
		}
	}
}
