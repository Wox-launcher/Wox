#define WIN32_LEAN_AND_MEAN
#define COBJMACROS
#include <windows.h>
#include <windowsx.h>
#include <dwmapi.h>
#include <uxtheme.h>
#include <commctrl.h>
#include <wincodec.h>
#include <objbase.h>
#include <stdbool.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <stdarg.h>
#include <stdio.h>
#include <wchar.h>
#include <math.h>

#ifndef DWMWA_USE_IMMERSIVE_DARK_MODE
#define DWMWA_USE_IMMERSIVE_DARK_MODE 20
#endif

#ifndef DWMWA_WINDOW_CORNER_PREFERENCE
#define DWMWA_WINDOW_CORNER_PREFERENCE 33
#endif

#ifndef DWMWCP_DEFAULT
#define DWMWCP_DEFAULT 0
#define DWMWCP_DONOTROUND 1
#define DWMWCP_ROUND 2
#define DWMWCP_ROUNDSMALL 3
#endif

#ifndef DWMWA_SYSTEMBACKDROP_TYPE
#define DWMWA_SYSTEMBACKDROP_TYPE 38
#endif

#ifndef DWMSBT_AUTO
#define DWMSBT_AUTO 0
#define DWMSBT_NONE 1
#define DWMSBT_MAINWINDOW 2
#define DWMSBT_TRANSIENTWINDOW 3
#define DWMSBT_TABBEDWINDOW 4
#endif

// -----------------------------------------------------------------------------
// Options Struct (Must match CGO / Go definition)
// -----------------------------------------------------------------------------
typedef struct {
    char* name;
    char* title;
    char* message;
    unsigned char* iconData;
    int iconLen;
    char* iconFilePath;
    bool transparent;
    bool hitTestIconOnly;
    float iconX;
    float iconY;
    float iconWidth;
    float iconHeight;
    bool closable;
    bool closeOnEscape;
    bool loading;
    bool topmost;
    bool absolutePosition;
    int stickyWindowPid; // 0 = Screen, >0 = Window
    int anchor;          // 0-8
    int autoCloseSeconds;
    bool movable;
    bool resizable;
    float cornerRadius;
    float aspectRatio;
    float offsetX;
    float offsetY;
    float width;         // 0 = auto
    float height;        // 0 = auto
    float fontSize;      // 0 = system default, unit: pt
    float iconSize;      // 0 = default (16), unit: DIP
    char* tooltip;
    unsigned char* tooltipIconData;
    int tooltipIconLen;
    float tooltipIconSize;
} OverlayOptions;

extern void overlayClickCallbackCGO(char* name);

static BOOL BuildTooltipLogPath(WCHAR *path, DWORD pathLen)
{
    DWORD n = GetEnvironmentVariableW(L"USERPROFILE", path, pathLen);
    if (n == 0 || n >= pathLen)
    {
        DWORD t = GetTempPathW(pathLen, path);
        if (t == 0 || t >= pathLen)
            return FALSE;
        wcscat_s(path, pathLen, L"wox");
        CreateDirectoryW(path, NULL);
        wcscat_s(path, pathLen, L"\\log");
        CreateDirectoryW(path, NULL);
        wcscat_s(path, pathLen, L"\\overlay_tooltip.log");
        return TRUE;
    }

    wcscat_s(path, pathLen, L"\\.wox");
    CreateDirectoryW(path, NULL);
    wcscat_s(path, pathLen, L"\\log");
    CreateDirectoryW(path, NULL);
    wcscat_s(path, pathLen, L"\\overlay_tooltip.log");
    return TRUE;
}

static void LogOverlayTooltip(const WCHAR *fmt, ...)
{
    WCHAR path[MAX_PATH];
    if (!BuildTooltipLogPath(path, MAX_PATH))
        return;

    WCHAR msg[512];
    va_list args;
    va_start(args, fmt);
    _vsnwprintf(msg, 511, fmt, args);
    msg[511] = L'\0';
    va_end(args);

    WCHAR line[520];
    size_t len = wcslen(msg);
    if (len > 510)
        len = 510;
    wcsncpy_s(line, 520, msg, len);
    line[len++] = L'\r';
    line[len++] = L'\n';
    line[len] = L'\0';

    char utf8[2048];
    int utf8Len = WideCharToMultiByte(CP_UTF8, 0, line, -1, utf8, (int)sizeof(utf8), NULL, NULL);
    if (utf8Len <= 0)
        return;

    HANDLE h = CreateFileW(path, FILE_APPEND_DATA, FILE_SHARE_READ | FILE_SHARE_WRITE, NULL, OPEN_ALWAYS, FILE_ATTRIBUTE_NORMAL, NULL);
    if (h == INVALID_HANDLE_VALUE)
        return;
    DWORD written = 0;
    WriteFile(h, utf8, (DWORD)(utf8Len - 1), &written, NULL);
    CloseHandle(h);
}

// -----------------------------------------------------------------------------
// Accent / Acrylic
// -----------------------------------------------------------------------------
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
    policy.GradientColor = gradientColor; // 0xAABBGGRR

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

// -----------------------------------------------------------------------------
// Constants
// -----------------------------------------------------------------------------
#define DEFAULT_WINDOW_WIDTH_DIP 400
#define MIN_WINDOW_WIDTH_DIP 100
#define PADDING_X_DIP 12
#define PADDING_Y_DIP 10
#define DEFAULT_ICON_SIZE_DIP 16
#define ICON_GAP_DIP 10
#define CLOSE_SIZE_DIP 20
#define CLOSE_PAD_DIP 10
#define TOOLTIP_GAP_DIP 6
#define CORNER_RADIUS_DIP 10
#define RESIZE_GRIP_DIP 10
#define MIN_RESIZE_SIZE_DIP 64
#define IMAGE_SHADOW_PADDING_DIP 20
#define IMAGE_SHADOW_MAX_ALPHA 96
#define WHEEL_ZOOM_STEP 1.12f

#define TIMER_AUTOCLOSE 1
#define TIMER_TRACK 2
#define TIMER_LIVE_FOLLOW 3
#define PREDICTIVE_CORRECTION_THRESHOLD_PX 48

#define WM_WOX_OVERLAY_COMMAND (WM_APP + 0x610)
#define WM_WOX_OVERLAY_REPOSITION (WM_APP + 0x611)

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------
typedef UINT(WINAPI *pfnGetDpiForSystem)(void);
typedef UINT(WINAPI *pfnGetDpiForWindow)(HWND);
typedef BOOL(WINAPI *pfnSetProcessDpiAwarenessContext)(HANDLE);

static UINT GetSystemDpiSafe(void)
{
    HMODULE user32 = GetModuleHandleW(L"user32.dll");
    if (!user32)
        return 96;
    pfnGetDpiForSystem fn = (pfnGetDpiForSystem)GetProcAddress(user32, "GetDpiForSystem");
    if (!fn)
        return 96;
    UINT dpi = fn();
    return dpi ? dpi : 96;
}

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

static void TryEnablePerMonitorDpiAwareness(void)
{
    HMODULE user32 = GetModuleHandleW(L"user32.dll");
    if (!user32)
        return;
    pfnSetProcessDpiAwarenessContext fn = (pfnSetProcessDpiAwarenessContext)GetProcAddress(user32, "SetProcessDpiAwarenessContext");
    if (!fn)
        return;
    fn((HANDLE)-4); // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2
}

static WCHAR *DupUtf8ToWide(const char *utf8)
{
    if (!utf8)
        return NULL;
    int wlen = MultiByteToWideChar(CP_UTF8, 0, utf8, -1, NULL, 0);
    if (wlen <= 0)
        return NULL;
    WCHAR *out = (WCHAR *)malloc((size_t)wlen * sizeof(WCHAR));
    if (!out)
        return NULL;
    MultiByteToWideChar(CP_UTF8, 0, utf8, -1, out, wlen);
    return out;
}

static char *DupWideToUtf8(const WCHAR *w)
{
    if (!w)
        return NULL;
    int len = WideCharToMultiByte(CP_UTF8, 0, w, -1, NULL, 0, NULL, NULL);
    if (len <= 0)
        return NULL;
    char *out = (char *)malloc((size_t)len);
    if (!out)
        return NULL;
    WideCharToMultiByte(CP_UTF8, 0, w, -1, out, len, NULL, NULL);
    return out;
}

static HBITMAP Create32BitDIBSection(HDC hdc, int width, int height, void **bits)
{
    if (bits)
        *bits = NULL;
    BITMAPINFO bmi;
    ZeroMemory(&bmi, sizeof(bmi));
    bmi.bmiHeader.biSize = sizeof(BITMAPINFOHEADER);
    bmi.bmiHeader.biWidth = width;
    bmi.bmiHeader.biHeight = -height;
    bmi.bmiHeader.biPlanes = 1;
    bmi.bmiHeader.biBitCount = 32;
    bmi.bmiHeader.biCompression = BI_RGB;
    return CreateDIBSection(hdc, &bmi, DIB_RGB_COLORS, bits, NULL, 0);
}

static HBITMAP CreateBitmapFromWicDecoder(IWICImagingFactory *factory, IWICBitmapDecoder *decoder, int *outW, int *outH)
{
    if (!factory || !decoder)
        return NULL;

    IWICBitmapFrameDecode *frame = NULL;
    HRESULT hr = IWICBitmapDecoder_GetFrame(decoder, 0, &frame);
    if (FAILED(hr) || !frame)
        return NULL;

    IWICFormatConverter *converter = NULL;
    hr = IWICImagingFactory_CreateFormatConverter(factory, &converter);
    if (FAILED(hr) || !converter)
    {
        IWICBitmapFrameDecode_Release(frame);
        return NULL;
    }

    hr = IWICFormatConverter_Initialize(converter, (IWICBitmapSource *)frame,
                                        &GUID_WICPixelFormat32bppBGRA, WICBitmapDitherTypeNone,
                                        NULL, 0.0, WICBitmapPaletteTypeCustom);
    if (FAILED(hr))
    {
        IWICFormatConverter_Release(converter);
        IWICBitmapFrameDecode_Release(frame);
        return NULL;
    }

    UINT w = 0, h = 0;
    IWICBitmapSource_GetSize((IWICBitmapSource *)converter, &w, &h);
    if (w == 0 || h == 0)
    {
        IWICFormatConverter_Release(converter);
        IWICBitmapFrameDecode_Release(frame);
        return NULL;
    }

    HDC hdc = GetDC(NULL);
    void *bits = NULL;
    HBITMAP dib = Create32BitDIBSection(hdc, (int)w, (int)h, &bits);
    ReleaseDC(NULL, hdc);
    if (!dib || !bits)
    {
        if (dib)
            DeleteObject(dib);
        IWICFormatConverter_Release(converter);
        IWICBitmapFrameDecode_Release(frame);
        return NULL;
    }

    WICRect rc;
    rc.X = 0;
    rc.Y = 0;
    rc.Width = (INT)w;
    rc.Height = (INT)h;
    hr = IWICBitmapSource_CopyPixels((IWICBitmapSource *)converter, &rc, w * 4, w * h * 4, (BYTE *)bits);
    if (FAILED(hr))
    {
        DeleteObject(dib);
        dib = NULL;
    }
    else
    {
        if (outW)
            *outW = (int)w;
        if (outH)
            *outH = (int)h;
    }

    IWICFormatConverter_Release(converter);
    IWICBitmapFrameDecode_Release(frame);

    return dib;
}

static HBITMAP CreateBitmapFromPngData(const unsigned char *data, int len, int *outW, int *outH)
{
    if (outW)
        *outW = 0;
    if (outH)
        *outH = 0;
    if (!data || len <= 0)
        return NULL;

    IWICImagingFactory *factory = NULL;
    HRESULT hr = CoCreateInstance(&CLSID_WICImagingFactory, NULL, CLSCTX_INPROC_SERVER,
                                  &IID_IWICImagingFactory, (LPVOID *)&factory);
    if (FAILED(hr) || !factory)
        return NULL;

    HGLOBAL hMem = GlobalAlloc(GMEM_MOVEABLE, (SIZE_T)len);
    if (!hMem)
    {
        IWICImagingFactory_Release(factory);
        return NULL;
    }
    void *pMem = GlobalLock(hMem);
    if (!pMem)
    {
        GlobalFree(hMem);
        IWICImagingFactory_Release(factory);
        return NULL;
    }
    memcpy(pMem, data, (SIZE_T)len);
    GlobalUnlock(hMem);

    IStream *stream = NULL;
    hr = CreateStreamOnHGlobal(hMem, TRUE, &stream);
    if (FAILED(hr) || !stream)
    {
        GlobalFree(hMem);
        IWICImagingFactory_Release(factory);
        return NULL;
    }

    IWICBitmapDecoder *decoder = NULL;
    hr = IWICImagingFactory_CreateDecoderFromStream(factory, stream, NULL, WICDecodeMetadataCacheOnLoad, &decoder);
    if (FAILED(hr) || !decoder)
    {
        IStream_Release(stream);
        IWICImagingFactory_Release(factory);
        return NULL;
    }

    HBITMAP bitmap = CreateBitmapFromWicDecoder(factory, decoder, outW, outH);
    IWICBitmapDecoder_Release(decoder);
    IStream_Release(stream);
    IWICImagingFactory_Release(factory);
    return bitmap;
}

