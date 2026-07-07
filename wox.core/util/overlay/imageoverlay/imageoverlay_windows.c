#define WIN32_LEAN_AND_MEAN
#define COBJMACROS
#include <windows.h>
#include <windowsx.h>
#include <wincodec.h>
#include <objbase.h>
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

#define IMAGE_OVERLAY_CLOSE_SIZE_DIP 24
#define IMAGE_OVERLAY_CLOSE_MARGIN_DIP 8

extern void overlayRequestCloseCallbackCGO(char *name);

typedef struct {
    HWND hwnd;
    HANDLE readyEvent;
    BOOL createOk;
    char *nameUtf8;
    HBITMAP bitmap;
    int imageWidth;
    int imageHeight;
    int surfaceWidth;
    int surfaceHeight;
    float cornerRadius;
    BOOL closable;
} ImageOverlayState;

static const wchar_t *kImageOverlayClassName = L"WoxImageOverlayAttachmentWindow";
static ATOM g_imageOverlayClass = 0;

static char *ImageOverlayCopyUtf8(const char *text)
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

static WCHAR *ImageOverlayUtf8ToWide(const char *text)
{
    if (!text)
        text = "";
    int count = MultiByteToWideChar(CP_UTF8, 0, text, -1, NULL, 0);
    if (count <= 0)
        count = 1;
    WCHAR *wide = (WCHAR *)calloc((size_t)count, sizeof(WCHAR));
    if (!wide)
        return NULL;
    MultiByteToWideChar(CP_UTF8, 0, text, -1, wide, count);
    return wide;
}

static HBITMAP ImageOverlayCreate32BitDIBSection(HDC hdc, int width, int height, void **bits)
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

static HBITMAP ImageOverlayCreateBitmapFromWicDecoder(IWICImagingFactory *factory, IWICBitmapDecoder *decoder, int *outW, int *outH)
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

    UINT w = 0;
    UINT h = 0;
    IWICBitmapSource_GetSize((IWICBitmapSource *)converter, &w, &h);
    if (w == 0 || h == 0)
    {
        IWICFormatConverter_Release(converter);
        IWICBitmapFrameDecode_Release(frame);
        return NULL;
    }

    HDC hdc = GetDC(NULL);
    void *bits = NULL;
    HBITMAP dib = ImageOverlayCreate32BitDIBSection(hdc, (int)w, (int)h, &bits);
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

static HBITMAP ImageOverlayCreateBitmapFromBytes(const unsigned char *data, int len, int *outW, int *outH)
{
    if (!data || len <= 0)
        return NULL;

    IWICImagingFactory *factory = NULL;
    HRESULT hr = CoCreateInstance(&CLSID_WICImagingFactory, NULL, CLSCTX_INPROC_SERVER, &IID_IWICImagingFactory, (LPVOID *)&factory);
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

    HBITMAP bitmap = ImageOverlayCreateBitmapFromWicDecoder(factory, decoder, outW, outH);
    IWICBitmapDecoder_Release(decoder);
    IStream_Release(stream);
    IWICImagingFactory_Release(factory);
    return bitmap;
}

static HBITMAP ImageOverlayCreateBitmapFromFile(const WCHAR *path, int *outW, int *outH)
{
    if (!path || !*path)
        return NULL;

    IWICImagingFactory *factory = NULL;
    HRESULT hr = CoCreateInstance(&CLSID_WICImagingFactory, NULL, CLSCTX_INPROC_SERVER, &IID_IWICImagingFactory, (LPVOID *)&factory);
    if (FAILED(hr) || !factory)
        return NULL;

    IWICBitmapDecoder *decoder = NULL;
    hr = IWICImagingFactory_CreateDecoderFromFilename(factory, path, NULL, GENERIC_READ, WICDecodeMetadataCacheOnLoad, &decoder);
    if (FAILED(hr) || !decoder)
    {
        IWICImagingFactory_Release(factory);
        return NULL;
    }

    HBITMAP bitmap = ImageOverlayCreateBitmapFromWicDecoder(factory, decoder, outW, outH);
    IWICBitmapDecoder_Release(decoder);
    IWICImagingFactory_Release(factory);
    return bitmap;
}

