package font

import (
	"context"
	"fmt"
	"wox/util"
	"wox/util/shell"
)

var fallbackLinuxFontFamilies = []string{
	"Noto Sans",
	"Noto Sans CJK SC",
	"DejaVu Sans",
	"Liberation Sans",
}

func getSystemFontFamilies(ctx context.Context) []string {
	output, err := shell.RunOutput("fc-list")
	if err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to get linux fonts from fc-list: %s", err.Error()))
		return fallbackLinuxFontFamilies
	}

	fontFamilies := parseFcListOutput(string(output))
	if len(fontFamilies) == 0 {
		return fallbackLinuxFontFamilies
	}

	return append(fontFamilies, fallbackLinuxFontFamilies...)
}