static HBITMAP CreateBitmapFromImageFilePath(const WCHAR *path, int *outW, int *outH)
{
    if (outW)
        *outW = 0;
    if (outH)
        *outH = 0;
    if (!path || !*path)
        return NULL;

    IWICImagingFactory *factory = NULL;
    HRESULT hr = CoCreateInstance(&CLSID_WICImagingFactory, NULL, CLSCTX_INPROC_SERVER,
                                  &IID_IWICImagingFactory, (LPVOID *)&factory);
    if (FAILED(hr) || !factory)
        return NULL;

    IWICBitmapDecoder *decoder = NULL;
    // File-backed pinned screenshots bypass the Go PNG re-encode path and let WIC
    // decode the capture from disk, matching the macOS AppKit file-source path.
    hr = IWICImagingFactory_CreateDecoderFromFilename(factory, path, NULL, GENERIC_READ, WICDecodeMetadataCacheOnLoad, &decoder);
    if (FAILED(hr) || !decoder)
    {
        IWICImagingFactory_Release(factory);
        return NULL;
    }

    HBITMAP bitmap = CreateBitmapFromWicDecoder(factory, decoder, outW, outH);
    IWICBitmapDecoder_Release(decoder);
    IWICImagingFactory_Release(factory);
    return bitmap;
}

static int MeasureTextHeightW(HDC hdc, const WCHAR *text, int width)
{
    if (!text || !*text || width <= 0)
        return 0;
    RECT rc = {0, 0, width, 0};
    DrawTextW(hdc, text, -1, &rc, DT_CALCRECT | DT_WORDBREAK | DT_EDITCONTROL | DT_NOPREFIX);
    int h = rc.bottom - rc.top;
    return h > 0 ? h : 0;
}

static float GetSystemMessageFontSizePt(void)
{
    NONCLIENTMETRICSW ncm;
    ZeroMemory(&ncm, sizeof(ncm));
    ncm.cbSize = sizeof(ncm);
    if (SystemParametersInfoW(SPI_GETNONCLIENTMETRICS, ncm.cbSize, &ncm, 0))
    {
        int px = ncm.lfMessageFont.lfHeight;
        if (px != 0)
        {
            if (px < 0)
                px = -px;

            HDC hdc = GetDC(NULL);
            int dpiY = hdc ? GetDeviceCaps(hdc, LOGPIXELSY) : 96;
            if (hdc)
                ReleaseDC(NULL, hdc);
            if (dpiY <= 0)
                dpiY = 96;

            return ((float)px * 72.0f) / (float)dpiY;
        }
    }

    return 9.0f;
}

static RECT GetWorkAreaForRect(const RECT *target)
{
    RECT workArea;
    ZeroMemory(&workArea, sizeof(workArea));

    HMONITOR mon = MonitorFromRect(target, MONITOR_DEFAULTTONEAREST);
    MONITORINFO mi;
    mi.cbSize = sizeof(mi);
    if (mon && GetMonitorInfo(mon, &mi))
    {
        workArea = mi.rcWork;
        return workArea;
    }

    SystemParametersInfo(SPI_GETWORKAREA, 0, &workArea, 0);
    return workArea;
}

static void ClampWindowToWorkArea(const RECT *work, int *x, int *y, int width, int height)
{
    if (!work || !x || !y)
        return;
    if (*x < work->left)
        *x = work->left;
    if (*y < work->top)
        *y = work->top;
    if (*x + width > work->right)
        *x = work->right - width;
    if (*y + height > work->bottom)
        *y = work->bottom - height;
}

// -----------------------------------------------------------------------------
// Overlay Structures
// -----------------------------------------------------------------------------

typedef struct OverlayWindow
{
    HWND hwnd;
    HWND shadowHwnd;
    WCHAR *name;
    WCHAR *title;
    WCHAR *message;
    WCHAR *tooltip;
    HBITMAP iconBitmap;
    int iconWidth;
    int iconHeight;
    BOOL transparent;
    BOOL hitTestIconOnly;
    float iconX;
    float iconY;
    float iconDrawWidth;
    float iconDrawHeight;
    RECT iconRect;
    HBITMAP tooltipIconBitmap;
    int tooltipIconWidth;
    int tooltipIconHeight;
    float tooltipIconSize;
    BOOL closable;
    BOOL closeOnEscape;
    BOOL loading;
    BOOL topmost;
    BOOL absolutePosition;
    BOOL movable;
    BOOL resizable;
    float cornerRadius;
    float aspectRatio;
    int autoCloseSeconds;
    int stickyWindowPid;
    int anchor;
    float offsetX;
    float offsetY;
    float width;
    float height;
    float fontSize; // pt, <=0 means system default
    float iconSize; // DIP, <=0 means default

    UINT dpi;
    HFONT messageFont;
    UINT fontDpi;
    float appliedFontSize;

    RECT closeRect;
    BOOL mouseInside;
    BOOL closeHover;
    BOOL closePressed;
    BOOL dragging;
    BOOL autoClosePending;
    POINT dragStart;
    POINT dragWindowOrigin;
    RECT lastTargetRect;
    BOOL hasLastTargetRect;
    RECT predictiveAnchorTargetRect;
    POINT predictiveAnchorMouse;
    BOOL hasPredictiveAnchor;
    BOOL liveFollowActive;
    BOOL hiddenForMove;
    BOOL targetReady;
    
    HWND targetHwnd;
    HWINEVENTHOOK locationHook;

    RECT tooltipRect;
    BOOL tooltipHover;
    HWND tooltipHwnd;

    struct OverlayWindow *next;
} OverlayWindow;

typedef struct OverlayPayload
{
    WCHAR *name;
    WCHAR *title;
    WCHAR *message;
    WCHAR *tooltip;
    unsigned char *iconData;
    int iconLen;
    WCHAR *iconFilePath;
    BOOL transparent;
    BOOL hitTestIconOnly;
    float iconX;
    float iconY;
    float iconWidth;
    float iconHeight;
    unsigned char *tooltipIconData;
    int tooltipIconLen;
    float tooltipIconSize;
    BOOL closable;
    BOOL closeOnEscape;
    BOOL loading;
    BOOL topmost;
    BOOL absolutePosition;
    int stickyWindowPid;
    int anchor;
    int autoCloseSeconds;
    BOOL movable;
    BOOL resizable;
    float cornerRadius;
    float aspectRatio;
    float offsetX;
    float offsetY;
    float width;
    float height;
    float fontSize;
    float iconSize;
} OverlayPayload;

typedef struct OverlayCommand
{
    int type; // 1 = show, 2 = close
    OverlayPayload *payload;
    WCHAR *name;
} OverlayCommand;

static OverlayWindow *g_overlays = NULL;
static const WCHAR *g_overlayClassName = L"WoxOverlayWindow";
static const WCHAR *g_shadowClassName = L"WoxOverlayShadowWindow";
static const WCHAR *g_controllerClassName = L"WoxOverlayController";
static const WCHAR *g_tooltipClassName = L"WoxOverlayTooltip";
static HANDLE g_threadReadyEvent = NULL;
static HANDLE g_overlayThread = NULL;
static DWORD g_overlayThreadId = 0;
static HWND g_controllerHwnd = NULL;
static INIT_ONCE g_initOnce = INIT_ONCE_STATIC_INIT;


// -----------------------------------------------------------------------------
// Forward Decls
// -----------------------------------------------------------------------------
static LRESULT CALLBACK OverlayWindowProc(HWND hwnd, UINT uMsg, WPARAM wParam, LPARAM lParam);
static LRESULT CALLBACK ShadowWindowProc(HWND hwnd, UINT uMsg, WPARAM wParam, LPARAM lParam);
static LRESULT CALLBACK OverlayControllerProc(HWND hwnd, UINT uMsg, WPARAM wParam, LPARAM lParam);
static DWORD WINAPI OverlayThreadProc(LPVOID param);
static BOOL GetTargetContentRect(HWND target, RECT *outRect);
static void StartLiveFollowTimerIfNeeded(OverlayWindow *ow);
static void StopLiveFollowTimer(OverlayWindow *ow);
static void RepositionOverlayToTargetRect(OverlayWindow *ow, const RECT *targetRect, BOOL preserveSmallPredictiveCorrection);
static void ShowOverlayWindowWithFocusPolicy(OverlayWindow *ow);

// -----------------------------------------------------------------------------
// Overlay Helpers
// -----------------------------------------------------------------------------

static OverlayWindow *FindOverlayByName(const WCHAR *name)
{
    for (OverlayWindow *it = g_overlays; it; it = it->next)
    {
        if (it->name && name && wcscmp(it->name, name) == 0)
            return it;
    }
    return NULL;
}

static void AddOverlay(OverlayWindow *ow)
{
    ow->next = g_overlays;
    g_overlays = ow;
}

static void RemoveOverlay(OverlayWindow *ow)
{
    OverlayWindow **pp = &g_overlays;
    while (*pp)
    {
        if (*pp == ow)
        {
            *pp = ow->next;
            return;
        }
        pp = &((*pp)->next);
    }
}

typedef struct
{
    DWORD pid;
    HWND hwnd;
    HWND fallback;
} FindWindowData;

static int IsExplorerWindowClass(const WCHAR *className)
{
    if (!className || !*className)
        return 0;
    if (_wcsicmp(className, L"CabinetWClass") == 0)
        return 1;
    if (_wcsicmp(className, L"ExploreWClass") == 0)
        return 1;
    return 0;
}

static int IsDesktopWindowClass(const WCHAR *className)
{
    if (!className || !*className)
        return 0;
    if (_wcsicmp(className, L"Progman") == 0)
        return 1;
    if (_wcsicmp(className, L"WorkerW") == 0)
        return 1;
    if (_wcsicmp(className, L"Shell_TrayWnd") == 0)
        return 1;
    if (_wcsicmp(className, L"Shell_SecondaryTrayWnd") == 0)
        return 1;
    return 0;
}

static BOOL CALLBACK EnumWindowByPidProc(HWND hwnd, LPARAM lParam)
{
    FindWindowData *d = (FindWindowData *)lParam;
    if (!IsWindowVisible(hwnd))
        return TRUE;

    DWORD wpid = 0;
    GetWindowThreadProcessId(hwnd, &wpid);
    if (wpid != d->pid)
        return TRUE;

    WCHAR className[128];
    int len = GetClassNameW(hwnd, className, (int)(sizeof(className) / sizeof(className[0])));
    if (len <= 0)
        return TRUE;
    if (IsDesktopWindowClass(className))
        return TRUE;

    LONG style = GetWindowLong(hwnd, GWL_STYLE);
    if (!(style & WS_OVERLAPPEDWINDOW) && !(style & WS_POPUP))
        return TRUE;

    if (IsExplorerWindowClass(className))
    {
        d->hwnd = hwnd;
        return FALSE;
    }

    if (!d->fallback)
        d->fallback = hwnd;
    return TRUE;
}

static BOOL FindWindowByPid(int pid, HWND *out)
{
    if (out)
        *out = NULL;
    if (pid <= 0)
        return FALSE;

    HWND fg = GetForegroundWindow();
    if (fg && IsWindowVisible(fg))
    {
        DWORD fgPid = 0;
        GetWindowThreadProcessId(fg, &fgPid);
        if ((int)fgPid == pid)
        {
            WCHAR className[128];
            int len = GetClassNameW(fg, className, (int)(sizeof(className) / sizeof(className[0])));
            if (len > 0 && IsExplorerWindowClass(className))
            {
                if (out)
                    *out = fg;
                return TRUE;
            }
        }
    }

    FindWindowData data;
    data.pid = (DWORD)pid;
    data.hwnd = NULL;
    data.fallback = NULL;

    EnumWindows(EnumWindowByPidProc, (LPARAM)&data);
    if (!data.hwnd && data.fallback)
        data.hwnd = data.fallback;
    if (out)
        *out = data.hwnd;
    return data.hwnd != NULL;
}

static void UpdateOverlayOwner(HWND hwnd, HWND target)
{
    if (!hwnd)
        return;
    HWND owner = (HWND)GetWindowLongPtr(hwnd, GWLP_HWNDPARENT);
    if (owner != target)
    {
        SetWindowLongPtr(hwnd, GWLP_HWNDPARENT, (LONG_PTR)target);
    }
}

static void SetOverlayZOrder(HWND hwnd, HWND target)
{
    if (target && IsWindow(target))
    {
        UpdateOverlayOwner(hwnd, target);
        if (GetForegroundWindow() == target)
        {
            SetWindowPos(hwnd, HWND_TOP, 0, 0, 0, 0,
                         SWP_NOMOVE | SWP_NOSIZE | SWP_NOACTIVATE | SWP_NOOWNERZORDER);
        }
    }
    else
    {
        UpdateOverlayOwner(hwnd, NULL);
        SetWindowPos(hwnd, HWND_TOPMOST, 0, 0, 0, 0, SWP_NOMOVE | SWP_NOSIZE | SWP_NOACTIVATE);
    }
}

static LRESULT GetResizeHitTest(OverlayWindow *ow, POINT pt)
{
    if (!ow || !ow->resizable)
        return HTCLIENT;

    RECT client;
    GetClientRect(ow->hwnd, &client);
    int grip = MulDiv(RESIZE_GRIP_DIP, (int)(ow->dpi ? ow->dpi : 96), 96);
    BOOL left = pt.x <= grip;
    BOOL right = pt.x >= client.right - grip;
    BOOL top = pt.y <= grip;
    BOOL bottom = pt.y >= client.bottom - grip;

    // Feature change: transparent image overlays are borderless, so Windows needs explicit
    // non-client hit-test results to start native edge and corner resizing without interfering
    // with the existing interior drag-to-move behavior.
    if (top && left) return HTTOPLEFT;
    if (top && right) return HTTOPRIGHT;
    if (bottom && left) return HTBOTTOMLEFT;
    if (bottom && right) return HTBOTTOMRIGHT;
    if (left) return HTLEFT;
    if (right) return HTRIGHT;
    if (top) return HTTOP;
    if (bottom) return HTBOTTOM;
    return HTCLIENT;
}

static void StartAutoCloseTimer(OverlayWindow *ow)
{
    if (!ow || !ow->hwnd)
        return;
    KillTimer(ow->hwnd, TIMER_AUTOCLOSE);
    ow->autoClosePending = FALSE;
    if (ow->autoCloseSeconds > 0)
    {
        SetTimer(ow->hwnd, TIMER_AUTOCLOSE, (UINT)(ow->autoCloseSeconds * 1000), NULL);
    }
}

