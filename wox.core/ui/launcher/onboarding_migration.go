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

func (c *onboardingController) prepareOnboardingMigration() {
	state := &c.migration
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
	if state.selectedID != "" && !state.loaded && !state.loading && !c.migrationNeedsUnlock() {
		c.loadOnboardingMigrationCatalog()
	}
	if c.migrationNeedsUnlock() {
		c.prefillOnboardingMigrationFile()
	}
}

func (c *onboardingController) loadOnboardingMigrationCatalog() {
	installation := c.selectedMigrationInstallation()
	if installation == nil {
		return
	}
	c.migration.loading = true
	c.migration.error = ""
	c.invalidateOnboardingWindow()
	var generation uint64
	if export, ok := installation.(migrate.ExportSource); ok {
		generation = export.Generation()
	}
	util.Go(c.lifecycleCtx, "load onboarding migration catalog", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		catalog, err := installation.Catalog(ctx)
		cancel()
		_ = c.runOnUI("apply onboarding migration catalog", func() {
			if c.selectedMigrationInstallation() == nil || c.selectedMigrationInstallation().ID() != installation.ID() {
				return
			}
			if export, ok := installation.(migrate.ExportSource); ok && export.Generation() != generation {
				return
			}
			state := &c.migration
			state.loading = false
			state.loaded = err == nil
			if err != nil {
				state.error = err.Error()
				state.categories = nil
			} else {
				state.error = ""
				state.categories = onboardingMigrationCategories(installation, catalog)
			}
			c.invalidateOnboardingWindow()
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

func (c *onboardingController) chooseOnboardingMigration(id string) {
	if c.migration.importing {
		return
	}
	if c.migrationInstallation(id) == nil {
		return
	}
	c.releaseOnboardingMigrationSession()
	c.migration.selectedID = id
	c.migration.choosing = false
	c.migration.categories = nil
	c.migration.loaded = false
	c.migration.done = false
	c.migration.error = ""
	if !c.migrationNeedsUnlock() {
		c.loadOnboardingMigrationCatalog()
	} else {
		c.prefillOnboardingMigrationFile()
	}
	c.invalidateOnboardingWindow()
}

func (c *onboardingController) toggleOnboardingMigrationItem(id string, selected bool) {
	if c.migration.importing || c.migration.done {
		return
	}
	for categoryIndex := range c.migration.categories {
		items := c.migration.categories[categoryIndex].items
		for index := range items {
			item := &items[index]
			if item.item.ID == id && item.item.Selectable && !item.item.Required {
				item.selected = selected
			}
		}
	}
	c.invalidateOnboardingWindow()
}

// retreatOnboardingMigration returns to the launcher picker when several are installed.
func (c *onboardingController) retreatOnboardingMigration() bool {
	state := &c.migration
	if state.importing || state.done || state.choosing || len(state.installations) < 2 || state.selectedID == "" {
		return false
	}
	c.releaseOnboardingMigrationSession()
	state.choosing = true
	state.selectedID = ""
	state.categories = nil
	state.loaded = false
	state.loading = false
	state.error = ""
	c.invalidateOnboardingWindow()
	return true
}

func (c *onboardingController) onboardingMigrationCanImport() bool {
	state := &c.migration
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

func (c *onboardingController) importOnboardingMigration() {
	if !c.onboardingMigrationCanImport() {
		return
	}
	installation := c.selectedMigrationInstallation()
	if installation == nil {
		return
	}
	var itemIDs []string
	for _, category := range c.migration.categories {
		for _, item := range category.items {
			if migrationItemChosen(item) {
				itemIDs = append(itemIDs, item.item.ID)
			}
		}
	}
	c.migration.importing = true
	c.migration.activeID = itemIDs[0]
	c.migration.error = ""
	session, sessionCancel := context.WithCancel(context.Background())
	c.migration.sessionCancel = sessionCancel
	kinds := map[string]string{}
	for _, id := range itemIDs {
		kinds[id] = c.migrationItemKind(id)
	}
	c.invalidateOnboardingWindow()
	util.Go(c.lifecycleCtx, "import onboarding migration", func() {
		defer sessionCancel()
		dataCtx, dataCancel := context.WithTimeout(session, 2*time.Minute)
		defer dataCancel()
		groups, failures := c.prepareOnboardingMigrationSettings(dataCtx, installation, itemIDs)
		// A launcher that is still running keeps the global shortcuts, so the later rows are skipped.
		closed := true
		for _, id := range itemIDs {
			if session.Err() != nil || !closed {
				c.finishOnboardingMigrationItem(id, "", "cancelled")
				continue
			}
			c.showOnboardingMigrationItem(id)
			started := time.Now()
			itemErr, status := c.importVisibleOnboardingMigrationItem(dataCtx, installation, id, kinds[id], groups, failures)
			waitOnboardingMigrationDisplay(session, started)
			c.finishOnboardingMigrationItem(id, itemErr, status)
			if kinds[id] == migrate.ItemKindCloseLauncher && itemErr != "" {
				closed = false
			}
		}
		_ = c.runOnUI("finish onboarding migration", func() {
			state := &c.migration
			state.importing = false
			state.activeID = ""
			state.done = true
			state.sessionCancel = nil
			c.invalidateOnboardingWindow()
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

func (c *onboardingController) prepareOnboardingMigrationSettings(ctx context.Context, installation migrate.Installation, itemIDs []string) (map[string]*migrationWriteGroup, map[string]string) {
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
func (c *onboardingController) importVisibleOnboardingMigrationItem(ctx context.Context, installation migrate.Installation, id, kind string, groups map[string]*migrationWriteGroup, failures map[string]string) (string, string) {
	switch kind {
	case migrate.ItemKindPlugin:
		return c.importOnboardingMigrationPlugin(ctx, installation, id), ""
	case migrate.ItemKindCloseLauncher:
		return c.closeOnboardingMigrationLauncher(ctx, installation)
	case migrate.ItemKindQuicklink, migrate.ItemKindFavorite, migrate.ItemKindNote, migrate.ItemKindClipboard, migrate.ItemKindResultBinding:
		outcome, ok := c.applyOnboardingMigrationDomain(ctx, installation, []string{id})[id]
		if !ok {
			return "failed", "failed"
		}
		return migrationOutcome(outcome)
	default:
		itemErr := c.importOnboardingMigrationSetting(ctx, id, groups, failures)
		if itemErr == "" {
			return "", "imported"
		}
		return itemErr, ""
	}
}

func (c *onboardingController) importOnboardingMigrationSetting(ctx context.Context, id string, groups map[string]*migrationWriteGroup, failures map[string]string) string {
	if group := groups[id]; group != nil {
		return c.applyMigrationWriteGroup(ctx, group)
	}
	if err := failures[id]; err != "" {
		return err
	}
	return "cannot be imported"
}

func (c *onboardingController) applyMigrationWriteGroup(ctx context.Context, group *migrationWriteGroup) string {
	if group.applied {
		return group.err
	}
	group.applied = true
	var errs []string
	for _, write := range group.writes {
		// The other launcher still owns this shortcut, so registering it here fails.
		// Keep the value for the next hotkey page, which registers it and can ask for another.
		if write.Key == "MainHotkey" {
			c.stageMigratedMainHotkey(write.Value)
			continue
		}
		if err := c.applyOnboardingMigrationSetting(ctx, write); err != nil {
			errs = append(errs, migrationApplyError(err))
		}
	}
	for _, theme := range group.themes {
		if err := c.applyMigratedTheme(ctx, theme.Theme); err != nil {
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
func (c *onboardingController) applyMigratedTheme(ctx context.Context, theme common.Theme) error {
	if strings.TrimSpace(theme.ThemeId) == "" || c.services == nil {
		return errors.New("theme cannot be imported")
	}
	if err := c.services.InstallMigratedTheme(ctx, c.sessionID, theme); err != nil {
		return err
	}
	c.rememberOnboardingMigrationSetting("ThemeId", theme.ThemeId)
	_ = c.runOnUI("remember imported onboarding theme", func() {
		c.migration.themeImported = true
	})
	return nil
}

// closeOnboardingMigrationLauncher quits the source launcher before later rows register its hotkeys.
func (c *onboardingController) closeOnboardingMigrationLauncher(ctx context.Context, installation migrate.Installation) (string, string) {
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

func (c *onboardingController) importOnboardingMigrationPlugin(ctx context.Context, installation migrate.Installation, id string) string {
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

func (c *onboardingController) showOnboardingMigrationItem(id string) {
	_ = c.runOnUI("show onboarding migration item", func() {
		c.migration.activeID = id
		c.invalidateOnboardingWindow()
	})
}

func (c *onboardingController) finishOnboardingMigrationItem(id, itemErr, status string) {
	_ = c.runOnUI("finish onboarding migration item", func() {
		if c.migration.activeID == id {
			c.migration.activeID = ""
		}
		for categoryIndex := range c.migration.categories {
			for index := range c.migration.categories[categoryIndex].items {
				item := &c.migration.categories[categoryIndex].items[index]
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
		c.invalidateOnboardingWindow()
	})
}

// stageMigratedMainHotkey records the shortcut without registering it with the OS.
func (c *onboardingController) stageMigratedMainHotkey(hotkey string) {
	hotkey = strings.TrimSpace(hotkey)
	if hotkey == "" {
		return
	}
	c.migration.stagedMainHotkey = hotkey
	c.rememberOnboardingMigrationSetting("MainHotkey", hotkey)
}

// applyStagedMigrationHotkey registers the staged main shortcut once the hotkey page is open.
// The imported chord is shown immediately. Registration can take a moment when another
// program still holds it, and the page must not keep the previous Alt+Space until that returns.
func (c *onboardingController) applyStagedMigrationHotkey() {
	hotkey := strings.TrimSpace(c.migration.stagedMainHotkey)
	if hotkey == "" {
		return
	}
	c.presentStagedMigrationHotkey(hotkey)
	if c.migration.mainHotkeyApplyStarted || c.services == nil {
		return
	}
	c.migration.mainHotkeyApplyStarted = true
	c.migration.mainHotkeyPending = true
	c.invalidateOnboardingWindow()
	util.Go(c.lifecycleCtx, "register migrated main hotkey", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		err := c.services.UpdateGeneralSetting(ctx, c.sessionID, "MainHotkey", hotkey)
		cancel()
		_ = c.runOnUI("apply migrated main hotkey result", func() {
			c.migration.mainHotkeyPending = false
			if err != nil {
				c.migration.stagedMainHotkeyError = migrationHotkeyPageMessage(err)
				if c.generalSettings() != nil {
					c.generalSettings().Update(func(data *settingsData) {
						data.MainHotkey = hotkey
						data.MainHotkeyRegistrationFailed = true
						data.MainHotkeyRegistrationError = c.migration.stagedMainHotkeyError
					})
				}
			} else {
				// Persistence now matches the imported chord, so later reloads can read it directly.
				c.migration.stagedMainHotkey = ""
				c.migration.stagedMainHotkeyError = ""
				if c.generalSettings() != nil {
					c.generalSettings().Update(func(data *settingsData) {
						data.MainHotkey = hotkey
						data.MainHotkeyRegistrationFailed = false
						data.MainHotkeyRegistrationError = ""
					})
				}
			}
			c.presentStagedMigrationHotkey(hotkey)
			c.invalidateOnboardingWindow()
		})
	})
}

// presentStagedMigrationHotkey paints the imported chord before registration finishes.
func (c *onboardingController) presentStagedMigrationHotkey(hotkey string) {
	if c.hotkeySettings() == nil {
		return
	}
	if form := c.hotkeySettings().Form(); form != nil {
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

// applyStagedHotkey overlays the staged migration shortcut while the guide is open.
func (c *onboardingController) applyStagedHotkey(data settingsData) settingsData {
	if !c.open {
		return data
	}
	return overlayStagedMigrationHotkey(data, c.migration)
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

func (c *onboardingController) applyOnboardingMigrationSetting(ctx context.Context, write migrate.SettingWrite) error {
	if write.Key == "" {
		return nil
	}
	value, skip, err := c.migrationSettingValue(write.Key, write.Value)
	if err != nil || skip {
		return err
	}
	if write.PluginID != "" {
		if c.services != nil {
			return c.services.UpdatePluginSettings(ctx, c.sessionID, write.PluginID, map[string]string{write.Key: value})
		}
		return nil
	}
	if c.services != nil {
		if err := c.services.UpdateGeneralSetting(ctx, c.sessionID, write.Key, value); err != nil {
			return err
		}
	}
	c.rememberOnboardingMigrationSetting(write.Key, value)
	return nil
}

// migrationSettingValue prepares one imported setting for storage.
// Query hotkeys and query aliases are table rows, so the selected rows are added
// beside the rows already stored. A repeated hotkey chord or alias is left as it is.
// skip is true when every imported row is already present.
func (c *onboardingController) migrationSettingValue(key, incoming string) (value string, skip bool, err error) {
	switch key {
	case "QueryHotkeys":
		var rows []queryHotkeySetting
		if err := json.Unmarshal([]byte(incoming), &rows); err != nil {
			return "", false, err
		}
		var current []queryHotkeySetting
		if c.generalSettings() != nil {
			current = c.generalSettings().Data().QueryHotkeys
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
		if c.generalSettings() != nil {
			current = c.generalSettings().Data().QueryAliases
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
func (c *onboardingController) rememberOnboardingMigrationSetting(key, value string) {
	_ = c.runOnUI("remember migrated setting", func() {
		if c.generalSettings() != nil {
			c.generalSettings().Update(func(data *settingsData) {
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
		if key == "MainHotkey" && c.hotkeySettings() != nil {
			if form := c.hotkeySettings().Form(); form != nil {
				form.values["MainHotkey"] = value
			}
		}
		if key == "QueryHotkeys" && c.hotkeySettings() != nil {
			if form := c.hotkeySettings().Form(); form != nil {
				form.values["QueryHotkeys"] = value
			}
		}
		if key == "QueryAliases" && c.generalSettings() != nil {
			if form := c.generalSettings().Form(); form != nil {
				form.values["QueryAliases"] = value
			}
		}
		c.invalidateOnboardingWindow()
	})
}

func (c *onboardingController) onboardingMigrationView(imageScale float32, background woxui.Color, labels map[string]string) ([]launcherview.OnboardingMigrationSource, []launcherview.OnboardingMigrationCategory) {
	state := c.migration
	installations := state.installations
	if !state.choosing && state.selectedID != "" {
		if selected := c.migrationInstallation(state.selectedID); selected != nil {
			installations = []migrate.Installation{selected}
		}
	}
	sources := make([]launcherview.OnboardingMigrationSource, len(installations))
	for index, installation := range installations {
		sources[index] = launcherview.OnboardingMigrationSource{
			ID: installation.ID(), Name: installation.Name(), Version: installation.Version(), Location: installation.Location(),
			Icon: c.imageForSurface(imageFromCommon(installation.Icon()), physicalImageSize(28, imageScale), background),
		}
	}
	categories := make([]launcherview.OnboardingMigrationCategory, 0, len(state.categories))
	for _, category := range state.categories {
		viewCategory := launcherview.OnboardingMigrationCategory{
			ID: category.id, Title: c.language.translate("i18n:onboarding_migrate_category_" + category.id),
			Items: make([]launcherview.OnboardingMigrationItem, len(category.items)),
		}
		for index, item := range category.items {
			viewCategory.Items[index] = c.onboardingMigrationItemView(item, category.id, imageScale, background, labels, state.activeID)
		}
		categories = append(categories, viewCategory)
	}
	return sources, categories
}

func (c *onboardingController) onboardingMigrationItemView(item onboardingMigrationItem, categoryID string, imageScale float32, background woxui.Color, labels map[string]string, activeID string) launcherview.OnboardingMigrationItem {
	title := item.item.Title
	if item.item.TitleKey != "" {
		title = migrationNamedText(c.language.translate("i18n:onboarding_migrate_item_"+item.item.TitleKey), item.item.Detail)
	}
	detail := item.item.Detail
	detailLines := 1
	if item.item.DetailKey == "raycast_clipboard_summary" {
		detail = c.migrationClipboardSummary(item.item.Detail)
	} else if item.item.DetailKey != "" {
		translated := c.language.translate("i18n:onboarding_migrate_value_" + item.item.DetailKey)
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
		reason = c.language.translate("i18n:onboarding_migrate_hotkey_taken")
	} else if reason != "" {
		reason = c.migrationExportMessage(reason)
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
		icon = c.imageForSurface(imageFromCommon(item.item.Icon), physicalImageSize(28, imageScale), background)
	}
	option := ""
	if item.item.OptionKey != "" {
		option = c.language.translate("i18n:onboarding_migrate_option_" + item.item.OptionKey)
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

func (c *onboardingController) migrationClipboardSummary(raw string) string {
	parts := strings.Split(raw, "|")
	for len(parts) < 5 {
		parts = append(parts, "0")
	}
	text := c.language.translate("i18n:onboarding_migrate_clipboard_summary")
	replacer := strings.NewReplacer(
		"{importable}", parts[0], "{duplicate}", parts[1], "{expired}", parts[2], "{capacity}", parts[3], "{missing}", parts[4],
	)
	return replacer.Replace(text)
}

func (c *onboardingController) selectedMigrationInstallation() migrate.Installation {
	return c.migrationInstallation(c.migration.selectedID)
}

func (c *onboardingController) migrationInstallation(id string) migrate.Installation {
	for _, installation := range c.migration.installations {
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

func (c *onboardingController) applyOnboardingMigrationDomain(ctx context.Context, installation migrate.Installation, itemIDs []string) map[string]contract.MigratedItemResult {
	outcomes := map[string]contract.MigratedItemResult{}
	if len(itemIDs) == 0 {
		return outcomes
	}
	if c.services == nil {
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
	for _, item := range c.services.ImportMigratedQuicklinks(ctx, c.sessionID, migrationQuicklinks(result.Quicklinks)) {
		outcomes[item.ID] = item
	}
	for _, item := range c.services.ImportMigratedFavorites(ctx, c.sessionID, migrationFavorites(result.Favorites)) {
		outcomes[item.ID] = item
	}
	for _, item := range c.services.ImportMigratedNotes(ctx, c.sessionID, migrationNotes(result.Notes)) {
		outcomes[item.ID] = item
	}
	for _, item := range c.services.ImportMigratedResultBindings(ctx, c.sessionID, migrationResultBindings(result.ResultBindings)) {
		outcomes[item.ID] = item
	}
	clipboardResults := c.services.ImportMigratedClipboard(ctx, c.sessionID, migrationClipboard(result.Clipboard))
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

func (c *onboardingController) migrationItemKind(id string) string {
	for _, category := range c.migration.categories {
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

func (c *onboardingController) migrationNeedsUnlock() bool {
	export, ok := c.selectedMigrationInstallation().(migrate.ExportSource)
	return ok && export != nil && !c.migration.unlocked
}

func (c *onboardingController) selectedExportSource() migrate.ExportSource {
	export, _ := c.selectedMigrationInstallation().(migrate.ExportSource)
	return export
}

func (c *onboardingController) prefillOnboardingMigrationFile() {
	if !util.IsTestMode() || c.migration.exportPath != "" {
		return
	}
	path := strings.TrimSpace(os.Getenv(util.TestRaycastFileEnv))
	if path == "" {
		return
	}
	if _, err := os.Stat(path); err != nil {
		return
	}
	c.migration.exportPath = path
}

func (c *onboardingController) migrationPassphraseController() *woxwidget.TextEditingController {
	if c.migration.passphrase == nil {
		c.migration.passphrase = woxwidget.NewTextEditingController("")
	}
	return c.migration.passphrase
}

// releaseOnboardingMigrationSession drops decrypted export data and the passphrase.
func (c *onboardingController) releaseOnboardingMigrationSession() {
	if c.migration.sessionCancel != nil {
		c.migration.sessionCancel()
		c.migration.sessionCancel = nil
	}
	if export := c.selectedExportSource(); export != nil {
		export.Reset()
	}
	if c.migration.passphrase != nil {
		c.migration.passphrase.SetText("", false)
	}
	state := &c.migration
	state.unlocked = false
	state.exportPath = ""
	state.categories = nil
	state.loaded = false
	state.loading = false
	state.error = ""
	state.done = false
	state.activeID = ""
}

func (c *onboardingController) pickOnboardingMigrationFile() {
	if c.migration.importing {
		return
	}
	window := c.onboardingNativeWindow()
	if window == nil {
		return
	}
	path, err := window.PickFile(woxui.FileDialogOptions{Filter: "*.rayconfig"})
	if err != nil || strings.TrimSpace(path) == "" {
		return
	}
	c.releaseOnboardingMigrationSession()
	c.migration.exportPath = path
	if export := c.selectedExportSource(); export != nil {
		if ok, detail := export.Recognize(path); !ok {
			c.migration.error = c.migrationExportMessage(detail)
		}
	}
	c.invalidateOnboardingWindow()
}

func (c *onboardingController) unlockOnboardingMigration() {
	export := c.selectedExportSource()
	if export == nil || c.migration.importing || c.migration.loading {
		return
	}
	path := strings.TrimSpace(c.migration.exportPath)
	if path == "" {
		c.migration.error = c.language.translate("i18n:onboarding_migrate_raycast_file")
		c.invalidateOnboardingWindow()
		return
	}
	if ok, detail := export.Recognize(path); !ok {
		c.migration.error = c.migrationExportMessage(detail)
		c.invalidateOnboardingWindow()
		return
	}
	passphrase := ""
	if controller := c.migration.passphrase; controller != nil {
		passphrase = controller.Text()
		controller.SetText("", false)
	}
	before := export.Generation()
	c.migration.loading = true
	c.migration.error = ""
	c.invalidateOnboardingWindow()
	util.Go(c.lifecycleCtx, "unlock onboarding migration", func() {
		err := export.Unlock(context.Background(), path, passphrase)
		_ = c.runOnUI("apply onboarding migration unlock", func() {
			if export.Generation() != before+1 || c.selectedExportSource() == nil {
				return
			}
			c.migration.loading = false
			if err != nil {
				c.migration.unlocked = false
				c.migration.error = c.migrationExportMessage(err.Error())
				c.invalidateOnboardingWindow()
				return
			}
			c.migration.unlocked = true
			c.migration.error = ""
			c.loadOnboardingMigrationCatalog()
		})
	})
}

func (c *onboardingController) toggleOnboardingMigrationOption(id string, keep bool) {
	export := c.selectedExportSource()
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
	for categoryIndex := range c.migration.categories {
		for index := range c.migration.categories[categoryIndex].items {
			item := &c.migration.categories[categoryIndex].items[index]
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
	c.invalidateOnboardingWindow()
}

func (c *onboardingController) migrationExportMessage(code string) string {
	switch code {
	case "not_export":
		return c.language.translate("i18n:onboarding_migrate_raycast_not_export")
	case "unsupported_version":
		return c.language.translate("i18n:onboarding_migrate_raycast_unsupported_version")
	case "corrupt":
		return c.language.translate("i18n:onboarding_migrate_raycast_corrupt")
	case "auth":
		return c.language.translate("i18n:onboarding_migrate_raycast_auth")
	case "limit":
		return c.language.translate("i18n:onboarding_migrate_raycast_limit")
	case "export_required":
		return c.language.translate("i18n:onboarding_migrate_raycast_export_required")
	case "keyword_taken":
		return c.language.translate("i18n:onboarding_migrate_value_raycast_keyword_taken")
	case "conflict":
		return c.language.translate("i18n:onboarding_migrate_status_conflict")
	case "missing":
		return c.language.translate("i18n:onboarding_migrate_value_raycast_image_missing")
	case "expired":
		return c.language.translate("i18n:onboarding_migrate_value_raycast_clipboard_expired")
	case "capacity":
		return c.language.translate("i18n:onboarding_migrate_value_raycast_clipboard_capacity")
	case "cancelled":
		return c.language.translate("i18n:onboarding_migrate_status_cancelled")
	case "unavailable", "failed":
		return c.language.translate("i18n:onboarding_migrate_status_failed")
	case "close_launcher":
		return c.language.translate("i18n:onboarding_migrate_close_launcher_failed")
	default:
		if strings.HasPrefix(code, "i18n:") {
			return c.language.translate(code)
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
