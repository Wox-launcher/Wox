#include <windows.h>
#include <windowsx.h>
#include <uxtheme.h>
#include <vssym32.h>
#include <wingdi.h>
#include <time.h>
#include <dwmapi.h>
#include <math.h>

#pragma comment(lib, "dwmapi.lib")
#pragma comment(lib, "uxtheme.lib")
#pragma comment(lib, "msimg32.lib")

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

typedef BOOL(WINAPI* pfnSetWindowCompositionAttribute)(HWND, WINDOWCOMPOSITIONATTRIBDATA*);

static BOOL TryEnableAccent(HWND hwnd, ACCENT_STATE state, DWORD gradientColor, DWORD accentFlags) {
    HMODULE user32 = GetModuleHandleW(L"user32.dll");
    if (!user32) return FALSE;
    pfnSetWindowCompositionAttribute fn = (pfnSetWindowCompositionAttribute)GetProcAddress(user32, "SetWindowCompositionAttribute");
    if (!fn) return FALSE;

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

static BOOL TryEnableHostBackdrop(HWND hwnd) {
    return TryEnableAccent(hwnd, ACCENT_ENABLE_HOSTBACKDROP, 0x70202020, 0);
}

static BOOL TryEnableAcrylic(HWND hwnd) {
    return TryEnableAccent(hwnd, ACCENT_ENABLE_ACRYLICBLURBEHIND, 0x2A202020, 2);
}

#define WINDOW_WIDTH 380
#define WINDOW_HEIGHT 80
#define CLOSE_TIMER 1

#ifndef DWMWA_USE_IMMERSIVE_DARK_MODE
#define DWMWA_USE_IMMERSIVE_DARK_MODE 20
#endif

#ifndef DWMWA_WINDOW_CORNER_PREFERENCE
#define DWMWA_WINDOW_CORNER_PREFERENCE 33
typedef enum {
    DWMWCP_DEFAULT = 0,
    DWMWCP_DONOTROUND = 1,
    DWMWCP_ROUND = 2,
    DWMWCP_ROUNDSMALL = 3
} DWM_WINDOW_CORNER_PREFERENCE;
#endif

#ifndef DWMWA_SYSTEMBACKDROP_TYPE
#define DWMWA_SYSTEMBACKDROP_TYPE 38
typedef enum {
    DWMSBT_AUTO = 0,
    DWMSBT_NONE = 1,
    DWMSBT_MAINWINDOW = 2,
    DWMSBT_TRANSIENTWINDOW = 3,
    DWMSBT_TABBEDWINDOW = 4
} DWM_SYSTEMBACKDROP_TYPE;
#endif

typedef struct {
    HWND hwnd;
    HFONT messageFont;
    WCHAR messageText[1024];
    UINT_PTR closeTimerId;
    BOOL mouseInside;
    BOOL closeHover;
    BOOL closePressed;
    UINT dpi;
} NotificationWindow;

typedef BOOL(WINAPI* pfnSetProcessDpiAwarenessContext)(HANDLE);
typedef UINT(WINAPI* pfnGetDpiForSystem)(void);
typedef UINT(WINAPI* pfnGetDpiForWindow)(HWND);

static UINT GetSystemDpiSafe(void) {
    HMODULE user32 = GetModuleHandleW(L"user32.dll");
    if (!user32) return 96;
    pfnGetDpiForSystem fn = (pfnGetDpiForSystem)GetProcAddress(user32, "GetDpiForSystem");
    if (!fn) return 96;
    UINT dpi = fn();
    return dpi ? dpi : 96;
}

static UINT GetWindowDpiSafe(HWND hwnd, UINT fallback) {
    HMODULE user32 = GetModuleHandleW(L"user32.dll");
    if (!user32) return fallback;
    pfnGetDpiForWindow fn = (pfnGetDpiForWindow)GetProcAddress(user32, "GetDpiForWindow");
    if (!fn) return fallback;
    UINT dpi = fn(hwnd);
    return dpi ? dpi : fallback;
}

static void TryEnablePerMonitorDpiAwareness(void) {
    HMODULE user32 = GetModuleHandleW(L"user32.dll");
    if (!user32) return;
    pfnSetProcessDpiAwarenessContext fn = (pfnSetProcessDpiAwarenessContext)GetProcAddress(user32, "SetProcessDpiAwarenessContext");
    if (!fn) return;
    fn((HANDLE)-4); // DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2
}

static RECT GetCloseRect(int width, UINT dpi) {
    int pad = MulDiv(10, (int)dpi, 96);
    int size = MulDiv(24, (int)dpi, 96);
    RECT r = {width - pad - size, pad, width - pad, pad + size};
    return r;
}

static HBITMAP Create32BitDIBSection(HDC hdc, int width, int height, void** outBits) {
    BITMAPINFO bi;
    ZeroMemory(&bi, sizeof(bi));
    bi.bmiHeader.biSize = sizeof(BITMAPINFOHEADER);
    bi.bmiHeader.biWidth = width;
    bi.bmiHeader.biHeight = -height;
    bi.bmiHeader.biPlanes = 1;
    bi.bmiHeader.biBitCount = 32;
    bi.bmiHeader.biCompression = BI_RGB;
    return CreateDIBSection(hdc, &bi, DIB_RGB_COLORS, outBits, NULL, 0);
}

static void ClearARGB(void* bits, int width, int height) {
    if (!bits) return;
    ZeroMemory(bits, (size_t)width * (size_t)height * 4);
}

static inline unsigned char ClampByte(int v) {
    if (v < 0) return 0;
    if (v > 255) return 255;
    return (unsigned char)v;
}

static void BlendPremulBGRA(UINT32* px, unsigned char srcB, unsigned char srcG, unsigned char srcR, unsigned char srcA) {
    unsigned char dstB = (unsigned char)((*px) & 0xFF);
    unsigned char dstG = (unsigned char)((*px >> 8) & 0xFF);
    unsigned char dstR = (unsigned char)((*px >> 16) & 0xFF);
    unsigned char dstA = (unsigned char)((*px >> 24) & 0xFF);

    int invA = 255 - (int)srcA;
    unsigned char outA = (unsigned char)(srcA + (dstA * invA + 127) / 255);
    unsigned char outB = (unsigned char)(srcB + (dstB * invA + 127) / 255);
    unsigned char outG = (unsigned char)(srcG + (dstG * invA + 127) / 255);
    unsigned char outR = (unsigned char)(srcR + (dstR * invA + 127) / 255);

    *px = ((UINT32)outA << 24) | ((UINT32)outR << 16) | ((UINT32)outG << 8) | (UINT32)outB;
}

static void FillRoundRectPremul(UINT32* pixels, int width, int height, RECT r, int radius, unsigned char a, unsigned char r8, unsigned char g8, unsigned char b8) {
    if (!pixels) return;
    if (radius < 0) radius = 0;

    unsigned char pr = (unsigned char)((r8 * a + 127) / 255);
    unsigned char pg = (unsigned char)((g8 * a + 127) / 255);
    unsigned char pb = (unsigned char)((b8 * a + 127) / 255);

    int left = r.left < 0 ? 0 : r.left;
    int top = r.top < 0 ? 0 : r.top;
    int right = r.right > width ? width : r.right;
    int bottom = r.bottom > height ? height : r.bottom;

    int rad = radius;
    int radiusSquared = rad * rad;

    for (int y = top; y < bottom; y++) {
        for (int x = left; x < right; x++) {
            int dx = 0;
            int dy = 0;

            if (x < r.left + rad) dx = (r.left + rad) - x;
            else if (x >= r.right - rad) dx = x - (r.right - rad - 1);

            if (y < r.top + rad) dy = (r.top + rad) - y;
            else if (y >= r.bottom - rad) dy = y - (r.bottom - rad - 1);

            if (dx == 0 || dy == 0) {
                BlendPremulBGRA(&pixels[y * width + x], pb, pg, pr, a);
            } else {
                if (dx * dx + dy * dy <= radiusSquared) {
                    BlendPremulBGRA(&pixels[y * width + x], pb, pg, pr, a);
                }
            }
        }
    }
}

static float DistPointToSegment(float px, float py, float ax, float ay, float bx, float by) {
    float vx = bx - ax;
    float vy = by - ay;
    float wx = px - ax;
    float wy = py - ay;

    float c1 = wx * vx + wy * vy;
    if (c1 <= 0.0f) {
        float dx = px - ax;
        float dy = py - ay;
        return (float)sqrt(dx * dx + dy * dy);
    }
    float c2 = vx * vx + vy * vy;
    if (c2 <= c1) {
        float dx = px - bx;
        float dy = py - by;
        return (float)sqrt(dx * dx + dy * dy);
    }
    float t = c1 / c2;
    float projx = ax + t * vx;
    float projy = ay + t * vy;
    float dx = px - projx;
    float dy = py - projy;
    return (float)sqrt(dx * dx + dy * dy);
}

static void DrawAALinePremul(UINT32* pixels, int width, int height,
                             float ax, float ay, float bx, float by,
                             float thickness,
                             unsigned char a, unsigned char r8, unsigned char g8, unsigned char b8) {
    if (!pixels) return;
    float half = thickness * 0.5f;
    float feather = 1.0f;

    int minx = (int)floorf(fminf(ax, bx) - half - feather - 1);
    int maxx = (int)ceilf(fmaxf(ax, bx) + half + feather + 1);
    int miny = (int)floorf(fminf(ay, by) - half - feather - 1);
    int maxy = (int)ceilf(fmaxf(ay, by) + half + feather + 1);
    if (minx < 0) minx = 0;
    if (miny < 0) miny = 0;
    if (maxx > width - 1) maxx = width - 1;
    if (maxy > height - 1) maxy = height - 1;

    for (int y = miny; y <= maxy; y++) {
        for (int x = minx; x <= maxx; x++) {
            float cx = x + 0.5f;
            float cy = y + 0.5f;
            float d = DistPointToSegment(cx, cy, ax, ay, bx, by);
            float edge0 = half;
            float edge1 = half + feather;
            if (d >= edge1) continue;

            float t = 1.0f;
            if (d > edge0) {
                t = 1.0f - (d - edge0) / (edge1 - edge0);
            }

            int srcA = (int)roundf(a * t);
            if (srcA <= 0) continue;

            unsigned char sa = ClampByte(srcA);
            unsigned char pr = (unsigned char)((r8 * sa + 127) / 255);
            unsigned char pg = (unsigned char)((g8 * sa + 127) / 255);
            unsigned char pb = (unsigned char)((b8 * sa + 127) / 255);

            BlendPremulBGRA(&pixels[y * width + x], pb, pg, pr, sa);
        }
    }
}

static void DrawCloseButtonFlat(HDC targetHdc, RECT closeRect, UINT dpi, BOOL hover, BOOL pressed) {
    int w = closeRect.right - closeRect.left;
    int h = closeRect.bottom - closeRect.top;
    if (w <= 0 || h <= 0) return;

    HDC memDC = CreateCompatibleDC(targetHdc);
    if (!memDC) return;

    void* bits = NULL;
    HBITMAP dib = Create32BitDIBSection(targetHdc, w, h, &bits);
    if (!dib || !bits) {
        if (dib) DeleteObject(dib);
        DeleteDC(memDC);
        return;
    }

    HBITMAP oldBmp = (HBITMAP)SelectObject(memDC, dib);
    ClearARGB(bits, w, h);

    UINT32* pixels = (UINT32*)bits;

    int radius = MulDiv(6, (int)dpi, 96);
    if (hover || pressed) {
        unsigned char bgA = (unsigned char)(pressed ? 80 : 48);
        RECT r = {0, 0, w, h};
        FillRoundRectPremul(pixels, w, h, r, radius, bgA, 255, 255, 255);
    }

    float pad = (float)MulDiv(7, (int)dpi, 96);
    float x0 = pad;
    float y0 = pad;
    float x1 = (float)w - pad;
    float y1 = (float)h - pad;
    float thickness = (float)MulDiv(2, (int)dpi, 96);
    if (thickness < 1.6f) thickness = 1.6f;
    unsigned char alpha = (unsigned char)(hover ? 255 : 220);
    DrawAALinePremul(pixels, w, h, x0, y0, x1, y1, thickness, alpha, 255, 255, 255);
    DrawAALinePremul(pixels, w, h, x1, y0, x0, y1, thickness, alpha, 255, 255, 255);

    BLENDFUNCTION bf;
    bf.BlendOp = AC_SRC_OVER;
    bf.BlendFlags = 0;
    bf.SourceConstantAlpha = 255;
    bf.AlphaFormat = AC_SRC_ALPHA;

    AlphaBlend(targetHdc, closeRect.left, closeRect.top, w, h, memDC, 0, 0, w, h, bf);

    if (oldBmp) SelectObject(memDC, oldBmp);
    DeleteObject(dib);
    DeleteDC(memDC);
}

LRESULT CALLBACK NotificationWindowProc(HWND hwnd, UINT uMsg, WPARAM wParam, LPARAM lParam);

void showNotification(const char* message) {
    TryEnablePerMonitorDpiAwareness();
    UINT dpi = GetSystemDpiSafe();

    WNDCLASSEXA wc = {0};
    wc.cbSize = sizeof(WNDCLASSEXA);
    wc.lpfnWndProc = NotificationWindowProc;
    wc.hInstance = GetModuleHandle(NULL);
    wc.lpszClassName = "WoxNotification";
    wc.hCursor = LoadCursor(NULL, IDC_ARROW);
    RegisterClassExA(&wc);

    RECT workArea;
    SystemParametersInfo(SPI_GETWORKAREA, 0, &workArea, 0);
    int workWidth = workArea.right - workArea.left;
    int workHeight = workArea.bottom - workArea.top;
    int windowWidth = MulDiv(WINDOW_WIDTH, (int)dpi, 96);
    int windowHeight = MulDiv(WINDOW_HEIGHT, (int)dpi, 96);
    int yMargin = MulDiv(60, (int)dpi, 96);
    int xPos = workArea.left + (workWidth - windowWidth) / 2;
    int yPos = workArea.top + workHeight - windowHeight - yMargin;

    NotificationWindow* nw = (NotificationWindow*)malloc(sizeof(NotificationWindow));
    memset(nw, 0, sizeof(NotificationWindow));
    nw->dpi = dpi;

    nw->hwnd = CreateWindowExA(
        WS_EX_TOPMOST | WS_EX_TOOLWINDOW | WS_EX_NOACTIVATE,
        "WoxNotification", "",
        WS_POPUP,
        xPos, yPos, windowWidth, windowHeight,
        NULL, NULL, GetModuleHandle(NULL), NULL
    );

    SetWindowLongPtr(nw->hwnd, GWLP_USERDATA, (LONG_PTR)nw);

    int fontHeight = -MulDiv(14, (int)nw->dpi, 72);
    nw->messageFont = CreateFontW(fontHeight, 0, 0, 0, FW_NORMAL, FALSE, FALSE, FALSE, DEFAULT_CHARSET,
                                  OUT_DEFAULT_PRECIS, CLIP_DEFAULT_PRECIS, CLEARTYPE_QUALITY,
                                  DEFAULT_PITCH | FF_DONTCARE, L"Microsoft YaHei UI");

    MultiByteToWideChar(CP_UTF8, 0, message, -1, nw->messageText, 1024);

    {
        BOOL dark = TRUE;
        DwmSetWindowAttribute(nw->hwnd, DWMWA_USE_IMMERSIVE_DARK_MODE, &dark, sizeof(dark));

        DWM_WINDOW_CORNER_PREFERENCE corner = DWMWCP_ROUND;
        HRESULT hrCorner = DwmSetWindowAttribute(nw->hwnd, DWMWA_WINDOW_CORNER_PREFERENCE, &corner, sizeof(corner));

        BOOL accentOk = TryEnableAcrylic(nw->hwnd);
        if (!accentOk) accentOk = TryEnableHostBackdrop(nw->hwnd);

        if (accentOk) {
            MARGINS margins = {0, 0, 0, 0};
            DwmExtendFrameIntoClientArea(nw->hwnd, &margins);

            DWM_SYSTEMBACKDROP_TYPE noneBackdrop = DWMSBT_NONE;
            DwmSetWindowAttribute(nw->hwnd, DWMWA_SYSTEMBACKDROP_TYPE, &noneBackdrop, sizeof(noneBackdrop));
        } else {
            DWM_SYSTEMBACKDROP_TYPE backdrop = DWMSBT_TRANSIENTWINDOW;
            HRESULT hrBackdrop = DwmSetWindowAttribute(nw->hwnd, DWMWA_SYSTEMBACKDROP_TYPE, &backdrop, sizeof(backdrop));
            if (SUCCEEDED(hrBackdrop)) {
                MARGINS margins = {-1};
                DwmExtendFrameIntoClientArea(nw->hwnd, &margins);
            }
        }

        if (FAILED(hrCorner)) {
            int rr = MulDiv(20, (int)nw->dpi, 96);
            HRGN rgn = CreateRoundRectRgn(0, 0, windowWidth + 1, windowHeight + 1, rr * 2, rr * 2);
            if (rgn) {
                SetWindowRgn(nw->hwnd, rgn, TRUE);
            }
        }
    }

    ShowWindow(nw->hwnd, SW_SHOWNA);
    UpdateWindow(nw->hwnd);

    AnimateWindow(nw->hwnd, 300, AW_BLEND);

    nw->closeTimerId = SetTimer(nw->hwnd, CLOSE_TIMER, 3000, NULL);

    MSG msg;
    while (GetMessage(&msg, NULL, 0, 0)) {
        TranslateMessage(&msg);
        DispatchMessage(&msg);
    }

    free(nw);
}

LRESULT CALLBACK NotificationWindowProc(HWND hwnd, UINT uMsg, WPARAM wParam, LPARAM lParam) {
    NotificationWindow* nw = (NotificationWindow*)GetWindowLongPtr(hwnd, GWLP_USERDATA);

    switch (uMsg) {
        case WM_ERASEBKGND:
            return 1;

        case WM_PAINT: {
            PAINTSTRUCT ps;
            HDC hdc = BeginPaint(hwnd, &ps);

            RECT clientRect;
            GetClientRect(hwnd, &clientRect);
            if (nw) {
                nw->dpi = GetWindowDpiSafe(hwnd, nw->dpi ? nw->dpi : 96);
            }
            int width = clientRect.right - clientRect.left;
            int height = clientRect.bottom - clientRect.top;

            if (nw) {
                RECT closeRect = GetCloseRect(width, nw->dpi ? nw->dpi : 96);
                int leftPad = MulDiv(20, (int)nw->dpi, 96);
                int topPad = MulDiv(18, (int)nw->dpi, 96);
                int textRight = closeRect.left - MulDiv(10, (int)nw->dpi, 96);
                int bottomPad = MulDiv(18, (int)nw->dpi, 96);
                RECT textRect = {leftPad, topPad, textRight, height - bottomPad};

                SetBkMode(hdc, TRANSPARENT);
                if (nw->messageFont) SelectObject(hdc, nw->messageFont);

                HTHEME theme = OpenThemeData(hwnd, L"WINDOW");
                if (theme) {
                    DTTOPTS opts;
                    ZeroMemory(&opts, sizeof(opts));
                    opts.dwSize = sizeof(opts);
                    opts.dwFlags = DTT_TEXTCOLOR | DTT_COMPOSITED;
                    opts.crText = RGB(255, 255, 255);
                    DrawThemeTextEx(theme, hdc, 0, 0, nw->messageText, -1,
                                    DT_LEFT | DT_VCENTER | DT_SINGLELINE | DT_END_ELLIPSIS,
                                    &textRect, &opts);
                    CloseThemeData(theme);
                } else {
                    SetTextColor(hdc, RGB(255, 255, 255));
                    DrawTextW(hdc, nw->messageText, -1, &textRect,
                              DT_LEFT | DT_VCENTER | DT_SINGLELINE | DT_END_ELLIPSIS);
                }

                BOOL pressedVisual = (nw->closePressed && nw->closeHover);
                DrawCloseButtonFlat(hdc, closeRect, nw->dpi ? nw->dpi : 96, nw->closeHover, pressedVisual);
            }

            EndPaint(hwnd, &ps);
            return 0;
        }

        case WM_SETCURSOR: {
            if (!nw) break;
            if (LOWORD(lParam) == HTCLIENT) {
                POINT pt;
                if (GetCursorPos(&pt)) {
                    ScreenToClient(hwnd, &pt);
                    RECT clientRect;
                    GetClientRect(hwnd, &clientRect);
                    RECT closeRect = GetCloseRect(clientRect.right - clientRect.left, nw->dpi ? nw->dpi : 96);
                    if (PtInRect(&closeRect, pt)) {
                        SetCursor(LoadCursor(NULL, IDC_HAND));
                    } else {
                        SetCursor(LoadCursor(NULL, IDC_ARROW));
                    }
                    return TRUE;
                }
            }
            break;
        }

        case WM_MOUSEMOVE: {
            if (!nw) return 0;

            if (!nw->mouseInside) {
                nw->mouseInside = TRUE;
                TRACKMOUSEEVENT tme = {sizeof(TRACKMOUSEEVENT), TME_LEAVE, hwnd, 0};
                TrackMouseEvent(&tme);
            }

            POINT pt = {GET_X_LPARAM(lParam), GET_Y_LPARAM(lParam)};
            RECT clientRect;
            GetClientRect(hwnd, &clientRect);
            RECT closeRect = GetCloseRect(clientRect.right - clientRect.left, nw->dpi ? nw->dpi : 96);
            BOOL hoverNow = PtInRect(&closeRect, pt);
            if (hoverNow != nw->closeHover) {
                nw->closeHover = hoverNow;
                InvalidateRect(hwnd, NULL, FALSE);
            }
            return 0;
        }

        case WM_MOUSELEAVE: {
            if (!nw) return 0;
            nw->mouseInside = FALSE;
            nw->closeHover = FALSE;
            nw->closePressed = FALSE;
            InvalidateRect(hwnd, NULL, FALSE);
            return 0;
        }

        case WM_LBUTTONDOWN: {
            if (!nw) return 0;
            POINT pt = {GET_X_LPARAM(lParam), GET_Y_LPARAM(lParam)};
            RECT clientRect;
            GetClientRect(hwnd, &clientRect);
            RECT closeRect = GetCloseRect(clientRect.right - clientRect.left, nw->dpi ? nw->dpi : 96);
            if (PtInRect(&closeRect, pt)) {
                nw->closePressed = TRUE;
                SetCapture(hwnd);
                InvalidateRect(hwnd, NULL, FALSE);
            }
            return 0;
        }

        case WM_LBUTTONUP: {
            if (!nw) return 0;
            POINT pt = {GET_X_LPARAM(lParam), GET_Y_LPARAM(lParam)};
            RECT clientRect;
            GetClientRect(hwnd, &clientRect);
            RECT closeRect = GetCloseRect(clientRect.right - clientRect.left, nw->dpi ? nw->dpi : 96);
            BOOL pressed = nw->closePressed;
            nw->closePressed = FALSE;
            if (GetCapture() == hwnd) ReleaseCapture();
            InvalidateRect(hwnd, NULL, FALSE);
            if (pressed && PtInRect(&closeRect, pt)) {
                DestroyWindow(hwnd);
            }
            return 0;
        }

        case WM_CAPTURECHANGED: {
            if (!nw) return 0;
            if (nw->closePressed) {
                nw->closePressed = FALSE;
                InvalidateRect(hwnd, NULL, FALSE);
            }
            return 0;
        }

        case WM_TIMER: {
            if (wParam == CLOSE_TIMER && (!nw || !nw->mouseInside)) {
                KillTimer(hwnd, CLOSE_TIMER);
                AnimateWindow(hwnd, 300, AW_BLEND | AW_HIDE);
                DestroyWindow(hwnd);
            }
            return 0;
        }

        case WM_DESTROY: {
            if (nw && nw->messageFont) {
                DeleteObject(nw->messageFont);
            }
            PostQuitMessage(0);
            return 0;
        }
    }

    return DefWindowProc(hwnd, uMsg, wParam, lParam);
}
