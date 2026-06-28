//go:build windows && cgo

#define WIN32_LEAN_AND_MEAN
#define INITGUID

// Single source of truth for the Go<->C ABI: shared struct definitions,
// command/event/key enums, and function declarations.
#include "ui_native.h"

#include <windows.h>
#include <windowsx.h>
#include <dwmapi.h>
#include <d2d1.h>
#include <d2d1_1.h>   // ID2D1Factory1, ID2D1Device, ID2D1DeviceContext, ID2D1Bitmap1
#include <d3d11.h>     // D3D11CreateDevice, ID3D11Device
#include <dxgi1_2.h>  // IDXGISwapChain1, CreateSwapChainForComposition
#include <dxgi1_3.h>  // CreateDXGIFactory2 (DXGI 1.3+ supports debug factory flag)
#include <dcomp.h>    // DCompositionCreateDevice, IDComposition*
#include <dwrite.h>
#include <wincodec.h>
#include <objbase.h>
#include <initguid.h>
#include <stdlib.h>
#include <string.h>
#include <stdio.h>
#include <stdarg.h>

// ── Diagnostic logging for Mica/transparency investigation ───────────────
// Writes to C:\Users\qianl\AppData\Local\Temp\opencode\wox-mica.log (append).
// Gated by WOX_MICA_LOG env var so production builds stay quiet by default.
static bool g_micaLogEnabled = false;
static int g_frameCounter = 0;
// Log the first N frames after each ShowWindow call so we can see what gets
// drawn when the window reappears, not just the very first show.
static int g_framesSinceShow = 0;
static const int kLogFramesPerShow = 3;

static void MicaLog(const char* fmt, ...) {
    if (!g_micaLogEnabled) return;
    static FILE* fp = nullptr;
    static bool firstOpen = true;
    if (!fp) {
        fp = fopen("C:\\Users\\qianl\\AppData\\Local\\Temp\\opencode\\wox-mica.log",
                   firstOpen ? "w" : "a");
        firstOpen = false;
        if (!fp) return;
    }
    SYSTEMTIME st;
    GetLocalTime(&st);
    fprintf(fp, "%02d:%02d:%02d.%03d ", st.wHour, st.wMinute, st.wSecond,
            st.wMilliseconds);
    va_list args;
    va_start(args, fmt);
    vfprintf(fp, fmt, args);
    va_end(args);
    fprintf(fp, "\n");
    fflush(fp);
}

static void MicaLogInit(void) {
    char buf[8] = {0};
    GetEnvironmentVariableA("WOX_MICA_LOG", buf, sizeof(buf));
    g_micaLogEnabled = (buf[0] == '1' || buf[0] == 'y' || buf[0] == 'Y');
    MicaLog("=== MicaLog init (enabled=%d) ===", (int)g_micaLogEnabled);
}

// ── Types matching the Go CGO declarations ──────────────────────────────

// Custom messages posted from other goroutines to drive show/hide/repaint on
// the main thread. Posting to the message queue is thread-safe and ensures
// the action runs inside StartEventLoop's GetMessage dispatch, avoiding
// cross-thread Win32 window calls that race with the message loop.
#define WM_APP_SHOW    (WM_APP + 1)
#define WM_APP_HIDE    (WM_APP + 2)
#define WM_APP_REPAINT (WM_APP + 3)
#define WM_APP_RESIZE  (WM_APP + 4)

// ── DPI awareness ───────────────────────────────────────────────────────

static void EnablePerMonitorDPI(void) {
    HMODULE user32 = GetModuleHandleW(L"user32.dll");
    if (user32) {
        typedef BOOL(WINAPI *pfnSetProcessDpiAwarenessContext)(HANDLE);
        pfnSetProcessDpiAwarenessContext fn = (pfnSetProcessDpiAwarenessContext)GetProcAddress(user32, "SetProcessDpiAwarenessContext");
        if (fn) {
            fn((HANDLE)-4); // PER_MONITOR_AWARE_V2
            return;
        }
    }
    SetProcessDPIAware();
}

static float GetDPIForHWND(HWND hwnd) {
    HMODULE user32 = GetModuleHandleW(L"user32.dll");
    if (user32) {
        typedef UINT(WINAPI *pfnGetDpiForWindow)(HWND);
        pfnGetDpiForWindow fn = (pfnGetDpiForWindow)GetProcAddress(user32, "GetDpiForWindow");
        if (fn) return (float)fn(hwnd);
    }
    HDC hdc = GetDC(NULL);
    float dpi = (float)GetDeviceCaps(hdc, LOGPIXELSX);
    ReleaseDC(NULL, hdc);
    return dpi > 0 ? dpi : 96.0f;
}

// ── UIWindow: one native window with Direct2D rendering ─────────────────

typedef struct UIBitmapCacheEntry {
    char* key;
    int32_t keyLen;
    ID2D1Bitmap* bitmap;
    struct UIBitmapCacheEntry* next;
} UIBitmapCacheEntry;

typedef struct {
    HWND hwnd;
    // Direct2D/DirectWrite factories remain shared across the window lifetime.
    ID2D1Factory1* d2dFactory;
    ID2D1Device* d2dDevice;
    ID2D1DeviceContext* rt;   // device-context render target bound to a swap-chain bitmap
    IDXGISwapChain1* swapChain;
    ID2D1Bitmap1* backBufferBitmap; // wraps the DXGI back buffer each frame
    ID3D11Device* d3dDevice;
    IDXGIFactory2* dxgiFactory;
    IDCompositionDevice* dcompDevice;
    IDCompositionTarget* dcompTarget;
    IDCompositionVisual* dcompVisual;
    IDWriteFactory* dwriteFactory;
    IDWriteTextFormat* textFormat;
    IWICImagingFactory* wicFactory;

    float dpi;
    float scale;          // dpi / 96
    int32_t width;        // logical (DIP)
    int32_t height;
    float cornerRadius;
    bool transparent;
    bool darkMode;   // tracks the DWMWA_USE_IMMERSIVE_DARK_MODE state
    bool visible;

    // Clip stack for PushClip/PopClip
    D2D1_RECT_F clipStack[32];
    int clipDepth;

    UIBitmapCacheEntry* bitmapCache;

    // Toolbar drag region in DIP (y1=top, y2=bottom). When the cursor is in
    // this band and not on an interactive element, WM_NCHITTEST returns
    // HTCAPTION so the user can drag the frameless window from the toolbar.
    float dragY1;
    float dragY2;
} UIWindow;

static UIWindow* g_windows[16];
static int g_windowCount = 0;
static const wchar_t* g_className = L"WoxUIWindow";
static bool g_classRegistered = false;

// Find UIWindow by HWND
static UIWindow* FindWindowByHWND(HWND hwnd) {
    for (int i = 0; i < g_windowCount; i++) {
        if (g_windows[i] && g_windows[i]->hwnd == hwnd)
            return g_windows[i];
    }
    return NULL;
}

// ── Color helpers ───────────────────────────────────────────────────────

static D2D1_COLOR_F ToColorF(float r, float g, float b, float a) {
    D2D1_COLOR_F c = { r, g, b, a };
    return c;
}

