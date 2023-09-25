package webview

import "wox/webview/macos"

func NewWebview() Webview {
	return &macos.WebViewMacOs{}
}
