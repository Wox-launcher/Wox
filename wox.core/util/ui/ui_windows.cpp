//go:build windows && cgo

#define WIN32_LEAN_AND_MEAN
#define INITGUID
#include <windows.h>
#include <windowsx.h>
#include <dwmapi.h>
#include <d2d1.h>
#include <dwrite.h>
#include <wincodec.h>
#include <objbase.h>
#include <initguid.h>
#include <stdint.h>
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

// ── Types matching the Go CGO declarations ──────────────────────────────

typedef struct {
    int32_t cmd_type;
    float x, y, w, h;
    float r, g, b, a;
    float radius;
    float strokeWidth;
    const char* text;
    int32_t textLen;
    float fontSize;
    const char* fontFamily;
    int32_t fontFamilyLen;
    const uint8_t* imageData;
    int32_t imageLen;
    float imageWidth, imageHeight;
} CDrawCommand;

typedef struct {
    int32_t width;
    int32_t height;
    float cornerRadius;
    bool frameless;
    bool transparent;
} CWindowConfig;

typedef struct {
    float width;
    float height;
} CMeasureResult;

// Command types (must match Go CommandType constants)
enum {
    CmdClear = 0,
    CmdDrawRect = 1,
    CmdDrawRoundedRect = 2,
    CmdDrawText = 3,
    CmdDrawImage = 4,
    CmdDrawLine = 5,
    CmdPushClip = 6,
    CmdPopClip = 7,
    CmdSetClipRect = 8,
};

// Event types (must match Go EventType constants)
enum {
    EventKeyPress = 0,
    EventKeyRelease = 1,
    EventTextInput = 2,
    EventIMECompose = 3,
    EventClick = 4,
    EventScroll = 5,
    EventFocusLost = 6,
    EventResize = 7,
};

// ── Forward declarations ────────────────────────────────────────────────

// uiEventCallback is implemented in Go (via //export) with C linkage.
// In C++ we must declare it extern "C" to avoid name mangling.
extern "C" void uiEventCallback(int32_t windowId, int32_t eventType, int32_t key, int32_t mods,
    char* text, int32_t textLen,
    char* composeText, int32_t composeTextLen, int32_t composeCursor,
    float x, float y, float deltaY,
    int32_t width, int32_t height);

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

