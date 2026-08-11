#define WIN32_LEAN_AND_MEAN
#define COBJMACROS
#include <windows.h>
#include <windowsx.h>
#include <wincodec.h>
#include <objbase.h>
#include <uxtheme.h>
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>
#include <math.h>

#define TEXT_OVERLAY_TIMER_COPY_FEEDBACK 1
#define TEXT_OVERLAY_TIMER_AUTOCLOSE 2
#define TEXT_OVERLAY_TIMER_LOADING 3
#define TEXT_OVERLAY_AUTOCLOSE_PENDING_MS 250
#define TEXT_OVERLAY_LOADING_INTERVAL_MS 80
#define TEXT_OVERLAY_DEFAULT_FONT_SIZE 10.0f
#define TEXT_OVERLAY_COPY_SIZE_DIP 28
#define TEXT_OVERLAY_COPY_GAP_DIP 8
#define TEXT_OVERLAY_CLOSE_SIZE_DIP 20
#define TEXT_OVERLAY_CLOSE_GAP_DIP 8
#define TEXT_OVERLAY_MESSAGE_UPDATE (WM_APP + 1)

typedef struct {
    void *handle;
    float width;
    float height;
} TextOverlayAttachment;

extern bool overlayClickCallbackCGO(char *name);
extern void overlayRequestCloseCallbackCGO(char *name);

typedef struct {
    HWND hwnd;
    HANDLE readyEvent;
    BOOL createOk;
    char *nameUtf8;
    WCHAR *message;
    BOOL loading;
    BOOL closable;
    BOOL centerContent;
    BOOL showCopyButton;
    BOOL copied;
    BOOL closeHover;
    BOOL closePressed;
    int loadingPhase;
    RECT loadingRect;
    RECT closeRect;
    HBITMAP iconBitmap;
    int autoCloseSeconds;
    float fontSize;
    float iconSize;
    float tooltipIconSize;
    int contentWidth;
    int contentHeight;
} TextOverlayState;

// TextOverlayUpdatePayload moves mutable renderer state to the HWND owner thread.
typedef struct {
    WCHAR *message;
    BOOL loading;
    BOOL closable;
    BOOL centerContent;
    BOOL showCopyButton;
    HBITMAP iconBitmap;
    BOOL hasIcon;
    BOOL hasTooltip;
    int autoCloseSeconds;
    float fontSize;
    float iconSize;
    float tooltipIconSize;
    float windowWidth;
    float minWindowWidth;
    float maxWindowWidth;
    float windowHeight;
    float maxWindowHeight;
    int contentWidth;
    int contentHeight;
    BOOL success;
} TextOverlayUpdatePayload;

static const wchar_t *kTextOverlayClassName = L"WoxTextOverlayAttachmentWindow";
static ATOM g_textOverlayClass = 0;

static char *TextOverlayCopyUtf8(const char *text)
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

static WCHAR *TextOverlayUtf8ToWide(const char *text)
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

// TextOverlayCreate32BitDIBSection allocates a top-down premultiplied-alpha surface for WIC pixels.
static HBITMAP TextOverlayCreate32BitDIBSection(HDC hdc, int width, int height, void **bits)
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

// TextOverlayCreateBitmapFromWicDecoder converts one WIC frame into a GDI bitmap usable by AlphaBlend.
static HBITMAP TextOverlayCreateBitmapFromWicDecoder(IWICImagingFactory *factory, IWICBitmapDecoder *decoder)
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
                                        &GUID_WICPixelFormat32bppPBGRA, WICBitmapDitherTypeNone,
                                        NULL, 0.0, WICBitmapPaletteTypeCustom);
    if (FAILED(hr))
    {
        IWICFormatConverter_Release(converter);
        IWICBitmapFrameDecode_Release(frame);
        return NULL;
    }

    UINT width = 0;
    UINT height = 0;
    IWICBitmapSource_GetSize((IWICBitmapSource *)converter, &width, &height);
    if (width == 0 || height == 0)
    {
        IWICFormatConverter_Release(converter);
        IWICBitmapFrameDecode_Release(frame);
        return NULL;
    }

    HDC hdc = GetDC(NULL);
    void *bits = NULL;
    HBITMAP bitmap = TextOverlayCreate32BitDIBSection(hdc, (int)width, (int)height, &bits);
    ReleaseDC(NULL, hdc);
    if (!bitmap || !bits)
    {
        if (bitmap)
            DeleteObject(bitmap);
        IWICFormatConverter_Release(converter);
        IWICBitmapFrameDecode_Release(frame);
        return NULL;
    }

    WICRect rect = {0, 0, (INT)width, (INT)height};
    hr = IWICBitmapSource_CopyPixels((IWICBitmapSource *)converter, &rect, width * 4, width * height * 4, (BYTE *)bits);
    if (FAILED(hr))
    {
        DeleteObject(bitmap);
        bitmap = NULL;
    }

    IWICFormatConverter_Release(converter);
    IWICBitmapFrameDecode_Release(frame);
    return bitmap;
}

// TextOverlayCreateBitmapFromBytes decodes the PNG payload produced by the Go renderer.
static HBITMAP TextOverlayCreateBitmapFromBytes(const unsigned char *data, int len)
{
    if (!data || len <= 0)
        return NULL;

    IWICImagingFactory *factory = NULL;
    HRESULT hr = CoCreateInstance(&CLSID_WICImagingFactory, NULL, CLSCTX_INPROC_SERVER, &IID_IWICImagingFactory, (LPVOID *)&factory);
    if (FAILED(hr) || !factory)
        return NULL;

    HGLOBAL memory = GlobalAlloc(GMEM_MOVEABLE, (SIZE_T)len);
    if (!memory)
    {
        IWICImagingFactory_Release(factory);
        return NULL;
    }
    void *lockedMemory = GlobalLock(memory);
    if (!lockedMemory)
    {
        GlobalFree(memory);
        IWICImagingFactory_Release(factory);
        return NULL;
    }
    memcpy(lockedMemory, data, (SIZE_T)len);
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

    HBITMAP bitmap = TextOverlayCreateBitmapFromWicDecoder(factory, decoder);
    IWICBitmapDecoder_Release(decoder);
    IStream_Release(stream);
    IWICImagingFactory_Release(factory);
    return bitmap;
}

static UINT TextOverlayGetDpi(HWND hwnd)
{
    HMODULE user32 = GetModuleHandleW(L"user32.dll");
    if (user32)
    {
        typedef UINT(WINAPI *GetDpiForWindowProc)(HWND);
        GetDpiForWindowProc getDpiForWindow = (GetDpiForWindowProc)GetProcAddress(user32, "GetDpiForWindow");
        if (getDpiForWindow)
            return getDpiForWindow(hwnd);
    }
    return 96;
}

static int TextOverlayDip(float value, UINT dpi)
{
    return MulDiv((int)(value + 0.5f), (int)dpi, 96);
}

