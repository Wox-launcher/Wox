package definition

import "context"

// PluginSettingValueServiceAction is one operation exposed by a Wox-owned service row.
type PluginSettingValueServiceAction struct {
	ID      string
	Label   string
	Primary bool
	Danger  bool
	Enabled bool
}

// PluginSettingValueService renders the state and lifecycle actions of a Wox-owned service row.
type PluginSettingValueService struct {
	Key         string
	Title       string
	Description string
	Status      string
	Detail      string
	Actions     []PluginSettingValueServiceAction
}

func (p *PluginSettingValueService) GetPluginSettingType() PluginSettingDefinitionType {
	return PluginSettingDefinitionTypeFileIndexService
}

func (p *PluginSettingValueService) GetKey() string { return p.Key }

func (p *PluginSettingValueService) GetDefaultValue() string { return "" }

func (p *PluginSettingValueService) Translate(translator func(ctx context.Context, key string) string) PluginSettingDefinitionValue {
	copy := *p
	copy.Title = translator(context.Background(), p.Title)
	copy.Description = translator(context.Background(), p.Description)
	copy.Status = translator(context.Background(), p.Status)
	copy.Actions = append([]PluginSettingValueServiceAction(nil), p.Actions...)
	for i := range copy.Actions {
		copy.Actions[i].Label = translator(context.Background(), copy.Actions[i].Label)
	}
	return &copy
}
