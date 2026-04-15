package main

import (
	"wox/app"
	"wox/util/mainthread"

	_ "wox/plugin/host"
	_ "wox/plugin/system"
	_ "wox/plugin/system/app"
	_ "wox/plugin/system/browser_bookmark"
	_ "wox/plugin/system/calculator"
	_ "wox/plugin/system/clipboard"
	_ "wox/plugin/system/converter"
	_ "wox/plugin/system/emoji"
	_ "wox/plugin/system/explorer"
	_ "wox/plugin/system/file_search"
	_ "wox/plugin/system/mediaplayer"
	_ "wox/plugin/system/shell"
)

func main() {
	mainthread.Init(app.Run)
}
