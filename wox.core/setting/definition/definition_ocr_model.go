package definition

import "context"

// PluginSettingValueOCRModel is the Wox-internal model picker shared by OCR
// consumers. Each consumer stores its own selected model while model files and
// download state remain shared by the OCR runtime.
type PluginSettingValueOCRModel struct {
	Key          string
	Label        string
	Tooltip      string
	DefaultValue string
	Options      []OCRModelOption

	Style PluginSettingValueStyle `json:"-"`
}

// OCRModelOption represents a system or downloadable OCR engine.
type OCRModelOption struct {
	ID               string
	DisplayName      string
	Description      string
	Languages        string
	Recommended      bool
	Available        bool
	Status           string
	DownloadProgress int
	SizeMB           int
	Error            string
}

func (p *PluginSettingValueOCRModel) GetPluginSettingType() PluginSettingDefinitionType {
	return PluginSettingDefinitionTypeOCRModel
}

func (p *PluginSettingValueOCRModel) GetKey() string {
	return p.Key
}

func (p *PluginSettingValueOCRModel) GetDefaultValue() string {
	return p.DefaultValue
}

func (p *PluginSettingValueOCRModel) Translate(translator func(ctx context.Context, key string) string) PluginSettingDefinitionValue {
	copy := *p
	copy.Label = translator(context.Background(), p.Label)
	copy.Tooltip = translator(context.Background(), p.Tooltip)
	copy.Options = make([]OCRModelOption, len(p.Options))
	for i := range p.Options {
		copy.Options[i] = p.Options[i]
		copy.Options[i].DisplayName = translator(context.Background(), p.Options[i].DisplayName)
		copy.Options[i].Description = translator(context.Background(), p.Options[i].Description)
		copy.Options[i].Languages = translator(context.Background(), p.Options[i].Languages)
	}
	return &copy
}