static void StartTrackTimer(OverlayWindow *ow)
{
    if (!ow || !ow->hwnd)
        return;
    KillTimer(ow->hwnd, TIMER_TRACK);
    if (ow->stickyWindowPid > 0)
    {
        SetTimer(ow->hwnd, TIMER_TRACK, 200, NULL);
    }
}

static BOOL IsLeftButtonDown(void)
{
    return (GetAsyncKeyState(VK_LBUTTON) & 0x8000) != 0;
}

static BOOL GetTargetContentRect(HWND target, RECT *outRect)
{
    if (!target || !IsWindow(target) || !outRect)
        return FALSE;

    RECT targetRect;
    RECT clientRect;
    if (GetClientRect(target, &clientRect))
    {
        POINT tl = {clientRect.left, clientRect.top};
        POINT br = {clientRect.right, clientRect.bottom};
        ClientToScreen(target, &tl);
        ClientToScreen(target, &br);
        targetRect.left = tl.x;
        targetRect.top = tl.y;
        targetRect.right = br.x;
        targetRect.bottom = br.y;
    }
    else if (!GetWindowRect(target, &targetRect))
    {
        return FALSE;
    }

    if (targetRect.right - targetRect.left <= 1 || targetRect.bottom - targetRect.top <= 1)
        return FALSE;

    *outRect = targetRect;
    return TRUE;
}

static void RefreshPredictiveAnchor(OverlayWindow *ow, const RECT *targetRect)
{
    if (!ow || !targetRect || !IsLeftButtonDown())
        return;

    // Predictive follow stores the latest trusted window sample with the cursor
    // position observed at the same time. Later timer ticks can then move the
    // sticky overlay from mouse deltas instead of waiting for lower-frequency
    // location events from the window manager.
    ow->predictiveAnchorTargetRect = *targetRect;
    GetCursorPos(&ow->predictiveAnchorMouse);
    ow->hasPredictiveAnchor = TRUE;
}

static BOOL GetPredictiveTargetRect(OverlayWindow *ow, RECT *outRect)
{
    if (!ow || !outRect || !ow->hasPredictiveAnchor)
        return FALSE;

    POINT cursor;
    if (!GetCursorPos(&cursor))
        return FALSE;

    int dx = cursor.x - ow->predictiveAnchorMouse.x;
    int dy = cursor.y - ow->predictiveAnchorMouse.y;
    *outRect = ow->predictiveAnchorTargetRect;
    OffsetRect(outRect, dx, dy);
    return TRUE;
}

static void StartLiveFollowTimerIfNeeded(OverlayWindow *ow)
{
    if (!ow || !ow->hwnd || ow->liveFollowActive || ow->stickyWindowPid <= 0 || !IsLeftButtonDown())
        return;

    // Optimization: Windows location hooks can still be coalesced during native
    // dragging. A 16ms live-follow timer keeps generic sticky overlays moving at
    // frame cadence, while real window samples continue to calibrate the anchor.
    SetTimer(ow->hwnd, TIMER_LIVE_FOLLOW, 16, NULL);
    ow->liveFollowActive = TRUE;
}

static void StopLiveFollowTimer(OverlayWindow *ow)
{
    if (!ow || !ow->hwnd || !ow->liveFollowActive)
        return;

    KillTimer(ow->hwnd, TIMER_LIVE_FOLLOW);
    ow->liveFollowActive = FALSE;
    ow->hasPredictiveAnchor = FALSE;
}

static void ShowOverlayWindowWithFocusPolicy(OverlayWindow *ow)
{
    if (!ow || !ow->hwnd)
        return;

    if (ow->closeOnEscape)
    {
        // Bug fix: SW_SHOWNOACTIVATE made Escape-to-close overlays visible but left keyboard focus
        // on the launcher. Focus only keyboard-dismissable overlays so ordinary notification
        // overlays keep their non-activating behavior.
        ShowWindow(ow->hwnd, SW_SHOW);
        SetForegroundWindow(ow->hwnd);
        SetFocus(ow->hwnd);
        return;
    }

    ShowWindow(ow->hwnd, SW_SHOWNOACTIVATE);
}

static void UpdateCloseRect(OverlayWindow *ow, int width, int height, UINT dpi)
{
    RECT r = {0, 0, 0, 0};
    if (!ow->closable)
    {
        ow->closeRect = r;
        return;
    }
    int closeSize = MulDiv(CLOSE_SIZE_DIP, (int)dpi, 96);
    int closePad = MulDiv(CLOSE_PAD_DIP, (int)dpi, 96);
    int x = width - closePad - closeSize;
    int y = closePad;
    r.left = x;
    r.top = y;
    r.right = x + closeSize;
    r.bottom = y + closeSize;
    ow->closeRect = r;
}

static void ComputeOverlayPosition(OverlayWindow *ow, const RECT *targetRect, int width, int height, int *outX, int *outY)
{
    int ax = targetRect->left;
    int ay = targetRect->top;
    int aw = targetRect->right - targetRect->left;
    int ah = targetRect->bottom - targetRect->top;

    int col = ow->anchor % 3;
    int row = ow->anchor / 3;

    int px = ax;
    if (col == 1)
        px = ax + aw / 2;
    else if (col == 2)
        px = ax + aw;

    int py = ay;
    if (row == 1)
        py = ay + ah / 2;
    else if (row == 2)
        py = ay + ah;

    int ox = 0;
    if (col == 1)
        ox = -width / 2;
    else if (col == 2)
        ox = -width;

    int oy = 0;
    if (row == 1)
        oy = -height / 2;
    else if (row == 2)
        oy = -height;

    int offX = (int)roundf(ow->offsetX * (float)ow->dpi / 96.0f);
    int offY = (int)roundf(ow->offsetY * (float)ow->dpi / 96.0f);

    if (outX)
        *outX = px + ox + offX;
    if (outY)
        *outY = py + oy + offY;
}

static void RepositionOverlayToTargetRect(OverlayWindow *ow, const RECT *targetRect, BOOL preserveSmallPredictiveCorrection)
{
    if (!ow || !ow->hwnd || !targetRect)
        return;

    RECT client;
    GetClientRect(ow->hwnd, &client);
    int width = client.right - client.left;
    int height = client.bottom - client.top;
    int x = 0;
    int y = 0;
    ComputeOverlayPosition(ow, targetRect, width, height, &x, &y);
    RECT workArea = GetWorkAreaForRect(targetRect);
    ClampWindowToWorkArea(&workArea, &x, &y, width, height);

    if (preserveSmallPredictiveCorrection && ow->liveFollowActive && ow->hasPredictiveAnchor && IsLeftButtonDown())
    {
        RECT current;
        if (GetWindowRect(ow->hwnd, &current))
        {
            int correctionX = x - current.left;
            int correctionY = y - current.top;
            if (abs(correctionX) <= PREDICTIVE_CORRECTION_THRESHOLD_PX &&
                abs(correctionY) <= PREDICTIVE_CORRECTION_THRESHOLD_PX)
            {
                // Optimization: WinEvent location samples are still needed to
                // refresh the predictive anchor, but small corrections should not
                // pull the overlay back to an older point while the live timer is
                // already following mouse movement. Large corrections still apply
                // so snapping, monitor changes, and real drift can recover.
                x = current.left;
                y = current.top;
            }
        }
    }

    SetWindowPos(ow->hwnd, NULL, x, y, 0, 0, SWP_NOACTIVATE | SWP_NOSIZE | SWP_NOZORDER);
}

static void ApplyCornerRadius(HWND hwnd, UINT dpi, int width, int height)
{
    UINT pref = DWMWCP_ROUND;
    HRESULT hr = DwmSetWindowAttribute(hwnd, DWMWA_WINDOW_CORNER_PREFERENCE, &pref, sizeof(pref));
    if (FAILED(hr))
    {
        int rr = MulDiv(CORNER_RADIUS_DIP, (int)dpi, 96);
        HRGN rgn = CreateRoundRectRgn(0, 0, width + 1, height + 1, rr * 2, rr * 2);
        if (rgn)
        {
            SetWindowRgn(hwnd, rgn, TRUE);
        }
    }
}

static int GetTransparentImageShadowPadding(UINT dpi)
{
    return MulDiv(IMAGE_SHADOW_PADDING_DIP, (int)(dpi ? dpi : 96), 96);
}

static void DestroyTransparentImageShadow(OverlayWindow *ow)
{
    if (ow && ow->shadowHwnd)
    {
        DestroyWindow(ow->shadowHwnd);
        ow->shadowHwnd = NULL;
    }
}

static BOOL EnsureTransparentImageShadowWindow(OverlayWindow *ow)
{
    if (!ow || !ow->hwnd)
        return FALSE;
    if (ow->shadowHwnd && IsWindow(ow->shadowHwnd))
        return TRUE;

    ow->shadowHwnd = CreateWindowExW(
        WS_EX_LAYERED | WS_EX_TRANSPARENT | WS_EX_TOOLWINDOW | WS_EX_NOACTIVATE | WS_EX_TOPMOST,
        g_shadowClassName,
        L"Wox image overlay shadow",
        WS_POPUP,
        0, 0, 0, 0,
        NULL, NULL, GetModuleHandleW(NULL), NULL);

    return ow->shadowHwnd != NULL;
}

static void UpdateTransparentImageShadow(OverlayWindow *ow)
{
    if (!ow || !ow->transparent || !ow->hwnd || !IsWindow(ow->hwnd))
    {
        DestroyTransparentImageShadow(ow);
        return;
    }

    RECT windowRect;
    if (!GetWindowRect(ow->hwnd, &windowRect))
        return;

    int imageW = windowRect.right - windowRect.left;
    int imageH = windowRect.bottom - windowRect.top;
    if (imageW <= 0 || imageH <= 0)
        return;

    UINT dpi = ow->dpi ? ow->dpi : GetWindowDpiSafe(ow->hwnd, 96);
    int pad = GetTransparentImageShadowPadding(dpi);
    int shadowW = imageW + pad * 2;
    int shadowH = imageH + pad * 2;
    if (pad <= 0 || shadowW <= 0 || shadowH <= 0)
        return;

    if (!EnsureTransparentImageShadowWindow(ow))
        return;

    HDC screenDC = GetDC(NULL);
    if (!screenDC)
        return;

    HDC memDC = CreateCompatibleDC(screenDC);
    if (!memDC)
    {
        ReleaseDC(NULL, screenDC);
        return;
    }

    void *bits = NULL;
    HBITMAP bitmap = Create32BitDIBSection(screenDC, shadowW, shadowH, &bits);
    if (!bitmap || !bits)
    {
        if (bitmap)
            DeleteObject(bitmap);
        DeleteDC(memDC);
        ReleaseDC(NULL, screenDC);
        return;
    }

    uint32_t *pixels = (uint32_t *)bits;
    ZeroMemory(pixels, (SIZE_T)shadowW * (SIZE_T)shadowH * sizeof(uint32_t));
    const int left = pad;
    const int top = pad;
    const int right = pad + imageW;
    const int bottom = pad + imageH;
    const double maxAlpha = (double)IMAGE_SHADOW_MAX_ALPHA;

    for (int y = 0; y < shadowH; y++)
    {
        for (int x = 0; x < shadowW; x++)
        {
            int dx = 0;
            if (x < left)
                dx = left - x;
            else if (x >= right)
                dx = x - right + 1;

            int dy = 0;
            if (y < top)
                dy = top - y;
            else if (y >= bottom)
                dy = y - bottom + 1;

            if (dx == 0 && dy == 0)
                continue;

            double distance = sqrt((double)(dx * dx + dy * dy));
            if (distance > (double)pad)
                continue;

            double t = 1.0 - distance / (double)pad;
            BYTE alpha = (BYTE)round(maxAlpha * t * t);
            if (alpha == 0)
                continue;

            pixels[(SIZE_T)y * (SIZE_T)shadowW + (SIZE_T)x] = ((uint32_t)alpha << 24);
        }
    }

    HGDIOBJ old = SelectObject(memDC, bitmap);
    POINT dst = {windowRect.left - pad, windowRect.top - pad};
    SIZE size = {shadowW, shadowH};
    POINT src = {0, 0};
    BLENDFUNCTION blend = {AC_SRC_OVER, 0, 255, AC_SRC_ALPHA};
    UpdateLayeredWindow(ow->shadowHwnd, screenDC, &dst, &size, memDC, &src, 0, &blend, ULW_ALPHA);
    SetWindowPos(ow->shadowHwnd, ow->hwnd, dst.x, dst.y, shadowW, shadowH, SWP_NOACTIVATE | SWP_SHOWWINDOW);

    if (old)
        SelectObject(memDC, old);
    DeleteObject(bitmap);
    DeleteDC(memDC);
    ReleaseDC(NULL, screenDC);
}

static void MoveTransparentImageShadow(OverlayWindow *ow)
{
    if (!ow || !ow->transparent || !ow->hwnd || !ow->shadowHwnd || !IsWindow(ow->shadowHwnd))
        return;

    RECT windowRect;
    if (!GetWindowRect(ow->hwnd, &windowRect))
        return;

    int imageW = windowRect.right - windowRect.left;
    int imageH = windowRect.bottom - windowRect.top;
    if (imageW <= 0 || imageH <= 0)
        return;

    UINT dpi = ow->dpi ? ow->dpi : GetWindowDpiSafe(ow->hwnd, 96);
    int pad = GetTransparentImageShadowPadding(dpi);
    int shadowW = imageW + pad * 2;
    int shadowH = imageH + pad * 2;
    if (pad <= 0 || shadowW <= 0 || shadowH <= 0)
        return;

    // Optimization: dragging does not change image size, so the expensive shadow bitmap can stay
    // cached in the layered shadow HWND. Moving that HWND with the image avoids per-mouse-move
    // DIB allocation and reduces flicker when crossing monitors.
    SetWindowPos(ow->shadowHwnd, ow->hwnd, windowRect.left - pad, windowRect.top - pad, shadowW, shadowH, SWP_NOACTIVATE | SWP_SHOWWINDOW);
}

