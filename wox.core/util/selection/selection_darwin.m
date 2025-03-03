#import <Cocoa/Cocoa.h>
#import <ApplicationServices/ApplicationServices.h>

// Get the selected text using Accessibility API
char* getSelectedTextA11y() {
    @autoreleasepool {
        // Get the current focused application
        AXUIElementRef systemWideElement = AXUIElementCreateSystemWide();
        AXUIElementRef focusedApp = NULL;
        AXError error = AXUIElementCopyAttributeValue(systemWideElement, kAXFocusedApplicationAttribute, (CFTypeRef *)&focusedApp);
        
        if (error != kAXErrorSuccess || focusedApp == NULL) {
            if (systemWideElement) CFRelease(systemWideElement);
            return NULL;
        }
        
        // Get the focused window
        AXUIElementRef focusedWindow = NULL;
        error = AXUIElementCopyAttributeValue(focusedApp, kAXFocusedWindowAttribute, (CFTypeRef *)&focusedWindow);
        
        if (error != kAXErrorSuccess || focusedWindow == NULL) {
            if (focusedApp) CFRelease(focusedApp);
            if (systemWideElement) CFRelease(systemWideElement);
            return NULL;
        }
        
        // Get the focused element
        AXUIElementRef focusedElement = NULL;
        error = AXUIElementCopyAttributeValue(focusedWindow, kAXFocusedUIElementAttribute, (CFTypeRef *)&focusedElement);
        
        if (error != kAXErrorSuccess || focusedElement == NULL) {
            if (focusedWindow) CFRelease(focusedWindow);
            if (focusedApp) CFRelease(focusedApp);
            if (systemWideElement) CFRelease(systemWideElement);
            return NULL;
        }
        
        // Try to get the selected text
        CFTypeRef selectedTextRef = NULL;
        error = AXUIElementCopyAttributeValue(focusedElement, kAXSelectedTextAttribute, &selectedTextRef);
        
        if (error != kAXErrorSuccess || selectedTextRef == NULL) {
            // If we can't get the selected text directly, try to get it from the selected text range
            AXValueRef selectedTextRangeRef = NULL;
            error = AXUIElementCopyAttributeValue(focusedElement, kAXSelectedTextRangeAttribute, (CFTypeRef *)&selectedTextRangeRef);
            
            if (error == kAXErrorSuccess && selectedTextRangeRef != NULL) {
                CFStringRef stringValue = NULL;
                error = AXUIElementCopyAttributeValue(focusedElement, kAXValueAttribute, (CFTypeRef *)&stringValue);
                
                if (error == kAXErrorSuccess && stringValue != NULL) {
                    CFRange range;
                    AXValueGetValue(selectedTextRangeRef, kAXValueCFRangeType, &range);
                    
                    if (range.length > 0) {
                        CFStringRef selectedText = CFStringCreateWithSubstring(kCFAllocatorDefault, stringValue, range);
                        if (selectedText) {
                            const char* cStr = CFStringGetCStringPtr(selectedText, kCFStringEncodingUTF8);
                            if (cStr == NULL) {
                                // If direct access fails, copy to a buffer
                                CFIndex length = CFStringGetLength(selectedText);
                                CFIndex maxSize = CFStringGetMaximumSizeForEncoding(length, kCFStringEncodingUTF8) + 1;
                                char* buffer = (char*)malloc(maxSize);
                                if (buffer && CFStringGetCString(selectedText, buffer, maxSize, kCFStringEncodingUTF8)) {
                                    CFRelease(selectedText);
                                    // Clean up other resources
                                    if (stringValue) CFRelease(stringValue);
                                    if (selectedTextRangeRef) CFRelease(selectedTextRangeRef);
                                    if (focusedElement) CFRelease(focusedElement);
                                    if (focusedWindow) CFRelease(focusedWindow);
                                    if (focusedApp) CFRelease(focusedApp);
                                    if (systemWideElement) CFRelease(systemWideElement);
                                    return buffer; // Caller must free this
                                }
                                if (buffer) free(buffer);
                            } else {
                                char* result = strdup(cStr);
                                CFRelease(selectedText);
                                // Clean up other resources
                                if (stringValue) CFRelease(stringValue);
                                if (selectedTextRangeRef) CFRelease(selectedTextRangeRef);
                                if (focusedElement) CFRelease(focusedElement);
                                if (focusedWindow) CFRelease(focusedWindow);
                                if (focusedApp) CFRelease(focusedApp);
                                if (systemWideElement) CFRelease(systemWideElement);
                                return result; // Caller must free this
                            }
                            CFRelease(selectedText);
                        }
                    }
                    if (stringValue) CFRelease(stringValue);
                }
                if (selectedTextRangeRef) CFRelease(selectedTextRangeRef);
            }
        } else {
            // We got the selected text directly
            const char* cStr = CFStringGetCStringPtr((CFStringRef)selectedTextRef, kCFStringEncodingUTF8);
            if (cStr == NULL) {
                // If direct access fails, copy to a buffer
                CFIndex length = CFStringGetLength((CFStringRef)selectedTextRef);
                CFIndex maxSize = CFStringGetMaximumSizeForEncoding(length, kCFStringEncodingUTF8) + 1;
                char* buffer = (char*)malloc(maxSize);
                if (buffer && CFStringGetCString((CFStringRef)selectedTextRef, buffer, maxSize, kCFStringEncodingUTF8)) {
                    CFRelease(selectedTextRef);
                    // Clean up other resources
                    if (focusedElement) CFRelease(focusedElement);
                    if (focusedWindow) CFRelease(focusedWindow);
                    if (focusedApp) CFRelease(focusedApp);
                    if (systemWideElement) CFRelease(systemWideElement);
                    return buffer; // Caller must free this
                }
                if (buffer) free(buffer);
            } else {
                char* result = strdup(cStr);
                CFRelease(selectedTextRef);
                // Clean up other resources
                if (focusedElement) CFRelease(focusedElement);
                if (focusedWindow) CFRelease(focusedWindow);
                if (focusedApp) CFRelease(focusedApp);
                if (systemWideElement) CFRelease(systemWideElement);
                return result; // Caller must free this
            }
            CFRelease(selectedTextRef);
        }
        
        // Clean up resources
        if (focusedElement) CFRelease(focusedElement);
        if (focusedWindow) CFRelease(focusedWindow);
        if (focusedApp) CFRelease(focusedApp);
        if (systemWideElement) CFRelease(systemWideElement);
    }
    
    return NULL;
}

