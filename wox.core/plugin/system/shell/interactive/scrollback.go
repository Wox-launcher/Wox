package interactive

import (
	"errors"
	"os"
	"runtime"
	"strings"
	"unicode"
)

// stageScrollback writes plain display text for the shell to print before its first prompt.
func stageScrollback(scrollback string) (string, error) {
	if scrollback == "" {
		return "", nil
	}
	text := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, scrollback)
	text = strings.TrimRight(text, "\n") + "\n"
	if runtime.GOOS == "windows" {
		text = strings.ReplaceAll(text, "\n", "\r\n")
	}
	file, err := os.CreateTemp("", "wox-shell-scrollback-*")
	if err != nil {
		return "", err
	}
	_, writeErr := file.WriteString(text)
	if err := errors.Join(writeErr, file.Close()); err != nil {
		_ = os.Remove(file.Name())
		return "", err
	}
	return file.Name(), nil
}
