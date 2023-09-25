package webview

import "wox/webview/windows"

func NewWebview() Webview {
	return &windows.WebViewWindows{}
}
