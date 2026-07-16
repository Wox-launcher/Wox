package definition

import (
	"context"
)

// PluginSettingValueDictationModel is a Wox-internal setting component for
// the dictation plugin's model manager. It renders as a dropdown of
// recommended models, each shown with a download status (not-downloaded,
// downloading XX%, downloaded). Selecting a not-downloaded model triggers
// a download with live progress; selecting a downloaded model switches the
// active model. The backend reports download status via the Options field.
type PluginSettingValueDictationModel struct {
	Key          string
	Label        string
	Tooltip      string
	DefaultValue string
	// Options lists all available model choices, including their download
	// status. The UI side uses Status to render the appropriate UI
	// (greyed-out + download button for not-downloaded, progress bar for
	// downloading, selectable for downloaded).
	Options []DictationModelOption

	Style PluginSettingValueStyle `json:"-"` // Deprecated: ignored on load so Wox keeps setting layouts consistent.
}

// DictationModelStatus describes the download state of a model.
type DictationModelStatus string

const (
	DictationModelStatusNotDownloaded DictationModelStatus = "not_downloaded"
	DictationModelStatusDownloading   DictationModelStatus = "downloading"
	DictationModelStatusExtracting    DictationModelStatus = "extracting"
	DictationModelStatusFinalizing    DictationModelStatus = "finalizing"
	DictationModelStatusDownloaded    DictationModelStatus = "downloaded"
	DictationModelStatusFailed        DictationModelStatus = "failed"
)

// DictationModelOption represents one entry in the model dropdown.
type DictationModelOption struct {
	// ID is the unique model identifier, used as the option value.
	ID string
	// DisplayName is the short user-facing label for this model.
	DisplayName string
	// Description is a detailed description shown in the dropdown, including
	// architecture, accuracy notes, and memory characteristics.
	Description string
	// Languages is a human-readable list of supported languages.
	Languages string
	// Recommended indicates whether this model is recommended for most users.
	Recommended bool
	// Status reports the current download state.
	Status DictationModelStatus
	// DownloadProgress is 0-100 when Status is "downloading", 0 otherwise.
	DownloadProgress int
	// SizeMB is the approximate download size in megabytes.
	SizeMB int
	// Error holds a human-readable error message when Status is "failed".
	Error string
}

func (p *PluginSettingValueDictationModel) GetPluginSettingType() PluginSettingDefinitionType {
	return PluginSettingDefinitionTypeDictationModel
}

func (p *PluginSettingValueDictationModel) GetKey() string {
	return p.Key
}

func (p *PluginSettingValueDictationModel) GetDefaultValue() string {
	return p.DefaultValue
}

func (p *PluginSettingValueDictationModel) Translate(translator func(ctx context.Context, key string) string) PluginSettingDefinitionValue {
	copy := *p
	copy.Label = translator(context.Background(), p.Label)
	copy.Tooltip = translator(context.Background(), p.Tooltip)
	copy.Options = make([]DictationModelOption, len(p.Options))
	for i := range p.Options {
		copy.Options[i] = p.Options[i]
	}
	return &copy
}
