#include <windows.h>
#include <windowsx.h>
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

#define TEXT_OVERLAY_TIMER_COPY_FEEDBACK 1
#define TEXT_OVERLAY_TIMER_AUTOCLOSE 2
#define TEXT_OVERLAY_AUTOCLOSE_PENDING_MS 250
#define TEXT_OVERLAY_COPY_SIZE_DIP 28
#define TEXT_OVERLAY_COPY_GAP_DIP 8
#define TEXT_OVERLAY_CLOSE_SIZE_DIP 20
#define TEXT_OVERLAY_CLOSE_GAP_DIP 8

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
    int autoCloseSeconds;
    float fontSize;
    float iconSize;
    float tooltipIconSize;
    int contentWidth;
    int contentHeight;
} TextOverlayState;

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
    float resolvedSize = fontSize > 0 ? fontSize : 13.0f;
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
    if (contentWidth < 64)
        contentWidth = 64;
    if (contentWidth > 364)
        contentWidth = 364;

    if (windowWidth > 0)
        contentWidth = (int)windowWidth - chromeWidth;
    else if (maxWindowWidth > 0 && contentWidth > (int)maxWindowWidth - chromeWidth)
        contentWidth = (int)maxWindowWidth - chromeWidth;
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

static RECT TextOverlayCopyButtonRect(TextOverlayState *state, UINT dpi)
{
    int size = TextOverlayDip(TEXT_OVERLAY_COPY_SIZE_DIP, dpi);
    RECT rc = {state->contentWidth - size, state->contentHeight - size, state->contentWidth, state->contentHeight};
    return rc;
}

static RECT TextOverlayCloseButtonRect(TextOverlayState *state, UINT dpi)
{
    int size = TextOverlayDip(TEXT_OVERLAY_CLOSE_SIZE_DIP, dpi);
    RECT rc = {state->contentWidth - size, 0, state->contentWidth, size};
    return rc;
}

