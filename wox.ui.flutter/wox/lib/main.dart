import 'dart:async';
import 'dart:io';
import 'dart:ui';

import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:protocol_handler/protocol_handler.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_border_drag_move_view.dart';
import 'package:wox/components/wox_drag_move_state.dart';
import 'package:wox/controllers/wox_ai_chat_controller.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/controllers/wox_screenshot_controller.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/utils/windows/window_manager.dart';
import 'package:wox/utils/windows/window_manager_interface.dart';
import 'package:wox/utils/windows/windows_keydata_compatibility.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/modules/launcher/views/wox_launcher_view.dart';
import 'package:wox/modules/onboarding/views/wox_onboarding_view.dart';
import 'package:wox/modules/screenshot/views/wox_screenshot_view.dart';
import 'package:wox/modules/setting/views/wox_setting_view.dart';
import 'package:wox/utils/env.dart';
import 'package:wox/utils/heartbeat_checker.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/wox_setting_util.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/utils/wox_interface_size_util.dart';
import 'package:wox/utils/wox_websocket_msg_util.dart';

Future<void> main(List<String> arguments) async {
  await runZonedGuarded(
    () async {
      await initialServices(arguments);
      await initWindow();

      await initDeepLink();

      runApp(const MyApp());
    },
    (error, stack) {
      Logger.instance.crash(const UuidV4().generate(), "Unhandled Flutter zone error: $error\n$stack");
      unawaited(Logger.instance.flush());
    },
  );
}

void registerFlutterGlobalErrorHandlers(String traceId) {
  final bindingType = WidgetsBinding.instance.runtimeType.toString();
  if (Platform.environment.containsKey("FLUTTER_TEST") || bindingType.contains("TestWidgetsFlutterBinding") || bindingType.contains("LiveTestWidgetsFlutterBinding")) {
    return;
  }

  // Bug fix: only crash handlers force synchronous disk writes. Normal errors
  // stay buffered so plugin error storms cannot block the query input path,
  // while framework/platform crashes still reach ui.log before process exit.
  FlutterError.onError = (details) {
    FlutterError.presentError(details);
    Logger.instance.crash(traceId, "FlutterError: ${details.exception}\n${details.stack}");
    unawaited(Logger.instance.flush());
  };

  PlatformDispatcher.instance.onError = (error, stack) {
    Logger.instance.crash(traceId, "PlatformDispatcher error: $error\n$stack");
    unawaited(Logger.instance.flush());
    return true;
  };
}

Future<void> initArgs(List<String> arguments) async {
  Logger.instance.info(const UuidV4().generate(), "Arguments: $arguments");
  if (arguments.isEmpty) {
    // dev env
    Env.isDev = true;
    Env.serverPort = Env.defaultDevServerPort;
    Env.serverPid = -1;
    Env.sessionId = const UuidV4().generate();
    return;
  }

  if (arguments.length != 3) {
    throw Exception("Invalid arguments");
  }

  Env.serverPort = int.parse(arguments[0]);
  Env.serverPid = int.parse(arguments[1]);
  Env.isDev = arguments[2] == "true";
  Env.sessionId = const UuidV4().generate();
}

Future<void> initialServices(List<String> arguments) async {
  final traceId = const UuidV4().generate();

  WidgetsFlutterBinding.ensureInitialized();

  // Desktop-tuned image cache: the launcher shows small icons, not large
  // photos. 200 entries / 20 MB is plenty and avoids reserving the full
  // 1000 entries / 100 MB default that mobile apps expect.
  PaintingBinding.instance.imageCache.maximumSize = 200;
  PaintingBinding.instance.imageCache.maximumSizeBytes = 20 * 1024 * 1024;

  await Logger.instance.initLogger();
  WindowsKeyDataCompatibility.install();
  registerFlutterGlobalErrorHandlers(traceId);
  HeartbeatChecker().init();
  await WoxWebsocketMsgUtil.instance.init();
  await initArgs(arguments);
  await WoxThemeUtil.instance.loadTheme(traceId);
  await WoxSettingUtil.instance.loadSetting(traceId);
  WoxInterfaceSizeUtil.instance.refreshFromDensity(WoxSettingUtil.instance.currentSetting.uiDensity);
  Logger.instance.setLogLevel(WoxSettingUtil.instance.currentSetting.logLevel);

  var launcherController = WoxLauncherController();
  // Register before invoking controller methods because GetX initializes child
  // list controllers from onInit().
  Get.put(launcherController);
  launcherController.doctorCheck();
  await launcherController.loadDiagnosticStatus(traceId);

  await WoxWebsocketMsgUtil.instance.initialize(Uri.parse("ws://127.0.0.1:${Env.serverPort}/ws"), onMessageReceived: launcherController.handleWebSocketMessage);
  HeartbeatChecker().startChecking();
  var woxSettingController = WoxSettingController();
  Get.put(woxSettingController);
  Get.put(WoxScreenshotController());
  var woxAIChatController = WoxAIChatController();
  Get.put(woxAIChatController);

  var langCode = WoxSettingUtil.instance.currentSetting.langCode;
  await woxSettingController.loadLang(langCode);
}

