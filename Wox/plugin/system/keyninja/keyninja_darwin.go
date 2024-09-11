package keyninja

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework ApplicationServices
#import <Foundation/Foundation.h>

NSArray* getVisibleUIElements();
void assignShortcuts(NSArray *elements);
void showShortcuts(NSArray *elements);
*/
import "C"

import (
	"context"
	"wox/plugin"
	"wox/util"

	"golang.design/x/hotkey/mainthread"
)

var keyninjaIcon = plugin.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" width="48" height="48" viewBox="0 0 24 24"><path fill="#06ac11" d="M4 19q-.825 0-1.412-.587T2 17V7q0-.825.588-1.412T4 5h16q.825 0 1.413.588T22 7v10q0 .825-.587 1.413T20 19zm4-3h8v-2H8zm-3-3h2v-2H5zm3 0h2v-2H8zm3 0h2v-2h-2zm3 0h2V8h-2zm3 0h2V8h-2zm3 0h2V8h-2z"/></svg>`)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &KeyNinjaPlugin{})
}

type KeyNinjaPlugin struct {
	api plugin.API
}

func (k *KeyNinjaPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "c392f4bd-ad10-48a4-981d-d53bc1603404",
		Name:          "Keyboard Ninja",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "Control Macos GUI elements with keyboard shortcuts, like shortcat or homerow",
		Icon:          keyninjaIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"kc",
		},
		SupportedOS: []string{
			"Macos",
		},
	}
}

func (k *KeyNinjaPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	k.api = initParams.API
}

func (k *KeyNinjaPlugin) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	elements := C.getVisibleUIElements()
	C.assignShortcuts(elements)
	mainthread.Call(func() {
		C.showShortcuts(elements)

	})
	return []plugin.QueryResult{}
}

//export logMessage
func logMessage(cMessage *C.char) {
	message := C.GoString(cMessage)
	util.GetLogger().Info(util.NewTraceContext(), message)
}
