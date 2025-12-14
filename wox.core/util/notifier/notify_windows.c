#include <windows.h>
#include <windowsx.h>
#include <uxtheme.h>
#include <vssym32.h>
#include <wingdi.h>
#include <time.h>
#include <dwmapi.h>
#include <math.h>
#include <stdlib.h>
#include <string.h>
#include <wchar.h>
#include <stdio.h>
#include <stdarg.h>

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
#define CLOSE_TIMER 1
#define MAX_TEXT_LINES 3
#define TEXT_LEFT_PAD_DIP 20
#define TEXT_VERT_PAD_DIP 12
#define TEXT_RIGHT_GAP_CLOSE_DIP 10
#define WM_WOX_NOTIFICATION_UPDATE (WM_USER + 0x510)

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
    DWORD magic;
    WCHAR messageText[1024];
    WCHAR* renderText;
    BOOL renderTextOwned;
    UINT_PTR closeTimerId;
    BOOL mouseInside;
    BOOL closeHover;
    BOOL closePressed;
    UINT dpi;
    BOOL useFallbackRgn;
    int fallbackRgnRadius;
    UINT paintCount;
    UINT updateCount;
} NotificationWindow;

#define WOX_NOTIFICATION_MAGIC 0x4E584F57u /* 'WOXN' */

typedef BOOL(WINAPI* pfnSetProcessDpiAwarenessContext)(HANDLE);
typedef UINT(WINAPI* pfnGetDpiForSystem)(void);
typedef UINT(WINAPI* pfnGetDpiForWindow)(HWND);

static INIT_ONCE g_initOnce = INIT_ONCE_STATIC_INIT;
static volatile PVOID g_activeHwndAtomic = NULL;
static CRITICAL_SECTION g_logCs;
static WCHAR g_logPath[MAX_PATH];
static const WCHAR* g_notifierPropName = L"WoxNotifierWindow";

static void LogLineA(const char* fmt, ...) {
    SYSTEMTIME st;
    GetLocalTime(&st);

    char msg[2048];
    int off = snprintf(msg, sizeof(msg),
                       "[%04u-%02u-%02u %02u:%02u:%02u.%03u][tid=%lu] ",
                       st.wYear, st.wMonth, st.wDay, st.wHour, st.wMinute, st.wSecond, st.wMilliseconds,
                       (unsigned long)GetCurrentThreadId());
    if (off < 0) off = 0;
    if (off > (int)sizeof(msg) - 1) off = (int)sizeof(msg) - 1;

    va_list ap;
    va_start(ap, fmt);
    int n = vsnprintf(msg + off, sizeof(msg) - (size_t)off, fmt, ap);
    va_end(ap);
    if (n < 0) n = 0;

    size_t len = strnlen(msg, sizeof(msg));
    if (len + 2 < sizeof(msg)) {
        msg[len++] = '\r';
        msg[len++] = '\n';
        msg[len] = '\0';
    }

    if (!TryEnterCriticalSection(&g_logCs)) {
        OutputDebugStringA(msg);
        return;
    }
    FILE* f = NULL;
    _wfopen_s(&f, g_logPath, L"ab");
    if (f) {
        fwrite(msg, 1, len, f);
        fflush(f);
        fclose(f);
    }
    LeaveCriticalSection(&g_logCs);

    OutputDebugStringA(msg);
}

static BOOL CALLBACK InitGlobals(PINIT_ONCE InitOnce, PVOID Parameter, PVOID* Context) {
    (void)InitOnce;
    (void)Parameter;
    (void)Context;
    InitializeCriticalSection(&g_logCs);
    BufferedPaintInit();
    WCHAR tmp[MAX_PATH];
    DWORD got = GetTempPathW((DWORD)(sizeof(tmp) / sizeof(tmp[0])), tmp);
    if (got == 0 || got >= (DWORD)(sizeof(tmp) / sizeof(tmp[0]))) {
        wcscpy_s(tmp, sizeof(tmp) / sizeof(tmp[0]), L".\\");
    }
    swprintf_s(g_logPath, sizeof(g_logPath) / sizeof(g_logPath[0]), L"%sWoxNotifierWindows.log", tmp);
    LogLineA("notifier init, logPath=%ls", g_logPath);
    return TRUE;
}

