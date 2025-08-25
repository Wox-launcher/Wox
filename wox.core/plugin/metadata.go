package plugin

import (
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"
	"wox/common"
	"wox/setting/definition"
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
)

// Metadata parsed from plugin.json, see `Plugin.json.md` for more detail
// All properties are immutable after initialization
type Metadata struct {
	Id                 string
	Name               string
	Author             string
	Version            string
	MinWoxVersion      string
	Runtime            string
	Description        string
	Icon               string // should be WoxImage.String()
	Website            string
	Entry              string
	TriggerKeywords    []string //User can add/update/delete trigger keywords
	Commands           []MetadataCommand
	SupportedOS        []string
	Features           []MetadataFeature
	SettingDefinitions definition.PluginSettingDefinitions
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
				timeInMilliseconds, convertErr := strconv.Atoi(v)
				if convertErr != nil {
					return MetadataFeatureParamsDebounce{}, fmt.Errorf("debounce feature intervalMs param is not a valid number: %s", convertErr.Error())
				}

				return MetadataFeatureParamsDebounce{
					IntervalMs: timeInMilliseconds,
				}, nil
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
				widthRatio, convertErr := strconv.ParseFloat(v, 64)
				if convertErr != nil {
					return MetadataFeatureParamsResultPreviewWidthRatio{}, fmt.Errorf("resultPreviewWidthRatio feature widthRatio param is not a valid number: %s", convertErr.Error())
				}
				if widthRatio < 0 || widthRatio > 1 {
					return MetadataFeatureParamsResultPreviewWidthRatio{}, fmt.Errorf("resultPreviewWidthRatio feature widthRatio param is not a valid number: %s", convertErr.Error())
				}

				return MetadataFeatureParamsResultPreviewWidthRatio{
					WidthRatio: widthRatio,
				}, nil
			}
		}
	}

	return MetadataFeatureParamsResultPreviewWidthRatio{}, errors.New("plugin does not support resultPreviewWidthRatio feature")
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
				if v == "true" {
					params.RequireActiveWindowName = true
				}
			}

			if v, ok := feature.Params["requireActiveWindowPid"]; ok {
				if v == "true" {
					params.RequireActiveWindowPid = true
				}
			}

			if v, ok := feature.Params["requireActiveWindowIcon"]; ok {
				if v == "true" {
					params.RequireActiveWindowIcon = true
				}
			}

			if v, ok := feature.Params["requireActiveBrowserUrl"]; ok {
				if v == "true" {
					params.RequireActiveBrowserUrl = true
				}
			}

			return params, nil
		}
	}

	return MetadataFeatureParamsQueryEnv{}, errors.New("plugin does not support queryEnv feature")
}

type MetadataFeature struct {
	Name   MetadataFeatureName
	Params map[string]string
}

type MetadataCommand struct {
	Command     string
	Description string
}

type MetadataWithDirectory struct {
	Metadata  Metadata
	Directory string // absolute path to plugin directory

	//for dev plugin
	IsDev              bool   // plugins loaded from `local plugin directories` which defined in wpm settings
	DevPluginDirectory string // absolute path to dev plugin directory defined in wpm settings, only available when IsDev is true
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
