package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"wox/common"
	"wox/i18n"
	"wox/setting/definition"
	"wox/setting/validator"
	"wox/util"
)

var ErrFeatureNotSupported = errors.New("Plugin does not support this feature")

type MetadataFeatureName = string

const (
	// enable this to handle QueryTypeSelection, by default Wox will only pass QueryTypeInput to plugin
	MetadataFeatureQuerySelection MetadataFeatureName = "querySelection"

	// enable this feature to let Wox debounce queries between user input
	// params see MetadataFeatureParamsDebounce
	MetadataFeatureDebounce MetadataFeatureName = "debounce"

	// enable this feature to let Wox don't auto score results
	// by default, Wox will auto score results by the frequency of their actioned times
	MetadataFeatureIgnoreAutoScore MetadataFeatureName = "ignoreAutoScore"

	// enable this feature to get query env in plugin
	MetadataFeatureQueryEnv MetadataFeatureName = "queryEnv"

	// enable this feature to chat with ai in plugin
	MetadataFeatureAI MetadataFeatureName = "ai"

	// enable this feature to execute custom deep link in plugin
	MetadataFeatureDeepLink MetadataFeatureName = "deepLink"

	// enable this feature to set the width ratio of the result list and preview panel
	// Deprecated: return QueryResponse.Layout.ResultPreviewWidthRatio instead. Metadata
	// can only express static plugin or command defaults, while QueryResponse lets each
	// query return the layout that matches its current results.
	MetadataFeatureResultPreviewWidthRatio MetadataFeatureName = "resultPreviewWidthRatio"

	// enable this feature to support MRU (Most Recently Used) functionality
	// plugin must implement OnMRURestore callback to restore results from MRU data
	MetadataFeatureMRU MetadataFeatureName = "mru"

	// enable this feature to display results in a grid layout instead of list
	// Deprecated: return QueryResponse.Layout.GridLayout instead. Metadata is kept for
	// existing plugins, but query-scoped layout is more flexible when only some result
	// sets should use a grid.
	MetadataFeatureGridLayout MetadataFeatureName = "gridLayout"
)

// Metadata parsed from plugin.json, see `Plugin.json.md` for more detail
// All properties are immutable after initialization
type Metadata struct {
	Id                 string
	Name               common.I18nString // support i18n: prefix, so don't use "name" directly
	Author             string
	Version            string
	MinWoxVersion      string
	Runtime            string
	Description        common.I18nString // support i18n: prefix, so don't use "description" directly
	Icon               string            // should be WoxImage.String()
	Website            string
	Entry              string
	TriggerKeywords    []string //User can add/update/delete trigger keywords
	Commands           []MetadataCommand
	SupportedOS        []string
	Features           []MetadataFeature
	Glances            []MetadataGlance
	SettingDefinitions definition.PluginSettingDefinitions
	QueryRequirements  MetadataQueryRequirements

	// I18n holds plugin-local translations.
	// Wox central translations stay in the i18n manager so system plugins do not
	// duplicate the same flattened language maps in every Metadata instance.
	// Map structure: langCode -> key -> translatedValue
	// Example: {"en_US": {"title": "Hello"}, "zh_CN": {"title": "你好"}}
	I18n map[string]map[string]string

	// Directory is the absolute path to the plugin directory.
	// It is populated during metadata initialization and not read from plugin.json.
	Directory string `json:"-"`

	//for dev plugin
	IsDev              bool   `json:"-"` // plugins loaded from `local plugin directories` which defined in wpm settings
	DevPluginDirectory string `json:"-"` // absolute path to dev plugin directory defined in wpm settings, only available when IsDev is true

	// cache for translations
	translateCache     *util.HashMap[string, string] `json:"-"`
	translateCacheOnce sync.Once                     `json:"-"`
}

func (m *Metadata) GetIconOrDefault(pluginDirectory string, defaultImage common.WoxImage) common.WoxImage {
	image := common.ParseWoxImageOrDefault(m.Icon, defaultImage)
	if image.ImageType == common.WoxImageTypeRelativePath {
		image.ImageData = path.Join(pluginDirectory, image.ImageData)
		image.ImageType = common.WoxImageTypeAbsolutePath
	}
	return image
}

