package main

import (
	"github.com/google/uuid"
	"wox/util"
)

func toggleWindow() {
	ctx := util.NewTraceContext()
	util.GetLogger().Info(ctx, "[UI] toggle window")
	RequestUI(ctx, websocketRequest{
		Id:     uuid.NewString(),
		Method: "toggleWindow",
		Params: make(map[string]string),
	})
}
