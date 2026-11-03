package emoji

import (
	"context"
	"wox/plugin/plugintest"
)

type stubAPI struct {
	plugintest.StubAPI

	settings map[string]string
}

func (s *stubAPI) setting(key string) string {
	if s.settings == nil {
		return ""
	}
	return s.settings[key]
}

func (s *stubAPI) GetTranslation(ctx context.Context, key string) string {
	return key
}

func (s *stubAPI) GetSetting(ctx context.Context, key string) string {
	return s.setting(key)
}
