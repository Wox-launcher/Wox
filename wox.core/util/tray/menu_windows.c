#define UNICODE
#define _UNICODE
#include "menu_windows.h"
#include <oleacc.h>
#include <stdlib.h>
#include <wctype.h>

extern int trayMenuIsDark(void);

typedef struct {
	COLORREF background;
	COLORREF text;
	COLORREF selected;
	COLORREF disabled;
	COLORREF separator;
} TrayMenuColors;

typedef struct {
	// MSAA requires this header first in the owner-drawn item's data.
	MSAAMENUINFO accessible;
	BOOL separator;
	UINT width;
	UINT height;
	UINT padding;
	UINT lineWidth;
	HFONT font;
	TrayMenuColors colors;
} TrayMenuItem;

// A popup owns its snapshot until tracking ends, including during nested messages.
typedef struct {
	HMENU menu;
	HFONT font;
	HBRUSH background;
	TrayMenuItem *items;
	int count;
} TrayPopup;

// The system owns high-contrast menus; these palettes only apply to ordinary menus.
static TrayMenuColors trayMenuColors(BOOL dark)
{
	if (dark)
	{
		return (TrayMenuColors){RGB(32, 32, 32), RGB(242, 242, 242), RGB(55, 55, 55), RGB(155, 155, 155), RGB(64, 64, 64)};
	}
	return (TrayMenuColors){RGB(250, 250, 250), RGB(26, 26, 26), RGB(232, 232, 232), RGB(110, 110, 110), RGB(220, 220, 220)};
}

// Dimensions are authored in logical units and converted once for the menu's monitor.
static void trayMenuItemSize(TrayMenuItem *item, UINT dpi, SIZE text)
{
	item->padding = MulDiv(12, dpi, 96);
	item->lineWidth = max(1, MulDiv(1, dpi, 96));
	item->width = max(MulDiv(160, dpi, 96), text.cx + 2 * item->padding);
	item->height = item->separator ? MulDiv(9, dpi, 96) : max(MulDiv(32, dpi, 96), text.cy + MulDiv(12, dpi, 96));
}

// Destroy the native menu before the item data and brushes it references.
static void trayDestroyPopup(TrayPopup *popup)
{
	if (popup->menu != NULL)
		DestroyMenu(popup->menu);
	for (int i = 0; i < popup->count; i++)
		free(popup->items[i].accessible.pszWText);
	free(popup->items);
	if (popup->font != NULL)
		DeleteObject(popup->font);
	if (popup->background != NULL)
		DeleteObject(popup->background);
	*popup = (TrayPopup){0};
}

