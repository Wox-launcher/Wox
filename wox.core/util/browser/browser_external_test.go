package browser

import "testing"

// TestOpenExternalURLRejectsUnsupportedSchemes fails before any native launch or UI-thread requirement.
func TestOpenExternalURLRejectsUnsupportedSchemes(t *testing.T) {
	for _, value := range []string{"javascript:alert(1)", "https:///missing-host", "mailto:", "https://example.com/\x00"} {
		if err := OpenExternalURL(value, 0); err == nil {
			t.Fatalf("invalid external URL %q reached native dispatch", value)
		}
	}
}

func TestParseExternalURL(t *testing.T) {
	tests := []struct {
		url   string
		valid bool
	}{
		{url: "https://woxlauncher.com", valid: true},
		{url: "https://checkout.stripe.com/c/pay/cs_live_abc#fidkUnmodified", valid: true},
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