static void ImageOverlayDraw(HDC hdc, RECT rc, ImageOverlayState *state)
{
    if (!state || !state->bitmap)
        return;

    HRGN clip = NULL;
    int radius = (int)(state->cornerRadius + 0.5f);
    if (radius > 0)
    {
        clip = CreateRoundRectRgn(rc.left, rc.top, rc.right + 1, rc.bottom + 1, radius * 2, radius * 2);
        if (clip)
            SelectClipRgn(hdc, clip);
    }

    HDC memDC = CreateCompatibleDC(hdc);
    HGDIOBJ oldBitmap = SelectObject(memDC, state->bitmap);
    SetStretchBltMode(hdc, HALFTONE);
    StretchBlt(hdc, rc.left, rc.top, rc.right - rc.left, rc.bottom - rc.top, memDC, 0, 0, state->imageWidth, state->imageHeight, SRCCOPY);
    if (oldBitmap)
        SelectObject(memDC, oldBitmap);
    DeleteDC(memDC);

    if (clip)
    {
        SelectClipRgn(hdc, NULL);
        DeleteObject(clip);
    }
}

static int ImageOverlayDip(int value)
{
    return value;
}

static RECT ImageOverlayCloseRect(const RECT *rect)
{
    int size = ImageOverlayDip(IMAGE_OVERLAY_CLOSE_SIZE_DIP);
    int margin = ImageOverlayDip(IMAGE_OVERLAY_CLOSE_MARGIN_DIP);
    RECT closeRc = {rect->right - margin - size, rect->top + margin, rect->right - margin, rect->top + margin + size};
    return closeRc;
}

static void ImageOverlayDrawCloseButton(HDC hdc, const RECT *rect)
{
    RECT closeRc = ImageOverlayCloseRect(rect);
    int size = closeRc.right - closeRc.left;
    HBRUSH brush = CreateSolidBrush(RGB(42, 42, 42));
    HGDIOBJ oldBrush = SelectObject(hdc, brush);
    HGDIOBJ oldPen = SelectObject(hdc, GetStockObject(NULL_PEN));
    RoundRect(hdc, closeRc.left, closeRc.top, closeRc.right, closeRc.bottom, size, size);
    if (oldPen)
        SelectObject(hdc, oldPen);
    if (oldBrush)
        SelectObject(hdc, oldBrush);
    DeleteObject(brush);
    SetBkMode(hdc, TRANSPARENT);
    SetTextColor(hdc, RGB(255, 255, 255));
    DrawTextW(hdc, L"X", -1, &closeRc, DT_CENTER | DT_VCENTER | DT_SINGLELINE | DT_NOPREFIX);
}

