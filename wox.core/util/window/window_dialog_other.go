//go:build !windows && !darwin

package window

func IsOpenSaveDialog() (bool, error) {
	return false, nil
}

func NavigateActiveFileDialog(targetPath string) bool {
	return false
}
