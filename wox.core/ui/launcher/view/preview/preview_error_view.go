package preview

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func previewError(message string, width, height float32, theme woxcomponent.Theme) woxwidget.Widget {
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.UniformInsets(18), Child: woxwidget.Constrained{
		FillWidth: true, FillHeight: true, Child: woxwidget.TextBlock{
			Value: message, Style: woxui.TextStyle{Size: 13}, Color: theme.ErrorText,
		},
	}}
}

// PreviewError builds the shared error state used by preview adapters.
func PreviewError(message string, width, height float32, theme woxcomponent.Theme) woxwidget.Widget {
	return previewError(message, width, height, theme)
}
