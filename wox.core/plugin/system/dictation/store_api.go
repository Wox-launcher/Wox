package dictation

import (
	"context"

	"wox/plugin"
)

type dictationSettingAPI interface {
	GetSetting(ctx context.Context, key string) string
	SaveSetting(ctx context.Context, key string, value string, isPlatformSpecific bool)
	Log(ctx context.Context, level plugin.LogLevel, msg string)
}

type dictationHistoryAPI interface {
	dictationSettingAPI
	RefreshQuery(ctx context.Context, param plugin.RefreshQueryParam)
}
