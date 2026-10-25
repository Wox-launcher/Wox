package launcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"wox/common"
	corehotkey "wox/hotkey"
	"wox/plugin/thirdparty/migrate"
	"wox/setting"
	"wox/ui/contract"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
	utilhotkey "wox/util/hotkey"
	"wox/util/keyboard"
)

// onboardingMigrationState is the detected launchers and the import shown on the migration step.
type onboardingMigrationState struct {
	installations          []migrate.Installation
	selectedID             string
	choosing               bool
	categories             []onboardingMigrationCategory
	loading                bool
	loaded                 bool
	importing              bool
	activeID               string
	done                   bool
	error                  string
	stagedMainHotkey       string
	stagedMainHotkeyError  string
	mainHotkeyPending      bool
	mainHotkeyApplyStarted bool
	// themeImported means a launcher theme was installed and selected, so the later theme page would replace it.
	themeImported bool
	exportPath    string
	unlocked      bool
	passphrase    *woxwidget.TextEditingController
	sessionCancel context.CancelFunc
}

type onboardingMigrationCategory struct {
	id    string
	items []onboardingMigrationItem
}

type onboardingMigrationItem struct {
	item     migrate.Item
	selected bool
	copied   bool
	err      string
}

// newOnboardingMigrationState opens the plugin list when only one launcher is installed.
func newOnboardingMigrationState(installations []migrate.Installation) onboardingMigrationState {
	state := onboardingMigrationState{installations: installations}
	if len(installations) == 1 {
		state.selectedID = installations[0].ID()
		return state
	}
	if len(installations) > 1 {
		state.choosing = true
	}
	return state
}

// detectOnboardingMigrations asks every registered launcher source what is installed.
func detectOnboardingMigrations(ctx context.Context) []migrate.Installation {
	found, err := migrate.Detect(ctx)
	if err != nil {
		util.GetLogger().Warn(ctx, "detect launcher migrations: "+err.Error())
	}
	return found
}

func (a *App) prepareOnboardingMigration() {
	state := &a.onboarding.onboardingMigration
	if state.importing || state.done {
		return
	}
	if len(state.installations) == 1 && state.selectedID == "" {
		state.selectedID = state.installations[0].ID()
		state.choosing = false
	}
	if len(state.installations) > 1 && state.selectedID == "" {
		state.choosing = true
	}
	if state.selectedID != "" && !state.loaded && !state.loading && !a.migrationNeedsUnlock() {
		a.loadOnboardingMigrationCatalog()
	}
	if a.migrationNeedsUnlock() {
		a.prefillOnboardingMigrationFile()
	}
}

func (a *App) loadOnboardingMigrationCatalog() {
	installation := a.selectedMigrationInstallation()
	if installation == nil {
		return
	}
	a.onboarding.onboardingMigration.loading = true
	a.onboarding.onboardingMigration.error = ""
	a.onboarding.invalidateOnboardingWindow()
	var generation uint64
	if export, ok := installation.(migrate.ExportSource); ok {
		generation = export.Generation()
	}
	util.Go(a.lifecycleCtx, "load onboarding migration catalog", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		catalog, err := installation.Catalog(ctx)
		cancel()
		_ = a.runOnUI("apply onboarding migration catalog", func() {
			if a.selectedMigrationInstallation() == nil || a.selectedMigrationInstallation().ID() != installation.ID() {
				return
			}
			if export, ok := installation.(migrate.ExportSource); ok && export.Generation() != generation {
				return
			}
			state := &a.onboarding.onboardingMigration
			state.loading = false
			state.loaded = err == nil
			if err != nil {
				state.error = err.Error()
				state.categories = nil
			} else {
				state.error = ""
				state.categories = onboardingMigrationCategories(installation, catalog)
			}
			a.onboarding.invalidateOnboardingWindow()
		})
	})
}

func onboardingMigrationCategories(installation migrate.Installation, catalog []migrate.Category) []onboardingMigrationCategory {
	if _, ok := installation.(migrate.RunningLauncher); ok {
		catalog = append([]migrate.Category{migrate.CloseLauncherCategory(installation.Name())}, catalog...)
	}
	categories := make([]onboardingMigrationCategory, 0, len(catalog))
	for _, category := range catalog {
		if len(category.Items) == 0 {
			continue
		}
		items := make([]onboardingMigrationItem, len(category.Items))
		for index, item := range category.Items {
			items[index] = onboardingMigrationItem{item: item, selected: item.Selectable || item.Required}
		}
		categories = append(categories, onboardingMigrationCategory{id: category.ID, items: items})
	}
	return categories
}

// migrationItemChosen reports a row the import will apply. A required row stays included.
func migrationItemChosen(item onboardingMigrationItem) bool {
	return item.selected && !item.copied && (item.item.Selectable || item.item.Required)
}

func (a *App) chooseOnboardingMigration(id string) {
	if a.onboarding.onboardingMigration.importing {
		return
	}
	if a.migrationInstallation(id) == nil {
		return
	}
	a.releaseOnboardingMigrationSession()
	a.onboarding.onboardingMigration.selectedID = id
	a.onboarding.onboardingMigration.choosing = false
	a.onboarding.onboardingMigration.categories = nil
	a.onboarding.onboardingMigration.loaded = false
	a.onboarding.onboardingMigration.done = false
	a.onboarding.onboardingMigration.error = ""
	if !a.migrationNeedsUnlock() {
		a.loadOnboardingMigrationCatalog()
	} else {
		a.prefillOnboardingMigrationFile()
	}
	a.onboarding.invalidateOnboardingWindow()
}

func (a *App) toggleOnboardingMigrationItem(id string, selected bool) {
	if a.onboarding.onboardingMigration.importing || a.onboarding.onboardingMigration.done {
		return
	}
	for categoryIndex := range a.onboarding.onboardingMigration.categories {
		items := a.onboarding.onboardingMigration.categories[categoryIndex].items
		for index := range items {
			item := &items[index]
			if item.item.ID == id && item.item.Selectable && !item.item.Required {
				item.selected = selected
			}
		}
	}
	a.onboarding.invalidateOnboardingWindow()
}

