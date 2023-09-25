package webview

import (
	_ "wox/webview/macos"
)

type Webview interface {
	CreateWebview(url string)
}
