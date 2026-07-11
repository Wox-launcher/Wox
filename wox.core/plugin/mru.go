package plugin

import "wox/common"

type MRUData struct {
	PluginID    string
	Title       string
	SubTitle    string
	Icon        common.WoxImage
	ContextData map[string]string
	Env         QueryEnv
	LastUsed    int64
	UseCount    int
}
