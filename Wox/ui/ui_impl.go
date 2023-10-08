package ui

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"wox/util"
)

type uiImpl struct {
}

func (u *uiImpl) ChangeQuery(ctx context.Context, query string) {
	u.send(ctx, "ChangeQuery", map[string]string{
		"Query": query,
	})
}

func (u *uiImpl) HideApp(ctx context.Context) {
	u.send(ctx, "HideApp", nil)
}

func (u *uiImpl) ShowApp(ctx context.Context) {
	u.send(ctx, "ShowApp", nil)
}

func (u *uiImpl) ToggleApp(ctx context.Context) {
	u.send(ctx, "ToggleApp", nil)
}

func (u *uiImpl) ShowMsg(ctx context.Context, title string, description string, icon string) {
	u.send(ctx, "ShowMsg", map[string]string{
		"Title":       title,
		"Description": description,
		"Icon":        icon,
	})
}

func (u *uiImpl) send(ctx context.Context, method string, params map[string]string) {
	if params == nil {
		params = make(map[string]string)
	}

	util.GetLogger().Info(ctx, fmt.Sprintf("[UI] %s", method))
	requestUI(ctx, websocketRequest{
		Id:     uuid.NewString(),
		Method: method,
		Params: params,
	})
}