// TextOverlayInvalidate repaints the parent backdrop before this transparent child redraws.
static void TextOverlayInvalidate(HWND hwnd)
{
    HWND parent = GetParent(hwnd);
    if (parent)
        InvalidateRect(parent, NULL, FALSE);
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

static void TextOverlayDraw(HDC hdc, RECT rc, TextOverlayState *state)
{
    UINT dpi = TextOverlayGetDpi(state->hwnd);
    // The base overlay owns the HUD background; the child paints only foreground
    // content so it does not leave an opaque rectangle over acrylic/backdrop.
    SetBkMode(hdc, TRANSPARENT);
    SetTextColor(hdc, RGB(245, 245, 245));

    HFONT font = TextOverlayCreateFont(state->fontSize, dpi);
    HGDIOBJ oldFont = SelectObject(hdc, font);

    int iconSize = TextOverlayDip(state->iconSize > 0 ? state->iconSize : 24.0f, dpi);
    int leadingWidth = state->loading ? iconSize : 0;
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
    if (renderedTextWidth < 1)
        renderedTextWidth = 1;
    if (renderedTextWidth > maxTextWidth)
        renderedTextWidth = maxTextWidth;
    int textLayoutWidth = state->centerContent ? renderedTextWidth : maxTextWidth;
    int rowHeight = textMeasure.bottom > iconSize ? textMeasure.bottom : iconSize;
    int closeSize = TextOverlayDip(TEXT_OVERLAY_CLOSE_SIZE_DIP, dpi);
    if (state->closable && rowHeight < closeSize)
        rowHeight = closeSize;
    int rowY = copyReserve + ((rc.bottom - rc.top - copyReserve - rowHeight) / 2);
    if (rowY < copyReserve)
        rowY = copyReserve;

    int groupWidth = leadingWidth + leadingGap + textLayoutWidth;
    int x = state->centerContent ? (contentAreaWidth - groupWidth) / 2 : 0;
    if (x < 0)
        x = 0;
    if (state->loading)
    {
        HBRUSH brush = CreateSolidBrush(RGB(230, 230, 230));
        HGDIOBJ oldBrush = SelectObject(hdc, brush);
        HGDIOBJ oldPen = SelectObject(hdc, GetStockObject(NULL_PEN));
        int y = rowY + (rowHeight - iconSize) / 2;
        Ellipse(hdc, x, y, x + iconSize, y + iconSize);
        if (oldPen)
            SelectObject(hdc, oldPen);
        if (oldBrush)
            SelectObject(hdc, oldBrush);
        DeleteObject(brush);
        x += leadingWidth + leadingGap;
    }

    RECT textRc = {x, rowY, x + textLayoutWidth, rowY + rowHeight};
    DrawTextW(hdc, state->message ? state->message : L"", -1, &textRc, DT_WORDBREAK | DT_NOPREFIX | DT_VCENTER);

    if (state->closable)
    {
        RECT closeRc = TextOverlayCloseButtonRect(state, dpi);
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

    if (state->showCopyButton)
    {
        RECT copyRc = TextOverlayCopyButtonRect(state, dpi);
        HBRUSH brush = CreateSolidBrush(state->copied ? RGB(46, 112, 82) : RGB(70, 70, 70));
        FillRect(hdc, &copyRc, brush);
        DeleteObject(brush);
        SetTextColor(hdc, RGB(255, 255, 255));
        DrawTextW(hdc, state->copied ? L"OK" : L"Copy", -1, &copyRc, DT_CENTER | DT_VCENTER | DT_SINGLELINE | DT_NOPREFIX);
    }

    if (oldFont)
        SelectObject(hdc, oldFont);
    DeleteObject(font);
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
        break;
    case WM_PAINT:
    {
        PAINTSTRUCT ps;
        HDC hdc = BeginPaint(hwnd, &ps);
        RECT rc;
        GetClientRect(hwnd, &rc);
        if (state)
            TextOverlayDraw(hdc, rc, state);
        EndPaint(hwnd, &ps);
        return 0;
    }
    case WM_CLOSE:
        DestroyWindow(hwnd);
        return 0;
    case WM_NCDESTROY:
        KillTimer(hwnd, TEXT_OVERLAY_TIMER_COPY_FEEDBACK);
        KillTimer(hwnd, TEXT_OVERLAY_TIMER_AUTOCLOSE);
        if (state)
        {
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
    HWND hwnd = CreateWindowExW(WS_EX_NOACTIVATE | WS_EX_TRANSPARENT, kTextOverlayClassName, L"", WS_POPUP, 0, 0, state->contentWidth, state->contentHeight, NULL, NULL, GetModuleHandleW(NULL), state);
    state->hwnd = hwnd;
    state->createOk = hwnd ? TRUE : FALSE;
    SetEvent(state->readyEvent);
    if (!hwnd)
        return 0;
    if (state->autoCloseSeconds > 0)
        SetTimer(hwnd, TEXT_OVERLAY_TIMER_AUTOCLOSE, (UINT)(state->autoCloseSeconds * 1000), NULL);

    MSG msg;
    while (GetMessageW(&msg, NULL, 0, 0) > 0)
    {
        TranslateMessage(&msg);
        DispatchMessageW(&msg);
    }
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
    (void)iconData;
    (void)iconLen;
    (void)tooltip;
    (void)tooltipIconData;
    (void)tooltipIconLen;
    (void)copyButtonTooltip;
    (void)copyButtonSuccessTooltip;
    if (!TextOverlayEnsureClass())
        return result;

    TextOverlayState *state = (TextOverlayState *)calloc(1, sizeof(TextOverlayState));
    if (!state)
        return result;
    state->nameUtf8 = TextOverlayCopyUtf8(name);
    state->message = TextOverlayUtf8ToWide(message);
    state->loading = loading ? TRUE : FALSE;
    state->closable = closable ? TRUE : FALSE;
    state->centerContent = centerContent ? TRUE : FALSE;
    state->showCopyButton = showCopyButton ? TRUE : FALSE;
    state->autoCloseSeconds = autoCloseSeconds;
    state->fontSize = fontSize > 0 ? fontSize : 13.0f;
    state->iconSize = iconSize > 0 ? iconSize : 24.0f;
    state->tooltipIconSize = tooltipIconSize > 0 ? tooltipIconSize : 18.0f;
    SIZE size = TextOverlayMeasure(state->message, state->loading, iconLen > 0, tooltipIconLen > 0, state->showCopyButton, closable, state->fontSize, state->iconSize, state->tooltipIconSize, windowWidth, minWindowWidth, maxWindowWidth, windowHeight, maxWindowHeight);
    state->contentWidth = size.cx;
    state->contentHeight = size.cy;
    state->readyEvent = CreateEventW(NULL, TRUE, FALSE, NULL);
    if (!state->readyEvent)
    {
        free(state->nameUtf8);
        free(state->message);
        free(state);
        return result;
    }

    HANDLE thread = CreateThread(NULL, 0, TextOverlayThreadProc, state, 0, NULL);
    if (!thread)
    {
        CloseHandle(state->readyEvent);
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

void TextOverlayDestroyWindow(void *rawHwnd)
{
    HWND hwnd = (HWND)rawHwnd;
    if (hwnd && IsWindow(hwnd))
        PostMessageW(hwnd, WM_CLOSE, 0, 0);
}
