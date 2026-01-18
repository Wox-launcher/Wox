#define UNICODE
#define _UNICODE
#include <windows.h>
#include <shellapi.h>

NOTIFYICONDATA nid;
HMENU hMenu;
UINT_PTR nextMenuId = 1;
HWND hwnd;

void reportClick(UINT_PTR menuId);
void reportLeftClick();

void addMenuItem(UINT_PTR menuId, const char *title)
{
	int len = MultiByteToWideChar(CP_UTF8, 0, title, -1, NULL, 0);
	wchar_t *wTitle = (wchar_t *)malloc(len * sizeof(wchar_t));
	MultiByteToWideChar(CP_UTF8, 0, title, -1, wTitle, len);

	AppendMenuW(hMenu, MF_STRING, menuId, wTitle);
	free(wTitle);
}

void setTrayIcon(const char *tooltip, HICON icon)
{
	nid.cbSize = sizeof(NOTIFYICONDATA);
	nid.hWnd = hwnd;
	nid.uID = 1;
	nid.uFlags = NIF_MESSAGE | NIF_ICON | NIF_TIP;
	nid.uCallbackMessage = WM_APP + 1;

	int len = MultiByteToWideChar(CP_UTF8, 0, tooltip, -1, NULL, 0);
	wchar_t wTooltip[128];
	MultiByteToWideChar(CP_UTF8, 0, tooltip, -1, wTooltip, len);
	wcsncpy(nid.szTip, wTooltip, sizeof(nid.szTip) / sizeof(wchar_t));

	nid.hIcon = icon;

	if (!Shell_NotifyIcon(NIM_MODIFY, &nid))
	{
		Shell_NotifyIcon(NIM_ADD, &nid);
	}
}

void removeTrayIcon()
{
	Shell_NotifyIcon(NIM_DELETE, &nid);
}

void showMenu(HWND hwnd)
{
	POINT p;
	GetCursorPos(&p);
	SetForegroundWindow(hwnd); // Set the foreground window before showing the menu for proper focus
	TrackPopupMenu(hMenu, TPM_BOTTOMALIGN | TPM_LEFTALIGN, p.x, p.y, 0, hwnd, NULL);
	PostMessage(hwnd, WM_NULL, 0, 0); // Post a null message to make the menu close properly
}

LRESULT CALLBACK WindowProc(HWND hwnd, UINT uMsg, WPARAM wParam, LPARAM lParam)
{
	switch (uMsg)
	{
	case WM_APP + 1:
		if (lParam == WM_RBUTTONUP)
		{
			showMenu(hwnd);
		}
		else if (lParam == WM_LBUTTONUP)
		{
			reportLeftClick();
		}
		break;
	case WM_COMMAND:
		if (lParam == 0)
		{
			reportClick(wParam);
		}
		break;
	case WM_DESTROY:
		PostQuitMessage(0);
		break;
	default:
		return DefWindowProc(hwnd, uMsg, wParam, lParam);
	}
	return 0;
}

HICON loadIcon(const char *iconName)
{
	int len = MultiByteToWideChar(CP_UTF8, 0, iconName, -1, NULL, 0);
	wchar_t *wIconName = (wchar_t *)malloc(len * sizeof(wchar_t));
	MultiByteToWideChar(CP_UTF8, 0, iconName, -1, wIconName, len);

	HICON icon = (HICON)LoadImageW(NULL, wIconName, IMAGE_ICON, 32, 32, LR_LOADFROMFILE);
	free(wIconName);

	if (icon == NULL)
	{
		icon = LoadIcon(NULL, IDI_APPLICATION);
	}
	return icon;
}

void init(const char *iconName, const char *tooltip)
{
	hMenu = CreatePopupMenu();
	HICON icon = loadIcon(iconName);

	WNDCLASSW wc = {0};
	wc.lpfnWndProc = WindowProc;
	wc.hInstance = GetModuleHandle(NULL);
	wc.lpszClassName = L"WoxWindowClass";
	RegisterClassW(&wc);

	hwnd = CreateWindowExW(0, L"WoxWindowClass", L"Wox", WS_OVERLAPPEDWINDOW,
						   CW_USEDEFAULT, CW_USEDEFAULT, CW_USEDEFAULT, CW_USEDEFAULT, NULL, NULL, wc.hInstance, NULL);

	if (hwnd == NULL)
	{
		return;
	}

	setTrayIcon(tooltip, icon);
}

void runMessageLoop()
{
	MSG msg;
	while (GetMessage(&msg, NULL, 0, 0))
	{
		TranslateMessage(&msg);
		DispatchMessage(&msg);
	}
}