static D2D1_RECT_F ToRectF(float x, float y, float w, float h) {
    D2D1_RECT_F r = { x, y, x + w, y + h };
    return r;
}

static ID2D1Bitmap* FindCachedBitmap(UIWindow* win, const char* key, int32_t keyLen) {
    if (!win || !key || keyLen <= 0) return NULL;
    for (UIBitmapCacheEntry* entry = win->bitmapCache; entry; entry = entry->next) {
        if (entry->keyLen == keyLen && memcmp(entry->key, key, keyLen) == 0) {
            return entry->bitmap;
        }
    }
    return NULL;
}

static bool CacheBitmap(UIWindow* win, const char* key, int32_t keyLen, ID2D1Bitmap* bitmap) {
    if (!win || !key || keyLen <= 0 || !bitmap || FindCachedBitmap(win, key, keyLen)) return false;

    UIBitmapCacheEntry* entry = (UIBitmapCacheEntry*)calloc(1, sizeof(UIBitmapCacheEntry));
    if (!entry) return false;
    entry->key = (char*)malloc((size_t)keyLen);
    if (!entry->key) {
        free(entry);
        return false;
    }
    memcpy(entry->key, key, (size_t)keyLen);
    entry->keyLen = keyLen;
    entry->bitmap = bitmap;
    entry->next = win->bitmapCache;
    win->bitmapCache = entry;
    return true;
}

static void ClearBitmapCache(UIWindow* win) {
    if (!win) return;
    UIBitmapCacheEntry* entry = win->bitmapCache;
    while (entry) {
        UIBitmapCacheEntry* next = entry->next;
        if (entry->bitmap) entry->bitmap->Release();
        free(entry->key);
        free(entry);
        entry = next;
    }
    win->bitmapCache = NULL;
}

static DWORD GetWindowsBuildNumberCpp(void) {
    OSVERSIONINFOEXW osvi = { 0 };
    osvi.dwOSVersionInfoSize = sizeof(osvi);
    // RtlGetVersion returns NTSTATUS, which is a LONG in the Windows headers.
    // MinGW does not expose NTSTATUS from <windows.h>, so use the equivalent
    // LONG return type to avoid pulling in <winternl.h>.
    typedef LONG(WINAPI *pfnRtlGetVersion)(PRTL_OSVERSIONINFOW);
    HMODULE ntdll = GetModuleHandleW(L"ntdll.dll");
    if (!ntdll) return 0;
    pfnRtlGetVersion fn = (pfnRtlGetVersion)GetProcAddress(ntdll, "RtlGetVersion");
    if (!fn) return 0;
    fn((PRTL_OSVERSIONINFOW)&osvi);
    return osvi.dwBuildNumber;
}

// Enable the native DWM system backdrop so translucent app backgrounds expose
// the Mica material. Windows 11 (build >= 22000) uses DWMSBT_TABBEDWINDOW
// (Mica Alt) to match the Flutter settings window; older builds get the legacy
// acrylic fallback via SetWindowCompositionAttribute, which is applied later
// by the host process if needed.
static void EnableMicaBackdrop(HWND hwnd) {
    MARGINS margins = { -1 };
    DwmExtendFrameIntoClientArea(hwnd, &margins);

    if (GetWindowsBuildNumberCpp() >= 22000) {
        // 3 == DWMSBT_TABBEDWINDOW (Mica Alt), matches wox.ui.flutter/.../win32_window.cpp
        int backdrop = 3;
        DwmSetWindowAttribute(hwnd, 38 /*DWMWA_SYSTEMBACKDROP_TYPE*/, &backdrop, sizeof(backdrop));
    }
}

// ── Window proc ─────────────────────────────────────────────────────────