// Build a fresh menu so Windows remeasures after monitor, font, or theme changes.
static BOOL trayPreparePopup(TrayPopup *popup, HWND owner, HMENU source, UINT dpi, BOOL dark)
{
	typedef BOOL(WINAPI *SystemParametersInfoForDpiFunc)(UINT, UINT, PVOID, UINT, UINT);
	SystemParametersInfoForDpiFunc systemParametersInfoForDpi =
		(SystemParametersInfoForDpiFunc)GetProcAddress(GetModuleHandleW(L"user32.dll"), "SystemParametersInfoForDpi");
	NONCLIENTMETRICSW metrics = {0};
	metrics.cbSize = sizeof(metrics);
	if (systemParametersInfoForDpi == NULL ||
		!systemParametersInfoForDpi(SPI_GETNONCLIENTMETRICS, sizeof(metrics), &metrics, 0, dpi))
		return FALSE;

	int count = GetMenuItemCount(source);
	if (count <= 0)
		return FALSE;
	popup->items = calloc(count, sizeof(TrayMenuItem));
	if (popup->items == NULL)
		return FALSE;
	popup->count = count;
	popup->font = CreateFontIndirectW(&metrics.lfMenuFont);
	popup->menu = CreatePopupMenu();
	TrayMenuColors colors = trayMenuColors(dark);
	popup->background = CreateSolidBrush(colors.background);
	if (popup->font == NULL || popup->menu == NULL || popup->background == NULL)
		return FALSE;

	MENUINFO info = {0};
	info.cbSize = sizeof(info);
	info.fMask = MIM_BACKGROUND | MIM_STYLE;
	info.hbrBack = popup->background;
	info.dwStyle = MNS_NOCHECK;
	if (!SetMenuInfo(popup->menu, &info))
		return FALSE;

	HDC dc = GetDC(owner);
	if (dc == NULL)
		return FALSE;
	HGDIOBJ previousFont = SelectObject(dc, popup->font);
	BOOL success = TRUE;
	for (int i = 0; i < count; i++)
	{
		MENUITEMINFOW sourceItem = {0};
		sourceItem.cbSize = sizeof(sourceItem);
		sourceItem.fMask = MIIM_FTYPE | MIIM_ID | MIIM_STATE | MIIM_STRING;
		if (!GetMenuItemInfoW(source, i, TRUE, &sourceItem))
		{
			success = FALSE;
			break;
		}
		TrayMenuItem *item = &popup->items[i];
		item->separator = (sourceItem.fType & MFT_SEPARATOR) != 0;
		item->font = popup->font;
		item->colors = colors;
		item->accessible.dwMSAASignature = MSAA_MENU_SIG;
		item->accessible.pszWText = calloc(sourceItem.cch + 1, sizeof(WCHAR));
		if (item->accessible.pszWText == NULL)
		{
			success = FALSE;
			break;
		}
		sourceItem.dwTypeData = item->accessible.pszWText;
		sourceItem.cch++;
		if (!GetMenuItemInfoW(source, i, TRUE, &sourceItem))
		{
			success = FALSE;
			break;
		}
		item->accessible.cchWText = (DWORD)wcslen(item->accessible.pszWText);
		RECT textRect = {0};
		if (!item->separator)
			DrawTextW(dc, item->accessible.pszWText, -1, &textRect, DT_CALCRECT | DT_SINGLELINE);
		trayMenuItemSize(item, dpi, (SIZE){textRect.right, textRect.bottom});

		// Keep both the native string and MSAA data; owner-draw alone loses spoken labels.
		sourceItem.fMask |= MIIM_DATA;
		sourceItem.fType |= MFT_OWNERDRAW;
		sourceItem.dwItemData = (ULONG_PTR)item;
		if (!InsertMenuItemW(popup->menu, i, TRUE, &sourceItem))
		{
			success = FALSE;
			break;
		}
	}
	SelectObject(dc, previousFont);
	ReleaseDC(owner, dc);
	return success;
}

// Keep tracking native so keyboard navigation, dismissal, and focus follow Windows.
UINT trayTrackPopupMenu(HWND owner, HMENU source, POINT anchor)
{
	static BOOL tracking = FALSE;
	if (tracking)
		return 0;
	tracking = TRUE;
	HIGHCONTRASTW contrast = {0};
	contrast.cbSize = sizeof(contrast);
	BOOL highContrast = SystemParametersInfoW(SPI_GETHIGHCONTRAST, sizeof(contrast), &contrast, 0) &&
		(contrast.dwFlags & HCF_HIGHCONTRASTON);
	TrayPopup popup = {0};
	// The transparent tray helper must follow the anchor monitor for native menu DPI.
	SetWindowPos(owner, NULL, anchor.x, anchor.y, 0, 0, SWP_NOSIZE | SWP_NOZORDER | SWP_NOACTIVATE);
	typedef UINT(WINAPI *GetDpiForWindowFunc)(HWND);
	GetDpiForWindowFunc getDpiForWindow =
		(GetDpiForWindowFunc)GetProcAddress(GetModuleHandleW(L"user32.dll"), "GetDpiForWindow");
	UINT dpi = getDpiForWindow != NULL ? getDpiForWindow(owner) : 96;
	BOOL styled = !highContrast && trayPreparePopup(&popup, owner, source, dpi ? dpi : 96, trayMenuIsDark());
	SetForegroundWindow(owner);
	UINT command = TrackPopupMenuEx(styled ? popup.menu : source,
		TPM_RETURNCMD | TPM_BOTTOMALIGN | TPM_LEFTALIGN | TPM_RIGHTBUTTON,
		anchor.x, anchor.y, owner, NULL);
	PostMessageW(owner, WM_NULL, 0, 0);
	trayDestroyPopup(&popup);
	tracking = FALSE;
	return command;
}

