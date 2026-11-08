package plugin

import "fmt"

// SetRefinementDefault serializes saves with uninstall and uses normal plugin-setting sync.
func (s *Store) SetRefinementDefault(pluginID, command string, values map[string]string) error {
	s.installMu.Lock()
	defer s.installMu.Unlock()
	instance := GetPluginManager().GetPluginInstanceById(pluginID)
	if instance == nil || instance.Setting == nil {
		return fmt.Errorf("plugin %q is not available", pluginID)
	}
	return instance.Setting.SetRefinementDefault(command, values)
}