static BOOL UpdateTransparentImageLayer(OverlayWindow *ow)
{
    if (!ow || !ow->transparent || !ow->hwnd || !IsWindow(ow->hwnd) || !ow->iconBitmap)
        return FALSE;

    RECT windowRect;
    if (!GetWindowRect(ow->hwnd, &windowRect))
        return FALSE;

    int imageW = windowRect.right - windowRect.left;
    int imageH = windowRect.bottom - windowRect.top;
    if (imageW <= 0 || imageH <= 0)
        return FALSE;

    HDC screenDC = GetDC(NULL);
    if (!screenDC)
        return FALSE;

    HDC layerDC = CreateCompatibleDC(screenDC);
    if (!layerDC)
    {
        ReleaseDC(NULL, screenDC);
        return FALSE;
    }

    void *bits = NULL;
    HBITMAP layerBitmap = Create32BitDIBSection(screenDC, imageW, imageH, &bits);
    if (!layerBitmap || !bits)
    {
        if (layerBitmap)
            DeleteObject(layerBitmap);
        DeleteDC(layerDC);
        ReleaseDC(NULL, screenDC);
        return FALSE;
    }
    ZeroMemory(bits, (SIZE_T)imageW * (SIZE_T)imageH * sizeof(uint32_t));

    HGDIOBJ oldLayer = SelectObject(layerDC, layerBitmap);
    HDC imageDC = CreateCompatibleDC(screenDC);
    HGDIOBJ oldImage = NULL;
    if (imageDC)
    {
        oldImage = SelectObject(imageDC, ow->iconBitmap);
        BLENDFUNCTION imageBlend = {AC_SRC_OVER, 0, 255, AC_SRC_ALPHA};
        AlphaBlend(layerDC, 0, 0, imageW, imageH, imageDC, 0, 0, ow->iconWidth, ow->iconHeight, imageBlend);
    }

    POINT dst = {windowRect.left, windowRect.top};
    SIZE size = {imageW, imageH};
    POINT src = {0, 0};
    BLENDFUNCTION layerBlend = {AC_SRC_OVER, 0, 255, AC_SRC_ALPHA};
    // Bug fix: regular WM_PAINT on a WS_EX_LAYERED image window can leave old scaled frames behind
    // during fast wheel zoom. Submitting the whole ARGB surface through UpdateLayeredWindow replaces
    // the previous pixels atomically, matching the shadow window's stable update path.
    BOOL updated = UpdateLayeredWindow(ow->hwnd, screenDC, &dst, &size, layerDC, &src, 0, &layerBlend, ULW_ALPHA);

    if (oldImage)
        SelectObject(imageDC, oldImage);
    if (imageDC)
        DeleteDC(imageDC);
    if (oldLayer)
        SelectObject(layerDC, oldLayer);
    DeleteObject(layerBitmap);
    DeleteDC(layerDC);
    ReleaseDC(NULL, screenDC);
    return updated;
}

static void ApplyAspectRatioToSizingRect(OverlayWindow *ow, WPARAM edge, RECT *rect)
{
    if (!ow || !rect || ow->aspectRatio <= 0.0f)
        return;

    int width = rect->right - rect->left;
    int height = rect->bottom - rect->top;
    if (width <= 0 || height <= 0)
        return;

    BOOL left = edge == WMSZ_LEFT || edge == WMSZ_TOPLEFT || edge == WMSZ_BOTTOMLEFT;
    BOOL right = edge == WMSZ_RIGHT || edge == WMSZ_TOPRIGHT || edge == WMSZ_BOTTOMRIGHT;
    BOOL top = edge == WMSZ_TOP || edge == WMSZ_TOPLEFT || edge == WMSZ_TOPRIGHT;
    BOOL bottom = edge == WMSZ_BOTTOM || edge == WMSZ_BOTTOMLEFT || edge == WMSZ_BOTTOMRIGHT;
    BOOL horizontal = left || right;
    BOOL vertical = top || bottom;

    int newWidth = width;
    int newHeight = height;
    if (horizontal)
    {
        newHeight = (int)roundf((float)newWidth / ow->aspectRatio);
    }
    else if (vertical)
    {
        newWidth = (int)roundf((float)newHeight * ow->aspectRatio);
    }

    UINT dpi = ow->dpi ? ow->dpi : 96;
    int minSize = MulDiv(MIN_RESIZE_SIZE_DIP, (int)dpi, 96);
    if (newWidth < minSize)
    {
        newWidth = minSize;
        newHeight = (int)roundf((float)newWidth / ow->aspectRatio);
    }
    if (newHeight < minSize)
    {
        newHeight = minSize;
        newWidth = (int)roundf((float)newHeight * ow->aspectRatio);
    }

    // Feature change: the native sizing rectangle is corrected before WM_SIZE so transparent image
    // overlays scale uniformly while the existing WM_SIZE path can still refresh the bitmap bounds
    // and rounded window region from one consistent final size.
    if (left)
    {
        rect->left = rect->right - newWidth;
    }
    else if (right)
    {
        rect->right = rect->left + newWidth;
    }
    else
    {
        int cx = (rect->left + rect->right) / 2;
        rect->left = cx - newWidth / 2;
        rect->right = rect->left + newWidth;
    }

    if (top)
    {
        rect->top = rect->bottom - newHeight;
    }
    else if (bottom)
    {
        rect->bottom = rect->top + newHeight;
    }
    else
    {
        int cy = (rect->top + rect->bottom) / 2;
        rect->top = cy - newHeight / 2;
        rect->bottom = rect->top + newHeight;
    }
}

static BOOL ZoomResizableImageOverlayAtScreenPoint(OverlayWindow *ow, POINT screenPt, int wheelDelta)
{
    if (!ow || !ow->hwnd || !ow->transparent || !ow->resizable || !ow->iconBitmap || wheelDelta == 0)
        return FALSE;

    RECT wr;
    if (!GetWindowRect(ow->hwnd, &wr))
        return FALSE;

    int currentWidth = wr.right - wr.left;
    int currentHeight = wr.bottom - wr.top;
    if (currentWidth <= 0 || currentHeight <= 0)
        return FALSE;

    UINT dpi = ow->dpi ? ow->dpi : GetWindowDpiSafe(ow->hwnd, 96);
    int minSize = MulDiv(MIN_RESIZE_SIZE_DIP, (int)dpi, 96);
    float factor = powf(WHEEL_ZOOM_STEP, (float)wheelDelta / (float)WHEEL_DELTA);
    int newWidth = (int)roundf((float)currentWidth * factor);
    int newHeight = (int)roundf((float)currentHeight * factor);

    if (ow->aspectRatio > 0.0f)
    {
        newWidth = max(minSize, newWidth);
        newHeight = (int)roundf((float)newWidth / ow->aspectRatio);
        if (newHeight < minSize)
        {
            newHeight = minSize;
            newWidth = (int)roundf((float)newHeight * ow->aspectRatio);
        }
    }
    else
    {
        newWidth = max(minSize, newWidth);
        newHeight = max(minSize, newHeight);
    }

    float anchorX = (float)(screenPt.x - wr.left) / (float)currentWidth;
    float anchorY = (float)(screenPt.y - wr.top) / (float)currentHeight;
    anchorX = min(1.0f, max(0.0f, anchorX));
    anchorY = min(1.0f, max(0.0f, anchorY));

    // Feature change: transparent image overlays could only be resized from their edges, which is
    // slow when inspecting preview images. Wheel zoom uses the same aspect/min-size constraints and
    // keeps the pixel under the cursor anchored so the overlay scales in place instead of jumping.
    int newX = screenPt.x - (int)roundf((float)newWidth * anchorX);
    int newY = screenPt.y - (int)roundf((float)newHeight * anchorY);
    SetWindowPos(ow->hwnd, NULL, newX, newY, newWidth, newHeight, SWP_NOACTIVATE | SWP_NOZORDER);
    return TRUE;
}

static void ShowTooltipWindow(OverlayWindow *ow, HWND owner, POINT clientPt);
static void HideTooltipWindow(OverlayWindow *ow);

static void ApplyOverlayLayout(OverlayWindow *ow)
{
    if (!ow || !ow->hwnd)
        return;

    ow->dpi = GetWindowDpiSafe(ow->hwnd, ow->dpi ? ow->dpi : GetSystemDpiSafe());
    float fontSizePt = (ow->fontSize > 0.0f) ? ow->fontSize : GetSystemMessageFontSizePt();
    float iconSizeDip = (ow->iconSize > 0.0f) ? ow->iconSize : DEFAULT_ICON_SIZE_DIP;

    if (!ow->messageFont || ow->fontDpi != ow->dpi || fabsf(ow->appliedFontSize - fontSizePt) > 0.01f)
    {
        if (ow->messageFont)
            DeleteObject(ow->messageFont);
        int fontHeight = -(int)roundf(fontSizePt * ((float)ow->dpi / 72.0f));
        if (fontHeight == 0)
            fontHeight = -1;
        ow->messageFont = CreateFontW(fontHeight, 0, 0, 0, FW_NORMAL, FALSE, FALSE, FALSE,
                                      DEFAULT_CHARSET, OUT_DEFAULT_PRECIS, CLIP_DEFAULT_PRECIS,
                                      CLEARTYPE_QUALITY, DEFAULT_PITCH | FF_DONTCARE, L"Segoe UI");
        ow->fontDpi = ow->dpi;
        ow->appliedFontSize = fontSizePt;
    }

    int width = 0;
    if (ow->width > 0)
        width = (int)roundf(ow->width * (float)ow->dpi / 96.0f);
    if (width <= 0)
        width = MulDiv(DEFAULT_WINDOW_WIDTH_DIP, (int)ow->dpi, 96);

    int minWidth = MulDiv(MIN_WINDOW_WIDTH_DIP, (int)ow->dpi, 96);
    if (width < minWidth)
        width = minWidth;

    int iconSize = (ow->iconBitmap ? (int)roundf(iconSizeDip * (float)ow->dpi / 96.0f) : 0);
    int iconGap = (ow->iconBitmap ? MulDiv(ICON_GAP_DIP, (int)ow->dpi, 96) : 0);
    int leftPad = MulDiv(PADDING_X_DIP, (int)ow->dpi, 96);
    int rightPad = MulDiv(PADDING_X_DIP, (int)ow->dpi, 96);
    int topPad = MulDiv(PADDING_Y_DIP, (int)ow->dpi, 96);
    int bottomPad = MulDiv(PADDING_Y_DIP, (int)ow->dpi, 96);

    int closeSize = ow->closable ? MulDiv(CLOSE_SIZE_DIP, (int)ow->dpi, 96) : 0;
    int closePad = ow->closable ? MulDiv(CLOSE_PAD_DIP, (int)ow->dpi, 96) : 0;

    float tooltipIconSizeDip = (ow->tooltipIconSize > 0.0f) ? ow->tooltipIconSize : DEFAULT_ICON_SIZE_DIP;
    int tooltipIconSize = (ow->tooltip ? (int)roundf(tooltipIconSizeDip * (float)ow->dpi / 96.0f) : 0);
    int tooltipIconGap = (ow->tooltip ? MulDiv(ICON_GAP_DIP, (int)ow->dpi, 96) : 0);

    int rightReserved = rightPad;
    if (ow->closable)
        rightReserved += closePad + closeSize;
    if (ow->tooltip)
        rightReserved += tooltipIconGap + tooltipIconSize;

    int textLeft = leftPad + iconSize + iconGap;
    int textRight = width - rightReserved;
    int textWidth = textRight - textLeft;
    if (textWidth < MulDiv(60, (int)ow->dpi, 96))
        textWidth = MulDiv(60, (int)ow->dpi, 96);

    int textHeight = 0;
    HDC hdc = GetDC(NULL);
    if (hdc)
    {
        HGDIOBJ oldFont = NULL;
        if (ow->messageFont)
            oldFont = SelectObject(hdc, ow->messageFont);
        textHeight = MeasureTextHeightW(hdc, ow->message ? ow->message : L"", textWidth);
        if (oldFont)
            SelectObject(hdc, oldFont);
        ReleaseDC(NULL, hdc);
    }

    int contentHeight = textHeight;
    if (iconSize > contentHeight)
        contentHeight = iconSize;
    if (closeSize > contentHeight)
        contentHeight = closeSize;
    if (tooltipIconSize > contentHeight)
        contentHeight = tooltipIconSize;

    int height = 0;
    if (ow->height > 0)
        height = (int)roundf(ow->height * (float)ow->dpi / 96.0f);
    if (height <= 0)
        height = topPad + bottomPad + contentHeight;

    if (ow->transparent)
    {
        // Bug fix: transparent image overlays must keep the whole client area as the screenshot.
        // Expanding this layered client area for an inline shadow becomes opaque black padding, so
        // the image HWND stays pure and a separate per-pixel shadow HWND provides separation.
        int drawW = ow->iconBitmap ? (int)roundf(((ow->iconDrawWidth > 0.0f) ? ow->iconDrawWidth : iconSizeDip) * (float)ow->dpi / 96.0f) : 0;
        int drawH = ow->iconBitmap ? (int)roundf(((ow->iconDrawHeight > 0.0f) ? ow->iconDrawHeight : iconSizeDip) * (float)ow->dpi / 96.0f) : 0;
        int iconX = (int)roundf(ow->iconX * (float)ow->dpi / 96.0f);
        int iconY = (int)roundf(ow->iconY * (float)ow->dpi / 96.0f);
        if (ow->resizable)
        {
            iconX = 0;
            iconY = 0;
            drawW = width;
            drawH = height;
        }
        RECT iconRect = {iconX, iconY, iconX + drawW, iconY + drawH};
        ow->iconRect = iconRect;
        RECT empty = {0, 0, 0, 0};
        ow->closeRect = empty;
        ow->tooltipRect = empty;
    }
    else
    {
        UpdateCloseRect(ow, width, height, ow->dpi);

        if (ow->tooltip)
        {
            int tx = textLeft + textWidth + tooltipIconGap;
            // Center vertically in content area?
            // Content area starts at topPad. contentHeight is height of content.
            // Center of content area: topPad + contentHeight / 2
            int cy = topPad + contentHeight / 2;
            int ty = cy - tooltipIconSize / 2;
            if (ty < topPad) ty = topPad;

            RECT r = {tx, ty, tx + tooltipIconSize, ty + tooltipIconSize};
            ow->tooltipRect = r;
        }
        else
        {
            RECT r = {0,0,0,0};
            ow->tooltipRect = r;
        }
    }

    RECT targetRect;
    BOOL targetFound = FALSE;
    BOOL preserveLiveFollowFrame = ow->stickyWindowPid > 0 && ow->liveFollowActive;
    if (ow->stickyWindowPid > 0)
    {
        HWND target = NULL;
        if (FindWindowByPid(ow->stickyWindowPid, &target))
        {
            ow->targetHwnd = target;
            if (GetTargetContentRect(target, &targetRect))
            {
                targetFound = TRUE;
                if (IsLeftButtonDown())
                {
                    // Optimization: pet frame refreshes can arrive while the user
                    // is already dragging the target window. Seeding the predictive
                    // anchor from this layout path reduces initial lag before the
                    // first WinEvent location notification is delivered.
                    RefreshPredictiveAnchor(ow, &targetRect);
                    StartLiveFollowTimerIfNeeded(ow);
                }
            }
            else
            {
                SystemParametersInfo(SPI_GETWORKAREA, 0, &targetRect, 0);
            }
            SetOverlayZOrder(ow->hwnd, target);
        }
        else
        {
            ow->targetHwnd = NULL;
            SystemParametersInfo(SPI_GETWORKAREA, 0, &targetRect, 0);
            SetOverlayZOrder(ow->hwnd, NULL);
        }
    }
    else
    {
        if (ow->absolutePosition)
        {
            // Bug fix: pinned screenshots pass desktop-absolute selection coordinates. The old
            // screen branch anchored those offsets to the primary work area and then clamped them,
            // which moved pins from secondary/negative-coordinate monitors back onto the main
            // screen. A zero-origin target lets the already-absolute offset land unchanged.
            SetRect(&targetRect, 0, 0, 0, 0);
        }
        else
        {
            SystemParametersInfo(SPI_GETWORKAREA, 0, &targetRect, 0);
        }
        SetOverlayZOrder(ow->hwnd, NULL);
    }

    ow->targetReady = (ow->stickyWindowPid <= 0) ? TRUE : targetFound;

    int x = 0;
    int y = 0;
    ComputeOverlayPosition(ow, &targetRect, width, height, &x, &y);


    if (ow->stickyWindowPid > 0)
    {
        ow->lastTargetRect = targetRect;
        ow->hasLastTargetRect = TRUE;
    }

    RECT workArea = GetWorkAreaForRect(&targetRect);
    if (!ow->absolutePosition)
    {
        ClampWindowToWorkArea(&workArea, &x, &y, width, height);
    }
    if (preserveLiveFollowFrame)
    {
        RECT current;
        if (GetWindowRect(ow->hwnd, &current))
        {
            // Bug fix: content refreshes should not re-anchor a sticky overlay
            // from a stale window sample during live follow. Preserve the current
            // predicted origin and let the live timer own position updates while
            // this layout pass only updates size and drawing metrics.
            x = current.left;
            y = current.top;
        }
    }

    SetWindowPos(ow->hwnd, NULL, x, y, width, height, SWP_NOACTIVATE | SWP_NOZORDER);
    if (ow->transparent)
    {
        // Feature change: the screenshot surface remains a clean image-only layered window. The
        // visible depth now comes from an independent shadow HWND, so no region or client padding
        // is applied to the pin window itself.
        SetWindowRgn(ow->hwnd, NULL, TRUE);
        UINT pref = DWMWCP_DONOTROUND;
        DwmSetWindowAttribute(ow->hwnd, DWMWA_WINDOW_CORNER_PREFERENCE, &pref, sizeof(pref));
        UpdateTransparentImageLayer(ow);
        UpdateTransparentImageShadow(ow);
    }
    else
    {
        DestroyTransparentImageShadow(ow);
        ApplyCornerRadius(ow->hwnd, ow->dpi, width, height);
    }

    if (ow->tooltipHwnd)
    {
        SetWindowPos(ow->tooltipHwnd, HWND_TOPMOST, 0, 0, 0, 0, SWP_NOMOVE | SWP_NOSIZE | SWP_NOACTIVATE);
        if (!ow->tooltip || !*ow->tooltip)
            HideTooltipWindow(ow);
    }

    StartAutoCloseTimer(ow);
    StartTrackTimer(ow);
}

