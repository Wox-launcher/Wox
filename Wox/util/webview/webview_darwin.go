package webview

import "wox/util/webview/macos"

func NewWebview() Webview {
	return &macos.WebViewMacOs{}
}
