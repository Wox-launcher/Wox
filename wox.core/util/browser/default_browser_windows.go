//go:build windows

package browser

import "golang.org/x/sys/windows/registry"

func detectDefaultBrowserID() string {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\Shell\Associations\UrlAssociations\https\UserChoice`, registry.QUERY_VALUE)
	if err != nil {
		return ""
	}
	defer key.Close()

	progID, _, err := key.GetStringValue("ProgId")
	if err != nil {
		return ""
	}
	return browserIDFromWindowsProgID(progID)
}
