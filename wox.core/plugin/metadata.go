package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"wox/common"
	"wox/i18n"
	"wox/resource"
	"wox/setting/definition"
	"wox/util"
)

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

	// enable this feature to set the width ratio of the result list and  preview panel
	// by default, the width ratio is 0.5, which means the result list and preview panel have the same width
	// if the width ratio is 0.3, which means the result list takes 30% of the width and the preview panel takes 70% of the width
	MetadataFeatureResultPreviewWidthRatio MetadataFeatureName = "resultPreviewWidthRatio"

	// enable this feature to support MRU (Most Recently Used) functionality
	// plugin must implement OnMRURestore callback to restore results from MRU data
	MetadataFeatureMRU MetadataFeatureName = "mru"

	// enable this feature to display results in a grid layout instead of list
	// useful for plugins that display visual items like emoji, icons, colors, etc.
	// params see MetadataFeatureParamsGridLayout
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
	SettingDefinitions definition.PluginSettingDefinitions

	// I18n holds inline translations for the plugin.
	// Map structure: langCode -> key -> translatedValue
	// Example: {"en_US": {"title": "Hello"}, "zh_CN": {"title": "你好"}}
	I18n map[string]map[string]string

	// Directory is the absolute path to the plugin directory.
	// It is populated during metadata initialization and not read from plugin.json.
	Directory string `json:"-"`

	//for dev plugin
	IsDev              bool   `json:"-"` // plugins loaded from `local plugin directories` which defined in wpm settings
	DevPluginDirectory string `json:"-"` // absolute path to dev plugin directory defined in wpm settings, only available when IsDev is true
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

func (m *Metadata) GetFeatureParamsForDebounce() (MetadataFeatureParamsDebounce, error) {
	for _, feature := range m.Features {
		if strings.EqualFold(feature.Name, MetadataFeatureDebounce) {
			if v, ok := feature.Params["IntervalMs"]; !ok {
				return MetadataFeatureParamsDebounce{}, errors.New("debounce feature does not have intervalMs param")
			} else {
				if seconds, ok := v.(string); ok {
					timeInMilliseconds, convertErr := strconv.Atoi(seconds)
					if convertErr != nil {
						return MetadataFeatureParamsDebounce{}, fmt.Errorf("debounce feature intervalMs param is not a valid number: %s", convertErr.Error())
					}
					return MetadataFeatureParamsDebounce{
						IntervalMs: timeInMilliseconds,
					}, nil
				}
				if milliseconds, ok := v.(int); ok {
					return MetadataFeatureParamsDebounce{
						IntervalMs: milliseconds,
					}, nil
				}
			}
		}
	}

	return MetadataFeatureParamsDebounce{}, errors.New("plugin does not support debounce feature")
}

func (m *Metadata) GetFeatureParamsForResultPreviewWidthRatio() (MetadataFeatureParamsResultPreviewWidthRatio, error) {
	for _, feature := range m.Features {
		if strings.EqualFold(feature.Name, MetadataFeatureResultPreviewWidthRatio) {
			if v, ok := feature.Params["WidthRatio"]; !ok {
				return MetadataFeatureParamsResultPreviewWidthRatio{}, errors.New("resultPreviewWidthRatio feature does not have widthRatio param")
			} else {
				parsed := false
				widthRatio := 0.0

				if parsedWidthRatio, ok := v.(float64); ok {
					widthRatio = parsedWidthRatio
					parsed = true
				}
				if parsedWidthRatioString, ok := v.(string); ok {
					convertedWidthRatio, convertErr := strconv.ParseFloat(parsedWidthRatioString, 64)
					if convertErr != nil {
						return MetadataFeatureParamsResultPreviewWidthRatio{}, fmt.Errorf("resultPreviewWidthRatio feature widthRatio param is not a valid number: %s", convertErr.Error())
					}
					widthRatio = convertedWidthRatio
					parsed = true
				}
				if !parsed {
					return MetadataFeatureParamsResultPreviewWidthRatio{}, fmt.Errorf("resultPreviewWidthRatio feature widthRatio param is not a valid number")
				}

				if widthRatio < 0 || widthRatio > 1 {
					return MetadataFeatureParamsResultPreviewWidthRatio{}, fmt.Errorf("resultPreviewWidthRatio feature widthRatio param is not a valid number: %s", "must be between 0 and 1")
				}

				return MetadataFeatureParamsResultPreviewWidthRatio{
					WidthRatio: widthRatio,
				}, nil
			}
		}
	}

	return MetadataFeatureParamsResultPreviewWidthRatio{}, errors.New("plugin does not support resultPreviewWidthRatio feature")
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
	return MetadataFeatureParamsMRU{}, errors.New("plugin does not support mru feature")
}

