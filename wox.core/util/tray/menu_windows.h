#ifndef WOX_TRAY_MENU_WINDOWS_H
#define WOX_TRAY_MENU_WINDOWS_H

#include <windows.h>

UINT trayTrackPopupMenu(HWND owner, HMENU source, POINT anchor);
BOOL trayMeasureMenuItem(MEASUREITEMSTRUCT *measure);
BOOL trayDrawMenuItem(DRAWITEMSTRUCT *draw);
LRESULT trayMenuChar(HMENU menu, WCHAR character);

#endif
