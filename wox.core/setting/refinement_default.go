package setting

import "encoding/base64"

func refinementDefaultKey(command string) string {
	return "__wox.refinement-default:" + base64.RawURLEncoding.EncodeToString([]byte(command))
}

// RefinementDefault reads a host-owned snapshot from this plugin's synced settings.
func (p *PluginSetting) RefinementDefault(command string) (map[string]string, bool) {
	var values map[string]string
	if err := p.store.Get(refinementDefaultKey(command), &values); err != nil || values == nil {
		return nil, false
	}
	return values, true
}

// SetRefinementDefault saves one command's snapshot or deletes it when values is nil.
func (p *PluginSetting) SetRefinementDefault(command string, values map[string]string) error {
	key := refinementDefaultKey(command)
	if values == nil {
		return p.store.DeleteWithSync(key, true)
	}
	return p.store.SetWithSync(key, values, true)
}