static LRESULT CALLBACK WndProc(HWND hwnd, UINT msg, WPARAM wParam, LPARAM lParam) {
    UIWindow* win = FindWindowByHWND(hwnd);

    switch (msg) {
    case WM_PAINT: {
        if (!win || !win->rt) {
            ValidateRect(hwnd, NULL);
            return 0;
        }
        // Go side drives rendering via uiWindowRender.
        // WM_PAINT just validates the rect; actual drawing happens in Render().
        ValidateRect(hwnd, NULL);
        return 0;
    }

    case WM_SIZE: {
        if (win && win->swapChain) {
            int w = LOWORD(lParam);
            int h = HIWORD(lParam);
            if (w > 0 && h > 0) {
                MicaLog("WM_SIZE w=%d h=%d (resizing swap chain + bitmap)", w, h);
                // Resize the composition swap chain. The D2D back-buffer bitmap
                // must be recreated from the resized surface to stay in sync.
                win->rt->SetTarget(NULL);
                if (win->backBufferBitmap) {
                    win->backBufferBitmap->Release();
                    win->backBufferBitmap = NULL;
                }
                HRESULT resizeHr = win->swapChain->ResizeBuffers(0, w, h,
                    DXGI_FORMAT_B8G8R8A8_UNORM, 0);
                MicaLog("WM_SIZE ResizeBuffers hr=0x%X", (unsigned)resizeHr);

                IDXGISurface* backBuffer = NULL;
                if (SUCCEEDED(win->swapChain->GetBuffer(0, IID_PPV_ARGS(&backBuffer))) && backBuffer) {
                    D2D1_BITMAP_PROPERTIES1 bmpProps = {};
                    bmpProps.pixelFormat.format = DXGI_FORMAT_B8G8R8A8_UNORM;
                    bmpProps.pixelFormat.alphaMode = D2D1_ALPHA_MODE_PREMULTIPLIED;
                    bmpProps.dpiX = win->dpi;
                    bmpProps.dpiY = win->dpi;
                    bmpProps.bitmapOptions = D2D1_BITMAP_OPTIONS_TARGET |
                                             D2D1_BITMAP_OPTIONS_CANNOT_DRAW;
                    HRESULT bmpHr = win->rt->CreateBitmapFromDxgiSurface(backBuffer, &bmpProps,
                        &win->backBufferBitmap);
                    backBuffer->Release();
                    MicaLog("WM_SIZE CreateBitmapFromDxgiSurface hr=0x%X bitmap=%p",
                            (unsigned)bmpHr, win->backBufferBitmap);
                    if (win->backBufferBitmap) {
                        win->rt->SetTarget(win->backBufferBitmap);
                    }
                }

                win->width = (int32_t)(w / win->scale);
                win->height = (int32_t)(h / win->scale);
            }
        }
        return 0;
    }

    case WM_KEYDOWN: {
        if (win) {
            // Map VK codes to our Key enum (simplified �?full mapping later)
            int32_t key = 0;
            switch (wParam) {
                case VK_ESCAPE: key = KeyEscape; break;
                case VK_RETURN:  key = KeyEnter; break;
                case VK_BACK:    key = KeyBackspace; break;
                case VK_TAB:     key = KeyTab; break;
                case VK_SPACE:   key = KeySpace; break;
                case VK_UP:      key = KeyUp; break;
                case VK_DOWN:    key = KeyDown; break;
                case VK_LEFT:    key = KeyLeft; break;
                case VK_RIGHT:   key = KeyRight; break;
                case VK_HOME:    key = KeyHome; break;
                case VK_END:     key = KeyEnd; break;
                case VK_PRIOR:   key = KeyPageUp; break;
                case VK_NEXT:    key = KeyPageDown; break;
                case VK_DELETE:   key = KeyDelete; break;
                case VK_F1:  key = KeyF1; break;
                case VK_F2:  key = KeyF2; break;
                case VK_F3:  key = KeyF3; break;
                case VK_F4:  key = KeyF4; break;
                case VK_F5:  key = KeyF5; break;
                case VK_F6:  key = KeyF6; break;
                case VK_F7:  key = KeyF7; break;
                case VK_F8:  key = KeyF8; break;
                case VK_F9:  key = KeyF9; break;
                case VK_F10: key = KeyF10; break;
                case VK_F11: key = KeyF11; break;
                case VK_F12: key = KeyF12; break;
                default:
                    if (wParam >= 0x41 && wParam <= 0x5A) {
                        key = (int32_t)(KeyA + (wParam - 0x41));
                    } else if (wParam >= 0x30 && wParam <= 0x39) {
                        key = (int32_t)(Key0 + (wParam - 0x30));
                    }
                    break;
            }
            int32_t mods = 0;
            if (GetKeyState(VK_SHIFT) & 0x8000) mods |= 1;
            if (GetKeyState(VK_CONTROL) & 0x8000) mods |= 2;
            if (GetKeyState(VK_MENU) & 0x8000) mods |= 4;
            uiEventCallback((int32_t)(intptr_t)win->hwnd, EventKeyPress, key, mods,
                NULL, 0, NULL, 0, 0, 0, 0, 0, 0, 0);
        }
        return 0;
    }

    case WM_CHAR: {
        if (win) {
            wchar_t wc = (wchar_t)wParam;
            char utf8[8];
            int len = WideCharToMultiByte(CP_UTF8, 0, &wc, 1, utf8, sizeof(utf8), NULL, NULL);
            if (len > 0 && wc >= 32) {
                uiEventCallback((int32_t)(intptr_t)win->hwnd, EventTextInput, 0, 0,
                    utf8, len, NULL, 0, 0, 0, 0, 0, 0, 0);
            }
        }
        return 0;
    }

    case WM_KILLFOCUS: {
        if (win) {
            win->visible = false;
            uiEventCallback((int32_t)(intptr_t)win->hwnd, EventFocusLost, 0, 0,
                NULL, 0, NULL, 0, 0, 0, 0, 0, 0, 0);
        }
        return 0;
    }

    case WM_DESTROY: {
        PostQuitMessage(0);
        return 0;
    }

    case WM_APP_SHOW: {
        if (win) {
            DWORD build = GetWindowsBuildNumberCpp();
            if (build >= 22000) {
                // Re-assert DWM backdrop before showing.
                int backdrop = 3; // DWMSBT_TABBEDWINDOW (Mica Alt)
                DwmSetWindowAttribute(hwnd, 38, &backdrop, sizeof(backdrop));
                BOOL useDark = win->darkMode ? TRUE : FALSE;
                DwmSetWindowAttribute(hwnd, 20, &useDark, sizeof(useDark));
                MARGINS margins = { -1 };
                DwmExtendFrameIntoClientArea(hwnd, &margins);
            }

            ShowWindow(hwnd, SW_SHOW);
            win->visible = true;
            g_framesSinceShow = 0;

            // Force DWM to re-evaluate the system backdrop after SW_SHOW.
            // A +1/-1 height nudge with SWP_FRAMECHANGED creates a real
            // WM_SIZE cycle that makes DWM re-attach the Mica material.
            // This mirrors the Flutter runner's delayedResizeRepaintNudge.
            RECT rc;
            if (GetWindowRect(hwnd, &rc)) {
                int w = rc.right - rc.left;
                int h = rc.bottom - rc.top;
                SetWindowPos(hwnd, NULL, rc.left, rc.top, w, h + 1,
                    SWP_NOZORDER | SWP_NOACTIVATE | SWP_FRAMECHANGED);
                SetWindowPos(hwnd, NULL, rc.left, rc.top, w, h,
                    SWP_NOZORDER | SWP_NOACTIVATE | SWP_FRAMECHANGED);
            }
            MicaLog("WM_APP_SHOW: ShowWindow + DWM backdrop + +1/-1 nudge");
        }
        return 0;
    }

    case WM_APP_HIDE: {
        if (win) {
            ShowWindow(hwnd, SW_HIDE);
            win->visible = false;
            MicaLog("WM_APP_HIDE: ShowWindow(SW_HIDE)");
        }
        return 0;
    }

    case WM_APP_REPAINT: {
        return 0;
    }

    case WM_APP_RESIZE: {
        // Cross-thread resize request: posted by uiWindowSetSize from a goroutine.
        // SetWindowPos runs on the main thread and synchronously triggers WM_SIZE,
        // which rebuilds the swap chain bitmap. When uiPumpMessages returns, onRender
        // uses the correctly-sized bitmap — no gap between window size and render
        // surface, eliminating the "half Mica" artifact during fast typing.
        if (win) {
            int physW = (int)wParam;
            int physH = (int)(LONG_PTR)lParam;
            if (physW > 0 && physH > 0) {
                SetWindowPos(hwnd, NULL, 0, 0, physW, physH,
                    SWP_NOMOVE | SWP_NOZORDER | SWP_FRAMECHANGED);
                MicaLog("WM_APP_RESIZE: SetWindowPos %dx%d (bitmap rebuilt via WM_SIZE)", physW, physH);
            }
        }
        return 0;
    }

    case WM_NCHITTEST: {
        // Frameless window: make the top 8px draggable, plus the toolbar drag
        // band (set via SetDragRegion) so the user can move the window from the
        // bottom toolbar.
        if (win) {
            POINT pt = { GET_X_LPARAM(lParam), GET_Y_LPARAM(lParam) };
            ScreenToClient(hwnd, &pt);
            if (pt.y < 8) {
                return HTCAPTION;
            }
            // Check toolbar drag region (convert DIP to physical pixels).
            float dragY1Pix = win->dragY1 * win->scale;
            float dragY2Pix = win->dragY2 * win->scale;
            if (win->dragY2 > win->dragY1 && pt.y >= (int)dragY1Pix && pt.y <= (int)dragY2Pix) {
                return HTCAPTION;
            }
        }
        return HTCLIENT;
    }
    }

    return DefWindowProcW(hwnd, msg, wParam, lParam);
}

// ── Window creation ─────────────────────────────────────────────────────

static void RegisterWindowClass(void) {
    if (g_classRegistered) return;
    WNDCLASSEXW wc = { sizeof(wc) };
    wc.lpfnWndProc = WndProc;
    wc.hInstance = GetModuleHandleW(NULL);
    wc.lpszClassName = g_className;
    wc.hCursor = LoadCursorW(NULL, (LPCWSTR)IDC_ARROW);
    wc.hbrBackground = NULL; // we paint everything
    RegisterClassExW(&wc);
    g_classRegistered = true;
}

