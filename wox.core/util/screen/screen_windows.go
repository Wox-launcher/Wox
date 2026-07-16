package screen

/*
// Build fix: this package calls GetDeviceCaps in the DPI fallback path. The
// dependency used to be implicit through larger Windows builds, which was not
// enough for standalone screen probes or package-level checks, so declare gdi32
// at the package boundary that actually owns the call.
#cgo windows LDFLAGS: -lgdi32
#include <windows.h>
#include <shellscalingapi.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>

#ifndef DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2
#define DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 ((HANDLE)-4)
#endif

typedef struct {
    int width;
    int height;
    int x;
    int y;
} ScreenInfo;

typedef struct {
    char id[64];
    int x;
    int y;
    int width;
    int height;
    int workX;
    int workY;
    int workWidth;
    int workHeight;
    int pixelX;
    int pixelY;
    int pixelWidth;
    int pixelHeight;
    int pixelWorkX;
    int pixelWorkY;
    int pixelWorkWidth;
    int pixelWorkHeight;
    double scale;
    int primary;
} ScreenDisplayInfo;

// Keep the combined core and Go UI process in PerMonitorV2 coordinates. Without
// this, Windows can virtualize GetMonitorInfo/GetCursorPos in development builds
// and make negative high-DPI monitor bounds disagree with native window placement.
// The release manifest applies the same policy before Go code starts; this path
// covers go run and go test binaries that do not carry that manifest.
void enableProcessPerMonitorDpiAwareness() {
    HMODULE user32 = LoadLibraryA("user32.dll");
    if (user32) {
        typedef BOOL (WINAPI *SetProcessDpiAwarenessContextFunc)(HANDLE);
        SetProcessDpiAwarenessContextFunc setProcessDpiAwarenessContext =
            (SetProcessDpiAwarenessContextFunc)GetProcAddress(user32, "SetProcessDpiAwarenessContext");
        if (setProcessDpiAwarenessContext) {
            if (setProcessDpiAwarenessContext(DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2)) {
                FreeLibrary(user32);
                return;
            }
        }
        FreeLibrary(user32);
    }

    HMODULE shcore = LoadLibraryA("Shcore.dll");
    if (shcore) {
        typedef HRESULT (WINAPI *SetProcessDpiAwarenessFunc)(int);
        SetProcessDpiAwarenessFunc setProcessDpiAwareness =
            (SetProcessDpiAwarenessFunc)GetProcAddress(shcore, "SetProcessDpiAwareness");
        if (setProcessDpiAwareness) {
            // PROCESS_PER_MONITOR_DPI_AWARE keeps older Windows versions out of
            // system-DPI virtualization when PerMonitorV2 is unavailable.
            setProcessDpiAwareness(2);
        }
        FreeLibrary(shcore);
    }
}

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
//   - Cross-platform UI layouts use logical coordinates for consistent sizing
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
//   - We return logical coordinates and the native window converts them with the monitor DPI

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
        // Use rcWork instead of rcMonitor to exclude taskbar and other app bars
        // rcWork = the work area (available area excluding taskbar)
        // rcMonitor = the full monitor area
        int physicalWidth = mi.rcWork.right - mi.rcWork.left;
        int physicalHeight = mi.rcWork.bottom - mi.rcWork.top;
        int physicalX = mi.rcWork.left;
        int physicalY = mi.rcWork.top;

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
            // Use rcWork instead of rcMonitor to exclude taskbar and other app bars
            int physicalWidth = mi.rcWork.right - mi.rcWork.left;
            int physicalHeight = mi.rcWork.bottom - mi.rcWork.top;
            int physicalX = mi.rcWork.left;
            int physicalY = mi.rcWork.top;

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

typedef struct {
    ScreenDisplayInfo* displays;
    int maxCount;
    int count;
} MonitorEnumContext;

static int physicalToLogical(int value, float scale) {
    return (int)(value / scale);
}

static BOOL CALLBACK enumerateMonitors(HMONITOR hMonitor, HDC hdcMonitor, LPRECT lprcMonitor, LPARAM dwData) {
    MonitorEnumContext* ctx = (MonitorEnumContext*)dwData;
    if (ctx->count >= ctx->maxCount) {
        return FALSE;
    }

    MONITORINFOEXW mi;
    ZeroMemory(&mi, sizeof(mi));
    mi.cbSize = sizeof(mi);
    if (!GetMonitorInfoW(hMonitor, (MONITORINFO*)&mi)) {
        return TRUE;
    }

    UINT dpi = GetDpiForMonitorCompat(hMonitor);
    float scale = (float)dpi / 96.0f;
    if (scale <= 0.0f) {
        scale = 1.0f;
    }

    ScreenDisplayInfo info;
    ZeroMemory(&info, sizeof(info));

    int utf8Len = WideCharToMultiByte(CP_UTF8, 0, mi.szDevice, -1, info.id, sizeof(info.id), NULL, NULL);
    if (utf8Len <= 0) {
        strcpy_s(info.id, sizeof(info.id), "monitor");
    }

    info.pixelX = mi.rcMonitor.left;
    info.pixelY = mi.rcMonitor.top;
    info.pixelWidth = mi.rcMonitor.right - mi.rcMonitor.left;
    info.pixelHeight = mi.rcMonitor.bottom - mi.rcMonitor.top;
    info.pixelWorkX = mi.rcWork.left;
    info.pixelWorkY = mi.rcWork.top;
    info.pixelWorkWidth = mi.rcWork.right - mi.rcWork.left;
    info.pixelWorkHeight = mi.rcWork.bottom - mi.rcWork.top;

    info.x = physicalToLogical(info.pixelX, scale);
    info.y = physicalToLogical(info.pixelY, scale);
    info.width = physicalToLogical(info.pixelWidth, scale);
    info.height = physicalToLogical(info.pixelHeight, scale);
    info.workX = physicalToLogical(info.pixelWorkX, scale);
    info.workY = physicalToLogical(info.pixelWorkY, scale);
    info.workWidth = physicalToLogical(info.pixelWorkWidth, scale);
    info.workHeight = physicalToLogical(info.pixelWorkHeight, scale);
    info.scale = scale;
    info.primary = (mi.dwFlags & MONITORINFOF_PRIMARY) != 0 ? 1 : 0;

    ctx->displays[ctx->count++] = info;
    return TRUE;
}

int listDisplays(ScreenDisplayInfo* displays, int maxCount) {
    if (!displays || maxCount <= 0) {
        return 0;
    }

    MonitorEnumContext ctx;
    ctx.displays = displays;
    ctx.maxCount = maxCount;
    ctx.count = 0;

    EnumDisplayMonitors(NULL, NULL, enumerateMonitors, (LPARAM)&ctx);
    return ctx.count;
}
*/
import "C"
import "fmt"

