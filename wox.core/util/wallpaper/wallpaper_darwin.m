#import <Cocoa/Cocoa.h>
#import <CoreGraphics/CoreGraphics.h>
#include <stdlib.h>
#include <string.h>

static BOOL woxIsSupportedWallpaperImagePath(NSString *path) {
  if (path.length == 0) {
    return NO;
  }
  static NSSet<NSString *> *extensions;
  static dispatch_once_t onceToken;
  dispatch_once(&onceToken, ^{
    // The process-lifetime static must own the set under manual reference counting.
    extensions = [[NSSet alloc] initWithArray:@[
      @"bmp", @"gif", @"heic", @"jpeg", @"jpg", @"png", @"tif", @"tiff",
      @"webp"
    ]];
  });
  return [extensions containsObject:path.pathExtension.lowercaseString];
}

static NSString *woxNormalizedWallpaperPath(NSString *rawPath) {
  if (rawPath.length == 0) {
    return nil;
  }
  if ([rawPath hasPrefix:@"file://"]) {
    return [NSURL URLWithString:rawPath].path;
  }
  return rawPath;
}

static NSString *woxExistingWallpaperImagePath(NSString *rawPath) {
  NSString *path = woxNormalizedWallpaperPath(rawPath);
  if (!woxIsSupportedWallpaperImagePath(path) ||
      ![[NSFileManager defaultManager] fileExistsAtPath:path]) {
    return nil;
  }
  return path;
}

// Read AppKit-owned screen state on the main thread and copy only plain bytes across the boundary.
static NSString *woxWorkspaceWallpaperPath(void) {
  __block char *pathBytes = NULL;
  void (^resolve)(void) = ^{
    NSMutableArray<NSScreen *> *screens =
        [NSMutableArray arrayWithArray:[NSScreen screens]];
    NSScreen *mainScreen = [NSScreen mainScreen];
    if (mainScreen != nil) {
      [screens removeObject:mainScreen];
      [screens insertObject:mainScreen atIndex:0];
    }
    for (NSScreen *screen in screens) {
      NSURL *url =
          [[NSWorkspace sharedWorkspace] desktopImageURLForScreen:screen];
      if (url.path.length > 0) {
        pathBytes = strdup(url.path.fileSystemRepresentation);
        return;
      }
    }
  };
  if ([NSThread isMainThread]) {
    resolve();
  } else {
    dispatch_sync(dispatch_get_main_queue(), resolve);
  }
  NSString *path =
      pathBytes == NULL ? nil : [NSString stringWithUTF8String:pathBytes];
  free(pathBytes);
  return path;
}

// Walk wallpaper store values, including encoded provider configuration plists.
static BOOL woxWallpaperValueContainsPath(id value, NSString *targetPath) {
  if ([value isKindOfClass:[NSString class]]) {
    NSString *path = woxNormalizedWallpaperPath(value);
    return [path isEqualToString:targetPath];
  }
  if ([value isKindOfClass:[NSData class]]) {
    id decoded = [NSPropertyListSerialization
        propertyListWithData:value
                     options:NSPropertyListImmutable
                      format:nil
                       error:nil];
    return woxWallpaperValueContainsPath(decoded, targetPath);
  }
  if ([value isKindOfClass:[NSArray class]]) {
    for (id item in value) {
      if (woxWallpaperValueContainsPath(item, targetPath)) {
        return YES;
      }
    }
    return NO;
  }
  if ([value isKindOfClass:[NSDictionary class]]) {
    for (id item in [value allValues]) {
      if (woxWallpaperValueContainsPath(item, targetPath)) {
        return YES;
      }
    }
  }
  return NO;
}

// Find the provider attached to the stale source path so only its cache is used.
static NSString *woxWallpaperProviderForPath(id value, NSString *targetPath) {
  if ([value isKindOfClass:[NSDictionary class]]) {
    NSDictionary *dictionary = value;
    NSString *provider =
        [dictionary[@"Provider"] isKindOfClass:[NSString class]]
            ? dictionary[@"Provider"]
            : nil;
    if (provider != nil &&
        (woxWallpaperValueContainsPath(dictionary[@"Files"], targetPath) ||
         woxWallpaperValueContainsPath(dictionary[@"Configuration"],
                                       targetPath))) {
      return provider;
    }
    for (id item in dictionary.allValues) {
      NSString *nestedProvider =
          woxWallpaperProviderForPath(item, targetPath);
      if (nestedProvider != nil) {
        return nestedProvider;
      }
    }
  } else if ([value isKindOfClass:[NSArray class]]) {
    for (id item in value) {
      NSString *provider =
          woxWallpaperProviderForPath(item, targetPath);
      if (provider != nil) {
        return provider;
      }
    }
  }
  return nil;
}

static NSString *woxWallpaperAgentCacheDirectoryName(NSString *provider) {
  if ([provider isEqualToString:@"com.apple.wallpaper.choice.image"] ||
      [provider isEqualToString:@"com.apple.wallpaper.extension.image"]) {
    return @"extension-com.apple.wallpaper.extension.image";
  }
  if ([provider isEqualToString:@"com.apple.wallpaper.choice.aerials"] ||
      [provider isEqualToString:@"com.apple.wallpaper.extension.aerials"] ||
      [provider isEqualToString:@"com.apple.wallpaper.choice.screen-saver"]) {
    return @"extension-com.apple.wallpaper.extension.aerials";
  }
  return nil;
}

