package font

import (
	"context"
	"fmt"
	"wox/util"
	"wox/util/shell"
)

var fallbackWindowsFontFamilies = []string{
	"Segoe UI",
	"Microsoft YaHei UI",
	"Arial",
}

func getSystemFontFamilies(ctx context.Context) []string {
	output, err := shell.RunOutput("reg", "query", `HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Fonts`)
	if err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to get windows fonts from registry: %s", err.Error()))
		return fallbackWindowsFontFamilies
	}

	fontFamilies := parseWindowsRegFontsOutput(string(output))
	if len(fontFamilies) == 0 {
		return fallbackWindowsFontFamilies
	}

	return append(fontFamilies, fallbackWindowsFontFamilies...)
}
