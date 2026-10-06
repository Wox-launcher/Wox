//go:build windows

package launcher

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"wox/common"
	"wox/i18n"
	"wox/ui/contract"
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// runNativeShortcutFlow creates windows on the UI thread and runs assertions while its event loop is active.
func runNativeShortcutFlow(t *testing.T, services contract.Services, prepare func(*App) (func() error, error)) {
	t.Helper()
	result := make(chan error, 1)
	err := woxui.Run(func() error {
		manager := woxui.NewWindowManager()
		app := newApp(false, services, manager, nil, nil, true, "primary", launcherWindowID)
		flow, err := prepare(app)
		if err != nil {
			app.cancel()
			_ = manager.CloseAll()
			return err
		}
		go func() {
			flowErr := flow()
			closeErr := manager.CloseAll()
			app.cancel()
			result <- errors.Join(flowErr, closeErr)
		}()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("native shortcut flow did not finish")
	}
}

// TestSettingsShortcutKeepsExistingDraft verifies secondary-window routing and focused editor precedence.
func TestSettingsShortcutKeepsExistingDraft(t *testing.T) {
	runNativeShortcutFlow(t, nil, func(app *App) (func() error, error) {
		draft := woxwidget.NewTextEditingController("unsaved settings draft")
		app.settingTab = "plugins"
		app.settingsSearch.SetEditor(draft)
		app.settingsTableEditor = &formTableEditorState{}
		tableDraft := app.settingsTableEditor
		consumed := 0
		host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
			return woxcomponent.WoxTextField(woxcomponent.TextFieldProps{ID: "settings.draft", Width: 200, Height: 40, Controller: draft, OnKey: func(woxui.KeyEvent) bool { consumed++; return true }})
		})
		secondary := &App{primary: app}
		managed, _, err := app.windows.Open(settingsWindowID, woxui.WindowOptions{Title: "Wox settings shortcut test", OnFrame: host.Frame, OnKey: func(event woxui.KeyEvent) bool {
			return secondary.dispatchWindowKey(event, host, "", func() {}, func(woxui.KeyEvent) bool { return false })
		}})
		if err != nil {
			return nil, err
		}
		host.Attach(managed.Window())
		host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: 200, Height: 80}, Scale: 1})
		if !host.RequestFocus("settings.draft") {
			return nil, errors.New("settings draft did not receive focus")
		}
		app.settingsView = managed
		visible := make(chan struct{}, 2)
		app.windows.SubscribeLifecycle(func(event woxui.WindowLifecycleEvent) {
			if event.ID == settingsWindowID && event.Current == woxui.WindowLifecycleVisible {
				visible <- struct{}{}
			}
		})
		return func() error {
			if handled, err := managed.Window().DispatchKey(woxui.KeyEvent{Key: ",", Modifiers: woxui.KeyModifierControl, Down: true}); err != nil || !handled {
				return fmt.Errorf("settings shortcut: handled=%t err=%v", handled, err)
			}
			select {
			case <-visible:
			case <-time.After(5 * time.Second):
				return errors.New("settings shortcut did not focus existing window")
			}
			if _, err := managed.Window().DispatchKey(woxui.KeyEvent{Key: ",", Modifiers: woxui.KeyModifierControl, Down: true, Repeat: true}); err != nil {
				return err
			}
			var stateErr error
			if err := woxui.Call(func() {
				if app.settingTab != "plugins" || app.settingsSearch.Editor() != draft || draft.Text() != "unsaved settings draft" || app.settingsTableEditor != tableDraft || consumed != 0 {
					stateErr = errors.New("settings shortcut reset a draft or reached the focused editor")
				}
			}); err != nil {
				return err
			}
			select {
			case <-visible:
				return errors.New("repeated settings shortcut presented the window again")
			default:
			}
			return stateErr
		}, nil
	})
}

type shortcutNotesServices struct {
	*notesWindowTestServices
	saveAttempt chan struct{}
}

type shortcutWindowServices struct {
	contract.Services
	hidden chan struct{}
}

func (s *shortcutWindowServices) Shown(context.Context, string) error { return nil }

func (s *shortcutWindowServices) SettingViewChanged(context.Context, string, bool) error {
	return nil
}

func (s *shortcutWindowServices) HideTooltip(context.Context, string, string) error { return nil }

