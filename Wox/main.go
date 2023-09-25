package main

import "wox/webview"

func main() {
	wv := webview.NewWebview()
	wv.CreateWebview("https://www.sina.com.cn")
}