static void EnsureGlobals(void) {
    InitOnceExecuteOnce(&g_initOnce, InitGlobals, NULL, NULL);
}

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

static WCHAR* DupWString(const WCHAR* s) {
    if (!s) return NULL;
    size_t len = wcslen(s);
    WCHAR* out = (WCHAR*)malloc((len + 1) * sizeof(WCHAR));
    if (!out) return NULL;
    memcpy(out, s, (len + 1) * sizeof(WCHAR));
    return out;
}

static int MeasureTextHeightW(HDC hdc, const WCHAR* text, int width) {
    RECT rc = {0, 0, width, 0};
    DrawTextW(hdc, text, -1, &rc, DT_CALCRECT | DT_WORDBREAK | DT_EDITCONTROL | DT_EXPANDTABS | DT_NOPREFIX);
    int h = rc.bottom - rc.top;
    return h > 0 ? h : 0;
}

static int CountNewlinesW(const WCHAR* s) {
    if (!s) return 0;
    int n = 0;
    for (const WCHAR* p = s; *p; p++) {
        if (*p == L'\n') n++;
    }
    return n;
}

static WCHAR* TruncateToCharBudgetW(const WCHAR* text, int budgetChars, BOOL replaceNewlines) {
    if (!text) return NULL;
    if (budgetChars <= 0) return DupWString(L"\x2026");

    size_t len = wcslen(text);
    if ((int)len <= budgetChars) {
        return DupWString(text);
    }

    WCHAR* buf = (WCHAR*)malloc(((size_t)budgetChars + 2) * sizeof(WCHAR));
    if (!buf) return DupWString(L"\x2026");

    size_t out = 0;
    for (size_t i = 0; i < len && (int)out < budgetChars; i++) {
        WCHAR c = text[i];
        if (c == L'\r') continue;
        if (replaceNewlines && (c == L'\n' || c == L'\t')) c = L' ';
        buf[out++] = c;
    }

    while (out > 0 && (buf[out - 1] == L' ' || buf[out - 1] == L'\n' || buf[out - 1] == L'\t')) {
        out--;
    }
    buf[out++] = L'\x2026';
    buf[out] = L'\0';
    return buf;
}

static BOOL IsHighSurrogate(WCHAR c) {
    return c >= 0xD800 && c <= 0xDBFF;
}

static BOOL IsLowSurrogate(WCHAR c) {
    return c >= 0xDC00 && c <= 0xDFFF;
}

static WCHAR* TruncateMultilineTextToFitW(HDC hdc, const WCHAR* text, int width, int maxHeight) {
    if (!text) return NULL;
    if (maxHeight <= 0) return DupWString(L"\x2026");

    int fullHeight = MeasureTextHeightW(hdc, text, width);
    if (fullHeight <= maxHeight) return DupWString(text);

    size_t len = wcslen(text);
    WCHAR* buf = (WCHAR*)malloc((len + 2) * sizeof(WCHAR));
    if (!buf) return DupWString(L"\x2026");

    size_t lo = 0;
    size_t hi = len;
    size_t best = 0;

    while (lo <= hi) {
        size_t mid = lo + (hi - lo) / 2;
        if (mid < len && mid > 0 && IsHighSurrogate(text[mid - 1]) && IsLowSurrogate(text[mid])) {
            mid--;
        }

        wmemcpy(buf, text, mid);
        while (mid > 0 && (buf[mid - 1] == L' ' || buf[mid - 1] == L'\r' || buf[mid - 1] == L'\n' || buf[mid - 1] == L'\t')) {
            mid--;
        }
        buf[mid] = L'\x2026';
        buf[mid + 1] = L'\0';

        int h = MeasureTextHeightW(hdc, buf, width);
        if (h <= maxHeight) {
            best = mid;
            lo = mid + 1;
        } else {
            if (mid == 0) break;
            hi = mid - 1;
        }
    }

    wmemcpy(buf, text, best);
    while (best > 0 && (buf[best - 1] == L' ' || buf[best - 1] == L'\r' || buf[best - 1] == L'\n' || buf[best - 1] == L'\t')) {
        best--;
    }
    buf[best] = L'\x2026';
    buf[best + 1] = L'\0';

    WCHAR* out = DupWString(buf);
    free(buf);
    return out ? out : DupWString(L"\x2026");
}

