#define WIN32_LEAN_AND_MEAN
#define COBJMACROS
#include <windows.h>
#include <stdint.h>
#include <windowsx.h>
#include <dwmapi.h>
#include <uxtheme.h>
#include <commctrl.h>
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>
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
    bool transparent;
    bool hitTestIconOnly;
    bool closeOnEscape;
    bool takeFocus;
    bool nativeAttachment;
    int nativeAttachmentKind;
    void* nativeAttachmentHandle;
    float nativeAttachmentWidth;
    float nativeAttachmentHeight;
    bool topmost;
    bool absolutePosition;
    bool preservePosition;
    int stickyWindowPid; // 0 = Screen, >0 = Window
    uintptr_t stickyWindowHandle;
    int anchor;          // 0-8
    bool movable;
    bool resizable;
    float cornerRadius;
    float aspectRatio;
    float offsetX;
    float offsetY;
    float width;         // 0 = auto
    float minWidth;      // 0 = platform default minimum width
    float maxWidth;      // 0 = no cap for auto width
    float height;        // 0 = auto
    float maxHeight;     // 0 = no cap for auto height
} OverlayOptions;

extern bool overlayClickCallbackCGO(char* name);
extern void overlayCloseCallbackCGO(char* name);

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
#define DEFAULT_WINDOW_HEIGHT_DIP 24
#define CORNER_RADIUS_DIP 10
#define RESIZE_GRIP_DIP 10
#define MIN_RESIZE_SIZE_DIP 64
#define NATIVE_ATTACHMENT_KIND_WINDOW 2

#define TIMER_TRACK 2
#define TIMER_LIVE_FOLLOW 3
#define TIMER_REPAINT 5
#define PREDICTIVE_CORRECTION_THRESHOLD_PX 48

#define WM_WOX_OVERLAY_COMMAND (WM_APP + 0x610)
#define WM_WOX_OVERLAY_REPOSITION (WM_APP + 0x611)

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------
typedef UINT(WINAPI *pfnGetDpiForSystem)(void);
typedef UINT(WINAPI *pfnGetDpiForWindow)(HWND);
typedef BOOL(WINAPI *pfnSetProcessDpiAwarenessContext)(HANDLE);
typedef void *(WINAPI *pfnAttachStickyHook)(HWND, DWORD, HWND);
typedef BOOL(WINAPI *pfnDetachStickyHook)(void *);

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
    WCHAR *name;
    BOOL transparent;
    BOOL hitTestIconOnly;
    BOOL closeOnEscape;
    BOOL takeFocus;
    BOOL nativeAttachment;
    int nativeAttachmentKind;
    HWND nativeAttachmentHwnd;
    float nativeAttachmentWidth;
    float nativeAttachmentHeight;
    BOOL topmost;
    BOOL absolutePosition;
    BOOL preservePosition;
    BOOL movable;
    BOOL resizable;
    float cornerRadius;
    float aspectRatio;
    int stickyWindowPid;
    HWND stickyWindowHwnd;
    int anchor;
    float offsetX;
    float offsetY;
    float width;
    float minWidth;
    float maxWidth;
    float height;
    float maxHeight;

    UINT dpi;

    RECT nativeAttachmentRect;
    BOOL repaintPending;
    BOOL layoutSizeChanged;
    BOOL mouseInside;
    BOOL dragging;
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
    HMODULE windowHookModule;
    void *injectedStickyHook;
    HWND injectedStickyTarget;

    struct OverlayWindow *next;
} OverlayWindow;

typedef struct OverlayPayload
{
    WCHAR *name;
    BOOL transparent;
    BOOL hitTestIconOnly;
    BOOL closeOnEscape;
    BOOL takeFocus;
    BOOL nativeAttachment;
    int nativeAttachmentKind;
    void *nativeAttachmentHandle;
    float nativeAttachmentWidth;
    float nativeAttachmentHeight;
    BOOL topmost;
    BOOL absolutePosition;
    BOOL preservePosition;
    int stickyWindowPid;
    HWND stickyWindowHwnd;
    int anchor;
    BOOL movable;
    BOOL resizable;
    float cornerRadius;
    float aspectRatio;
    float offsetX;
    float offsetY;
    float width;
    float minWidth;
    float maxWidth;
    float height;
    float maxHeight;
} OverlayPayload;

typedef struct OverlayCommand
{
    int type; // 1 = show, 2 = close
    OverlayPayload *payload;
    WCHAR *name;
} OverlayCommand;

static OverlayWindow *g_overlays = NULL;
static const WCHAR *g_overlayClassName = L"WoxOverlayWindow";
static const WCHAR *g_controllerClassName = L"WoxOverlayController";
static HANDLE g_threadReadyEvent = NULL;
static HANDLE g_overlayThread = NULL;
static DWORD g_overlayThreadId = 0;
static HWND g_controllerHwnd = NULL;
static INIT_ONCE g_initOnce = INIT_ONCE_STATIC_INIT;
static WCHAR *g_windowHookDllPath = NULL;
static UINT g_stickyChangedMessage = 0;


