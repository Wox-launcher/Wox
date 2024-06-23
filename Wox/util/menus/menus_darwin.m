#import <Cocoa/Cocoa.h>
#import <AppKit/AppKit.h>

typedef void (^MenuItemCallback)(AXUIElementRef menuItem, NSString *fullTitle);

void getMenuItemTitles(AXUIElementRef menuItem, NSMutableArray *titles, NSString *parentTitle, MenuItemCallback callback) {
    @autoreleasepool {
        CFStringRef roleRef = NULL;
        NSString *role = @"";
        AXError roleError = AXUIElementCopyAttributeValue(menuItem, kAXRoleAttribute, (CFTypeRef *)&roleRef);
        if (roleError == kAXErrorSuccess) {
            role = CFBridgingRelease(roleRef);
        }

        CFStringRef titleRef = NULL;
        NSString *title = @"";
        AXError titleError = AXUIElementCopyAttributeValue(menuItem, kAXTitleAttribute, (CFTypeRef *)&titleRef);
        if (titleError == kAXErrorSuccess) {
            title = CFBridgingRelease(titleRef);
        }

        if ([title isEqualToString:@"Apple"]) {
            return;
        }

        CFArrayRef children = NULL;
        AXError childrenError = AXUIElementCopyAttributeValue(menuItem, kAXChildrenAttribute, (CFTypeRef *)&children);
        if (childrenError == kAXErrorSuccess) {
            CFIndex count = CFArrayGetCount(children);

            if (count == 0) {
                if (title.length > 0) {
                    NSString *fullTitle = [NSString stringWithFormat:@"%@->%@", parentTitle, title];
                    if (parentTitle.length == 0) {
                        fullTitle = title;
                    }
                    [titles addObject:fullTitle];
                    if (callback) {
                        callback(menuItem, fullTitle);
                    }
                }
            } else {
                for (CFIndex i = 0; i < count; i++) {
                    AXUIElementRef child = CFArrayGetValueAtIndex(children, i);
                    NSString *newParentTitle;
                    if (title.length == 0) {
                        newParentTitle = parentTitle;
                    } else {
                        newParentTitle = [NSString stringWithFormat:@"%@->%@", parentTitle, title];
                        if (parentTitle.length == 0) {
                            newParentTitle = title;
                        }
                    }
                    getMenuItemTitles(child, titles, newParentTitle, callback);
                }
            }

            CFRelease(children);
        }
    }
}

char** getMenuItems(int* count) {
    @autoreleasepool {
        NSRunningApplication *activeApp = [[NSWorkspace sharedWorkspace] frontmostApplication];
        pid_t pid = [activeApp processIdentifier];

        AXUIElementRef app = AXUIElementCreateApplication(pid);
        if (!app) {
            NSLog(@"Failed to create AXUIElementRef for app with pid %d", pid);
            *count = 0;
            return NULL;
        }

        AXUIElementRef menuBar;
        AXError error = AXUIElementCopyAttributeValue(app, kAXMenuBarAttribute, (CFTypeRef *)&menuBar);
        if (error != kAXErrorSuccess) {
            NSLog(@"Failed to get menu bar for app with pid %d", pid);
            CFRelease(app);
            *count = 0;
            return NULL;
        }

        NSMutableArray *titles = [NSMutableArray array];
        getMenuItemTitles(menuBar, titles, @"", nil);

        CFRelease(menuBar);
        CFRelease(app);

        char **items = malloc(sizeof(char*) * [titles count]);
        if (items == NULL) {
            NSLog(@"Failed to allocate memory for items");
            *count = 0;
            return NULL;
        }

        *count = (int)[titles count];

        for (int i = 0; i < [titles count]; i++) {
            items[i] = strdup([titles[i] UTF8String]);
            if (items[i] == NULL) {
                NSLog(@"Failed to allocate memory for item title at index %d", i);
                for (int j = 0; j < i; j++) {
                    free(items[j]);
                }
                free(items);
                *count = 0;
                return NULL;
            }
        }

        return items;
    }
}

void performMenuAction(const char* title) {
    @autoreleasepool {
        NSString *menuPath = [NSString stringWithUTF8String:title];

        NSRunningApplication *activeApp = [[NSWorkspace sharedWorkspace] frontmostApplication];
        pid_t pid = [activeApp processIdentifier];

        AXUIElementRef app = AXUIElementCreateApplication(pid);
        if (app == NULL) {
            NSLog(@"Failed to create AXUIElementRef for app with pid %d", pid);
            return;
        }

        AXUIElementRef menuBar;
        AXError error = AXUIElementCopyAttributeValue(app, kAXMenuBarAttribute, (CFTypeRef *)&menuBar);
        if (error != kAXErrorSuccess) {
            NSLog(@"Failed to get menu bar for app with pid %d", pid);
            CFRelease(app);
            return;
        }

        NSMutableArray *titles = [NSMutableArray array];
        __block BOOL found = NO;
        getMenuItemTitles(menuBar, titles, @"", ^(AXUIElementRef menuItem, NSString *fullTitle) {
            if ([fullTitle isEqualToString:menuPath]) {
                AXUIElementPerformAction(menuItem, kAXPressAction);
                found = YES;
            }
        });

        if (!found) {
            NSLog(@"Menu item not found: %@", menuPath);
        }

        CFRelease(menuBar);
        CFRelease(app);
    }
}