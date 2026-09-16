package system

import (
	"strings"
	"testing"
	"wox/common"
)

func TestInstalledThemeListGroupOrdersSystemBelowCurrent(t *testing.T) {
	currentGroup, currentScore := installedThemeListGroup(true, true, "current", "system", "available")
	if currentGroup != "current" || currentScore != 100 {
		t.Fatalf("current system theme: group=%q score=%d", currentGroup, currentScore)
	}

	systemGroup, systemScore := installedThemeListGroup(false, true, "current", "system", "available")
	if systemGroup != "system" || systemScore != 75 {
		t.Fatalf("other system theme: group=%q score=%d", systemGroup, systemScore)
	}

	availableGroup, availableScore := installedThemeListGroup(false, false, "current", "system", "available")
	if availableGroup != "available" || availableScore != 50 {
		t.Fatalf("user theme: group=%q score=%d", availableGroup, availableScore)
	}

	if !(currentScore > systemScore && systemScore > availableScore) {
		t.Fatalf("group order scores: current=%d system=%d available=%d", currentScore, systemScore, availableScore)
	}
}

func TestThemeResultIconReusesSettingsSwatch(t *testing.T) {
	regular := common.Theme{
		ThemeId:                         "dark",
		AppBackgroundColor:              "#112233",
		QueryBoxBackgroundColor:         "#445566",
		ResultItemActiveBackgroundColor: "#778899",
	}
	icon := themeResultIcon(regular, nil)
	if icon.ImageType != common.WoxImageTypeSvg || !strings.Contains(icon.ImageData, `rx="8"`) {
		t.Fatalf("regular icon = %+v, want the Settings rounded swatch", icon)
	}

	auto := common.Theme{ThemeId: "auto", IsAutoAppearance: true, LightThemeId: "light", DarkThemeId: "dark"}
	light := common.Theme{ThemeId: "light", AppBackgroundColor: "#F5F5F5"}
	icon = themeResultIcon(auto, []common.Theme{light, regular})
	if !strings.Contains(icon.ImageData, `id="theme-auto-swatch"`) || !strings.Contains(icon.ImageData, `fill="#F5F5F5"`) || !strings.Contains(icon.ImageData, `fill="#112233"`) {
		t.Fatalf("auto icon = %s, want the Settings diagonal catalog swatch", icon.ImageData)
	}
}