func (m *Metadata) IsSupportFeature(f MetadataFeatureName) bool {
	for _, feature := range m.Features {
		if strings.EqualFold(feature.Name, f) {
			return true
		}
	}
	return false
}

func parseFeatureIntParam(value any) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int32:
		return int(v), nil
	case int64:
		return int(v), nil
	case float64:
		if math.Trunc(v) != v {
			return 0, fmt.Errorf("must be an integer")
		}
		return int(v), nil
	case string:
		parsed, err := strconv.Atoi(v)
		if err != nil {
			return 0, err
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unsupported type %T", value)
	}
}

func parseFeatureFloatParam(value any) (float64, error) {
	switch v := value.(type) {
	case int:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case float64:
		return v, nil
	case string:
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, err
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unsupported type %T", value)
	}
}

func parseFeatureStringListParam(value any) []string {
	items := []string{}
	if vString, ok := value.(string); ok {
		if vString != "" {
			items = strings.Split(vString, ",")
		}
	}
	if vArray, ok := value.([]any); ok {
		for _, item := range vArray {
			if itemString, ok := item.(string); ok {
				items = append(items, itemString)
			}
		}
	}
	if vArray, ok := value.([]string); ok {
		// Built-in Go plugins pass feature params directly instead of through JSON decoding.
		// Accepting []string keeps command-scoped features equivalent for built-in and external plugins.
		items = append(items, vArray...)
	}

	for i := range items {
		items[i] = strings.TrimSpace(items[i])
	}
	return items
}

func isFeatureEnabledForCommand(commands []string, command string) bool {
	if len(commands) == 0 {
		return true
	}

	// A leading ! keeps the existing exclusion-mode contract while centralizing
	// command matching for metadata features that can be scoped by query command.
	if strings.HasPrefix(commands[0], "!") {
		for _, cmd := range commands {
			if strings.TrimPrefix(cmd, "!") == command {
				return false
			}
		}
		return true
	}

	for _, cmd := range commands {
		if cmd == command {
			return true
		}
	}
	return false
}

func (m *Metadata) GetFeatureParamsForDebounce() (MetadataFeatureParamsDebounce, error) {
	for _, feature := range m.Features {
		if strings.EqualFold(feature.Name, MetadataFeatureDebounce) {
			if v, ok := feature.Params["IntervalMs"]; !ok {
				return MetadataFeatureParamsDebounce{}, errors.New("debounce feature does not have IntervalMs param")
			} else {
				intervalMs, err := parseFeatureIntParam(v)
				if err != nil {
					return MetadataFeatureParamsDebounce{}, fmt.Errorf("debounce feature IntervalMs param is not a valid number: %s", err.Error())
				}
				return MetadataFeatureParamsDebounce{
					IntervalMs: intervalMs,
				}, nil
			}
		}
	}

	return MetadataFeatureParamsDebounce{}, ErrFeatureNotSupported
}

func (m *Metadata) GetFeatureParamsForResultPreviewWidthRatio() (MetadataFeatureParamsResultPreviewWidthRatio, error) {
	for _, feature := range m.Features {
		if strings.EqualFold(feature.Name, MetadataFeatureResultPreviewWidthRatio) {
			if v, ok := feature.Params["WidthRatio"]; !ok {
				return MetadataFeatureParamsResultPreviewWidthRatio{}, errors.New("resultPreviewWidthRatio feature does not have widthRatio param")
			} else {
				widthRatio, err := parseFeatureFloatParam(v)
				if err != nil {
					return MetadataFeatureParamsResultPreviewWidthRatio{}, fmt.Errorf("resultPreviewWidthRatio feature widthRatio param is not a valid number: %s", err.Error())
				}

				if widthRatio < 0 || widthRatio > 1 {
					return MetadataFeatureParamsResultPreviewWidthRatio{}, fmt.Errorf("resultPreviewWidthRatio feature widthRatio param is not a valid number: %s", "must be between 0 and 1")
				}

				commands := []string{}
				if commandValue, ok := feature.Params["Commands"]; ok {
					// This feature used to apply only at plugin scope. Commands gives plugin
					// authors the same command-scoped control as gridLayout, so a preview-only
					// command can use WidthRatio 0 without changing the plugin's normal queries.
					commands = parseFeatureStringListParam(commandValue)
				}

				return MetadataFeatureParamsResultPreviewWidthRatio{
					WidthRatio: widthRatio,
					Commands:   commands,
				}, nil
			}
		}
	}

	return MetadataFeatureParamsResultPreviewWidthRatio{}, ErrFeatureNotSupported
}

