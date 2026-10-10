package launcher

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"wox/common"
	corehotkey "wox/hotkey"
	"wox/plugin/thirdparty/migrate"
	"wox/setting"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
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
	mainHotkeyPending      bool
	mainHotkeyApplyStarted bool
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
	state := &a.onboardingMigration
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
	if state.selectedID != "" && !state.loaded && !state.loading {
		a.loadOnboardingMigrationCatalog()
	}
}

func (a *App) loadOnboardingMigrationCatalog() {
	installation := a.selectedMigrationInstallation()
	if installation == nil {
		return
	}
	a.onboardingMigration.loading = true
	a.onboardingMigration.error = ""
	util.Go(a.lifecycleCtx, "load onboarding migration catalog", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		catalog, err := installation.Catalog(ctx)
		cancel()
		_ = a.runOnUI("apply onboarding migration catalog", func() {
			if a.selectedMigrationInstallation() == nil || a.selectedMigrationInstallation().ID() != installation.ID() {
				return
			}
			state := &a.onboardingMigration
			state.loading = false
			state.loaded = err == nil
			if err != nil {
				state.error = err.Error()
				state.categories = nil
			} else {
				state.error = ""
				state.categories = onboardingMigrationCategories(catalog)
			}
			a.invalidateOnboardingWindow()
		})
	})
}

func onboardingMigrationCategories(catalog []migrate.Category) []onboardingMigrationCategory {
	categories := make([]onboardingMigrationCategory, 0, len(catalog))
	for _, category := range catalog {
		if len(category.Items) == 0 {
			continue
		}
		items := make([]onboardingMigrationItem, len(category.Items))
		for index, item := range category.Items {
			items[index] = onboardingMigrationItem{item: item, selected: item.Selectable}
		}
		categories = append(categories, onboardingMigrationCategory{id: category.ID, items: items})
	}
	return categories
}

func (a *App) chooseOnboardingMigration(id string) {
	if a.onboardingMigration.importing {
		return
	}
	if a.migrationInstallation(id) == nil {
		return
	}
	a.onboardingMigration.selectedID = id
	a.onboardingMigration.choosing = false
	a.onboardingMigration.categories = nil
	a.onboardingMigration.loaded = false
	a.onboardingMigration.done = false
	a.onboardingMigration.error = ""
	a.loadOnboardingMigrationCatalog()
	a.invalidateOnboardingWindow()
}

func (a *App) toggleOnboardingMigrationItem(id string, selected bool) {
	if a.onboardingMigration.importing || a.onboardingMigration.done {
		return
	}
	for categoryIndex := range a.onboardingMigration.categories {
		items := a.onboardingMigration.categories[categoryIndex].items
		for index := range items {
			item := &items[index]
			if item.item.ID == id && item.item.Selectable {
				item.selected = selected
			}
		}
	}
	a.invalidateOnboardingWindow()
}

// retreatOnboardingMigration returns to the launcher picker when several are installed.
func (a *App) retreatOnboardingMigration() bool {
	state := &a.onboardingMigration
	if state.importing || state.done || state.choosing || len(state.installations) < 2 || state.selectedID == "" {
		return false
	}
	state.choosing = true
	state.selectedID = ""
	state.categories = nil
	state.loaded = false
	state.loading = false
	state.error = ""
	a.invalidateOnboardingWindow()
	return true
}

func (a *App) onboardingMigrationCanImport() bool {
	state := &a.onboardingMigration
	if state.done || state.choosing || state.importing || state.loading {
		return false
	}
	for _, category := range state.categories {
		for _, item := range category.items {
			if item.selected && item.item.Selectable && !item.copied {
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
	for _, category := range a.onboardingMigration.categories {
		for _, item := range category.items {
			if item.selected && item.item.Selectable && !item.copied {
				itemIDs = append(itemIDs, item.item.ID)
			}
		}
	}
	a.onboardingMigration.importing = true
	a.onboardingMigration.activeID = ""
	a.onboardingMigration.error = ""
	a.invalidateOnboardingWindow()
	util.Go(a.lifecycleCtx, "import onboarding migration", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		groups, failures := a.prepareOnboardingMigrationSettings(ctx, installation, itemIDs)
		for _, id := range itemIDs {
			a.showOnboardingMigrationItem(id)
			started := time.Now()
			var itemErr string
			if strings.HasPrefix(id, "setting:") {
				itemErr = a.importOnboardingMigrationSetting(ctx, id, groups, failures)
			} else {
				itemErr = a.importOnboardingMigrationPlugin(ctx, installation, id)
			}
			if remain := onboardingMigrationStepDelay - time.Since(started); remain > 0 {
				time.Sleep(remain)
			}
			a.finishOnboardingMigrationItem(id, itemErr)
			// Hold the result icon long enough to paint before the next row becomes active.
			time.Sleep(onboardingMigrationStepDelay)
		}
		_ = a.runOnUI("finish onboarding migration", func() {
			state := &a.onboardingMigration
			state.importing = false
			state.activeID = ""
			state.done = true
			a.invalidateOnboardingWindow()
		})
	})
}

const onboardingMigrationStepDelay = 160 * time.Millisecond

const migrationHotkeyTaken = "hotkey_taken"

type migrationWriteGroup struct {
	writes  []migrate.SettingWrite
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
		var group *migrationWriteGroup
		for _, id := range write.ItemIDs {
			if existing := groups[id]; existing != nil {
				group = existing
				break
			}
		}
		if group == nil {
			group = &migrationWriteGroup{}
		}
		group.writes = append(group.writes, write)
		for _, id := range write.ItemIDs {
			groups[id] = group
		}
	}
	for _, item := range result.Plugins {
		if item.Error != "" {
			failures[item.ID] = item.Error
		}
	}
	return groups, failures
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
	group.err = strings.Join(errs, " ")
	return group.err
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
		a.onboardingMigration.activeID = id
		a.invalidateOnboardingWindow()
	})
}