static HFONT TextOverlayCreateFont(float fontSize, UINT dpi)
{
    float resolvedSize = fontSize > 0 ? fontSize : TEXT_OVERLAY_DEFAULT_FONT_SIZE;
    int height = -MulDiv((int)(resolvedSize + 0.5f), (int)dpi, 72);
    return CreateFontW(height, 0, 0, 0, FW_NORMAL, FALSE, FALSE, FALSE, DEFAULT_CHARSET, OUT_DEFAULT_PRECIS, CLIP_DEFAULT_PRECIS, CLEARTYPE_QUALITY, DEFAULT_PITCH | FF_SWISS, L"Segoe UI");
}

static SIZE TextOverlayMeasureText(WCHAR *message, int textWidth, float fontSize)
{
    HDC hdc = GetDC(NULL);
    UINT dpi = 96;
    HFONT font = TextOverlayCreateFont(fontSize, dpi);
    HGDIOBJ oldFont = SelectObject(hdc, font);
    RECT rc = {0, 0, textWidth > 0 ? textWidth : 1, 1};
    DrawTextW(hdc, message ? message : L"", -1, &rc, DT_CALCRECT | DT_WORDBREAK | DT_NOPREFIX);
    SIZE size;
    size.cx = rc.right - rc.left;
    size.cy = rc.bottom - rc.top;
    if (oldFont)
        SelectObject(hdc, oldFont);
    DeleteObject(font);
    ReleaseDC(NULL, hdc);
    if (size.cx < 1)
        size.cx = 1;
    if (size.cy < 1)
        size.cy = 1;
    return size;
}

static SIZE TextOverlayMeasure(WCHAR *message, BOOL loading, BOOL hasIcon, BOOL hasTooltip, BOOL showCopyButton, BOOL closable, float fontSize, float iconSize, float tooltipIconSize, float windowWidth, float minWindowWidth, float maxWindowWidth, float windowHeight, float maxWindowHeight)
{
    int leadingWidth = (loading || hasIcon) ? (int)(iconSize > 0 ? iconSize : 24) : 0;
    int leadingGap = leadingWidth > 0 ? 8 : 0;
    int tooltipWidth = hasTooltip ? (int)(tooltipIconSize > 0 ? tooltipIconSize : 18) : 0;
    int tooltipGap = tooltipWidth > 0 ? 8 : 0;
    int closeReserve = closable ? TEXT_OVERLAY_CLOSE_SIZE_DIP + TEXT_OVERLAY_CLOSE_GAP_DIP : 0;
    int chromeWidth = 36;
    int chromeHeight = 24;

    SIZE naturalText = TextOverlayMeasureText(message, 4096, fontSize);
    int naturalContentWidth = leadingWidth + leadingGap + naturalText.cx + tooltipGap + tooltipWidth + closeReserve;
    int contentWidth = naturalContentWidth;
    // Use the legacy 400 DIP window cap only when the caller does not provide a larger or smaller maximum.
    int maxContentWidth = maxWindowWidth > 0 ? (int)maxWindowWidth - chromeWidth : 364;
    if (maxContentWidth < 1)
        maxContentWidth = 1;
    int minContentWidth = minWindowWidth > 0 ? (int)minWindowWidth - chromeWidth : 64;
    if (minContentWidth < 1)
        minContentWidth = 1;
    if (contentWidth < minContentWidth)
        contentWidth = minContentWidth;
    if (contentWidth > maxContentWidth)
        contentWidth = maxContentWidth;

    if (windowWidth > 0)
        contentWidth = (int)windowWidth - chromeWidth;
    if (minWindowWidth > 0 && contentWidth < (int)minWindowWidth - chromeWidth)
        contentWidth = (int)minWindowWidth - chromeWidth;
    if (contentWidth < 1)
        contentWidth = 1;

    int textWidth = contentWidth - leadingWidth - leadingGap - tooltipWidth - tooltipGap - closeReserve;
    if (textWidth < 1)
        textWidth = 1;
    SIZE wrappedText = TextOverlayMeasureText(message, textWidth, fontSize);
    int copyReserve = showCopyButton ? TEXT_OVERLAY_COPY_SIZE_DIP + TEXT_OVERLAY_COPY_GAP_DIP : 0;
    int rowHeight = wrappedText.cy > leadingWidth ? wrappedText.cy : leadingWidth;
    if (closable && rowHeight < TEXT_OVERLAY_CLOSE_SIZE_DIP)
        rowHeight = TEXT_OVERLAY_CLOSE_SIZE_DIP;
    int contentHeight = rowHeight + copyReserve;
    if (windowHeight > 0)
        contentHeight = (int)windowHeight - chromeHeight;
    else if (maxWindowHeight > 0 && contentHeight > (int)maxWindowHeight - chromeHeight)
        contentHeight = (int)maxWindowHeight - chromeHeight;
    if (contentHeight < 1)
        contentHeight = 1;

    SIZE result;
    result.cx = contentWidth;
    result.cy = contentHeight;
    return result;
}

static BOOL TextOverlayCursorInsideWindow(HWND hwnd)
{
    if (!hwnd)
        return FALSE;

    HWND target = GetParent(hwnd);
    if (!target)
        target = hwnd;

    POINT screenPt;
    RECT windowRect;
    if (!GetCursorPos(&screenPt) || !GetWindowRect(target, &windowRect))
        return FALSE;
    return PtInRect(&windowRect, screenPt);
}

// Native attachment sizes are reported in logical units, while this child paints in physical pixels.
static RECT TextOverlayClientRect(TextOverlayState *state)
{
    RECT client = {0, 0, state ? state->contentWidth : 0, state ? state->contentHeight : 0};
    RECT measured;
    if (state && state->hwnd && GetClientRect(state->hwnd, &measured) && measured.right > 0 && measured.bottom > 0)
        client = measured;
    return client;
}

static RECT TextOverlayCopyButtonRect(TextOverlayState *state, UINT dpi)
{
    RECT client = TextOverlayClientRect(state);

    int size = TextOverlayDip(TEXT_OVERLAY_COPY_SIZE_DIP, dpi);
    RECT rc = {client.right - size, client.bottom - size, client.right, client.bottom};
    return rc;
}

static RECT TextOverlayCloseButtonRect(TextOverlayState *state, UINT dpi)
{
    int size = TextOverlayDip(TEXT_OVERLAY_CLOSE_SIZE_DIP, dpi);
    RECT client = TextOverlayClientRect(state);
    if (state->closeRect.right > state->closeRect.left && state->closeRect.bottom > state->closeRect.top)
    {
        RECT closeRect = state->closeRect;
        closeRect.left = client.right - size;
        closeRect.right = client.right;
        if (closeRect.bottom > client.bottom)
        {
            closeRect.bottom = client.bottom;
            closeRect.top = closeRect.bottom - size;
        }
        if (closeRect.top < 0)
            closeRect.top = 0;
        return closeRect;
    }

    int top = (client.bottom - size) / 2;
    if (top < 0)
        top = 0;
    if (top + size > client.bottom)
        top = client.bottom - size;
    if (top < 0)
        top = 0;
    RECT rc = {client.right - size, top, client.right, top + size};
    return rc;
}