static void ApplyPayloadToOverlay(OverlayWindow *ow, OverlayPayload *payload, BOOL isNew)
{
    if (!ow || !payload)
        return;

    int previousStickyWindowPid = ow->stickyWindowPid;

    if (!isNew)
    {
        if (ow->title)
            free(ow->title);
        if (ow->message)
            free(ow->message);
        if (ow->tooltip)
            free(ow->tooltip);
    }

    if (isNew)
        ow->name = payload->name;
    else if (payload->name)
        free(payload->name);

    ow->title = payload->title;
    ow->message = payload->message;
    ow->tooltip = payload->tooltip;

    if (ow->iconBitmap)
        DeleteObject(ow->iconBitmap);
    ow->iconBitmap = NULL;
    ow->iconWidth = 0;
    ow->iconHeight = 0;

    if ((payload->iconFilePath && payload->iconFilePath[0]) || (payload->iconData && payload->iconLen > 0))
    {
        int iw = 0;
        int ih = 0;
        BOOL useFileIcon = payload->iconFilePath && payload->iconFilePath[0];
        HBITMAP bmp = useFileIcon ? CreateBitmapFromImageFilePath(payload->iconFilePath, &iw, &ih) : CreateBitmapFromPngData(payload->iconData, payload->iconLen, &iw, &ih);
        if (bmp)
        {
            ow->iconBitmap = bmp;
            ow->iconWidth = iw;
            ow->iconHeight = ih;
        }
    }

    if (ow->tooltipIconBitmap)
        DeleteObject(ow->tooltipIconBitmap);
    ow->tooltipIconBitmap = NULL;
    ow->tooltipIconWidth = 0;
    ow->tooltipIconHeight = 0;

    if (payload->tooltipIconData && payload->tooltipIconLen > 0)
    {
        int iw = 0;
        int ih = 0;
        HBITMAP bmp = CreateBitmapFromPngData(payload->tooltipIconData, payload->tooltipIconLen, &iw, &ih);
        if (bmp)
        {
            ow->tooltipIconBitmap = bmp;
            ow->tooltipIconWidth = iw;
            ow->tooltipIconHeight = ih;
        }
    }

    if (payload->iconData)
        free(payload->iconData);
    if (payload->iconFilePath)
        free(payload->iconFilePath);

    if (payload->tooltipIconData)
        free(payload->tooltipIconData);

    ow->closable = payload->closable;
    ow->closeOnEscape = payload->closeOnEscape;
    ow->loading = payload->loading;
    ow->topmost = payload->topmost;
    ow->absolutePosition = payload->absolutePosition;
    ow->transparent = payload->transparent;
    ow->hitTestIconOnly = payload->hitTestIconOnly;
    ow->iconX = payload->iconX;
    ow->iconY = payload->iconY;
    ow->iconDrawWidth = payload->iconWidth;
    ow->iconDrawHeight = payload->iconHeight;
    ow->stickyWindowPid = payload->stickyWindowPid;
    ow->anchor = payload->anchor;
    ow->autoCloseSeconds = payload->autoCloseSeconds;
    ow->movable = payload->movable;
    ow->resizable = payload->resizable;
    ow->cornerRadius = payload->cornerRadius;
    ow->aspectRatio = payload->aspectRatio > 0.0f ? payload->aspectRatio : 0.0f;
    ow->offsetX = payload->offsetX;
    ow->offsetY = payload->offsetY;
    ow->width = payload->width;
    ow->height = payload->height;
    ow->fontSize = payload->fontSize;
    ow->iconSize = payload->iconSize;
    ow->tooltipIconSize = payload->tooltipIconSize;
    ow->hasLastTargetRect = FALSE;
    ow->hiddenForMove = FALSE;
    if (ow->hwnd)
    {
        LONG_PTR style = GetWindowLongPtrW(ow->hwnd, GWL_STYLE);
        LONG_PTR updatedStyle = (ow->resizable && !ow->transparent) ? (style | WS_THICKFRAME) : (style & ~WS_THICKFRAME);
        if (updatedStyle != style)
        {
            // Bug fix: reused image overlay windows can switch between HUD and transparent image
            // modes. Keep the thick frame off transparent screenshots so the pinned image is the
            // full client surface instead of being inset behind a system-drawn border.
            SetWindowLongPtrW(ow->hwnd, GWL_STYLE, updatedStyle);
            SetWindowPos(ow->hwnd, NULL, 0, 0, 0, 0,
                         SWP_NOMOVE | SWP_NOSIZE | SWP_NOZORDER | SWP_NOACTIVATE | SWP_FRAMECHANGED);
        }

        LONG_PTR exStyle = GetWindowLongPtrW(ow->hwnd, GWL_EXSTYLE);
        LONG_PTR updatedExStyle = ow->transparent ? (exStyle | WS_EX_LAYERED) : (exStyle & ~WS_EX_LAYERED);
        if (updatedExStyle != exStyle)
        {
            // Bug fix: URL image overlays can reuse a non-transparent loading window for the final
            // transparent image. Keep the extended layered style aligned with the current payload so
            // the image surface can use the atomic UpdateLayeredWindow path instead of stale paint.
            SetWindowLongPtrW(ow->hwnd, GWL_EXSTYLE, updatedExStyle);
        }
    }
    if (!isNew && previousStickyWindowPid != ow->stickyWindowPid)
    {
        // Bug fix: predictive anchors are tied to a specific target window. When
        // an overlay is reused for a different sticky PID, keep the generic API
        // simple by clearing the old live-follow state at the platform boundary.
        StopLiveFollowTimer(ow);
        if (ow->locationHook)
        {
            UnhookWinEvent(ow->locationHook);
            ow->locationHook = NULL;
        }
        ow->hasPredictiveAnchor = FALSE;
        ow->targetHwnd = NULL;
    }

    if (ow->title && ow->hwnd)
        SetWindowTextW(ow->hwnd, ow->title);

    free(payload);
}

static void DrawCloseButton(HDC hdc, const RECT *rect, UINT dpi, BOOL hover, BOOL pressed)
{
    if (!rect)
        return;

    if (hover || pressed)
    {
        COLORREF bg = pressed ? RGB(70, 70, 70) : RGB(55, 55, 55);
        HBRUSH brush = CreateSolidBrush(bg);
        FillRect(hdc, rect, brush);
        DeleteObject(brush);
    }

    int pad = MulDiv(6, (int)dpi, 96);
    int thickness = MulDiv(2, (int)dpi, 96);
    if (thickness < 1)
        thickness = 1;

    HPEN pen = CreatePen(PS_SOLID, thickness, RGB(230, 230, 230));
    HGDIOBJ oldPen = SelectObject(hdc, pen);

    MoveToEx(hdc, rect->left + pad, rect->top + pad, NULL);
    LineTo(hdc, rect->right - pad, rect->bottom - pad);
    MoveToEx(hdc, rect->right - pad, rect->top + pad, NULL);
    LineTo(hdc, rect->left + pad, rect->bottom - pad);

    if (oldPen)
        SelectObject(hdc, oldPen);
    DeleteObject(pen);
}

static void HandleOverlayClick(OverlayWindow *ow)
{
    if (!ow || !ow->name)
        return;
    char *nameUtf8 = DupWideToUtf8(ow->name);
    if (!nameUtf8)
        return;
    overlayClickCallbackCGO(nameUtf8);
    free(nameUtf8);
}

static HFONT g_tooltipFont = NULL;
static UINT g_tooltipFontDpi = 0;
static float g_tooltipFontSizePt = 0.0f;

static HFONT GetTooltipFont(UINT dpi)
{
    float fontSizePt = GetSystemMessageFontSizePt();
    if (!g_tooltipFont || g_tooltipFontDpi != dpi || fabsf(g_tooltipFontSizePt - fontSizePt) > 0.01f)
    {
        if (g_tooltipFont)
            DeleteObject(g_tooltipFont);
        int fontHeight = -(int)roundf(fontSizePt * ((float)dpi / 72.0f));
        if (fontHeight == 0)
            fontHeight = -1;
        g_tooltipFont = CreateFontW(fontHeight, 0, 0, 0, FW_NORMAL, FALSE, FALSE, FALSE,
                                    DEFAULT_CHARSET, OUT_DEFAULT_PRECIS, CLIP_DEFAULT_PRECIS,
                                    CLEARTYPE_QUALITY, DEFAULT_PITCH | FF_DONTCARE, L"Segoe UI");
        g_tooltipFontDpi = dpi;
        g_tooltipFontSizePt = fontSizePt;
    }
    return g_tooltipFont;
}

static void MeasureTooltipTextRect(HDC hdc, const WCHAR *text, int maxWidth, RECT *outRect)
{
    RECT rc = {0, 0, maxWidth, 0};
    if (!text)
        text = L"";
    DrawTextW(hdc, text, -1, &rc, DT_CALCRECT | DT_WORDBREAK | DT_NOPREFIX);
    if (outRect)
        *outRect = rc;
}