func (m *Metadata) GetFeatureParamsForResultPreviewWidthRatioCommand(command string) (MetadataFeatureParamsResultPreviewWidthRatio, bool, error) {
	params, err := m.GetFeatureParamsForResultPreviewWidthRatio()
	if err != nil {
		return MetadataFeatureParamsResultPreviewWidthRatio{}, false, err
	}

	return params, params.IsEnabledForCommand(command), nil
}

type MetadataFeatureParamsMRU struct {
	// HashBy controls how MRU identity hash is calculated for this plugin.
	// Supported values:
	//   - "title"    (default): use result Title + SubTitle (backward compatible)
	//   - "rawQuery": use original Query.RawQuery as identity
	//   - "search":  use Query.Search as identity
	HashBy string
}

func (m *Metadata) GetFeatureParamsForMRU() (MetadataFeatureParamsMRU, error) {
	for _, feature := range m.Features {
		if strings.EqualFold(feature.Name, MetadataFeatureMRU) {
			params := MetadataFeatureParamsMRU{HashBy: "title"}
			if v, ok := feature.Params["HashBy"]; ok && v != "" {
				if hashby, ok := v.(string); ok {
					params.HashBy = strings.ToLower(hashby)
				}
			}
			return params, nil
		}
	}
	return MetadataFeatureParamsMRU{}, ErrFeatureNotSupported
}

func (m *Metadata) GetFeatureParamsForQueryEnv() (MetadataFeatureParamsQueryEnv, error) {
	for _, feature := range m.Features {
		if strings.EqualFold(feature.Name, MetadataFeatureQueryEnv) {
			params := MetadataFeatureParamsQueryEnv{
				RequireActiveWindowName:             false,
				RequireActiveWindowPid:              false,
				RequireActiveWindowId:               false,
				RequireActiveWindowIcon:             false,
				RequireActiveWindowIsOpenSaveDialog: false,
				RequireActiveBrowserUrl:             false,
			}

			if v, ok := feature.Params["requireActiveWindowName"]; ok {
				if vString, ok := v.(string); ok {
					if vString == "true" {
						params.RequireActiveWindowName = true
					}
				}
				if vBool, ok := v.(bool); ok {
					params.RequireActiveWindowName = vBool
				}
			}

			if v, ok := feature.Params["requireActiveWindowPid"]; ok {
				if vString, ok := v.(string); ok {
					if vString == "true" {
						params.RequireActiveWindowPid = true
					}
				}
				if vBool, ok := v.(bool); ok {
					params.RequireActiveWindowPid = vBool
				}
			}

			if v, ok := feature.Params["requireActiveWindowId"]; ok {
				if vString, ok := v.(string); ok {
					if vString == "true" {
						params.RequireActiveWindowId = true
					}
				}
				if vBool, ok := v.(bool); ok {
					params.RequireActiveWindowId = vBool
				}
			}

			if v, ok := feature.Params["requireActiveWindowIcon"]; ok {
				if vString, ok := v.(string); ok {
					if vString == "true" {
						params.RequireActiveWindowIcon = true
					}
				}
				if vBool, ok := v.(bool); ok {
					params.RequireActiveWindowIcon = vBool
				}
			}

			if v, ok := feature.Params["requireActiveWindowIsOpenSaveDialog"]; ok {
				if vString, ok := v.(string); ok {
					if vString == "true" {
						params.RequireActiveWindowIsOpenSaveDialog = true
					}
				}
				if vBool, ok := v.(bool); ok {
					params.RequireActiveWindowIsOpenSaveDialog = vBool
				}
			}

			if v, ok := feature.Params["requireActiveBrowserUrl"]; ok {
				if vString, ok := v.(string); ok {
					if vString == "true" {
						params.RequireActiveBrowserUrl = true
					}
				}
				if vBool, ok := v.(bool); ok {
					params.RequireActiveBrowserUrl = vBool
				}
			}

			return params, nil
		}
	}

	return MetadataFeatureParamsQueryEnv{}, ErrFeatureNotSupported
}