func (s *shortcutWindowServices) Hidden(context.Context, string) error {
	if s.hidden != nil {
		s.hidden <- struct{}{}
	}
	return nil
}

type shortcutPluginSettingsServices struct {
	*shortcutWindowServices
}

func (s *shortcutPluginSettingsServices) GeneralSettings(context.Context, string) (contract.GeneralSettings, error) {
	return contract.GeneralSettings{LangCode: i18n.LangCodeEnUs, UIDensity: "normal"}, nil
}

func (s *shortcutPluginSettingsServices) AvailableLanguages(context.Context, string) ([]i18n.Lang, error) {
	return nil, nil
}

func (s *shortcutPluginSettingsServices) LanguageBundle(context.Context, string, i18n.LangCode) (map[string]string, error) {
	return map[string]string{}, nil
}

func (s *shortcutPluginSettingsServices) UpdateChannelVersions(context.Context, string) ([]contract.UpdateChannelVersion, error) {
	return nil, nil
}

func (s *shortcutPluginSettingsServices) Plugins(context.Context, string, contract.PluginCatalog) ([]contract.PluginCatalogItem, error) {
	return []contract.PluginCatalogItem{
		{ID: common.NotesPluginID, Name: "Notes", IsInstalled: true, IsSystem: true},
		{ID: common.AIChatPluginID, Name: "AI Chat", IsInstalled: true, IsSystem: true},
	}, nil
}

// TestPluginWindowSettingsShortcutRoutesAndKeepsDraft covers a new open, cross-window routing, and a repeated open of the same plugin.
func TestPluginWindowSettingsShortcutRoutesAndKeepsDraft(t *testing.T) {
	services := &shortcutPluginSettingsServices{shortcutWindowServices: &shortcutWindowServices{Services: &notesWindowTestServices{}}}
	runNativeShortcutFlow(t, services, func(app *App) (func() error, error) {
		note := newNotesWindowController(app, common.NoteRecord{ID: "note", Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{{ID: "p", Type: common.NoteBlockParagraph, Text: "note draft"}}}})
		app.noteWindows["note"] = note
		noteWindow, err := note.ensure()
		if err != nil {
			return nil, err
		}
		chatWindow, err := app.ensureChatWindow()
		if err != nil {
			return nil, err
		}
		visible := make(chan struct{}, 3)
		app.windows.SubscribeLifecycle(func(event woxui.WindowLifecycleEvent) {
			if event.ID == settingsWindowID && event.Current == woxui.WindowLifecycleVisible {
				visible <- struct{}{}
			}
		})
		return func() error {
			var settings *woxui.ManagedWindow
			var draft *pluginSettingsFormState
			for index, source := range []*woxui.ManagedWindow{noteWindow, chatWindow, chatWindow} {
				if handled, err := source.Window().DispatchKey(woxui.KeyEvent{Key: ",", Modifiers: woxui.KeyModifierControl, Down: true}); err != nil || !handled {
					return fmt.Errorf("plugin settings shortcut %d: handled=%t err=%v", index, handled, err)
				}
				select {
				case <-visible:
				case <-time.After(5 * time.Second):
					return fmt.Errorf("plugin settings shortcut %d did not show Settings", index)
				}
				deadline := time.Now().Add(5 * time.Second)
				for app.openingSettingsShortcut.Load() && time.Now().Before(deadline) {
					time.Sleep(time.Millisecond)
				}
				want := common.AIChatPluginID
				if index == 0 {
					want = common.NotesPluginID
				}
				var stateErr error
				if err := woxui.Call(func() {
					form := app.pluginSettings.Form()
					if app.settingTab != "plugins" || app.pluginSettings.PluginsStore() || app.pluginSettings.DetailTab() != "settings" || form == nil || form.pluginID != want {
						stateErr = fmt.Errorf("shortcut %d did not navigate to plugin %s", index, want)
						return
					}
					if index == 0 {
						settings = app.settingsView
					} else if app.settingsView != settings {
						stateErr = errors.New("plugin navigation replaced the Settings window")
					}
					if index == 1 {
						draft = form
						form.values["TriggerKeywords"] = `[{"keyword":"unsaved-chat"}]`
						app.pluginSettings.SetDetailTab("description")
					} else if index == 2 && (form != draft || form.values["TriggerKeywords"] != `[{"keyword":"unsaved-chat"}]`) {
						stateErr = errors.New("reopening the same plugin reset its draft")
					}
				}); err != nil || stateErr != nil {
					return errors.Join(err, stateErr)
				}
			}
			if noteWindow.Lifecycle() == woxui.WindowLifecycleClosed || chatWindow.Lifecycle() == woxui.WindowLifecycleClosed {
				return errors.New("opening plugin settings closed the originating utility window")
			}
			return nil
		}, nil
	})
}

