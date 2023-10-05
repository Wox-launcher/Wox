package main

import (
	"wox/plugin"
	"wox/util"
)

import _ "wox/plugin/host" // import all hosts

func main() {
	locationErr := util.GetLocation().Init()
	if locationErr != nil {
		panic(locationErr)
	}

	plugin.GetPluginManager().Start(util.NewTraceContext())
}