type MetadataFeature struct {
	Name   MetadataFeatureName
	Params map[string]any
}

type MetadataCommand struct {
	Command     string
	Description common.I18nString // support i18n: prefix
}

// MetadataGlance declares one plugin-local Global Glance candidate. The UI stores
// selections as PluginId + Id, so only the Id needs to be unique within this
// metadata object.
type MetadataGlance struct {
	Id                string
	Name              common.I18nString
	Description       common.I18nString
	Icon              string
	RefreshIntervalMs int
}

func (m *Metadata) ValidateGlances() error {
	seen := map[string]bool{}
	for _, glance := range m.Glances {
		id := strings.TrimSpace(glance.Id)
		if id == "" {
			return fmt.Errorf("glance id is empty")
		}
		if seen[id] {
			return fmt.Errorf("duplicate glance id: %s", id)
		}
		seen[id] = true
	}
	return nil
}

func (m *Metadata) HasGlance(id string) bool {
	for _, glance := range m.Glances {
		if glance.Id == id {
			return true
		}
	}
	return false
}

// MetadataQueryRequirement declares a setting that must pass validation before
// a plugin query is allowed to run. This is separate from setting definitions
// because normal setting validators describe form validity, while query
// requirements describe runtime readiness for a specific query scope.
type MetadataQueryRequirement struct {
	SettingKey string
	Validators []validator.PluginSettingValidator
	Message    common.I18nString
}

// MetadataQueryRequirements groups query prerequisites by the exact query scope
// that activates them. The explicit names avoid overloading Commands and make
// the no-command query case readable in plugin.json.
type MetadataQueryRequirements struct {
	AnyQuery            []MetadataQueryRequirement
	QueryWithoutCommand []MetadataQueryRequirement
	QueryWithCommand    map[string][]MetadataQueryRequirement
}

func (m MetadataQueryRequirements) GetRequirementsForQuery(query Query) []MetadataQueryRequirement {
	requirements := append([]MetadataQueryRequirement{}, m.AnyQuery...)
	if query.Command == "" {
		requirements = append(requirements, m.QueryWithoutCommand...)
		return requirements
	}

	if commandRequirements, ok := m.QueryWithCommand[query.Command]; ok {
		requirements = append(requirements, commandRequirements...)
	}
	return requirements
}

type MetadataFeatureParamsDebounce struct {
	IntervalMs int
}

type MetadataFeatureParamsQueryEnv struct {
	RequireActiveWindowName             bool
	RequireActiveWindowPid              bool
	RequireActiveWindowId               bool
	RequireActiveWindowIcon             bool
	RequireActiveWindowIsOpenSaveDialog bool
	RequireActiveBrowserUrl             bool
}

// MetadataFeatureParamsResultPreviewWidthRatio keeps compatibility with the
// deprecated metadata feature. New plugins should return QueryResponse.Layout
// so preview width can follow the current query instead of a static default.
type MetadataFeatureParamsResultPreviewWidthRatio struct {
	WidthRatio float64 // [0-1]
	Commands   []string
}

func (p MetadataFeatureParamsResultPreviewWidthRatio) IsEnabledForCommand(command string) bool {
	return isFeatureEnabledForCommand(p.Commands, command)
}

