package appearance

/*
#cgo LDFLAGS: -luser32 -lugdi -ldwmapi

#include <windows.h>
#include <winuser.h>

static void (*appearanceCallback)(bool);
static HWND g_hWnd = NULL;
static HHOOK g_hook = NULL;

typedef struct {
    DWORD cbSize;
    HWND hWnd;
    UINT uMsg;
    WPARAM wParam;
    LPARAM lParam;
    DWORD time;
    DWORD dwExtraInfo;
} KBDLLHOOKSTRUCT, *PKBDLLHOOKSTRUCT;

BOOL isDark() {
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

LRESULT CALLBACK CallWndProc(int nCode, WPARAM wParam, LPARAM lParam) {
    if (nCode == HC_ACTION) {
        if (wParam == WM_SETTINGCHANGE) {
            // Check if the setting change is related to theme
            appearanceCallback(isDark());
        }
    }
    return CallNextHookEx(NULL, nCode, wParam, lParam);
}

void watchSystemAppearance(void (*callback)(bool)) {
    appearanceCallback = callback;
}

void stopWatching() {
    appearanceCallback = NULL;
}
*/
import "C"

func isDark() bool {
	return bool(C.isDark())
}

func watchSystemAppearance(callback func(isDark bool)) {
	C.watchSystemAppearance(func(isDark C.bool) {
		callback(bool(isDark))
	})
}

func stopWatching() {
	C.stopWatching()
}