Future<void> initDeepLink() async {
  // Bug fix: macOS smoke startup can leave plugin method calls unresolved when
  // Flutter's foreground handoff fails. Deep-link registration is useful but
  // not required for drawing the first frame, so bound the wait to avoid
  // blocking runApp forever during startup smoke runs.
  try {
    // Register a custom protocol
    // For macOS platform needs to declare the scheme in ios/Runner/Info.plist
    await protocolHandler.register('wox').timeout(const Duration(seconds: 3));
  } catch (e) {
    Logger.instance.error(const UuidV4().generate(), "Error registering deep link protocol: $e");
  }
}

Future<void> initWindow() async {
  await windowManager.waitUntilReadyToShow();
}

class MyApp extends StatelessWidget {
  const MyApp({super.key});

  // Platform-specific CJK font fallback list. Flutter's engine-level fallback
  // on desktop is unreliable for CJK glyphs, so we explicitly name the system
  // fonts that ship with each OS. This replaces chinese_font_library without
  // pulling in any dependency or bundling font files.
  List<String> get _cjkFontFallback {
    if (Platform.isWindows) return ['Microsoft YaHei', 'SimSun'];
    if (Platform.isMacOS) return ['PingFang SC', 'Heiti SC', 'STHeiti'];
    return ['Noto Sans CJK SC', 'WenQuanYi Micro Hei', 'Droid Sans Fallback'];
  }

  TextTheme buildAppTextTheme(String appFontFamily) {
    final baseTextTheme = Typography.material2021().black;
    final scaledTextTheme = baseTextTheme.copyWith(
      bodyLarge: baseTextTheme.bodyLarge?.copyWith(fontSize: 13),
      bodyMedium: baseTextTheme.bodyMedium?.copyWith(fontSize: 13),
      bodySmall: baseTextTheme.bodySmall?.copyWith(fontSize: 12),
      labelLarge: baseTextTheme.labelLarge?.copyWith(fontSize: 13),
      labelMedium: baseTextTheme.labelMedium?.copyWith(fontSize: 12),
      labelSmall: baseTextTheme.labelSmall?.copyWith(fontSize: 11),
    );

    final fallback = _cjkFontFallback;
    if (appFontFamily.isEmpty) {
      return scaledTextTheme.apply(fontFamilyFallback: fallback);
    }

    return scaledTextTheme.apply(fontFamily: appFontFamily, fontFamilyFallback: fallback);
  }

  @override
  Widget build(BuildContext context) {
    final settingController = Get.find<WoxSettingController>();

    return Obx(() {
      final appFontFamily = settingController.woxSetting.value.appFontFamily.trim();
      final textTheme = buildAppTextTheme(appFontFamily);

      return MaterialApp(
        navigatorKey: Get.key,
        theme: ThemeData(
          useMaterial3: true,
          textTheme: textTheme,
          fontFamily: appFontFamily.isEmpty ? null : appFontFamily,
          splashFactory: NoSplash.splashFactory,
          splashColor: Colors.transparent,
          highlightColor: Colors.transparent,
        ),
        debugShowCheckedModeBanner: false,
        home: const WoxApp(),
      );
    });
  }
}

class WoxApp extends StatefulWidget {
  const WoxApp({super.key});

  @override
  State<WoxApp> createState() => _WoxAppState();
}

class _WoxAppState extends State<WoxApp> with WindowListener, ProtocolListener {
  final launcherController = Get.find<WoxLauncherController>();
  final screenshotController = Get.find<WoxScreenshotController>();
  final settingController = Get.find<WoxSettingController>();
  bool _isRecoveringScreenshotWindowFocus = false;