extern "C" int32_t uiWindowCreate(CWindowConfig config) {
    MicaLogInit();
    EnablePerMonitorDPI();
    RegisterWindowClass();
    MicaLog("uiWindowCreate: w=%d h=%d radius=%.1f frameless=%d transparent=%d",
            config.width, config.height, config.cornerRadius,
            (int)config.frameless, (int)config.transparent);

    UIWindow* win = (UIWindow*)calloc(1, sizeof(UIWindow));
    if (!win) return 0;

    win->width = config.width;
    win->height = config.height;
    win->cornerRadius = config.cornerRadius;
    win->transparent = config.transparent;
    win->darkMode = config.darkMode;
    win->visible = false;
    win->clipDepth = 0;

    // Create frameless window. WS_EX_NOREDIRECTIONBITMAP is required for
    // DirectComposition alpha compositing: without it DWM allocates a GDI
    // redirection surface that paints solid black behind our swap chain, so
    // the Mica backdrop never shows through translucent app backgrounds.
    DWORD exStyle = WS_EX_TOOLWINDOW | WS_EX_TOPMOST | WS_EX_NOREDIRECTIONBITMAP;
    DWORD style = WS_POPUP;

    // Estimate DPI for initial size
    float dpi = 96.0f;
    HMODULE user32 = GetModuleHandleW(L"user32.dll");
    if (user32) {
        typedef UINT(WINAPI *pfnGetDpiForSystem)(void);
        pfnGetDpiForSystem fn = (pfnGetDpiForSystem)GetProcAddress(user32, "GetDpiForSystem");
        if (fn) dpi = (float)fn();
    }
    win->dpi = dpi;
    win->scale = dpi / 96.0f;

    int physW = (int)(config.width * win->scale);
    int physH = (int)(config.height * win->scale);

    win->hwnd = CreateWindowExW(
        exStyle, g_className, L"Wox",
        style,
        CW_USEDEFAULT, CW_USEDEFAULT, physW, physH,
        NULL, NULL, GetModuleHandleW(NULL), NULL);

    if (!win->hwnd) {
        MicaLog("uiWindowCreate: CreateWindowExW FAILED");
        free(win);
        return 0;
    }

    LONG actualEx = GetWindowLongW(win->hwnd, GWL_EXSTYLE);
    MicaLog("uiWindowCreate: hwnd=0x%p exStyle=0x%X NOREDIR=%d LAYERED=%d",
            win->hwnd, (unsigned)actualEx,
            (actualEx & 0x200000) ? 1 : 0,
            (actualEx & 0x80000) ? 1 : 0);

    // Get actual DPI for the window
    win->dpi = GetDPIForHWND(win->hwnd);
    win->scale = win->dpi / 96.0f;
    physW = (int)(config.width * win->scale);
    physH = (int)(config.height * win->scale);
    SetWindowPos(win->hwnd, NULL, 0, 0, physW, physH, SWP_NOMOVE | SWP_NOZORDER);

    // Enable rounded corners via DWM
    if (config.cornerRadius > 0) {
        int preference = 2; // DWMWCP_ROUND
        DwmSetWindowAttribute(win->hwnd, 33 /*DWMWA_WINDOW_CORNER_PREFERENCE*/,
            &preference, sizeof(preference));
    }
    EnableMicaBackdrop(win->hwnd);

    // Tell DWM whether this window should render its system backdrop (Mica)
    // in dark or light tone. Without this, DWM follows the OS appearance, so a
    // dark theme like glass-dark would get a light-toned Mica when the system
    // is in light mode. DWMWA_USE_IMMERSIVE_DARK_MODE = 20.
    BOOL useDark = config.darkMode ? TRUE : FALSE;
    DwmSetWindowAttribute(win->hwnd, 20, &useDark, sizeof(useDark));

    // Verify DWM backdrop actually applied. The Win11 build check inside
    // EnableMicaBackdrop may have skipped the attribute on older builds.
    int actualBackdrop = -1;
    DwmGetWindowAttribute(win->hwnd, 38, &actualBackdrop, sizeof(actualBackdrop));
    DWORD buildNo = GetWindowsBuildNumberCpp();
    MicaLog("uiWindowCreate: build=%u DWMWA_SYSTEMBACKDROP_TYPE readback=%d (expect 3 on Win11)",
            buildNo, actualBackdrop);

    // Initialize Direct2D factory (ID2D1Factory1 needed for device/context).
    D2D1_FACTORY_OPTIONS opts = { D2D1_DEBUG_LEVEL_NONE };
    HRESULT hr = D2D1CreateFactory(D2D1_FACTORY_TYPE_SINGLE_THREADED,
        opts, &win->d2dFactory);
    if (FAILED(hr)) {
        DestroyWindow(win->hwnd);
        free(win);
        return 0;
    }

    // Create the D3D11 device + DXGI swap chain + DirectComposition visual so
    // the Direct2D surface participates in per-pixel alpha compositing with DWM.
    // Without this composition path a Clear(0,0,0,0) fills the window with solid
    // black instead of exposing the native Mica backdrop.
    // B8G8R8A8_UNORM + PREMULTIPLIED keeps the back buffer alpha-compatible so
    // translucent app backgrounds blend with the system backdrop.
    D3D_FEATURE_LEVEL featureLevels[] = {
        D3D_FEATURE_LEVEL_11_1, D3D_FEATURE_LEVEL_11_0,
        D3D_FEATURE_LEVEL_10_1, D3D_FEATURE_LEVEL_10_0,
        D3D_FEATURE_LEVEL_9_3
    };
    UINT d3dFlags = D3D11_CREATE_DEVICE_BGRA_SUPPORT;
    hr = D3D11CreateDevice(NULL, D3D_DRIVER_TYPE_HARDWARE, NULL, d3dFlags,
        featureLevels, ARRAYSIZE(featureLevels), D3D11_SDK_VERSION,
        &win->d3dDevice, NULL, NULL);
    if (FAILED(hr) || !win->d3dDevice) {
        win->d2dFactory->Release();
        DestroyWindow(win->hwnd);
        free(win);
        return 0;
    }

    IDXGIDevice* dxgiDevice = NULL;
    hr = win->d3dDevice->QueryInterface(IID_PPV_ARGS(&dxgiDevice));
    if (FAILED(hr) || !dxgiDevice) {
        win->d3dDevice->Release();
        win->d2dFactory->Release();
        DestroyWindow(win->hwnd);
        free(win);
        return 0;
    }

    hr = CreateDXGIFactory2(0, IID_PPV_ARGS(&win->dxgiFactory));
    if (FAILED(hr) || !win->dxgiFactory) {
        dxgiDevice->Release();
        win->d3dDevice->Release();
        win->d2dFactory->Release();
        DestroyWindow(win->hwnd);
        free(win);
        return 0;
    }

    // Flip-model swap chain is required for DirectComposition. PREMULTIPLIED
    // alpha lets the DWM blend our translucent pixels with the system backdrop.
    DXGI_SWAP_CHAIN_DESC1 scd = {};
    scd.Width = (UINT)physW;
    scd.Height = (UINT)physH;
    scd.Format = DXGI_FORMAT_B8G8R8A8_UNORM;
    scd.SampleDesc.Count = 1;
    scd.BufferUsage = DXGI_USAGE_RENDER_TARGET_OUTPUT;
    scd.BufferCount = 2;
    scd.SwapEffect = DXGI_SWAP_EFFECT_FLIP_SEQUENTIAL;
    scd.AlphaMode = DXGI_ALPHA_MODE_PREMULTIPLIED;
    scd.Scaling = DXGI_SCALING_STRETCH;

    hr = win->dxgiFactory->CreateSwapChainForComposition(win->d3dDevice, &scd,
        NULL, &win->swapChain);
    if (FAILED(hr) || !win->swapChain) {
        win->dxgiFactory->Release();
        dxgiDevice->Release();
        win->d3dDevice->Release();
        win->d2dFactory->Release();
        DestroyWindow(win->hwnd);
        free(win);
        return 0;
    }

    // Bridge Direct2D to the DXGI back buffer through a D2D bitmap.
    ID2D1Device* d2dDevice = NULL;
    hr = win->d2dFactory->CreateDevice(dxgiDevice, &d2dDevice);
    if (FAILED(hr) || !d2dDevice) {
        win->swapChain->Release();
        win->dxgiFactory->Release();
        dxgiDevice->Release();
        win->d3dDevice->Release();
        win->d2dFactory->Release();
        DestroyWindow(win->hwnd);
        free(win);
        return 0;
    }
    win->d2dDevice = d2dDevice;

    hr = d2dDevice->CreateDeviceContext(
        D2D1_DEVICE_CONTEXT_OPTIONS_NONE, &win->rt);
    if (FAILED(hr) || !win->rt) {
        d2dDevice->Release();
        win->swapChain->Release();
        win->dxgiFactory->Release();
        dxgiDevice->Release();
        win->d3dDevice->Release();
        win->d2dFactory->Release();
        DestroyWindow(win->hwnd);
        free(win);
        return 0;
    }

    // Set the DPI on the device context so DIP coordinates map to physical
    // pixels correctly on high-DPI displays.
    win->rt->SetDpi(win->dpi, win->dpi);

    // Wrap the DXGI back buffer as a D2D bitmap and bind it as the render target.
    IDXGISurface* backBuffer = NULL;
    hr = win->swapChain->GetBuffer(0, IID_PPV_ARGS(&backBuffer));
    if (FAILED(hr) || !backBuffer) {
        win->rt->Release();
        d2dDevice->Release();
        win->swapChain->Release();
        win->dxgiFactory->Release();
        dxgiDevice->Release();
        win->d3dDevice->Release();
        win->d2dFactory->Release();
        DestroyWindow(win->hwnd);
        free(win);
        return 0;
    }

    D2D1_BITMAP_PROPERTIES1 bmpProps = {};
    bmpProps.pixelFormat.format = DXGI_FORMAT_B8G8R8A8_UNORM;
    bmpProps.pixelFormat.alphaMode = D2D1_ALPHA_MODE_PREMULTIPLIED;
    bmpProps.dpiX = win->dpi;
    bmpProps.dpiY = win->dpi;
    bmpProps.bitmapOptions = D2D1_BITMAP_OPTIONS_TARGET |
                             D2D1_BITMAP_OPTIONS_CANNOT_DRAW;

    hr = win->rt->CreateBitmapFromDxgiSurface(backBuffer, &bmpProps,
        &win->backBufferBitmap);
    backBuffer->Release();
    if (FAILED(hr) || !win->backBufferBitmap) {
        win->rt->Release();
        d2dDevice->Release();
        win->swapChain->Release();
        win->dxgiFactory->Release();
        dxgiDevice->Release();
        win->d3dDevice->Release();
        win->d2dFactory->Release();
        DestroyWindow(win->hwnd);
        free(win);
        return 0;
    }

    win->rt->SetTarget(win->backBufferBitmap);

    // Create the DirectComposition target+visual and attach the swap chain so
    // the DWM composites our alpha-bearing surface on top of the HWND.
    hr = DCompositionCreateDevice(dxgiDevice, IID_PPV_ARGS(&win->dcompDevice));
    dxgiDevice->Release();
    if (FAILED(hr) || !win->dcompDevice) {
        win->backBufferBitmap->Release();
        win->rt->Release();
        d2dDevice->Release();
        win->swapChain->Release();
        win->dxgiFactory->Release();
        win->d3dDevice->Release();
        win->d2dFactory->Release();
        DestroyWindow(win->hwnd);
        free(win);
        return 0;
    }

    hr = win->dcompDevice->CreateTargetForHwnd(win->hwnd, TRUE,
        &win->dcompTarget);
    if (FAILED(hr) || !win->dcompTarget) {
        win->dcompDevice->Release();
        win->backBufferBitmap->Release();
        win->rt->Release();
        d2dDevice->Release();
        win->swapChain->Release();
        win->dxgiFactory->Release();
        win->d3dDevice->Release();
        win->d2dFactory->Release();
        DestroyWindow(win->hwnd);
        free(win);
        return 0;
    }

    hr = win->dcompDevice->CreateVisual(&win->dcompVisual);
    if (FAILED(hr) || !win->dcompVisual) {
        win->dcompTarget->Release();
        win->dcompDevice->Release();
        win->backBufferBitmap->Release();
        win->rt->Release();
        d2dDevice->Release();
        win->swapChain->Release();
        win->dxgiFactory->Release();
        win->d3dDevice->Release();
        win->d2dFactory->Release();
        DestroyWindow(win->hwnd);
        free(win);
        return 0;
    }

    win->dcompVisual->SetContent(win->swapChain);
    win->dcompTarget->SetRoot(win->dcompVisual);
    hr = win->dcompDevice->Commit();
    MicaLog("uiWindowCreate: DComp setup done. dcompDevice=0x%p target=0x%p visual=0x%p "
            "swapChain=0x%p d2dDevice=0x%p rt=0x%p backBufferBitmap=0x%p Commit hr=0x%X",
            win->dcompDevice, win->dcompTarget, win->dcompVisual,
            win->swapChain, win->d2dDevice, win->rt, win->backBufferBitmap, (unsigned)hr);

    // Initialize DirectWrite
    hr = DWriteCreateFactory(DWRITE_FACTORY_TYPE_SHARED, IID_IDWriteFactory,
        (IUnknown**)&win->dwriteFactory);
    if (FAILED(hr)) {
        win->dcompVisual->Release();
        win->dcompTarget->Release();
        win->dcompDevice->Release();
        win->backBufferBitmap->Release();
        win->rt->Release();
        d2dDevice->Release();
        win->swapChain->Release();
        win->dxgiFactory->Release();
        win->d3dDevice->Release();
        win->d2dFactory->Release();
        DestroyWindow(win->hwnd);
        free(win);
        return 0;
    }

    // Initialize WIC factory for image decoding
    hr = CoCreateInstance(CLSID_WICImagingFactory, NULL, CLSCTX_INPROC_SERVER,
        IID_IWICImagingFactory, (void**)&win->wicFactory);
    if (FAILED(hr)) {
        win->wicFactory = NULL; // non-fatal, images just won't render
    }

    // Register in global array
    if (g_windowCount < 16) {
        g_windows[g_windowCount++] = win;
    } else {
        win->dcompVisual->Release();
        win->dcompTarget->Release();
        win->dcompDevice->Release();
        win->backBufferBitmap->Release();
        win->rt->Release();
        win->d2dDevice->Release();
        win->swapChain->Release();
        win->dxgiFactory->Release();
        win->d3dDevice->Release();
        win->dwriteFactory->Release();
        if (win->wicFactory) win->wicFactory->Release();
        win->d2dFactory->Release();
        DestroyWindow(win->hwnd);
        free(win);
        return 0;
    }

    return (int32_t)(intptr_t)win->hwnd;
}

