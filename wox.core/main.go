package main

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"wox/ai"
	"wox/analytics"
	"wox/database"
	"wox/diagnostic"
	"wox/migration"
	"wox/telemetry"

	"runtime"
	"strings"
	"time"
	"wox/common"
	"wox/i18n"
	"wox/plugin"
	"wox/resource"
	"wox/setting"
	"wox/ui"
	"wox/updater"
	"wox/util"
	"wox/util/clipboard"
	"wox/util/imagecache"
	"wox/util/mainthread"
	"wox/util/selection"

	_ "wox/plugin/host"

	// import all hosts

	// import all system plugins
	_ "wox/plugin/system"

	_ "wox/plugin/system/sys"

	_ "wox/plugin/system/app"

	_ "wox/plugin/system/calculator"

	_ "wox/plugin/system/converter"

	_ "wox/plugin/system/clipboard"

	_ "wox/plugin/system/mediaplayer"

	_ "wox/plugin/system/shell"

	_ "wox/plugin/system/emoji"

	_ "wox/plugin/system/explorer"

	_ "wox/plugin/system/browser_bookmark"

	_ "wox/plugin/system/file_search"

	_ "wox/plugin/system/glance"

	_ "wox/plugin/system/window_manager"
)

func main() {
	if diagnostic.GetManager().IsSupervisorArg(os.Args) {
		ctx := util.NewTraceContext()
		if locationErr := util.GetLocation().Init(); locationErr != nil {
			os.Exit(1)
		}
		os.Exit(diagnostic.GetManager().RunSupervisor(ctx, os.Args))
	}
	mainthread.Init(run)
}

