#include <windows.h>
#include <windowsx.h>
#include <uxtheme.h>
#include <math.h>
#include <stdbool.h>
#include <stdlib.h>
#include <string.h>

#define DICTATION_OVERLAY_TIMER 1
#define WM_DICTATION_OVERLAY_SET_ACTIVE (WM_APP + 0x681)
#define DICTATION_CLOSE_SIZE_DIP 20
#define DICTATION_CLOSE_GAP_DIP 8

extern void overlayRequestCloseCallbackCGO(char *name);

typedef struct {
    HWND hwnd;
    HANDLE readyEvent;
    BOOL createOk;
    char *nameUtf8;
    BOOL closable;
    BOOL active;
    BOOL closeHover;
    BOOL closePressed;
    int phase;
} DictationOverlayState;

static const wchar_t *kDictationOverlayClassName = L"WoxDictationOverlayWindow";
static ATOM g_dictationOverlayClass = 0;

static char *DictationOverlayCopyUtf8(const char *text)
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

static UINT DictationOverlayGetDpi(HWND hwnd)
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

static int DictationOverlayDip(int value, UINT dpi)
{
    return MulDiv(value, (int)dpi, 96);
}

static RECT DictationOverlayCloseRect(const RECT *rect, UINT dpi)
{
    int size = DictationOverlayDip(DICTATION_CLOSE_SIZE_DIP, dpi);
    RECT closeRect = {rect->right - size, rect->top + ((rect->bottom - rect->top) - size) / 2, rect->right, rect->top + ((rect->bottom - rect->top) - size) / 2 + size};
    return closeRect;
}

// DictationOverlayInvalidate only invalidates this window. After switching to WS_EX_LAYERED +
// UpdateLayeredWindow the child owns an opaque alpha surface, so the parent backdrop no longer
// needs to repaint behind it. This removes the cross-thread RDW_UPDATENOW that caused flicker.
static void DictationOverlayInvalidate(HWND hwnd)
{
    InvalidateRect(hwnd, NULL, FALSE);
}

static void DictationOverlayDraw(HDC hdc, const RECT *rect, UINT dpi, BOOL active, int phase)
{
    if (!hdc || !rect)
        return;

    int barCount = 7;
    int barWidth = DictationOverlayDip(4, dpi);
    int gap = DictationOverlayDip(5, dpi);
    if (barWidth < 2)
        barWidth = 2;

    int totalWidth = barCount * barWidth + (barCount - 1) * gap;
    int startX = rect->left + ((rect->right - rect->left) - totalWidth) / 2;
    int centerY = rect->top + (rect->bottom - rect->top) / 2;
    int maxHeight = (rect->bottom - rect->top) - DictationOverlayDip(2, dpi);
    if (maxHeight < DictationOverlayDip(8, dpi))
        maxHeight = DictationOverlayDip(8, dpi);

    double idleScales[7] = {0.32, 0.46, 0.36, 0.56, 0.36, 0.46, 0.32};
    HBRUSH brush = CreateSolidBrush(RGB(245, 245, 245));
    HGDIOBJ oldBrush = SelectObject(hdc, brush);
    HGDIOBJ oldPen = SelectObject(hdc, GetStockObject(NULL_PEN));

    for (int i = 0; i < barCount; i++)
    {
        double scale = idleScales[i];
        if (active)
            scale = 0.28 + 0.72 * (0.5 + 0.5 * sin((double)phase * 0.32 + (double)i * 0.85));
        int barHeight = (int)((double)maxHeight * scale + 0.5);
        int minHeight = DictationOverlayDip(5, dpi);
        if (barHeight < minHeight)
            barHeight = minHeight;

        int x = startX + i * (barWidth + gap);
        int y = centerY - barHeight / 2;
        RoundRect(hdc, x, y, x + barWidth, y + barHeight, barWidth, barWidth);
    }

    if (oldPen)
        SelectObject(hdc, oldPen);
    if (oldBrush)
        SelectObject(hdc, oldBrush);
    DeleteObject(brush);
}

