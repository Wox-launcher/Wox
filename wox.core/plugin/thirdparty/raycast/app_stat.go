package raycast

import (
	"os"
)

type appStat struct {
	isDir bool
}

var statApp = func(path string) (appStat, error) {
	info, err := os.Stat(path)
	if err != nil {
		return appStat{}, err
	}
	return appStat{isDir: info.IsDir()}, nil
}
