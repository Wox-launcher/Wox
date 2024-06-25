#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h> // 引入 WebView 框架

void createAndShowWindow(const char *url) {
    NSRect frame = NSMakeRect(0, 0, 400, 800);
    NSWindow *window = [[NSWindow alloc] initWithContentRect:frame
                                                    styleMask:NSWindowStyleMaskTitled | NSWindowStyleMaskResizable | NSWindowStyleMaskClosable
                                                      backing:NSBackingStoreBuffered
                                                        defer:NO];

    [window setTitle:@"My Window"];
    [window makeKeyAndOrderFront:nil];

    // use default WKWebViewConfiguration
    WKWebViewConfiguration *configuration = [[WKWebViewConfiguration alloc] init];


    WKWebView *webView = [[WKWebView alloc] initWithFrame:frame configuration:configuration];
    webView.customUserAgent = @"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36";
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

    // 创建一个新线程并运行 NSRunLoop
    dispatch_async(dispatch_get_global_queue(DISPATCH_QUEUE_PRIORITY_DEFAULT, 0), ^{
       [[NSRunLoop currentRunLoop] run];
    });
}