// ── Window lifecycle ────────────────────────────────────────────────────

extern "C" void uiWindowShow(int32_t windowId) {
    HWND hwnd = (HWND)(intptr_t)windowId;
    if (!hwnd) return;
    // Post to the message queue so ShowWindow runs on the main thread inside
    // the message loop. Direct cross-thread ShowWindow races with GetMessage
    // and can leave the window in a half-visible state (the "needs two hotkey
    // presses" bug).
    PostMessageW(hwnd, WM_APP_SHOW, 0, 0);
}

extern "C" void uiWindowHide(int32_t windowId) {
    HWND hwnd = (HWND)(intptr_t)windowId;
    if (!hwnd) return;
    PostMessageW(hwnd, WM_APP_HIDE, 0, 0);
}

// Toggle DWM immersive dark mode so the Mica backdrop matches the theme tone.
// Called when the user switches between dark and light themes at runtime.
extern "C" void uiWindowSetDarkMode(int32_t windowId, bool darkMode) {
    HWND hwnd = (HWND)(intptr_t)windowId;
    if (!hwnd) return;
    BOOL useDark = darkMode ? TRUE : FALSE;
    DwmSetWindowAttribute(hwnd, 20 /*DWMWA_USE_IMMERSIVE_DARK_MODE*/,
        &useDark, sizeof(useDark));
    UIWindow* win = FindWindowByHWND(hwnd);
    if (win) win->darkMode = darkMode;
}