// retreatOnboardingMigration returns to the launcher picker when several are installed.
func (a *App) retreatOnboardingMigration() bool {
	state := &a.onboarding.onboardingMigration
	if state.importing || state.done || state.choosing || len(state.installations) < 2 || state.selectedID == "" {
		return false
	}
	a.releaseOnboardingMigrationSession()
	state.choosing = true
	state.selectedID = ""
	state.categories = nil
	state.loaded = false
	state.loading = false
	state.error = ""
	a.onboarding.invalidateOnboardingWindow()
	return true
}

func (a *App) onboardingMigrationCanImport() bool {
	state := &a.onboarding.onboardingMigration
	if state.done || state.choosing || state.importing || state.loading {
		return false
	}
	for _, category := range state.categories {
		for _, item := range category.items {
			if migrationItemChosen(item) {
				return true
			}
		}
	}
	return false
}

func (a *App) importOnboardingMigration() {
	if !a.onboardingMigrationCanImport() {
		return
	}
	installation := a.selectedMigrationInstallation()
	if installation == nil {
		return
	}
	var itemIDs []string
	for _, category := range a.onboarding.onboardingMigration.categories {
		for _, item := range category.items {
			if migrationItemChosen(item) {
				itemIDs = append(itemIDs, item.item.ID)
			}
		}
	}
	a.onboarding.onboardingMigration.importing = true
	a.onboarding.onboardingMigration.activeID = itemIDs[0]
	a.onboarding.onboardingMigration.error = ""
	session, sessionCancel := context.WithCancel(context.Background())
	a.onboarding.onboardingMigration.sessionCancel = sessionCancel
	kinds := map[string]string{}
	for _, id := range itemIDs {
		kinds[id] = a.migrationItemKind(id)
	}
	a.onboarding.invalidateOnboardingWindow()
	util.Go(a.lifecycleCtx, "import onboarding migration", func() {
		defer sessionCancel()
		dataCtx, dataCancel := context.WithTimeout(session, 2*time.Minute)
		defer dataCancel()
		groups, failures := a.prepareOnboardingMigrationSettings(dataCtx, installation, itemIDs)
		// A launcher that is still running keeps the global shortcuts, so the later rows are skipped.
		closed := true
		for _, id := range itemIDs {
			if session.Err() != nil || !closed {
				a.finishOnboardingMigrationItem(id, "", "cancelled")
				continue
			}
			a.showOnboardingMigrationItem(id)
			started := time.Now()
			itemErr, status := a.importVisibleOnboardingMigrationItem(dataCtx, installation, id, kinds[id], groups, failures)
			waitOnboardingMigrationDisplay(session, started)
			a.finishOnboardingMigrationItem(id, itemErr, status)
			if kinds[id] == migrate.ItemKindCloseLauncher && itemErr != "" {
				closed = false
			}
		}
		_ = a.runOnUI("finish onboarding migration", func() {
			state := &a.onboarding.onboardingMigration
			state.importing = false
			state.activeID = ""
			state.done = true
			state.sessionCancel = nil
			a.onboarding.invalidateOnboardingWindow()
		})
	})
}

func waitOnboardingMigrationDisplay(ctx context.Context, started time.Time) {
	remain := onboardingMigrationStepDelay - time.Since(started)
	if remain <= 0 {
		return
	}
	timer := time.NewTimer(remain)
	defer timer.Stop()
	select {
	case <-ctx.Done():
	case <-timer.C:
	}
}

// Each row keeps its status icon up long enough to read before the next row starts.
const onboardingMigrationStepDelay = 400 * time.Millisecond

const migrationHotkeyTaken = "hotkey_taken"

type migrationWriteGroup struct {
	writes  []migrate.SettingWrite
	themes  []migrate.ThemeWrite
	applied bool
	err     string
}

func (a *App) prepareOnboardingMigrationSettings(ctx context.Context, installation migrate.Installation, itemIDs []string) (map[string]*migrationWriteGroup, map[string]string) {
	var settingIDs []string
	for _, id := range itemIDs {
		if strings.HasPrefix(id, "setting:") {
			settingIDs = append(settingIDs, id)
		}
	}
	groups := map[string]*migrationWriteGroup{}
	failures := map[string]string{}
	if len(settingIDs) == 0 {
		return groups, failures
	}
	result, err := installation.Import(ctx, settingIDs)
	if err != nil {
		for _, id := range settingIDs {
			failures[id] = err.Error()
		}
		return groups, failures
	}
	for _, write := range result.Settings {
		attachMigrationWrite(groups, write.ItemIDs, &write, nil)
	}
	for _, theme := range result.Themes {
		attachMigrationWrite(groups, theme.ItemIDs, nil, &theme)
	}
	for _, item := range result.Plugins {
		if item.Error != "" {
			failures[item.ID] = item.Error
		}
	}
	return groups, failures
}

// importVisibleOnboardingMigrationItem writes one catalog row after its status icon is showing.
func (a *App) importVisibleOnboardingMigrationItem(ctx context.Context, installation migrate.Installation, id, kind string, groups map[string]*migrationWriteGroup, failures map[string]string) (string, string) {
	switch kind {
	case migrate.ItemKindPlugin:
		return a.importOnboardingMigrationPlugin(ctx, installation, id), ""
	case migrate.ItemKindCloseLauncher:
		return a.closeOnboardingMigrationLauncher(ctx, installation)
	case migrate.ItemKindQuicklink, migrate.ItemKindFavorite, migrate.ItemKindNote, migrate.ItemKindClipboard, migrate.ItemKindResultBinding:
		outcome, ok := a.applyOnboardingMigrationDomain(ctx, installation, []string{id})[id]
		if !ok {
			return "failed", "failed"
		}
		return migrationOutcome(outcome)
	default:
		itemErr := a.importOnboardingMigrationSetting(ctx, id, groups, failures)
		if itemErr == "" {
			return "", "imported"
		}
		return itemErr, ""
	}
}