typedef struct {
    HWND hwnd;
    ID2D1Factory* d2dFactory;
    ID2D1HwndRenderTarget* rt;
    IDWriteFactory* dwriteFactory;
    IDWriteTextFormat* textFormat;
    IWICImagingFactory* wicFactory;

    float dpi;
    float scale;          // dpi / 96
    int32_t width;        // logical (DIP)
    int32_t height;
    float cornerRadius;
    bool transparent;
    bool visible;

    // Clip stack for PushClip/PopClip
    D2D1_RECT_F clipStack[32];
    int clipDepth;
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
        if (win && win->rt) {
            int w = LOWORD(lParam);
            int h = HIWORD(lParam);
            if (w > 0 && h > 0) {
                D2D1_SIZE_U size = { (UINT)w, (UINT)h };
                win->rt->Resize(&size);
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
                case VK_ESCAPE: key = 1; break;
                case VK_RETURN: key = 2; break;
                case VK_BACK:   key = 3; break;
                case VK_TAB:    key = 4; break;
                case VK_SPACE:  key = 5; break;
                case VK_UP:     key = 6; break;
                case VK_DOWN:   key = 7; break;
                case VK_LEFT:   key = 8; break;
                case VK_RIGHT:  key = 9; break;
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

    case WM_NCHITTEST: {
        // Frameless window: make entire client area draggable from top
        if (win) {
            POINT pt = { GET_X_LPARAM(lParam), GET_Y_LPARAM(lParam) };
            ScreenToClient(hwnd, &pt);
            if (pt.y < 8) {
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
    EnablePerMonitorDPI();
    RegisterWindowClass();

    UIWindow* win = (UIWindow*)calloc(1, sizeof(UIWindow));
    if (!win) return 0;

    win->width = config.width;
    win->height = config.height;
    win->cornerRadius = config.cornerRadius;
    win->transparent = config.transparent;
    win->visible = false;
    win->clipDepth = 0;

    // Create frameless window (no WS_EX_LAYERED — Direct2D paints directly)
    DWORD exStyle = WS_EX_TOOLWINDOW | WS_EX_TOPMOST;
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
        free(win);
        return 0;
    }

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

    // Initialize Direct2D
    D2D1_FACTORY_OPTIONS opts = { D2D1_DEBUG_LEVEL_NONE };
    HRESULT hr = D2D1CreateFactory(D2D1_FACTORY_TYPE_SINGLE_THREADED,
        opts, &win->d2dFactory);
    if (FAILED(hr)) {
        DestroyWindow(win->hwnd);
        free(win);
        return 0;
    }

    // Create render target
    D2D1_RENDER_TARGET_PROPERTIES rtProps = {};
    rtProps.type = D2D1_RENDER_TARGET_TYPE_DEFAULT;
    rtProps.pixelFormat.format = DXGI_FORMAT_B8G8R8A8_UNORM;
    rtProps.pixelFormat.alphaMode = D2D1_ALPHA_MODE_PREMULTIPLIED;
    rtProps.dpiX = win->dpi;
    rtProps.dpiY = win->dpi;
    rtProps.usage = D2D1_RENDER_TARGET_USAGE_NONE;
    rtProps.minLevel = D2D1_FEATURE_LEVEL_DEFAULT;

    D2D1_HWND_RENDER_TARGET_PROPERTIES hwndProps = {};
    hwndProps.hwnd = win->hwnd;
    hwndProps.pixelSize.width = physW;
    hwndProps.pixelSize.height = physH;
    hwndProps.presentOptions = D2D1_PRESENT_OPTIONS_NONE;

    hr = win->d2dFactory->CreateHwndRenderTarget(&rtProps, &hwndProps, &win->rt);
    if (FAILED(hr)) {
        win->d2dFactory->Release();
        DestroyWindow(win->hwnd);
        free(win);
        return 0;
    }

    // Initialize DirectWrite
    hr = DWriteCreateFactory(DWRITE_FACTORY_TYPE_SHARED, IID_IDWriteFactory,
        (IUnknown**)&win->dwriteFactory);
    if (FAILED(hr)) {
        win->rt->Release();
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
        win->rt->Release();
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
    UIWindow* win = FindWindowByHWND(hwnd);
    if (!win) return;
    ShowWindow(hwnd, SW_SHOW);
    win->visible = true;
}

extern "C" void uiWindowHide(int32_t windowId) {
    HWND hwnd = (HWND)(intptr_t)windowId;
    UIWindow* win = FindWindowByHWND(hwnd);
    if (!win) return;
    ShowWindow(hwnd, SW_HIDE);
    win->visible = false;
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
    SetWindowPos(hwnd, NULL, 0, 0,
        (int)(w * win->scale), (int)(h * win->scale),
        SWP_NOMOVE | SWP_NOZORDER);
}

extern "C" bool uiWindowIsVisible(int32_t windowId) {
    HWND hwnd = (HWND)(intptr_t)windowId;
    UIWindow* win = FindWindowByHWND(hwnd);
    if (!win) return false;
    return win->visible;
}

extern "C" void uiWindowDestroy(int32_t windowId) {
    HWND hwnd = (HWND)(intptr_t)windowId;
    UIWindow* win = FindWindowByHWND(hwnd);
    if (!win) return;

    if (win->textFormat) win->textFormat->Release();
    if (win->wicFactory) win->wicFactory->Release();
    if (win->dwriteFactory) win->dwriteFactory->Release();
    if (win->rt) win->rt->Release();
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
    if (!win || !win->rt) return;

    win->rt->BeginDraw();
    win->rt->SetTransform(D2D1::IdentityMatrix());
    win->clipDepth = 0;

    // Push default clip to full window
    D2D1_RECT_F fullRect = { 0, 0, (float)win->width, (float)win->height };
    win->clipStack[0] = fullRect;
    win->clipDepth = 1;

    for (int32_t i = 0; i < count; i++) {
        const CDrawCommand* cmd = &cmds[i];

        switch (cmd->cmd_type) {
        case CmdClear: {
            win->rt->Clear(ToColorF(cmd->r, cmd->g, cmd->b, cmd->a));
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
            if (!cmd->imageData || cmd->imageLen <= 0 || !win->wicFactory) break;

            // Decode PNG via WIC
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

            // Create Direct2D bitmap from WIC converter
            float w = cmd->w > 0 ? cmd->w : (float)srcW;
            float h = cmd->h > 0 ? cmd->h : (float)srcH;

            D2D1_BITMAP_PROPERTIES bmpProps = {};
            bmpProps.pixelFormat.format = DXGI_FORMAT_B8G8R8A8_UNORM;
            bmpProps.pixelFormat.alphaMode = D2D1_ALPHA_MODE_PREMULTIPLIED;
            bmpProps.dpiX = win->dpi;
            bmpProps.dpiY = win->dpi;

            ID2D1Bitmap* bitmap = NULL;
            hr = win->rt->CreateBitmapFromWicBitmap(converter, &bmpProps, &bitmap);

            if (SUCCEEDED(hr) && bitmap) {
                // Draw bitmap scaled to target size
                D2D1_RECT_F destRect = ToRectF(cmd->x, cmd->y, w, h);
                win->rt->DrawBitmap(bitmap, destRect, 1.0f,
                    D2D1_BITMAP_INTERPOLATION_MODE_LINEAR, NULL);
                bitmap->Release();
            }

            converter->Release();
            frame->Release();
            decoder->Release();
            stream->Release();
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
    if (hr == D2DERR_RECREATE_TARGET) {
        // TODO: recreate render target
    }
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

extern "C" bool uiPumpMessages(void) {
    MSG msg;
    // Process all pending messages. Returns false when WM_QUIT is seen.
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
    InvalidateRect(hwnd, NULL, FALSE);
}