// TextOverlayInvalidate only invalidates this window. After switching to WS_EX_LAYERED +
// UpdateLayeredWindow the child owns an opaque alpha surface, so the parent backdrop no longer
// needs to repaint behind it. This removes the cross-thread RDW_UPDATENOW that caused flicker.
static void TextOverlayInvalidate(HWND hwnd)
{
    InvalidateRect(hwnd, NULL, FALSE);
}

// TextOverlayForwardMouseMessage lets the parent overlay keep shared drag and click behavior.
static BOOL TextOverlayForwardMouseMessage(HWND hwnd, UINT msg, WPARAM wParam, LPARAM lParam)
{
    HWND parent = GetParent(hwnd);
    if (!parent)
        return FALSE;
    LPARAM forwardedLParam = lParam;
    if (msg != WM_MOUSEWHEEL)
    {
        POINT pt = {GET_X_LPARAM(lParam), GET_Y_LPARAM(lParam)};
        MapWindowPoints(hwnd, parent, &pt, 1);
        forwardedLParam = MAKELPARAM(pt.x, pt.y);
    }
    SendMessageW(parent, msg, wParam, forwardedLParam);
    return TRUE;
}

static void TextOverlayPutAlphaPixel(BYTE *pixels, int width, int px, int py, BYTE alpha)
{
    BYTE *pixel = pixels + ((py * width + px) * 4);
    if (alpha <= pixel[3])
        return;
    pixel[0] = alpha;
    pixel[1] = alpha;
    pixel[2] = alpha;
    pixel[3] = alpha;
}

static void TextOverlayFillLoadingSpinnerPixels(BYTE *pixels, int size, int phase)
{
    ZeroMemory(pixels, (size_t)size * (size_t)size * 4);

    static const int dx[8] = {0, 707, 1000, 707, 0, -707, -1000, -707};
    static const int dy[8] = {-1000, -707, 0, 707, 1000, 707, 0, -707};
    float center = ((float)size - 1.0f) / 2.0f;
    float orbit = (float)size * 0.32f;
    float radius = (float)size * 0.085f;
    if (radius < 1.25f)
        radius = 1.25f;
    float inner = radius - 0.5f;
    float outer = radius + 0.5f;
    if (inner < 0.0f)
        inner = 0.0f;
    float innerSq = inner * inner;
    float outerSq = outer * outer;
    float fadeRange = outerSq - innerSq;
    if (fadeRange <= 0.0f)
        fadeRange = 1.0f;

    int active = phase % 8;
    for (int i = 0; i < 8; i++)
    {
        int age = (i - active + 8) % 8;
        int alpha = 235 - age * 22;
        if (alpha < 70)
            alpha = 70;

        float dotX = center + ((float)dx[i] * orbit / 1000.0f);
        float dotY = center + ((float)dy[i] * orbit / 1000.0f);
        int left = (int)floorf(dotX - outer);
        int top = (int)floorf(dotY - outer);
        int right = (int)ceilf(dotX + outer);
        int bottom = (int)ceilf(dotY + outer);
        if (left < 0)
            left = 0;
        if (top < 0)
            top = 0;
        if (right >= size)
            right = size - 1;
        if (bottom >= size)
            bottom = size - 1;

        for (int py = top; py <= bottom; py++)
        {
            for (int px = left; px <= right; px++)
            {
                float fx = (float)px + 0.5f - dotX;
                float fy = (float)py + 0.5f - dotY;
                float distSq = fx * fx + fy * fy;
                if (distSq > outerSq)
                    continue;

                float coverage = 1.0f;
                if (distSq > innerSq)
                    coverage = (outerSq - distSq) / fadeRange;
                int pixelAlpha = (int)((float)alpha * coverage + 0.5f);
                if (pixelAlpha > 0)
                    TextOverlayPutAlphaPixel(pixels, size, px, py, (BYTE)pixelAlpha);
            }
        }
    }
}

// TextOverlayDrawTextAlpha avoids ClearType color fringes without painting an opaque child background.
static void TextOverlayDrawTextAlpha(HDC hdc, HFONT font, WCHAR *text, RECT rc, UINT flags, COLORREF color)
{
    int width = rc.right - rc.left;
    int height = rc.bottom - rc.top;
    if (width <= 0 || height <= 0)
        return;

    BITMAPINFO bmi;
    ZeroMemory(&bmi, sizeof(bmi));
    bmi.bmiHeader.biSize = sizeof(BITMAPINFOHEADER);
    bmi.bmiHeader.biWidth = width;
    bmi.bmiHeader.biHeight = -height;
    bmi.bmiHeader.biPlanes = 1;
    bmi.bmiHeader.biBitCount = 32;
    bmi.bmiHeader.biCompression = BI_RGB;

    void *rawBits = NULL;
    HBITMAP dib = CreateDIBSection(hdc, &bmi, DIB_RGB_COLORS, &rawBits, NULL, 0);
    if (!dib || !rawBits)
    {
        if (dib)
            DeleteObject(dib);
        return;
    }

    HDC memDC = CreateCompatibleDC(hdc);
    HGDIOBJ oldBitmap = SelectObject(memDC, dib);
    HGDIOBJ oldFont = SelectObject(memDC, font);
    SetBkMode(memDC, TRANSPARENT);
    SetTextColor(memDC, RGB(255, 255, 255));

    RECT textRc = {0, 0, width, height};
    DrawTextW(memDC, text ? text : L"", -1, &textRc, flags);

    BYTE textR = GetRValue(color);
    BYTE textG = GetGValue(color);
    BYTE textB = GetBValue(color);
    BYTE *pixels = (BYTE *)rawBits;
    for (int i = 0; i < width * height; i++)
    {
        BYTE b = pixels[i * 4 + 0];
        BYTE g = pixels[i * 4 + 1];
        BYTE r = pixels[i * 4 + 2];
        BYTE alpha = r > g ? r : g;
        if (b > alpha)
            alpha = b;
        pixels[i * 4 + 0] = (BYTE)((int)textB * alpha / 255);
        pixels[i * 4 + 1] = (BYTE)((int)textG * alpha / 255);
        pixels[i * 4 + 2] = (BYTE)((int)textR * alpha / 255);
        pixels[i * 4 + 3] = alpha;
    }

    BLENDFUNCTION blend = {AC_SRC_OVER, 0, 255, AC_SRC_ALPHA};
    AlphaBlend(hdc, rc.left, rc.top, width, height, memDC, 0, 0, width, height, blend);

    if (oldFont)
        SelectObject(memDC, oldFont);
    if (oldBitmap)
        SelectObject(memDC, oldBitmap);
    DeleteDC(memDC);
    DeleteObject(dib);
}

