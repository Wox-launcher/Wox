package launcher

import (
	"math"
	"strings"

	"wox/setting"
)

const launcherQueryMaxLines = 4

// launcherDensityMetrics contains the launcher-only values derived from the shared UiDensity setting.
type launcherDensityMetrics struct {
	scale               float32
	queryBoxHeight      float32
	resultRowBaseHeight float32
	toolbarHeight       float32
	refinementBarHeight float32
}

// launcherDensityMetricsFor keeps native launcher geometry aligned with Flutter's density buckets.
func launcherDensityMetricsFor(value string) launcherDensityMetrics {
	scale := float32(1)
	switch setting.NormalizeUiDensity(value) {
	case setting.UiDensityCompact:
		scale = 0.9
	case setting.UiDensityComfortable:
		scale = 1.1
	}
	metrics := launcherDensityMetrics{scale: scale}
	metrics.queryBoxHeight = metrics.scaled(55)
	metrics.resultRowBaseHeight = metrics.scaled(50)
	metrics.toolbarHeight = metrics.scaled(40)
	metrics.refinementBarHeight = metrics.scaled(44)
	return metrics
}

func (metrics launcherDensityMetrics) normalized() launcherDensityMetrics {
	if metrics.scale <= 0 {
		return launcherDensityMetricsFor("")
	}
	return metrics
}

func (metrics launcherDensityMetrics) scaled(value float32) float32 {
	scale := metrics.scale
	if scale <= 0 {
		scale = 1
	}
	return float32(math.Round(float64(value * scale)))
}

func (metrics launcherDensityMetrics) resultRowHeight(palette uiPalette) float32 {
	return metrics.resultRowBaseHeight + palette.resultItemPadding.Top + palette.resultItemPadding.Bottom
}

func (metrics launcherDensityMetrics) queryLineHeight(measuredHeight float32) float32 {
	// Native text renderers clip to the supplied line rectangle, so configured
	// fonts with a taller line box must expand it to preserve descenders.
	return max(metrics.scaled(34), float32(math.Ceil(float64(measuredHeight))))
}

func (metrics launcherDensityMetrics) queryBoxHeightForText(text string, lineHeight float32) float32 {
	return metrics.queryBoxHeight + float32(launcherQueryLineCount(text)-1)*lineHeight
}

func launcherQueryLineCount(text string) int {
	text = strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\r", "\n")
	return min(launcherQueryMaxLines, strings.Count(text, "\n")+1)
}
