#include <windows.h>
#include <windowsx.h>
#include <dwmapi.h>
#include <uxtheme.h>
#include <wingdi.h>
#include <stdlib.h>
#include <string.h>
#include <wchar.h>

#pragma comment(lib, "dwmapi.lib")
#pragma comment(lib, "uxtheme.lib")
#pragma comment(lib, "msimg32.lib")

// ============================================================================
// ACCENT_POLICY structures and functions (from notify_windows.c)
// ============================================================================

typedef enum {
    ACCENT_DISABLED = 0,
    ACCENT_ENABLE_GRADIENT = 1,
    ACCENT_ENABLE_TRANSPARENTGRADIENT = 2,
    ACCENT_ENABLE_BLURBEHIND = 3,
    ACCENT_ENABLE_ACRYLICBLURBEHIND = 4,
    ACCENT_ENABLE_HOSTBACKDROP = 5
} ACCENT_STATE;

typedef struct {
    ACCENT_STATE AccentState;
    DWORD AccentFlags;
    DWORD GradientColor;
    DWORD AnimationId;
} ACCENT_POLICY;

typedef enum {
    WCA_UNDEFINED = 0,
    WCA_NCRENDERING_ENABLED = 1,
    WCA_NCRENDERING_POLICY = 2,
    WCA_TRANSITIONS_FORCEDISABLED = 3,
    WCA_ALLOW_NCPAINT = 4,
    WCA_CAPTION_BUTTON_BOUNDS = 5,
    WCA_NONCLIENT_RTL_LAYOUT = 6,
    WCA_FORCE_ICONIC_REPRESENTATION = 7,
    WCA_EXTENDED_FRAME_BOUNDS = 8,
    WCA_HAS_ICONIC_BITMAP = 9,
    WCA_THEME_ATTRIBUTES = 10,
    WCA_NCRENDERING_EXILED = 11,
    WCA_NCADORNMENTINFO = 12,
    WCA_EXCLUDED_FROM_LIVEPREVIEW = 13,
    WCA_VIDEO_OVERLAY_ACTIVE = 14,
    WCA_FORCE_ACTIVEWINDOW_APPEARANCE = 15,
    WCA_DISALLOW_PEEK = 16,
    WCA_CLOAK = 17,
    WCA_CLOAKED = 18,
    WCA_ACCENT_POLICY = 19
} WINDOWCOMPOSITIONATTRIB;

typedef struct {
    WINDOWCOMPOSITIONATTRIB Attrib;
    PVOID pvData;
    SIZE_T cbData;
} WINDOWCOMPOSITIONATTRIBDATA;

typedef BOOL(WINAPI *pfnSetWindowCompositionAttribute)(HWND, WINDOWCOMPOSITIONATTRIBDATA *);

static BOOL TryEnableAccent(HWND hwnd, ACCENT_STATE state, DWORD gradientColor, DWORD accentFlags)
{
    HMODULE user32 = GetModuleHandleW(L"user32.dll");
    if (!user32)
        return FALSE;
    pfnSetWindowCompositionAttribute fn = (pfnSetWindowCompositionAttribute)GetProcAddress(user32, "SetWindowCompositionAttribute");
    if (!fn)
        return FALSE;

    ACCENT_POLICY policy;
    ZeroMemory(&policy, sizeof(policy));
    policy.AccentState = state;
    policy.AccentFlags = accentFlags;
    policy.GradientColor = gradientColor;

    WINDOWCOMPOSITIONATTRIBDATA data;
    data.Attrib = WCA_ACCENT_POLICY;
    data.pvData = &policy;
    data.cbData = sizeof(policy);

    return fn(hwnd, &data);
}

static BOOL TryEnableHostBackdrop(HWND hwnd)
{
    return TryEnableAccent(hwnd, ACCENT_ENABLE_HOSTBACKDROP, 0x70202020, 0);
}

static BOOL TryEnableAcrylic(HWND hwnd)
{
    return TryEnableAccent(hwnd, ACCENT_ENABLE_ACRYLICBLURBEHIND, 0x2A202020, 2);
}

// ============================================================================
// Constants
// ============================================================================

