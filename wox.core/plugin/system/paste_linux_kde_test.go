//go:build linux

package system

import (
	"context"
	"errors"
	"slices"
	"testing"
	"wox/plugin"
	"wox/plugin/plugintest"
)

func TestKDEPasteUsesKonsoleShortcut(t *testing.T) {
	if !kdePasteUsesShift("org.kde.konsole.desktop") || !kdePasteUsesShift("Konsole") {
		t.Fatal("Konsole must paste with Ctrl+Shift+V")
	}
	if kdePasteUsesShift("google-chrome") || kdePasteUsesShift("") {
		t.Fatal("standard paste must use Ctrl+V")
	}
}

type kdePasteAPI struct {
	plugintest.StubAPI
	steps []string
}

func (a *kdePasteAPI) HideApp(context.Context) {
	a.steps = append(a.steps, "hide")
}

func TestKDEPastePreparesClipboardBeforeHidingLauncher(t *testing.T) {
	t.Setenv("XDG_CURRENT_DESKTOP", "KDE")
	t.Setenv("XDG_SESSION_TYPE", "wayland")
	api := &kdePasteAPI{}
	ctx := context.Background()
	action, err := GetPasteToActiveWindowAction(ctx, api, plugin.QueryEnv{ActiveWindowTitle: "Konsole"}, func(context.Context) error {
		api.steps = append(api.steps, "prepare")
		return errors.New("clipboard write failed")
	})
	if err != nil {
		t.Fatal(err)
	}
	if !action.PreventHideAfterAction {
		t.Fatal("launcher must leave the hide to the paste action")
	}
	action.Action(ctx, plugin.ActionContext{})
	if !slices.Equal(api.steps, []string{"prepare", "hide"}) {
		t.Fatalf("action order = %v", api.steps)
	}
}
