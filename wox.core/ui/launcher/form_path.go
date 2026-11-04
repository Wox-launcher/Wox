package launcher

import (
	"strings"
	"unicode"
	woxui "wox/ui/runtime"
)

// formPathDialogOptions converts declared extensions into native picker patterns.
func formPathDialogOptions(definition formDefinition) woxui.FileDialogOptions {
	options := woxui.FileDialogOptions{Directory: definition.Type == "dirPath"}
	if options.Directory {
		return options
	}
	var patterns []string
	for _, extension := range definition.Value.Extensions {
		extension = strings.TrimPrefix(strings.TrimSpace(extension), ".")
		if extension == "" {
			continue
		}
		valid := true
		for _, ch := range extension {
			if !unicode.IsLetter(ch) && !unicode.IsDigit(ch) && ch != '.' && ch != '-' && ch != '_' && ch != '+' {
				valid = false
				break
			}
		}
		if valid {
			patterns = append(patterns, "*."+extension)
		}
	}
	options.Filter = strings.Join(patterns, ";")
	return options
}