#define OVERLAY_WIDTH 400
#define OVERLAY_HEIGHT 60
#define TIMER_ID_AUTOCLOSE 1
#define ICON_PADDING 16
#define ICON_SIZE 32
#define TEXT_PADDING 12

#ifndef DWMWA_USE_IMMERSIVE_DARK_MODE
#define DWMWA_USE_IMMERSIVE_DARK_MODE 20
#endif

typedef struct {
    HWND hwnd;
    const unsigned char *bgra;
    int iconWidth;
    int iconHeight;
    HBITMAP iconBitmap;
} OverlayWindow;

// ============================================================================
// Global state
// ============================================================================

static HWND g_hintWindow = NULL;
static HWINEVENTHOOK g_hHook = NULL;
static OverlayWindow g_overlay = {0};

// ============================================================================
// External callbacks (CGO)
// ============================================================================

extern void overlayClickCallbackCGO();
extern void explorerActivationCallbackCGO(int x, int y, int width, int height);

// ============================================================================
// Helper: DPI awareness
// ============================================================================

typedef UINT(WINAPI *pfnGetDpiForWindow)(HWND);

static UINT GetWindowDpiSafe(HWND hwnd, UINT fallback)
{
    HMODULE user32 = GetModuleHandleW(L"user32.dll");
    if (!user32)
        return fallback;
    pfnGetDpiForWindow fn = (pfnGetDpiForWindow)GetProcAddress(user32, "GetDpiForWindow");
    if (!fn)
        return fallback;
    UINT dpi = fn(hwnd);
    return dpi ? dpi : fallback;
}

// ============================================================================
// Helper: Create bitmap from BGRA data
// ============================================================================

static HBITMAP CreateBitmapFromBGRA(const unsigned char *bgra, int width, int height)
{
    if (!bgra || width <= 0 || height <= 0)
        return NULL;

    HDC hdc = GetDC(NULL);
    if (!hdc)
        return NULL;

    BITMAPINFO bmi = {0};
    bmi.bmiHeader.biSize = sizeof(BITMAPINFOHEADER);
    bmi.bmiHeader.biWidth = width;
    bmi.bmiHeader.biHeight = -height;  // negative = top-down
    bmi.bmiHeader.biPlanes = 1;
    bmi.bmiHeader.biBitCount = 32;
    bmi.bmiHeader.biCompression = BI_RGB;

    HBITMAP hbitmap = CreateDIBitmap(hdc, &bmi.bmiHeader, CBM_INIT, bgra, &bmi, DIB_RGB_COLORS);
    ReleaseDC(NULL, hdc);
    return hbitmap;
}

// ============================================================================
// Helper: Check if process is explorer.exe
// ============================================================================

static BOOL IsExplorerProcess(DWORD dwProcessId)
{
    HANDLE hProcess = OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, FALSE, dwProcessId);
    if (!hProcess)
        return FALSE;

    WCHAR szImageName[MAX_PATH] = {0};
    DWORD dwLen = GetProcessImageFileNameW(hProcess, szImageName, MAX_PATH);
    CloseHandle(hProcess);

    if (dwLen == 0)
        return FALSE;

    // Check if ends with explorer.exe
    const WCHAR *fileName = wcsrchr(szImageName, L'\\');
    if (!fileName)
        fileName = szImageName;
    else
        fileName++;

    return _wcsicmp(fileName, L"explorer.exe") == 0;
}

// ============================================================================
// Window procedure
// ============================================================================

