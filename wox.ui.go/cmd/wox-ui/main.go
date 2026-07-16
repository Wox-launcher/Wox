//go:build windows || darwin || linux

package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	woxui "github.com/Wox-launcher/wox.ui.go"
	"github.com/Wox-launcher/wox.ui.go/launcher"
)

const defaultDevServerPort = 34987

func main() {
	config, err := parseLaunchConfig(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
	app := launcher.New(config.port, config.isDev)
	err = woxui.Run(app.Start)
	if closeErr := app.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		log.Fatal(err)
	}
}

type launchConfig struct {
	port  int
	isDev bool
}

// parseLaunchConfig accepts the same production arguments as Flutter and keeps a no-argument development default.
func parseLaunchConfig(arguments []string) (launchConfig, error) {
	if len(arguments) == 0 {
		return launchConfig{port: defaultDevServerPort, isDev: true}, nil
	}
	if len(arguments) != 3 {
		return launchConfig{}, fmt.Errorf("expected <server-port> <server-pid> <is-dev>, got %d arguments", len(arguments))
	}
	port, err := strconv.Atoi(arguments[0])
	if err != nil || port <= 0 {
		return launchConfig{}, fmt.Errorf("invalid server port %q", arguments[0])
	}
	if _, err := strconv.Atoi(arguments[1]); err != nil {
		return launchConfig{}, fmt.Errorf("invalid server pid %q", arguments[1])
	}
	isDev, err := strconv.ParseBool(arguments[2])
	if err != nil {
		return launchConfig{}, fmt.Errorf("invalid development flag %q", arguments[2])
	}
	return launchConfig{port: port, isDev: isDev}, nil
}