  @override
  void initState() {
    super.initState();
    // Preload plugins at app startup so settings view has data ready
    final startupTraceId = const UuidV4().generate();
    settingController.preloadPlugins(startupTraceId);

    protocolHandler.addListener(this);
    windowManager.addListener(this);

    // notify server that ui is ready
    WidgetsBinding.instance.addPostFrameCallback((timeStamp) async {
      // Adjust the window height to match the query box height.
      // This is necessary due to dynamic height calculations on Windows caused by DPI scaling issues.
      launcherController.resizeHeight(traceId: startupTraceId, reason: "initial resize after first frame");

      // Notify the backend that the UI is ready. The server-side will determine whether to display the UI window.
      await WoxApi.instance.onUIReady(startupTraceId);
    });
  }

  @override
  void onProtocolUrlReceived(String url) {
    final traceId = const UuidV4().generate();
    Logger.instance.info(traceId, "deep link received: $url");
    WoxApi.instance.onProtocolUrlReceived(traceId, url);
  }

  @override
  void dispose() {
    protocolHandler.removeListener(this);
    windowManager.removeListener(this);
    super.dispose();
  }

  @override
  void onWindowBlur() async {
    final traceId = UuidV4().generate();
    Logger.instance.debug(
      traceId,
      "onWindowBlur triggered: forceHideOnBlur=${launcherController.forceHideOnBlur}, isShowFormActionPanel=${launcherController.isShowFormActionPanel.value}, isShowActionPanel=${launcherController.isShowActionPanel.value}, isInSettingView=${launcherController.isInSettingView.value}, isInOnboardingView=${launcherController.isInOnboardingView.value}",
    );
    // if windows is already hidden, return
    // in Windows, when the window is hidden, the onWindowBlur event will be triggered which will cause
    // resize function to be called, and then the focus will be got again.
    // User will not be able to input anything because the focus is lost.
    final isVisible = await windowManager.isVisible();
    if (!isVisible) {
      Logger.instance.debug(traceId, "onWindowBlur ignored: window is not visible");
      return;
    }

    if (screenshotController.isSessionActive.value) {
      // Ignoring blur used to leave the screenshot session alive but unfocused after the native
      // selection overlay handed control back to Flutter. Reclaim focus here so the annotation
      // workspace stays interactive even if macOS briefly moves key-window ownership elsewhere.
      await _recoverScreenshotWindowFocus(traceId);
      return;
    }

    // Settings and onboarding are management views. They intentionally ignore
    // launcher hide-on-blur so setup is not interrupted by System Settings or
    // other windows opened during the guide.
    if (launcherController.isInSettingView.value || launcherController.isInOnboardingView.value) {
      Logger.instance.debug(traceId, "onWindowBlur ignored: management view is active");
      return;
    }

    if (Platform.isLinux && WoxDragMoveState.isActive) {
      Logger.instance.info(traceId, "linux-window-drag dart stage=blur-during-drag action=ignore ${WoxDragMoveState.debugSummary}");
      return;
    }

    if (launcherController.forceHideOnBlur) {
      Logger.instance.debug(traceId, "onWindowBlur triggers hideApp because forceHideOnBlur is true");
      launcherController.hideApp(traceId);
      return;
    }

    Logger.instance.debug(traceId, "onWindowBlur notify backend focus lost");
    WoxApi.instance.onFocusLost(traceId);
  }

  Future<void> _recoverScreenshotWindowFocus(String traceId) async {
    if (_isRecoveringScreenshotWindowFocus) {
      return;
    }

    _isRecoveringScreenshotWindowFocus = true;
    try {
      await windowManager.focus();
    } catch (e) {
      Logger.instance.error(traceId, "onWindowBlur failed screenshot focus recovery: $e");
    } finally {
      _isRecoveringScreenshotWindowFocus = false;
    }
  }

  @override
  Widget build(BuildContext context) {
    return Obx(() {
      if (screenshotController.isSessionActive.value) {
        // Screenshot capture needs the entire virtual-desktop surface for region selection. The
        // launcher border drag overlays would steal edge pointer input and make secondary displays
        // feel "dead", so the capture view bypasses them while the session is active.
        return const WoxScreenshotView();
      }

      return WoxBorderDragMoveArea(
        borderWidth: WoxThemeUtil.instance.currentTheme.value.appPaddingTop.toDouble(),
        onDragEnd: () {
          if (launcherController.isInSettingView.value || launcherController.isInOnboardingView.value) {
            return;
          }

          unawaited(launcherController.focusLauncherKeyboardTarget());
          launcherController.saveWindowPositionIfNeeded(reason: "drag-end");
        },
        child:
            launcherController.isInOnboardingView.value
                ? const WoxOnboardingView()
                : launcherController.isInSettingView.value
                ? const WoxSettingView()
                : const WoxLauncherView(),
      );
    });
  }
}