func (a *App) importOnboardingMigrationSetting(ctx context.Context, id string, groups map[string]*migrationWriteGroup, failures map[string]string) string {
	if group := groups[id]; group != nil {
		return a.applyMigrationWriteGroup(ctx, group)
	}
	if err := failures[id]; err != "" {
		return err
	}
	return "cannot be imported"
}

func (a *App) applyMigrationWriteGroup(ctx context.Context, group *migrationWriteGroup) string {
	if group.applied {
		return group.err
	}
	group.applied = true
	var errs []string
	for _, write := range group.writes {
		// The other launcher still owns this shortcut, so registering it here fails.
		// Keep the value for the next hotkey page, which registers it and can ask for another.
		if write.Key == "MainHotkey" {
			a.stageMigratedMainHotkey(write.Value)
			continue
		}
		if err := a.applyOnboardingMigrationSetting(ctx, write); err != nil {
			errs = append(errs, migrationApplyError(err))
		}
	}
	for _, theme := range group.themes {
		if err := a.applyMigratedTheme(ctx, theme.Theme); err != nil {
			errs = append(errs, migrationApplyError(err))
		}
	}
	group.err = strings.Join(errs, " ")
	return group.err
}

// attachMigrationWrite keeps every selected item that shares a stored value on one group.
func attachMigrationWrite(groups map[string]*migrationWriteGroup, ids []string, write *migrate.SettingWrite, theme *migrate.ThemeWrite) {
	if len(ids) == 0 {
		return
	}
	var group *migrationWriteGroup
	for _, id := range ids {
		if existing := groups[id]; existing != nil {
			group = existing
			break
		}
	}
	if group == nil {
		group = &migrationWriteGroup{}
	}
	if write != nil {
		group.writes = append(group.writes, *write)
	}
	if theme != nil {
		group.themes = append(group.themes, *theme)
	}
	for _, id := range ids {
		groups[id] = group
	}
}

// applyMigratedTheme installs the mapped theme and makes it the current theme.
func (a *App) applyMigratedTheme(ctx context.Context, theme common.Theme) error {
	if strings.TrimSpace(theme.ThemeId) == "" || a.services == nil {
		return errors.New("theme cannot be imported")
	}
	if err := a.services.InstallMigratedTheme(ctx, a.sessionID, theme); err != nil {
		return err
	}
	a.rememberOnboardingMigrationSetting("ThemeId", theme.ThemeId)
	_ = a.runOnUI("remember imported onboarding theme", func() {
		a.onboarding.onboardingMigration.themeImported = true
	})
	return nil
}

// closeOnboardingMigrationLauncher quits the source launcher before later rows register its hotkeys.
func (a *App) closeOnboardingMigrationLauncher(ctx context.Context, installation migrate.Installation) (string, string) {
	closer, ok := installation.(migrate.RunningLauncher)
	if !ok {
		return "close_launcher", "failed"
	}
	util.GetLogger().Info(ctx, fmt.Sprintf("migration: closing %s so imported hotkeys can be registered", installation.Name()))
	if err := closer.CloseRunning(ctx); err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("migration: close %s: %s", installation.Name(), err.Error()))
		return "close_launcher", "failed"
	}
	return "", "imported"
}

func (a *App) importOnboardingMigrationPlugin(ctx context.Context, installation migrate.Installation, id string) string {
	result, err := installation.Import(ctx, []string{id})
	if err != nil {
		return err.Error()
	}
	for _, item := range result.Plugins {
		if item.ID == id {
			return item.Error
		}
	}
	return "cannot be imported"
}

func (a *App) showOnboardingMigrationItem(id string) {
	_ = a.runOnUI("show onboarding migration item", func() {
		a.onboarding.onboardingMigration.activeID = id
		a.onboarding.invalidateOnboardingWindow()
	})
}

func (a *App) finishOnboardingMigrationItem(id, itemErr, status string) {
	_ = a.runOnUI("finish onboarding migration item", func() {
		if a.onboarding.onboardingMigration.activeID == id {
			a.onboarding.onboardingMigration.activeID = ""
		}
		for categoryIndex := range a.onboarding.onboardingMigration.categories {
			for index := range a.onboarding.onboardingMigration.categories[categoryIndex].items {
				item := &a.onboarding.onboardingMigration.categories[categoryIndex].items[index]
				if item.item.ID != id {
					continue
				}
				item.err = itemErr
				item.copied = itemErr == "" && (status == "" || status == "imported")
				if status == "exists" {
					item.item.Status = migrate.PluginImported
					item.copied = false
				} else if status == "conflict" || status == "cancelled" || status == migrate.PluginUnsupported {
					item.item.Status = status
					item.copied = false
				}
			}
		}
		a.onboarding.invalidateOnboardingWindow()
	})
}

// stageMigratedMainHotkey records the shortcut without registering it with the OS.
func (a *App) stageMigratedMainHotkey(hotkey string) {
	hotkey = strings.TrimSpace(hotkey)
	if hotkey == "" {
		return
	}
	a.onboarding.onboardingMigration.stagedMainHotkey = hotkey
	a.rememberOnboardingMigrationSetting("MainHotkey", hotkey)
}