// -----------------------------------------------------------------------------
// Forward Decls
// -----------------------------------------------------------------------------
static LRESULT CALLBACK OverlayWindowProc(HWND hwnd, UINT uMsg, WPARAM wParam, LPARAM lParam);
static LRESULT CALLBACK OverlayControllerProc(HWND hwnd, UINT uMsg, WPARAM wParam, LPARAM lParam);
static DWORD WINAPI OverlayThreadProc(LPVOID param);
static BOOL GetTargetWindowRect(HWND target, RECT *outRect);
static void StartLiveFollowTimerIfNeeded(OverlayWindow *ow);
static void StopLiveFollowTimer(OverlayWindow *ow);
static void RepositionOverlayToTargetRect(OverlayWindow *ow, const RECT *targetRect, BOOL preserveSmallPredictiveCorrection);
static void ShowOverlayWindowWithFocusPolicy(OverlayWindow *ow);
static void NotifyOverlayClose(OverlayWindow *ow);
static void AttachInjectedStickyHook(OverlayWindow *ow);
static void DetachInjectedStickyHook(OverlayWindow *ow);

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

// IsWindowFromPid validates an exact sticky HWND before it is reused across tracking ticks.
static BOOL IsWindowFromPid(HWND hwnd, int pid)
{
    if (!hwnd || !IsWindow(hwnd) || pid <= 0)
        return FALSE;
    DWORD windowPid = 0;
    GetWindowThreadProcessId(hwnd, &windowPid);
    return (int)windowPid == pid;
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

    // Feature change: transparent resizable overlays are borderless, so Windows needs explicit
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

// GetStickyChangedMessage shares the registered notification name with the injected DLL.
static UINT GetStickyChangedMessage(void)
{
    if (g_stickyChangedMessage == 0)
        g_stickyChangedMessage = RegisterWindowMessageW(L"Wox.WindowHook.StickyChanged.v1");
    return g_stickyChangedMessage;
}

// DetachInjectedStickyHook removes the target subclass before unloading the DLL from Wox.
static void DetachInjectedStickyHook(OverlayWindow *ow)
{
    if (!ow)
        return;

    if (ow->windowHookModule && ow->injectedStickyHook)
    {
        pfnDetachStickyHook detach = (pfnDetachStickyHook)GetProcAddress(ow->windowHookModule, "WoxWindowHookDetachSticky");
        if (detach)
            detach(ow->injectedStickyHook);
    }
    ow->injectedStickyHook = NULL;
    ow->injectedStickyTarget = NULL;
    if (ow->windowHookModule)
    {
        FreeLibrary(ow->windowHookModule);
        ow->windowHookModule = NULL;
    }
}

// AttachInjectedStickyHook adds the target-thread signal used instead of the fallback position sources.
static void AttachInjectedStickyHook(OverlayWindow *ow)
{
    if (!ow || !ow->hwnd || !ow->targetHwnd || ow->stickyWindowPid <= 0 || !g_windowHookDllPath)
        return;
    if (ow->injectedStickyHook && ow->injectedStickyTarget == ow->targetHwnd)
        return;

    DetachInjectedStickyHook(ow);
    HMODULE module = LoadLibraryW(g_windowHookDllPath);
    if (!module)
        return;
    pfnAttachStickyHook attach = (pfnAttachStickyHook)GetProcAddress(module, "WoxWindowHookAttachSticky");
    if (!attach)
    {
        FreeLibrary(module);
        return;
    }

    void *hook = attach(ow->targetHwnd, (DWORD)ow->stickyWindowPid, ow->hwnd);
    if (!hook)
    {
        FreeLibrary(module);
        return;
    }
    ow->windowHookModule = module;
    ow->injectedStickyHook = hook;
    ow->injectedStickyTarget = ow->targetHwnd;
    StopLiveFollowTimer(ow);
}

static BOOL IsLeftButtonDown(void)
{
    return (GetAsyncKeyState(VK_LBUTTON) & 0x8000) != 0;
}

static BOOL GetTargetWindowRect(HWND target, RECT *outRect)
{
    if (!target || !IsWindow(target) || !outRect)
        return FALSE;

    RECT targetRect;
    // DWM exposes the committed visual frame used by the compositor. Falling back
    // to GetWindowRect keeps sticky overlays working when DWM bounds are unavailable.
    if (FAILED(DwmGetWindowAttribute(target, DWMWA_EXTENDED_FRAME_BOUNDS, &targetRect, sizeof(targetRect))) &&
        !GetWindowRect(target, &targetRect))
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
    if (!ow || !ow->hwnd || ow->injectedStickyHook || ow->liveFollowActive || ow->stickyWindowPid <= 0 || !IsLeftButtonDown())
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

    // Most overlays must not steal focus from the user's active app, so they
    // show without activating. Overlays that opt into TakeFocus (e.g. the
    // dictation recording overlay, which needs Esc to cancel without an extra
    // click) receive keyboard focus immediately after showing.
    ShowWindow(ow->hwnd, SW_SHOWNOACTIVATE);
    if (ow->takeFocus)
        SetFocus(ow->hwnd);
}

static void ScheduleOverlayRepaint(OverlayWindow *ow)
{
    if (!ow || !ow->hwnd || ow->repaintPending)
        return;

    ow->repaintPending = TRUE;
    SetTimer(ow->hwnd, TIMER_REPAINT, 16, NULL);
}

static void DetachNativeAttachment(OverlayWindow *ow)
{
    if (!ow || !ow->nativeAttachmentHwnd)
        return;

    HWND child = ow->nativeAttachmentHwnd;
    if (IsWindow(child))
    {
        ShowWindow(child, SW_HIDE);
        SetParent(child, NULL);
        LONG_PTR style = GetWindowLongPtrW(child, GWL_STYLE);
        style &= ~WS_CHILD;
        style |= WS_POPUP;
        SetWindowLongPtrW(child, GWL_STYLE, style);
    }

    ow->nativeAttachmentHwnd = NULL;
    RECT empty = {0, 0, 0, 0};
    ow->nativeAttachmentRect = empty;
}

static void LayoutNativeAttachment(OverlayWindow *ow)
{
    if (!ow || !ow->hwnd)
        return;

    if (!ow->nativeAttachment || ow->nativeAttachmentKind != NATIVE_ATTACHMENT_KIND_WINDOW || !ow->nativeAttachmentHwnd || !IsWindow(ow->nativeAttachmentHwnd))
    {
        DetachNativeAttachment(ow);
        return;
    }

    HWND child = ow->nativeAttachmentHwnd;
    LONG_PTR style = GetWindowLongPtrW(child, GWL_STYLE);
    style &= ~WS_POPUP;
    style |= WS_CHILD | WS_VISIBLE;
    SetWindowLongPtrW(child, GWL_STYLE, style);
    SetParent(child, ow->hwnd);

    int x = ow->nativeAttachmentRect.left;
    int y = ow->nativeAttachmentRect.top;
    int width = ow->nativeAttachmentRect.right - ow->nativeAttachmentRect.left;
    int height = ow->nativeAttachmentRect.bottom - ow->nativeAttachmentRect.top;
    SetWindowPos(child, HWND_TOP, x, y, width, height, SWP_NOACTIVATE | SWP_SHOWWINDOW);
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
            if (!SetWindowRgn(hwnd, rgn, TRUE))
                DeleteObject(rgn);
        }
    }
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

    // Feature change: the native sizing rectangle is corrected before WM_SIZE so transparent
    // overlays scale uniformly while WM_SIZE refreshes child bounds from one consistent final size.
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

