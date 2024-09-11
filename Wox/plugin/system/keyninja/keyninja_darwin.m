#import <Cocoa/Cocoa.h>
#import <ApplicationServices/ApplicationServices.h>

// 在文件顶部声明一个全局字典
static NSMutableDictionary *elementShortcuts;


extern void logMessage(const char *message);

// 新增函数：限制层级的getChildElements
void getChildElementsLimited(AXUIElementRef element, NSMutableArray *elements, int maxDepth) {
    if (maxDepth <= 0) return;
    
    CFArrayRef children;
    AXError error = AXUIElementCopyAttributeValue(element, kAXChildrenAttribute, (CFTypeRef *)&children);
    
    if (error == kAXErrorSuccess && children) {
        for (CFIndex i = 0; i < CFArrayGetCount(children); i++) {
            AXUIElementRef child = CFArrayGetValueAtIndex(children, i);
            [elements addObject:(__bridge id)child];
            getChildElementsLimited(child, elements, maxDepth - 1);
        }
        CFRelease(children);
    }
}

// 递归获取子元素
void getChildElements(AXUIElementRef element, NSMutableArray *elements) {
    CFArrayRef children;
    AXError error = AXUIElementCopyAttributeValue(element, kAXChildrenAttribute, (CFTypeRef *)&children);
    
    if (error == kAXErrorSuccess && children) {
        char message[100];
        snprintf(message, sizeof(message), "获取到 %ld 个子元素\n", CFArrayGetCount(children));
        logMessage(message);
        
        for (CFIndex i = 0; i < CFArrayGetCount(children); i++) {
            AXUIElementRef child = CFArrayGetValueAtIndex(children, i);
            [elements addObject:(__bridge id)child];
            getChildElements(child, elements);
        }
        CFRelease(children);
    } else {
        char message[100];
        snprintf(message, sizeof(message), "无法获取子元素，错误代码：%d\n", error);
        logMessage(message);
    }
}

// 修改getVisibleUIElements函数
NSArray* getVisibleUIElements() {
    logMessage("开始获取可见 UI 元素\n");
    NSMutableArray *elements = [NSMutableArray array];
    CFArrayRef windows = CGWindowListCopyWindowInfo(kCGWindowListOptionOnScreenOnly | kCGWindowListExcludeDesktopElements, kCGNullWindowID);
    
    char message[100];
    snprintf(message, sizeof(message), "获取到 %ld 个窗口\n", CFArrayGetCount(windows));
    logMessage(message);
    
    for (NSDictionary *window in (__bridge NSArray *)windows) {
        // 获取窗口的 AXUIElement
        pid_t pid = [window[(id)kCGWindowOwnerPID] intValue];
        NSString *windowName = window[(id)kCGWindowName];
        NSString *ownerName = window[(id)kCGWindowOwnerName];
        
        char windowInfo[256];
        snprintf(windowInfo, sizeof(windowInfo), "处理窗口：PID=%d, 名称='%s', 所有者='%s'\n", pid, [windowName UTF8String], [ownerName UTF8String]);
        logMessage(windowInfo);
        
        AXUIElementRef app = AXUIElementCreateApplication(pid);
        AXUIElementRef appWindow;
        AXError error = AXUIElementCopyAttributeValue(app, kAXFocusedWindowAttribute, (CFTypeRef *)&appWindow);
        
        if (error == kAXErrorSuccess && appWindow) {
            logMessage("成功获取窗口的 AXUIElement\n");
            
            NSMutableArray *windowElements = [NSMutableArray array];
            getChildElementsLimited(appWindow, windowElements, 3); // 限制层级为3
            
            char windowElementsCount[100];
            snprintf(windowElementsCount, sizeof(windowElementsCount), "窗口 '%s' 包含 %lu 个子元素\n", [windowName UTF8String], (unsigned long)windowElements.count);
            logMessage(windowElementsCount);
            
            [elements addObjectsFromArray:windowElements];
            
            CFRelease(appWindow);
        } else {
            char errorMessage[100];
            snprintf(errorMessage, sizeof(errorMessage), "无法获取窗口的 AXUIElement，错误代码：%d\n", error);
            logMessage(errorMessage);
        }
        
        CFRelease(app);
    }
    
    CFRelease(windows);
    char elementCount[100];
    snprintf(elementCount, sizeof(elementCount), "总共获取到 %lu 个可见 UI 元素\n", (unsigned long)elements.count);
    logMessage(elementCount);
    return elements;
}



