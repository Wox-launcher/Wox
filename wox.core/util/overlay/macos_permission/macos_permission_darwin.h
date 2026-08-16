#ifndef WOX_MACOS_PERMISSION_OVERLAY_DARWIN_H
#define WOX_MACOS_PERMISSION_OVERLAY_DARWIN_H

typedef struct {
  double x;
  double y;
  double width;
  double height;
} WoxMacOSPermissionRect;

void woxMacOSPermissionOpenSettings(const char *anchor);
int woxMacOSPermissionSettingsWindow(WoxMacOSPermissionRect *out, WoxMacOSPermissionRect *workAreaOut);
char *woxMacOSPermissionApplicationPath(void);

#endif