static void ShowTooltipWindow(OverlayWindow *ow, HWND owner, POINT clientPt)
{
    if (!ow || !ow->tooltipHwnd || !ow->tooltip || !*ow->tooltip)
        return;

    UINT dpi = ow->dpi ? ow->dpi : GetWindowDpiSafe(owner, 96);
    int pad = MulDiv(8, (int)dpi, 96);
    int maxWidth = MulDiv(400, (int)dpi, 96);
    int gap = MulDiv(TOOLTIP_GAP_DIP, (int)dpi, 96);

    HDC hdc = GetDC(NULL);
    RECT textRc = {0, 0, maxWidth, 0};
    if (hdc)
    {
        HFONT font = GetTooltipFont(dpi);
        HGDIOBJ oldFont = NULL;
        if (font)
            oldFont = SelectObject(hdc, font);
        MeasureTooltipTextRect(hdc, ow->tooltip, maxWidth, &textRc);
        if (oldFont)
            SelectObject(hdc, oldFont);
        ReleaseDC(NULL, hdc);
    }

    int textW = textRc.right - textRc.left;
    int textH = textRc.bottom - textRc.top;
    if (textW < 1)
        textW = 1;
    if (textH < 1)
        textH = 1;

    int width = textW + pad * 2;
    int height = textH + pad * 2;

    RECT iconRc = ow->tooltipRect;
    POINT tl = {iconRc.left, iconRc.top};
    POINT br = {iconRc.right, iconRc.bottom};
    ClientToScreen(owner, &tl);
    ClientToScreen(owner, &br);

    int iconW = br.x - tl.x;
    int iconH = br.y - tl.y;
    if (iconW < 1)
        iconW = 1;
    if (iconH < 1)
        iconH = 1;

    int x = tl.x + (iconW - width) / 2;
    int y = br.y + gap;
    RECT anchor = {tl.x, tl.y, br.x, br.y};
    RECT work = GetWorkAreaForRect(&anchor);
    if (y + height > work.bottom)
        y = tl.y - height - gap;
    if (x + width > work.right)
        x = work.right - width;
    if (x < work.left)
        x = work.left;
    if (y < work.top)
        y = work.top;

    SetWindowPos(ow->tooltipHwnd, HWND_TOPMOST, x, y, width, height,
                 SWP_NOACTIVATE | SWP_SHOWWINDOW);
    InvalidateRect(ow->tooltipHwnd, NULL, TRUE);

    LogOverlayTooltip(L"[WoxOverlayTooltip] show x=%d y=%d w=%d h=%d icon=(%d,%d,%d,%d) topmost=1",
                      x, y, width, height, tl.x, tl.y, br.x, br.y);
}

static void HideTooltipWindow(OverlayWindow *ow)
{
    if (!ow || !ow->tooltipHwnd)
        return;
    ShowWindow(ow->tooltipHwnd, SW_HIDE);
    LogOverlayTooltip(L"[WoxOverlayTooltip] hide");
}

static LRESULT CALLBACK TooltipWindowProc(HWND hwnd, UINT uMsg, WPARAM wParam, LPARAM lParam)
{
    if (uMsg == WM_NCCREATE)
    {
        CREATESTRUCT *cs = (CREATESTRUCT *)lParam;
        if (cs && cs->lpCreateParams)
            SetWindowLongPtr(hwnd, GWLP_USERDATA, (LONG_PTR)cs->lpCreateParams);
        return DefWindowProc(hwnd, uMsg, wParam, lParam);
    }

    OverlayWindow *ow = (OverlayWindow *)GetWindowLongPtr(hwnd, GWLP_USERDATA);
    switch (uMsg)
    {
    case WM_ERASEBKGND:
        return 1;
    case WM_PAINT:
    {
        if (!ow)
            break;
        PAINTSTRUCT ps;
        HDC hdc = BeginPaint(hwnd, &ps);
        RECT rc;
        GetClientRect(hwnd, &rc);

        HBRUSH bg = CreateSolidBrush(RGB(32, 32, 32));
        FillRect(hdc, &rc, bg);
        DeleteObject(bg);

        UINT dpi = GetWindowDpiSafe(hwnd, ow->dpi ? ow->dpi : 96);
        int pad = MulDiv(8, (int)dpi, 96);
        HFONT font = GetTooltipFont(dpi);
        HGDIOBJ oldFont = NULL;
        if (font)
            oldFont = SelectObject(hdc, font);
        SetBkMode(hdc, TRANSPARENT);
        SetTextColor(hdc, RGB(240, 240, 240));

        RECT textRc = rc;
        InflateRect(&textRc, -pad, -pad);
        DrawTextW(hdc, ow->tooltip ? ow->tooltip : L"", -1, &textRc,
                  DT_LEFT | DT_WORDBREAK | DT_NOPREFIX);

        if (oldFont)
            SelectObject(hdc, oldFont);
        EndPaint(hwnd, &ps);
        return 0;
    }
    }

    return DefWindowProc(hwnd, uMsg, wParam, lParam);
}

// -----------------------------------------------------------------------------
// Window Proc
// -----------------------------------------------------------------------------

static void CALLBACK OverlayLocationChangeHook(HWINEVENTHOOK hWinEventHook, DWORD event, HWND hwnd, 
                                               LONG idObject, LONG idChild, DWORD dwEventThread, DWORD dwmsEventTime)
{
    if (!g_overlays || idObject != OBJID_WINDOW)
        return;

    // Linear search for overlays tracking this window.
    for (OverlayWindow *ow = g_overlays; ow; ow = ow->next)
    {
        if (ow->targetHwnd == hwnd && ow->hwnd && IsWindow(ow->hwnd))
        {
            PostMessageW(ow->hwnd, WM_WOX_OVERLAY_REPOSITION, 0, 0);
        }
    }
}