// DictationOverlayPaint draws one full frame into the given DC using the same BufferedPaint API
// the base overlay uses. Painting bars and close button into one buffered DC in a single pass
// eliminates the flicker the old cross-thread parent invalidation caused.
static void DictationOverlayPaint(HWND hwnd, HDC paintHdc, DictationOverlayState *state)
{
    RECT rc;
    GetClientRect(hwnd, &rc);

    HDC hdc = paintHdc;
    HPAINTBUFFER paintBuf = BeginBufferedPaint(paintHdc, &rc, BPBF_TOPDOWNDIB, NULL, &hdc);
    if (paintBuf)
        BufferedPaintClear(paintBuf, &rc);

    SetBkMode(hdc, TRANSPARENT);
    UINT dpi = DictationOverlayGetDpi(hwnd);
    RECT contentRc = rc;
    if (state && state->closable)
    {
        contentRc.right -= DictationOverlayDip(DICTATION_CLOSE_SIZE_DIP + DICTATION_CLOSE_GAP_DIP, dpi);
        if (contentRc.right < contentRc.left)
            contentRc.right = contentRc.left;
    }
    DictationOverlayDraw(hdc, &contentRc, dpi, state ? state->active : FALSE, state ? state->phase : 0);
    if (state && state->closable)
    {
        RECT closeRc = DictationOverlayCloseRect(&rc, dpi);
        if (state->closeHover || state->closePressed)
        {
            COLORREF bg = state->closePressed ? RGB(70, 70, 70) : RGB(55, 55, 55);
            HBRUSH brush = CreateSolidBrush(bg);
            FillRect(hdc, &closeRc, brush);
            DeleteObject(brush);
        }

        int pad = DictationOverlayDip(6, dpi);
        int thickness = DictationOverlayDip(2, dpi);
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

    if (paintBuf)
        EndBufferedPaint(paintBuf, TRUE);
}

static LRESULT CALLBACK DictationOverlayProc(HWND hwnd, UINT msg, WPARAM wParam, LPARAM lParam)
{
    DictationOverlayState *state = (DictationOverlayState *)GetWindowLongPtrW(hwnd, GWLP_USERDATA);

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
            RECT rc;
            GetClientRect(hwnd, &rc);
            RECT closeRc = DictationOverlayCloseRect(&rc, DictationOverlayGetDpi(hwnd));
            if (PtInRect(&closeRc, pt))
            {
                SetCursor(LoadCursor(NULL, IDC_HAND));
                return TRUE;
            }
        }
        break;
    }
    case WM_TIMER:
        if (wParam == DICTATION_OVERLAY_TIMER)
        {
            if (!state || !state->active)
            {
                KillTimer(hwnd, DICTATION_OVERLAY_TIMER);
                return 0;
            }
            state->phase++;
            DictationOverlayInvalidate(hwnd);
            return 0;
        }
        break;
    case WM_DICTATION_OVERLAY_SET_ACTIVE:
        if (!state)
            return 0;
        state->active = wParam ? TRUE : FALSE;
        if (state->active)
        {
            SetTimer(hwnd, DICTATION_OVERLAY_TIMER, 33, NULL);
        }
        else
        {
            KillTimer(hwnd, DICTATION_OVERLAY_TIMER);
            state->phase = 0;
        }
        DictationOverlayInvalidate(hwnd);
        return 0;
    case WM_MOUSEMOVE:
    {
        if (!state)
            break;
        TRACKMOUSEEVENT tme = {sizeof(TRACKMOUSEEVENT), TME_LEAVE, hwnd, 0};
        TrackMouseEvent(&tme);

        POINT pt = {GET_X_LPARAM(lParam), GET_Y_LPARAM(lParam)};
        RECT rc;
        GetClientRect(hwnd, &rc);
        RECT closeRc = DictationOverlayCloseRect(&rc, DictationOverlayGetDpi(hwnd));
        BOOL closeHoverNow = state->closable && PtInRect(&closeRc, pt);
        if (closeHoverNow != state->closeHover)
        {
            state->closeHover = closeHoverNow;
            DictationOverlayInvalidate(hwnd);
        }
        return 0;
    }
    case WM_MOUSELEAVE:
        if (state)
        {
            state->closeHover = FALSE;
            if (!state->closePressed)
                DictationOverlayInvalidate(hwnd);
        }
        return 0;
    case WM_LBUTTONDOWN:
        if (!state)
            return 0;
        if (state->closable)
        {
            RECT rc;
            GetClientRect(hwnd, &rc);
            RECT closeRc = DictationOverlayCloseRect(&rc, DictationOverlayGetDpi(hwnd));
            POINT pt = {GET_X_LPARAM(lParam), GET_Y_LPARAM(lParam)};
            if (PtInRect(&closeRc, pt))
            {
                state->closePressed = TRUE;
                SetCapture(hwnd);
                DictationOverlayInvalidate(hwnd);
                return 0;
            }
        }
        return 0;
    case WM_PAINT:
    {
        PAINTSTRUCT ps;
        HDC hdc = BeginPaint(hwnd, &ps);
        DictationOverlayPaint(hwnd, hdc, state);
        EndPaint(hwnd, &ps);
        return 0;
    }
    case WM_LBUTTONUP:
        if (!state || !state->nameUtf8)
            return 0;
        if (state->closePressed)
        {
            RECT rc;
            GetClientRect(hwnd, &rc);
            RECT closeRc = DictationOverlayCloseRect(&rc, DictationOverlayGetDpi(hwnd));
            POINT pt = {GET_X_LPARAM(lParam), GET_Y_LPARAM(lParam)};
            state->closePressed = FALSE;
            if (GetCapture() == hwnd)
                ReleaseCapture();
            DictationOverlayInvalidate(hwnd);
            if (state->closable && PtInRect(&closeRc, pt))
            {
                overlayRequestCloseCallbackCGO(state->nameUtf8);
                return 0;
            }
            return 0;
        }
        return 0;
    case WM_CLOSE:
        DestroyWindow(hwnd);
        return 0;
    case WM_NCDESTROY:
        KillTimer(hwnd, DICTATION_OVERLAY_TIMER);
        if (state)
        {
            free(state->nameUtf8);
            free(state);
            SetWindowLongPtrW(hwnd, GWLP_USERDATA, 0);
        }
        PostQuitMessage(0);
        break;
    }

    return DefWindowProcW(hwnd, msg, wParam, lParam);
}

