#pragma once

#import <ApplicationServices/ApplicationServices.h>
#import <Foundation/Foundation.h>

// AppKit can expose a standalone file panel as an ordinary AXStandardWindow.
BOOL woxIsAppKitFilePanel(AXUIElementRef window);

// Generic dialogs may expose only a search field. On ordinary windows it is
// not evidence of a filename input (e.g. Activity Monitor's process filter).
BOOL woxHasFilePickerControls(AXUIElementRef window, BOOL allowSearchField);