BOOL trayMeasureMenuItem(MEASUREITEMSTRUCT *measure)
{
	if (measure->CtlType != ODT_MENU || measure->itemData == 0)
		return FALSE;
	TrayMenuItem *item = (TrayMenuItem *)measure->itemData;
	measure->itemWidth = item->width;
	measure->itemHeight = item->height;
	return TRUE;
}

// Draw into the supplied item rectangle; the system retains popup chrome ownership.
BOOL trayDrawMenuItem(DRAWITEMSTRUCT *draw)
{
	if (draw->CtlType != ODT_MENU || draw->itemData == 0)
		return FALSE;
	TrayMenuItem *item = (TrayMenuItem *)draw->itemData;
	int saved = SaveDC(draw->hDC);
	BOOL selected = !item->separator && (draw->itemState & ODS_SELECTED);
	SetDCBrushColor(draw->hDC, selected ? item->colors.selected : item->colors.background);
	FillRect(draw->hDC, &draw->rcItem, GetStockObject(DC_BRUSH));
	RECT content = draw->rcItem;
	content.left += item->padding;
	content.right -= item->padding;
	if (item->separator)
	{
		content.top = (content.top + content.bottom) / 2;
		content.bottom = content.top + item->lineWidth;
		SetDCBrushColor(draw->hDC, item->colors.separator);
		FillRect(draw->hDC, &content, GetStockObject(DC_BRUSH));
	}
	else
	{
		SelectObject(draw->hDC, item->font);
		SetBkMode(draw->hDC, TRANSPARENT);
		SetTextColor(draw->hDC, (draw->itemState & (ODS_DISABLED | ODS_GRAYED)) ? item->colors.disabled : item->colors.text);
		UINT flags = DT_SINGLELINE | DT_VCENTER | DT_LEFT;
		if (draw->itemState & ODS_NOACCEL)
			flags |= DT_HIDEPREFIX;
		DrawTextW(draw->hDC, item->accessible.pszWText, -1, &content, flags);
		if ((draw->itemState & ODS_FOCUS) && !(draw->itemState & ODS_NOFOCUSRECT))
			DrawFocusRect(draw->hDC, &content);
	}
	RestoreDC(draw->hDC, saved);
	return TRUE;
}

// Owner-draw menus need explicit mnemonic handling, including cycling duplicate matches.
LRESULT trayMenuChar(HMENU menu, WCHAR character)
{
	int first = -1, next = -1, selected = -1, matches = 0;
	int count = GetMenuItemCount(menu);
	for (int i = 0; i < count; i++)
	{
		if (GetMenuState(menu, i, MF_BYPOSITION) & MF_HILITE)
			selected = i;
	}
	for (int i = 0; i < count; i++)
	{
		MENUITEMINFOW info = {0};
		info.cbSize = sizeof(info);
		info.fMask = MIIM_DATA | MIIM_STATE | MIIM_FTYPE;
		if (!GetMenuItemInfoW(menu, i, TRUE, &info) || !(info.fType & MFT_OWNERDRAW))
			continue;
		if ((info.fType & MFT_SEPARATOR) || (info.fState & MFS_DISABLED) || info.dwItemData == 0)
			continue;
		TrayMenuItem *item = (TrayMenuItem *)info.dwItemData;
		WCHAR *text = item->accessible.pszWText;
		WCHAR mnemonic = text[0];
		for (WCHAR *p = text; *p; p++)
		{
			if (*p == L'&' && p[1])
			{
				if (p[1] == L'&')
					p++;
				else
				{
					mnemonic = p[1];
					break;
				}
			}
		}
		if (towlower(mnemonic) == towlower(character))
		{
			if (first < 0)
				first = i;
			if (i > selected && next < 0)
				next = i;
			matches++;
		}
	}
	if (matches == 0)
		return MAKELRESULT(0, MNC_IGNORE);
	return MAKELRESULT(next >= 0 ? next : first, matches == 1 ? MNC_EXECUTE : MNC_SELECT);
}
