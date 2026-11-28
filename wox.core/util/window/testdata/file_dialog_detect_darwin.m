#import <Cocoa/Cocoa.h>
#import <ApplicationServices/ApplicationServices.h>
#import <assert.h>

// Dictionary nodes stand in for AX elements so this test needs no desktop or AX permission.
static AXError copyFixtureAttribute(AXUIElementRef element, CFStringRef attribute, CFTypeRef *value) {
    NSDictionary *node = (__bridge NSDictionary *)element;
    if (CFEqual(attribute, kAXChildrenAttribute)) {
        assert(![node[@"AXRole"] isEqualToString:@"AXTable"] && "dialog detection read the data rows");
    }
    id item = node[(__bridge NSString *)attribute];
    *value = item ? CFRetain((__bridge CFTypeRef)item) : NULL;
    return item ? kAXErrorSuccess : kAXErrorNoValue;
}

#define AXUIElementCopyAttributeValue copyFixtureAttribute
#define AXUIElementGetTypeID CFDictionaryGetTypeID
#import "../window_darwin.m"

int main(void) {
    @autoreleasepool {
        NSDictionary *table = @{@"AXRole": @"AXTable"};
        NSDictionary *search = @{@"AXRole": @"AXTextField", @"AXSubrole": @"AXSearchField"};
        NSDictionary *monitor = @{@"AXRole": @"AXWindow", @"AXSubrole": @"AXStandardWindow", @"AXChildren": @[table, search]};
        assert(!isLikelyOpenSaveDialogWindowElement((AXUIElementRef)monitor));

        NSMutableDictionary *panel = [monitor mutableCopy];
        panel[@"AXIdentifier"] = @"open-panel";
        assert(isLikelyOpenSaveDialogWindowElement((AXUIElementRef)panel));
        assert(!isOpenSaveDialogSelectFolderWindow((AXUIElementRef)panel));
        panel[@"AXChildren"] = @[table, @{@"AXRole": @"AXGroup", @"AXChildren": @[@{@"AXRole": @"AXButton", @"AXTitle": @"Choose Folder"}]}];
        assert(isOpenSaveDialogSelectFolderWindow((AXUIElementRef)panel));

        [panel removeObjectForKey:@"AXIdentifier"];
        panel[@"AXChildren"] = @[table, @{@"AXRole": @"AXTextField"}];
        assert(isLikelyOpenSaveDialogWindowElement((AXUIElementRef)panel));
        [panel release];
    }
    return 0;
}
