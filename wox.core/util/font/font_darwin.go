package font

import (
	"context"
	"fmt"
	"wox/util"
	"wox/util/shell"
)

var fallbackMacFontFamilies = []string{
	"SF Pro Text",
	"PingFang SC",
	"Helvetica Neue",
}

func getSystemFontFamilies(ctx context.Context) []string {
	output, err := shell.RunOutput("system_profiler", "SPFontsDataType", "-json")
	if err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to get macOS fonts from system_profiler: %s", err.Error()))
		return fallbackMacFontFamilies
	}

	fontFamilies := parseSystemProfilerFontsOutput(output)
	if len(fontFamilies) == 0 {
		return fallbackMacFontFamilies
	}

	return append(fontFamilies, fallbackMacFontFamilies...)
}