// MetadataFeatureParamsGridLayout keeps compatibility with the deprecated
// metadata feature. New plugins should return QueryResponse.Layout.GridLayout
// so grid presentation can follow the current query instead of a static default.
// Commands behavior:
//   - Empty: grid enabled for all commands
//   - "!cmd1,cmd2": exclusion mode - grid enabled for all except cmd1,cmd2 (commands starting with ! are excluded)
//   - "cmd1,cmd2": inclusion mode - grid enabled only for cmd1,cmd2
type MetadataFeatureParamsGridLayout struct {
	Columns     int      // number of columns per row, default 8
	ShowTitle   bool     // whether to show title below icon, default false
	ItemPadding int      // padding inside each item, default 0
	ItemMargin  int      // margin outside each item (all sides), default 6
	AspectRatio float64  // width / height for each grid visual item, default 1.0
	Commands    []string // commands to enable grid layout for, empty means all commands
}

func (m *Metadata) GetFeatureParamsForGridLayoutCommand(command string) (MetadataFeatureParamsGridLayout, bool, error) {
	params, err := m.GetFeatureParamsForGridLayout()
	if err != nil {
		return MetadataFeatureParamsGridLayout{}, false, err
	}

	return params, params.IsEnabledForCommand(command), nil
}

func (p MetadataFeatureParamsGridLayout) IsEnabledForCommand(command string) bool {
	return isFeatureEnabledForCommand(p.Commands, command)
}

func (m *Metadata) GetFeatureParamsForGridLayout() (MetadataFeatureParamsGridLayout, error) {
	for _, feature := range m.Features {
		if strings.EqualFold(feature.Name, MetadataFeatureGridLayout) {
			params := MetadataFeatureParamsGridLayout{
				Columns:     8,
				ShowTitle:   false,
				ItemPadding: 0,
				ItemMargin:  6,
				AspectRatio: 1.0,
				Commands:    []string{},
			}

			if v, ok := feature.Params["Columns"]; ok {
				columns, err := parseFeatureIntParam(v)
				if err != nil {
					return MetadataFeatureParamsGridLayout{}, fmt.Errorf("gridLayout feature Columns param is not a valid number: %s", err.Error())
				}
				params.Columns = columns
			}

			if v, ok := feature.Params["ShowTitle"]; ok {
				if vString, ok := v.(string); ok {
					params.ShowTitle = vString == "true"
				}
				if vBool, ok := v.(bool); ok {
					params.ShowTitle = vBool
				}
			}

			if v, ok := feature.Params["ItemPadding"]; ok {
				padding, err := parseFeatureIntParam(v)
				if err != nil {
					return MetadataFeatureParamsGridLayout{}, fmt.Errorf("gridLayout feature ItemPadding param is not a valid number: %s", err.Error())
				}
				params.ItemPadding = padding
			}

			if v, ok := feature.Params["ItemMargin"]; ok {
				margin, err := parseFeatureIntParam(v)
				if err != nil {
					return MetadataFeatureParamsGridLayout{}, fmt.Errorf("gridLayout feature ItemMargin param is not a valid number: %s", err.Error())
				}
				params.ItemMargin = margin
			}

			if v, ok := feature.Params["AspectRatio"]; ok {
				aspectRatio, err := parseFeatureFloatParam(v)
				if err != nil {
					return MetadataFeatureParamsGridLayout{}, fmt.Errorf("gridLayout feature AspectRatio param is not a valid number: %s", err.Error())
				}
				if aspectRatio <= 0 {
					return MetadataFeatureParamsGridLayout{}, fmt.Errorf("gridLayout feature AspectRatio param is not a valid number: must be greater than 0")
				}
				params.AspectRatio = aspectRatio
			}

			if v, ok := feature.Params["Commands"]; ok {
				params.Commands = parseFeatureStringListParam(v)
			}

			return params, nil
		}
	}

	return MetadataFeatureParamsGridLayout{}, ErrFeatureNotSupported
}

func (m *Metadata) GetName(ctx context.Context) string {
	return m.translate(ctx, m.Name)
}

