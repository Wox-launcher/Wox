package webview

import "example.com/app/webview/windows"

func NewWebview() Webview {
	return &windows.WebViewWindows{}
}