// Get selected files from Finder
char* getSelectedFilesA11y() {
    @autoreleasepool {
        // Get the Finder process
        pid_t finderPID = 0;
        NSArray *apps = [NSWorkspace.sharedWorkspace runningApplications];
        for (NSRunningApplication *app in apps) {
            if ([[app bundleIdentifier] isEqualToString:@"com.apple.finder"]) {
                finderPID = [app processIdentifier];
                break;
            }
        }
        
        if (finderPID == 0) {
            return NULL;
        }
        
        // Get the Finder application element
        AXUIElementRef finderApp = AXUIElementCreateApplication(finderPID);
        if (finderApp == NULL) {
            return NULL;
        }
        
        // Get the focused window
        AXUIElementRef focusedWindow = NULL;
        AXError error = AXUIElementCopyAttributeValue(finderApp, kAXFocusedWindowAttribute, (CFTypeRef *)&focusedWindow);
        
        if (error != kAXErrorSuccess || focusedWindow == NULL) {
            if (finderApp) CFRelease(finderApp);
            return NULL;
        }
        
        // Get the selected items
        CFArrayRef selectedItems = NULL;
        error = AXUIElementCopyAttributeValue(focusedWindow, kAXSelectedChildrenAttribute, (CFTypeRef *)&selectedItems);
        
        if (error != kAXErrorSuccess || selectedItems == NULL) {
            if (focusedWindow) CFRelease(focusedWindow);
            if (finderApp) CFRelease(finderApp);
            return NULL;
        }
        
        // Get the file paths
        NSMutableString *paths = [NSMutableString string];
        CFIndex count = CFArrayGetCount(selectedItems);
        
        for (CFIndex i = 0; i < count; i++) {
            AXUIElementRef item = (AXUIElementRef)CFArrayGetValueAtIndex(selectedItems, i);
            CFStringRef filename = NULL;
            error = AXUIElementCopyAttributeValue(item, kAXFilenameAttribute, (CFTypeRef *)&filename);
            
            if (error == kAXErrorSuccess && filename != NULL) {
                [paths appendFormat:@"%@\n", (__bridge NSString *)filename];
                CFRelease(filename);
            }
        }
        
        // Clean up resources
        if (selectedItems) CFRelease(selectedItems);
        if (focusedWindow) CFRelease(focusedWindow);
        if (finderApp) CFRelease(finderApp);
        
        if ([paths length] > 0) {
            const char *cStr = [paths UTF8String];
            return strdup(cStr); // Caller must free this
        }
    }
    
    return NULL;
}

// Check if the application has accessibility permissions
bool hasAccessibilityPermissions() {
    return AXIsProcessTrustedWithOptions(NULL);
}

// Temporarily mute system alert sound
void muteAlertSound() {
    NSSound *sound = [NSSound soundNamed:@"Tink"];
    if (sound) {
        [sound setVolume:0.0];
    }
}

// Restore system alert sound
void restoreAlertSound() {
    NSSound *sound = [NSSound soundNamed:@"Tink"];
    if (sound) {
        [sound setVolume:1.0];
    }
} 