func (m *Metadata) GetDescription(ctx context.Context) string {
	return m.translate(ctx, m.Description)
}

func (m *Metadata) GetNameEn(ctx context.Context) string {
	return m.translateEn(ctx, m.Name)
}

func (m *Metadata) GetDescriptionEn(ctx context.Context) string {
	return m.translateEn(ctx, m.Description)
}

func (m *Metadata) translateEn(ctx context.Context, text common.I18nString) string {
	rawText := strings.TrimSpace(string(text))

	key := strings.TrimPrefix(rawText, "i18n:")
	if m.I18n != nil {
		if enMap, ok := m.I18n["en_US"]; ok {
			if val, ok := enMap[key]; ok {
				return val
			}
		}
	}

	return i18n.GetI18nManager().TranslateWoxEnUs(ctx, rawText)
}

func (m *Metadata) translate(ctx context.Context, text common.I18nString) string {
	rawText := strings.TrimSpace(string(text))
	m.translateCacheOnce.Do(func() {
		m.translateCache = util.NewHashMap[string, string]()
	})
	langCode := i18n.GetI18nManager().GetCurrentLangCode()
	cacheKey := string(langCode) + "|" + rawText
	if cached, ok := m.translateCache.Load(cacheKey); ok {
		return cached
	}

	if translated := i18n.GetI18nManager().TranslateI18nMap(ctx, rawText, m.I18n); translated != rawText {
		m.translateCache.Store(cacheKey, translated)
		return translated
	}

	translated := i18n.GetI18nManager().TranslateWox(ctx, rawText)
	m.translateCache.Store(cacheKey, translated)
	return translated
}

// LoadPluginI18nFromDirectory merges translations from lang files into Metadata.I18n.
// Supported files: lang/<langCode>.json where langCode is one of supported languages.
func (m *Metadata) LoadPluginI18nFromDirectory(ctx context.Context) {
	langDir := path.Join(m.Directory, "lang")
	entries, err := os.ReadDir(langDir)
	if err != nil {
		return
	}

	supportedLangs := make(map[string]struct{})
	for _, lang := range i18n.GetSupportedLanguages() {
		supportedLangs[string(lang.Code)] = struct{}{}
	}

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		langCode := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		if _, ok := supportedLangs[langCode]; !ok {
			continue
		}

		content, readErr := os.ReadFile(path.Join(langDir, entry.Name()))
		if readErr != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("failed to read lang file %s: %s", entry.Name(), readErr.Error()))
			continue
		}

		translations, parseErr := flattenI18nJSON(content)
		if parseErr != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("failed to parse lang file %s: %s", entry.Name(), parseErr.Error()))
			continue
		}

		m.mergeI18n(langCode, translations)
	}
}

func (m *Metadata) mergeI18n(langCode string, translations map[string]string) {
	if translations == nil {
		return
	}
	if m.I18n == nil {
		m.I18n = map[string]map[string]string{}
	}
	if _, ok := m.I18n[langCode]; !ok {
		m.I18n[langCode] = translations
		return
	}

	for k, v := range translations {
		m.I18n[langCode][k] = v
	}
}

func flattenI18nJSON(content []byte) (map[string]string, error) {
	var data any
	if err := json.Unmarshal(content, &data); err != nil {
		return nil, err
	}

	flattened := map[string]string{}
	var walk func(prefix string, value any)
	walk = func(prefix string, value any) {
		switch v := value.(type) {
		case map[string]any:
			for key, child := range v {
				nextPrefix := key
				if prefix != "" {
					nextPrefix = prefix + "." + key
				}
				walk(nextPrefix, child)
			}
		case []any:
			for idx, child := range v {
				nextPrefix := prefix + "." + strconv.Itoa(idx)
				walk(nextPrefix, child)
			}
		case string:
			if prefix != "" {
				flattened[prefix] = v
			}
		default:
			if prefix != "" {
				flattened[prefix] = fmt.Sprint(v)
			}
		}
	}

	walk("", data)
	return flattened, nil
}