// 检查元素是否支持特定属性
BOOL elementSupportsAttribute(AXUIElementRef element, CFStringRef attribute) {
    CFArrayRef attributeNames;
    if (AXUIElementCopyAttributeNames(element, &attributeNames) == kAXErrorSuccess) {
        BOOL supported = CFArrayContainsValue(attributeNames, CFRangeMake(0, CFArrayGetCount(attributeNames)), attribute);
        CFRelease(attributeNames);
        return supported;
    }
    return NO;
}

// 检查元素是否有效
BOOL isElementValid(AXUIElementRef element) {
    pid_t pid;
    return (AXUIElementGetPid(element, &pid) == kAXErrorSuccess);
}


// 修改assignShortcuts函数
void assignShortcuts(NSArray *elements) {
    if (!elementShortcuts) {
        elementShortcuts = [NSMutableDictionary dictionary];
    }
    
    char shortcut[3] = "aa";
    for (int i = 0; i < [elements count]; i++) {
        AXUIElementRef element = (__bridge AXUIElementRef)[elements objectAtIndex:i];
        
        // 检查元素是否有效
        if (!isElementValid(element)) {
            char invalidElement[100];
            snprintf(invalidElement, sizeof(invalidElement), "元素 %d 无效，跳过\n", i);
            logMessage(invalidElement);
            continue;
        }
        
        NSString *shortcutString = [NSString stringWithUTF8String:shortcut];
        
        // 使用元素的内存地址作为键
        NSString *key = [NSString stringWithFormat:@"%p", element];
        [elementShortcuts setObject:shortcutString forKey:key];
        
        char successMessage[100];
        snprintf(successMessage, sizeof(successMessage), "成功为元素 %d 设置自定义快捷键 '%s'\n", i, shortcut);
        logMessage(successMessage);
        
        // 更新编号
        if (shortcut[1] == 'z') {
            shortcut[0]++;
            shortcut[1] = 'a';
        } else {
            shortcut[1]++;
        }
    }
}

