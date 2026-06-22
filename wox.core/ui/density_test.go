package ui

import (
	"testing"
	"wox/setting"
)

func TestScaledDensityHeightSmoke(t *testing.T) {
	tests := []struct {
		name    string
		base    int
		density setting.UiDensity
		want    int
	}{
		{name: "compact result item", base: densityResultItemBaseHeight, density: setting.UiDensityCompact, want: 45},
		{name: "normal result item", base: densityResultItemBaseHeight, density: setting.UiDensityNormal, want: 50},
		{name: "comfortable result item", base: densityResultItemBaseHeight, density: setting.UiDensityComfortable, want: 55},
		{name: "compact query box", base: densityQueryBoxBaseHeight, density: setting.UiDensityCompact, want: 50},
		{name: "normal query box", base: densityQueryBoxBaseHeight, density: setting.UiDensityNormal, want: 55},
		{name: "comfortable query box", base: densityQueryBoxBaseHeight, density: setting.UiDensityComfortable, want: 61},
		{name: "compact toolbar", base: densityToolbarBaseHeight, density: setting.UiDensityCompact, want: 36},
		{name: "normal toolbar", base: densityToolbarBaseHeight, density: setting.UiDensityNormal, want: 40},
		{name: "comfortable toolbar", base: densityToolbarBaseHeight, density: setting.UiDensityComfortable, want: 44},
		{name: "invalid falls back to normal", base: densityResultItemBaseHeight, density: setting.UiDensity("oversized"), want: 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := scaledDensityHeight(tt.base, tt.density); got != tt.want {
				t.Fatalf("scaledDensityHeight(%d, %q) = %d, want %d", tt.base, tt.density, got, tt.want)
			}
		})
	}
}