// applyStagedMigrationHotkey registers the staged main shortcut once the hotkey page is open.
// The imported chord is shown immediately. Registration can take a moment when another
// program still holds it, and the page must not keep the previous Alt+Space until that returns.
func (a *App) applyStagedMigrationHotkey() {
	hotkey := strings.TrimSpace(a.onboarding.onboardingMigration.stagedMainHotkey)
	if hotkey == "" {
		return
	}
	a.presentStagedMigrationHotkey(hotkey)
	if a.onboarding.onboardingMigration.mainHotkeyApplyStarted || a.services == nil {
		return
	}
	a.onboarding.onboardingMigration.mainHotkeyApplyStarted = true
	a.onboarding.onboardingMigration.mainHotkeyPending = true
	a.onboarding.invalidateOnboardingWindow()
	util.Go(a.lifecycleCtx, "register migrated main hotkey", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		err := a.services.UpdateGeneralSetting(ctx, a.sessionID, "MainHotkey", hotkey)
		cancel()
		_ = a.runOnUI("apply migrated main hotkey result", func() {
			a.onboarding.onboardingMigration.mainHotkeyPending = false
			if err != nil {
				a.onboarding.onboardingMigration.stagedMainHotkeyError = migrationHotkeyPageMessage(err)
				if a.generalSettings != nil {
					a.generalSettings.Update(func(data *settingsData) {
						data.MainHotkey = hotkey
						data.MainHotkeyRegistrationFailed = true
						data.MainHotkeyRegistrationError = a.onboarding.onboardingMigration.stagedMainHotkeyError
					})
				}
			} else {
				// Persistence now matches the imported chord, so later reloads can read it directly.
				a.onboarding.onboardingMigration.stagedMainHotkey = ""
				a.onboarding.onboardingMigration.stagedMainHotkeyError = ""
				if a.generalSettings != nil {
					a.generalSettings.Update(func(data *settingsData) {
						data.MainHotkey = hotkey
						data.MainHotkeyRegistrationFailed = false
						data.MainHotkeyRegistrationError = ""
					})
				}
			}
			a.presentStagedMigrationHotkey(hotkey)
			a.onboarding.invalidateOnboardingWindow()
		})
	})
}

// presentStagedMigrationHotkey paints the imported chord before registration finishes.
func (a *App) presentStagedMigrationHotkey(hotkey string) {
	if a.hotkeySettings == nil {
		return
	}
	if form := a.hotkeySettings.Form(); form != nil {
		form.values["MainHotkey"] = hotkey
	}
}

// overlayStagedMigrationHotkey keeps a settings reload from putting the old main shortcut back.
// The imported chord is not persisted until registration succeeds, so a reload still loads Alt+Space.
func overlayStagedMigrationHotkey(data settingsData, state onboardingMigrationState) settingsData {
	hotkey := strings.TrimSpace(state.stagedMainHotkey)
	if hotkey == "" {
		return data
	}
	data.MainHotkey = hotkey
	if state.stagedMainHotkeyError != "" {
		data.MainHotkeyRegistrationFailed = true
		data.MainHotkeyRegistrationError = state.stagedMainHotkeyError
	}
	return data
}

// migrationHotkeyPageMessage describes a failed main-hotkey registration.
// The OS reports that the shortcut is taken, not which process holds it.
func migrationHotkeyPageMessage(err error) string {
	if migrationHotkeyConflict(err) {
		return "i18n:onboarding_migrate_hotkey_next"
	}
	return corehotkey.RegistrationErrorKey(err)
}

func migrationApplyError(err error) string {
	if err == nil {
		return ""
	}
	if migrationHotkeyConflict(err) {
		return migrationHotkeyTaken
	}
	return err.Error()
}

// migrationHotkeyConflict is true only when the platform confirms the shortcut is already registered.
func migrationHotkeyConflict(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, keyboard.ErrHotkeyConflict) {
		return true
	}
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "1409") || strings.Contains(text, "already registered")
}

func (a *App) applyOnboardingMigrationSetting(ctx context.Context, write migrate.SettingWrite) error {
	if write.Key == "" {
		return nil
	}
	value, skip, err := a.migrationSettingValue(write.Key, write.Value)
	if err != nil || skip {
		return err
	}
	if write.PluginID != "" {
		if a.services != nil {
			return a.services.UpdatePluginSettings(ctx, a.sessionID, write.PluginID, map[string]string{write.Key: value})
		}
		return nil
	}
	if a.services != nil {
		if err := a.services.UpdateGeneralSetting(ctx, a.sessionID, write.Key, value); err != nil {
			return err
		}
	}
	a.rememberOnboardingMigrationSetting(write.Key, value)
	return nil
}

// migrationSettingValue prepares one imported setting for storage.
// Query hotkeys and query aliases are table rows, so the selected rows are added
// beside the rows already stored. A repeated hotkey chord or alias is left as it is.
// skip is true when every imported row is already present.
func (a *App) migrationSettingValue(key, incoming string) (value string, skip bool, err error) {
	switch key {
	case "QueryHotkeys":
		var rows []queryHotkeySetting
		if err := json.Unmarshal([]byte(incoming), &rows); err != nil {
			return "", false, err
		}
		var current []queryHotkeySetting
		if a.generalSettings != nil {
			current = a.generalSettings.Data().QueryHotkeys
		}
		merged := appendImportedQueryHotkeys(current, rows)
		if len(merged) == len(current) {
			return "", true, nil
		}
		raw, err := json.Marshal(merged)
		if err != nil {
			return "", false, err
		}
		return string(raw), false, nil
	case "QueryAliases":
		var rows []queryAliasSetting
		if err := json.Unmarshal([]byte(incoming), &rows); err != nil {
			return "", false, err
		}
		var current []queryAliasSetting
		if a.generalSettings != nil {
			current = a.generalSettings.Data().QueryAliases
		}
		merged := appendImportedQueryAliases(current, rows)
		if len(merged) == len(current) {
			return "", true, nil
		}
		raw, err := json.Marshal(merged)
		if err != nil {
			return "", false, err
		}
		return string(raw), false, nil
	default:
		return incoming, false, nil
	}
}

// appendImportedQueryHotkeys keeps current query hotkeys and adds imported rows that use a new chord.
func appendImportedQueryHotkeys(current, incoming []queryHotkeySetting) []queryHotkeySetting {
	seen := map[string]struct{}{}
	for _, item := range current {
		if id := queryHotkeyChord(item.Hotkey); id != "" {
			seen[id] = struct{}{}
		}
	}
	merged := append([]queryHotkeySetting(nil), current...)
	for _, item := range incoming {
		item.Hotkey = strings.TrimSpace(item.Hotkey)
		item.Query = strings.TrimSpace(item.Query)
		id := queryHotkeyChord(item.Hotkey)
		if id == "" || item.Query == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		if strings.TrimSpace(item.Position) == "" {
			item.Position = string(setting.QueryHotkeyPositionSystemDefault)
		}
		merged = append(merged, item)
	}
	return merged
}