// TextOverlayDrawLoadingSpinner uses a premultiplied alpha DIB so the small dots stay anti-aliased over the HUD backdrop.
static void TextOverlayDrawLoadingSpinner(HDC hdc, int x, int y, int size, int phase)
{
    if (size < 8)
        return;

    BITMAPINFO bmi;
    ZeroMemory(&bmi, sizeof(bmi));
    bmi.bmiHeader.biSize = sizeof(BITMAPINFOHEADER);
    bmi.bmiHeader.biWidth = size;
    bmi.bmiHeader.biHeight = -size;
    bmi.bmiHeader.biPlanes = 1;
    bmi.bmiHeader.biBitCount = 32;
    bmi.bmiHeader.biCompression = BI_RGB;

    void *rawBits = NULL;
    HBITMAP dib = CreateDIBSection(hdc, &bmi, DIB_RGB_COLORS, &rawBits, NULL, 0);
    if (!dib || !rawBits)
    {
        if (dib)
            DeleteObject(dib);
        return;
    }

    BYTE *pixels = (BYTE *)rawBits;
    TextOverlayFillLoadingSpinnerPixels(pixels, size, phase);

    HDC memDC = CreateCompatibleDC(hdc);
    HGDIOBJ oldBitmap = SelectObject(memDC, dib);
    BLENDFUNCTION blend = {AC_SRC_OVER, 0, 255, AC_SRC_ALPHA};
    AlphaBlend(hdc, x, y, size, size, memDC, 0, 0, size, size, blend);
    if (oldBitmap)
        SelectObject(memDC, oldBitmap);
    DeleteDC(memDC);
    DeleteObject(dib);
}

// TextOverlayDrawIcon paints the decoded plugin icon with alpha preservation on the buffered surface.
static void TextOverlayDrawIcon(HDC hdc, HBITMAP bitmap, int x, int y, int size)
{
    if (!bitmap || size <= 0)
        return;

    BITMAP source;
    ZeroMemory(&source, sizeof(source));
    if (GetObject(bitmap, sizeof(source), &source) == 0 || source.bmWidth <= 0)
        return;
    if (source.bmHeight < 0)
        source.bmHeight = -source.bmHeight;
    if (source.bmHeight <= 0)
        return;

    HDC sourceDC = CreateCompatibleDC(hdc);
    if (!sourceDC)
        return;
    HGDIOBJ oldBitmap = SelectObject(sourceDC, bitmap);
    SetStretchBltMode(hdc, HALFTONE);
    BLENDFUNCTION blend = {AC_SRC_OVER, 0, 255, AC_SRC_ALPHA};
    AlphaBlend(hdc, x, y, size, size, sourceDC, 0, 0, source.bmWidth, source.bmHeight, blend);
    if (oldBitmap)
        SelectObject(sourceDC, oldBitmap);
    DeleteDC(sourceDC);
}

// TextOverlayDrawCopyGlyph keeps the copy action readable at high DPI without reusing the
// message font, which is too large for the compact 28 DIP button.
static void TextOverlayDrawCopyGlyph(HDC hdc, RECT rc, UINT dpi, BOOL copied)
{
    int width = rc.right - rc.left;
    int height = rc.bottom - rc.top;
    int iconWidth = TextOverlayDip(14, dpi);
    int iconHeight = TextOverlayDip(14, dpi);
    int left = rc.left + (width - iconWidth) / 2;
    int top = rc.top + (height - iconHeight) / 2;
    int stroke = TextOverlayDip(1, dpi);
    if (stroke < 1)
        stroke = 1;

    HPEN pen = CreatePen(PS_SOLID, stroke, RGB(245, 245, 245));
    HGDIOBJ oldPen = SelectObject(hdc, pen);
    HGDIOBJ oldBrush = SelectObject(hdc, GetStockObject(NULL_BRUSH));

    if (copied)
    {
        MoveToEx(hdc, left + TextOverlayDip(2, dpi), top + TextOverlayDip(7, dpi), NULL);
        LineTo(hdc, left + TextOverlayDip(5, dpi), top + TextOverlayDip(10, dpi));
        LineTo(hdc, left + TextOverlayDip(12, dpi), top + TextOverlayDip(3, dpi));
    }
    else
    {
        int backLeft = left + TextOverlayDip(4, dpi);
        int backTop = top + TextOverlayDip(1, dpi);
        int backRight = left + TextOverlayDip(12, dpi);
        int backBottom = top + TextOverlayDip(10, dpi);
        int frontLeft = left + TextOverlayDip(2, dpi);
        int frontTop = top + TextOverlayDip(4, dpi);
        int frontRight = left + TextOverlayDip(10, dpi);
        int frontBottom = top + TextOverlayDip(13, dpi);
        RoundRect(hdc, backLeft, backTop, backRight, backBottom, TextOverlayDip(2, dpi), TextOverlayDip(2, dpi));
        RoundRect(hdc, frontLeft, frontTop, frontRight, frontBottom, TextOverlayDip(2, dpi), TextOverlayDip(2, dpi));
    }

    if (oldBrush)
        SelectObject(hdc, oldBrush);
    if (oldPen)
        SelectObject(hdc, oldPen);
    DeleteObject(pen);
}

// TextOverlayDrawCopyButton matches the native overlay button treatment across platforms.
static void TextOverlayDrawCopyButton(HDC hdc, RECT rc, UINT dpi, BOOL copied)
{
    COLORREF background = copied ? RGB(46, 112, 82) : RGB(66, 66, 66);
    COLORREF border = copied ? RGB(92, 158, 125) : RGB(105, 105, 105);
    int radius = TextOverlayDip(6, dpi);
    int stroke = TextOverlayDip(1, dpi);
    if (stroke < 1)
        stroke = 1;

    HBRUSH brush = CreateSolidBrush(background);
    HPEN pen = CreatePen(PS_SOLID, stroke, border);
    HGDIOBJ oldBrush = SelectObject(hdc, brush);
    HGDIOBJ oldPen = SelectObject(hdc, pen);
    RoundRect(hdc, rc.left, rc.top, rc.right, rc.bottom, radius, radius);
    if (oldPen)
        SelectObject(hdc, oldPen);
    if (oldBrush)
        SelectObject(hdc, oldBrush);
    DeleteObject(pen);
    DeleteObject(brush);

    TextOverlayDrawCopyGlyph(hdc, rc, dpi, copied);
}

static LRESULT CALLBACK TextOverlayProc(HWND hwnd, UINT msg, WPARAM wParam, LPARAM lParam);

