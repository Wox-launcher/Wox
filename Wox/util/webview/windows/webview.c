#include <Windows.h>
#include <WebView2.h>

HWND g_hWnd;
IWebView2WebView* g_webView;

LRESULT CALLBACK WndProc(HWND hWnd, UINT message, WPARAM wParam, LPARAM lParam) {
    if (g_webView) {
        IWebView2WebView* webView = g_webView;
        switch (message) {
            case WM_SIZE: {
                RECT bounds;
                GetClientRect(hWnd, &bounds);
                webView->put_Bounds(bounds);
                break;
            }
            case WM_CLOSE: {
                DestroyWindow(hWnd);
                break;
            }
            default:
                return DefWindowProc(hWnd, message, wParam, lParam);
        }
    } else {
        return DefWindowProc(hWnd, message, wParam, lParam);
    }
    return 0;
}

void createAndShowWindow(const char* url) {
    // Initialize COM
    CoInitialize(NULL);

    // Register the window class
    WNDCLASSEX wcex = {sizeof(WNDCLASSEX), CS_HREDRAW | CS_VREDRAW, WndProc, 0, 0,
                       GetModuleHandle(NULL), NULL, NULL, NULL, NULL, _T("WebView2Sample"), NULL};
    RegisterClassEx(&wcex);

    // Create the window
    g_hWnd = CreateWindow(wcex.lpszClassName, _T("WebView2 Sample"), WS_OVERLAPPEDWINDOW, CW_USEDEFAULT,
                          CW_USEDEFAULT, CW_USEDEFAULT, CW_USEDEFAULT, NULL, NULL, wcex.hInstance, NULL);

    // Create WebView2 environment
    CreateWebView2Environment(NULL, NULL, NULL, WebView2CreateEnvironmentCompleted, g_hWnd);

    // Show the window
    ShowWindow(g_hWnd, SW_SHOWNORMAL);
    UpdateWindow(g_hWnd);

    // Enter the message loop
    MSG msg = {0};
    while (GetMessage(&msg, NULL, 0, 0)) {
        TranslateMessage(&msg);
        DispatchMessage(&msg);
    }

    // Clean up
    CoUninitialize();
}

void CALLBACK WebView2CreateEnvironmentCompleted(HRESULT result, IWebView2Environment* environment, void* userData) {
    if (result == S_OK) {
        IWebView2CreateWebView2EnvironmentCompletedHandler* handler;
        IWebView2WebView* webView;
        environment->CreateWebView(g_hWnd, nullptr, handler, &webView);
        g_webView = webView;
        IWebView2Settings* settings;
        webView->get_Settings(&settings);
        settings->put_IsScriptEnabled(TRUE);
        settings->put_AreDefaultScriptDialogsEnabled(TRUE);
        settings->put_IsWebMessageEnabled(TRUE);
        webView->Navigate(L"https://example.com"); // 替换为你要加载的 URL
    }
}
