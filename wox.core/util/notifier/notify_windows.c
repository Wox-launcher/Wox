#include <windows.h>
#include <gdiplus.h>
#include <time.h>

#pragma comment(lib, "gdiplus.lib")

#define WM_TRAYICON (WM_USER + 1)
#define ID_TRAYICON 1
#define WINDOW_WIDTH 380
#define WINDOW_HEIGHT 80
#define CLOSE_TIMER 1

typedef struct {
    HWND hwnd;
    HFONT messageFont;
    WCHAR messageText[1024];
    UINT_PTR closeTimerId;
    POINT mousePosition;
    BOOL mouseInside;
} NotificationWindow;

LRESULT CALLBACK NotificationWindowProc(HWND hwnd, UINT uMsg, WPARAM wParam, LPARAM lParam);
void DrawRoundedRectangle(HDC hdc, RECT rect, int radius);

void showNotification(const char* message) {
    WNDCLASSEXA wc = {0};
    wc.cbSize = sizeof(WNDCLASSEXA);
    wc.lpfnWndProc = NotificationWindowProc;
    wc.hInstance = GetModuleHandle(NULL);
    wc.lpszClassName = "WoxNotification";
    RegisterClassExA(&wc);

    int screenWidth = GetSystemMetrics(SM_CXSCREEN);
    int screenHeight = GetSystemMetrics(SM_CYSCREEN);
    int xPos = (screenWidth - WINDOW_WIDTH) / 2;
    int yPos = (int)(screenHeight * 0.2) - WINDOW_HEIGHT / 2;

    NotificationWindow* nw = (NotificationWindow*)malloc(sizeof(NotificationWindow));
    memset(nw, 0, sizeof(NotificationWindow));

    nw->hwnd = CreateWindowExA(
        WS_EX_TOPMOST | WS_EX_LAYERED,
        "WoxNotification", "",
        WS_POPUP,
        xPos, yPos, WINDOW_WIDTH, WINDOW_HEIGHT,
        NULL, NULL, GetModuleHandle(NULL), NULL
    );

    SetWindowLongPtr(nw->hwnd, GWLP_USERDATA, (LONG_PTR)nw);

    nw->messageFont = CreateFontA(14, 0, 0, 0, FW_NORMAL, FALSE, FALSE, FALSE, DEFAULT_CHARSET,
        OUT_DEFAULT_PRECIS, CLIP_DEFAULT_PRECIS, DEFAULT_QUALITY, DEFAULT_PITCH | FF_DONTCARE, "Segoe UI");

    MultiByteToWideChar(CP_UTF8, 0, message, -1, nw->messageText, 1024);

    SetLayeredWindowAttributes(nw->hwnd, RGB(0,0,0), 0, LWA_COLORKEY);

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
        case WM_PAINT: {
            PAINTSTRUCT ps;
            HDC hdc = BeginPaint(hwnd, &ps);
            
            RECT clientRect;
            GetClientRect(hwnd, &clientRect);

            HDC memDC = CreateCompatibleDC(hdc);
            HBITMAP memBitmap = CreateCompatibleBitmap(hdc, clientRect.right, clientRect.bottom);
            SelectObject(memDC, memBitmap);

            HBRUSH backgroundBrush = CreateSolidBrush(RGB(64, 64, 64));
            FillRect(memDC, &clientRect, backgroundBrush);
            DeleteObject(backgroundBrush);

            DrawRoundedRectangle(memDC, clientRect, 20);

            SetBkMode(memDC, TRANSPARENT);
            SetTextColor(memDC, RGB(255, 255, 255));

            char messageTextA[1024];
            WideCharToMultiByte(CP_UTF8, 0, nw->messageText, -1, messageTextA, sizeof(messageTextA), NULL, NULL);
            RECT messageRect = {20, 20, clientRect.right - 20, clientRect.bottom - 20};
            DrawTextA(memDC, messageTextA, -1, &messageRect, DT_LEFT | DT_WORDBREAK);

            if (nw->mouseInside) {
                HPEN pen = CreatePen(PS_SOLID, 1, RGB(255, 255, 255));
                SelectObject(memDC, pen);
                HBRUSH oldBrush = (HBRUSH)SelectObject(memDC, GetStockObject(NULL_BRUSH));
                Ellipse(memDC, clientRect.right - 30, 10, clientRect.right - 10, 30);
                MoveToEx(memDC, clientRect.right - 25, 15, NULL);
                LineTo(memDC, clientRect.right - 15, 25);
                MoveToEx(memDC, clientRect.right - 25, 25, NULL);
                LineTo(memDC, clientRect.right - 15, 15);
                SelectObject(memDC, oldBrush);
                DeleteObject(pen);
            }

            BitBlt(hdc, 0, 0, clientRect.right, clientRect.bottom, memDC, 0, 0, SRCCOPY);

            DeleteObject(memBitmap);
            DeleteDC(memDC);

            EndPaint(hwnd, &ps);
            return 0;
        }

        case WM_MOUSEMOVE: {
            if (!nw->mouseInside) {
                nw->mouseInside = TRUE;
                TRACKMOUSEEVENT tme = {sizeof(TRACKMOUSEEVENT), TME_LEAVE, hwnd, 0};
                TrackMouseEvent(&tme);
                InvalidateRect(hwnd, NULL, FALSE);
            }
            return 0;
        }

        case WM_MOUSELEAVE: {
            nw->mouseInside = FALSE;
            InvalidateRect(hwnd, NULL, FALSE);
            return 0;
        }

        case WM_LBUTTONUP: {
            if (nw->mouseInside) {
                POINT pt;
                GetCursorPos(&pt);
                ScreenToClient(hwnd, &pt);
                RECT closeRect = {WINDOW_WIDTH - 30, 10, WINDOW_WIDTH - 10, 30};
                if (PtInRect(&closeRect, pt)) {
                    DestroyWindow(hwnd);
                }
            }
            return 0;
        }

        case WM_TIMER: {
            if (wParam == CLOSE_TIMER && !nw->mouseInside) {
                KillTimer(hwnd, CLOSE_TIMER);
                AnimateWindow(hwnd, 300, AW_BLEND | AW_HIDE);
                DestroyWindow(hwnd);
            }
            return 0;
        }

        case WM_DESTROY: {
            DeleteObject(nw->messageFont);
            PostQuitMessage(0);
            return 0;
        }
    }

    return DefWindowProc(hwnd, uMsg, wParam, lParam);
}