static void TextOverlayDraw(HDC hdc, RECT rc, TextOverlayState *state)
{
    UINT dpi = TextOverlayGetDpi(state->hwnd);
    SetBkMode(hdc, TRANSPARENT);
    COLORREF textColor = RGB(245, 245, 245);
    SetTextColor(hdc, textColor);

    HFONT font = TextOverlayCreateFont(state->fontSize, dpi);
    HGDIOBJ oldFont = SelectObject(hdc, font);

    int iconSize = TextOverlayDip(state->iconSize > 0 ? state->iconSize : 24.0f, dpi);
    int leadingWidth = (state->loading || state->iconBitmap) ? iconSize : 0;
    int leadingGap = leadingWidth > 0 ? TextOverlayDip(8, dpi) : 0;
    int copyReserve = state->showCopyButton ? TextOverlayDip(TEXT_OVERLAY_COPY_SIZE_DIP + TEXT_OVERLAY_COPY_GAP_DIP, dpi) : 0;
    int closeReserve = state->closable ? TextOverlayDip(TEXT_OVERLAY_CLOSE_SIZE_DIP + TEXT_OVERLAY_CLOSE_GAP_DIP, dpi) : 0;
    int contentAreaWidth = rc.right - rc.left - closeReserve;
    if (contentAreaWidth < 1)
        contentAreaWidth = 1;
    int maxTextWidth = contentAreaWidth - leadingWidth - leadingGap;
    if (maxTextWidth < 1)
        maxTextWidth = 1;

    RECT textMeasure = {0, 0, maxTextWidth, 1};
    DrawTextW(hdc, state->message ? state->message : L"", -1, &textMeasure, DT_CALCRECT | DT_WORDBREAK | DT_NOPREFIX);
    int renderedTextWidth = textMeasure.right - textMeasure.left;
    int textHeight = textMeasure.bottom - textMeasure.top;
    if (renderedTextWidth < 1)
        renderedTextWidth = 1;
    if (renderedTextWidth > maxTextWidth)
        renderedTextWidth = maxTextWidth;
    if (textHeight < 1)
        textHeight = 1;
    int textLayoutWidth = state->centerContent ? renderedTextWidth : maxTextWidth;
    int leadingHeight = (state->loading || state->iconBitmap) ? iconSize : 0;
    int rowHeight = textHeight > leadingHeight ? textHeight : leadingHeight;
    int closeSize = TextOverlayDip(TEXT_OVERLAY_CLOSE_SIZE_DIP, dpi);
    if (state->closable && rowHeight < closeSize)
        rowHeight = closeSize;
    int rowY = (rc.bottom - rc.top - copyReserve - rowHeight) / 2;
    if (rowY < 0)
        rowY = 0;

    int groupWidth = leadingWidth + leadingGap + textLayoutWidth;
    int x = state->centerContent ? (contentAreaWidth - groupWidth) / 2 : 0;
    if (x < 0)
        x = 0;

    int textY = rowY + (rowHeight - textHeight) / 2;
    if (state->loading)
    {
        // DrawText top-aligns glyphs inside the measured line box. Center the spinner on the
        // first-line ascent/ink so it lines up with visible text instead of unused descent.
        int y = rowY + (rowHeight - iconSize) / 2;
        TEXTMETRICW metrics;
        if (GetTextMetricsW(hdc, &metrics) && metrics.tmAscent > 0)
        {
            int inkTop = textY + metrics.tmInternalLeading;
            int inkHeight = metrics.tmAscent - metrics.tmInternalLeading;
            if (inkHeight < 1)
                inkHeight = metrics.tmAscent;
            y = inkTop + (inkHeight - iconSize) / 2;
        }
        if (y < 0)
            y = 0;
        if (y + iconSize > rc.bottom - copyReserve)
            y = rc.bottom - copyReserve - iconSize;
        if (y < 0)
            y = 0;

        state->loadingRect.left = x;
        state->loadingRect.top = y;
        state->loadingRect.right = x + iconSize;
        state->loadingRect.bottom = y + iconSize;
        TextOverlayDrawLoadingSpinner(hdc, x, y, iconSize, state->loadingPhase);
        x += leadingWidth + leadingGap;
    }
    else if (state->iconBitmap)
    {
        int y = rowY + (rowHeight - iconSize) / 2;
        TextOverlayDrawIcon(hdc, state->iconBitmap, x, y, iconSize);
        x += leadingWidth + leadingGap;
    }
    else
    {
        RECT empty = {0, 0, 0, 0};
        state->loadingRect = empty;
    }

    RECT textRc = {x, textY, x + textLayoutWidth, textY + textHeight};
    TextOverlayDrawTextAlpha(hdc, font, state->message, textRc, DT_WORDBREAK | DT_NOPREFIX, textColor);

    if (state->closable)
    {
        TEXTMETRICW metrics;
        int lineHeight = GetTextMetricsW(hdc, &metrics) ? metrics.tmHeight : textHeight;
        if (lineHeight < 1)
            lineHeight = textHeight;
        BOOL multiline = textHeight > lineHeight + TextOverlayDip(2, dpi);
        int closeTop = multiline ? textY + (lineHeight - closeSize) / 2 : rowY + (rowHeight - closeSize) / 2;
        if (closeTop < 0)
            closeTop = 0;
        if (closeTop + closeSize > rc.bottom)
            closeTop = rc.bottom - closeSize;
        if (closeTop < 0)
            closeTop = 0;
        state->closeRect.left = rc.right - closeSize;
        state->closeRect.top = closeTop;
        state->closeRect.right = rc.right;
        state->closeRect.bottom = closeTop + closeSize;

        RECT closeRc = state->closeRect;
        if (state->closeHover || state->closePressed)
        {
            COLORREF bg = state->closePressed ? RGB(70, 70, 70) : RGB(55, 55, 55);
            HBRUSH brush = CreateSolidBrush(bg);
            FillRect(hdc, &closeRc, brush);
            DeleteObject(brush);
        }

        int pad = TextOverlayDip(6, dpi);
        int thickness = TextOverlayDip(2, dpi);
        if (thickness < 1)
            thickness = 1;

        HPEN pen = CreatePen(PS_SOLID, thickness, RGB(230, 230, 230));
        HGDIOBJ oldPen = SelectObject(hdc, pen);

        MoveToEx(hdc, closeRc.left + pad, closeRc.top + pad, NULL);
        LineTo(hdc, closeRc.right - pad, closeRc.bottom - pad);
        MoveToEx(hdc, closeRc.right - pad, closeRc.top + pad, NULL);
        LineTo(hdc, closeRc.left + pad, closeRc.bottom - pad);

        if (oldPen)
            SelectObject(hdc, oldPen);
        DeleteObject(pen);
    }
    else
    {
        RECT empty = {0, 0, 0, 0};
        state->closeRect = empty;
    }

    if (state->showCopyButton)
    {
        RECT copyRc = TextOverlayCopyButtonRect(state, dpi);
        TextOverlayDrawCopyButton(hdc, copyRc, dpi, state->copied);
    }

    if (oldFont)
        SelectObject(hdc, oldFont);
    DeleteObject(font);
}

