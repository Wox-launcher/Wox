#define WIN32_LEAN_AND_MEAN
#define COBJMACROS
#include <windows.h>
#include <windowsx.h>
#include <wincodec.h>
#include <objbase.h>
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

typedef struct {
    void *handle;
    float width;
    float height;
} IconOverlayAttachment;

extern bool overlayClickCallbackCGO(char *name);

typedef struct {
    HWND hwnd;
    HANDLE readyEvent;
    BOOL createOk;
    char *nameUtf8;
    HBITMAP bitmap;
    void *bitmapBits;
    int bitmapWidth;
    int bitmapHeight;
    float width;
    float height;
    float iconSize;
    UINT dpi;
} IconOverlayState;

static const wchar_t *kIconOverlayClassName = L"WoxIconOverlayAttachmentWindow";
static ATOM g_iconOverlayClass = 0;

static char *IconOverlayCopyUtf8(const char *text)
{
    if (!text)
        text = "";
    size_t length = strlen(text);
    char *copy = (char *)calloc(length + 1, sizeof(char));
    if (!copy)
        return NULL;
    memcpy(copy, text, length);
    return copy;
}

static HBITMAP IconOverlayCreate32BitDIBSection(HDC hdc, int width, int height, void **bits)
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

static HBITMAP IconOverlayCreateBitmapFromBytes(const unsigned char *data, int length, int *outWidth, int *outHeight, void **outBits)
{
    if (outBits)
        *outBits = NULL;
    if (!data || length <= 0)
        return NULL;

    IWICImagingFactory *factory = NULL;
    HRESULT hr = CoCreateInstance(&CLSID_WICImagingFactory, NULL, CLSCTX_INPROC_SERVER,
                                  &IID_IWICImagingFactory, (LPVOID *)&factory);
    if (FAILED(hr) || !factory)
        return NULL;

    HGLOBAL memory = GlobalAlloc(GMEM_MOVEABLE, (SIZE_T)length);
    if (!memory)
    {
        IWICImagingFactory_Release(factory);
        return NULL;
    }
    void *memoryData = GlobalLock(memory);
    if (!memoryData)
    {
        GlobalFree(memory);
        IWICImagingFactory_Release(factory);
        return NULL;
    }
    memcpy(memoryData, data, (SIZE_T)length);
    GlobalUnlock(memory);

    IStream *stream = NULL;
    hr = CreateStreamOnHGlobal(memory, TRUE, &stream);
    if (FAILED(hr) || !stream)
    {
        GlobalFree(memory);
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

    IWICBitmapFrameDecode *frame = NULL;
    hr = IWICBitmapDecoder_GetFrame(decoder, 0, &frame);
    IWICFormatConverter *converter = NULL;
    if (SUCCEEDED(hr))
        hr = IWICImagingFactory_CreateFormatConverter(factory, &converter);
    if (SUCCEEDED(hr))
    {
        hr = IWICFormatConverter_Initialize(converter, (IWICBitmapSource *)frame,
                                            &GUID_WICPixelFormat32bppBGRA, WICBitmapDitherTypeNone,
                                            NULL, 0.0, WICBitmapPaletteTypeCustom);
    }

    UINT width = 0;
    UINT height = 0;
    if (SUCCEEDED(hr))
        IWICBitmapSource_GetSize((IWICBitmapSource *)converter, &width, &height);

    HBITMAP bitmap = NULL;
    if (SUCCEEDED(hr) && width > 0 && height > 0)
    {
        HDC screen = GetDC(NULL);
        bitmap = IconOverlayCreate32BitDIBSection(screen, (int)width, (int)height, outBits);
        ReleaseDC(NULL, screen);
        if (!bitmap || !outBits || !*outBits)
        {
            if (bitmap)
                DeleteObject(bitmap);
            bitmap = NULL;
        }
        else
        {
            WICRect rect = {0, 0, (INT)width, (INT)height};
            hr = IWICBitmapSource_CopyPixels((IWICBitmapSource *)converter, &rect, width * 4,
                                             width * height * 4, (BYTE *)*outBits);
            if (FAILED(hr))
            {
                DeleteObject(bitmap);
                bitmap = NULL;
                *outBits = NULL;
            }
        }
    }

    if (converter)
        IWICFormatConverter_Release(converter);
    if (frame)
        IWICBitmapFrameDecode_Release(frame);
    IWICBitmapDecoder_Release(decoder);
    IStream_Release(stream);
    IWICImagingFactory_Release(factory);

    if (bitmap)
    {
        if (outWidth)
            *outWidth = (int)width;
        if (outHeight)
            *outHeight = (int)height;
    }
    return bitmap;
}

static UINT IconOverlayDpi(HWND hwnd, UINT fallback)
{
    UINT dpi = 0;
    if (hwnd)
    {
        typedef UINT(WINAPI * GetDpiForWindowFn)(HWND);
        HMODULE user32 = GetModuleHandleW(L"user32.dll");
        GetDpiForWindowFn getDpiForWindow = user32 ? (GetDpiForWindowFn)GetProcAddress(user32, "GetDpiForWindow") : NULL;
        if (getDpiForWindow)
            dpi = getDpiForWindow(hwnd);
    }
    if (dpi == 0)
    {
        typedef UINT(WINAPI * GetDpiForSystemFn)(void);
        HMODULE user32 = GetModuleHandleW(L"user32.dll");
        GetDpiForSystemFn getDpiForSystem = user32 ? (GetDpiForSystemFn)GetProcAddress(user32, "GetDpiForSystem") : NULL;
        if (getDpiForSystem)
            dpi = getDpiForSystem();
    }
    return dpi > 0 ? dpi : (fallback > 0 ? fallback : 96);
}

static HBITMAP IconOverlayCreateSurface(IconOverlayState *state, int width, int height, UINT dpi, void **outBits)
{
    if (outBits)
        *outBits = NULL;
    if (!state || !state->bitmapBits || state->bitmapWidth <= 0 || state->bitmapHeight <= 0 || width <= 0 || height <= 0)
        return NULL;

    HDC screen = GetDC(NULL);
    HBITMAP surface = IconOverlayCreate32BitDIBSection(screen, width, height, outBits);
    ReleaseDC(NULL, screen);
    if (!surface || !outBits || !*outBits)
    {
        if (surface)
            DeleteObject(surface);
        return NULL;
    }
    memset(*outBits, 0, (size_t)width * (size_t)height * 4);

    int iconWidth = (int)((state->iconSize * (float)dpi / 96.0f) + 0.5f);
    int iconHeight = iconWidth;
    if (iconWidth < 1)
        iconWidth = 1;
    if (iconHeight < 1)
        iconHeight = 1;
    if (iconWidth > width)
        iconWidth = width;
    if (iconHeight > height)
        iconHeight = height;
    int offsetX = (width - iconWidth) / 2;
    int offsetY = (height - iconHeight) / 2;
    BYTE *source = (BYTE *)state->bitmapBits;
    BYTE *destination = (BYTE *)*outBits;
    for (int y = 0; y < iconHeight; y++)
    {
        int sourceY = (int)((long long)y * state->bitmapHeight / iconHeight);
        for (int x = 0; x < iconWidth; x++)
        {
            int sourceX = (int)((long long)x * state->bitmapWidth / iconWidth);
            BYTE *sourcePixel = source + ((size_t)sourceY * state->bitmapWidth + sourceX) * 4;
            BYTE *destinationPixel = destination + ((size_t)(offsetY + y) * width + offsetX + x) * 4;
            BYTE alpha = sourcePixel[3];
            destinationPixel[0] = (BYTE)((sourcePixel[0] * alpha + 127) / 255);
            destinationPixel[1] = (BYTE)((sourcePixel[1] * alpha + 127) / 255);
            destinationPixel[2] = (BYTE)((sourcePixel[2] * alpha + 127) / 255);
            destinationPixel[3] = alpha;
        }
    }
    return surface;
}

static BOOL IconOverlayRenderLayered(IconOverlayState *state)
{
    if (!state || !state->hwnd || !IsWindow(state->hwnd))
        return FALSE;

    RECT client;
    if (!GetClientRect(state->hwnd, &client))
        return FALSE;
    int width = client.right - client.left;
    int height = client.bottom - client.top;
    if (width <= 0 || height <= 0)
        return FALSE;

    state->dpi = IconOverlayDpi(state->hwnd, state->dpi);
    void *surfaceBits = NULL;
    HBITMAP surface = IconOverlayCreateSurface(state, width, height, state->dpi, &surfaceBits);
    if (!surface)
        return FALSE;

    HDC screen = GetDC(NULL);
    HDC source = CreateCompatibleDC(screen);
    HGDIOBJ oldBitmap = SelectObject(source, surface);
    SIZE size = {width, height};
    POINT sourcePoint = {0, 0};
    BLENDFUNCTION blend = {AC_SRC_OVER, 0, 255, AC_SRC_ALPHA};
    BOOL updated = UpdateLayeredWindow(state->hwnd, screen, NULL, &size, source, &sourcePoint, 0, &blend, ULW_ALPHA);
    if (oldBitmap)
        SelectObject(source, oldBitmap);
    DeleteDC(source);
    ReleaseDC(NULL, screen);
    DeleteObject(surface);
    return updated;
}

static BOOL IconOverlayContainsPoint(IconOverlayState *state, POINT point)
{
    if (!state || !state->bitmapBits || point.x < 0 || point.y < 0)
        return FALSE;
    RECT client;
    if (!GetClientRect(state->hwnd, &client) || point.x >= client.right || point.y >= client.bottom)
        return FALSE;

    int width = client.right - client.left;
    int height = client.bottom - client.top;
    int iconWidth = (int)((state->iconSize * (float)state->dpi / 96.0f) + 0.5f);
    if (iconWidth < 1)
        iconWidth = 1;
    if (iconWidth > width)
        iconWidth = width;
    int iconHeight = iconWidth > height ? height : iconWidth;
    int offsetX = (width - iconWidth) / 2;
    int offsetY = (height - iconHeight) / 2;
    if (point.x < offsetX || point.x >= offsetX + iconWidth || point.y < offsetY || point.y >= offsetY + iconHeight)
        return FALSE;

    int sourceX = (int)((long long)(point.x - offsetX) * state->bitmapWidth / iconWidth);
    int sourceY = (int)((long long)(point.y - offsetY) * state->bitmapHeight / iconHeight);
    BYTE *source = (BYTE *)state->bitmapBits + ((size_t)sourceY * state->bitmapWidth + sourceX) * 4;
    return source[3] > 16;
}

static LRESULT CALLBACK IconOverlayProc(HWND hwnd, UINT msg, WPARAM wParam, LPARAM lParam)
{
    IconOverlayState *state = (IconOverlayState *)GetWindowLongPtrW(hwnd, GWLP_USERDATA);
    switch (msg)
    {
    case WM_NCCREATE:
    {
        CREATESTRUCTW *create = (CREATESTRUCTW *)lParam;
        SetWindowLongPtrW(hwnd, GWLP_USERDATA, (LONG_PTR)create->lpCreateParams);
        return TRUE;
    }
    case WM_ERASEBKGND:
        return 1;
    case WM_SIZE:
        IconOverlayRenderLayered(state);
        return 0;
    case WM_NCHITTEST:
    {
        POINT point = {GET_X_LPARAM(lParam), GET_Y_LPARAM(lParam)};
        ScreenToClient(hwnd, &point);
        return IconOverlayContainsPoint(state, point) ? HTCLIENT : HTTRANSPARENT;
    }
    case WM_LBUTTONDOWN:
    {
        POINT point = {GET_X_LPARAM(lParam), GET_Y_LPARAM(lParam)};
        if (IconOverlayContainsPoint(state, point))
            return 0;
        break;
    }
    case WM_LBUTTONUP:
    {
        POINT point = {GET_X_LPARAM(lParam), GET_Y_LPARAM(lParam)};
        if (IconOverlayContainsPoint(state, point) && state && state->nameUtf8)
        {
            overlayClickCallbackCGO(state->nameUtf8);
            return 0;
        }
        break;
    }
    case WM_DPICHANGED:
        if (state)
        {
            state->dpi = HIWORD(wParam);
            IconOverlayRenderLayered(state);
        }
        return 0;
    case WM_CLOSE:
        DestroyWindow(hwnd);
        return 0;
    case WM_NCDESTROY:
        if (state)
        {
            if (state->bitmap)
                DeleteObject(state->bitmap);
            free(state->nameUtf8);
            free(state);
            SetWindowLongPtrW(hwnd, GWLP_USERDATA, 0);
        }
        PostQuitMessage(0);
        break;
    }
    return DefWindowProcW(hwnd, msg, wParam, lParam);
}

static BOOL IconOverlayEnsureClass(void)
{
    if (g_iconOverlayClass)
        return TRUE;

    WNDCLASSEXW wc;
    ZeroMemory(&wc, sizeof(wc));
    wc.cbSize = sizeof(wc);
    wc.lpfnWndProc = IconOverlayProc;
    wc.hInstance = GetModuleHandleW(NULL);
    wc.hCursor = LoadCursor(NULL, IDC_ARROW);
    wc.lpszClassName = kIconOverlayClassName;
    g_iconOverlayClass = RegisterClassExW(&wc);
    return g_iconOverlayClass != 0;
}

static DWORD WINAPI IconOverlayThreadProc(LPVOID param)
{
    IconOverlayState *state = (IconOverlayState *)param;
    HWND hwnd = CreateWindowExW(WS_EX_NOACTIVATE | WS_EX_LAYERED, kIconOverlayClassName, L"", WS_POPUP,
                                0, 0, (int)state->width, (int)state->height, NULL, NULL,
                                GetModuleHandleW(NULL), state);
    state->hwnd = hwnd;
    state->createOk = hwnd ? TRUE : FALSE;
    if (hwnd)
        IconOverlayRenderLayered(state);
    SetEvent(state->readyEvent);
    if (!hwnd)
        return 0;

    MSG message;
    while (GetMessageW(&message, NULL, 0, 0) > 0)
    {
        TranslateMessage(&message);
        DispatchMessageW(&message);
    }
    return 0;
}

IconOverlayAttachment IconOverlayCreateWindow(char *name, unsigned char *iconData, int iconLen, float width, float height, float iconSize)
{
    IconOverlayAttachment result = {0};
    if (!IconOverlayEnsureClass())
        return result;

    HRESULT coResult = CoInitializeEx(NULL, COINIT_APARTMENTTHREADED);
    IconOverlayState *state = (IconOverlayState *)calloc(1, sizeof(IconOverlayState));
    if (!state)
    {
        if (SUCCEEDED(coResult))
            CoUninitialize();
        return result;
    }
    state->nameUtf8 = IconOverlayCopyUtf8(name);
    state->bitmap = IconOverlayCreateBitmapFromBytes(iconData, iconLen, &state->bitmapWidth, &state->bitmapHeight, &state->bitmapBits);
    state->width = width > 0 ? width : 1;
    state->height = height > 0 ? height : state->width;
    state->iconSize = iconSize > 0 ? iconSize : (state->width < state->height ? state->width : state->height);
    state->readyEvent = CreateEventW(NULL, TRUE, FALSE, NULL);
    if (!state->nameUtf8 || !state->bitmap || !state->readyEvent)
    {
        if (state->bitmap)
            DeleteObject(state->bitmap);
        free(state->nameUtf8);
        if (state->readyEvent)
            CloseHandle(state->readyEvent);
        free(state);
        if (SUCCEEDED(coResult))
            CoUninitialize();
        return result;
    }

    HANDLE thread = CreateThread(NULL, 0, IconOverlayThreadProc, state, 0, NULL);
    if (!thread)
    {
        CloseHandle(state->readyEvent);
        DeleteObject(state->bitmap);
        free(state->nameUtf8);
        free(state);
        if (SUCCEEDED(coResult))
            CoUninitialize();
        return result;
    }
    WaitForSingleObject(state->readyEvent, INFINITE);
    CloseHandle(state->readyEvent);
    state->readyEvent = NULL;
    CloseHandle(thread);
    if (SUCCEEDED(coResult))
        CoUninitialize();

    if (!state->createOk || !state->hwnd)
    {
        if (state->bitmap)
            DeleteObject(state->bitmap);
        free(state->nameUtf8);
        free(state);
        return result;
    }
    result.handle = state->hwnd;
    result.width = state->width;
    result.height = state->height;
    return result;
}

void IconOverlayDestroyWindow(void *rawHwnd)
{
    HWND hwnd = (HWND)rawHwnd;
    if (hwnd && IsWindow(hwnd))
        PostMessageW(hwnd, WM_CLOSE, 0, 0);
}
