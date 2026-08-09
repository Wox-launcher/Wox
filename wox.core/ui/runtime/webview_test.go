package woxui

import "testing"

func TestIsAbsoluteWebViewURL(t *testing.T) {
	for _, test := range []struct {
		value string
		valid bool
	}{
		{value: "https://example.com/path", valid: true},
		{value: "http://localhost:8080", valid: true},
		{value: "example.com/path", valid: false},
		{value: "file:///tmp/index.html", valid: false},
		{value: "https:///missing-host", valid: false},
	} {
		if actual := isAbsoluteWebViewURL(test.value); actual != test.valid {
			t.Fatalf("isAbsoluteWebViewURL(%q) = %t, want %t", test.value, actual, test.valid)
		}
	}
}