// 修改showShortcuts函数
void showShortcuts(NSArray *elements) {
    char message[100];
    snprintf(message, sizeof(message), "开始显示快捷键，元素数量：%lu\n", (unsigned long)[elements count]);
    logMessage(message);
    
    int displayedCount = 0;
    for (int i = 0; i < [elements count] && displayedCount < 30; i++) {
        AXUIElementRef element = (__bridge AXUIElementRef)[elements objectAtIndex:i];
        char elementMessage[100];
        snprintf(elementMessage, sizeof(elementMessage), "处理元素 %d：%p\n", i, element);
        logMessage(elementMessage);
        
        // 检查元素是否有效
        if (!isElementValid(element)) {
            char invalidElement[100];
            snprintf(invalidElement, sizeof(invalidElement), "元素 %d 无效，跳过\n", i);
            logMessage(invalidElement);
            continue;
        }
        
        // 获取自定义快捷键
        NSString *key = [NSString stringWithFormat:@"%p", element];
        NSString *shortcutString = [elementShortcuts objectForKey:key];
        
        if (shortcutString) {
            // 获取元素的位置和大小
            CGPoint position;
            CGSize size;
            AXValueRef positionValue, sizeValue;
            
            AXUIElementCopyAttributeValue(element, kAXPositionAttribute, (CFTypeRef *)&positionValue);
            AXUIElementCopyAttributeValue(element, kAXSizeAttribute, (CFTypeRef *)&sizeValue);
            
            if (positionValue && sizeValue) {
                AXValueGetValue(positionValue, kAXValueCGPointType, &position);
                AXValueGetValue(sizeValue, kAXValueCGSizeType, &size);
                
                char positionSizeMessage[100];
                snprintf(positionSizeMessage, sizeof(positionSizeMessage), "元素位置：(%f, %f)，大小：(%f, %f)\n", position.x, position.y, size.width, size.height);
                logMessage(positionSizeMessage);
                
                // 创建一个小的、半透明的背景窗口
                CGFloat tagWidth = 30;  // 标签宽度
                CGFloat tagHeight = 20; // 标签高度
                NSRect windowRect = NSMakeRect(position.x, [[NSScreen mainScreen] frame].size.height - position.y - tagHeight, tagWidth, tagHeight);
                char windowCreationMessage[100];
                snprintf(windowCreationMessage, sizeof(windowCreationMessage), "创建窗口，位置：(%f, %f)，大小：(%f, %f)\n", windowRect.origin.x, windowRect.origin.y, windowRect.size.width, windowRect.size.height);
                logMessage(windowCreationMessage);
                
                NSWindow *overlayWindow = [[NSWindow alloc] initWithContentRect:windowRect
                                                                      styleMask:NSWindowStyleMaskBorderless
                                                                        backing:NSBackingStoreBuffered
                                                                          defer:NO];
                [overlayWindow setLevel:NSFloatingWindowLevel];
                [overlayWindow setBackgroundColor:[NSColor clearColor]];
                [overlayWindow setOpaque:NO];
                
                // 修改标签样式
                NSTextField *label = [[NSTextField alloc] initWithFrame:NSMakeRect(0, 0, tagWidth, tagHeight)];
                [label setStringValue:shortcutString];
                [label setAlignment:NSTextAlignmentCenter];
                [label setTextColor:[NSColor blackColor]];
                [label setFont:[NSFont boldSystemFontOfSize:12]];
                [label setBackgroundColor:[NSColor yellowColor]];
                [label setBezeled:NO];
                [label setBordered:YES];
                [label setEditable:NO];
                [label setSelectable:NO];
                [label setWantsLayer:YES];
                label.layer.cornerRadius = 4.0;
                label.layer.masksToBounds = YES;

                NSView *contentView = [[NSView alloc] initWithFrame:NSMakeRect(0, 0, tagWidth, tagHeight)];
                [contentView addSubview:label];

                [overlayWindow setContentView:contentView];
                [overlayWindow makeKeyAndOrderFront:nil];
                displayedCount++;

                logMessage("创建了覆盖窗口和标签\n");
                
                CFRelease(positionValue);
                CFRelease(sizeValue);
            } else {
                logMessage("无法获取元素的位置或大小\n");
            }
        } else {
            char errorMessage[100];
            snprintf(errorMessage, sizeof(errorMessage), "无法获取元素 %d 的自定义快捷键\n", i);
            logMessage(errorMessage);
        }
        
        if (displayedCount >= 30) {
            logMessage("已达到最大显示窗体数量(30个)\n");
            break;
        }
    }
    
    logMessage("快捷键显示完成\n");
}

// 模拟点击指定编号的元素
void clickElementWithShortcut(NSString *shortcut, NSArray *elements) {
    for (id element in elements) {
        CFStringRef elementShortcut;
        AXUIElementCopyAttributeValue((__bridge AXUIElementRef)element, CFSTR("AXShortcut"), (CFTypeRef *)&elementShortcut);
        
        if (elementShortcut && [(__bridge NSString *)elementShortcut isEqualToString:shortcut]) {
            AXUIElementPerformAction((__bridge AXUIElementRef)element, kAXPressAction);
            CFRelease(elementShortcut);
            break;
        }
        
        if (elementShortcut) {
            CFRelease(elementShortcut);
        }
    }
}