extern "C" void uiWindowReleaseMemory(int32_t windowId) {
    HWND hwnd = (HWND)(intptr_t)windowId;
    UIWindow* win = FindWindowByHWND(hwnd);
    if (!win) return;
    ClearBitmapCache(win);
}

extern "C" void uiWindowSetPosition(int32_t windowId, int32_t x, int32_t y) {
    HWND hwnd = (HWND)(intptr_t)windowId;
    UIWindow* win = FindWindowByHWND(hwnd);
    if (!win) return;
    SetWindowPos(hwnd, NULL, (int)(x * win->scale), (int)(y * win->scale),
        0, 0, SWP_NOSIZE | SWP_NOZORDER);
}

extern "C" void uiWindowSetSize(int32_t windowId, int32_t w, int32_t h) {
    HWND hwnd = (HWND)(intptr_t)windowId;
    UIWindow* win = FindWindowByHWND(hwnd);
    if (!win) return;
    // Post the resize to the message queue so SetWindowPos runs on the main
    // thread. This atomically chains SetWindowPos → WM_SIZE (bitmap rebuild)
    // → onRender in a single message-loop pass, avoiding the race where a
    // separate WM_APP_REPAINT renders with a stale bitmap before the swap
    // chain has been resized.
    int physW = (int)(w * win->scale);
    int physH = (int)(h * win->scale);
    PostMessageW(hwnd, WM_APP_RESIZE, (WPARAM)physW, (LPARAM)physH);
}

extern "C" bool uiWindowIsVisible(int32_t windowId) {
    HWND hwnd = (HWND)(intptr_t)windowId;
    UIWindow* win = FindWindowByHWND(hwnd);
    if (!win) return false;
    return win->visible;
}

// Return the current logical (DIP) window size so the Go layout pass uses the
// real dimensions instead of a hardcoded constant.
extern "C" void uiWindowGetSize(int32_t windowId, int32_t* outW, int32_t* outH) {
    HWND hwnd = (HWND)(intptr_t)windowId;
    UIWindow* win = FindWindowByHWND(hwnd);
    if (!win || !outW || !outH) {
        if (outW) *outW = 0;
        if (outH) *outH = 0;
        return;
    }
    *outW = win->width;
    *outH = win->height;
}

extern "C" float uiWindowGetDPI(int32_t windowId) {
    HWND hwnd = (HWND)(intptr_t)windowId;
    UIWindow* win = FindWindowByHWND(hwnd);
    if (!win) return 96.0f;
    return win->dpi;
}

extern "C" void uiWindowSetDragRegion(int32_t windowId, float y1, float y2) {
    HWND hwnd = (HWND)(intptr_t)windowId;
    UIWindow* win = FindWindowByHWND(hwnd);
    if (!win) return;
    win->dragY1 = y1;
    win->dragY2 = y2;
}

extern "C" void uiWindowDestroy(int32_t windowId) {
    HWND hwnd = (HWND)(intptr_t)windowId;
    UIWindow* win = FindWindowByHWND(hwnd);
    if (!win) return;

    if (win->textFormat) win->textFormat->Release();
    ClearBitmapCache(win);
    if (win->wicFactory) win->wicFactory->Release();
    if (win->dwriteFactory) win->dwriteFactory->Release();
    if (win->backBufferBitmap) win->backBufferBitmap->Release();
    if (win->rt) win->rt->Release();
    if (win->d2dDevice) win->d2dDevice->Release();
    if (win->swapChain) win->swapChain->Release();
    if (win->dcompVisual) win->dcompVisual->Release();
    if (win->dcompTarget) win->dcompTarget->Release();
    if (win->dcompDevice) win->dcompDevice->Release();
    if (win->dxgiFactory) win->dxgiFactory->Release();
    if (win->d3dDevice) win->d3dDevice->Release();
    if (win->d2dFactory) win->d2dFactory->Release();

    DestroyWindow(win->hwnd);

    // Remove from global array
    for (int i = 0; i < g_windowCount; i++) {
        if (g_windows[i] == win) {
            g_windows[i] = g_windows[g_windowCount - 1];
            g_windows[g_windowCount - 1] = NULL;
            g_windowCount--;
            break;
        }
    }
    free(win);
}

// ── Command execution ───────────────────────────────────────────────────

static ID2D1SolidColorBrush* GetBrush(UIWindow* win, D2D1_COLOR_F color) {
    ID2D1SolidColorBrush* brush = NULL;
    win->rt->CreateSolidColorBrush(color, &brush);
    return brush;
}

