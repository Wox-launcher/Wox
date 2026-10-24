package app

import (
	"context"
	"reflect"
	"testing"
	"wox/plugin"
	"wox/util"
)

func TestNewWindowPolicies(t *testing.T) {
	for _, tc := range []struct {
		executable string
		args       []string
	}{
		{"firefox.exe", []string{"--new-window", "about:blank"}},
		{"Chrome.EXE", []string{"--new-window"}},
		{"msedge.exe", []string{"--new-window"}},
		{"brave-browser", []string{"--new-window"}},
		{"code - insiders.exe", []string{"--new-window"}},
		{"wt.exe", []string{"-w", "new"}},
		{"unknown.exe", nil},
		{"My Firefox Shortcut", nil},
		{"env", nil},
	} {
		t.Run(tc.executable, func(t *testing.T) {
			if got := appNewWindowArguments(tc.executable); !reflect.DeepEqual(got, tc.args) {
				t.Fatalf("arguments = %v, want %v", got, tc.args)
			}
		})
	}
}

func TestNewWindowRejectsConflictingLaunchModes(t *testing.T) {
	for _, args := range [][]string{
		{"--app=https://example.com"}, {"--app-id=example"}, {"-osint", "-url", "https://example.com"},
		{"-no-remote"}, {"--new-instance"}, {"-private-window"}, {"--reuse-window"}, {"-w", "0"}, {"--", "file"},
	} {
		if supportsNewWindowArguments(args) {
			t.Errorf("conflicting arguments accepted: %v", args)
		}
	}
	if !supportsNewWindowArguments([]string{"--profile-directory=Profile 1", "--user-data-dir=C:/用户 data", "-P", "app"}) {
		t.Fatal("profile arguments must remain supported")
	}
}

func TestNewWindowActionPreservesDefaultOpen(t *testing.T) {
	for _, info := range []appInfo{
		{Name: "Firefox", Path: "firefox.exe", Type: AppTypeDesktop},
		{Name: "Unknown", Path: "unknown.exe", Type: AppTypeDesktop},
		{Name: "Shortcut", Path: "missing.lnk", Type: AppTypeDesktop},
		{Name: "Packaged app", Path: "shell:AppsFolder\\Example!App", Type: AppTypeUWP},
		{Name: "Setting", Path: "ms-settings:display", Type: AppTypeWindowsSetting},
	} {
		actions := (&ApplicationPlugin{}).buildAppActions(info, "Example", nil)
		if actions[0].Name != "i18n:plugin_app_open" {
			t.Fatal("new window replaced the default action")
		}
		count := 0
		for _, action := range actions {
			if action.Name == "i18n:plugin_app_open_new_window" {
				count++
				if action.Hotkey != util.PrimaryHotkey("enter") {
					t.Fatalf("new window shortcut = %q", action.Hotkey)
				}
			}
		}
		if count != 1 {
			t.Fatalf("app=%s, new window actions=%d", info.Name, count)
		}
	}
}

type newWindowNotificationAPI struct {
	emptyAPIImpl
	message string
}

func (api *newWindowNotificationAPI) GetTranslation(ctx context.Context, key string) string {
	return "translated:" + key
}

func (api *newWindowNotificationAPI) Notify(ctx context.Context, message string) {
	api.message = message
}

func TestUnsupportedNewWindowUsesDefaultOpen(t *testing.T) {
	api := &newWindowNotificationAPI{}
	app := &ApplicationPlugin{api: api}
	calls := 0
	ctx := context.Background()
	openAction := plugin.QueryResultAction{Action: func(gotCtx context.Context, actionContext plugin.ActionContext) {
		calls++
		if gotCtx != ctx {
			t.Fatal("fallback lost action context")
		}
	}}
	action := app.buildNewWindowAction(appInfo{Path: "unsupported", Type: AppTypeWindowsSetting}, "Unsupported", nil, openAction)
	action.Action(ctx, plugin.ActionContext{})
	if calls != 1 || api.message != "" {
		t.Fatalf("fallback calls=%d notification=%q", calls, api.message)
	}
}