static int ComputeWindowHeightAndRenderText(NotificationWindow* nw, int windowWidth, UINT dpi) {
    int topPad = MulDiv(TEXT_VERT_PAD_DIP, (int)dpi, 96);
    int bottomPad = MulDiv(TEXT_VERT_PAD_DIP, (int)dpi, 96);
    int leftPad = MulDiv(TEXT_LEFT_PAD_DIP, (int)dpi, 96);
    RECT closeRect = GetCloseRect(windowWidth, dpi);
    int textRight = closeRect.left - MulDiv(TEXT_RIGHT_GAP_CLOSE_DIP, (int)dpi, 96);
    int textWidth = textRight - leftPad;

    if (nw->renderTextOwned && nw->renderText) {
        free(nw->renderText);
    }
    nw->renderText = nw->messageText;
    nw->renderTextOwned = FALSE;

    int windowHeight = MulDiv(52, (int)dpi, 96);
    if (textWidth <= 0 || !nw->messageFont) return windowHeight;

    HDC hdc = CreateCompatibleDC(NULL);
    if (!hdc) return windowHeight;

    HGDIOBJ old = SelectObject(hdc, nw->messageFont);
    TEXTMETRICW tm;
    if (GetTextMetricsW(hdc, &tm)) {
        int lineHeight = tm.tmHeight > 0 ? tm.tmHeight : MulDiv(18, (int)dpi, 96);
        int maxLines = MAX_TEXT_LINES;
        if (maxLines < 1) maxLines = 1;

        int newlineCount = CountNewlinesW(nw->messageText);
        int estimatedLines = 1;

        if (newlineCount > 0) {
            estimatedLines = newlineCount + 1;
        } else {
            SIZE sz;
            if (GetTextExtentPoint32W(hdc, nw->messageText, (int)wcslen(nw->messageText), &sz) && sz.cx > 0) {
                estimatedLines = (sz.cx + textWidth - 1) / textWidth;
            }
        }
        if (estimatedLines < 1) estimatedLines = 1;
        if (estimatedLines > maxLines) estimatedLines = maxLines;

        int requiredHeight = lineHeight * estimatedLines;

        // If we are at max lines, apply a cheap truncation based on average char width.
        if (estimatedLines == maxLines) {
            int ave = tm.tmAveCharWidth > 0 ? tm.tmAveCharWidth : MulDiv(7, (int)dpi, 96);
            int charsPerLine = textWidth / (ave > 0 ? ave : 1);
            int budget = charsPerLine * maxLines;
            if (budget < 24) budget = 24;
            if (budget > 900) budget = 900;

            size_t msgLen = wcslen(nw->messageText);
            if ((int)msgLen > budget || newlineCount + 1 > maxLines) {
                WCHAR* truncated = TruncateToCharBudgetW(nw->messageText, budget, TRUE);
                if (truncated) {
                    nw->renderText = truncated;
                    nw->renderTextOwned = TRUE;
                }
            }
        }

        windowHeight = topPad + bottomPad + requiredHeight;

        int minHeight = closeRect.bottom + MulDiv(10, (int)dpi, 96);
        if (windowHeight < minHeight) windowHeight = minHeight;
    }

    if (old) SelectObject(hdc, old);
    DeleteDC(hdc);

    return windowHeight;
}