static void ExecuteCommands(UIWindow* win, const CDrawCommand* cmds, int32_t count) {
    if (!win || !win->rt) {
        MicaLog("ExecuteCommands: SKIP (win=%p rt=%p)", win, win ? win->rt : nullptr);
        return;
    }

    if (g_framesSinceShow < kLogFramesPerShow) {
        MicaLog("ExecuteCommands frame[%d] count=%d swapChain=%p backBufferBitmap=%p "
                "dcompDevice=%p target=%p visual=%p",
                g_frameCounter, count, win->swapChain, win->backBufferBitmap,
                win->dcompDevice, win->dcompTarget, win->dcompVisual);
    }

    win->rt->BeginDraw();
    win->rt->SetTransform(D2D1::IdentityMatrix());
    win->clipDepth = 0;

    // Push default clip to full window
    D2D1_RECT_F fullRect = { 0, 0, (float)win->width, (float)win->height };
    win->clipStack[0] = fullRect;
    win->clipDepth = 1;

    for (int32_t i = 0; i < count; i++) {
        const CDrawCommand* cmd = &cmds[i];

        if (g_framesSinceShow < kLogFramesPerShow) {
            const char* names[] = {"Clear","DrawRect","DrawRoundedRect","DrawText","DrawImage","DrawLine","PushClip","PopClip","SetClipRect"};
            const char* nm = (cmd->cmd_type >= 0 && cmd->cmd_type <= 8) ? names[cmd->cmd_type] : "?";
            MicaLog("  cmd[%d] type=%s(%d) x=%.1f y=%.1f w=%.1f h=%.1f rgba=%.2f,%.2f,%.2f,%.2f",
                    i, nm, cmd->cmd_type, cmd->x, cmd->y, cmd->w, cmd->h,
                    cmd->r, cmd->g, cmd->b, cmd->a);
        }

        switch (cmd->cmd_type) {
        case CmdClear: {
            win->rt->Clear(ToColorF(cmd->r, cmd->g, cmd->b, cmd->a));
            if (g_framesSinceShow < kLogFramesPerShow) {
                MicaLog("frame[%d] CmdClear r=%.3f g=%.3f b=%.3f a=%.3f "
                        "(transparent when a<1 exposes Mica)",
                        g_frameCounter, cmd->r, cmd->g, cmd->b, cmd->a);
            }
            break;
        }

        case CmdDrawRect: {
            ID2D1SolidColorBrush* brush = GetBrush(win, ToColorF(cmd->r, cmd->g, cmd->b, cmd->a));
            if (brush) {
                win->rt->FillRectangle(ToRectF(cmd->x, cmd->y, cmd->w, cmd->h), brush);
                brush->Release();
            }
            break;
        }

        case CmdDrawRoundedRect: {
            ID2D1SolidColorBrush* brush = GetBrush(win, ToColorF(cmd->r, cmd->g, cmd->b, cmd->a));
            if (brush) {
                D2D1_ROUNDED_RECT rr = { ToRectF(cmd->x, cmd->y, cmd->w, cmd->h), cmd->radius, cmd->radius };
                win->rt->FillRoundedRectangle(rr, brush);
                brush->Release();
            }
            break;
        }

        case CmdDrawText: {
            if (!cmd->text || cmd->textLen <= 0) break;

            // Convert UTF-8 to UTF-16
            int wlen = MultiByteToWideChar(CP_UTF8, 0, cmd->text, cmd->textLen, NULL, 0);
            if (wlen <= 0) break;
            wchar_t* wtext = (wchar_t*)malloc((wlen + 1) * sizeof(wchar_t));
            MultiByteToWideChar(CP_UTF8, 0, cmd->text, cmd->textLen, wtext, wlen);
            wtext[wlen] = 0;

            // Create or reuse text format
            float fontSize = cmd->fontSize > 0 ? cmd->fontSize : 16.0f;
            const wchar_t* family = L"Microsoft YaHei"; // default CJK-capable font
            // TODO: use cmd->fontFamily if provided

            IDWriteTextFormat* fmt = NULL;
            win->dwriteFactory->CreateTextFormat(
                family, NULL, DWRITE_FONT_WEIGHT_REGULAR,
                DWRITE_FONT_STYLE_NORMAL, DWRITE_FONT_STRETCH_NORMAL,
                fontSize, L"", &fmt);

            if (fmt) {
                ID2D1SolidColorBrush* brush = GetBrush(win, ToColorF(cmd->r, cmd->g, cmd->b, cmd->a));
                if (brush) {
                    fmt->SetTextAlignment(DWRITE_TEXT_ALIGNMENT_LEADING);
                    fmt->SetParagraphAlignment(DWRITE_PARAGRAPH_ALIGNMENT_CENTER);
                    fmt->SetWordWrapping(DWRITE_WORD_WRAPPING_NO_WRAP);

                    D2D1_RECT_F layoutRect = ToRectF(cmd->x, cmd->y, cmd->w, cmd->h);
                    win->rt->DrawText(wtext, wlen, fmt, layoutRect, brush,
                        D2D1_DRAW_TEXT_OPTIONS_NONE);
                    brush->Release();
                }
                fmt->Release();
            }

            free(wtext);
            break;
        }

        case CmdDrawImage: {
            ID2D1Bitmap* bitmap = FindCachedBitmap(win, cmd->imageKey, cmd->imageKeyLen);
            bool bitmapFromCache = bitmap != NULL;
            bool bitmapStoredInCache = bitmapFromCache;
            if (!bitmap && (!cmd->imageData || cmd->imageLen <= 0 || !win->wicFactory)) break;

            float w = cmd->w;
            float h = cmd->h;

            if (!bitmap) {
                // Decode PNG once per image key while the launcher is visible.
                IWICStream* stream = NULL;
                HRESULT hr = win->wicFactory->CreateStream(&stream);
                if (FAILED(hr) || !stream) break;
                hr = stream->InitializeFromMemory((BYTE*)cmd->imageData, (DWORD)cmd->imageLen);
                if (FAILED(hr)) { stream->Release(); break; }

                IWICBitmapDecoder* decoder = NULL;
                hr = win->wicFactory->CreateDecoderFromStream(stream, NULL,
                    WICDecodeMetadataCacheOnLoad, &decoder);
                if (FAILED(hr) || !decoder) { stream->Release(); break; }

                IWICBitmapFrameDecode* frame = NULL;
                hr = decoder->GetFrame(0, &frame);
                if (FAILED(hr) || !frame) { decoder->Release(); stream->Release(); break; }

                UINT srcW = 0, srcH = 0;
                frame->GetSize(&srcW, &srcH);
                if (w <= 0) w = (float)srcW;
                if (h <= 0) h = (float)srcH;

                // Convert to 32bppPBGRA for Direct2D
                IWICFormatConverter* converter = NULL;
                hr = win->wicFactory->CreateFormatConverter(&converter);
                if (FAILED(hr) || !converter) {
                    frame->Release(); decoder->Release(); stream->Release(); break;
                }
                hr = converter->Initialize(frame, GUID_WICPixelFormat32bppPBGRA,
                    WICBitmapDitherTypeNone, NULL, 0.0, WICBitmapPaletteTypeCustom);
                if (FAILED(hr)) {
                    converter->Release(); frame->Release(); decoder->Release(); stream->Release(); break;
                }

                D2D1_BITMAP_PROPERTIES bmpProps = {};
                bmpProps.pixelFormat.format = DXGI_FORMAT_B8G8R8A8_UNORM;
                bmpProps.pixelFormat.alphaMode = D2D1_ALPHA_MODE_PREMULTIPLIED;
                bmpProps.dpiX = win->dpi;
                bmpProps.dpiY = win->dpi;

                hr = win->rt->CreateBitmapFromWicBitmap(converter, &bmpProps, &bitmap);

                converter->Release();
                frame->Release();
                decoder->Release();
                stream->Release();

                if (FAILED(hr) || !bitmap) break;

                if (cmd->imageKey && cmd->imageKeyLen > 0) {
                    bitmapStoredInCache = CacheBitmap(win, cmd->imageKey, cmd->imageKeyLen, bitmap);
                }
            }

            if (w <= 0 || h <= 0) {
                D2D1_SIZE_F size = bitmap->GetSize();
                w = size.width;
                h = size.height;
            }

            D2D1_RECT_F destRect = ToRectF(cmd->x, cmd->y, w, h);
            // ID2D1DeviceContext::DrawBitmap uses the D2D1_INTERPOLATION_MODE
            // enum (not the legacy D2D1_BITMAP_INTERPOLATION_MODE). HIGH_QUALITY
            // applies high-quality filtering for smoother icon scaling.
            win->rt->DrawBitmap(bitmap, destRect, 1.0f,
                D2D1_INTERPOLATION_MODE_HIGH_QUALITY, NULL);

            if (!bitmapStoredInCache) {
                bitmap->Release();
            }
            break;
        }

        case CmdDrawLine: {
            ID2D1SolidColorBrush* brush = GetBrush(win, ToColorF(cmd->r, cmd->g, cmd->b, cmd->a));
            if (brush) {
                win->rt->DrawLine(
                    D2D1::Point2F(cmd->x, cmd->y),
                    D2D1::Point2F(cmd->w, cmd->h),
                    brush, cmd->strokeWidth > 0 ? cmd->strokeWidth : 1.0f, NULL);
                brush->Release();
            }
            break;
        }

        case CmdPushClip: {
            if (win->clipDepth < 32) {
                D2D1_RECT_F clip = ToRectF(cmd->x, cmd->y, cmd->w, cmd->h);
                win->clipStack[win->clipDepth] = clip;
                win->clipDepth++;
                win->rt->PushAxisAlignedClip(clip, D2D1_ANTIALIAS_MODE_PER_PRIMITIVE);
            }
            break;
        }

        case CmdPopClip: {
            if (win->clipDepth > 1) {
                win->clipDepth--;
                win->rt->PopAxisAlignedClip();
            }
            break;
        }
        }
    }

    // Pop any remaining pushed clips
    while (win->clipDepth > 1) {
        win->clipDepth--;
        win->rt->PopAxisAlignedClip();
    }
    win->clipDepth = 0;

    HRESULT hr = win->rt->EndDraw(NULL, NULL);
    if (g_framesSinceShow < kLogFramesPerShow) {
        MicaLog("frame[%d] EndDraw hr=0x%X", g_frameCounter, (unsigned)hr);
    }
    if (hr == D2DERR_RECREATE_TARGET) {
        // The D2D device is lost (e.g. display mode change). Drop the device-bound
        // back-buffer bitmap so the next frame rebuilds it from the swap chain.
        win->rt->SetTarget(NULL);
        if (win->backBufferBitmap) {
            win->backBufferBitmap->Release();
            win->backBufferBitmap = NULL;
        }
        return;
    }

    // Present the flip-model swap chain and commit the composition visual so
    // the DWM composites the new alpha-bearing frame over the Mica backdrop.
    // Present(0, 0) does not wait for VSync — the DWM compositor handles
    // frame timing. VSync-waiting here (Present(1,0)) blocks the message loop
    // for ~16ms per frame, delaying input processing and worsening judder.
    hr = win->swapChain->Present(0, 0);
    HRESULT commitHr = S_OK;
    if (win->dcompDevice) {
        commitHr = win->dcompDevice->Commit();
    }
    if (g_framesSinceShow < kLogFramesPerShow) {
        MicaLog("frame[%d] Present hr=0x%X Commit hr=0x%X", g_frameCounter,
                (unsigned)hr, (unsigned)commitHr);
    }
    g_frameCounter++;
    g_framesSinceShow++;
}

