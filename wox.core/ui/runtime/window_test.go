package woxui

import "testing"

func TestParseExternalURL(t *testing.T) {
	tests := []struct {
		url   string
		valid bool
	}{
		{url: "https://woxlauncher.com", valid: true},
		{url: "mailto:billing@woxlauncher.com?subject=Billing+help", valid: true},
		{url: "javascript:alert(1)", valid: false},
		{url: "https:///missing-host", valid: false},
		{url: "mailto:", valid: false},
	}

	for _, test := range tests {
		t.Run(test.url, func(t *testing.T) {
			_, err := parseExternalURL(test.url)
			if (err == nil) != test.valid {
				t.Fatalf("parseExternalURL(%q) error = %v, valid = %t", test.url, err, test.valid)
			}
		})
	}
}