static void ClampWindowToWorkArea(const RECT* workArea, UINT dpi, int* xPos, int* yPos, int windowWidth, int* windowHeight) {
    if (!workArea || !xPos || !yPos || !windowHeight) return;

    int yMargin = MulDiv(60, (int)dpi, 96);
    int minTop = workArea->top + MulDiv(10, (int)dpi, 96);
    int maxBottom = workArea->bottom - yMargin;
    if (maxBottom < minTop) maxBottom = workArea->bottom;

    int maxHeight = maxBottom - minTop;
    if (maxHeight < MulDiv(36, (int)dpi, 96)) {
        maxHeight = maxBottom - workArea->top;
        minTop = workArea->top;
    }

    if (*windowHeight > maxHeight) *windowHeight = maxHeight;
    if (*windowHeight < MulDiv(36, (int)dpi, 96)) *windowHeight = MulDiv(36, (int)dpi, 96);

    if (*xPos < workArea->left) *xPos = workArea->left;
    if (*xPos + windowWidth > workArea->right) *xPos = workArea->right - windowWidth;

    if (*yPos < minTop) *yPos = minTop;
    if (*yPos + *windowHeight > maxBottom) *yPos = maxBottom - *windowHeight;
    if (*yPos < workArea->top) *yPos = workArea->top;
}

static void ApplyWindowLayout(HWND hwnd, NotificationWindow* nw, int windowWidth, UINT dpi, BOOL resetTimer) {
    if (!hwnd || !nw) return;

    int newHeight = ComputeWindowHeightAndRenderText(nw, windowWidth, dpi);

    RECT workArea;
    SystemParametersInfo(SPI_GETWORKAREA, 0, &workArea, 0);
    int workWidth = workArea.right - workArea.left;
    int workHeight = workArea.bottom - workArea.top;

    int xPos = workArea.left + (workWidth - windowWidth) / 2;
    int yPos = workArea.top + workHeight - newHeight - MulDiv(60, (int)dpi, 96);
    ClampWindowToWorkArea(&workArea, dpi, &xPos, &yPos, windowWidth, &newHeight);

    SetWindowPos(hwnd, NULL, xPos, yPos, windowWidth, newHeight,
                 SWP_NOACTIVATE | SWP_NOZORDER | SWP_SHOWWINDOW | SWP_ASYNCWINDOWPOS);

    if (nw->useFallbackRgn) {
        int rr = nw->fallbackRgnRadius > 0 ? nw->fallbackRgnRadius : MulDiv(20, (int)dpi, 96);
        HRGN rgn = CreateRoundRectRgn(0, 0, windowWidth + 1, newHeight + 1, rr * 2, rr * 2);
        if (rgn) {
            SetWindowRgn(hwnd, rgn, TRUE);
        }
    }

    if (resetTimer) {
        KillTimer(hwnd, CLOSE_TIMER);
        nw->closeTimerId = SetTimer(hwnd, CLOSE_TIMER, 3000, NULL);
        ShowWindow(hwnd, SW_SHOWNA);
        RedrawWindow(hwnd, NULL, NULL, RDW_INVALIDATE | RDW_ERASE | RDW_UPDATENOW);
    }

    LogLineA("layout hwnd=%p w=%d h=%d resetTimer=%d", hwnd, windowWidth, newHeight, resetTimer ? 1 : 0);
}

