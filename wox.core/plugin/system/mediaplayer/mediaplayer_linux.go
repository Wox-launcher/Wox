package mediaplayer

import (
	"context"
	"errors"
	"wox/plugin"
	"wox/util"
)

var mediaRetriever = &LinuxRetriever{}

type LinuxRetriever struct {
	api plugin.API
}

func (l *LinuxRetriever) UpdateAPI(api plugin.API) {
	l.api = api
}

func (l *LinuxRetriever) GetPlatform() string {
	return util.PlatformLinux
}
func (w *LinuxRetriever) GetCurrentMedia(ctx context.Context) (*MediaInfo, error) {
	return nil, errors.New("GetCurrentMedia not implemented on Linux")
}

func (w *LinuxRetriever) TogglePlayPause(ctx context.Context) error {
	return errors.New("TogglePlayPause not implemented on Linux")
}
