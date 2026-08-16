#import "macos_permission_darwin.h"

#import <Cocoa/Cocoa.h>
#import <CoreGraphics/CoreGraphics.h>
#include <string.h>

static NSString *const kWoxSystemSettingsBundleIdentifier = @"com.apple.systempreferences";

static CGFloat woxPermissionDesktopTop(NSArray<NSScreen *> *screens) {
  CGFloat top = 0;
  for (NSScreen *screen in screens) {
    top = MAX(top, NSMaxY(screen.frame));
  }
  return top;
}

// CGWindowList and AppKit use different Y directions and can use different
// origins on mixed-display desktops, so map through the display that owns most
// of the tracked window before handing coordinates to the shared UI runtime.
static NSRect woxPermissionAppKitFrame(CGRect quartzFrame, NSArray<NSScreen *> *screens, NSScreen **screenOut) {
  NSScreen *matched = nil;
  CGFloat bestArea = 0;
  CGRect matchedQuartz = CGRectZero;
  for (NSScreen *screen in screens) {
    NSNumber *number = screen.deviceDescription[@"NSScreenNumber"];
    if (number == nil) {
      continue;
    }
    CGRect displayQuartz = CGDisplayBounds((CGDirectDisplayID)number.unsignedIntValue);
    CGRect intersection = CGRectIntersection(displayQuartz, quartzFrame);
    CGFloat area = CGRectIsNull(intersection) ? 0 : intersection.size.width * intersection.size.height;
    if (area > bestArea) {
      bestArea = area;
      matched = screen;
      matchedQuartz = displayQuartz;
    }
  }
  if (screenOut != NULL) {
    *screenOut = matched;
  }
  if (matched == nil) {
    CGFloat desktopTop = woxPermissionDesktopTop(screens);
    return NSMakeRect(quartzFrame.origin.x, desktopTop - CGRectGetMaxY(quartzFrame), quartzFrame.size.width, quartzFrame.size.height);
  }
  CGFloat localX = quartzFrame.origin.x - matchedQuartz.origin.x;
  CGFloat localY = quartzFrame.origin.y - matchedQuartz.origin.y;
  return NSMakeRect(NSMinX(matched.frame) + localX,
                    NSMaxY(matched.frame) - localY - quartzFrame.size.height,
                    quartzFrame.size.width,
                    quartzFrame.size.height);
}

static void woxPermissionWriteRuntimeRect(NSRect frame, CGFloat desktopTop, WoxMacOSPermissionRect *out) {
  if (out == NULL) {
    return;
  }
  out->x = NSMinX(frame);
  out->y = desktopTop - NSMaxY(frame);
  out->width = NSWidth(frame);
  out->height = NSHeight(frame);
}

void woxMacOSPermissionOpenSettings(const char *anchor) {
  @autoreleasepool {
    if (anchor == NULL) {
      return;
    }
    NSString *anchorValue = [NSString stringWithUTF8String:anchor];
    if (anchorValue.length == 0) {
      return;
    }
    NSString *urlValue = [NSString stringWithFormat:@"x-apple.systempreferences:com.apple.preference.security?%@", anchorValue];
    NSURL *url = [NSURL URLWithString:urlValue];
    if (url != nil) {
      [NSWorkspace.sharedWorkspace openURL:url];
    }
  }
}

int woxMacOSPermissionSettingsWindow(WoxMacOSPermissionRect *out, WoxMacOSPermissionRect *workAreaOut) {
  @autoreleasepool {
    NSArray<NSRunningApplication *> *settingsApps = [NSRunningApplication runningApplicationsWithBundleIdentifier:kWoxSystemSettingsBundleIdentifier];
    if (settingsApps.count == 0) {
      return 0;
    }
    if (out == NULL) {
      return 1;
    }

    CFArrayRef windowInfoRef = CGWindowListCopyWindowInfo(kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements, kCGNullWindowID);
    if (windowInfoRef == NULL) {
      return 1;
    }
    NSArray<NSDictionary *> *windowInfo = CFBridgingRelease(windowInfoRef);
    pid_t currentPID = NSProcessInfo.processInfo.processIdentifier;
    CGRect bestFrame = CGRectZero;
    BOOL bestHasTitle = NO;
    CGFloat bestArea = -1;
    for (NSDictionary *info in windowInfo) {
      pid_t ownerPID = [info[(id)kCGWindowOwnerPID] intValue];
      if (ownerPID == currentPID || [info[(id)kCGWindowLayer] intValue] != 0 || [info[(id)kCGWindowAlpha] doubleValue] <= 0) {
        continue;
      }
      NSRunningApplication *owner = [NSRunningApplication runningApplicationWithProcessIdentifier:ownerPID];
      if (![owner.bundleIdentifier isEqualToString:kWoxSystemSettingsBundleIdentifier]) {
        continue;
      }
      NSDictionary *bounds = info[(id)kCGWindowBounds];
      CGRect frame = CGRectZero;
      if (bounds == nil || !CGRectMakeWithDictionaryRepresentation((__bridge CFDictionaryRef)bounds, &frame) || frame.size.width <= 200 || frame.size.height <= 200) {
        continue;
      }
      NSString *title = [info[(id)kCGWindowName] stringByTrimmingCharactersInSet:NSCharacterSet.whitespaceAndNewlineCharacterSet] ?: @"";
      BOOL hasTitle = title.length > 0;
      CGFloat area = frame.size.width * frame.size.height;
      if (bestArea < 0 || (hasTitle != bestHasTitle ? hasTitle : area > bestArea)) {
        bestFrame = frame;
        bestHasTitle = hasTitle;
        bestArea = area;
      }
    }
    if (bestArea < 0) {
      return 1;
    }
    NSArray<NSScreen *> *screens = NSScreen.screens;
    NSScreen *matchedScreen = nil;
    NSRect appKitFrame = woxPermissionAppKitFrame(bestFrame, screens, &matchedScreen);
    CGFloat desktopTop = woxPermissionDesktopTop(screens);
    woxPermissionWriteRuntimeRect(appKitFrame, desktopTop, out);
    if (matchedScreen != nil) {
      woxPermissionWriteRuntimeRect(matchedScreen.visibleFrame, desktopTop, workAreaOut);
    }
    return 1;
  }
}

char *woxMacOSPermissionApplicationPath(void) {
  @autoreleasepool {
    NSURL *bundleURL = NSRunningApplication.currentApplication.bundleURL;
    if (bundleURL == nil || ![bundleURL.pathExtension.lowercaseString isEqualToString:@"app"] || ![NSFileManager.defaultManager fileExistsAtPath:bundleURL.path]) {
      return NULL;
    }
    return strdup(bundleURL.path.UTF8String);
  }
}
