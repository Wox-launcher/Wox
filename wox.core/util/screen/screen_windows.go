package screen

/*
#include <windows.h>

typedef struct {
    int width;
    int height;
    int x;
    int y;
} ScreenInfo;

ScreenInfo getMouseScreenSize() {
    POINT pt;
    GetCursorPos(&pt);

    HMONITOR hMonitor = MonitorFromPoint(pt, MONITOR_DEFAULTTONEAREST);
    MONITORINFO mi;
    mi.cbSize = sizeof(MONITORINFO);
    if (GetMonitorInfo(hMonitor, &mi)) {
        int width = mi.rcMonitor.right - mi.rcMonitor.left;
        int height = mi.rcMonitor.bottom - mi.rcMonitor.top;
        int x = mi.rcMonitor.left;
        int y = mi.rcMonitor.top;
        return (ScreenInfo){.width = width, .height = height, .x = x, .y = y};
    }
    return (ScreenInfo){.width = 0, .height = 0, .x = 0, .y = 0};
}

ScreenInfo getActiveScreenSize() {
    HWND hWnd = GetForegroundWindow();
    if (hWnd) {
        RECT rect;
        GetWindowRect(hWnd, &rect);
        POINT pt = {rect.left + (rect.right - rect.left) / 2, rect.top + (rect.bottom - rect.top) / 2};
        HMONITOR hMonitor = MonitorFromPoint(pt, MONITOR_DEFAULTTONEAREST);
        MONITORINFO mi;
        mi.cbSize = sizeof(MONITORINFO);
        if (GetMonitorInfo(hMonitor, &mi)) {
            int width = mi.rcMonitor.right - mi.rcMonitor.left;
            int height = mi.rcMonitor.bottom - mi.rcMonitor.top;
            int x = mi.rcMonitor.left;
            int y = mi.rcMonitor.top;
            return (ScreenInfo){.width = width, .height = height, .x = x, .y = y};
        }
    }

    // Fallback to mouse screen
    return getMouseScreenSize();
}
*/
import "C"

func GetMouseScreen() Size {
	screenInfo := C.getMouseScreenSize()
	return Size{
		Width:  int(screenInfo.width),
		Height: int(screenInfo.height),
		X:      int(screenInfo.x),
		Y:      int(screenInfo.y),
	}
}

func GetActiveScreen() Size {
	screenInfo := C.getActiveScreenSize()
	return Size{
		Width:  int(screenInfo.width),
		Height: int(screenInfo.height),
		X:      int(screenInfo.x),
		Y:      int(screenInfo.y),
	}
}
