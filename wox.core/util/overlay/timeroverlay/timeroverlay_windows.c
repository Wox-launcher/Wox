#include <windows.h>
#include <windowsx.h>
#include <uxtheme.h>
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

#define TIMER_OVERLAY_MESSAGE_UPDATE (WM_APP + 1)
#define TIMER_OVERLAY_TIMER_DRAG_WATCH 1
#define TIMER_COUNTDOWN_DEFAULT_FONT 22.0f
#define TIMER_NOTE_DEFAULT_FONT 11.0f
#define TIMER_LINE_GAP_DIP 8
#define TIMER_DETAIL_PAD_V_DIP 2
#define TIMER_CLOSE_SIZE_DIP 16
#define TIMER_CLOSE_GAP_DIP 8
#define TIMER_CLOSE_INSET_DIP 5

typedef struct {
    void *handle;
    float width;
    float height;
} TimerOverlayAttachment;

extern void overlayRequestCloseCallbackCGO(char *name);

typedef struct {
    HWND hwnd;
    HANDLE readyEvent;
    BOOL createOk;
    char *nameUtf8;
    WCHAR *countdown;
    WCHAR *note;
    BOOL closable;
    BOOL overlayHover;
    BOOL keepExpandedForDrag;
    BOOL closeHover;
    BOOL closePressed;
    HFONT countdownFont;
    HFONT noteFont;
    UINT cachedFontDpi;
    float cachedCountdownFontSize;
    float cachedNoteFontSize;
    RECT closeRect;
    float countdownFontSize;
    float noteFontSize;
    float windowWidth;
    float minWindowWidth;
    float maxWindowWidth;
    float windowHeight;
    float maxWindowHeight;
    int contentWidth;
    int contentHeight;
} TimerOverlayState;

typedef struct {
    WCHAR *countdown;
    WCHAR *note;
    BOOL closable;
    float countdownFontSize;
    float noteFontSize;
    float windowWidth;
    float minWindowWidth;
    float maxWindowWidth;
    float windowHeight;
    float maxWindowHeight;
    int contentWidth;
    int contentHeight;
    BOOL success;
} TimerOverlayUpdatePayload;

static const wchar_t *kTimerOverlayClassName = L"WoxTimerOverlayAttachmentWindow";
static ATOM g_timerOverlayClass = 0;

static char *TimerOverlayCopyUtf8(const char *text)
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

static WCHAR *TimerOverlayUtf8ToWide(const char *text)
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

static UINT TimerOverlayGetDpi(HWND hwnd)
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

static int TimerOverlayDip(float value, UINT dpi)
{
    return MulDiv((int)(value + 0.5f), (int)dpi, 96);
}

static HFONT TimerOverlayCreateFont(float fontSize, UINT dpi, int weight)
{
    float resolvedSize = fontSize > 0 ? fontSize : TIMER_COUNTDOWN_DEFAULT_FONT;
    int height = -MulDiv((int)(resolvedSize + 0.5f), (int)dpi, 72);
    return CreateFontW(height, 0, 0, 0, weight, FALSE, FALSE, FALSE, DEFAULT_CHARSET, OUT_DEFAULT_PRECIS, CLIP_DEFAULT_PRECIS, CLEARTYPE_QUALITY, DEFAULT_PITCH | FF_SWISS, L"Segoe UI");
}

