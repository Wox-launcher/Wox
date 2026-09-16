package window

/*
#include <windows.h>

// Compare client and monitor bounds in physical desktop pixels on the same thread.
// Client bounds exclude title bars and borders, so maximized windows do not match.
static int woxIsWindowFullscreen(HWND hwnd) {
    if (!hwnd || !IsWindowVisible(hwnd) || IsIconic(hwnd) || hwnd == GetDesktopWindow() || hwnd == GetShellWindow()) {
        return 0;
    }
    WCHAR className[256] = {0};
    GetClassNameW(hwnd, className, 256);
    if (wcscmp(className, L"Progman") == 0 || wcscmp(className, L"WorkerW") == 0 || wcscmp(className, L"MultitaskingViewFrame") == 0) {
        return 0;
    }
    typedef HANDLE (WINAPI *SetThreadDpiContext)(HANDLE);
    SetThreadDpiContext setDpiContext = (SetThreadDpiContext)GetProcAddress(GetModuleHandleW(L"user32.dll"), "SetThreadDpiAwarenessContext");
    HANDLE previous = setDpiContext ? setDpiContext((HANDLE)(LONG_PTR)-4) : NULL;
    RECT client;
    POINT origin = {0, 0};
    MONITORINFO monitor = {0};
    monitor.cbSize = sizeof(monitor);
    int fullscreen = 0;
    if (GetClientRect(hwnd, &client) && ClientToScreen(hwnd, &origin) &&
        GetMonitorInfoW(MonitorFromWindow(hwnd, MONITOR_DEFAULTTONEAREST), &monitor)) {
        fullscreen = client.right > 0 && client.bottom > 0 &&
            origin.x == monitor.rcMonitor.left && origin.y == monitor.rcMonitor.top &&
            origin.x + client.right == monitor.rcMonitor.right && origin.y + client.bottom == monitor.rcMonitor.bottom;
    }
    if (previous) {
        setDpiContext(previous);
    }
    return fullscreen;
}
*/
import "C"
import "unsafe"

func SupportsActiveWindowFullscreen() bool { return true }

func IsActiveWindowFullscreen() bool {
	return isWindowFullscreen(uintptr(unsafe.Pointer(C.GetForegroundWindow())))
}

func isWindowFullscreen(hwnd uintptr) bool {
	return C.woxIsWindowFullscreen((C.HWND)(unsafe.Pointer(hwnd))) != 0
}
