package plugin

import (
	"strings"
	"testing"

	"wox/updater"
	"wox/util"
)

func TestWoxVersionCompatibilityInDevelopmentAndProduction(t *testing.T) {
	previous := util.ProdEnv
	t.Cleanup(func() { util.ProdEnv = previous })

	for _, test := range []struct {
		name      string
		prod      string
		minimum   string
		wantError string
	}{
		{name: "development allows future version", minimum: "9999.0.0"},
		{name: "production rejects future version", prod: "true", minimum: "9999.0.0", wantError: "requires Wox 9999.0.0 or later"},
		{name: "development rejects invalid version", minimum: "not-a-version", wantError: "invalid MinWoxVersion"},
		{name: "production rejects invalid version", prod: "true", minimum: "not-a-version", wantError: "invalid MinWoxVersion"},
		{name: "production accepts current version", prod: "true", minimum: updater.CURRENT_VERSION},
		{name: "production accepts default version", prod: "true"},
	} {
		t.Run(test.name, func(t *testing.T) {
			util.ProdEnv = test.prod
			err := ensureWoxVersionSupported("test plugin", test.minimum)
			if test.wantError == "" && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if test.wantError != "" && (err == nil || !strings.Contains(err.Error(), test.wantError)) {
				t.Fatalf("error = %v, want %q", err, test.wantError)
			}
		})
	}
}