static BOOL DictationOverlayEnsureClass(void)
{
    if (g_dictationOverlayClass)
        return TRUE;

    WNDCLASSEXW wc;
    ZeroMemory(&wc, sizeof(wc));
    wc.cbSize = sizeof(wc);
    wc.lpfnWndProc = DictationOverlayProc;
    wc.hInstance = GetModuleHandleW(NULL);
    wc.hCursor = LoadCursor(NULL, IDC_ARROW);
    wc.lpszClassName = kDictationOverlayClassName;
    g_dictationOverlayClass = RegisterClassExW(&wc);
    return g_dictationOverlayClass != 0;
}

static DWORD WINAPI DictationOverlayThreadProc(LPVOID param)
{
    DictationOverlayState *state = (DictationOverlayState *)param;
    // WS_EX_TRANSPARENT keeps mouse forwarding to the parent while the child owns the visible
    // painted surface. BufferedPaint (initialized per thread) gives the child the same flicker-free
    // double-buffering the base overlay uses.
    BufferedPaintInit();
    HWND hwnd = CreateWindowExW(WS_EX_NOACTIVATE | WS_EX_TRANSPARENT, kDictationOverlayClassName, L"", WS_POPUP, 0, 0, 132, 24, NULL, NULL, GetModuleHandleW(NULL), state);
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

void* DictationOverlayCreateWindow(char *name, bool closable)
{
    if (!DictationOverlayEnsureClass())
        return NULL;

    DictationOverlayState *state = (DictationOverlayState *)calloc(1, sizeof(DictationOverlayState));
    if (!state)
        return NULL;

    state->nameUtf8 = DictationOverlayCopyUtf8(name);
    state->closable = closable ? TRUE : FALSE;
    state->readyEvent = CreateEventW(NULL, TRUE, FALSE, NULL);
    if (!state->readyEvent)
    {
        free(state->nameUtf8);
        free(state);
        return NULL;
    }

    HANDLE thread = CreateThread(NULL, 0, DictationOverlayThreadProc, state, 0, NULL);
    if (!thread)
    {
        CloseHandle(state->readyEvent);
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
        free(state->nameUtf8);
        free(state);
        return NULL;
    }
    return state->hwnd;
}

void DictationOverlaySetActive(void* rawHwnd, bool active)
{
    HWND hwnd = (HWND)rawHwnd;
    if (!hwnd || !IsWindow(hwnd))
        return;

    PostMessageW(hwnd, WM_DICTATION_OVERLAY_SET_ACTIVE, active ? 1 : 0, 0);
}

void DictationOverlayDestroyWindow(void* rawHwnd)
{
    HWND hwnd = (HWND)rawHwnd;
    if (hwnd && IsWindow(hwnd))
        PostMessageW(hwnd, WM_CLOSE, 0, 0);
}
