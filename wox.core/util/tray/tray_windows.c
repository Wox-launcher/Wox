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
UINT queryIconMenuIds[256] = {0};
wchar_t *queryIconMenuTitles[256] = {0};
DWORD lastActivateTick = 0;
UINT lastActivateMessage = 0;
UINT lastActivateIconId = 0;
UINT lastActivateEventCode = 0;

void reportClick(UINT_PTR menuId);
void reportLeftClick();
void reportQueryClick(UINT_PTR iconId, int x, int y, int width, int height);
void clearQueryTrayIcons();

static int findQueryIconIndex(UINT iconId)
{
	for (UINT i = 0; i < queryIconCount; i++)
	{
		if (queryIconIds[i] == iconId)
		{
			return (int)i;
		}
	}
	return -1;
}

static void showQueryIconMenu(UINT iconId)
{
	if (hwnd == NULL)
	{
		return;
	}

	int index = findQueryIconIndex(iconId);
	if (index < 0 || queryIconMenuIds[index] == 0 || queryIconMenuTitles[index] == NULL)
	{
		return;
	}

	HMENU queryMenu = CreatePopupMenu();
	if (queryMenu == NULL)
	{
		return;
	}

	AppendMenuW(queryMenu, MF_STRING, queryIconMenuIds[index], queryIconMenuTitles[index]);

	POINT p;
	GetCursorPos(&p);
	SetForegroundWindow(hwnd);
	TrackPopupMenu(queryMenu, TPM_BOTTOMALIGN | TPM_LEFTALIGN, p.x, p.y, 0, hwnd, NULL);
	PostMessage(hwnd, WM_NULL, 0, 0);
	DestroyMenu(queryMenu);
}

static BOOL isTrayActivateEvent(UINT eventCode)
{
	return eventCode == NIN_SELECT || eventCode == NIN_KEYSELECT || eventCode == WM_LBUTTONUP;
}

static BOOL isDuplicateTrayActivateEvent(UINT callbackMessage, UINT iconId, UINT eventCode)
{
	if (!isTrayActivateEvent(eventCode))
	{
		return FALSE;
	}

	DWORD now = GetTickCount();
	BOOL isSelectPair = (eventCode == NIN_SELECT && lastActivateEventCode == WM_LBUTTONUP) ||
						(eventCode == WM_LBUTTONUP && lastActivateEventCode == NIN_SELECT);
	BOOL isSameTarget = lastActivateMessage == callbackMessage && lastActivateIconId == iconId;
	BOOL isRecentPair = now - lastActivateTick <= 250;

	lastActivateTick = now;
	lastActivateMessage = callbackMessage;
	lastActivateIconId = iconId;
	lastActivateEventCode = eventCode;

	return isSameTarget && isSelectPair && isRecentPair;
}

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

	nid.uVersion = NOTIFYICON_VERSION_4;
	Shell_NotifyIcon(NIM_SETVERSION, &nid);
}

void removeTrayIcon()
{
	clearQueryTrayIcons();
	Shell_NotifyIcon(NIM_DELETE, &nid);
	if (hwnd != NULL)
	{
		PostMessage(hwnd, WM_CLOSE, 0, 0);
	}
}

void showMenu()
{
	if (hwnd == NULL || hMenu == NULL)
	{
		return;
	}

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
	{
		UINT eventCode = LOWORD((DWORD)lParam);
		if (eventCode == WM_CONTEXTMENU || eventCode == WM_RBUTTONUP)
		{
			showMenu();
		}
		else if (!isDuplicateTrayActivateEvent(WM_APP + 1, 1, eventCode) && isTrayActivateEvent(eventCode))
		{
			reportLeftClick();
		}
		break;
	}
	case WM_APP + 2:
	{
		UINT eventCode = LOWORD((DWORD)lParam);
		UINT iconId = HIWORD((DWORD)lParam);
		if (iconId == 0)
		{
			iconId = (UINT)wParam;
		}

		if (!isDuplicateTrayActivateEvent(WM_APP + 2, iconId, eventCode) && isTrayActivateEvent(eventCode))
		{
			NOTIFYICONIDENTIFIER nidIdentifier = {0};
			nidIdentifier.cbSize = sizeof(NOTIFYICONIDENTIFIER);
			nidIdentifier.hWnd = hwnd;
			nidIdentifier.uID = iconId;

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

				reportQueryClick((UINT_PTR)iconId, x, y, width, height);
			}
			else
			{
				int anchorX = (int)(short)LOWORD((DWORD)wParam);
				int anchorY = (int)(short)HIWORD((DWORD)wParam);
				reportQueryClick((UINT_PTR)iconId, anchorX, anchorY, 0, 0);
			}
		}
		else if (eventCode == WM_CONTEXTMENU || eventCode == WM_RBUTTONUP)
		{
			showQueryIconMenu(iconId);
		}
		break;
	}
	case WM_COMMAND:
		if (lParam == 0)
		{
			reportClick(wParam);
		}
		break;
	case WM_DESTROY:
		if (hMenu != NULL)
		{
			DestroyMenu(hMenu);
			hMenu = NULL;
		}
		if (nid.hIcon != NULL)
		{
			DestroyIcon(nid.hIcon);
			nid.hIcon = NULL;
		}
		hwnd = NULL;
		nextMenuId = 1;
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
	nextMenuId = 1;
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

void addQueryTrayIcon(UINT id, const char *iconName, const char *tooltip, UINT menuId, const char *menuTitle)
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
	queryNid.uVersion = NOTIFYICON_VERSION_4;
	Shell_NotifyIcon(NIM_SETVERSION, &queryNid);
	queryIconIds[queryIconCount] = id;
	queryIconHandles[queryIconCount] = icon;
	queryIconMenuIds[queryIconCount] = menuId;
	if (menuId != 0 && menuTitle != NULL && menuTitle[0] != '\0')
	{
		int menuLen = MultiByteToWideChar(CP_UTF8, 0, menuTitle, -1, NULL, 0);
		if (menuLen > 0)
		{
			queryIconMenuTitles[queryIconCount] = (wchar_t *)malloc(menuLen * sizeof(wchar_t));
			MultiByteToWideChar(CP_UTF8, 0, menuTitle, -1, queryIconMenuTitles[queryIconCount], menuLen);
		}
	}
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
		if (queryIconMenuTitles[i] != NULL)
		{
			free(queryIconMenuTitles[i]);
			queryIconMenuTitles[i] = NULL;
		}
		queryIconIds[i] = 0;
		queryIconMenuIds[i] = 0;
	}
	queryIconCount = 0;
}