// TestWindowShortcutRecordingPrecedesCommands verifies both reserved combinations remain recordable.
func TestWindowShortcutRecordingPrecedesCommands(t *testing.T) {
	recording := &localHotkeyTestServices{candidates: make(chan string, 2)}
	services := &shortcutWindowServices{Services: recording}
	runNativeShortcutFlow(t, services, func(app *App) (func() error, error) {
		form := newHotkeySettingsForm(settingsData{})
		app.hotkeySettings.SetForm(&form)
		app.hotkeySettings.SetRecording(&hotkeyRecordingState{target: &form, ready: true, raw: true, fallback: true, diagnosticCtx: context.Background()})
		managed, err := app.ensureSettingsWindow()
		if err != nil {
			return nil, err
		}
		return func() error {
			defer woxui.Call(func() { app.hotkeySettings.ClearRecording() })
			for _, key := range []woxui.Key{",", "w"} {
				if handled, err := managed.Window().DispatchKey(woxui.KeyEvent{Key: key, Modifiers: woxui.KeyModifierControl, Down: true}); err != nil || !handled {
					return fmt.Errorf("record shortcut %q: handled=%t err=%v", key, handled, err)
				}
				select {
				case candidate := <-recording.candidates:
					if candidate != "ctrl+"+string(key) {
						return fmt.Errorf("recorded %q instead of ctrl+%s", candidate, key)
					}
				case <-time.After(5 * time.Second):
					return fmt.Errorf("reserved shortcut %q did not reach recording", key)
				}
			}
			if managed.Lifecycle() != woxui.WindowLifecycleCreated {
				return errors.New("recorded shortcut opened or closed Settings")
			}
			return nil
		}, nil
	})
}

// TestLauncherCloseShortcutHidesAndReopens exercises the existing launcher lifecycle.
func TestLauncherCloseShortcutHidesAndReopens(t *testing.T) {
	services := &shortcutWindowServices{hidden: make(chan struct{}, 1)}
	runNativeShortcutFlow(t, services, func(app *App) (func() error, error) {
		managed, _, err := app.windows.Open(launcherWindowID, woxui.WindowOptions{Title: "Wox launcher shortcut test", OnKey: func(event woxui.KeyEvent) bool {
			return app.dispatchWindowKey(event, nil, "", app.requestLauncherShortcutClose, app.onKey)
		}})
		if err != nil {
			return nil, err
		}
		app.launcher, app.window = managed, managed.Window()
		return func() error {
			params := showAppParams{WindowWidth: 500, MaxResultCount: 8, StartPage: "none", LaunchMode: "continue"}
			if err := app.showWindow(params); err != nil {
				return err
			}
			if handled, err := managed.Window().DispatchKey(woxui.KeyEvent{Key: "w", Modifiers: woxui.KeyModifierControl, Down: true}); err != nil || !handled {
				return fmt.Errorf("hide launcher: handled=%t err=%v", handled, err)
			}
			select {
			case <-services.hidden:
			case <-time.After(5 * time.Second):
				return errors.New("launcher shortcut did not hide the window")
			}
			if managed.Lifecycle() != woxui.WindowLifecycleHidden {
				return errors.New("launcher shortcut destroyed the native window")
			}
			if err := app.showWindow(params); err != nil {
				return err
			}
			if managed.Lifecycle() != woxui.WindowLifecycleVisible {
				return errors.New("hidden launcher could not be shown again")
			}
			return nil
		}, nil
	})
}

