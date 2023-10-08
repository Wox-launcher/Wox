package plugin

import (
	"context"
	"path"
	"wox/util"
)

type API interface {
	ChangeQuery(ctx context.Context, query string)
	HideApp(ctx context.Context)
	ShowApp(ctx context.Context)
	ShowMsg(ctx context.Context, title string, description string, icon string)
	Log(ctx context.Context, msg string)
	GetTranslation(ctx context.Context, key string) string
}

type APIImpl struct {
	metadata Metadata
	logger   *util.Log
}

func (a *APIImpl) ChangeQuery(ctx context.Context, query string) {
	GetPluginManager().GetUI().ChangeQuery(ctx, query)
}

func (a *APIImpl) HideApp(ctx context.Context) {
	GetPluginManager().GetUI().HideApp(ctx)
}

func (a *APIImpl) ShowApp(ctx context.Context) {
	GetPluginManager().GetUI().ShowApp(ctx)
}

func (a *APIImpl) ShowMsg(ctx context.Context, title string, description string, icon string) {
	GetPluginManager().GetUI().ShowMsg(ctx, title, description, icon)
}

func (a *APIImpl) Log(ctx context.Context, msg string) {
	a.logger.Info(ctx, msg)
}

func (a *APIImpl) GetTranslation(ctx context.Context, key string) string {
	return ""
}

func NewAPI(metadata Metadata) API {
	apiImpl := &APIImpl{metadata: metadata}
	logFolder := path.Join(util.GetLocation().GetLogPluginDirectory(), metadata.Name)
	apiImpl.logger = util.CreateLogger(logFolder)
	return apiImpl
}
