package plugin

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"wox/setting"
)

type MetadataFeatureName = string

const (
	// enable query file feature
	// user may drag multiple files into Wox, and Wox will pass these files to plugin
	// plugin need to handle Query.Type == "file" in query
	// params see MetadataFeatureParamsQueryFile
	MetadataFeatureNameQueryFile MetadataFeatureName = "queryFile"

	// enable this feature to let Wox debounce queries between user input
	// params see MetadataFeatureParamsDebounce
	MetadataFeatureDebounce MetadataFeatureName = "debounce"

	// enable this feature to let Wox don't auto score results
	// by default, Wox will auto score results by the frequency of their actioned times
	MetadataFeatureNameIgnoreAutoScore MetadataFeatureName = "ignoreAutoScore"
)

// Metadata parsed from plugin.json, see `Plugin.json.md` for more detail
// All properties are immutable after initialization
type Metadata struct {
	Id              string
	Name            string
	Author          string
	Version         string
	MinWoxVersion   string
	Runtime         string
	Description     string
	Icon            string
	Website         string
	Entry           string
	TriggerKeywords []string //User can add/update/delete trigger keywords
	Commands        []MetadataCommand
	SupportedOS     []string
	Features        []MetadataFeature
	Settings        setting.CustomizedPluginSettings
}

func (m *Metadata) IsSupportFeature(f MetadataFeatureName) bool {
	for _, feature := range m.Features {
		if strings.ToLower(feature.Name) == strings.ToLower(f) {
			return true
		}
	}
	return false
}

func (m *Metadata) GetFeatureParamsForQueryFile() (MetadataFeatureParamsQueryFile, error) {
	for _, feature := range m.Features {
		if strings.ToLower(feature.Name) == strings.ToLower(MetadataFeatureNameQueryFile) {
			if v, ok := feature.Params["extensions"]; !ok {
				return MetadataFeatureParamsQueryFile{}, errors.New("queryFile feature does not have extensions param")
			} else {
				return MetadataFeatureParamsQueryFile{
					FileExtensions: strings.Split(v, ","),
				}, nil
			}
		}
	}

	return MetadataFeatureParamsQueryFile{}, errors.New("plugin does not support queryFile feature")
}

func (m *Metadata) GetFeatureParamsForDebounce() (MetadataFeatureParamsDebounce, error) {
	for _, feature := range m.Features {
		if strings.ToLower(feature.Name) == strings.ToLower(MetadataFeatureDebounce) {
			if v, ok := feature.Params["intervalMs"]; !ok {
				return MetadataFeatureParamsDebounce{}, errors.New("debounce feature does not have intervalMs param")
			} else {
				timeInMilliseconds, convertErr := strconv.Atoi(v)
				if convertErr != nil {
					return MetadataFeatureParamsDebounce{}, fmt.Errorf("debounce feature intervalMs param is not a valid number: %s", convertErr.Error())
				}

				return MetadataFeatureParamsDebounce{
					intervalMs: timeInMilliseconds,
				}, nil
			}
		}
	}

	return MetadataFeatureParamsDebounce{}, errors.New("plugin does not support debounce feature")
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
	Directory string
}

type MetadataFeatureParamsQueryFile struct {
	FileExtensions []string
}

type MetadataFeatureParamsDebounce struct {
	intervalMs int
}