// TextOverlayPaint draws one full frame into the given DC using the same BufferedPaint API the
// base overlay uses. Painting the spinner, text, and buttons into one buffered DC in a single
// pass eliminates the flicker the old separate popup + cross-thread parent invalidation caused.
static void TextOverlayPaint(HWND hwnd, HDC paintHdc, TextOverlayState *state)
{
    RECT rc;
    GetClientRect(hwnd, &rc);

    HDC hdc = paintHdc;
    HPAINTBUFFER paintBuf = BeginBufferedPaint(paintHdc, &rc, BPBF_TOPDOWNDIB, NULL, &hdc);
    if (paintBuf)
        BufferedPaintClear(paintBuf, &rc);

    if (state)
        TextOverlayDraw(hdc, rc, state);

    if (paintBuf)
        EndBufferedPaint(paintBuf, TRUE);
}

static LRESULT CALLBACK TextOverlayProc(HWND hwnd, UINT msg, WPARAM wParam, LPARAM lParam)
{
    TextOverlayState *state = (TextOverlayState *)GetWindowLongPtrW(hwnd, GWLP_USERDATA);
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
        // Size changes (from parent layout) must trigger a fresh paint at the new dimensions.
        InvalidateRect(hwnd, NULL, FALSE);
        return 0;
    case TEXT_OVERLAY_MESSAGE_UPDATE:
    {
        TextOverlayUpdatePayload *payload = (TextOverlayUpdatePayload *)lParam;
        if (!state || !payload || !payload->message)
            return 0;

        float resolvedFontSize = payload->fontSize > 0 ? payload->fontSize : TEXT_OVERLAY_DEFAULT_FONT_SIZE;
        float resolvedIconSize = payload->iconSize > 0 ? payload->iconSize : 24.0f;
        float resolvedTooltipIconSize = payload->tooltipIconSize > 0 ? payload->tooltipIconSize : 18.0f;
        SIZE size = TextOverlayMeasure(payload->message, payload->loading, payload->hasIcon, payload->hasTooltip, payload->showCopyButton, payload->closable, resolvedFontSize, resolvedIconSize, resolvedTooltipIconSize, payload->windowWidth, payload->minWindowWidth, payload->maxWindowWidth, payload->windowHeight, payload->maxWindowHeight);
        BOOL sizeChanged = state->contentWidth != size.cx || state->contentHeight != size.cy;
        BOOL visualChanged = sizeChanged ||
                             wcscmp(state->message ? state->message : L"", payload->message) != 0 ||
                             state->loading != payload->loading ||
                             state->closable != payload->closable ||
                             state->centerContent != payload->centerContent ||
                             state->showCopyButton != payload->showCopyButton ||
                             state->iconBitmap != payload->iconBitmap ||
                             state->fontSize != resolvedFontSize ||
                             state->iconSize != resolvedIconSize ||
                             state->tooltipIconSize != resolvedTooltipIconSize ||
                             state->copied || state->closeHover || state->closePressed;

        free(state->message);
        state->message = payload->message;
        payload->message = NULL;
        state->loading = payload->loading;
        state->closable = payload->closable;
        state->centerContent = payload->centerContent;
        state->showCopyButton = payload->showCopyButton;
        if (state->iconBitmap)
            DeleteObject(state->iconBitmap);
        state->iconBitmap = payload->iconBitmap;
        payload->iconBitmap = NULL;
        state->autoCloseSeconds = payload->autoCloseSeconds;
        state->fontSize = resolvedFontSize;
        state->iconSize = resolvedIconSize;
        state->tooltipIconSize = resolvedTooltipIconSize;
        if (visualChanged)
        {
            if (GetCapture() == hwnd)
                ReleaseCapture();
            state->copied = FALSE;
            state->closeHover = FALSE;
            state->closePressed = FALSE;
            state->loadingPhase = 0;
        }

        KillTimer(hwnd, TEXT_OVERLAY_TIMER_COPY_FEEDBACK);
        KillTimer(hwnd, TEXT_OVERLAY_TIMER_AUTOCLOSE);
        KillTimer(hwnd, TEXT_OVERLAY_TIMER_LOADING);
        if (state->loading)
            SetTimer(hwnd, TEXT_OVERLAY_TIMER_LOADING, TEXT_OVERLAY_LOADING_INTERVAL_MS, NULL);
        if (state->autoCloseSeconds > 0)
            SetTimer(hwnd, TEXT_OVERLAY_TIMER_AUTOCLOSE, (UINT)(state->autoCloseSeconds * 1000), NULL);

        state->contentWidth = size.cx;
        state->contentHeight = size.cy;
        payload->contentWidth = size.cx;
        payload->contentHeight = size.cy;
        payload->success = TRUE;
        if (visualChanged && !sizeChanged)
            InvalidateRect(hwnd, NULL, FALSE);
        return 0;
    }
    case WM_SETCURSOR:
    {
        if (!state || LOWORD(lParam) != HTCLIENT)
            break;
        POINT pt;
        if (!GetCursorPos(&pt))
            break;
        ScreenToClient(hwnd, &pt);
        if (state->closable)
        {
            RECT closeRc = TextOverlayCloseButtonRect(state, TextOverlayGetDpi(hwnd));
            if (PtInRect(&closeRc, pt))
            {
                SetCursor(LoadCursor(NULL, IDC_HAND));
                return TRUE;
            }
        }
        break;
    }
    case WM_MOUSEMOVE:
    {
        if (!state)
            break;
        TRACKMOUSEEVENT tme = {sizeof(TRACKMOUSEEVENT), TME_LEAVE, hwnd, 0};
        TrackMouseEvent(&tme);

        POINT pt = {GET_X_LPARAM(lParam), GET_Y_LPARAM(lParam)};
        RECT closeRc = TextOverlayCloseButtonRect(state, TextOverlayGetDpi(hwnd));
        RECT copyRc = TextOverlayCopyButtonRect(state, TextOverlayGetDpi(hwnd));
        BOOL closeHoverNow = state->closable && PtInRect(&closeRc, pt);
        if (closeHoverNow != state->closeHover)
        {
            state->closeHover = closeHoverNow;
            TextOverlayInvalidate(hwnd);
        }
        if (!closeHoverNow && !(state->showCopyButton && PtInRect(&copyRc, pt)))
            TextOverlayForwardMouseMessage(hwnd, msg, wParam, lParam);
        return 0;
    }
    case WM_MOUSELEAVE:
        if (state)
        {
            state->closeHover = FALSE;
            if (!state->closePressed)
                TextOverlayInvalidate(hwnd);
        }
        return 0;
    case WM_LBUTTONDOWN:
        if (!state)
            return 0;
        if (state->closable)
        {
            POINT pt = {GET_X_LPARAM(lParam), GET_Y_LPARAM(lParam)};
            RECT closeRc = TextOverlayCloseButtonRect(state, TextOverlayGetDpi(hwnd));
            if (PtInRect(&closeRc, pt))
            {
                state->closePressed = TRUE;
                SetCapture(hwnd);
                TextOverlayInvalidate(hwnd);
                return 0;
            }
        }
        if (state->showCopyButton)
        {
            POINT pt = {GET_X_LPARAM(lParam), GET_Y_LPARAM(lParam)};
            RECT copyRc = TextOverlayCopyButtonRect(state, TextOverlayGetDpi(hwnd));
            if (PtInRect(&copyRc, pt))
                return 0;
        }
        TextOverlayForwardMouseMessage(hwnd, msg, wParam, lParam);
        return 0;
    case WM_LBUTTONUP:
        if (!state || !state->nameUtf8)
            return 0;
        if (state->closePressed)
        {
            POINT pt = {GET_X_LPARAM(lParam), GET_Y_LPARAM(lParam)};
            RECT closeRc = TextOverlayCloseButtonRect(state, TextOverlayGetDpi(hwnd));
            state->closePressed = FALSE;
            if (GetCapture() == hwnd)
                ReleaseCapture();
            TextOverlayInvalidate(hwnd);
            if (state->closable && PtInRect(&closeRc, pt))
            {
                overlayRequestCloseCallbackCGO(state->nameUtf8);
                return 0;
            }
            return 0;
        }
        if (state->showCopyButton)
        {
            POINT pt = {GET_X_LPARAM(lParam), GET_Y_LPARAM(lParam)};
            RECT copyRc = TextOverlayCopyButtonRect(state, TextOverlayGetDpi(hwnd));
            if (PtInRect(&copyRc, pt))
            {
                if (overlayClickCallbackCGO(state->nameUtf8))
                {
                    state->copied = TRUE;
                    SetTimer(hwnd, TEXT_OVERLAY_TIMER_COPY_FEEDBACK, 1200, NULL);
                    InvalidateRect(hwnd, NULL, FALSE);
                }
                return 0;
            }
        }
        TextOverlayForwardMouseMessage(hwnd, msg, wParam, lParam);
        return 0;
    case WM_MOUSEWHEEL:
        if (TextOverlayForwardMouseMessage(hwnd, msg, wParam, lParam))
            return 0;
        break;
    case WM_TIMER:
        if (wParam == TEXT_OVERLAY_TIMER_COPY_FEEDBACK)
        {
            KillTimer(hwnd, TEXT_OVERLAY_TIMER_COPY_FEEDBACK);
            if (state)
            {
                state->copied = FALSE;
                InvalidateRect(hwnd, NULL, FALSE);
            }
            return 0;
        }
        if (wParam == TEXT_OVERLAY_TIMER_AUTOCLOSE)
        {
            if (!state || !state->nameUtf8)
                return 0;
            if (TextOverlayCursorInsideWindow(hwnd))
            {
                // Text overlays own hover-delayed notification close behavior because the native
                // attachment child window receives the mouse events, not the base overlay window.
                SetTimer(hwnd, TEXT_OVERLAY_TIMER_AUTOCLOSE, TEXT_OVERLAY_AUTOCLOSE_PENDING_MS, NULL);
                return 0;
            }
            KillTimer(hwnd, TEXT_OVERLAY_TIMER_AUTOCLOSE);
            overlayRequestCloseCallbackCGO(state->nameUtf8);
            return 0;
        }
        if (wParam == TEXT_OVERLAY_TIMER_LOADING)
        {
            if (!state || !state->loading)
            {
                KillTimer(hwnd, TEXT_OVERLAY_TIMER_LOADING);
                return 0;
            }
            state->loadingPhase++;
            InvalidateRect(hwnd, NULL, FALSE);
            return 0;
        }
        break;
    case WM_PAINT:
    {
        PAINTSTRUCT ps;
        HDC hdc = BeginPaint(hwnd, &ps);
        if (state)
            TextOverlayPaint(hwnd, hdc, state);
        EndPaint(hwnd, &ps);
        return 0;
    }
    case WM_CLOSE:
        DestroyWindow(hwnd);
        return 0;
    case WM_NCDESTROY:
        KillTimer(hwnd, TEXT_OVERLAY_TIMER_COPY_FEEDBACK);
        KillTimer(hwnd, TEXT_OVERLAY_TIMER_AUTOCLOSE);
        KillTimer(hwnd, TEXT_OVERLAY_TIMER_LOADING);
        if (state)
        {
            if (state->iconBitmap)
                DeleteObject(state->iconBitmap);
            free(state->nameUtf8);
            free(state->message);
            free(state);
            SetWindowLongPtrW(hwnd, GWLP_USERDATA, 0);
        }
        PostQuitMessage(0);
        break;
    }
    return DefWindowProcW(hwnd, msg, wParam, lParam);
}