// appendImportedQueryAliases keeps current aliases and adds imported rows whose alias is new.
func appendImportedQueryAliases(current, incoming []queryAliasSetting) []queryAliasSetting {
	seen := map[string]struct{}{}
	for _, item := range current {
		if id := queryAliasIdentity(item.Alias); id != "" {
			seen[id] = struct{}{}
		}
	}
	merged := append([]queryAliasSetting(nil), current...)
	for _, item := range incoming {
		item.Alias = strings.TrimSpace(item.Alias)
		item.Query = strings.TrimSpace(item.Query)
		id := queryAliasIdentity(item.Alias)
		if id == "" || item.Query == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		merged = append(merged, item)
	}
	return merged
}

func queryHotkeyChord(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if key, err := utilhotkey.BindingKey(value); err == nil && key != "" {
		return key
	}
	return strings.ToLower(strings.ReplaceAll(value, " ", ""))
}

func queryAliasIdentity(alias string) string {
	return strings.ToLower(strings.TrimSpace(alias))
}

// rememberOnboardingMigrationSetting keeps the in-memory snapshot aligned with a setting just imported.
// Later onboarding steps read that snapshot, so an imported hotkey is already filled in when that page opens.
func (a *App) rememberOnboardingMigrationSetting(key, value string) {
	_ = a.runOnUI("remember migrated setting", func() {
		if a.generalSettings != nil {
			a.generalSettings.Update(func(data *settingsData) {
				switch key {
				case "MainHotkey":
					data.MainHotkey = value
					data.MainHotkeyRegistrationFailed = false
					data.MainHotkeyRegistrationError = ""
				case "ActionPanelHotkey":
					data.ActionPanelHotkey = value
				case "IgnoreHotkeysOnFullscreen":
					data.IgnoreHotkeysOnFullscreen, _ = strconv.ParseBool(value)
				case "QueryHotkeys":
					_ = json.Unmarshal([]byte(value), &data.QueryHotkeys)
				case "QueryAliases":
					_ = json.Unmarshal([]byte(value), &data.QueryAliases)
				case "LaunchMode":
					data.LaunchMode = value
				case "StartPage":
					data.StartPage = value
				case "UsePinYin":
					data.UsePinYin, _ = strconv.ParseBool(value)
				case "SwitchInputMethodABC":
					data.SwitchInputMethodABC, _ = strconv.ParseBool(value)
				case "EnableAutostart":
					data.EnableAutostart, _ = strconv.ParseBool(value)
				case "HideOnStart":
					data.HideOnStart, _ = strconv.ParseBool(value)
				case "HideOnLostFocus":
					data.HideOnLostFocus, _ = strconv.ParseBool(value)
				case "ShowTray":
					data.ShowTray, _ = strconv.ParseBool(value)
				case "AppWidth":
					data.AppWidth, _ = strconv.Atoi(value)
				case "MaxResultCount":
					data.MaxResultCount, _ = strconv.Atoi(value)
				case "ShowPosition":
					data.ShowPosition = value
				case "LangCode":
					data.LangCode = value
				case "AppFontFamily":
					data.AppFontFamily = value
				case "CustomPythonPath":
					data.CustomPythonPath = value
				case "CustomNodejsPath":
					data.CustomNodejsPath = value
				case "HttpProxyEnabled":
					data.HttpProxyEnabled, _ = strconv.ParseBool(value)
				case "HttpProxyUrl":
					data.HttpProxyURL = value
				case "EnableAutoUpdate":
					data.EnableAutoUpdate, _ = strconv.ParseBool(value)
				case "ThemeId":
					data.ThemeID = value
				case "UiDensity":
					data.UIDensity = value
				case "AIMCPServers":
					data.AIMCPServers = json.RawMessage(value)
				}
			})
		}
		if key == "MainHotkey" && a.hotkeySettings != nil {
			if form := a.hotkeySettings.Form(); form != nil {
				form.values["MainHotkey"] = value
			}
		}
		if key == "QueryHotkeys" && a.hotkeySettings != nil {
			if form := a.hotkeySettings.Form(); form != nil {
				form.values["QueryHotkeys"] = value
			}
		}
		if key == "QueryAliases" && a.generalSettings != nil {
			if form := a.generalSettings.Form(); form != nil {
				form.values["QueryAliases"] = value
			}
		}
		a.onboarding.invalidateOnboardingWindow()
	})
}

func (a *App) onboardingMigrationView(imageScale float32, background woxui.Color, labels map[string]string) ([]launcherview.OnboardingMigrationSource, []launcherview.OnboardingMigrationCategory) {
	state := a.onboarding.onboardingMigration
	installations := state.installations
	if !state.choosing && state.selectedID != "" {
		if selected := a.migrationInstallation(state.selectedID); selected != nil {
			installations = []migrate.Installation{selected}
		}
	}
	sources := make([]launcherview.OnboardingMigrationSource, len(installations))
	for index, installation := range installations {
		sources[index] = launcherview.OnboardingMigrationSource{
			ID: installation.ID(), Name: installation.Name(), Version: installation.Version(), Location: installation.Location(),
			Icon: a.imageForSurface(imageFromCommon(installation.Icon()), physicalImageSize(28, imageScale), background),
		}
	}
	categories := make([]launcherview.OnboardingMigrationCategory, 0, len(state.categories))
	for _, category := range state.categories {
		viewCategory := launcherview.OnboardingMigrationCategory{
			ID: category.id, Title: a.language.translate("i18n:onboarding_migrate_category_" + category.id),
			Items: make([]launcherview.OnboardingMigrationItem, len(category.items)),
		}
		for index, item := range category.items {
			viewCategory.Items[index] = a.onboardingMigrationItemView(item, category.id, imageScale, background, labels, state.activeID)
		}
		categories = append(categories, viewCategory)
	}
	return sources, categories
}

