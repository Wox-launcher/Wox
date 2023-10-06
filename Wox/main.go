package main

import (
	"fmt"
	"runtime"
	"strings"
	"wox/plugin"
	"wox/resource"
	"wox/util"
)

import _ "wox/plugin/host"   // import all hosts
import _ "wox/plugin/system" // import all system plugins

func main() {
	// logger depends on location, so location must be initialized first
	locationErr := util.GetLocation().Init()
	if locationErr != nil {
		panic(locationErr)
	}

	ctx := util.NewTraceContext()
	util.GetLogger().Info(ctx, "------------------------------")
	util.GetLogger().Info(ctx, "Wox starting")
	util.GetLogger().Info(ctx, fmt.Sprintf("golang version: %s", strings.ReplaceAll(runtime.Version(), "go", "")))

	extractErr := resource.ExtractHosts(ctx)
	if extractErr != nil {
		util.GetLogger().Error(ctx, fmt.Errorf("failed to extract embed host file: %w", extractErr).Error())
		return
	}

	plugin.GetPluginManager().Start(ctx)

	ServeAndWait(ctx, 34987)
}
