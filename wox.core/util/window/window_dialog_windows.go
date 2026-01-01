//go:build windows

package window

func NavigateActiveFileDialog(targetPath string) bool {
	return NavigateActiveFileExplorer(targetPath)
}
