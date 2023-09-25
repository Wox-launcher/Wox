#import "webview.h"
#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h> // 引入 WebView 框架

void createAndShowWindow(const char *url) {
    NSRect frame = NSMakeRect(0, 0, 800, 600);
    NSWindow *window = [[NSWindow alloc] initWithContentRect:frame
                                                    styleMask:NSWindowStyleMaskTitled | NSWindowStyleMaskResizable
                                                      backing:NSBackingStoreBuffered
                                                        defer:NO];

    [window setTitle:@"My Window"];
    [window makeKeyAndOrderFront:nil];

    // 创建 WebView 并添加到窗口
    WKWebView *webView = [[WKWebView alloc] initWithFrame:frame];
    [window.contentView addSubview:webView];

    // 将传递的URL参数转换为NSString
    NSString *urlString = [NSString stringWithUTF8String:url];
    NSURL *webURL = [NSURL URLWithString:urlString];

    // 设置 WebView 加载的 URL
    if (webURL) {
        NSURLRequest *request = [NSURLRequest requestWithURL:webURL];
        [webView loadRequest:request];
    }

    // 设置窗口显示在所有 Space 中
    [window setCollectionBehavior:NSWindowCollectionBehaviorCanJoinAllSpaces];

    // 设置窗口级别，以便在最上层显示
    [window setLevel:NSMainMenuWindowLevel + 1];

    // 运行应用程序事件循环
    [NSApp run];
}