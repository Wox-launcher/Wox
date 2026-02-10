#define UNICODE
#define _UNICODE
#include <windows.h>
#include <shellapi.h>
#include <stdlib.h>

NOTIFYICONDATA nid;
HMENU hMenu;
UINT_PTR nextMenuId = 1;
HWND hwnd;
UINT queryIconCount = 0;
UINT queryIconIds[256] = {0};
HICON queryIconHandles[256] = {0};

void reportClick(UINT_PTR menuId);
void reportLeftClick();
void reportQueryClick(UINT_PTR iconId, int x, int y, int width, int height);
void clearQueryTrayIcons();

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
	clearQueryTrayIcons();
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
	case WM_APP + 2:
		if (lParam == WM_LBUTTONUP)
		{
			NOTIFYICONIDENTIFIER nidIdentifier = {0};
			nidIdentifier.cbSize = sizeof(NOTIFYICONIDENTIFIER);
			nidIdentifier.hWnd = hwnd;
			nidIdentifier.uID = (UINT)wParam;

			RECT rect;
			HRESULT hr = Shell_NotifyIconGetRect(&nidIdentifier, &rect);
			if (SUCCEEDED(hr))
			{
				HMONITOR hMonitor = MonitorFromRect(&rect, MONITOR_DEFAULTTONEAREST);
				UINT dpi = 96;

				HMODULE shcore = LoadLibraryA("Shcore.dll");
				if (shcore)
				{
					typedef HRESULT(WINAPI * GetDpiForMonitorFunc)(HMONITOR, int, UINT *, UINT *);
					GetDpiForMonitorFunc getDpiForMonitor = (GetDpiForMonitorFunc)GetProcAddress(shcore, "GetDpiForMonitor");
					if (getDpiForMonitor)
					{
						UINT dpiX = 96, dpiY = 96;
						if (SUCCEEDED(getDpiForMonitor(hMonitor, 0, &dpiX, &dpiY)))
						{
							dpi = dpiX;
						}
					}
					FreeLibrary(shcore);
				}

				float scale = (float)dpi / 96.0f;
				int x = (int)(rect.left / scale);
				int y = (int)(rect.top / scale);
				int width = (int)((rect.right - rect.left) / scale);
				int height = (int)((rect.bottom - rect.top) / scale);

				reportQueryClick((UINT_PTR)wParam, x, y, width, height);
			}
			else
			{
				reportQueryClick((UINT_PTR)wParam, 0, 0, 0, 0);
			}
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

void addQueryTrayIcon(UINT id, const char *iconName, const char *tooltip)
{
	if (queryIconCount >= 256)
	{
		return;
	}

	HICON icon = loadIcon(iconName);
	NOTIFYICONDATA queryNid = {0};
	queryNid.cbSize = sizeof(NOTIFYICONDATA);
	queryNid.hWnd = hwnd;
	queryNid.uID = id;
	queryNid.uFlags = NIF_MESSAGE | NIF_ICON | NIF_TIP;
	queryNid.uCallbackMessage = WM_APP + 2;
	queryNid.hIcon = icon;

	if (tooltip != NULL)
	{
		int len = MultiByteToWideChar(CP_UTF8, 0, tooltip, -1, NULL, 0);
		if (len > 0)
		{
			wchar_t wTooltip[128] = {0};
			MultiByteToWideChar(CP_UTF8, 0, tooltip, -1, wTooltip, len);
			wcsncpy(queryNid.szTip, wTooltip, sizeof(queryNid.szTip) / sizeof(wchar_t) - 1);
		}
	}

	Shell_NotifyIcon(NIM_ADD, &queryNid);
	queryIconIds[queryIconCount] = id;
	queryIconHandles[queryIconCount] = icon;
	queryIconCount++;
}

void clearQueryTrayIcons()
{
	for (UINT i = 0; i < queryIconCount; i++)
	{
		NOTIFYICONDATA queryNid = {0};
		queryNid.cbSize = sizeof(NOTIFYICONDATA);
		queryNid.hWnd = hwnd;
		queryNid.uID = queryIconIds[i];
		Shell_NotifyIcon(NIM_DELETE, &queryNid);
		if (queryIconHandles[i] != NULL)
		{
			DestroyIcon(queryIconHandles[i]);
			queryIconHandles[i] = NULL;
		}
		queryIconIds[i] = 0;
	}
	queryIconCount = 0;
}