func (a *App) finishOnboardingMigrationItem(id, itemErr string) {
	_ = a.runOnUI("finish onboarding migration item", func() {
		if a.onboardingMigration.activeID == id {
			a.onboardingMigration.activeID = ""
		}
		for categoryIndex := range a.onboardingMigration.categories {
			for index := range a.onboardingMigration.categories[categoryIndex].items {
				item := &a.onboardingMigration.categories[categoryIndex].items[index]
				if item.item.ID != id {
					continue
				}
				item.err = itemErr
				item.copied = itemErr == ""
			}
		}
		a.invalidateOnboardingWindow()
	})
}

// stageMigratedMainHotkey records the shortcut without registering it with the OS.
func (a *App) stageMigratedMainHotkey(hotkey string) {
	hotkey = strings.TrimSpace(hotkey)
	if hotkey == "" {
		return
	}
	a.onboardingMigration.stagedMainHotkey = hotkey
	a.rememberOnboardingMigrationSetting("MainHotkey", hotkey)
}

// applyStagedMigrationHotkey registers the staged main shortcut once the hotkey page is open.
func (a *App) applyStagedMigrationHotkey() {
	hotkey := strings.TrimSpace(a.onboardingMigration.stagedMainHotkey)
	if hotkey == "" || a.onboardingMigration.mainHotkeyApplyStarted || a.services == nil {
		return
	}
	a.onboardingMigration.mainHotkeyApplyStarted = true
	a.onboardingMigration.mainHotkeyPending = true
	a.invalidateOnboardingWindow()
	util.Go(a.lifecycleCtx, "register migrated main hotkey", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		err := a.services.UpdateGeneralSetting(ctx, a.sessionID, "MainHotkey", hotkey)
		cancel()
		_ = a.runOnUI("apply migrated main hotkey result", func() {
			a.onboardingMigration.mainHotkeyPending = false
			if err != nil && a.generalSettings != nil {
				a.generalSettings.Update(func(data *settingsData) {
					data.MainHotkey = hotkey
					data.MainHotkeyRegistrationFailed = true
					data.MainHotkeyRegistrationError = migrationHotkeyPageMessage(err)
				})
			}
			if a.hotkeySettings != nil {
				if form := a.hotkeySettings.Form(); form != nil {
					form.values["MainHotkey"] = hotkey
				}
			}
			a.invalidateOnboardingWindow()
		})
	})
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
		a.invalidateOnboardingWindow()
	})
}

func (a *App) onboardingMigrationView(imageScale float32, background woxui.Color, labels map[string]string) ([]launcherview.OnboardingMigrationSource, []launcherview.OnboardingMigrationCategory) {
	state := a.onboardingMigration
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
			ID: category.id, Title: a.translate("i18n:onboarding_migrate_category_" + category.id),
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
		title = a.translate("i18n:onboarding_migrate_item_" + item.item.TitleKey)
	}
	detail := item.item.Detail
	if item.item.DetailKey != "" {
		detail = a.translate("i18n:onboarding_migrate_value_" + item.item.DetailKey)
	}
	if item.item.DetailCode == migrate.DetailPythonMissing {
		if detail != "" {
			detail += " "
		}
		detail += labels["migrate.python"]
	}
	reason := item.err
	if reason == migrationHotkeyTaken {
		reason = a.translate("i18n:onboarding_migrate_hotkey_taken")
	}
	status := ""
	switch {
	case item.item.Status == migrate.PluginImported && item.err == "" && !item.copied:
		status = labels["migrate.imported"]
	case item.item.Status == migrate.PluginUnsupported && item.err == "" && !item.copied:
		status = labels["migrate.unsupported"]
	}
	var icon *woxui.Image
	if categoryID == migrate.CategoryPlugins {
		icon = a.imageForSurface(imageFromCommon(item.item.Icon), physicalImageSize(28, imageScale), background)
	}
	return launcherview.OnboardingMigrationItem{
		ID: item.item.ID, Title: title, Detail: detail, Status: status, Error: reason, Icon: icon,
		Selected: item.selected, Selectable: item.item.Selectable, ShowIcon: categoryID == migrate.CategoryPlugins,
		Active: activeID != "" && activeID == item.item.ID && item.err == "" && !item.copied, Succeeded: item.copied,
	}
}

func (a *App) selectedMigrationInstallation() migrate.Installation {
	return a.migrationInstallation(a.onboardingMigration.selectedID)
}

func (a *App) migrationInstallation(id string) migrate.Installation {
	for _, installation := range a.onboardingMigration.installations {
		if installation.ID() == id {
			return installation
		}
	}
	return nil
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