static LRESULT CALLBACK ImageOverlayProc(HWND hwnd, UINT msg, WPARAM wParam, LPARAM lParam)
{
    ImageOverlayState *state = (ImageOverlayState *)GetWindowLongPtrW(hwnd, GWLP_USERDATA);
    switch (msg)
    {
    case WM_NCCREATE:
    {
        CREATESTRUCTW *cs = (CREATESTRUCTW *)lParam;
        SetWindowLongPtrW(hwnd, GWLP_USERDATA, (LONG_PTR)cs->lpCreateParams);
        return TRUE;
    }
    case WM_ERASEBKGND:
        return 1;
    case WM_SIZE:
        InvalidateRect(hwnd, NULL, FALSE);
        return 0;
    case WM_PAINT:
    {
        PAINTSTRUCT ps;
        HDC hdc = BeginPaint(hwnd, &ps);
        RECT rc;
        GetClientRect(hwnd, &rc);
        ImageOverlayDraw(hdc, rc, state);
        if (state && state->closable)
            ImageOverlayDrawCloseButton(hdc, &rc);
        EndPaint(hwnd, &ps);
        return 0;
    }
    case WM_LBUTTONUP:
        if (state && state->closable && state->nameUtf8)
        {
            RECT rc;
            GetClientRect(hwnd, &rc);
            RECT closeRc = ImageOverlayCloseRect(&rc);
            POINT pt = {GET_X_LPARAM(lParam), GET_Y_LPARAM(lParam)};
            if (PtInRect(&closeRc, pt))
            {
                overlayRequestCloseCallbackCGO(state->nameUtf8);
                return 0;
            }
        }
        break;
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

static BOOL ImageOverlayEnsureClass(void)
{
    if (g_imageOverlayClass)
        return TRUE;

    WNDCLASSEXW wc;
    ZeroMemory(&wc, sizeof(wc));
    wc.cbSize = sizeof(wc);
    wc.lpfnWndProc = ImageOverlayProc;
    wc.hInstance = GetModuleHandleW(NULL);
    wc.hCursor = LoadCursor(NULL, IDC_ARROW);
    wc.lpszClassName = kImageOverlayClassName;
    g_imageOverlayClass = RegisterClassExW(&wc);
    return g_imageOverlayClass != 0;
}

static DWORD WINAPI ImageOverlayThreadProc(LPVOID param)
{
    ImageOverlayState *state = (ImageOverlayState *)param;
    HWND hwnd = CreateWindowExW(WS_EX_NOACTIVATE, kImageOverlayClassName, L"", WS_POPUP, 0, 0, state->surfaceWidth, state->surfaceHeight, NULL, NULL, GetModuleHandleW(NULL), state);
    state->hwnd = hwnd;
    state->createOk = hwnd ? TRUE : FALSE;
    SetEvent(state->readyEvent);
    if (!hwnd)
        return 0;

    MSG msg;
    while (GetMessageW(&msg, NULL, 0, 0) > 0)
    {
        TranslateMessage(&msg);
        DispatchMessageW(&msg);
    }
    return 0;
}

void *ImageOverlayCreateWindow(char *name, unsigned char *imageData, int imageLen, char *imageFilePath, float width, float height, float cornerRadius, bool closable)
{
    if (!ImageOverlayEnsureClass())
        return NULL;

    HRESULT coResult = CoInitializeEx(NULL, COINIT_APARTMENTTHREADED);
    WCHAR *widePath = ImageOverlayUtf8ToWide(imageFilePath);
    int imageWidth = 0;
    int imageHeight = 0;
    HBITMAP bitmap = NULL;
    if (widePath && *widePath)
        bitmap = ImageOverlayCreateBitmapFromFile(widePath, &imageWidth, &imageHeight);
    if (!bitmap)
        bitmap = ImageOverlayCreateBitmapFromBytes(imageData, imageLen, &imageWidth, &imageHeight);
    free(widePath);
    if (SUCCEEDED(coResult))
        CoUninitialize();
    if (!bitmap)
        return NULL;

    ImageOverlayState *state = (ImageOverlayState *)calloc(1, sizeof(ImageOverlayState));
    if (!state)
    {
        DeleteObject(bitmap);
        return NULL;
    }
    state->nameUtf8 = ImageOverlayCopyUtf8(name);
    state->bitmap = bitmap;
    state->imageWidth = imageWidth > 0 ? imageWidth : 1;
    state->imageHeight = imageHeight > 0 ? imageHeight : 1;
    state->surfaceWidth = width > 0 ? (int)(width + 0.5f) : state->imageWidth;
    state->surfaceHeight = height > 0 ? (int)(height + 0.5f) : state->imageHeight;
    state->cornerRadius = cornerRadius;
    state->closable = closable ? TRUE : FALSE;
    if (state->surfaceWidth < 1)
        state->surfaceWidth = 1;
    if (state->surfaceHeight < 1)
        state->surfaceHeight = 1;

    state->readyEvent = CreateEventW(NULL, TRUE, FALSE, NULL);
    if (!state->readyEvent)
    {
        DeleteObject(state->bitmap);
        free(state->nameUtf8);
        free(state);
        return NULL;
    }

    HANDLE thread = CreateThread(NULL, 0, ImageOverlayThreadProc, state, 0, NULL);
    if (!thread)
    {
        CloseHandle(state->readyEvent);
        DeleteObject(state->bitmap);
        free(state->nameUtf8);
        free(state);
        return NULL;
    }

    WaitForSingleObject(state->readyEvent, INFINITE);
    CloseHandle(state->readyEvent);
    state->readyEvent = NULL;
    CloseHandle(thread);

    if (!state->createOk || !state->hwnd)
    {
        DeleteObject(state->bitmap);
        free(state->nameUtf8);
        free(state);
        return NULL;
    }
    return state->hwnd;
}

void ImageOverlayDestroyWindow(void *rawHwnd)
{
    HWND hwnd = (HWND)rawHwnd;
    if (hwnd && IsWindow(hwnd))
        PostMessageW(hwnd, WM_CLOSE, 0, 0);
}
