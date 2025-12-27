package appearance

/*
#cgo LDFLAGS: -luser32 -lgdi32 -ldwmapi

#include <windows.h>

static BOOL isDark(void) {
    DWORD value = 0;
    DWORD size = sizeof(value);
    HKEY hKey;
    if (RegOpenKeyExA(HKEY_CURRENT_USER, "Software\\Microsoft\\Windows\\CurrentVersion\\Themes\\Personalize", 0, KEY_READ, &hKey) == ERROR_SUCCESS) {
        if (RegQueryValueExA(hKey, "AppsUseLightTheme", NULL, NULL, (LPBYTE)&value, &size) == ERROR_SUCCESS) {
            RegCloseKey(hKey);
            return value == 0;
        }
        RegCloseKey(hKey);
    }
    return FALSE;
}
*/
import "C"

import "time"

var (
	appearanceHandler func(bool)
	stopChan          chan struct{}
	lastIsDark        bool
	checkInterval     = time.Second
)

func isDark() bool {
	return C.isDark() != 0
}

func watchSystemAppearance(callback func(isDark bool)) {
	appearanceHandler = callback
	lastIsDark = isDark()
	stopChan = make(chan struct{})

	go func() {
		ticker := time.NewTicker(checkInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				current := isDark()
				if current != lastIsDark {
					lastIsDark = current
					if appearanceHandler != nil {
						appearanceHandler(current)
					}
				}
			case <-stopChan:
				return
			}
		}
	}()
}

func stopWatching() {
	if stopChan != nil {
		close(stopChan)
		stopChan = nil
	}
	appearanceHandler = nil
}