func (m *Metadata) GetFeatureParamsForQueryEnv() (MetadataFeatureParamsQueryEnv, error) {
	for _, feature := range m.Features {
		if strings.EqualFold(feature.Name, MetadataFeatureQueryEnv) {
			params := MetadataFeatureParamsQueryEnv{
				RequireActiveWindowName: false,
				RequireActiveWindowPid:  false,
				RequireActiveWindowIcon: false,
				RequireActiveBrowserUrl: false,
			}

			if v, ok := feature.Params["requireActiveWindowName"]; ok {
				if vString, ok := v.(string); ok {
					if vString == "true" {
						params.RequireActiveWindowName = true
					}
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

	return MetadataFeatureParamsQueryEnv{}, errors.New("plugin does not support queryEnv feature")
}

type MetadataFeature struct {
	Name   MetadataFeatureName
	Params map[string]any
}

type MetadataCommand struct {
	Command     string
	Description common.I18nString // support i18n: prefix
}

type MetadataFeatureParamsDebounce struct {
	IntervalMs int
}

type MetadataFeatureParamsQueryEnv struct {
	RequireActiveWindowName bool
	RequireActiveWindowPid  bool
	RequireActiveWindowIcon bool
	RequireActiveBrowserUrl bool
}

type MetadataFeatureParamsResultPreviewWidthRatio struct {
	WidthRatio float64 // [0-1]
}

// MetadataFeatureParamsGridLayout contains parameters for grid layout feature
// Commands behavior:
//   - Empty: grid enabled for all commands
//   - "!cmd1,cmd2": exclusion mode - grid enabled for all except cmd1,cmd2 (commands starting with ! are excluded)
//   - "cmd1,cmd2": inclusion mode - grid enabled only for cmd1,cmd2
type MetadataFeatureParamsGridLayout struct {
	Columns     int      // number of columns per row, default 8
	ShowTitle   bool     // whether to show title below icon, default false
	ItemPadding int      // padding inside each item, default 12
	ItemMargin  int      // margin outside each item (all sides), default 6
	Commands    []string // commands to enable grid layout for, empty means all commands
}

func (m *Metadata) GetFeatureParamsForGridLayout() (MetadataFeatureParamsGridLayout, error) {
	for _, feature := range m.Features {
		if strings.EqualFold(feature.Name, MetadataFeatureGridLayout) {
			params := MetadataFeatureParamsGridLayout{
				Columns:     8,
				ShowTitle:   false,
				ItemPadding: 12,
				ItemMargin:  6,
				Commands:    []string{},
			}

			if v, ok := feature.Params["Columns"]; ok {
				if columnsString, ok := v.(string); ok {
					if columns, err := strconv.Atoi(columnsString); err == nil {
						params.Columns = columns
					} else {
						return MetadataFeatureParamsGridLayout{}, fmt.Errorf("gridLayout feature Columns param is not a valid number: %s", err.Error())
					}
				}
				if columnInt, ok := v.(int); ok {
					params.Columns = columnInt
				}
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
				if vInt, ok := v.(int); ok {
					params.ItemPadding = vInt
				}
				if vString, ok := v.(string); ok {
					if padding, err := strconv.Atoi(vString); err == nil {
						params.ItemPadding = padding
					} else {
						return MetadataFeatureParamsGridLayout{}, fmt.Errorf("gridLayout feature ItemPadding param is not a valid number: %s", err.Error())
					}
				}
			}

			if v, ok := feature.Params["ItemMargin"]; ok {
				if vString, ok := v.(string); ok {
					if margin, err := strconv.Atoi(vString); err == nil {
						params.ItemMargin = margin
					} else {
						return MetadataFeatureParamsGridLayout{}, fmt.Errorf("gridLayout feature ItemMargin param is not a valid number: %s", err.Error())
					}
				}
				if vInt, ok := v.(int); ok {
					params.ItemMargin = vInt
				}
			}

			if v, ok := feature.Params["Commands"]; ok {
				if vString, ok := v.(string); ok {
					if vString != "" {
						params.Commands = strings.Split(vString, ",")
						for i := range params.Commands {
							params.Commands[i] = strings.TrimSpace(params.Commands[i])
						}
					}
				}
				if vArray, ok := v.([]any); ok {
					for _, item := range vArray {
						if itemString, ok := item.(string); ok {
							params.Commands = append(params.Commands, itemString)
						}
					}
				}
			}

			return params, nil
		}
	}

	return MetadataFeatureParamsGridLayout{}, errors.New("plugin does not support gridLayout feature")
}

func (m *Metadata) GetName(ctx context.Context) string {
	return m.translate(ctx, m.Name)
}

func (m *Metadata) GetDescription(ctx context.Context) string {
	return m.translate(ctx, m.Description)
}

func (m *Metadata) translate(ctx context.Context, text common.I18nString) string {
	rawText := strings.TrimSpace(string(text))
	if translated := i18n.GetI18nManager().TranslateI18nMap(ctx, rawText, m.I18n); translated != rawText {
		return translated
	}

	return i18n.GetI18nManager().TranslateWox(ctx, rawText)
}

// LoadSystemI18nFromDirectory merges translations from Wox's central lang files into Metadata.I18n.
func (m *Metadata) LoadSystemI18nFromDirectory(ctx context.Context) {
	// System plugins share Wox's central lang files instead of a per-plugin lang folder.
	for _, lang := range i18n.GetSupportedLanguages() {
		langJson, err := resource.GetLangJson(ctx, string(lang.Code))
		if err != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("failed to read wox lang %s: %s", lang.Code, err.Error()))
			continue
		}

		translations, parseErr := flattenI18nJSON(langJson)
		if parseErr != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("failed to parse wox lang %s: %s", lang.Code, parseErr.Error()))
			continue
		}

		m.mergeI18n(string(lang.Code), translations)
	}
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
		m.I18n[langCode] = map[string]string{}
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
				nextPrefix := fmt.Sprintf("%s.%d", prefix, idx)
				walk(nextPrefix, child)
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