static BOOL TextOverlayEnsureClass(void)
{
    if (g_textOverlayClass)
        return TRUE;

    WNDCLASSEXW wc;
    ZeroMemory(&wc, sizeof(wc));
    wc.cbSize = sizeof(wc);
    wc.lpfnWndProc = TextOverlayProc;
    wc.hInstance = GetModuleHandleW(NULL);
    wc.hCursor = LoadCursor(NULL, IDC_ARROW);
    wc.lpszClassName = kTextOverlayClassName;
    g_textOverlayClass = RegisterClassExW(&wc);
    return g_textOverlayClass != 0;
}

static DWORD WINAPI TextOverlayThreadProc(LPVOID param)
{
    TextOverlayState *state = (TextOverlayState *)param;
    // WS_EX_TRANSPARENT keeps mouse forwarding to the parent while the child owns the visible
    // painted surface. BufferedPaint (initialized per thread) gives the child the same flicker-free
    // double-buffering the base overlay uses.
    BufferedPaintInit();
    HWND hwnd = CreateWindowExW(WS_EX_NOACTIVATE | WS_EX_TRANSPARENT, kTextOverlayClassName, L"", WS_POPUP, 0, 0, state->contentWidth, state->contentHeight, NULL, NULL, GetModuleHandleW(NULL), state);
    state->hwnd = hwnd;
    state->createOk = hwnd ? TRUE : FALSE;
    SetEvent(state->readyEvent);
    if (!hwnd)
    {
        BufferedPaintUnInit();
        return 0;
    }
    if (state->loading)
        SetTimer(hwnd, TEXT_OVERLAY_TIMER_LOADING, TEXT_OVERLAY_LOADING_INTERVAL_MS, NULL);
    if (state->autoCloseSeconds > 0)
        SetTimer(hwnd, TEXT_OVERLAY_TIMER_AUTOCLOSE, (UINT)(state->autoCloseSeconds * 1000), NULL);

    MSG msg;
    while (GetMessageW(&msg, NULL, 0, 0) > 0)
    {
        TranslateMessage(&msg);
        DispatchMessageW(&msg);
    }
    BufferedPaintUnInit();
    return 0;
}