static void PaintBackground(HDC hdc, RECT clientRect, UINT dpi) {
    int width = clientRect.right - clientRect.left;
    int height = clientRect.bottom - clientRect.top;
    if (width <= 0 || height <= 0) return;

    int border = MulDiv(1, (int)dpi, 96);
    if (border < 1) border = 1;
    int rr = MulDiv(18, (int)dpi, 96);

    HRGN rgn = CreateRoundRectRgn(0, 0, width + 1, height + 1, rr * 2, rr * 2);
    if (!rgn) return;

    HBRUSH bg = CreateSolidBrush(RGB(28, 28, 28));
    if (bg) {
        FillRgn(hdc, rgn, bg);
        DeleteObject(bg);
    }

    HBRUSH borderBrush = CreateSolidBrush(RGB(70, 70, 70));
    if (borderBrush) {
        FrameRgn(hdc, rgn, borderBrush, border, border);
        DeleteObject(borderBrush);
    }

    DeleteObject(rgn);
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
    EnsureGlobals();

    // Defensive: clear any stale WM_QUIT on this thread so a new notification's message loop won't exit immediately.
    MSG quitMsg;
    while (PeekMessage(&quitMsg, NULL, WM_QUIT, WM_QUIT, PM_REMOVE)) {
    }

    LogLineA("showNotification enter, msgLen=%zu", message ? strlen(message) : 0);

    HWND active = (HWND)InterlockedCompareExchangePointer((PVOID*)&g_activeHwndAtomic, NULL, NULL);
    LogLineA("active hwnd snapshot=%p", active);

    if (active && IsWindow(active)) {
        WCHAR cls[64];
        cls[0] = L'\0';
        GetClassNameW(active, cls, (int)(sizeof(cls) / sizeof(cls[0])));
        HANDLE prop = GetPropW(active, g_notifierPropName);
        if (prop && wcscmp(cls, L"WoxNotification") == 0) {
            int wlen = MultiByteToWideChar(CP_UTF8, 0, message, -1, NULL, 0);
            if (wlen > 0) {
                WCHAR* wmsg = (WCHAR*)malloc((size_t)wlen * sizeof(WCHAR));
                if (wmsg) {
                    MultiByteToWideChar(CP_UTF8, 0, message, -1, wmsg, wlen);
                    if (PostMessageW(active, WM_WOX_NOTIFICATION_UPDATE, 0, (LPARAM)wmsg)) {
                        LogLineA("update posted to active hwnd=%p", active);
                        return;
                    }
                    LogLineA("PostMessageW failed hwnd=%p err=%lu", active, (unsigned long)GetLastError());
                    free(wmsg);
                }
            }
        } else {
            InterlockedCompareExchangePointer((PVOID*)&g_activeHwndAtomic, NULL, active);
            LogLineA("active hwnd invalidated hwnd=%p class=%ls prop=%p", active, cls, prop);
        }
    }

    TryEnablePerMonitorDpiAwareness();
    UINT dpi = GetSystemDpiSafe();
    LogLineA("dpi=%u", dpi);

    WNDCLASSEXA wc = {0};
    wc.cbSize = sizeof(WNDCLASSEXA);
    wc.lpfnWndProc = NotificationWindowProc;
    wc.hInstance = GetModuleHandle(NULL);
    wc.lpszClassName = "WoxNotification";
    wc.hCursor = LoadCursor(NULL, IDC_ARROW);
    ATOM atom = RegisterClassExA(&wc);
    LogLineA("RegisterClassExA atom=%u err=%lu", (unsigned)atom, (unsigned long)GetLastError());

    RECT workArea;
    BOOL okWorkArea = SystemParametersInfo(SPI_GETWORKAREA, 0, &workArea, 0);
    LogLineA("workArea ok=%d l=%ld t=%ld r=%ld b=%ld err=%lu", okWorkArea ? 1 : 0,
             (long)workArea.left, (long)workArea.top, (long)workArea.right, (long)workArea.bottom,
             (unsigned long)GetLastError());
    int workWidth = workArea.right - workArea.left;
    int workHeight = workArea.bottom - workArea.top;
    int windowWidth = MulDiv(WINDOW_WIDTH, (int)dpi, 96);

    NotificationWindow* nw = (NotificationWindow*)malloc(sizeof(NotificationWindow));
    memset(nw, 0, sizeof(NotificationWindow));
    nw->dpi = dpi;
    nw->magic = WOX_NOTIFICATION_MAGIC;
    LogLineA("alloc nw=%p", nw);

    int fontHeight = -MulDiv(14, (int)nw->dpi, 72);
    nw->messageFont = CreateFontW(fontHeight, 0, 0, 0, FW_NORMAL, FALSE, FALSE, FALSE, DEFAULT_CHARSET,
                                  OUT_DEFAULT_PRECIS, CLIP_DEFAULT_PRECIS, CLEARTYPE_QUALITY,
                                  DEFAULT_PITCH | FF_DONTCARE, L"Microsoft YaHei UI");
    LogLineA("CreateFontW font=%p err=%lu", nw->messageFont, (unsigned long)GetLastError());

    MultiByteToWideChar(CP_UTF8, 0, message, -1, nw->messageText, 1024);
    int windowHeight = MulDiv(52, (int)dpi, 96);

    int xPos = workArea.left + (workWidth - windowWidth) / 2;
    int yPos = workArea.top + workHeight - windowHeight - MulDiv(60, (int)dpi, 96);
    ClampWindowToWorkArea(&workArea, dpi, &xPos, &yPos, windowWidth, &windowHeight);

    LogLineA("CreateWindowExA begin w=%d h=%d x=%d y=%d", windowWidth, windowHeight, xPos, yPos);
    nw->hwnd = CreateWindowExA(
        WS_EX_TOPMOST | WS_EX_TOOLWINDOW | WS_EX_NOACTIVATE | WS_EX_LAYERED,
        "WoxNotification", "",
        WS_POPUP,
        xPos, yPos, windowWidth, windowHeight,
        NULL, NULL, GetModuleHandle(NULL), nw
    );

    if (!nw->hwnd) {
        LogLineA("CreateWindowExA failed err=%lu", (unsigned long)GetLastError());
        if (nw->renderTextOwned && nw->renderText) free(nw->renderText);
        if (nw->messageFont) DeleteObject(nw->messageFont);
        free(nw);
        return;
    }

    InterlockedExchangePointer((PVOID*)&g_activeHwndAtomic, nw->hwnd);
    LogLineA("window created hwnd=%p w=%d h=%d x=%d y=%d dpi=%u", nw->hwnd, windowWidth, windowHeight, xPos, yPos, dpi);
    SetLayeredWindowAttributes(nw->hwnd, 0, 240, LWA_ALPHA);
    nw->useFallbackRgn = TRUE;
    nw->fallbackRgnRadius = MulDiv(18, (int)nw->dpi, 96);

    ApplyWindowLayout(nw->hwnd, nw, windowWidth, dpi, FALSE);

    ShowWindow(nw->hwnd, SW_SHOWNA);
    UpdateWindow(nw->hwnd);

    AnimateWindow(nw->hwnd, 300, AW_BLEND);

    nw->closeTimerId = SetTimer(nw->hwnd, CLOSE_TIMER, 3000, NULL);
    LogLineA("message loop start hwnd=%p timer=%p", nw->hwnd, (void*)nw->closeTimerId);

    MSG msg;
    int gm = 0;
    while ((gm = GetMessage(&msg, NULL, 0, 0)) > 0) {
        TranslateMessage(&msg);
        DispatchMessage(&msg);
    }
    LogLineA("message loop exit hwnd=%p GetMessage=%d err=%lu", nw->hwnd, gm, (unsigned long)GetLastError());

    free(nw);
}