static LRESULT CALLBACK OverlayWndProc(HWND hwnd, UINT msg, WPARAM wParam, LPARAM lParam)
{
    OverlayWindow *pOverlay = NULL;

    if (msg == WM_CREATE) {
        CREATESTRUCT *pCreate = (CREATESTRUCT *)lParam;
        pOverlay = (OverlayWindow *)pCreate->lpCreateParams;
        SetWindowLongPtrW(hwnd, GWLP_USERDATA, (LONG_PTR)pOverlay);
        if (pOverlay)
            pOverlay->hwnd = hwnd;
    } else {
        pOverlay = (OverlayWindow *)GetWindowLongPtrW(hwnd, GWLP_USERDATA);
    }

    switch (msg) {
    case WM_CREATE: {
        // Set window to not accept activation
        HWND hWndInsertAfter = HWND_TOPMOST;
        SetWindowPos(hwnd, hWndInsertAfter, 0, 0, 0, 0,
                     SWP_NOSIZE | SWP_NOMOVE | SWP_NOACTIVATE);

        // Enable dark mode for title bar
        BOOL darkMode = TRUE;
        DwmSetWindowAttribute(hwnd, DWMWA_USE_IMMERSIVE_DARK_MODE, &darkMode, sizeof(darkMode));

        // Try to enable acrylic/backdrop effect
        if (!TryEnableAcrylic(hwnd)) {
            TryEnableHostBackdrop(hwnd);
        }

        // Set timer for auto-close
        SetTimer(hwnd, TIMER_ID_AUTOCLOSE, 3000, NULL);
        return 0;
    }

    case WM_TIMER: {
        if (wParam == TIMER_ID_AUTOCLOSE) {
            KillTimer(hwnd, TIMER_ID_AUTOCLOSE);
            DestroyWindow(hwnd);
        }
        return 0;
    }

    case WM_LBUTTONDOWN: {
        // Click on window
        KillTimer(hwnd, TIMER_ID_AUTOCLOSE);
        overlayClickCallbackCGO();
        DestroyWindow(hwnd);
        return 0;
    }

    case WM_PAINT: {
        PAINTSTRUCT ps;
        HDC hdc = BeginPaint(hwnd, &ps);
        if (!hdc)
            return 0;

        RECT clientRect;
        GetClientRect(hwnd, &clientRect);
        int width = clientRect.right - clientRect.left;
        int height = clientRect.bottom - clientRect.top;

        // Clear background (semi-transparent dark)
        // The DWM will handle the blur effect
        FillRect(hdc, &clientRect, (HBRUSH)GetStockObject(BLACK_BRUSH));

        // Draw icon if available
        if (pOverlay && pOverlay->iconBitmap && pOverlay->iconWidth > 0 && pOverlay->iconHeight > 0) {
            HDC hdcMem = CreateCompatibleDC(hdc);
            HGDIOBJ hOldBitmap = SelectObject(hdcMem, pOverlay->iconBitmap);

            int iconSize = ICON_SIZE;
            int iconY = (height - iconSize) / 2;
            if (iconY < 0) iconY = 0;

            StretchBlt(hdc,
                       ICON_PADDING, iconY, iconSize, iconSize,
                       hdcMem, 0, 0, pOverlay->iconWidth, pOverlay->iconHeight,
                       SRCCOPY);

            SelectObject(hdcMem, hOldBitmap);
            DeleteDC(hdcMem);
        }

        // Draw text (placeholder - real message drawing would go here)
        HFONT hFont = CreateFontW(14, 0, 0, 0, FW_NORMAL, FALSE, FALSE, FALSE,
                                  DEFAULT_CHARSET, OUT_DEFAULT_PRECIS,
                                  CLIP_DEFAULT_PRECIS, DEFAULT_QUALITY,
                                  DEFAULT_PITCH | FF_DONTCARE, L"Segoe UI");
        HGDIOBJ hOldFont = SelectObject(hdc, hFont);

        SetTextColor(hdc, RGB(255, 255, 255));
        SetBkMode(hdc, TRANSPARENT);

        int textX = ICON_PADDING + ICON_SIZE + TEXT_PADDING;
        int textY = (height - 20) / 2;
        if (textY < 0) textY = 0;

        RECT textRect = {textX, textY, width - 10, height - 5};
        DrawTextW(hdc, L"Explorer Hint", -1, &textRect, DT_LEFT | DT_VCENTER | DT_SINGLELINE);

        SelectObject(hdc, hOldFont);
        DeleteObject(hFont);

        EndPaint(hwnd, &ps);
        return 0;
    }

    case WM_DESTROY: {
        if (pOverlay) {
            if (pOverlay->iconBitmap) {
                DeleteObject(pOverlay->iconBitmap);
                pOverlay->iconBitmap = NULL;
            }
        }
        if (hwnd == g_hintWindow) {
            g_hintWindow = NULL;
        }
        PostQuitMessage(0);
        return 0;
    }

    default:
        return DefWindowProcW(hwnd, msg, wParam, lParam);
    }
}

