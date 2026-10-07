package locale

import "testing"

// TestCurrencyRegionFromLocales tests locale priority and region parsing.
func TestCurrencyRegionFromLocales(t *testing.T) {
	for _, tc := range []struct{ all, monetary, lang, region string }{
		{"de_DE.UTF-8", "pt_BR.UTF-8", "en_US.UTF-8", "DE"},
		{"", "pt_BR.UTF-8", "en_US.UTF-8", "BR"},
		{"", "", "pt_BR.UTF-8", "BR"},
		{"", "", "de_DE@euro", "DE"},
		{"", "", "sr_Latn_RS.UTF-8", "RS"},
		{"", "", "zh-Hans-CN", "CN"},
		{"", "", "pt_BR.UTF-8@modifier", "BR"},
		{"C", "pt_BR.UTF-8", "en_US.UTF-8", ""},
		{"C.UTF-8", "pt_BR.UTF-8", "en_US.UTF-8", ""},
		{"", "POSIX", "pt_BR.UTF-8", ""},
		{"invalid!", "pt_BR.UTF-8", "en_US.UTF-8", ""},
		{"", "en", "pt_BR.UTF-8", ""},
		{"", "", "", ""},
	} {
		if got := currencyRegionFromLocales(tc.all, tc.monetary, tc.lang); got != tc.region {
			t.Errorf("LC_ALL=%q, LC_MONETARY=%q, LANG=%q: got %q, want %q",
				tc.all, tc.monetary, tc.lang, got, tc.region)
		}
	}
}