// TestChatCloseShortcutClosesOnlyDedicatedWindow keeps the owning launcher alive.
func TestChatCloseShortcutClosesOnlyDedicatedWindow(t *testing.T) {
	runNativeShortcutFlow(t, nil, func(app *App) (func() error, error) {
		launcher, _, err := app.windows.Open(launcherWindowID, woxui.WindowOptions{Title: "Wox chat owner shortcut test"})
		if err != nil {
			return nil, err
		}
		app.launcher, app.window = launcher, launcher.Window()
		chat, err := app.ensureChatWindow()
		if err != nil {
			return nil, err
		}
		closed := make(chan struct{}, 1)
		app.windows.SubscribeLifecycle(func(event woxui.WindowLifecycleEvent) {
			if event.ID == chatWindowID && event.Current == woxui.WindowLifecycleClosed {
				closed <- struct{}{}
			}
		})
		return func() error {
			if handled, err := chat.Window().DispatchKey(woxui.KeyEvent{Key: "w", Modifiers: woxui.KeyModifierControl, Down: true}); err != nil || !handled {
				return fmt.Errorf("close Chat: handled=%t err=%v", handled, err)
			}
			select {
			case <-closed:
			case <-time.After(5 * time.Second):
				return errors.New("dedicated Chat shortcut did not close the window")
			}
			if launcher.Lifecycle() == woxui.WindowLifecycleClosed {
				return errors.New("closing Chat also destroyed the launcher")
			}
			return nil
		}, nil
	})
}

// NotesSave signals failed attempts as well as successful writes for close-flow assertions.
func (s *shortcutNotesServices) NotesSave(ctx context.Context, id, revision string, document common.NoteDocument) (common.NoteSaveResult, error) {
	result, err := s.notesWindowTestServices.NotesSave(ctx, id, revision, document)
	s.saveAttempt <- struct{}{}
	return result, err
}

// TestNoteCloseShortcutPreservesFailedSaveAndOtherWindows covers the real controller and native close path.
func TestNoteCloseShortcutPreservesFailedSaveAndOtherWindows(t *testing.T) {
	services := &shortcutNotesServices{notesWindowTestServices: &notesWindowTestServices{saveErr: errors.New("disk full")}, saveAttempt: make(chan struct{}, 2)}
	runNativeShortcutFlow(t, services, func(app *App) (func() error, error) {
		first := newNotesWindowController(app, common.NoteRecord{ID: "first", Revision: "initial", Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{{ID: "p", Type: common.NoteBlockParagraph, Text: "unsaved note"}}}})
		second := newNotesWindowController(app, common.NoteRecord{ID: "second", Document: common.NoteDocument{Version: 1, Blocks: []common.NoteBlock{{ID: "p", Type: common.NoteBlockParagraph, Text: "other note"}}}})
		app.noteWindows["first"], app.noteWindows["second"] = first, second
		first.dirty = true
		firstWindow, err := first.ensure()
		if err != nil {
			return nil, err
		}
		secondWindow, err := second.ensure()
		if err != nil {
			return nil, err
		}
		closed := make(chan struct{}, 1)
		app.windows.SubscribeLifecycle(func(event woxui.WindowLifecycleEvent) {
			if event.ID == first.windowID && event.Current == woxui.WindowLifecycleClosed {
				closed <- struct{}{}
			}
		})
		return func() error {
			pressClose := func() error {
				handled, err := firstWindow.Window().DispatchKey(woxui.KeyEvent{Key: "w", Modifiers: woxui.KeyModifierControl, Down: true})
				if err == nil && !handled {
					err = errors.New("Note did not handle close shortcut")
				}
				return err
			}
			if err := pressClose(); err != nil {
				return err
			}
			select {
			case <-services.saveAttempt:
			case <-time.After(5 * time.Second):
				return errors.New("Note close did not attempt to save")
			}
			var stateErr error
			if err := woxui.Call(func() {
				if !first.dirty || first.errorText == "" || firstWindow.Lifecycle() == woxui.WindowLifecycleClosed || secondWindow.Lifecycle() == woxui.WindowLifecycleClosed {
					stateErr = errors.New("failed save lost the note or closed another window")
				}
			}); err != nil || stateErr != nil {
				return errors.Join(err, stateErr)
			}
			services.mu.Lock()
			services.saveErr = nil
			services.mu.Unlock()
			if err := pressClose(); err != nil {
				return err
			}
			select {
			case <-closed:
			case <-time.After(5 * time.Second):
				return errors.New("Note remained open after save succeeded")
			}
			if secondWindow.Lifecycle() == woxui.WindowLifecycleClosed || services.record.Document.Blocks[0].Text != "unsaved note" {
				return errors.New("close lost saved content or affected the other Note")
			}
			return nil
		}, nil
	})
}
