package plugin

import "wox/common"

type MRUData struct {
	PluginID    string            `json:"pluginId"`
	Title       string            `json:"title"`
	SubTitle    string            `json:"subTitle"`
	Icon        common.WoxImage   `json:"icon"`
	ContextData map[string]string `json:"contextData"`
	LastUsed    int64             `json:"lastUsed"`
	UseCount    int               `json:"useCount"`
}
