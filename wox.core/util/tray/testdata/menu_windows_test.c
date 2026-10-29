// Native checks exercise the production GDI/menu path without installing a tray icon.
#include "../menu_windows.c"
#include <assert.h>
#include <stdio.h>
#include <string.h>

static BOOL previewDark = TRUE;

int trayMenuIsDark(void)
{
	return previewDark;
}

static HMENU testMenu(void)
{
	HMENU menu = CreatePopupMenu();
	assert(menu != NULL);
	assert(AppendMenuW(menu, MF_STRING, 1, L"Toggle Wox"));
	assert(AppendMenuW(menu, MF_STRING, 2, L"Settings"));
	assert(AppendMenuW(menu, MF_SEPARATOR, 0, NULL));
	assert(AppendMenuW(menu, MF_STRING, 3, L"Quit"));
	return menu;
}

// Assert actual pixels rather than mirroring the drawing implementation.
static void testDrawing(TrayPopup *popup, BOOL dark)
{
	HDC dc = CreateCompatibleDC(NULL);
	HDC screen = GetDC(NULL);
	HBITMAP bitmap = CreateCompatibleBitmap(screen, 500, 200);
	ReleaseDC(NULL, screen);
	HGDIOBJ previous = SelectObject(dc, bitmap);
	DRAWITEMSTRUCT draw = {0};
	draw.CtlType = ODT_MENU;
	draw.hDC = dc;
	draw.rcItem = (RECT){0, 0, 300, 64};
	draw.itemData = (ULONG_PTR)&popup->items[0];
	assert(trayDrawMenuItem(&draw));
	assert(GetPixel(dc, 1, 1) == (dark ? RGB(32, 32, 32) : RGB(250, 250, 250)));
	draw.itemState = ODS_SELECTED;
	assert(trayDrawMenuItem(&draw));
	assert(GetPixel(dc, 1, 1) == (dark ? RGB(55, 55, 55) : RGB(232, 232, 232)));
	draw.itemData = (ULONG_PTR)&popup->items[2];
	assert(trayDrawMenuItem(&draw));
	assert(GetPixel(dc, 150, 32) == popup->items[2].colors.separator);
	assert(GetPixel(dc, 1, 32) == popup->items[2].colors.background);
	SelectObject(dc, previous);
	DeleteObject(bitmap);
	DeleteDC(dc);
}

// Cover mnemonic cycling, escaped ampersands, disabled items, and explicit access keys.
static void testKeyboard(void)
{
	HMENU source = CreatePopupMenu();
	AppendMenuW(source, MF_STRING, 1, L"&Settings");
	AppendMenuW(source, MF_STRING, 2, L"&Show Wox");
	AppendMenuW(source, MF_STRING | MF_DISABLED, 3, L"&Stop");
	AppendMenuW(source, MF_STRING, 4, L"Rock && roll &Exit");
	TrayPopup popup = {0};
	assert(trayPreparePopup(&popup, NULL, source, 96, TRUE));
	assert(trayMenuChar(popup.menu, L's') == MAKELRESULT(0, MNC_SELECT));
	MENUITEMINFOW state = {0};
	state.cbSize = sizeof(state);
	state.fMask = MIIM_STATE;
	state.fState = MFS_HILITE;
	assert(SetMenuItemInfoW(popup.menu, 0, TRUE, &state));
	assert(trayMenuChar(popup.menu, L's') == MAKELRESULT(1, MNC_SELECT));
	state.fState = 0;
	assert(SetMenuItemInfoW(popup.menu, 0, TRUE, &state));
	state.fState = MFS_HILITE;
	assert(SetMenuItemInfoW(popup.menu, 1, TRUE, &state));
	assert(trayMenuChar(popup.menu, L's') == MAKELRESULT(0, MNC_SELECT));
	assert(trayMenuChar(popup.menu, L'e') == MAKELRESULT(3, MNC_EXECUTE));
	assert(trayMenuChar(popup.menu, L'z') == MAKELRESULT(0, MNC_IGNORE));
	trayDestroyPopup(&popup);
	DestroyMenu(source);
}