extern "C" void uiWindowRender(int32_t windowId, const CDrawCommand* commands, int32_t count) {
    HWND hwnd = (HWND)(intptr_t)windowId;
    UIWindow* win = FindWindowByHWND(hwnd);
    if (!win) return;
    ExecuteCommands(win, commands, count);
}

// ── Text measurement ────────────────────────────────────────────────────

extern "C" CMeasureResult uiMeasureText(const char* text, int32_t textLen, float fontSize, const char* fontFamily, int32_t fontFamilyLen) {
    CMeasureResult result = { 0, fontSize * 1.2f };
    if (!text || textLen <= 0) return result;

    // Convert UTF-8 to UTF-16
    int wlen = MultiByteToWideChar(CP_UTF8, 0, text, textLen, NULL, 0);
    if (wlen <= 0) return result;
    wchar_t* wtext = (wchar_t*)malloc((wlen + 1) * sizeof(wchar_t));
    MultiByteToWideChar(CP_UTF8, 0, text, textLen, wtext, wlen);
    wtext[wlen] = 0;

    const wchar_t* family = L"Microsoft YaHei";
    // TODO: use fontFamily if provided

    IDWriteFactory* dwf = NULL;
    if (SUCCEEDED(DWriteCreateFactory(DWRITE_FACTORY_TYPE_SHARED, IID_IDWriteFactory, (IUnknown**)&dwf)) && dwf) {
        IDWriteTextFormat* fmt = NULL;
        float size = fontSize > 0 ? fontSize : 16.0f;
        if (SUCCEEDED(dwf->CreateTextFormat(family, NULL, DWRITE_FONT_WEIGHT_REGULAR,
            DWRITE_FONT_STYLE_NORMAL, DWRITE_FONT_STRETCH_NORMAL, size, L"", &fmt)) && fmt) {

            IDWriteTextLayout* layout = NULL;
            // Use a large maxWidth so text doesn't wrap during measurement.
            if (SUCCEEDED(dwf->CreateTextLayout(wtext, wlen, fmt, 10000.0f, fontSize * 2.0f, &layout)) && layout) {
                layout->SetWordWrapping(DWRITE_WORD_WRAPPING_NO_WRAP);
                DWRITE_TEXT_METRICS metrics;
                layout->GetMetrics(&metrics);
                result.width = metrics.width;
                result.height = metrics.height;
                layout->Release();
            }
            fmt->Release();
        }
        dwf->Release();
    }

    free(wtext);
    return result;
}

// ── Message loop ──────────────────────────────────────────────────────────

// Blocking message pump: waits for messages (GetMessage) instead of busy
// polling (PeekMessage). Returns false on WM_QUIT. After processing messages,
// returns true so the Go side can render one frame if needed.
extern "C" bool uiPumpMessages(void) {
    MSG msg;
    // Block until a message arrives — avoids 100% CPU busy-poll.
    if (GetMessageW(&msg, NULL, 0, 0) <= 0) {
        return false; // WM_QUIT or error
    }
    TranslateMessage(&msg);
    DispatchMessageW(&msg);

    // Drain any remaining queued messages so input stays responsive.
    while (PeekMessageW(&msg, NULL, 0, 0, PM_REMOVE)) {
        if (msg.message == WM_QUIT) {
            return false;
        }
        TranslateMessage(&msg);
        DispatchMessageW(&msg);
    }
    return true;
}

extern "C" void uiInvalidateWindow(int32_t windowId) {
    HWND hwnd = (HWND)(intptr_t)windowId;
    // Post a repaint request instead of calling InvalidateRect. The main thread
    // picks it up in the message loop and triggers a render pass.
    PostMessageW(hwnd, WM_APP_REPAINT, 0, 0);
}