// ============================================================================
// SetWinEventHook callback for Explorer activation
// ============================================================================

static void CALLBACK WinEventProc(HWINEVENTHOOK hWinEventHook, DWORD event,
                                  HWND hwnd, LONG idObject, LONG idChild,
                                  DWORD dwEventThread, DWORD dwmsEventTime)
{
    (void)hWinEventHook;
    (void)idObject;
    (void)idChild;
    (void)dwEventThread;
    (void)dwmsEventTime;

    if (event != EVENT_SYSTEM_FOREGROUND)
        return;

    DWORD dwProcessId = 0;
    GetWindowThreadProcessId(hwnd, &dwProcessId);

    if (!IsExplorerProcess(dwProcessId))
        return;

    RECT rect;
    if (!GetWindowRect(hwnd, &rect))
        return;

    int x = rect.left;
    int y = rect.top;
    int width = rect.right - rect.left;
    int height = rect.bottom - rect.top;

    explorerActivationCallbackCGO(x, y, width, height);
}

// ============================================================================
// Exported functions
// ============================================================================

void showExplorerHint(int x, int y, int width, int height, const char *message,
                      const unsigned char *bgra, int iconWidth, int iconHeight)
{
    (void)message;  // TODO: render message text

    // Clean up old window if exists
    if (g_hintWindow) {
        DestroyWindow(g_hintWindow);
        g_hintWindow = NULL;
    }

    // Register window class
    static BOOL classRegistered = FALSE;
    if (!classRegistered) {
        WNDCLASSW wc = {0};
        wc.style = CS_HREDRAW | CS_VREDRAW;
        wc.lpfnWndProc = OverlayWndProc;
        wc.hInstance = GetModuleHandleW(NULL);
        wc.lpszClassName = L"WoxExplorerHintWindow";
        wc.hCursor = LoadCursorW(NULL, IDC_ARROW);
        wc.hbrBackground = (HBRUSH)GetStockObject(BLACK_BRUSH);

        if (!RegisterClassW(&wc))
            return;
        classRegistered = TRUE;
    }

    // Prepare overlay structure
    ZeroMemory(&g_overlay, sizeof(g_overlay));
    g_overlay.bgra = bgra;
    g_overlay.iconWidth = iconWidth;
    g_overlay.iconHeight = iconHeight;
    if (bgra && iconWidth > 0 && iconHeight > 0) {
        g_overlay.iconBitmap = CreateBitmapFromBGRA(bgra, iconWidth, iconHeight);
    }

    // Create window
    HWND hWnd = CreateWindowExW(
        WS_EX_LAYERED | WS_EX_TOPMOST | WS_EX_TOOLWINDOW | WS_EX_NOACTIVATE,
        L"WoxExplorerHintWindow",
        L"",
        WS_POPUP,
        x, y, width, height,
        NULL,
        NULL,
        GetModuleHandleW(NULL),
        &g_overlay);

    if (!hWnd) {
        if (g_overlay.iconBitmap) {
            DeleteObject(g_overlay.iconBitmap);
            g_overlay.iconBitmap = NULL;
        }
        return;
    }

    g_hintWindow = hWnd;

    // Set window transparency
    SetLayeredWindowAttributes(hWnd, RGB(0, 0, 0), 200, LWA_ALPHA);

    // Show window
    ShowWindow(hWnd, SW_SHOWNOACTIVATE);
    UpdateWindow(hWnd);
}

void hideExplorerHint()
{
    if (g_hintWindow) {
        DestroyWindow(g_hintWindow);
        g_hintWindow = NULL;
    }
    if (g_overlay.iconBitmap) {
        DeleteObject(g_overlay.iconBitmap);
        g_overlay.iconBitmap = NULL;
    }
}

void startAppActivationListener()
{
    if (g_hHook)
        return;

    g_hHook = SetWinEventHook(EVENT_SYSTEM_FOREGROUND, EVENT_SYSTEM_FOREGROUND,
                              NULL, WinEventProc, 0, 0,
                              EVENT_OBJECT_ALL);
}

void stopAppActivationListener()
{
    if (g_hHook) {
        UnhookWinEvent(g_hHook);
        g_hHook = NULL;
    }
}
