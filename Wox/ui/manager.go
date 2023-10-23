package ui

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"os"
	"os/exec"
	"sync"
	"wox/setting"
	"wox/share"
	"wox/util"
)

var managerInstance *Manager
var managerOnce sync.Once
var logger *util.Log

type Manager struct {
	mainHotkey   *util.Hotkey
	queryHotkeys []*util.Hotkey
	ui           share.UI
	serverPort   int
	uiProcess    *os.Process
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
	if !util.IsProd() {
		logger.Info(ctx, "skip stopping ui app in dev mode")
		return
	}

	logger.Info(ctx, "start stopping ui app")
	var pid = m.uiProcess.Pid
	killErr := m.uiProcess.Kill()
	if killErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to kill ui process(%d): %s", pid, killErr))
	} else {
		util.GetLogger().Info(ctx, fmt.Sprintf("killed ui process(%d)", pid))
	}
}

func (m *Manager) RegisterMainHotkey(ctx context.Context, combineKey string) error {
	return m.mainHotkey.Register(ctx, combineKey, func() {
		m.ui.ToggleApp(ctx)
	})
}

func (m *Manager) RegisterQueryHotkey(ctx context.Context, queryHotkey setting.QueryHotkey) error {
	hotkey := &util.Hotkey{}
	err := hotkey.Register(ctx, queryHotkey.Hotkey, func() {
		m.ui.ChangeQuery(ctx, queryHotkey.Query)
		m.ui.ShowApp(ctx, share.ShowContext{SelectAll: false})
	})
	if err != nil {
		return err
	}

	m.queryHotkeys = append(m.queryHotkeys, hotkey)
	return nil
}

func (m *Manager) StartWebsocketAndWait(ctx context.Context, port int) {
	m.serverPort = port
	serveAndWait(ctx, port)
}

func (m *Manager) StartUIApp(ctx context.Context, port int) error {
	var appPath = util.GetLocation().GetUIAppPath()
	logger.Info(ctx, fmt.Sprintf("start ui app: %s", appPath))
	cmd := exec.Command(appPath, fmt.Sprintf("%d", port))
	cmd.Stdout = util.GetLogger().GetWriter()
	cmd.Stderr = util.GetLogger().GetWriter()
	cmdErr := cmd.Start()
	if cmdErr != nil {
		return cmdErr
	}

	m.uiProcess = cmd.Process
	util.GetLogger().Info(ctx, fmt.Sprintf("ui app pid: %d", cmd.Process.Pid))
	return nil
}

func (m *Manager) ToggleWindow() {
	ctx := util.NewTraceContext()
	logger.Info(ctx, "[UI] toggle window")
	requestUI(ctx, WebsocketMsg{
		Id:     uuid.NewString(),
		Method: "toggleWindow",
	})
}

func (m *Manager) GetUI(ctx context.Context) share.UI {
	return m.ui
}