static void ApplyOverlayLayout(OverlayWindow *ow)
{
    if (!ow || !ow->hwnd)
        return;

    ow->dpi = GetWindowDpiSafe(ow->hwnd, ow->dpi ? ow->dpi : GetSystemDpiSafe());

    int minWidth = MulDiv(MIN_WINDOW_WIDTH_DIP, (int)ow->dpi, 96);
    if (ow->minWidth > 0.0f)
        minWidth = (int)roundf(ow->minWidth * (float)ow->dpi / 96.0f);
    int width = (ow->width > 0.0f) ? (int)roundf(ow->width * (float)ow->dpi / 96.0f) : MulDiv(DEFAULT_WINDOW_WIDTH_DIP, (int)ow->dpi, 96);
    int height = (ow->height > 0.0f) ? (int)roundf(ow->height * (float)ow->dpi / 96.0f) : MulDiv(DEFAULT_WINDOW_HEIGHT_DIP, (int)ow->dpi, 96);

    if (ow->nativeAttachment && ow->nativeAttachmentKind == NATIVE_ATTACHMENT_KIND_WINDOW && ow->nativeAttachmentHwnd)
    {
        BOOL transparentAttachment = ow->transparent;
        int attachmentWidth = (ow->nativeAttachmentWidth > 0.0f) ? (int)roundf(ow->nativeAttachmentWidth * (float)ow->dpi / 96.0f) : MulDiv(132, (int)ow->dpi, 96);
        int attachmentHeight = (ow->nativeAttachmentHeight > 0.0f) ? (int)roundf(ow->nativeAttachmentHeight * (float)ow->dpi / 96.0f) : MulDiv(24, (int)ow->dpi, 96);
        // Native attachments report their content size; base window chrome must be added around it.
        width = (ow->width > 0.0f) ? (int)roundf(ow->width * (float)ow->dpi / 96.0f) : attachmentWidth + (transparentAttachment ? 0 : MulDiv(36, (int)ow->dpi, 96));
        height = (ow->height > 0.0f) ? (int)roundf(ow->height * (float)ow->dpi / 96.0f) : attachmentHeight + (transparentAttachment ? 0 : MulDiv(24, (int)ow->dpi, 96));
        if (!transparentAttachment && width < minWidth)
            width = minWidth;
        if (width < 1)
            width = 1;
        if (height < 1)
            height = 1;

        if (transparentAttachment)
        {
            SetRect(&ow->nativeAttachmentRect, 0, 0, width, height);
        }
        else
        {
            int attachmentPad = MulDiv(18, (int)ow->dpi, 96);
            int attachmentLeft = attachmentPad;
            int attachmentRight = width - attachmentPad;
            if (attachmentRight - attachmentLeft < MulDiv(48, (int)ow->dpi, 96))
                attachmentRight = attachmentLeft + MulDiv(48, (int)ow->dpi, 96);
            int attachmentTop = (height - attachmentHeight) / 2;
            SetRect(&ow->nativeAttachmentRect, attachmentLeft, attachmentTop, attachmentRight, attachmentTop + attachmentHeight);
        }
    }
    else
    {
        RECT empty = {0, 0, 0, 0};
        ow->nativeAttachmentRect = empty;
    }
    if (!ow->transparent && width < minWidth)
        width = minWidth;
    if (width < 1)
        width = 1;
    if (height < 1)
        height = 1;

    RECT targetRect;
    BOOL targetFound = FALSE;
    BOOL preserveLiveFollowFrame = ow->stickyWindowPid > 0 && ow->liveFollowActive;
    if (ow->stickyWindowPid > 0)
    {
        HWND target = ow->stickyWindowHwnd;
        if (!IsWindowFromPid(target, ow->stickyWindowPid))
        {
            target = NULL;
            FindWindowByPid(ow->stickyWindowPid, &target);
        }
        if (target)
        {
            ow->targetHwnd = target;
            if (GetTargetWindowRect(target, &targetRect))
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
            // Bug fix: absolute overlays pass desktop coordinates directly. The old
            // screen branch anchored those offsets to the primary work area and then clamped them,
            // which moved windows from secondary/negative-coordinate monitors back onto the main
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
    else if (ow->preservePosition)
    {
        RECT current;
        if (GetWindowRect(ow->hwnd, &current))
        {
            // Some callers refresh only the content of an existing overlay. Keep the
            // current user-visible position instead of reapplying the original anchor.
            x = current.left;
            y = current.top;
        }
    }

    RECT currentWindowRect;
    ow->layoutSizeChanged = TRUE;
    if (GetWindowRect(ow->hwnd, &currentWindowRect))
    {
        ow->layoutSizeChanged = (currentWindowRect.right - currentWindowRect.left) != width || (currentWindowRect.bottom - currentWindowRect.top) != height;
    }

    SetWindowPos(ow->hwnd, NULL, x, y, width, height, SWP_NOACTIVATE | SWP_NOZORDER);
    LayoutNativeAttachment(ow);
    if (ow->transparent)
    {
        SetWindowRgn(ow->hwnd, NULL, TRUE);
        UINT pref = DWMWCP_DONOTROUND;
        DwmSetWindowAttribute(ow->hwnd, DWMWA_WINDOW_CORNER_PREFERENCE, &pref, sizeof(pref));
    }
    else
    {
        ApplyCornerRadius(ow->hwnd, ow->dpi, width, height);
    }

    StartTrackTimer(ow);
    AttachInjectedStickyHook(ow);
}

static void ApplyPayloadToOverlay(OverlayWindow *ow, OverlayPayload *payload, BOOL isNew)
{
    if (!ow || !payload)
        return;

    int previousStickyWindowPid = ow->stickyWindowPid;
    HWND previousStickyWindowHwnd = ow->stickyWindowHwnd;

    if (isNew)
        ow->name = payload->name;
    else if (payload->name)
        free(payload->name);

    ow->closeOnEscape = payload->closeOnEscape;
    ow->takeFocus = payload->takeFocus;
    ow->nativeAttachment = payload->nativeAttachment;
    ow->nativeAttachmentKind = payload->nativeAttachmentKind;
    if (payload->nativeAttachment)
    {
        HWND nextAttachment = (HWND)payload->nativeAttachmentHandle;
        if (ow->nativeAttachmentHwnd && ow->nativeAttachmentHwnd != nextAttachment)
            DetachNativeAttachment(ow);
        ow->nativeAttachmentHwnd = nextAttachment;
    }
    ow->nativeAttachmentWidth = payload->nativeAttachmentWidth;
    ow->nativeAttachmentHeight = payload->nativeAttachmentHeight;
    ow->topmost = payload->topmost;
    ow->absolutePosition = payload->absolutePosition;
    ow->preservePosition = payload->preservePosition;
    ow->transparent = payload->transparent;
    ow->hitTestIconOnly = payload->hitTestIconOnly;
    ow->stickyWindowPid = payload->stickyWindowPid;
    ow->stickyWindowHwnd = payload->stickyWindowHwnd;
    ow->anchor = payload->anchor;
    ow->movable = payload->movable;
    ow->resizable = payload->resizable;
    ow->cornerRadius = payload->cornerRadius;
    ow->aspectRatio = payload->aspectRatio > 0.0f ? payload->aspectRatio : 0.0f;
    ow->offsetX = payload->offsetX;
    ow->offsetY = payload->offsetY;
    ow->width = payload->width;
    ow->minWidth = payload->minWidth;
    ow->maxWidth = payload->maxWidth;
    ow->height = payload->height;
    ow->maxHeight = payload->maxHeight;
    ow->hasLastTargetRect = FALSE;
    ow->hiddenForMove = FALSE;
    if (ow->hwnd)
    {
        LONG_PTR style = GetWindowLongPtrW(ow->hwnd, GWL_STYLE);
        LONG_PTR updatedStyle = (ow->resizable && !ow->transparent) ? (style | WS_THICKFRAME) : (style & ~WS_THICKFRAME);
        if (updatedStyle != style)
        {
            // Bug fix: reused overlay windows can switch between HUD and transparent native-child
            // modes. Keep the thick frame off transparent windows so child content fills the client
            // surface instead of being inset behind a system-drawn border.
            SetWindowLongPtrW(ow->hwnd, GWL_STYLE, updatedStyle);
            SetWindowPos(ow->hwnd, NULL, 0, 0, 0, 0,
                         SWP_NOMOVE | SWP_NOSIZE | SWP_NOZORDER | SWP_NOACTIVATE | SWP_FRAMECHANGED);
        }

        LONG_PTR exStyle = GetWindowLongPtrW(ow->hwnd, GWL_EXSTYLE);
        LONG_PTR updatedExStyle = (ow->transparent && !ow->nativeAttachment) ? (exStyle | WS_EX_LAYERED) : (exStyle & ~WS_EX_LAYERED);
        if (updatedExStyle != exStyle)
        {
            // Keep any legacy transparent non-attachment fallback aligned with the current payload;
            // native attachment overlays render transparency inside the child window instead.
            SetWindowLongPtrW(ow->hwnd, GWL_EXSTYLE, updatedExStyle);
        }
    }
    if (!isNew && (previousStickyWindowPid != ow->stickyWindowPid || previousStickyWindowHwnd != ow->stickyWindowHwnd))
    {
        // Predictive anchors are tied to one exact target. Clear the old live-follow
        // state when either the PID or the caller-provided HWND changes.
        StopLiveFollowTimer(ow);
        DetachInjectedStickyHook(ow);
        if (ow->locationHook)
        {
            UnhookWinEvent(ow->locationHook);
            ow->locationHook = NULL;
        }
        ow->hasPredictiveAnchor = FALSE;
        ow->targetHwnd = NULL;
    }

    free(payload);
}

// OverlayPayloadMatchesCurrent keeps repeated Show calls from relaying out and repainting an unchanged native attachment.
static BOOL OverlayPayloadMatchesCurrent(OverlayWindow *ow, OverlayPayload *payload)
{
    if (!ow || !payload)
        return FALSE;

    return ow->transparent == payload->transparent &&
           ow->hitTestIconOnly == payload->hitTestIconOnly &&
           ow->closeOnEscape == payload->closeOnEscape &&
           ow->takeFocus == payload->takeFocus &&
           ow->nativeAttachment == payload->nativeAttachment &&
           ow->nativeAttachmentKind == payload->nativeAttachmentKind &&
           ow->nativeAttachmentHwnd == (HWND)payload->nativeAttachmentHandle &&
           ow->nativeAttachmentWidth == payload->nativeAttachmentWidth &&
           ow->nativeAttachmentHeight == payload->nativeAttachmentHeight &&
           ow->topmost == payload->topmost &&
           ow->absolutePosition == payload->absolutePosition &&
           ow->preservePosition == payload->preservePosition &&
           ow->stickyWindowPid == payload->stickyWindowPid &&
           ow->stickyWindowHwnd == payload->stickyWindowHwnd &&
           ow->anchor == payload->anchor &&
           ow->movable == payload->movable &&
           ow->resizable == payload->resizable &&
           ow->cornerRadius == payload->cornerRadius &&
           ow->aspectRatio == payload->aspectRatio &&
           ow->offsetX == payload->offsetX &&
           ow->offsetY == payload->offsetY &&
           ow->width == payload->width &&
           ow->minWidth == payload->minWidth &&
           ow->maxWidth == payload->maxWidth &&
           ow->height == payload->height &&
           ow->maxHeight == payload->maxHeight;
}

static BOOL HandleOverlayClick(OverlayWindow *ow)
{
    if (!ow || !ow->name)
        return FALSE;
    char *nameUtf8 = DupWideToUtf8(ow->name);
    if (!nameUtf8)
        return FALSE;
    BOOL ok = overlayClickCallbackCGO(nameUtf8) ? TRUE : FALSE;
    free(nameUtf8);
    return ok;
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
            PostMessageW(ow->hwnd, WM_WOX_OVERLAY_REPOSITION, 0, 0);
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

    if (ow && uMsg == GetStickyChangedMessage() && (HWND)wParam == ow->targetHwnd)
        uMsg = WM_WOX_OVERLAY_REPOSITION;

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
            // normal anchor layout here can snap borderless overlays back to the primary work area.
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
            if (ow->nativeAttachment)
            {
                RECT client;
                GetClientRect(hwnd, &client);
                ow->nativeAttachmentRect = client;
                LayoutNativeAttachment(ow);
                return 0;
            }
            SetWindowRgn(hwnd, NULL, TRUE);
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

        HDC hdc = paintHdc;
        HPAINTBUFFER paintBuf = BeginBufferedPaint(paintHdc, &client, BPBF_TOPDOWNDIB, NULL, &hdc);
        if (paintBuf)
        {
            BufferedPaintClear(paintBuf, &client);
        }

        if (ow->transparent)
        {
            // Native attachment overlays paint transparent content in their child window.
            if (paintBuf)
                EndBufferedPaint(paintBuf, FALSE);
            EndPaint(hwnd, &ps);
            return 0;
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
            if (!PtInRect(&ow->nativeAttachmentRect, pt))
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
            // Feature change: resize hit testing is custom for borderless overlays, so set
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

        if (ow->dragging)
        {
            POINT screenPt;
            GetCursorPos(&screenPt);
            int dx = screenPt.x - ow->dragStart.x;
            int dy = screenPt.y - ow->dragStart.y;
            SetWindowPos(hwnd, NULL, ow->dragWindowOrigin.x + dx, ow->dragWindowOrigin.y + dy, 0, 0,
                         SWP_NOACTIVATE | SWP_NOZORDER | SWP_NOSIZE);
        }
        return 0;
    }
    case WM_MOUSELEAVE:
    {
        if (!ow)
            break;
        ow->mouseInside = FALSE;
        return 0;
    }
    case WM_LBUTTONDOWN:
    {
        if (!ow)
            break;
        if (ow->closeOnEscape)
        {
            // Focus-sensitive overlays must receive Escape themselves. Setting focus only when this
            // option is enabled keeps notification overlays non-activating while focused overlays
            // close one window at a time.
            SetFocus(hwnd);
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
        BOOL wasDragging = ow->dragging;
        ow->dragging = FALSE;
        if (GetCapture() == hwnd)
            ReleaseCapture();
        InvalidateRect(hwnd, NULL, FALSE);

        if (!wasDragging)
            HandleOverlayClick(ow);
        return 0;
    }
    case WM_KEYDOWN:
    {
        if (ow && ow->closeOnEscape && wParam == VK_ESCAPE)
        {
            // Escape is intentionally scoped to the overlay window that currently has focus instead
            // of being handled by a global keyboard hook that would close every overlay.
            NotifyOverlayClose(ow);
            DestroyWindow(hwnd);
            return 0;
        }
        break;
    }
    case WM_TIMER:
    {
        if (!ow)
            break;
        if (wParam == TIMER_TRACK)
        {
            if (ow->dragging)
                return 0;
            if (ow->stickyWindowPid > 0)
            {
                // Keep following the exact window selected during initial layout. Tooltips and menus
                // can share its PID and must not replace the sticky target while that HWND is alive.
                HWND target = ow->targetHwnd;
                if (!IsWindowFromPid(target, ow->stickyWindowPid))
                {
                    target = ow->stickyWindowHwnd;
                    if (!IsWindowFromPid(target, ow->stickyWindowPid))
                    {
                        target = NULL;
                        FindWindowByPid(ow->stickyWindowPid, &target);
                    }
                }
                if (target)
                {
                    BOOL targetChanged = ow->targetHwnd != target;
                    if (targetChanged)
                    {
                        if (ow->locationHook)
                        {
                            UnhookWinEvent(ow->locationHook);
                            ow->locationHook = NULL;
                        }
                        ow->targetHwnd = target;
                        AttachInjectedStickyHook(ow);
                    }
                    if (ow->injectedStickyHook)
                    {
                        if (ow->locationHook)
                        {
                            // The injected target event is lower latency; WinEvent remains available only when injection fails.
                            UnhookWinEvent(ow->locationHook);
                            ow->locationHook = NULL;
                        }
                    }
                    else if (!ow->locationHook)
                    {
                        DWORD pid = 0;
                        DWORD tid = GetWindowThreadProcessId(target, &pid);
                        ow->locationHook = SetWinEventHook(EVENT_OBJECT_LOCATIONCHANGE, EVENT_OBJECT_LOCATIONCHANGE, 
                                                           NULL, OverlayLocationChangeHook, pid, tid, WINEVENT_OUTOFCONTEXT);
                    }
                    if (!ow->injectedStickyHook)
                    {
                        StartLiveFollowTimerIfNeeded(ow);
                        SendMessage(hwnd, WM_WOX_OVERLAY_REPOSITION, 0, 0);
                    }
                }
                else
                {
                    DetachInjectedStickyHook(ow);
                    if (ow->locationHook)
                    {
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
            if (ow->injectedStickyHook)
            {
                StopLiveFollowTimer(ow);
                return 0;
            }
            if (!IsLeftButtonDown() || ow->dragging)
            {
                StopLiveFollowTimer(ow);
                SendMessage(hwnd, WM_WOX_OVERLAY_REPOSITION, 0, 0);
                return 0;
            }

            RECT targetRect;
            BOOL targetFound = GetPredictiveTargetRect(ow, &targetRect);
            if (!targetFound && ow->targetHwnd && IsWindow(ow->targetHwnd))
                targetFound = GetTargetWindowRect(ow->targetHwnd, &targetRect);
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
        if (wParam == TIMER_REPAINT)
        {
            KillTimer(hwnd, TIMER_REPAINT);
            ow->repaintPending = FALSE;
            RedrawWindow(hwnd, NULL, NULL, RDW_INVALIDATE | RDW_UPDATENOW | RDW_ALLCHILDREN);
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
        if (!GetTargetWindowRect(target, &targetRect))
        {
            return 0;
        }

        if (!ow->injectedStickyHook && IsLeftButtonDown())
        {
            RefreshPredictiveAnchor(ow, &targetRect);
            StartLiveFollowTimerIfNeeded(ow);
        }

        SetOverlayZOrder(hwnd, target);
        RepositionOverlayToTargetRect(ow, &targetRect, !ow->injectedStickyHook);
        
        if (!IsWindowVisible(hwnd))
             ShowOverlayWindowWithFocusPolicy(ow);

        return 0;
    }
    case WM_DESTROY:
    {
        if (ow)
        {
            DetachInjectedStickyHook(ow);
            if (ow->locationHook)
            {
                UnhookWinEvent(ow->locationHook);
                ow->locationHook = NULL;
            }
            KillTimer(hwnd, TIMER_TRACK);
            KillTimer(hwnd, TIMER_LIVE_FOLLOW);
            KillTimer(hwnd, TIMER_REPAINT);
            DetachNativeAttachment(ow);
            RemoveOverlay(ow);
            if (ow->name)
                free(ow->name);
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
            free(payload);
        return;
    }

    OverlayWindow *ow = FindOverlayByName(payload->name);
    if (ow && ow->hwnd && IsWindow(ow->hwnd))
    {
        if (IsWindowVisible(ow->hwnd) && OverlayPayloadMatchesCurrent(ow, payload))
        {
            free(payload->name);
            free(payload);
            return;
        }
        ApplyPayloadToOverlay(ow, payload, FALSE);
        ApplyOverlayLayout(ow);
        ShowOverlayWindowWithFocusPolicy(ow);
        if (ow->layoutSizeChanged)
        {
            // Size changes expose new client area immediately. Force that frame to paint now,
            // while same-size streaming updates stay coalesced to keep scrolling responsive.
            if (ow->repaintPending)
            {
                KillTimer(ow->hwnd, TIMER_REPAINT);
                ow->repaintPending = FALSE;
            }
            RedrawWindow(ow->hwnd, NULL, NULL, RDW_INVALIDATE | RDW_UPDATENOW | RDW_ALLCHILDREN);
        }
        else
        {
            ScheduleOverlayRepaint(ow);
        }
        return;
    }

    ow = (OverlayWindow *)calloc(1, sizeof(OverlayWindow));
    if (!ow)
    {
        if (payload->name)
            free(payload->name);
        free(payload);
        return;
    }

    ApplyPayloadToOverlay(ow, payload, TRUE);

    DWORD exStyle = WS_EX_TOOLWINDOW;
    if (!ow->closeOnEscape)
        exStyle |= WS_EX_NOACTIVATE;
    if (ow->transparent && !ow->nativeAttachment)
        exStyle |= WS_EX_LAYERED;
    if (ow->topmost || ow->stickyWindowPid <= 0)
        exStyle |= WS_EX_TOPMOST;

    HWND owner = NULL;
    if (ow->stickyWindowPid > 0)
    {
        owner = ow->stickyWindowHwnd;
        if (!IsWindowFromPid(owner, ow->stickyWindowPid))
        {
            owner = NULL;
            FindWindowByPid(ow->stickyWindowPid, &owner);
        }
        if (!owner)
        {
            exStyle |= WS_EX_TOPMOST;
        }
    }

    DWORD style = WS_POPUP;
    if (ow->resizable && !ow->transparent)
    {
        // Bug fix: transparent overlays must use the full client surface. Applying the system thick
        // frame to that path creates a visible non-client border and shrinks child content, so only
        // non-transparent overlays may ask Windows to draw a resize frame.
        style |= WS_THICKFRAME;
    }

    ow->hwnd = CreateWindowExW(exStyle, g_overlayClassName, L"",
                               style, 0, 0, 0, 0, owner, NULL, GetModuleHandleW(NULL), ow);
    if (!ow->hwnd)
    {
        DWORD err = GetLastError();
        if (owner && (err == 5 || err == ERROR_ACCESS_DENIED))
        {
            owner = NULL;
            exStyle |= WS_EX_TOPMOST;
            ow->hwnd = CreateWindowExW(exStyle, g_overlayClassName, L"",
                                       style, 0, 0, 0, 0, owner, NULL, GetModuleHandleW(NULL), ow);
        }
    }

    if (!ow->hwnd)
    {
        if (ow->name)
            free(ow->name);
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

// NotifyOverlayClose fires the Go-side OnClose callback before the overlay
// window is destroyed by a base-window action such as Escape.
static void NotifyOverlayClose(OverlayWindow *ow)
{
    if (!ow || !ow->name)
        return;
    char *nameUtf8 = DupWideToUtf8(ow->name);
    if (nameUtf8)
    {
        overlayCloseCallbackCGO(nameUtf8);
        free(nameUtf8);
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

    WNDCLASSEXW wc2;
    ZeroMemory(&wc2, sizeof(wc2));
    wc2.cbSize = sizeof(wc2);
    wc2.lpfnWndProc = OverlayControllerProc;
    wc2.hInstance = GetModuleHandleW(NULL);
    wc2.lpszClassName = g_controllerClassName;
    RegisterClassExW(&wc2);

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
    {
        CloseHandle(g_threadReadyEvent);
        g_threadReadyEvent = NULL;
        return FALSE;
    }

    WaitForSingleObject(g_threadReadyEvent, INFINITE);
    CloseHandle(g_threadReadyEvent);
    g_threadReadyEvent = NULL;
    CloseHandle(g_overlayThread);
    g_overlayThread = NULL;
    return TRUE;
}

static void EnsureOverlayThread(void)
{
    InitOnceExecuteOnce(&g_initOnce, InitOverlayThread, NULL, NULL);
}

// -----------------------------------------------------------------------------
// C Exported Functions
// -----------------------------------------------------------------------------

// SetOverlayWindowHookDllPath configures the extracted Windows-only hook before overlays are shown.
void SetOverlayWindowHookDllPath(char *path)
{
    WCHAR *widePath = DupUtf8ToWide(path);
    if (!widePath)
        return;
    if (g_windowHookDllPath)
        free(g_windowHookDllPath);
    g_windowHookDllPath = widePath;
}

void ShowOverlay(OverlayOptions opts)
{
    EnsureOverlayThread();
    if (!g_controllerHwnd)
        return;

    OverlayPayload *payload = (OverlayPayload *)calloc(1, sizeof(OverlayPayload));
    if (!payload)
        return;

    payload->name = DupUtf8ToWide(opts.name);
    payload->transparent = opts.transparent ? TRUE : FALSE;
    payload->hitTestIconOnly = opts.hitTestIconOnly ? TRUE : FALSE;
    payload->closeOnEscape = opts.closeOnEscape ? TRUE : FALSE;
    payload->takeFocus = opts.takeFocus ? TRUE : FALSE;
    payload->nativeAttachment = opts.nativeAttachment ? TRUE : FALSE;
    payload->nativeAttachmentKind = opts.nativeAttachmentKind;
    payload->nativeAttachmentHandle = opts.nativeAttachmentHandle;
    payload->nativeAttachmentWidth = opts.nativeAttachmentWidth;
    payload->nativeAttachmentHeight = opts.nativeAttachmentHeight;
    payload->topmost = opts.topmost ? TRUE : FALSE;
    payload->absolutePosition = opts.absolutePosition ? TRUE : FALSE;
    payload->preservePosition = opts.preservePosition ? TRUE : FALSE;
    payload->stickyWindowPid = opts.stickyWindowPid;
    payload->stickyWindowHwnd = (HWND)opts.stickyWindowHandle;
    payload->anchor = opts.anchor;
    payload->movable = opts.movable ? TRUE : FALSE;
    payload->resizable = opts.resizable ? TRUE : FALSE;
    payload->cornerRadius = opts.cornerRadius;
    payload->aspectRatio = opts.aspectRatio;
    payload->offsetX = opts.offsetX;
    payload->offsetY = opts.offsetY;
    payload->width = opts.width;
    payload->minWidth = opts.minWidth;
    payload->maxWidth = opts.maxWidth;
    payload->height = opts.height;
    payload->maxHeight = opts.maxHeight;

    OverlayCommand *cmd = (OverlayCommand *)calloc(1, sizeof(OverlayCommand));
    if (!cmd)
    {
        if (payload->name)
            free(payload->name);
        free(payload);
        return;
    }
    cmd->type = 1;
    cmd->payload = payload;

    // Apply attachment replacements before returning so Go can safely release
    // the previous native attachment without exposing an empty overlay frame.
    SendMessageW(g_controllerHwnd, WM_WOX_OVERLAY_COMMAND, 0, (LPARAM)cmd);
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
