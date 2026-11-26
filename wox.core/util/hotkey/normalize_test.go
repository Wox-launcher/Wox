package hotkey

import "testing"

func TestMapHotkeyBetweenPlatforms(t *testing.T) {
	cases := []struct {
		value, from, to, want string
	}{
		{"Ctrl+Shift+K", "windows", "darwin", "cmd+Shift+K"},
		{"command+option+k", "darwin", "linux", "ctrl+alt+k"},
		{"ctrl+alt+k", "linux", "windows", "ctrl+alt+k"},
		{"hold:left_ctrl+left_alt", "linux", "darwin", "hold:left_cmd+left_option"},
		{"right_cmd+right_cmd", "darwin", "windows", "right_ctrl+right_ctrl"},
		{"control+command+k", "darwin", "windows", "super+ctrl+k"},
		{"super+ctrl+k", "windows", "darwin", "ctrl+cmd+k"},
		{"", "darwin", "windows", ""},
	}
	for _, tc := range cases {
		got, err := MapHotkeyBetweenPlatforms(tc.value, tc.from, tc.to)
		if err != nil || got != tc.want {
			t.Errorf("mapping %q from %s to %s = %q, %v; want %q", tc.value, tc.from, tc.to, got, err, tc.want)
		}
	}
	for _, bad := range []string{"hold:", "ctrl+invalid-key"} {
		if got, err := MapHotkeyBetweenPlatforms(bad, "darwin", "windows"); err == nil {
			t.Errorf("mapped invalid combination %q to %q", bad, got)
		}
	}
}