// Exercise fresh snapshots at multiple DPIs, translated labels, and partial failure cleanup.
static void testPopup(void)
{
	HMENU source = testMenu();
	UINT dpis[] = {96, 144, 192};
	for (int dark = 0; dark <= 1; dark++)
	{
		for (int d = 0; d < 3; d++)
		{
			TrayPopup popup = {0};
			assert(trayPreparePopup(&popup, NULL, source, dpis[d], dark));
			assert(GetMenuItemCount(popup.menu) == 4);
			assert(GetMenuItemID(popup.menu, 3) == 3);
			assert(GetMenuState(popup.menu, 2, MF_BYPOSITION) & MF_SEPARATOR);
			assert(popup.items[0].accessible.dwMSAASignature == MSAA_MENU_SIG);
			assert(wcscmp(popup.items[0].accessible.pszWText, L"Toggle Wox") == 0);
			assert(popup.items[0].height >= 32 * dpis[d] / 96);
			assert(popup.items[0].width >= 160 * dpis[d] / 96);
			assert(popup.items[0].padding == 12 * dpis[d] / 96);
			MEASUREITEMSTRUCT measure = {0};
			measure.CtlType = ODT_MENU;
			measure.itemData = (ULONG_PTR)&popup.items[0];
			assert(trayMeasureMenuItem(&measure));
			assert(measure.itemHeight == popup.items[0].height);
			testDrawing(&popup, dark);
			trayDestroyPopup(&popup);
			assert(popup.menu == NULL && popup.items == NULL && popup.font == NULL && popup.background == NULL);
		}
	}
	ModifyMenuW(source, 0, MF_BYPOSITION | MF_STRING, 1, L"\x663e\x793a/\x9690\x85cf Wox - A long translated menu command that must remain visible");
	TrayPopup popup = {0};
	assert(trayPreparePopup(&popup, NULL, source, 96, TRUE));
	assert(popup.items[0].width > 160);
	trayDestroyPopup(&popup);
	assert(!trayPreparePopup(&popup, NULL, NULL, 96, TRUE));
	trayDestroyPopup(&popup);
	DestroyMenu(source);
}

// Preview mode exposes the same native tracker for manual visual and keyboard checks.
static LRESULT CALLBACK previewProc(HWND window, UINT message, WPARAM wParam, LPARAM lParam)
{
	if (message == WM_MEASUREITEM && trayMeasureMenuItem((MEASUREITEMSTRUCT *)lParam))
		return TRUE;
	if (message == WM_DRAWITEM && trayDrawMenuItem((DRAWITEMSTRUCT *)lParam))
		return TRUE;
	if (message == WM_MENUCHAR)
		return trayMenuChar((HMENU)lParam, LOWORD(wParam));
	if (message == WM_CONTEXTMENU || (message == WM_KEYDOWN && wParam == VK_SPACE))
	{
		POINT point;
		GetCursorPos(&point);
		HMENU menu = testMenu();
		UINT command = trayTrackPopupMenu(window, menu, point);
		DestroyMenu(menu);
		WCHAR title[100];
		swprintf(title, 100, L"Wox tray menu preview - command %u", command);
		SetWindowTextW(window, title);
		return 0;
	}
	if (message == WM_KEYDOWN && (wParam == VK_F6 || wParam == VK_F7))
		previewDark = wParam == VK_F6;
	if (message == WM_KEYDOWN && wParam == VK_ESCAPE)
		DestroyWindow(window);
	if (message == WM_DESTROY)
		PostQuitMessage(0);
	return DefWindowProcW(window, message, wParam, lParam);
}

int main(int argc, char **argv)
{
#ifdef TRAY_MENU_PREVIEW
	(void)argc;
	(void)argv;
	if (TRUE)
#else
	if (argc > 1 && strcmp(argv[1], "--preview") == 0)
#endif
	{
		SetThreadDpiAwarenessContext((HANDLE)-4);
		WNDCLASSW wc = {0};
		wc.lpfnWndProc = previewProc;
		wc.hInstance = GetModuleHandleW(NULL);
		wc.lpszClassName = L"WoxTrayMenuPreview";
		wc.hbrBackground = GetSysColorBrush(COLOR_WINDOW);
		RegisterClassW(&wc);
		HWND window = CreateWindowW(wc.lpszClassName, L"Wox tray menu preview", WS_OVERLAPPEDWINDOW,
			600, 500, 440, 180, NULL, NULL, wc.hInstance, NULL);
		CreateWindowW(L"STATIC", L"Right click: menu\nF6: dark / F7: light\nEsc: close preview", WS_CHILD | WS_VISIBLE,
			12, 12, 350, 100, window, NULL, wc.hInstance, NULL);
		ShowWindow(window, SW_SHOW);
		MSG message;
		while (GetMessageW(&message, NULL, 0, 0) > 0)
		{
			TranslateMessage(&message);
			DispatchMessageW(&message);
		}
		return 0;
	}
	testPopup();
	testKeyboard();
	DWORD before = GetGuiResources(GetCurrentProcess(), GR_GDIOBJECTS);
	for (int i = 0; i < 50; i++)
		testPopup();
	assert(GetGuiResources(GetCurrentProcess(), GR_GDIOBJECTS) == before);
	puts("Native tray menu checks passed");
	return 0;
}