func (a *App) onboardingMigrationItemView(item onboardingMigrationItem, categoryID string, imageScale float32, background woxui.Color, labels map[string]string, activeID string) launcherview.OnboardingMigrationItem {
	title := item.item.Title
	if item.item.TitleKey != "" {
		title = migrationNamedText(a.language.translate("i18n:onboarding_migrate_item_"+item.item.TitleKey), item.item.Detail)
	}
	detail := item.item.Detail
	detailLines := 1
	if item.item.DetailKey == "raycast_clipboard_summary" {
		detail = a.migrationClipboardSummary(item.item.Detail)
	} else if item.item.DetailKey != "" {
		translated := a.language.translate("i18n:onboarding_migrate_value_" + item.item.DetailKey)
		if strings.Contains(translated, "{name}") {
			detail = migrationNamedText(translated, item.item.Detail)
		} else if item.item.Detail != "" {
			detail = translated + "  " + item.item.Detail
		} else {
			detail = translated
		}
	}
	if item.item.Kind == migrate.ItemKindCloseLauncher {
		detailLines = 2
	}
	if item.item.DetailCode == migrate.DetailPythonMissing {
		if detail != "" {
			detail += " "
		}
		detail += labels["migrate.python"]
	}
	reason := item.err
	if reason == migrationHotkeyTaken {
		reason = a.language.translate("i18n:onboarding_migrate_hotkey_taken")
	} else if reason != "" {
		reason = a.migrationExportMessage(reason)
	}
	status := ""
	switch {
	case item.item.Status == migrate.PluginImported && item.err == "" && !item.copied:
		status = labels["migrate.imported"]
	case item.item.Status == migrate.PluginUnsupported && item.err == "" && !item.copied:
		status = labels["migrate.unsupported"]
	case item.item.Status == "conflict" && item.err == "" && !item.copied:
		status = labels["migrate.conflict"]
	case item.item.Status == "cancelled" && !item.copied:
		status = labels["migrate.cancelled"]
	}
	var icon *woxui.Image
	if categoryID == migrate.CategoryPlugins {
		icon = a.imageForSurface(imageFromCommon(item.item.Icon), physicalImageSize(28, imageScale), background)
	}
	option := ""
	if item.item.OptionKey != "" {
		option = a.language.translate("i18n:onboarding_migrate_option_" + item.item.OptionKey)
	}
	return launcherview.OnboardingMigrationItem{
		ID: item.item.ID, Title: title, Detail: detail, DetailLines: detailLines, Status: status, Error: reason, Icon: icon,
		Selected: item.selected, Selectable: item.item.Selectable, Required: item.item.Required, ShowIcon: categoryID == migrate.CategoryPlugins,
		Active: activeID != "" && activeID == item.item.ID && item.err == "" && !item.copied, Succeeded: item.copied,
		OptionLabel: option, OptionOn: item.item.OptionOn,
	}
}

func migrationNamedText(text, name string) string {
	if !strings.Contains(text, "{name}") {
		return text
	}
	return strings.ReplaceAll(text, "{name}", name)
}

func (a *App) migrationClipboardSummary(raw string) string {
	parts := strings.Split(raw, "|")
	for len(parts) < 5 {
		parts = append(parts, "0")
	}
	text := a.language.translate("i18n:onboarding_migrate_clipboard_summary")
	replacer := strings.NewReplacer(
		"{importable}", parts[0], "{duplicate}", parts[1], "{expired}", parts[2], "{capacity}", parts[3], "{missing}", parts[4],
	)
	return replacer.Replace(text)
}

func (a *App) selectedMigrationInstallation() migrate.Installation {
	return a.migrationInstallation(a.onboarding.onboardingMigration.selectedID)
}

func (a *App) migrationInstallation(id string) migrate.Installation {
	for _, installation := range a.onboarding.onboardingMigration.installations {
		if installation.ID() == id {
			return installation
		}
	}
	return nil
}

func migrationOutcome(result contract.MigratedItemResult) (string, string) {
	switch result.Status {
	case "", "imported":
		return "", "imported"
	case "exists":
		return "", "exists"
	case "conflict":
		return "", "conflict"
	case "cancelled":
		return "", "cancelled"
	default:
		code := result.ErrorCode
		if code == "" {
			code = "failed"
		}
		return code, result.Status
	}
}

func (a *App) applyOnboardingMigrationDomain(ctx context.Context, installation migrate.Installation, itemIDs []string) map[string]contract.MigratedItemResult {
	outcomes := map[string]contract.MigratedItemResult{}
	if len(itemIDs) == 0 {
		return outcomes
	}
	if a.services == nil {
		for _, id := range itemIDs {
			outcomes[id] = contract.MigratedItemResult{ID: id, Status: "failed", ErrorCode: "unavailable"}
		}
		return outcomes
	}
	result, err := installation.Import(ctx, itemIDs)
	if err != nil {
		for _, id := range itemIDs {
			outcomes[id] = contract.MigratedItemResult{ID: id, Status: "failed", ErrorCode: err.Error()}
		}
		return outcomes
	}
	for _, item := range a.services.ImportMigratedQuicklinks(ctx, a.sessionID, migrationQuicklinks(result.Quicklinks)) {
		outcomes[item.ID] = item
	}
	for _, item := range a.services.ImportMigratedFavorites(ctx, a.sessionID, migrationFavorites(result.Favorites)) {
		outcomes[item.ID] = item
	}
	for _, item := range a.services.ImportMigratedNotes(ctx, a.sessionID, migrationNotes(result.Notes)) {
		outcomes[item.ID] = item
	}
	for _, item := range a.services.ImportMigratedResultBindings(ctx, a.sessionID, migrationResultBindings(result.ResultBindings)) {
		outcomes[item.ID] = item
	}
	clipboardResults := a.services.ImportMigratedClipboard(ctx, a.sessionID, migrationClipboard(result.Clipboard))
	if migrationIDSelected(itemIDs, migrate.ClipboardSummaryID) {
		outcomes[migrate.ClipboardSummaryID] = clipboardSummaryOutcome(clipboardResults)
	} else {
		for _, item := range clipboardResults {
			outcomes[item.ID] = item
		}
	}
	for _, id := range itemIDs {
		if _, ok := outcomes[id]; !ok {
			outcomes[id] = contract.MigratedItemResult{ID: id, Status: "failed", ErrorCode: "failed"}
		}
	}
	return outcomes
}