const maxDisplayCount = 16

func init() {
	C.enableProcessPerMonitorDpiAwareness()
}

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

func listDisplays() ([]Display, error) {
	buffer := make([]C.ScreenDisplayInfo, maxDisplayCount)
	count := int(C.listDisplays(&buffer[0], C.int(len(buffer))))
	if count < 0 {
		return nil, fmt.Errorf("failed to enumerate displays")
	}

	displays := make([]Display, 0, count)
	for i := 0; i < count; i++ {
		info := buffer[i]
		displays = append(displays, Display{
			ID:   C.GoString(&info.id[0]),
			Name: C.GoString(&info.id[0]),
			Bounds: Rect{
				X:      int(info.x),
				Y:      int(info.y),
				Width:  int(info.width),
				Height: int(info.height),
			},
			WorkArea: Rect{
				X:      int(info.workX),
				Y:      int(info.workY),
				Width:  int(info.workWidth),
				Height: int(info.workHeight),
			},
			PixelBounds: Rect{
				X:      int(info.pixelX),
				Y:      int(info.pixelY),
				Width:  int(info.pixelWidth),
				Height: int(info.pixelHeight),
			},
			PixelWorkArea: Rect{
				X:      int(info.pixelWorkX),
				Y:      int(info.pixelWorkY),
				Width:  int(info.pixelWorkWidth),
				Height: int(info.pixelWorkHeight),
			},
			Scale:   float64(info.scale),
			Primary: int(info.primary) == 1,
		})
	}

	return displays, nil
}
