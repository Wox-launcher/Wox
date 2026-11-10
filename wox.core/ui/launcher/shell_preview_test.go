package launcher

import (
	"testing"

	woxui "wox/ui/runtime"
)

func TestShellViewportFollowsConfiguredFont(t *testing.T) {
	app := bindTestApp(&App{})
	theme := defaultPalette().componentTheme()
	for _, test := range []struct {
		font   string
		family woxui.FontFamily
	}{
		{"", woxui.FontFamilyMonospace},
		{"Pixel Operator", woxui.FontFamilyUI},
		{"", woxui.FontFamilyMonospace},
	} {
		app.generalSettings.ApplyData(settingsData{AppFontFamily: test.font})
		props, _ := app.shellViewportProps(theme, 400, 300, 1.5, 34)
		if props.Style.Family != test.family {
			t.Errorf("font %q: got family %v, want %v", test.font, props.Style.Family, test.family)
		}
	}
}

func TestShellCellMetricsFitProportionalAndWideGlyphs(t *testing.T) {
	measure := func(value string, _ woxui.TextStyle) (woxui.TextMetrics, error) {
		size := woxui.Size{Width: 8, Height: 18}
		switch value {
		case "W", "@":
			size.Width = 12
		case "界":
			size = woxui.Size{Width: 26, Height: 20}
		}
		return woxui.TextMetrics{Size: size}, nil
	}
	for _, test := range []struct {
		family        woxui.FontFamily
		width, height float32
	}{
		{woxui.FontFamilyMonospace, 8, 18},
		{woxui.FontFamilyUI, 13, 20},
	} {
		width, height := shellCellMetrics(woxui.TextStyle{Size: 13, Family: test.family}, measure)
		if width != test.width || height != test.height {
			t.Errorf("family %v: got %vx%v, want %vx%v", test.family, width, height, test.width, test.height)
		}
	}
}

func TestShellGridMetricsFollowDisplayScale(t *testing.T) {
	for _, test := range []struct {
		scale, logicalWidth float32
		pixelWidth, columns int
	}{
		{1, 8, 8, 30},
		{1.5, 8, 12, 30},
		{2, 7.5, 15, 32},
	} {
		size, width, height := shellGridMetrics(240, 180, 7.45, 18, test.scale)
		if size.Columns != test.columns || size.Rows != 10 || size.CellWidthPixels != test.pixelWidth || width != test.logicalWidth ||
			height != 18 || size.CellHeightPixels != int(18*test.scale) {
			t.Errorf("scale %v: grid %+v, logical cell %vx%v", test.scale, size, width, height)
		}
	}
}
