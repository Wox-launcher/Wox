package system

import (
	"context"

	"wox/common"
	"wox/plugin"
)

// folderNavigationTestAPI records navigation without depending on another plugin's tests.
type folderNavigationTestAPI struct {
	plugin.API
	changed common.PlainQuery
}

func (a *folderNavigationTestAPI) ChangeQuery(_ context.Context, query common.PlainQuery) {
	a.changed = query
}

func (*folderNavigationTestAPI) GetSetting(context.Context, string) string { return "" }
func (*folderNavigationTestAPI) ShowApp(context.Context)                   {}