// Resolve the newest rendered cache only for the provider that owned the stale source.
static NSString *woxWallpaperAgentCachePathForSource(NSString *sourcePath) {
  NSString *storePath =
      [NSHomeDirectory() stringByAppendingPathComponent:
                             @"Library/Application Support/com.apple.wallpaper/"
                             @"Store/Index.plist"];
  NSData *storeData = [NSData dataWithContentsOfFile:storePath];
  id store =
      storeData == nil
          ? nil
          : [NSPropertyListSerialization propertyListWithData:storeData
                                                      options:
                                                          NSPropertyListImmutable
                                                       format:nil
                                                        error:nil];
  NSString *provider = woxWallpaperProviderForPath(store, sourcePath);
  NSString *directoryName =
      woxWallpaperAgentCacheDirectoryName(provider);
  if (directoryName == nil) {
    return nil;
  }

  NSString *directoryPath =
      [[NSHomeDirectory()
          stringByAppendingPathComponent:
              @"Library/Containers/com.apple.wallpaper.agent/Data/Library/"
              @"Caches/com.apple.wallpaper.caches"]
          stringByAppendingPathComponent:directoryName];
  NSFileManager *fileManager = [NSFileManager defaultManager];
  NSArray<NSString *> *files =
      [fileManager contentsOfDirectoryAtPath:directoryPath error:nil];
  NSString *latestPath = nil;
  NSDate *latestModificationDate = nil;
  for (NSString *file in files) {
    NSString *path = [directoryPath stringByAppendingPathComponent:file];
    if (!woxIsSupportedWallpaperImagePath(path)) {
      continue;
    }
    NSDictionary *attributes =
        [fileManager attributesOfItemAtPath:path error:nil];
    if (![attributes[NSFileType] isEqualToString:NSFileTypeRegular]) {
      continue;
    }
    NSDate *modificationDate = attributes[NSFileModificationDate];
    if (modificationDate != nil &&
        (latestModificationDate == nil ||
         [modificationDate compare:latestModificationDate] ==
             NSOrderedDescending)) {
      latestPath = path;
      latestModificationDate = modificationDate;
    }
  }
  return latestPath;
}

// Keep the fallback capture on the display whose wallpaper AppKit attempted to resolve.
static BOOL woxWallpaperWindowMatchesMainDisplay(NSDictionary *window) {
  NSDictionary *boundsDictionary = window[(id)kCGWindowBounds];
  CGRect windowBounds = CGRectZero;
  if (![boundsDictionary isKindOfClass:[NSDictionary class]] ||
      !CGRectMakeWithDictionaryRepresentation(
          (__bridge CFDictionaryRef)boundsDictionary, &windowBounds)) {
    return NO;
  }
  CGRect mainDisplayBounds = CGDisplayBounds(CGMainDisplayID());
  return fabs(windowBounds.origin.x - mainDisplayBounds.origin.x) < 1 &&
         fabs(windowBounds.origin.y - mainDisplayBounds.origin.y) < 1 &&
         fabs(windowBounds.size.width - mainDisplayBounds.size.width) < 1 &&
         fabs(windowBounds.size.height - mainDisplayBounds.size.height) < 1;
}

// Capture the composited wallpaper when macOS retained it after deleting the source.
static NSString *woxCaptureCurrentWallpaperWindow(void) {
  CFArrayRef rawWindows = CGWindowListCopyWindowInfo(
      kCGWindowListOptionOnScreenOnly, kCGNullWindowID);
  NSArray<NSDictionary *> *windows = CFBridgingRelease(rawWindows);
  NSNumber *wallpaperWindowNumber = nil;
  for (NSDictionary *window in windows) {
    NSString *owner = window[(id)kCGWindowOwnerName];
    NSString *name = window[(id)kCGWindowName];
    NSNumber *sharingState = window[(id)kCGWindowSharingState];
    if ([owner isEqualToString:@"Dock"] &&
        [name hasPrefix:@"Wallpaper-"] && sharingState.integerValue != 0 &&
        woxWallpaperWindowMatchesMainDisplay(window)) {
      wallpaperWindowNumber = window[(id)kCGWindowNumber];
      break;
    }
  }
  if (wallpaperWindowNumber == nil) {
    return nil;
  }

  NSString *cachePath =
      [NSTemporaryDirectory() stringByAppendingPathComponent:
                                  @"wox-system-wallpaper.png"];
  [[NSFileManager defaultManager] removeItemAtPath:cachePath error:nil];
  NSTask *task = [[NSTask alloc] init];
  task.executableURL = [NSURL fileURLWithPath:@"/usr/sbin/screencapture"];
  task.arguments = @[
    @"-x", @"-l", wallpaperWindowNumber.stringValue, cachePath
  ];
  task.standardOutput = [NSPipe pipe];
  task.standardError = [NSPipe pipe];
  if (![task launchAndReturnError:nil]) {
    return nil;
  }
  [task waitUntilExit];
  if (task.terminationStatus != 0) {
    return nil;
  }
  return woxExistingWallpaperImagePath(cachePath);
}

// Resolve the active wallpaper through source, provider cache, and compositor fallbacks.
char *woxGetSystemWallpaperPath(void) {
  @autoreleasepool {
    NSString *sourcePath = woxWorkspaceWallpaperPath();
    NSString *path = woxExistingWallpaperImagePath(sourcePath);
    if (path == nil) {
      path = woxWallpaperAgentCachePathForSource(sourcePath);
    }
    if (path == nil) {
      path = woxCaptureCurrentWallpaperWindow();
    }
    return path.length == 0 ? NULL : strdup(path.fileSystemRepresentation);
  }
}