func migrationQuicklinks(items []migrate.QuicklinkWrite) []contract.MigratedQuicklink {
	rows := make([]contract.MigratedQuicklink, len(items))
	for index, item := range items {
		rows[index] = contract.MigratedQuicklink{ItemID: item.ItemID, Title: item.Title, Keyword: item.Keyword, URL: item.URL, Browser: item.Browser}
	}
	return rows
}

func migrationResultBindings(items []migrate.ResultBindingWrite) []contract.MigratedResultBinding {
	rows := make([]contract.MigratedResultBinding, len(items))
	for index, item := range items {
		rows[index] = contract.MigratedResultBinding{
			ItemID: item.ItemID, PluginID: item.PluginID, Search: item.Search, Command: item.Command,
			TriggerKeyword: item.TriggerKeyword, IdentityKey: item.IdentityKey, Path: item.Path, Title: item.Title,
			Keyword: item.Keyword, RequireSettingsPath: item.RequireSettingsPath, Alias: item.Alias, Hotkey: item.Hotkey,
		}
	}
	return rows
}

func migrationFavorites(items []migrate.FavoriteWrite) []contract.MigratedFavorite {
	rows := make([]contract.MigratedFavorite, len(items))
	for index, item := range items {
		rows[index] = contract.MigratedFavorite{ItemID: item.ItemID, Name: item.Name, Path: item.Path}
	}
	return rows
}

func migrationNotes(items []migrate.NoteWrite) []contract.MigratedNote {
	rows := make([]contract.MigratedNote, len(items))
	for index, item := range items {
		rows[index] = contract.MigratedNote{
			ItemID: item.ItemID, Source: "raycast", SourceID: item.SourceID, Fingerprint: item.Fingerprint,
			Title: item.Title, Body: item.Body, Document: item.Document,
		}
	}
	return rows
}