static LRESULT CALLBACK OverlayWindowProc(HWND hwnd, UINT uMsg, WPARAM wParam, LPARAM lParam)
{
    if (uMsg == WM_NCCREATE)
    {
        CREATESTRUCT *cs = (CREATESTRUCT *)lParam;
        if (cs && cs->lpCreateParams)
            SetWindowLongPtr(hwnd, GWLP_USERDATA, (LONG_PTR)cs->lpCreateParams);
        return DefWindowProc(hwnd, uMsg, wParam, lParam);
    }

    OverlayWindow *ow = (OverlayWindow *)GetWindowLongPtr(hwnd, GWLP_USERDATA);

    switch (uMsg)
    {
    case WM_CREATE:
    {
        BOOL dark = TRUE;
        DwmSetWindowAttribute(hwnd, DWMWA_USE_IMMERSIVE_DARK_MODE, &dark, sizeof(dark));
        UINT cornerPreference = (ow && ow->transparent) ? DWMWCP_DONOTROUND : DWMWCP_ROUND;
        DwmSetWindowAttribute(hwnd, DWMWA_WINDOW_CORNER_PREFERENCE, &cornerPreference, sizeof(cornerPreference));

        if (!(ow && ow->transparent))
        {
            BOOL accentOk = TryEnableAcrylic(hwnd);
            if (!accentOk)
                accentOk = TryEnableHostBackdrop(hwnd);

            if (accentOk)
            {
                MARGINS margins = {0, 0, 0, 0};
                DwmExtendFrameIntoClientArea(hwnd, &margins);

                UINT noneBackdrop = DWMSBT_NONE;
                DwmSetWindowAttribute(hwnd, DWMWA_SYSTEMBACKDROP_TYPE, &noneBackdrop, sizeof(noneBackdrop));
            }
            else
            {
                UINT backdrop = DWMSBT_TRANSIENTWINDOW;
                HRESULT hrBackdrop = DwmSetWindowAttribute(hwnd, DWMWA_SYSTEMBACKDROP_TYPE, &backdrop, sizeof(backdrop));
                if (SUCCEEDED(hrBackdrop))
                {
                    MARGINS margins = {-1};
                    DwmExtendFrameIntoClientArea(hwnd, &margins);
                }
            }
        }

        ow->tooltipHwnd = CreateWindowExW(WS_EX_TOPMOST | WS_EX_TOOLWINDOW | WS_EX_NOACTIVATE,
                                          g_tooltipClassName, L"",
                                          WS_POPUP,
                                          CW_USEDEFAULT, CW_USEDEFAULT,
                                          CW_USEDEFAULT, CW_USEDEFAULT,
                                          hwnd, NULL, GetModuleHandleW(NULL), ow);
        if (ow->tooltipHwnd)
        {
            SetWindowPos(ow->tooltipHwnd, HWND_TOPMOST, 0, 0, 0, 0, SWP_NOMOVE | SWP_NOSIZE | SWP_NOACTIVATE);
            LogOverlayTooltip(L"[WoxOverlayTooltip] created hwnd=%p text=%ls", ow->tooltipHwnd, ow->tooltip ? ow->tooltip : L"(null)");
        }
        return 0;
    }
    case WM_ERASEBKGND:
        return 1;
    case WM_DPICHANGED:
    {
        if (!ow)
            return 0;
        ow->dpi = HIWORD(wParam);
        if (ow->dragging)
        {
            // Bug fix: crossing monitors with different DPI sends WM_DPICHANGED while the custom
            // drag loop is still positioning the overlay from raw screen pixels. Re-running the
            // normal anchor layout here snaps preview overlays back to the primary work area, so
            // keep the drag-owned frame and only refresh DPI-sensitive transparent drawing assets.
            if (ow->transparent)
            {
                UpdateTransparentImageLayer(ow);
                UpdateTransparentImageShadow(ow);
            }
            return 0;
        }
        RECT *suggested = (RECT *)lParam;
        if (suggested)
        {
            SetWindowPos(hwnd, NULL, suggested->left, suggested->top,
                         suggested->right - suggested->left,
                         suggested->bottom - suggested->top,
                         SWP_NOACTIVATE | SWP_NOZORDER);
        }
        ApplyOverlayLayout(ow);
        InvalidateRect(hwnd, NULL, TRUE);
        return 0;
    }
    case WM_GETMINMAXINFO:
    {
        if (ow && ow->resizable)
        {
            MINMAXINFO *mmi = (MINMAXINFO *)lParam;
            UINT dpi = ow->dpi ? ow->dpi : GetWindowDpiSafe(hwnd, 96);
            int minSize = MulDiv(MIN_RESIZE_SIZE_DIP, (int)dpi, 96);
            mmi->ptMinTrackSize.x = minSize;
            mmi->ptMinTrackSize.y = minSize;
            return 0;
        }
        break;
    }
    case WM_SIZING:
    {
        if (ow && ow->transparent && ow->resizable && ow->aspectRatio > 0.0f)
        {
            ApplyAspectRatioToSizingRect(ow, wParam, (RECT *)lParam);
            return TRUE;
        }
        break;
    }
    case WM_SIZE:
    {
        if (ow && ow->transparent && ow->resizable)
        {
            RECT client;
            GetClientRect(hwnd, &client);
            ow->iconRect = client;
            SetWindowRgn(hwnd, NULL, TRUE);
            UpdateTransparentImageLayer(ow);
            UpdateTransparentImageShadow(ow);
            return 0;
        }
        break;
    }
    case WM_PAINT:
    {
        if (!ow)
            break;
        PAINTSTRUCT ps;
        HDC paintHdc = BeginPaint(hwnd, &ps);

        RECT client;
        GetClientRect(hwnd, &client);
        ow->dpi = GetWindowDpiSafe(hwnd, ow->dpi ? ow->dpi : 96);
        int width = client.right - client.left;
        int height = client.bottom - client.top;

        HDC hdc = paintHdc;
        HPAINTBUFFER paintBuf = BeginBufferedPaint(paintHdc, &client, BPBF_TOPDOWNDIB, NULL, &hdc);
        if (paintBuf)
        {
            BufferedPaintClear(paintBuf, &client);
        }

        if (ow->transparent)
        {
            // Bug fix: transparent image overlays are painted by UpdateTransparentImageLayer.
            // Leaving WM_PAINT to AlphaBlend into a layered client DC during rapid wheel zoom caused
            // stale scaled image rectangles to remain in DWM composition until a later repaint.
            if (paintBuf)
                EndBufferedPaint(paintBuf, FALSE);
            EndPaint(hwnd, &ps);
            return 0;
        }

        int leftPad = MulDiv(PADDING_X_DIP, (int)ow->dpi, 96);
        int rightPad = MulDiv(PADDING_X_DIP, (int)ow->dpi, 96);
        int topPad = MulDiv(PADDING_Y_DIP, (int)ow->dpi, 96);
        int bottomPad = MulDiv(PADDING_Y_DIP, (int)ow->dpi, 96);

        float iconSizeDip = (ow->iconSize > 0.0f) ? ow->iconSize : DEFAULT_ICON_SIZE_DIP;
        int iconSize = (ow->iconBitmap ? (int)roundf(iconSizeDip * (float)ow->dpi / 96.0f) : 0);
        int iconGap = (ow->iconBitmap ? MulDiv(ICON_GAP_DIP, (int)ow->dpi, 96) : 0);

        float tooltipIconSizeDip = (ow->tooltipIconSize > 0.0f) ? ow->tooltipIconSize : DEFAULT_ICON_SIZE_DIP;
        int tooltipIconSize = (ow->tooltip ? (int)roundf(tooltipIconSizeDip * (float)ow->dpi / 96.0f) : 0);
        int tooltipIconGap = (ow->tooltip ? MulDiv(ICON_GAP_DIP, (int)ow->dpi, 96) : 0);

        int closeSize = ow->closable ? MulDiv(CLOSE_SIZE_DIP, (int)ow->dpi, 96) : 0;
        int closePad = ow->closable ? MulDiv(CLOSE_PAD_DIP, (int)ow->dpi, 96) : 0;

        int rightReserved = rightPad;
        if (ow->closable)
            rightReserved += closePad + closeSize;
        if (ow->tooltip)
            rightReserved += tooltipIconGap + tooltipIconSize;

        int textLeft = leftPad + iconSize + iconGap;
        int textRight = width - rightReserved;
        RECT textRect = {textLeft, topPad, textRight, height - bottomPad};

        SetBkMode(hdc, TRANSPARENT);
        SetTextColor(hdc, RGB(240, 240, 240));
        if (ow->messageFont)
            SelectObject(hdc, ow->messageFont);
        DrawTextW(hdc, ow->message ? ow->message : L"", -1, &textRect,
                  DT_LEFT | DT_TOP | DT_WORDBREAK | DT_EDITCONTROL | DT_NOPREFIX);

        if (ow->iconBitmap)
        {
            int iconX = leftPad;
            int iconY = (height - iconSize) / 2;
            if (iconY < topPad)
                iconY = topPad;
            if (iconY + iconSize > height - bottomPad)
                iconY = height - bottomPad - iconSize;
            if (iconY < 0)
                iconY = 0;

            HDC memDC = CreateCompatibleDC(hdc);
            if (memDC)
            {
                HGDIOBJ oldBmp = SelectObject(memDC, ow->iconBitmap);
                BLENDFUNCTION bf = {AC_SRC_OVER, 0, 255, AC_SRC_ALPHA};
                AlphaBlend(hdc, iconX, iconY, iconSize, iconSize, memDC, 0, 0, ow->iconWidth, ow->iconHeight, bf);
                if (oldBmp)
                    SelectObject(memDC, oldBmp);
                DeleteDC(memDC);
            }
        }

        if (ow->tooltip && ow->tooltipIconBitmap)
        {
            HDC memDC = CreateCompatibleDC(hdc);
            if (memDC)
            {
                HGDIOBJ oldBmp = SelectObject(memDC, ow->tooltipIconBitmap);
                BLENDFUNCTION bf = {AC_SRC_OVER, 0, 255, AC_SRC_ALPHA};
                AlphaBlend(hdc, ow->tooltipRect.left, ow->tooltipRect.top, 
                           ow->tooltipRect.right - ow->tooltipRect.left, 
                           ow->tooltipRect.bottom - ow->tooltipRect.top, 
                           memDC, 0, 0, ow->tooltipIconWidth, ow->tooltipIconHeight, bf);
                if (oldBmp)
                    SelectObject(memDC, oldBmp);
                DeleteDC(memDC);
            }
        }

        if (ow->closable)
        {
            DrawCloseButton(hdc, &ow->closeRect, ow->dpi, ow->closeHover, ow->closePressed);
        }

        if (paintBuf)
        {
            EndBufferedPaint(paintBuf, TRUE);
        }

        EndPaint(hwnd, &ps);
        return 0;
    }
    case WM_NCHITTEST:
    {
        if (ow && ow->resizable)
        {
            POINT pt = {GET_X_LPARAM(lParam), GET_Y_LPARAM(lParam)};
            ScreenToClient(hwnd, &pt);
            LRESULT resizeHit = GetResizeHitTest(ow, pt);
            if (resizeHit != HTCLIENT)
                return resizeHit;
        }
        if (ow && ow->transparent && ow->hitTestIconOnly)
        {
            POINT pt = {GET_X_LPARAM(lParam), GET_Y_LPARAM(lParam)};
            ScreenToClient(hwnd, &pt);
            if (!PtInRect(&ow->iconRect, pt))
                return HTTRANSPARENT;
        }
        break;
    }
    case WM_SETCURSOR:
    {
        if (!ow)
            break;
        switch (LOWORD(lParam))
        {
        case HTTOPLEFT:
        case HTBOTTOMRIGHT:
            // Feature change: resize hit testing is custom for borderless image overlays, so set
            // the matching system cursors explicitly instead of relying on the hidden frame.
            SetCursor(LoadCursor(NULL, IDC_SIZENWSE));
            return TRUE;
        case HTTOPRIGHT:
        case HTBOTTOMLEFT:
            SetCursor(LoadCursor(NULL, IDC_SIZENESW));
            return TRUE;
        case HTLEFT:
        case HTRIGHT:
            SetCursor(LoadCursor(NULL, IDC_SIZEWE));
            return TRUE;
        case HTTOP:
        case HTBOTTOM:
            SetCursor(LoadCursor(NULL, IDC_SIZENS));
            return TRUE;
        default:
            break;
        }
        if (LOWORD(lParam) == HTCLIENT)
        {
            POINT pt;
            if (GetCursorPos(&pt))
            {
                ScreenToClient(hwnd, &pt);
                if (ow->closable && PtInRect(&ow->closeRect, pt))
                {
                    SetCursor(LoadCursor(NULL, IDC_HAND));
                    return TRUE;
                }
            }
        }
        break;
    }
    case WM_MOUSEMOVE:
    {
        if (!ow)
            break;
        if (!ow->mouseInside)
        {
            ow->mouseInside = TRUE;
            TRACKMOUSEEVENT tme = {sizeof(TRACKMOUSEEVENT), TME_LEAVE, hwnd, 0};
            TrackMouseEvent(&tme);
        }

        POINT pt = {GET_X_LPARAM(lParam), GET_Y_LPARAM(lParam)};
        if (ow->tooltipHwnd && ow->tooltip && *ow->tooltip)
        {
            BOOL hoverTooltip = PtInRect(&ow->tooltipRect, pt);
            if (hoverTooltip != ow->tooltipHover)
            {
                ow->tooltipHover = hoverTooltip;
                if (hoverTooltip)
                {
                    ShowTooltipWindow(ow, hwnd, pt);
                }
                else
                {
                    HideTooltipWindow(ow);
                }
            }
        }
        BOOL hoverNow = ow->closable && PtInRect(&ow->closeRect, pt);
        if (hoverNow != ow->closeHover)
        {
            ow->closeHover = hoverNow;
            InvalidateRect(hwnd, NULL, FALSE);
        }

        if (ow->dragging)
        {
            POINT screenPt;
            GetCursorPos(&screenPt);
            int dx = screenPt.x - ow->dragStart.x;
            int dy = screenPt.y - ow->dragStart.y;
            SetWindowPos(hwnd, NULL, ow->dragWindowOrigin.x + dx, ow->dragWindowOrigin.y + dy, 0, 0,
                         SWP_NOACTIVATE | SWP_NOZORDER | SWP_NOSIZE);
            // Feature change: pinned screenshots use a companion shadow HWND. Moving only the image
            // would leave the shadow behind. Dragging changes only origin, so move the cached
            // shadow surface instead of rebuilding it for every mouse event.
            MoveTransparentImageShadow(ow);
        }
        return 0;
    }
    case WM_MOUSELEAVE:
    {
        if (!ow)
            break;
        ow->mouseInside = FALSE;
        ow->closeHover = FALSE;
        if (ow->tooltipHwnd && ow->tooltipHover)
        {
            ow->tooltipHover = FALSE;
            HideTooltipWindow(ow);
        }
        if (!ow->closePressed)
            InvalidateRect(hwnd, NULL, FALSE);
        if (ow->autoClosePending && !ow->dragging)
        {
            DestroyWindow(hwnd);
        }
        return 0;
    }
    case WM_MOUSEWHEEL:
    {
        if (!ow)
            break;
        POINT screenPt = {GET_X_LPARAM(lParam), GET_Y_LPARAM(lParam)};
        if (ZoomResizableImageOverlayAtScreenPoint(ow, screenPt, GET_WHEEL_DELTA_WPARAM(wParam)))
            return 0;
        break;
    }
    case WM_LBUTTONDOWN:
    {
        if (!ow)
            break;
        if (ow->closeOnEscape)
        {
            // Focus-sensitive overlays must receive Escape themselves. Setting focus only when this
            // option is enabled keeps notification overlays non-activating while pinned screenshots
            // close one focused image at a time.
            SetFocus(hwnd);
        }
        POINT pt = {GET_X_LPARAM(lParam), GET_Y_LPARAM(lParam)};
        if (ow->closable && PtInRect(&ow->closeRect, pt))
        {
            ow->closePressed = TRUE;
            SetCapture(hwnd);
            InvalidateRect(hwnd, NULL, FALSE);
            return 0;
        }
        if (ow->movable)
        {
            ow->dragging = TRUE;
            SetCapture(hwnd);
            GetCursorPos(&ow->dragStart);
            RECT wr;
            GetWindowRect(hwnd, &wr);
            ow->dragWindowOrigin.x = wr.left;
            ow->dragWindowOrigin.y = wr.top;
        }
        return 0;
    }
    case WM_LBUTTONUP:
    {
        if (!ow)
            break;
        POINT pt = {GET_X_LPARAM(lParam), GET_Y_LPARAM(lParam)};
        BOOL wasClosePressed = ow->closePressed;
        BOOL wasDragging = ow->dragging;
        ow->closePressed = FALSE;
        ow->dragging = FALSE;
        if (GetCapture() == hwnd)
            ReleaseCapture();
        InvalidateRect(hwnd, NULL, FALSE);

        if (wasClosePressed && ow->closable && PtInRect(&ow->closeRect, pt))
        {
            DestroyWindow(hwnd);
            return 0;
        }

        if (!wasDragging)
        {
            HandleOverlayClick(ow);
        }
        return 0;
    }
    case WM_KEYDOWN:
    {
        if (ow && ow->closeOnEscape && wParam == VK_ESCAPE)
        {
            // Escape is intentionally scoped to the overlay window that currently has focus instead
            // of being handled by a global keyboard hook that would close every pinned screenshot.
            DestroyWindow(hwnd);
            return 0;
        }
        break;
    }
    case WM_TIMER:
    {
        if (!ow)
            break;
        if (wParam == TIMER_AUTOCLOSE)
        {
            if (ow->mouseInside || ow->dragging)
            {
                ow->autoClosePending = TRUE;
            }
            else
            {
                DestroyWindow(hwnd);
            }
            return 0;
        }
        if (wParam == TIMER_TRACK)
        {
            if (ow->dragging)
                return 0;
            if (ow->stickyWindowPid > 0)
            {
                HWND target = NULL;
                if (FindWindowByPid(ow->stickyWindowPid, &target))
                {
                    if (ow->targetHwnd != target || !ow->locationHook)
                    {
                        if (ow->locationHook) UnhookWinEvent(ow->locationHook);
                        ow->targetHwnd = target;
                        DWORD pid = 0;
                        DWORD tid = GetWindowThreadProcessId(target, &pid);
                        ow->locationHook = SetWinEventHook(EVENT_OBJECT_LOCATIONCHANGE, EVENT_OBJECT_LOCATIONCHANGE, 
                                                           NULL, OverlayLocationChangeHook, pid, tid, WINEVENT_OUTOFCONTEXT);
                    }
                    StartLiveFollowTimerIfNeeded(ow);
                    SendMessage(hwnd, WM_WOX_OVERLAY_REPOSITION, 0, 0);
                }
                else
                {
                   if (ow->locationHook) {
                        UnhookWinEvent(ow->locationHook);
                        ow->locationHook = NULL;
                   }
                   ow->targetHwnd = NULL;
                   DestroyWindow(hwnd);
                }
            }
            return 0;
        }
        if (wParam == TIMER_LIVE_FOLLOW)
        {
            if (!IsLeftButtonDown() || ow->dragging)
            {
                StopLiveFollowTimer(ow);
                SendMessage(hwnd, WM_WOX_OVERLAY_REPOSITION, 0, 0);
                return 0;
            }

            RECT targetRect;
            BOOL targetFound = GetPredictiveTargetRect(ow, &targetRect);
            if (!targetFound && ow->targetHwnd && IsWindow(ow->targetHwnd))
                targetFound = GetTargetContentRect(ow->targetHwnd, &targetRect);
            if (!targetFound)
                return 0;

            // Optimization: live follow uses mouse-predicted geometry for smooth
            // movement between lower-frequency location events. Real samples keep
            // updating the anchor through WM_WOX_OVERLAY_REPOSITION.
            RepositionOverlayToTargetRect(ow, &targetRect, FALSE);
            SetOverlayZOrder(hwnd, ow->targetHwnd);
            if (!IsWindowVisible(hwnd))
                ShowOverlayWindowWithFocusPolicy(ow);
            return 0;
        }
        break;
    }
    case WM_WOX_OVERLAY_REPOSITION:
    {
        if (!ow || !ow->targetHwnd || !IsWindow(ow->targetHwnd))
            return 0;

        HWND target = ow->targetHwnd;
        RECT targetRect;
        if (!GetTargetContentRect(target, &targetRect))
        {
            return 0;
        }

        if (IsLeftButtonDown())
        {
            RefreshPredictiveAnchor(ow, &targetRect);
            StartLiveFollowTimerIfNeeded(ow);
        }

        SetOverlayZOrder(hwnd, target);
        RepositionOverlayToTargetRect(ow, &targetRect, TRUE);
        
        if (!IsWindowVisible(hwnd))
             ShowOverlayWindowWithFocusPolicy(ow);

        return 0;
    }
    case WM_DESTROY:
    {
        if (ow)
        {
            if (ow->locationHook)
            {
                UnhookWinEvent(ow->locationHook);
                ow->locationHook = NULL;
            }
            KillTimer(hwnd, TIMER_AUTOCLOSE);
            KillTimer(hwnd, TIMER_TRACK);
            KillTimer(hwnd, TIMER_LIVE_FOLLOW);
            DestroyTransparentImageShadow(ow);
            RemoveOverlay(ow);
            if (ow->messageFont)
                DeleteObject(ow->messageFont);
            if (ow->iconBitmap)
                DeleteObject(ow->iconBitmap);
            if (ow->tooltipIconBitmap)
                DeleteObject(ow->tooltipIconBitmap);
            if (ow->name)
                free(ow->name);
            if (ow->title)
                free(ow->title);
            if (ow->message)
                free(ow->message);
            if (ow->tooltip)
                free(ow->tooltip);
            free(ow);
        }
        return 0;
    }
    }

    return DefWindowProc(hwnd, uMsg, wParam, lParam);
}

