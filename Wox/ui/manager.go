package ui

import (
	"context"
	"github.com/google/uuid"
	"sync"
	"wox/share"
	"wox/util"
)

var managerInstance *Manager
var managerOnce sync.Once
var logger *util.Log

type Manager struct {
	mainHotkey *util.Hotkey
	ui         share.UI
	serverPort int
}

func GetUIManager() *Manager {
	managerOnce.Do(func() {
		managerInstance = &Manager{}
		managerInstance.mainHotkey = &util.Hotkey{}
		managerInstance.ui = &uiImpl{}
		logger = util.GetLogger()
	})
	return managerInstance
}

func (m *Manager) Send(ctx context.Context) error {
	return nil
}

func (m *Manager) Stop(ctx context.Context) {
	logger.Info(ctx, "start stopping ui client")
}

func (m *Manager) RegisterMainHotkey(ctx context.Context, combineKey string) error {
	return m.mainHotkey.Register(ctx, combineKey, func() {
		m.ui.ToggleApp(ctx)
	})
}

func (m *Manager) StartWebsocketAndWait(ctx context.Context, port int) {
	m.serverPort = port
	serveAndWait(ctx, port)
}

func (m *Manager) ToggleWindow() {
	ctx := util.NewTraceContext()
	logger.Info(ctx, "[UI] toggle window")
	requestUI(ctx, websocketRequest{
		Id:     uuid.NewString(),
		Method: "toggleWindow",
		Params: make(map[string]string),
	})
}

func (m *Manager) GetUI(ctx context.Context) share.UI {
	return m.ui
}