func migrationIDSelected(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

// clipboardSummaryOutcome folds per-entry clipboard results into the single summary row.
func clipboardSummaryOutcome(results []contract.MigratedItemResult) contract.MigratedItemResult {
	outcome := contract.MigratedItemResult{ID: migrate.ClipboardSummaryID, Status: "imported"}
	imported := 0
	var failure contract.MigratedItemResult
	for _, result := range results {
		switch result.Status {
		case "", "imported":
			imported++
		case "exists":
		default:
			if failure.Status == "" {
				failure = result
			}
		}
	}
	if imported == 0 && failure.Status != "" {
		code := failure.ErrorCode
		if code == "" {
			code = failure.Status
		}
		return contract.MigratedItemResult{ID: migrate.ClipboardSummaryID, Status: "failed", ErrorCode: code}
	}
	return outcome
}

func migrationClipboard(items []migrate.ClipboardWrite) []contract.MigratedClipboard {
	rows := make([]contract.MigratedClipboard, len(items))
	for index, item := range items {
		rows[index] = contract.MigratedClipboard{ItemID: item.ItemID, Text: item.Text, ImagePath: item.ImagePath, FilePaths: append([]string(nil), item.FilePaths...), OCRText: item.OCRText, CreatedAt: item.CreatedAt, Pinned: item.Pinned}
	}
	return rows
}

func (a *App) migrationItemKind(id string) string {
	for _, category := range a.onboarding.onboardingMigration.categories {
		for _, item := range category.items {
			if item.item.ID != id {
				continue
			}
			if item.item.Kind != "" {
				return item.item.Kind
			}
		}
	}
	if strings.HasPrefix(id, "setting:") {
		return migrate.ItemKindSetting
	}
	return migrate.ItemKindPlugin
}

func (a *App) migrationNeedsUnlock() bool {
	export, ok := a.selectedMigrationInstallation().(migrate.ExportSource)
	return ok && export != nil && !a.onboarding.onboardingMigration.unlocked
}

func (a *App) selectedExportSource() migrate.ExportSource {
	export, _ := a.selectedMigrationInstallation().(migrate.ExportSource)
	return export
}

func (a *App) prefillOnboardingMigrationFile() {
	if !util.IsTestMode() || a.onboarding.onboardingMigration.exportPath != "" {
		return
	}
	path := strings.TrimSpace(os.Getenv(util.TestRaycastFileEnv))
	if path == "" {
		return
	}
	if _, err := os.Stat(path); err != nil {
		return
	}
	a.onboarding.onboardingMigration.exportPath = path
}

func (a *App) migrationPassphraseController() *woxwidget.TextEditingController {
	if a.onboarding.onboardingMigration.passphrase == nil {
		a.onboarding.onboardingMigration.passphrase = woxwidget.NewTextEditingController("")
	}
	return a.onboarding.onboardingMigration.passphrase
}

// releaseOnboardingMigrationSession drops decrypted export data and the passphrase.
func (a *App) releaseOnboardingMigrationSession() {
	if a.onboarding.onboardingMigration.sessionCancel != nil {
		a.onboarding.onboardingMigration.sessionCancel()
		a.onboarding.onboardingMigration.sessionCancel = nil
	}
	if export := a.selectedExportSource(); export != nil {
		export.Reset()
	}
	if a.onboarding.onboardingMigration.passphrase != nil {
		a.onboarding.onboardingMigration.passphrase.SetText("", false)
	}
	state := &a.onboarding.onboardingMigration
	state.unlocked = false
	state.exportPath = ""
	state.categories = nil
	state.loaded = false
	state.loading = false
	state.error = ""
	state.done = false
	state.activeID = ""
}

func (a *App) pickOnboardingMigrationFile() {
	if a.onboarding.onboardingMigration.importing {
		return
	}
	window := a.onboarding.onboardingNativeWindow()
	if window == nil {
		return
	}
	path, err := window.PickFile(woxui.FileDialogOptions{Filter: "*.rayconfig"})
	if err != nil || strings.TrimSpace(path) == "" {
		return
	}
	a.releaseOnboardingMigrationSession()
	a.onboarding.onboardingMigration.exportPath = path
	if export := a.selectedExportSource(); export != nil {
		if ok, detail := export.Recognize(path); !ok {
			a.onboarding.onboardingMigration.error = a.migrationExportMessage(detail)
		}
	}
	a.onboarding.invalidateOnboardingWindow()
}

func (a *App) unlockOnboardingMigration() {
	export := a.selectedExportSource()
	if export == nil || a.onboarding.onboardingMigration.importing || a.onboarding.onboardingMigration.loading {
		return
	}
	path := strings.TrimSpace(a.onboarding.onboardingMigration.exportPath)
	if path == "" {
		a.onboarding.onboardingMigration.error = a.language.translate("i18n:onboarding_migrate_raycast_file")
		a.onboarding.invalidateOnboardingWindow()
		return
	}
	if ok, detail := export.Recognize(path); !ok {
		a.onboarding.onboardingMigration.error = a.migrationExportMessage(detail)
		a.onboarding.invalidateOnboardingWindow()
		return
	}
	passphrase := ""
	if controller := a.onboarding.onboardingMigration.passphrase; controller != nil {
		passphrase = controller.Text()
		controller.SetText("", false)
	}
	before := export.Generation()
	a.onboarding.onboardingMigration.loading = true
	a.onboarding.onboardingMigration.error = ""
	a.onboarding.invalidateOnboardingWindow()
	util.Go(a.lifecycleCtx, "unlock onboarding migration", func() {
		err := export.Unlock(context.Background(), path, passphrase)
		_ = a.runOnUI("apply onboarding migration unlock", func() {
			if export.Generation() != before+1 || a.selectedExportSource() == nil {
				return
			}
			a.onboarding.onboardingMigration.loading = false
			if err != nil {
				a.onboarding.onboardingMigration.unlocked = false
				a.onboarding.onboardingMigration.error = a.migrationExportMessage(err.Error())
				a.onboarding.invalidateOnboardingWindow()
				return
			}
			a.onboarding.onboardingMigration.unlocked = true
			a.onboarding.onboardingMigration.error = ""
			a.loadOnboardingMigrationCatalog()
		})
	})
}

func (a *App) toggleOnboardingMigrationOption(id string, keep bool) {
	export := a.selectedExportSource()
	if export == nil || !export.SetKeepKeyword(id, keep) {
		return
	}
	catalog, err := export.Catalog(context.Background())
	if err != nil {
		return
	}
	fresh := map[string]migrate.Item{}
	for _, category := range catalog {
		for _, item := range category.Items {
			fresh[item.ID] = item
		}
	}
	for categoryIndex := range a.onboarding.onboardingMigration.categories {
		for index := range a.onboarding.onboardingMigration.categories[categoryIndex].items {
			item := &a.onboarding.onboardingMigration.categories[categoryIndex].items[index]
			if item.item.ID != id {
				continue
			}
			if updated, ok := fresh[id]; ok {
				selected := item.selected && updated.Selectable
				item.item = updated
				item.selected = selected
			}
		}
	}
	a.onboarding.invalidateOnboardingWindow()
}

func (a *App) migrationExportMessage(code string) string {
	switch code {
	case "not_export":
		return a.language.translate("i18n:onboarding_migrate_raycast_not_export")
	case "unsupported_version":
		return a.language.translate("i18n:onboarding_migrate_raycast_unsupported_version")
	case "corrupt":
		return a.language.translate("i18n:onboarding_migrate_raycast_corrupt")
	case "auth":
		return a.language.translate("i18n:onboarding_migrate_raycast_auth")
	case "limit":
		return a.language.translate("i18n:onboarding_migrate_raycast_limit")
	case "export_required":
		return a.language.translate("i18n:onboarding_migrate_raycast_export_required")
	case "keyword_taken":
		return a.language.translate("i18n:onboarding_migrate_value_raycast_keyword_taken")
	case "conflict":
		return a.language.translate("i18n:onboarding_migrate_status_conflict")
	case "missing":
		return a.language.translate("i18n:onboarding_migrate_value_raycast_image_missing")
	case "expired":
		return a.language.translate("i18n:onboarding_migrate_value_raycast_clipboard_expired")
	case "capacity":
		return a.language.translate("i18n:onboarding_migrate_value_raycast_clipboard_capacity")
	case "cancelled":
		return a.language.translate("i18n:onboarding_migrate_status_cancelled")
	case "unavailable", "failed":
		return a.language.translate("i18n:onboarding_migrate_status_failed")
	case "close_launcher":
		return a.language.translate("i18n:onboarding_migrate_close_launcher_failed")
	default:
		if strings.HasPrefix(code, "i18n:") {
			return a.language.translate(code)
		}
		return code
	}
}

func imageFromCommon(image common.WoxImage) woxImage {
	return woxImage{ImageType: image.ImageType, ImageData: image.ImageData}
}

// normalizeLauncherHotkey converts a foreign shortcut into the form Wox stores.
func normalizeLauncherHotkey(value string) (string, bool) {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, " + ", "+")
	value = strings.ReplaceAll(value, " ", "")
	if value == "" {
		return "", false
	}
	if _, ok := woxui.ParseHotkey(value); !ok {
		return "", false
	}
	return value, true
}
