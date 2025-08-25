package mediaplayer

import (
	"context"
	"errors"
	"syscall"
	"wox/plugin"
)

var mediaRetriever = &WindowsRetriever{}

var (
	user32                       = syscall.NewLazyDLL("user32.dll")
	procGetForegroundWindow      = user32.NewProc("GetForegroundWindow")
	procGetWindowTextW           = user32.NewProc("GetWindowTextW")
	procGetClassNameW            = user32.NewProc("GetClassNameW")
	procGetWindowThreadProcessId = user32.NewProc("GetWindowThreadProcessId")
)

type WindowsRetriever struct {
	api plugin.API
}

func (w *WindowsRetriever) UpdateAPI(api plugin.API) {
	w.api = api
}

func (w *WindowsRetriever) GetCurrentMedia(ctx context.Context) (*MediaInfo, error) {
	return nil, errors.New("GetCurrentMedia not implemented on Windows")
}

func (w *WindowsRetriever) TogglePlayPause(ctx context.Context) error {
	return errors.New("TogglePlayPause not implemented on Windows")
}
