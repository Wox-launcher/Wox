package plugin

import (
	"errors"
	"strings"
)

type MetadataFeatureName = string

const (
	MetadataFeatureNamePreview   MetadataFeatureName = "preview"
	MetadataFeatureNameQueryFile MetadataFeatureName = "queryFile"
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