TextOverlayAttachment TextOverlayCreateWindow(char *name,
                                              char *message,
                                              unsigned char *iconData,
                                              int iconLen,
                                              bool loading,
                                              bool centerContent,
                                              float fontSize,
                                              float iconSize,
                                              char *tooltip,
                                              unsigned char *tooltipIconData,
                                              int tooltipIconLen,
                                              float tooltipIconSize,
                                              bool showCopyButton,
                                              char *copyButtonTooltip,
                                              char *copyButtonSuccessTooltip,
                                              bool closable,
                                              int autoCloseSeconds,
                                              float windowWidth,
                                              float minWindowWidth,
                                              float maxWindowWidth,
                                              float windowHeight,
                                              float maxWindowHeight)
{
    TextOverlayAttachment result = {0};
    (void)tooltip;
    (void)tooltipIconData;
    (void)tooltipIconLen;
    (void)copyButtonTooltip;
    (void)copyButtonSuccessTooltip;
    if (!TextOverlayEnsureClass())
        return result;

    HRESULT coResult = CoInitializeEx(NULL, COINIT_APARTMENTTHREADED);
    HBITMAP iconBitmap = TextOverlayCreateBitmapFromBytes(iconData, iconLen);
    if (SUCCEEDED(coResult))
        CoUninitialize();

    TextOverlayState *state = (TextOverlayState *)calloc(1, sizeof(TextOverlayState));
    if (!state)
    {
        if (iconBitmap)
            DeleteObject(iconBitmap);
        return result;
    }
    state->nameUtf8 = TextOverlayCopyUtf8(name);
    state->message = TextOverlayUtf8ToWide(message);
    state->loading = loading ? TRUE : FALSE;
    state->closable = closable ? TRUE : FALSE;
    state->centerContent = centerContent ? TRUE : FALSE;
    state->showCopyButton = showCopyButton ? TRUE : FALSE;
    state->iconBitmap = iconBitmap;
    state->autoCloseSeconds = autoCloseSeconds;
    state->fontSize = fontSize > 0 ? fontSize : TEXT_OVERLAY_DEFAULT_FONT_SIZE;
    state->iconSize = iconSize > 0 ? iconSize : 24.0f;
    state->tooltipIconSize = tooltipIconSize > 0 ? tooltipIconSize : 18.0f;
    SIZE size = TextOverlayMeasure(state->message, state->loading, iconBitmap != NULL, tooltipIconLen > 0, state->showCopyButton, closable, state->fontSize, state->iconSize, state->tooltipIconSize, windowWidth, minWindowWidth, maxWindowWidth, windowHeight, maxWindowHeight);
    state->contentWidth = size.cx;
    state->contentHeight = size.cy;
    state->readyEvent = CreateEventW(NULL, TRUE, FALSE, NULL);
    if (!state->readyEvent)
    {
        if (state->iconBitmap)
            DeleteObject(state->iconBitmap);
        free(state->nameUtf8);
        free(state->message);
        free(state);
        return result;
    }

    HANDLE thread = CreateThread(NULL, 0, TextOverlayThreadProc, state, 0, NULL);
    if (!thread)
    {
        CloseHandle(state->readyEvent);
        if (state->iconBitmap)
            DeleteObject(state->iconBitmap);
        free(state->nameUtf8);
        free(state->message);
        free(state);
        return result;
    }
    WaitForSingleObject(state->readyEvent, INFINITE);
    CloseHandle(state->readyEvent);
    state->readyEvent = NULL;
    CloseHandle(thread);

    if (!state->createOk || !state->hwnd)
    {
        if (state->iconBitmap)
            DeleteObject(state->iconBitmap);
        free(state->nameUtf8);
        free(state->message);
        free(state);
        return result;
    }

    result.handle = state->hwnd;
    result.width = (float)state->contentWidth;
    result.height = (float)state->contentHeight;
    return result;
}

// TextOverlayUpdateWindow updates an existing renderer without replacing its HWND.
TextOverlayAttachment TextOverlayUpdateWindow(void *rawHwnd,
                                              char *message,
                                              unsigned char *iconData,
                                              int iconLen,
                                              bool loading,
                                              bool centerContent,
                                              float fontSize,
                                              float iconSize,
                                              int tooltipIconLen,
                                              float tooltipIconSize,
                                              bool showCopyButton,
                                              bool closable,
                                              int autoCloseSeconds,
                                              float windowWidth,
                                              float minWindowWidth,
                                              float maxWindowWidth,
                                              float windowHeight,
                                              float maxWindowHeight)
{
    TextOverlayAttachment result = {0};
    HWND hwnd = (HWND)rawHwnd;
    if (!hwnd || !IsWindow(hwnd))
        return result;

    TextOverlayUpdatePayload payload = {0};
    payload.message = TextOverlayUtf8ToWide(message);
    if (!payload.message)
        return result;
    payload.loading = loading ? TRUE : FALSE;
    payload.closable = closable ? TRUE : FALSE;
    payload.centerContent = centerContent ? TRUE : FALSE;
    payload.showCopyButton = showCopyButton ? TRUE : FALSE;
    HRESULT coResult = CoInitializeEx(NULL, COINIT_APARTMENTTHREADED);
    payload.iconBitmap = TextOverlayCreateBitmapFromBytes(iconData, iconLen);
    if (SUCCEEDED(coResult))
        CoUninitialize();
    payload.hasIcon = payload.iconBitmap != NULL;
    payload.hasTooltip = tooltipIconLen > 0 ? TRUE : FALSE;
    payload.autoCloseSeconds = autoCloseSeconds;
    payload.fontSize = fontSize;
    payload.iconSize = iconSize;
    payload.tooltipIconSize = tooltipIconSize;
    payload.windowWidth = windowWidth;
    payload.minWindowWidth = minWindowWidth;
    payload.maxWindowWidth = maxWindowWidth;
    payload.windowHeight = windowHeight;
    payload.maxWindowHeight = maxWindowHeight;

    SendMessageW(hwnd, TEXT_OVERLAY_MESSAGE_UPDATE, 0, (LPARAM)&payload);
    if (payload.message)
        free(payload.message);
    if (!payload.success)
    {
        if (payload.iconBitmap)
            DeleteObject(payload.iconBitmap);
        return result;
    }

    result.handle = hwnd;
    result.width = (float)payload.contentWidth;
    result.height = (float)payload.contentHeight;
    return result;
}

void TextOverlayDestroyWindow(void *rawHwnd)
{
    HWND hwnd = (HWND)rawHwnd;
    if (hwnd && IsWindow(hwnd))
        PostMessageW(hwnd, WM_CLOSE, 0, 0);
}