func run() {
	// logger depends on location, so location must be initialized first
	locationErr := util.GetLocation().Init()
	if locationErr != nil {
		panic(locationErr)
	}

	defer util.GoRecover(context.Background(), "main panic", func(err error) {
		util.GetLogger().Error(context.Background(), fmt.Sprintf("main panic: %s", err.Error()))
	})

	ctx := util.NewTraceContext()
	bugReportArg := diagnostic.GetManager().IsBugReportArg(os.Args)
	if diagnostic.GetManager().IsEnabled() {
		util.GetLogger().SetLevel(setting.LogLevelDebug)
	}
	util.GetLogger().Info(ctx, "------------------------------")
	util.GetLogger().Info(ctx, fmt.Sprintf("Wox starting: %s", updater.CURRENT_VERSION))
	util.GetLogger().Info(ctx, fmt.Sprintf("golang version: %s", strings.ReplaceAll(runtime.Version(), "go", "")))
	util.GetLogger().Info(ctx, fmt.Sprintf("wox data location: %s", util.GetLocation().GetWoxDataDirectory()))
	util.GetLogger().Info(ctx, fmt.Sprintf("user data location: %s", util.GetLocation().GetUserDataDirectory()))
	if execPath, execErr := os.Executable(); execErr == nil {
		util.GetLogger().Info(ctx, fmt.Sprintf("startup pid: %d, executable: %s, args: %v", os.Getpid(), execPath, os.Args))
	} else {
		util.GetLogger().Info(ctx, fmt.Sprintf("startup pid: %d, executable: <error>, args: %v", os.Getpid(), os.Args))
	}

	// Check for an existing instance BEFORE doing any heavy initialization (database, analytics,
	// migrations). When this process is launched as a one-shot deeplink forwarder (e.g. via the
	// desktop URL-scheme handler on Linux), we just need to forward the request and exit.
	// Running the full startup sequence in that case wastes time and leaves an orphan process,
	// because mainthread.Init keeps the main goroutine alive in its event loop even after run()
	// returns. Using os.Exit(0) is the only reliable way to terminate cleanly here.
	if existingPort := getExistingInstancePort(ctx); existingPort > 0 {
		util.GetLogger().Info(ctx, fmt.Sprintf("there is existing instance running, port: %d", existingPort))

		if bugReportArg {
			_, postBugReportErr := util.HttpPost(ctx, fmt.Sprintf("http://127.0.0.1:%d/diagnostics/monitor/enable-restart", existingPort), "")
			if postBugReportErr != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("failed to enable bug aware mode in existing instance: %s", postBugReportErr.Error()))
			} else {
				util.GetLogger().Info(ctx, "enabled bug aware mode in existing instance, bye~")
			}
			os.Exit(0)
		}

		// if args has deeplink, post it to the existing instance and exit immediately
		for _, arg := range os.Args[1:] {
			if strings.HasPrefix(arg, "wox://") {
				_, postDeepLinkErr := util.HttpPost(ctx, fmt.Sprintf("http://127.0.0.1:%d/deeplink", existingPort), map[string]string{
					"deeplink": arg,
				})
				if postDeepLinkErr != nil {
					util.GetLogger().Error(ctx, fmt.Sprintf("failed to post deeplink to existing instance: %s", postDeepLinkErr.Error()))
				} else {
					util.GetLogger().Info(ctx, "post deeplink to existing instance successfully, bye~")
				}
				// Exit regardless of success/failure: this process has no further role.
				os.Exit(0)
			}
		}

		// show existing instance if no deeplink is provided
		_, postShowErr := util.HttpPost(ctx, fmt.Sprintf("http://127.0.0.1:%d/show", existingPort), "")
		if postShowErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("failed to show existing instance: %s", postShowErr.Error()))
		} else {
			util.GetLogger().Info(ctx, "show existing instance successfully, bye~")
		}
		// Exit regardless: the main goroutine is blocked in mainthread's event loop and will
		// never terminate on its own, so os.Exit is required to avoid a zombie process.
		os.Exit(0)
	}

	if bugReportArg && !diagnostic.GetManager().IsChildArg(os.Args) {
		if _, enableErr := diagnostic.GetManager().Enable(ctx, ""); enableErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("failed to enable bug aware mode from startup arg: %s", enableErr.Error()))
		} else {
			util.GetLogger().SetLevel(setting.LogLevelDebug)
			if supervisorErr := diagnostic.GetManager().StartSupervisorDetached(ctx, true); supervisorErr != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("failed to start bug aware supervisor from startup arg: %s", supervisorErr.Error()))
			} else {
				util.GetLogger().Info(ctx, "bug aware supervisor started from startup arg, exiting current process")
				diagnostic.GetManager().MarkCleanExit(ctx)
				os.Exit(0)
			}
		}
	}

	// User may launch Wox manually (not from bugreport) with the intent to enable bug aware mode
	// In this case, we should relaunch the supervisor and enable bug aware mode before the main instance starts.
	if diagnostic.GetManager().IsEnabled() && !diagnostic.GetManager().IsChildArg(os.Args) {
		if supervisorErr := diagnostic.GetManager().StartSupervisorDetached(ctx, true); supervisorErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("failed to start bug aware supervisor: %s", supervisorErr.Error()))
		} else {
			util.GetLogger().Info(ctx, "bug aware supervisor started, exiting current process")
			diagnostic.GetManager().MarkCleanExit(ctx)
			os.Exit(0)
		}
	}

	resource.EnsureLinuxDesktopIcon(ctx)
	if desktopEntryReady := util.EnsureDeepLinkProtocolHandler(ctx); desktopEntryReady && util.ShouldRelaunchLinuxFromDesktopEntry(os.Args[1:]) {
		util.GetLogger().Info(ctx, "Wayland session started without stable desktop identity, relaunching from Linux desktop entry")
		if relaunchErr := util.RelaunchLinuxFromDesktopEntry(ctx); relaunchErr != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("failed to relaunch from Linux desktop entry: %s", relaunchErr.Error()))
		} else {
			util.GetLogger().Info(ctx, "relaunched from Linux desktop entry, exiting current process")
			diagnostic.GetManager().MarkCleanExit(ctx)
			os.Exit(0)
		}
	}

	diagnostic.GetManager().RecordRunStart(ctx, diagnostic.GetManager().IsChildArg(os.Args))

	util.GetLogger().Info(ctx, "no existing instance found, proceeding with full startup")

	if err := database.Init(ctx); err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to initialize database: %s", err.Error()))
		return
	}

	if err := analytics.Init(ctx, database.GetDB()); err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to initialize analytics: %s", err.Error()))
	}

	if err := migration.Run(ctx); err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to run migration: %s", err.Error()))
		// In some cases, we might want to exit if migration fails, but for now we just log it.
	}

	serverPort, serverPortErr := resolveServerPort(ctx)
	if serverPortErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to get server port: %s", serverPortErr.Error()))
		return
	}
	util.GetLogger().Info(ctx, fmt.Sprintf("server port: %d", serverPort))
	ui.GetUIManager().UpdateServerPort(serverPort)
	common.SetServerPort(serverPort)

	writeErr := os.WriteFile(util.GetLocation().GetAppLockPath(), []byte(fmt.Sprintf("%d", serverPort)), 0644)
	if writeErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to write lock file: %s", writeErr.Error()))
	}

	extractErr := resource.Extract(ctx)
	if extractErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to extract embed file: %s", extractErr.Error()))
		return
	}

	settingErr := setting.GetSettingManager().Init(ctx)
	if settingErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to initialize settings: %s", settingErr.Error()))
		return
	}
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	util.GetLogger().SetLevel(woxSetting.LogLevel.Get())
	if diagnostic.GetManager().IsEnabled() {
		util.GetLogger().SetLevel(setting.LogLevelDebug)
	}

	// update proxy
	if woxSetting.HttpProxyEnabled.Get() {
		util.UpdateHTTPProxy(ctx, woxSetting.HttpProxyUrl.Get())
	}

	initCloudSync(ctx)

	langErr := i18n.GetI18nManager().UpdateLang(ctx, woxSetting.LangCode.Get())
	if langErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to initialize lang(%s): %s", woxSetting.LangCode.Get(), langErr.Error()))
		return
	}

	util.Go(ctx, "start ai command store manager", func() {
		ai.GetStoreManager().Start(util.NewTraceContext())
	})

	for _, arg := range os.Args {
		if arg == "--updated" {
			ui.GetUIManager().SetStartupNotify(common.NotifyMsg{
				Text:           i18n.GetI18nManager().TranslateWox(ctx, "ui_update_success"),
				DisplaySeconds: 5,
			})
			break
		}
	}

	themeErr := ui.GetUIManager().Start(ctx)
	if themeErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to initialize themes: %s", themeErr.Error()))
		return
	}

	if woxSetting.ShowTray.Get() {
		ui.GetUIManager().ShowTray()
	}

	shareUI := ui.GetUIManager().GetUI(ctx)
	clipboard.SetNativeImageFileWriter(shareUI.WriteClipboardImageFile)
	plugin.GetPluginManager().Start(ctx, shareUI)

	selection.InitSelection()

	// Start auto backup if enabled
	setting.GetSettingManager().StartAutoBackup(ctx)

	// Start MRU cleanup
	setting.GetSettingManager().StartMRUCleanup(ctx)

	// Start image cache cleanup
	imagecache.StartCleanupRoutine(ctx)

	// Start auto update checker if enabled
	updater.StartAutoUpdateChecker(ctx)

	if util.ShouldDisableTelemetryForTest() {
		util.GetLogger().Info(ctx, "skip telemetry in test mode")
	} else {
		// Send anonymous usage telemetry if enabled
		telemetry.SendPresenceIfNeeded(ctx)

		// Start periodic telemetry heartbeat for long-running processes
		telemetry.StartPeriodicHeartbeat(ctx)
	}

	// Platform-specific keyboard implementations handle their own main-thread dispatch.
	registerMainHotkeyErr := ui.GetUIManager().RegisterMainHotkey(ctx, woxSetting.MainHotkey.Get())
	if registerMainHotkeyErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to register main hotkey: %s", registerMainHotkeyErr.Error()))
	}
	registerSelectionHotkeyErr := ui.GetUIManager().RegisterSelectionHotkey(ctx, woxSetting.SelectionHotkey.Get())
	if registerSelectionHotkeyErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to register selection hotkey: %s", registerSelectionHotkeyErr.Error()))
	}
	for _, queryHotkey := range woxSetting.QueryHotkeys.Get() {
		registerQueryHotkeyErr := ui.GetUIManager().RegisterQueryHotkey(ctx, queryHotkey)
		if registerQueryHotkeyErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("failed to register query hotkey: %s", registerQueryHotkeyErr.Error()))
		}
	}

	if util.IsProd() {
		util.Go(ctx, "start ui", func() {
			time.Sleep(time.Millisecond * 200) // wait websocket server start
			appErr := ui.GetUIManager().StartUIApp(ctx)
			if appErr != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("failed to start ui app: %s", appErr.Error()))
				return
			}
		})
	}

	ui.GetUIManager().StartWebsocketAndWait(ctx)
}

func resolveServerPort(ctx context.Context) (int, error) {
	if util.IsProd() {
		return util.GetAvailableTcpPort(ctx)
	}

	testPort, testErr := util.GetTestServerPortOverride()
	if testErr == nil {
		return testPort, nil
	}

	return util.DefaultDevServerPort, nil
}

// retrieves the instance port from the existing instance lock file.
// It returns 0 if the lock file doesn't exist or fails to read the file.
func getExistingInstancePort(ctx context.Context) int {
	filePath := util.GetLocation().GetAppLockPath()
	if !util.IsFileExists(filePath) {
		return 0
	}

	file, err := os.ReadFile(filePath)
	if err != nil {
		return 0
	}

	port, err := strconv.Atoi(string(file))
	if err != nil {
		return 0
	}

	//check if the port is valid
	response, err := util.HttpGet(ctx, fmt.Sprintf("http://127.0.0.1:%d/ping", port))
	if err != nil {
		return 0
	}

	if !strings.Contains(string(response), "pong") {
		return 0
	}

	return port
}
