package screen

/*
#include <windows.h>
#include <shellscalingapi.h>

typedef struct {
    int width;
    int height;
    int x;
    int y;
} ScreenInfo;

// IMPORTANT: Understanding Physical vs Logical Coordinates in Windows
//
// Physical Coordinates (Device Pixels):
//   - The actual pixel count on the monitor hardware
//   - Example: A 4K monitor has 3840x2160 physical pixels
//
// Logical Coordinates (DPI-scaled):
//   - Coordinates adjusted for DPI scaling to maintain consistent UI sizes
//   - Example: Same 4K monitor at 200% DPI scaling = 1920x1080 logical pixels
//
// Why we need this distinction:
//   - Windows API (GetMonitorInfo, SetWindowPos) uses physical coordinates
//   - Flutter/UI frameworks typically use logical coordinates for cross-platform consistency
//   - Without conversion, a window positioned at logical (100,100) on a 200% DPI monitor
//     would appear at physical (100,100) instead of physical (200,200), causing misalignment
//
// Multi-monitor example:
//   Monitor 1 (Primary): 2560x1440 physical, 150% DPI → 1706x960 logical, offset (0,0)
//   Monitor 2 (Secondary): 1920x1080 physical, 100% DPI → 1920x1080 logical, offset (2560,360) physical or (1706,360) logical
//
//   If we want to show a window at the center of Monitor 2:
//   - Physical calculation: x = 2560 + 1920/2 = 3520, y = 360 + 1080/2 = 900
//   - Logical calculation: x = 1706 + 1920/2 = 2666, y = 360 + 1080/2 = 900
//   - We return logical coordinates to Flutter, Flutter converts back to physical using correct DPI

// Get DPI for a monitor
UINT GetDpiForMonitorCompat(HMONITOR hMonitor) {
    // Try to use GetDpiForMonitor (available on Windows 8.1+)
    HMODULE shcore = LoadLibraryA("Shcore.dll");
    if (shcore) {
        typedef HRESULT (WINAPI *GetDpiForMonitorFunc)(HMONITOR, int, UINT*, UINT*);
        GetDpiForMonitorFunc getDpiForMonitor =
            (GetDpiForMonitorFunc)GetProcAddress(shcore, "GetDpiForMonitor");

        if (getDpiForMonitor) {
            UINT dpiX = 96, dpiY = 96;
            // MDT_EFFECTIVE_DPI = 0
            if (SUCCEEDED(getDpiForMonitor(hMonitor, 0, &dpiX, &dpiY))) {
                FreeLibrary(shcore);
                return dpiX;
            }
        }
        FreeLibrary(shcore);
    }

    // Fallback to system DPI
    HDC hdc = GetDC(NULL);
    UINT dpi = GetDeviceCaps(hdc, LOGPIXELSX);
    ReleaseDC(NULL, hdc);
    return dpi;
}

ScreenInfo getMouseScreenSize() {
    POINT pt;
    GetCursorPos(&pt);  // Returns physical coordinates

    HMONITOR hMonitor = MonitorFromPoint(pt, MONITOR_DEFAULTTONEAREST);
    MONITORINFO mi;
    mi.cbSize = sizeof(MONITORINFO);
    if (GetMonitorInfo(hMonitor, &mi)) {
        // GetMonitorInfo returns physical pixel coordinates
        // Example: On a 5120x2880 monitor, rcMonitor = {0,0,5120,2880}
        int physicalWidth = mi.rcMonitor.right - mi.rcMonitor.left;
        int physicalHeight = mi.rcMonitor.bottom - mi.rcMonitor.top;
        int physicalX = mi.rcMonitor.left;
        int physicalY = mi.rcMonitor.top;

        // Get DPI for this monitor
        // Example: 216 DPI = 225% scaling (216/96 = 2.25)
        UINT dpi = GetDpiForMonitorCompat(hMonitor);
        float scale = (float)dpi / 96.0f;

        // Convert physical coordinates to logical coordinates
        // Example: 5120 physical pixels / 2.25 scale = 2275 logical pixels
        // This ensures UI elements appear the same size across different DPI monitors
        int width = (int)(physicalWidth / scale);
        int height = (int)(physicalHeight / scale);
        int x = (int)(physicalX / scale);
        int y = (int)(physicalY / scale);

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
            // Get physical dimensions
            int physicalWidth = mi.rcMonitor.right - mi.rcMonitor.left;
            int physicalHeight = mi.rcMonitor.bottom - mi.rcMonitor.top;
            int physicalX = mi.rcMonitor.left;
            int physicalY = mi.rcMonitor.top;

            // Get DPI for this monitor
            UINT dpi = GetDpiForMonitorCompat(hMonitor);
            float scale = (float)dpi / 96.0f;

            // Convert to logical coordinates
            int width = (int)(physicalWidth / scale);
            int height = (int)(physicalHeight / scale);
            int x = (int)(physicalX / scale);
            int y = (int)(physicalY / scale);

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