void DrawRoundedRectangle(HDC hdc, RECT rect, int radius) {
    HPEN pen = CreatePen(PS_SOLID, 1, RGB(64, 64, 64));
    HPEN oldPen = SelectObject(hdc, pen);
    HBRUSH brush = CreateSolidBrush(RGB(64, 64, 64));
    HBRUSH oldBrush = SelectObject(hdc, brush);

    int diameter = radius * 2;
    POINT pt;

    MoveToEx(hdc, rect.left + radius, rect.top, &pt);
    LineTo(hdc, rect.right - radius, rect.top);
    Arc(hdc, rect.right - diameter, rect.top, rect.right, rect.top + diameter, rect.right, rect.top, rect.right, rect.top + radius);
    LineTo(hdc, rect.right, rect.bottom - radius);
    Arc(hdc, rect.right - diameter, rect.bottom - diameter, rect.right, rect.bottom, rect.right, rect.bottom, rect.right - radius, rect.bottom);
    LineTo(hdc, rect.left + radius, rect.bottom);
    Arc(hdc, rect.left, rect.bottom - diameter, rect.left + diameter, rect.bottom, rect.left + radius, rect.bottom, rect.left, rect.bottom);
    LineTo(hdc, rect.left, rect.top + radius);
    Arc(hdc, rect.left, rect.top, rect.left + diameter, rect.top + diameter, rect.left, rect.top + radius, rect.left + radius, rect.top);

    SelectObject(hdc, oldPen);
    SelectObject(hdc, oldBrush);
    DeleteObject(pen);
    DeleteObject(brush);
}