LRESULT CALLBACK NotificationWindowProc(HWND hwnd, UINT uMsg, WPARAM wParam, LPARAM lParam) {
    if (uMsg == WM_NCCREATE) {
        CREATESTRUCT* cs = (CREATESTRUCT*)lParam;
        if (cs && cs->lpCreateParams) {
            SetWindowLongPtr(hwnd, GWLP_USERDATA, (LONG_PTR)cs->lpCreateParams);
            SetPropW(hwnd, g_notifierPropName, (HANDLE)1);
            LogLineA("WM_NCCREATE hwnd=%p", hwnd);
        }
        return DefWindowProc(hwnd, uMsg, wParam, lParam);
    }

    NotificationWindow* nw = (NotificationWindow*)GetWindowLongPtr(hwnd, GWLP_USERDATA);

    switch (uMsg) {
        case WM_WOX_NOTIFICATION_UPDATE: {
            if (nw) nw->updateCount++;
            WCHAR* newText = (WCHAR*)lParam;
            if (newText) {
                if (nw) {
                    wcsncpy_s(nw->messageText, 1024, newText, _TRUNCATE);
                }
                free(newText);
            }
            if (!nw) return 0;
            if (nw->magic != WOX_NOTIFICATION_MAGIC) return 0;

            nw->dpi = GetWindowDpiSafe(hwnd, nw->dpi ? nw->dpi : 96);
            nw->mouseInside = FALSE;
            nw->closeHover = FALSE;
            nw->closePressed = FALSE;

            RECT wr;
            GetWindowRect(hwnd, &wr);
            int windowWidth = wr.right - wr.left;
            ApplyWindowLayout(hwnd, nw, windowWidth, nw->dpi, TRUE);
            LogLineA("UPDATE hwnd=%p count=%u", hwnd, nw->updateCount);
            return 0;
        }

        case WM_ERASEBKGND:
            return 1;

        case WM_PAINT: {
            PAINTSTRUCT ps;
            HDC paintHdc = BeginPaint(hwnd, &ps);

            RECT clientRect;
            GetClientRect(hwnd, &clientRect);
            if (nw) {
                nw->dpi = GetWindowDpiSafe(hwnd, nw->dpi ? nw->dpi : 96);
            }
            int width = clientRect.right - clientRect.left;
            int height = clientRect.bottom - clientRect.top;

            HDC hdc = paintHdc;
            HPAINTBUFFER paintBuf = BeginBufferedPaint(paintHdc, &clientRect, BPBF_TOPDOWNDIB, NULL, &hdc);
            if (paintBuf) {
                BufferedPaintClear(paintBuf, &clientRect);
            }

            if (nw) {
                nw->paintCount++;
                if (nw->paintCount == 1 || (nw->paintCount % 60) == 0) {
                    LogLineA("PAINT hwnd=%p count=%u", hwnd, nw->paintCount);
                }
                PaintBackground(hdc, clientRect, nw->dpi ? nw->dpi : 96);
                RECT closeRect = GetCloseRect(width, nw->dpi ? nw->dpi : 96);
                int leftPad = MulDiv(TEXT_LEFT_PAD_DIP, (int)nw->dpi, 96);
                int topPad = MulDiv(TEXT_VERT_PAD_DIP, (int)nw->dpi, 96);
                int textRight = closeRect.left - MulDiv(TEXT_RIGHT_GAP_CLOSE_DIP, (int)nw->dpi, 96);
                int bottomPad = MulDiv(TEXT_VERT_PAD_DIP, (int)nw->dpi, 96);
                RECT textRect = {leftPad, topPad, textRight, height - bottomPad};

                SetBkMode(hdc, TRANSPARENT);
                if (nw->messageFont) SelectObject(hdc, nw->messageFont);

                const WCHAR* text = nw->renderText ? nw->renderText : nw->messageText;
                SetTextColor(hdc, RGB(255, 255, 255));
                DrawTextW(hdc, text, -1, &textRect,
                          DT_LEFT | DT_TOP | DT_WORDBREAK | DT_EDITCONTROL | DT_EXPANDTABS | DT_NOPREFIX);

                BOOL pressedVisual = (nw->closePressed && nw->closeHover);
                DrawCloseButtonFlat(hdc, closeRect, nw->dpi ? nw->dpi : 96, nw->closeHover, pressedVisual);
            }

            if (paintBuf) {
                EndBufferedPaint(paintBuf, TRUE);
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
            if (nw) {
                if (nw->renderTextOwned && nw->renderText) free(nw->renderText);
                if (nw->messageFont) DeleteObject(nw->messageFont);
            }
            EnsureGlobals();
            InterlockedCompareExchangePointer((PVOID*)&g_activeHwndAtomic, NULL, hwnd);
            RemovePropW(hwnd, g_notifierPropName);
            LogLineA("WM_DESTROY hwnd=%p", hwnd);
            PostQuitMessage(0);
            return 0;
        }

        case WM_NCDESTROY: {
            RemovePropW(hwnd, g_notifierPropName);
            return DefWindowProc(hwnd, uMsg, wParam, lParam);
        }
    }

    return DefWindowProc(hwnd, uMsg, wParam, lParam);
}
