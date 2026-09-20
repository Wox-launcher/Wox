package ai

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

// PluginToolAIName stays stable across catalog changes and fits the 64-byte provider limit.
func PluginToolAIName(pluginID, toolName string) string {
	// Hash both identities before truncating the readable suffix, so long tool names
	// and plugins with identical display names still receive distinct names.
	id := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(pluginID)) + "\x00" + toolName))
	return fmt.Sprintf("plugin_%x__%.23s", id[:16], toolName)
}