// -----------------------------------------------------------------------------
// Controller Proc / Thread
// -----------------------------------------------------------------------------

static void HandleShowCommand(OverlayPayload *payload)
{
    if (!payload || !payload->name)
    {
        if (payload)
        {
            if (payload->title)
                free(payload->title);
            if (payload->message)
                free(payload->message);
            if (payload->tooltip)
                free(payload->tooltip);
            if (payload->iconData)
                free(payload->iconData);
            if (payload->iconFilePath)
                free(payload->iconFilePath);
            if (payload->tooltipIconData)
                free(payload->tooltipIconData);
            free(payload);
        }
        return;
    }

    OverlayWindow *ow = FindOverlayByName(payload->name);
    if (ow && ow->hwnd && IsWindow(ow->hwnd))
    {
        ApplyPayloadToOverlay(ow, payload, FALSE);
        ApplyOverlayLayout(ow);
        ShowOverlayWindowWithFocusPolicy(ow);
        InvalidateRect(ow->hwnd, NULL, TRUE);
        return;
    }

    ow = (OverlayWindow *)calloc(1, sizeof(OverlayWindow));
    if (!ow)
    {
        if (payload->name)
            free(payload->name);
        if (payload->title)
            free(payload->title);
        if (payload->message)
            free(payload->message);
        if (payload->tooltip)
            free(payload->tooltip);
        if (payload->iconData)
            free(payload->iconData);
        if (payload->iconFilePath)
            free(payload->iconFilePath);
        if (payload->tooltipIconData)
            free(payload->tooltipIconData);
        free(payload);
        return;
    }

    ApplyPayloadToOverlay(ow, payload, TRUE);

    DWORD exStyle = WS_EX_TOOLWINDOW;
    if (!ow->closeOnEscape)
        exStyle |= WS_EX_NOACTIVATE;
    if (ow->transparent)
        exStyle |= WS_EX_LAYERED;
    if (ow->topmost || ow->stickyWindowPid <= 0)
        exStyle |= WS_EX_TOPMOST;

    HWND owner = NULL;
    if (ow->stickyWindowPid > 0)
    {
        FindWindowByPid(ow->stickyWindowPid, &owner);
        if (!owner)
        {
            exStyle |= WS_EX_TOPMOST;
        }
    }

    DWORD style = WS_POPUP;
    if (ow->resizable && !ow->transparent)
    {
        // Bug fix: transparent pinned screenshots must be exactly the image surface. Applying the
        // system thick frame to that path created a visible non-client border and shrank the client
        // bitmap, so only non-transparent overlays may ask Windows to draw a resize frame.
        style |= WS_THICKFRAME;
    }

    ow->hwnd = CreateWindowExW(exStyle, g_overlayClassName, ow->title ? ow->title : L"",
                               style, 0, 0, 0, 0, owner, NULL, GetModuleHandleW(NULL), ow);
    if (!ow->hwnd)
    {
        DWORD err = GetLastError();
        if (owner && (err == 5 || err == ERROR_ACCESS_DENIED))
        {
            owner = NULL;
            exStyle |= WS_EX_TOPMOST;
            ow->hwnd = CreateWindowExW(exStyle, g_overlayClassName, ow->title ? ow->title : L"",
                                       style, 0, 0, 0, 0, owner, NULL, GetModuleHandleW(NULL), ow);
        }
    }

    if (!ow->hwnd)
    {
        if (ow->name)
            free(ow->name);
        if (ow->title)
            free(ow->title);
        if (ow->message)
            free(ow->message);
        if (ow->tooltip)
            free(ow->tooltip);
        if (ow->iconBitmap)
            DeleteObject(ow->iconBitmap);
        if (ow->tooltipIconBitmap)
            DeleteObject(ow->tooltipIconBitmap);
        free(ow);
        return;
    }
    AddOverlay(ow);
    ApplyOverlayLayout(ow);
    if (ow->stickyWindowPid > 0 && !ow->targetReady)
    {
        ow->hiddenForMove = TRUE;
        ShowWindow(ow->hwnd, SW_HIDE);
    }
    else
    {
        ShowOverlayWindowWithFocusPolicy(ow);
        UpdateWindow(ow->hwnd);
    }

}

static void HandleCloseCommand(const WCHAR *name)
{
    if (!name)
        return;
    OverlayWindow *ow = FindOverlayByName(name);
    if (ow && ow->hwnd && IsWindow(ow->hwnd))
    {
        DestroyWindow(ow->hwnd);
    }
}

static LRESULT CALLBACK OverlayControllerProc(HWND hwnd, UINT uMsg, WPARAM wParam, LPARAM lParam)
{
    if (uMsg == WM_WOX_OVERLAY_COMMAND)
    {
        OverlayCommand *cmd = (OverlayCommand *)lParam;
        if (!cmd)
            return 0;
        if (cmd->type == 1)
        {
            HandleShowCommand(cmd->payload);
        }
        else if (cmd->type == 2)
        {
            HandleCloseCommand(cmd->name);
            if (cmd->name)
                free(cmd->name);
        }
        free(cmd);
        return 0;
    }
    return DefWindowProc(hwnd, uMsg, wParam, lParam);
}

static LRESULT CALLBACK ShadowWindowProc(HWND hwnd, UINT uMsg, WPARAM wParam, LPARAM lParam)
{
    if (uMsg == WM_NCHITTEST)
    {
        // Feature change: the shadow is only a visual depth cue for pinned screenshots. It must
        // never steal mouse input from the image overlay or the app behind it.
        return HTTRANSPARENT;
    }
    return DefWindowProc(hwnd, uMsg, wParam, lParam);
}

static DWORD WINAPI OverlayThreadProc(LPVOID param)
{
    (void)param;
    CoInitializeEx(NULL, COINIT_MULTITHREADED);
    
    INITCOMMONCONTROLSEX iccex;
    iccex.dwSize = sizeof(INITCOMMONCONTROLSEX);
    iccex.dwICC = ICC_WIN95_CLASSES | ICC_STANDARD_CLASSES;
    InitCommonControlsEx(&iccex);

    BufferedPaintInit();
    TryEnablePerMonitorDpiAwareness();

    WNDCLASSEXW wc;
    ZeroMemory(&wc, sizeof(wc));
    wc.cbSize = sizeof(wc);
    wc.lpfnWndProc = OverlayWindowProc;
    wc.hInstance = GetModuleHandleW(NULL);
    wc.lpszClassName = g_overlayClassName;
    wc.hCursor = LoadCursor(NULL, IDC_ARROW);
    RegisterClassExW(&wc);

    WNDCLASSEXW wcShadow;
    ZeroMemory(&wcShadow, sizeof(wcShadow));
    wcShadow.cbSize = sizeof(wcShadow);
    wcShadow.lpfnWndProc = ShadowWindowProc;
    wcShadow.hInstance = GetModuleHandleW(NULL);
    wcShadow.lpszClassName = g_shadowClassName;
    wcShadow.hCursor = LoadCursor(NULL, IDC_ARROW);
    RegisterClassExW(&wcShadow);

    WNDCLASSEXW wc2;
    ZeroMemory(&wc2, sizeof(wc2));
    wc2.cbSize = sizeof(wc2);
    wc2.lpfnWndProc = OverlayControllerProc;
    wc2.hInstance = GetModuleHandleW(NULL);
    wc2.lpszClassName = g_controllerClassName;
    RegisterClassExW(&wc2);

    WNDCLASSEXW wc3;
    ZeroMemory(&wc3, sizeof(wc3));
    wc3.cbSize = sizeof(wc3);
    wc3.lpfnWndProc = TooltipWindowProc;
    wc3.hInstance = GetModuleHandleW(NULL);
    wc3.lpszClassName = g_tooltipClassName;
    wc3.hCursor = LoadCursor(NULL, IDC_ARROW);
    RegisterClassExW(&wc3);

    HWND controller = CreateWindowExW(0, g_controllerClassName, L"", 0, 0, 0, 0, 0,
                                      HWND_MESSAGE, NULL, GetModuleHandleW(NULL), NULL);
    g_controllerHwnd = controller;
    g_overlayThreadId = GetCurrentThreadId();

    if (g_threadReadyEvent)
        SetEvent(g_threadReadyEvent);

    MSG msg;
    while (GetMessage(&msg, NULL, 0, 0) > 0)
    {
        TranslateMessage(&msg);
        DispatchMessage(&msg);
    }

    CoUninitialize();
    return 0;
}

static BOOL CALLBACK InitOverlayThread(PINIT_ONCE InitOnce, PVOID Parameter, PVOID *Context)
{
    (void)InitOnce;
    (void)Parameter;
    (void)Context;

    g_threadReadyEvent = CreateEventW(NULL, TRUE, FALSE, NULL);
    if (!g_threadReadyEvent)
        return FALSE;

    g_overlayThread = CreateThread(NULL, 0, OverlayThreadProc, NULL, 0, &g_overlayThreadId);
    if (!g_overlayThread)
        return FALSE;

    WaitForSingleObject(g_threadReadyEvent, INFINITE);
    return TRUE;
}

static void EnsureOverlayThread(void)
{
    InitOnceExecuteOnce(&g_initOnce, InitOverlayThread, NULL, NULL);
}

// -----------------------------------------------------------------------------
// C Exported Functions
// -----------------------------------------------------------------------------

void ShowOverlay(OverlayOptions opts)
{
    EnsureOverlayThread();
    if (!g_controllerHwnd)
        return;

    OverlayPayload *payload = (OverlayPayload *)calloc(1, sizeof(OverlayPayload));
    if (!payload)
        return;

    payload->name = DupUtf8ToWide(opts.name);
    payload->title = DupUtf8ToWide(opts.title);
    payload->message = DupUtf8ToWide(opts.message);
    payload->tooltip = DupUtf8ToWide(opts.tooltip);
    payload->iconFilePath = DupUtf8ToWide(opts.iconFilePath);
    payload->transparent = opts.transparent ? TRUE : FALSE;
    payload->hitTestIconOnly = opts.hitTestIconOnly ? TRUE : FALSE;
    payload->iconX = opts.iconX;
    payload->iconY = opts.iconY;
    payload->iconWidth = opts.iconWidth;
    payload->iconHeight = opts.iconHeight;
    payload->closable = opts.closable ? TRUE : FALSE;
    payload->closeOnEscape = opts.closeOnEscape ? TRUE : FALSE;
    payload->loading = opts.loading ? TRUE : FALSE;
    payload->topmost = opts.topmost ? TRUE : FALSE;
    payload->absolutePosition = opts.absolutePosition ? TRUE : FALSE;
    payload->stickyWindowPid = opts.stickyWindowPid;
    payload->anchor = opts.anchor;
    payload->autoCloseSeconds = opts.autoCloseSeconds;
    payload->movable = opts.movable ? TRUE : FALSE;
    payload->resizable = opts.resizable ? TRUE : FALSE;
    payload->cornerRadius = opts.cornerRadius;
    payload->aspectRatio = opts.aspectRatio;
    payload->offsetX = opts.offsetX;
    payload->offsetY = opts.offsetY;
    payload->width = opts.width;
    payload->height = opts.height;
    payload->fontSize = opts.fontSize;
    payload->iconSize = opts.iconSize;
    payload->tooltipIconSize = opts.tooltipIconSize;

    if (opts.iconData && opts.iconLen > 0)
    {
        payload->iconData = (unsigned char *)malloc((size_t)opts.iconLen);
        if (payload->iconData)
        {
            memcpy(payload->iconData, opts.iconData, (size_t)opts.iconLen);
            payload->iconLen = opts.iconLen;
        }
    }

    if (opts.tooltipIconData && opts.tooltipIconLen > 0)
    {
        payload->tooltipIconData = (unsigned char *)malloc((size_t)opts.tooltipIconLen);
        if (payload->tooltipIconData)
        {
            memcpy(payload->tooltipIconData, opts.tooltipIconData, (size_t)opts.tooltipIconLen);
            payload->tooltipIconLen = opts.tooltipIconLen;
        }
    }

    OverlayCommand *cmd = (OverlayCommand *)calloc(1, sizeof(OverlayCommand));
    if (!cmd)
    {
        if (payload->name)
            free(payload->name);
        if (payload->title)
            free(payload->title);
        if (payload->message)
            free(payload->message);
        if (payload->tooltip)
            free(payload->tooltip);
        if (payload->iconData)
            free(payload->iconData);
        if (payload->iconFilePath)
            free(payload->iconFilePath);
        if (payload->tooltipIconData)
            free(payload->tooltipIconData);
        free(payload);
        return;
    }
    cmd->type = 1;
    cmd->payload = payload;

    if (!PostMessageW(g_controllerHwnd, WM_WOX_OVERLAY_COMMAND, 0, (LPARAM)cmd))
    {
        if (payload->name)
            free(payload->name);
        if (payload->title)
            free(payload->title);
        if (payload->message)
            free(payload->message);
        if (payload->tooltip)
            free(payload->tooltip);
        if (payload->iconData)
            free(payload->iconData);
        if (payload->iconFilePath)
            free(payload->iconFilePath);
        if (payload->tooltipIconData)
            free(payload->tooltipIconData);
        free(payload);
        free(cmd);
    }
}

void CloseOverlay(char *name)
{
    if (!name)
        return;
    EnsureOverlayThread();
    if (!g_controllerHwnd)
        return;

    OverlayCommand *cmd = (OverlayCommand *)calloc(1, sizeof(OverlayCommand));
    if (!cmd)
        return;

    cmd->type = 2;
    cmd->name = DupUtf8ToWide(name);

    if (!PostMessageW(g_controllerHwnd, WM_WOX_OVERLAY_COMMAND, 0, (LPARAM)cmd))
    {
        if (cmd->name)
            free(cmd->name);
        free(cmd);
    }
}