static SIZE TimerOverlayMeasureText(WCHAR *text, int textWidth, float fontSize, int weight)
{
    HDC hdc = GetDC(NULL);
    HFONT font = TimerOverlayCreateFont(fontSize, 96, weight);
    HGDIOBJ oldFont = SelectObject(hdc, font);
    RECT rc = {0, 0, textWidth > 0 ? textWidth : 1, 1};
    DrawTextW(hdc, text ? text : L"", -1, &rc, DT_CALCRECT | DT_CENTER | DT_WORDBREAK | DT_NOPREFIX);
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

static BOOL TimerOverlayHasNote(WCHAR *note)
{
    return note && note[0] != L'\0';
}

static BOOL TimerOverlayDetailsVisible(TimerOverlayState *state)
{
    return state && (state->overlayHover || state->keepExpandedForDrag);
}

static BOOL TimerOverlayCloseInteractive(TimerOverlayState *state)
{
    return state && state->closable && TimerOverlayDetailsVisible(state);
}

static SIZE TimerOverlayMeasure(WCHAR *countdown, WCHAR *note, BOOL closable, BOOL detailsVisible, float countdownFont, float noteFont, float windowWidth, float minWindowWidth, float maxWindowWidth, float windowHeight, float maxWindowHeight);

static void TimerOverlayReleaseFonts(TimerOverlayState *state)
{
    if (!state)
        return;
    if (state->countdownFont)
    {
        DeleteObject(state->countdownFont);
        state->countdownFont = NULL;
    }
    if (state->noteFont)
    {
        DeleteObject(state->noteFont);
        state->noteFont = NULL;
    }
    state->cachedFontDpi = 0;
}

static void TimerOverlayEnsureFonts(TimerOverlayState *state, UINT dpi)
{
    if (!state)
        return;
    if (state->countdownFont && state->noteFont && state->cachedFontDpi == dpi &&
        state->cachedCountdownFontSize == state->countdownFontSize &&
        state->cachedNoteFontSize == state->noteFontSize)
        return;

    TimerOverlayReleaseFonts(state);
    state->countdownFont = TimerOverlayCreateFont(state->countdownFontSize, dpi, FW_SEMIBOLD);
    state->noteFont = TimerOverlayCreateFont(state->noteFontSize, dpi, FW_NORMAL);
    state->cachedFontDpi = dpi;
    state->cachedCountdownFontSize = state->countdownFontSize;
    state->cachedNoteFontSize = state->noteFontSize;
}

// detailsVisible controls compact vs hover layout: compact is countdown-only; hover adds note + close.
static SIZE TimerOverlayMeasure(WCHAR *countdown, WCHAR *note, BOOL closable, BOOL detailsVisible, float countdownFont, float noteFont, float windowWidth, float minWindowWidth, float maxWindowWidth, float windowHeight, float maxWindowHeight)
{
    BOOL showClose = detailsVisible && closable;
    BOOL showNote = detailsVisible && TimerOverlayHasNote(note);
    // Symmetric side padding keeps countdown/note optically centered while leaving room for the close control.
    int sidePad = showClose ? TIMER_CLOSE_SIZE_DIP + TIMER_CLOSE_GAP_DIP : 0;
    int chromeWidth = 36;
    int chromeHeight = 24;

    SIZE countdownNatural = TimerOverlayMeasureText(countdown, 4096, countdownFont, FW_SEMIBOLD);
    SIZE noteNatural = showNote ? TimerOverlayMeasureText(note, 4096, noteFont, FW_NORMAL) : (SIZE){0, 0};
    int natural = countdownNatural.cx > noteNatural.cx ? countdownNatural.cx : noteNatural.cx;
    natural += sidePad * 2;

    int maxContentWidth = maxWindowWidth > 0 ? (int)maxWindowWidth - chromeWidth : 280;
    if (maxContentWidth < 1)
        maxContentWidth = 1;
    int minContentWidth = minWindowWidth > 0 ? (int)minWindowWidth - chromeWidth : (detailsVisible ? 96 : 1);
    if (minContentWidth < 1)
        minContentWidth = 1;

    int contentWidth = natural;
    if (contentWidth < minContentWidth)
        contentWidth = minContentWidth;
    if (contentWidth > maxContentWidth)
        contentWidth = maxContentWidth;
    if (windowWidth > 0)
        contentWidth = (int)windowWidth - chromeWidth;
    if (contentWidth < 1)
        contentWidth = 1;

    int textWidth = contentWidth - sidePad * 2;
    if (textWidth < 1)
        textWidth = 1;

    SIZE countdownWrapped = TimerOverlayMeasureText(countdown, textWidth, countdownFont, FW_SEMIBOLD);
    SIZE noteWrapped = showNote ? TimerOverlayMeasureText(note, textWidth, noteFont, FW_NORMAL) : (SIZE){0, 0};
    int contentHeight = countdownWrapped.cy;
    if (showNote)
        contentHeight += TIMER_LINE_GAP_DIP + noteWrapped.cy;
    if (detailsVisible)
        contentHeight += TIMER_DETAIL_PAD_V_DIP * 2;

    if (windowHeight > 0)
        contentHeight = (int)windowHeight - chromeHeight;
    else if (maxWindowHeight > 0 && contentHeight > (int)maxWindowHeight - chromeHeight)
        contentHeight = (int)maxWindowHeight - chromeHeight;
    if (contentHeight < 1)
        contentHeight = 1;

    SIZE result = {contentWidth, contentHeight};
    return result;
}

static RECT TimerOverlayClientRect(TimerOverlayState *state)
{
    RECT client = {0, 0, state ? state->contentWidth : 0, state ? state->contentHeight : 0};
    RECT measured;
    if (state && state->hwnd && GetClientRect(state->hwnd, &measured) && measured.right > 0 && measured.bottom > 0)
        client = measured;
    return client;
}

static RECT TimerOverlayCloseButtonRect(TimerOverlayState *state, UINT dpi)
{
    int size = TimerOverlayDip(TIMER_CLOSE_SIZE_DIP, dpi);
    int inset = TimerOverlayDip(TIMER_CLOSE_INSET_DIP, dpi);
    RECT client = TimerOverlayClientRect(state);
    // Inset is relative to the visible HUD outer edge (child fills the parent client).
    RECT rc = {client.right - size - inset, inset, client.right - inset, inset + size};
    return rc;
}

// Timer overlay paints on a transparent child; fill the parent client so chrome like the close
// control can sit on the visible HUD edge instead of the inner content pad.
static void TimerOverlayEnsureChildFillsParent(TimerOverlayState *state)
{
    if (!state || !state->hwnd)
        return;
    HWND parent = GetParent(state->hwnd);
    if (!parent)
        return;

    RECT parentClient;
    if (!GetClientRect(parent, &parentClient))
        return;
    int parentW = parentClient.right - parentClient.left;
    int parentH = parentClient.bottom - parentClient.top;
    if (parentW < 1 || parentH < 1)
        return;

    RECT childClient;
    GetClientRect(state->hwnd, &childClient);
    POINT childOrigin = {0, 0};
    MapWindowPoints(state->hwnd, parent, &childOrigin, 1);
    if (childOrigin.x == 0 && childOrigin.y == 0 &&
        childClient.right - childClient.left == parentW &&
        childClient.bottom - childClient.top == parentH)
        return;

    SetWindowPos(state->hwnd, NULL, 0, 0, parentW, parentH, SWP_NOACTIVATE | SWP_NOZORDER | SWP_SHOWWINDOW);
}

static BOOL TimerOverlayCursorOverOverlay(TimerOverlayState *state)
{
    if (!state || !state->hwnd)
        return FALSE;
    HWND target = GetParent(state->hwnd);
    if (!target)
        target = state->hwnd;
    POINT pt;
    RECT rc;
    if (!GetCursorPos(&pt) || !GetWindowRect(target, &rc))
        return FALSE;
    return PtInRect(&rc, pt);
}

static BOOL TimerOverlayIsLeftButtonDown(void)
{
    return (GetAsyncKeyState(VK_LBUTTON) & 0x8000) != 0;
}

static BOOL TimerOverlayIsInteracting(TimerOverlayState *state)
{
    if (!state || !state->hwnd)
        return FALSE;
    if (state->closePressed || TimerOverlayIsLeftButtonDown())
        return TRUE;
    HWND capture = GetCapture();
    HWND parent = GetParent(state->hwnd);
    return capture == state->hwnd || (parent && capture == parent);
}

static BOOL TimerOverlayForwardMouseMessage(HWND hwnd, UINT msg, WPARAM wParam, LPARAM lParam)
{
    HWND parent = GetParent(hwnd);
    if (!parent)
        return FALSE;
    LPARAM forwardedLParam = lParam;
    POINT pt = {GET_X_LPARAM(lParam), GET_Y_LPARAM(lParam)};
    MapWindowPoints(hwnd, parent, &pt, 1);
    forwardedLParam = MAKELPARAM(pt.x, pt.y);
    SendMessageW(parent, msg, wParam, forwardedLParam);
    return TRUE;
}

// TimerOverlaySyncDetailsLayout expands/collapses the attachment and parent HUD around the
// current bottom-center so hover details do not jump the countdown away from the cursor.
static void TimerOverlaySyncDetailsLayout(TimerOverlayState *state)
{
    if (!state || !state->hwnd)
        return;

    BOOL detailsVisible = TimerOverlayDetailsVisible(state);
    SIZE size = TimerOverlayMeasure(state->countdown, state->note, state->closable, detailsVisible, state->countdownFontSize, state->noteFontSize, state->windowWidth, state->minWindowWidth, state->maxWindowWidth, state->windowHeight, state->maxWindowHeight);
    BOOL sizeChanged = state->contentWidth != size.cx || state->contentHeight != size.cy;
    state->contentWidth = size.cx;
    state->contentHeight = size.cy;

    UINT dpi = TimerOverlayGetDpi(state->hwnd);
    int contentPhysicalW = TimerOverlayDip((float)size.cx, dpi);
    int contentPhysicalH = TimerOverlayDip((float)size.cy, dpi);
    if (contentPhysicalW < 1)
        contentPhysicalW = 1;
    if (contentPhysicalH < 1)
        contentPhysicalH = 1;

    // Match non-transparent HUD chrome used by the base overlay layout.
    int chromeW = TimerOverlayDip(36, dpi);
    int chromeH = TimerOverlayDip(24, dpi);
    int parentW = contentPhysicalW + chromeW;
    int parentH = contentPhysicalH + chromeH;
    if (parentW < 1)
        parentW = 1;
    if (parentH < 1)
        parentH = 1;

    HWND parent = GetParent(state->hwnd);
    if (parent && sizeChanged)
    {
        RECT parentRect;
        if (GetWindowRect(parent, &parentRect))
        {
            int centerX = (parentRect.left + parentRect.right) / 2;
            int bottom = parentRect.bottom;
            int x = centerX - parentW / 2;
            int y = bottom - parentH;
            SetWindowPos(parent, NULL, x, y, parentW, parentH, SWP_NOACTIVATE | SWP_NOZORDER);
        }
    }

    // Fill the HUD client so the close control's 5px inset is against the visible outer edge.
    if (parent)
        SetWindowPos(state->hwnd, NULL, 0, 0, parentW, parentH, SWP_NOACTIVATE | SWP_NOZORDER | SWP_SHOWWINDOW);
    else
        SetWindowPos(state->hwnd, NULL, 0, 0, contentPhysicalW, contentPhysicalH, SWP_NOACTIVATE | SWP_NOZORDER | SWP_SHOWWINDOW);
    InvalidateRect(state->hwnd, NULL, FALSE);
    if (parent)
        InvalidateRect(parent, NULL, FALSE);
}

// TimerOverlayDrawTextAlpha avoids ClearType color fringes on the transparent HUD attachment.
static void TimerOverlayDrawTextAlpha(HDC hdc, HFONT font, WCHAR *text, RECT rc, UINT flags, COLORREF color, BYTE opacity)
{
    int width = rc.right - rc.left;
    int height = rc.bottom - rc.top;
    if (width <= 0 || height <= 0 || opacity == 0)
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
        alpha = (BYTE)((int)alpha * opacity / 255);
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

static void TimerOverlayDraw(HDC hdc, RECT rc, TimerOverlayState *state)
{
    UINT dpi = TimerOverlayGetDpi(state->hwnd);
    SetBkMode(hdc, TRANSPARENT);
    TimerOverlayEnsureFonts(state, dpi);

    BOOL detailsVisible = TimerOverlayDetailsVisible(state);
    BOOL showClose = state->closable && detailsVisible;
    BOOL showNote = detailsVisible && TimerOverlayHasNote(state->note);
    int lineGap = TimerOverlayDip(TIMER_LINE_GAP_DIP, dpi);
    int detailPad = detailsVisible ? TimerOverlayDip(TIMER_DETAIL_PAD_V_DIP, dpi) : 0;
    int sidePad = showClose ? TimerOverlayDip(TIMER_CLOSE_SIZE_DIP + TIMER_CLOSE_GAP_DIP, dpi) : 0;
    // Symmetric insets keep text optically centered while clearing the top-right close control.
    int textLeft = sidePad;
    int textRight = (rc.right - rc.left) - sidePad;
    int textAreaWidth = textRight - textLeft;
    if (textAreaWidth < 1)
        textAreaWidth = 1;

    HFONT countdownFont = state->countdownFont;
    HFONT noteFont = state->noteFont;
    if (!countdownFont || !noteFont)
        return;

    RECT measure = {0, 0, textAreaWidth, 1};
    HGDIOBJ oldFont = SelectObject(hdc, countdownFont);
    DrawTextW(hdc, state->countdown ? state->countdown : L"", -1, &measure, DT_CALCRECT | DT_CENTER | DT_WORDBREAK | DT_NOPREFIX);
    int countdownHeight = measure.bottom - measure.top;
    if (countdownHeight < 1)
        countdownHeight = 1;

    int noteHeight = 0;
    if (showNote)
    {
        SelectObject(hdc, noteFont);
        measure.left = 0;
        measure.top = 0;
        measure.right = textAreaWidth;
        measure.bottom = 1;
        DrawTextW(hdc, state->note, -1, &measure, DT_CALCRECT | DT_CENTER | DT_WORDBREAK | DT_NOPREFIX);
        noteHeight = measure.bottom - measure.top;
        if (noteHeight < 1)
            noteHeight = 1;
    }

    int blockHeight = countdownHeight + (showNote ? lineGap + noteHeight : 0);
    int y = detailPad + ((rc.bottom - rc.top - detailPad * 2) - blockHeight) / 2;
    if (y < detailPad)
        y = detailPad;

    RECT countdownRc = {textLeft, y, textRight, y + countdownHeight};
    TimerOverlayDrawTextAlpha(hdc, countdownFont, state->countdown, countdownRc, DT_CENTER | DT_WORDBREAK | DT_NOPREFIX, RGB(245, 245, 245), 255);

    if (showNote)
    {
        RECT noteRc = {textLeft, y + countdownHeight + lineGap, textRight, y + countdownHeight + lineGap + noteHeight};
        TimerOverlayDrawTextAlpha(hdc, noteFont, state->note, noteRc, DT_CENTER | DT_WORDBREAK | DT_NOPREFIX, RGB(170, 170, 170), 255);
    }

    if (showClose)
    {
        RECT closeRc = TimerOverlayCloseButtonRect(state, dpi);
        state->closeRect = closeRc;
        if (state->closeHover || state->closePressed)
        {
            COLORREF bg = state->closePressed ? RGB(70, 70, 70) : RGB(55, 55, 55);
            HBRUSH brush = CreateSolidBrush(bg);
            FillRect(hdc, &closeRc, brush);
            DeleteObject(brush);
        }

        int pad = TimerOverlayDip(4, dpi);
        int thickness = TimerOverlayDip(2, dpi);
        if (thickness < 1)
            thickness = 1;
        HPEN pen = CreatePen(PS_SOLID, thickness, RGB(210, 210, 210));
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

    if (oldFont)
        SelectObject(hdc, oldFont);
}

static void TimerOverlayPaint(HWND hwnd, HDC paintHdc, TimerOverlayState *state)
{
    if (state)
        TimerOverlayEnsureChildFillsParent(state);

    RECT rc;
    GetClientRect(hwnd, &rc);

    HDC hdc = paintHdc;
    HPAINTBUFFER paintBuf = BeginBufferedPaint(paintHdc, &rc, BPBF_TOPDOWNDIB, NULL, &hdc);
    if (paintBuf)
        BufferedPaintClear(paintBuf, &rc);

    if (state)
        TimerOverlayDraw(hdc, rc, state);

    if (paintBuf)
        EndBufferedPaint(paintBuf, TRUE);
}

static LRESULT CALLBACK TimerOverlayProc(HWND hwnd, UINT msg, WPARAM wParam, LPARAM lParam)
{
    TimerOverlayState *state = (TimerOverlayState *)GetWindowLongPtrW(hwnd, GWLP_USERDATA);
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
    case TIMER_OVERLAY_MESSAGE_UPDATE:
    {
        TimerOverlayUpdatePayload *payload = (TimerOverlayUpdatePayload *)lParam;
        if (!state || !payload || !payload->countdown)
            return 0;

        float countdownFont = payload->countdownFontSize > 0 ? payload->countdownFontSize : TIMER_COUNTDOWN_DEFAULT_FONT;
        float noteFont = payload->noteFontSize > 0 ? payload->noteFontSize : TIMER_NOTE_DEFAULT_FONT;
        BOOL visualChanged =
            wcscmp(state->countdown ? state->countdown : L"", payload->countdown) != 0 ||
            wcscmp(state->note ? state->note : L"", payload->note ? payload->note : L"") != 0 ||
            state->closable != payload->closable ||
            state->countdownFontSize != countdownFont ||
            state->noteFontSize != noteFont ||
            state->windowWidth != payload->windowWidth ||
            state->minWindowWidth != payload->minWindowWidth ||
            state->maxWindowWidth != payload->maxWindowWidth ||
            state->windowHeight != payload->windowHeight ||
            state->maxWindowHeight != payload->maxWindowHeight;

        free(state->countdown);
        free(state->note);
        state->countdown = payload->countdown;
        state->note = payload->note;
        payload->countdown = NULL;
        payload->note = NULL;
        state->closable = payload->closable;
        state->countdownFontSize = countdownFont;
        state->noteFontSize = noteFont;
        state->windowWidth = payload->windowWidth;
        state->minWindowWidth = payload->minWindowWidth;
        state->maxWindowWidth = payload->maxWindowWidth;
        state->windowHeight = payload->windowHeight;
        state->maxWindowHeight = payload->maxWindowHeight;
        if (visualChanged)
        {
            if (GetCapture() == hwnd)
                ReleaseCapture();
            state->closeHover = FALSE;
            state->closePressed = FALSE;
        }

        SIZE size = TimerOverlayMeasure(state->countdown, state->note, state->closable, TimerOverlayDetailsVisible(state), countdownFont, noteFont, payload->windowWidth, payload->minWindowWidth, payload->maxWindowWidth, payload->windowHeight, payload->maxWindowHeight);
        BOOL sizeChanged = state->contentWidth != size.cx || state->contentHeight != size.cy;
        state->contentWidth = size.cx;
        state->contentHeight = size.cy;
        payload->contentWidth = size.cx;
        payload->contentHeight = size.cy;
        payload->success = TRUE;
        if (visualChanged || sizeChanged)
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
        if (state->closable && TimerOverlayCloseInteractive(state))
        {
            RECT closeRc = TimerOverlayCloseButtonRect(state, TimerOverlayGetDpi(hwnd));
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

        BOOL entered = FALSE;
        if (!state->overlayHover)
        {
            state->overlayHover = TRUE;
            state->keepExpandedForDrag = FALSE;
            KillTimer(hwnd, TIMER_OVERLAY_TIMER_DRAG_WATCH);
            entered = TRUE;
            TimerOverlaySyncDetailsLayout(state);
        }

        POINT pt = {GET_X_LPARAM(lParam), GET_Y_LPARAM(lParam)};
        RECT closeRc = TimerOverlayCloseButtonRect(state, TimerOverlayGetDpi(hwnd));
        BOOL closeHoverNow = TimerOverlayCloseInteractive(state) && PtInRect(&closeRc, pt);
        if (closeHoverNow != state->closeHover)
        {
            state->closeHover = closeHoverNow;
            InvalidateRect(hwnd, NULL, FALSE);
        }
        else if (entered)
        {
            InvalidateRect(hwnd, NULL, FALSE);
        }
        if (!closeHoverNow)
            TimerOverlayForwardMouseMessage(hwnd, msg, wParam, lParam);
        return 0;
    }
    case WM_MOUSELEAVE:
        if (state)
        {
            state->overlayHover = FALSE;
            state->closeHover = FALSE;
            // Dragging captures the parent; child gets MOUSELEAVE even though the pointer is still over the HUD.
            if (TimerOverlayIsInteracting(state))
            {
                if (!state->keepExpandedForDrag)
                {
                    state->keepExpandedForDrag = TRUE;
                    SetTimer(hwnd, TIMER_OVERLAY_TIMER_DRAG_WATCH, 50, NULL);
                }
                // Keep the expanded target while drag capture owns the cursor.
                TimerOverlaySyncDetailsLayout(state);
                return 0;
            }
            state->keepExpandedForDrag = FALSE;
            KillTimer(hwnd, TIMER_OVERLAY_TIMER_DRAG_WATCH);
            if (!state->closePressed)
                TimerOverlaySyncDetailsLayout(state);
        }
        return 0;
    case WM_TIMER:
        if (state && wParam == TIMER_OVERLAY_TIMER_DRAG_WATCH)
        {
            if (TimerOverlayIsInteracting(state))
                return 0;
            KillTimer(hwnd, TIMER_OVERLAY_TIMER_DRAG_WATCH);
            state->keepExpandedForDrag = FALSE;
            // Collapse only after the drag ends and the cursor has left the overlay.
            if (!TimerOverlayCursorOverOverlay(state))
            {
                state->overlayHover = FALSE;
                state->closeHover = FALSE;
                TimerOverlaySyncDetailsLayout(state);
            }
            else
            {
                state->overlayHover = TRUE;
                TimerOverlaySyncDetailsLayout(state);
            }
            return 0;
        }
        break;
    case WM_LBUTTONDOWN:
        if (!state)
            return 0;
        if (TimerOverlayCloseInteractive(state))
        {
            POINT pt = {GET_X_LPARAM(lParam), GET_Y_LPARAM(lParam)};
            RECT closeRc = TimerOverlayCloseButtonRect(state, TimerOverlayGetDpi(hwnd));
            if (PtInRect(&closeRc, pt))
            {
                state->closePressed = TRUE;
                SetCapture(hwnd);
                InvalidateRect(hwnd, NULL, FALSE);
                return 0;
            }
        }
        // Keep expanded while the parent HUD owns the drag capture.
        state->keepExpandedForDrag = TRUE;
        SetTimer(hwnd, TIMER_OVERLAY_TIMER_DRAG_WATCH, 50, NULL);
        TimerOverlaySyncDetailsLayout(state);
        TimerOverlayForwardMouseMessage(hwnd, msg, wParam, lParam);
        return 0;
    case WM_LBUTTONUP:
        if (!state || !state->nameUtf8)
            return 0;
        if (state->closePressed)
        {
            POINT pt = {GET_X_LPARAM(lParam), GET_Y_LPARAM(lParam)};
            RECT closeRc = TimerOverlayCloseButtonRect(state, TimerOverlayGetDpi(hwnd));
            state->closePressed = FALSE;
            if (GetCapture() == hwnd)
                ReleaseCapture();
            InvalidateRect(hwnd, NULL, FALSE);
            if (state->closable && PtInRect(&closeRc, pt))
            {
                overlayRequestCloseCallbackCGO(state->nameUtf8);
                return 0;
            }
            return 0;
        }
        TimerOverlayForwardMouseMessage(hwnd, msg, wParam, lParam);
        return 0;
    case WM_PAINT:
    {
        PAINTSTRUCT ps;
        HDC hdc = BeginPaint(hwnd, &ps);
        if (state)
            TimerOverlayPaint(hwnd, hdc, state);
        EndPaint(hwnd, &ps);
        return 0;
    }
    case WM_CLOSE:
        DestroyWindow(hwnd);
        return 0;
    case WM_NCDESTROY:
        if (state)
        {
            KillTimer(hwnd, TIMER_OVERLAY_TIMER_DRAG_WATCH);
            TimerOverlayReleaseFonts(state);
            free(state->nameUtf8);
            free(state->countdown);
            free(state->note);
            free(state);
            SetWindowLongPtrW(hwnd, GWLP_USERDATA, 0);
        }
        PostQuitMessage(0);
        break;
    }
    return DefWindowProcW(hwnd, msg, wParam, lParam);
}

static BOOL TimerOverlayEnsureClass(void)
{
    if (g_timerOverlayClass)
        return TRUE;

    WNDCLASSEXW wc;
    ZeroMemory(&wc, sizeof(wc));
    wc.cbSize = sizeof(wc);
    wc.lpfnWndProc = TimerOverlayProc;
    wc.hInstance = GetModuleHandleW(NULL);
    wc.hCursor = LoadCursor(NULL, IDC_ARROW);
    wc.lpszClassName = kTimerOverlayClassName;
    g_timerOverlayClass = RegisterClassExW(&wc);
    return g_timerOverlayClass != 0;
}

static DWORD WINAPI TimerOverlayThreadProc(LPVOID param)
{
    TimerOverlayState *state = (TimerOverlayState *)param;
    BufferedPaintInit();
    HWND hwnd = CreateWindowExW(WS_EX_NOACTIVATE | WS_EX_TRANSPARENT, kTimerOverlayClassName, L"", WS_POPUP, 0, 0, state->contentWidth, state->contentHeight, NULL, NULL, GetModuleHandleW(NULL), state);
    state->hwnd = hwnd;
    state->createOk = hwnd ? TRUE : FALSE;
    SetEvent(state->readyEvent);
    if (!hwnd)
    {
        BufferedPaintUnInit();
        return 0;
    }

    MSG msg;
    while (GetMessageW(&msg, NULL, 0, 0) > 0)
    {
        TranslateMessage(&msg);
        DispatchMessageW(&msg);
    }
    BufferedPaintUnInit();
    return 0;
}

TimerOverlayAttachment TimerOverlayCreateWindow(char *name,
                                                char *countdown,
                                                char *note,
                                                bool closable,
                                                float countdownFontSize,
                                                float noteFontSize,
                                                float windowWidth,
                                                float minWindowWidth,
                                                float maxWindowWidth,
                                                float windowHeight,
                                                float maxWindowHeight)
{
    TimerOverlayAttachment result = {0};
    if (!TimerOverlayEnsureClass())
        return result;

    TimerOverlayState *state = (TimerOverlayState *)calloc(1, sizeof(TimerOverlayState));
    if (!state)
        return result;
    state->nameUtf8 = TimerOverlayCopyUtf8(name);
    state->countdown = TimerOverlayUtf8ToWide(countdown);
    state->note = TimerOverlayUtf8ToWide(note);
    state->closable = closable ? TRUE : FALSE;
    state->countdownFontSize = countdownFontSize > 0 ? countdownFontSize : TIMER_COUNTDOWN_DEFAULT_FONT;
    state->noteFontSize = noteFontSize > 0 ? noteFontSize : TIMER_NOTE_DEFAULT_FONT;
    state->windowWidth = windowWidth;
    state->minWindowWidth = minWindowWidth;
    state->maxWindowWidth = maxWindowWidth;
    state->windowHeight = windowHeight;
    state->maxWindowHeight = maxWindowHeight;
    // Initial layout is always compact; hover expands to note + close.
    SIZE size = TimerOverlayMeasure(state->countdown, state->note, state->closable, FALSE, state->countdownFontSize, state->noteFontSize, windowWidth, minWindowWidth, maxWindowWidth, windowHeight, maxWindowHeight);
    state->contentWidth = size.cx;
    state->contentHeight = size.cy;
    state->readyEvent = CreateEventW(NULL, TRUE, FALSE, NULL);
    if (!state->readyEvent)
    {
        free(state->nameUtf8);
        free(state->countdown);
        free(state->note);
        free(state);
        return result;
    }

    HANDLE thread = CreateThread(NULL, 0, TimerOverlayThreadProc, state, 0, NULL);
    if (!thread)
    {
        CloseHandle(state->readyEvent);
        free(state->nameUtf8);
        free(state->countdown);
        free(state->note);
        free(state);
        return result;
    }
    WaitForSingleObject(state->readyEvent, INFINITE);
    CloseHandle(state->readyEvent);
    state->readyEvent = NULL;
    CloseHandle(thread);

    if (!state->createOk || !state->hwnd)
    {
        free(state->nameUtf8);
        free(state->countdown);
        free(state->note);
        free(state);
        return result;
    }

    result.handle = state->hwnd;
    result.width = (float)state->contentWidth;
    result.height = (float)state->contentHeight;
    return result;
}

TimerOverlayAttachment TimerOverlayUpdateWindow(void *rawHwnd,
                                                char *countdown,
                                                char *note,
                                                bool closable,
                                                float countdownFontSize,
                                                float noteFontSize,
                                                float windowWidth,
                                                float minWindowWidth,
                                                float maxWindowWidth,
                                                float windowHeight,
                                                float maxWindowHeight)
{
    TimerOverlayAttachment result = {0};
    HWND hwnd = (HWND)rawHwnd;
    if (!hwnd || !IsWindow(hwnd))
        return result;

    TimerOverlayUpdatePayload payload = {0};
    payload.countdown = TimerOverlayUtf8ToWide(countdown);
    payload.note = TimerOverlayUtf8ToWide(note);
    if (!payload.countdown)
    {
        free(payload.note);
        return result;
    }
    payload.closable = closable ? TRUE : FALSE;
    payload.countdownFontSize = countdownFontSize;
    payload.noteFontSize = noteFontSize;
    payload.windowWidth = windowWidth;
    payload.minWindowWidth = minWindowWidth;
    payload.maxWindowWidth = maxWindowWidth;
    payload.windowHeight = windowHeight;
    payload.maxWindowHeight = maxWindowHeight;

    SendMessageW(hwnd, TIMER_OVERLAY_MESSAGE_UPDATE, 0, (LPARAM)&payload);
    if (payload.countdown)
        free(payload.countdown);
    if (payload.note)
        free(payload.note);
    if (!payload.success)
        return result;

    result.handle = hwnd;
    result.width = (float)payload.contentWidth;
    result.height = (float)payload.contentHeight;
    return result;
}

void TimerOverlayDestroyWindow(void *rawHwnd)
{
    HWND hwnd = (HWND)rawHwnd;
    if (hwnd && IsWindow(hwnd))
        PostMessageW(hwnd, WM_CLOSE, 0, 0);
}
