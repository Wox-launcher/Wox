import 'dart:async';

import 'package:chinese_font_library/chinese_font_library.dart';
import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:protocol_handler/protocol_handler.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_border_drag_move_view.dart';
import 'package:wox/controllers/wox_ai_chat_controller.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/controllers/wox_setting_controller.dart';
import 'package:wox/utils/windows/window_manager.dart';
import 'package:wox/utils/windows/window_manager_interface.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/modules/launcher/views/wox_launcher_view.dart';
import 'package:wox/modules/setting/views/wox_setting_view.dart';
import 'package:wox/utils/env.dart';
import 'package:wox/utils/heartbeat_checker.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/wox_setting_util.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/utils/wox_websocket_msg_util.dart';

void main(List<String> arguments) async {
  await initialServices(arguments);
  await initWindow();
  await initDeepLink();
  runApp(const MyApp());
}

Future<void> initArgs(List<String> arguments) async {
  Logger.instance.info(const UuidV4().generate(), "Arguments: $arguments");
  if (arguments.isEmpty) {
    // dev env
    Env.isDev = true;
    Env.serverPort = 34987;
    Env.serverPid = -1;
    return;
  }

  if (arguments.length != 3) {
    throw Exception("Invalid arguments");
  }

  Env.serverPort = int.parse(arguments[0]);
  Env.serverPid = int.parse(arguments[1]);
  Env.isDev = arguments[2] == "true";
}

Future<void> initialServices(List<String> arguments) async {
  WidgetsFlutterBinding.ensureInitialized();
  await Logger.instance.initLogger();
  await initArgs(arguments);
  await WoxThemeUtil.instance.loadTheme();
  await WoxSettingUtil.instance.loadSetting();

  var launcherController = WoxLauncherController();
  launcherController.startRefreshSchedule();

  Timer.periodic(const Duration(minutes: 1), (timer) async {
    launcherController.doctorCheck();
  });

  await WoxWebsocketMsgUtil.instance.initialize(Uri.parse("ws://localhost:${Env.serverPort}/ws"), onMessageReceived: launcherController.handleWebSocketMessage);
  HeartbeatChecker().startChecking();
  Get.put(launcherController);
  var woxSettingController = WoxSettingController();
  Get.put(woxSettingController);
  var woxAIChatController = WoxAIChatController();
  Get.put(woxAIChatController);

  //load lang
  var langCode = WoxSettingUtil.instance.currentSetting.langCode;
  woxSettingController.updateLang(langCode);
}

Future<void> initDeepLink() async {
  // Register a custom protocol
  // For macOS platform needs to declare the scheme in ios/Runner/Info.plist
  await protocolHandler.register('wox');
}

Future<void> initWindow() async {
  await windowManager.waitUntilReadyToShow();
}

class MyApp extends StatelessWidget {
  const MyApp({super.key});

  @override
  Widget build(BuildContext context) {
    // Get the base text theme with Chinese font support
    final baseTextTheme = SystemChineseFont.textTheme(Brightness.light);

    // Scale down all font sizes to match Fluent UI appearance
    final scaledTextTheme = baseTextTheme.copyWith(
      bodyLarge: baseTextTheme.bodyLarge?.copyWith(fontSize: 13),
      bodyMedium: baseTextTheme.bodyMedium?.copyWith(fontSize: 13),
      bodySmall: baseTextTheme.bodySmall?.copyWith(fontSize: 12),
      labelLarge: baseTextTheme.labelLarge?.copyWith(fontSize: 13),
      labelMedium: baseTextTheme.labelMedium?.copyWith(fontSize: 12),
      labelSmall: baseTextTheme.labelSmall?.copyWith(fontSize: 11),
    );

    return MaterialApp(
      theme: ThemeData(
        useMaterial3: true,
        textTheme: scaledTextTheme,
      ),
      debugShowCheckedModeBanner: false,
      home: const WoxApp(),
    );
  }
}

class WoxApp extends StatefulWidget {
  const WoxApp({super.key});

  @override
  State<WoxApp> createState() => _WoxAppState();
}

class _WoxAppState extends State<WoxApp> with WindowListener, ProtocolListener {
  final launcherController = Get.find<WoxLauncherController>();
  final settingController = Get.find<WoxSettingController>();

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
      launcherController.resizeHeight();

      // Notify the backend that the UI is ready. The server-side will determine whether to display the UI window.
      WoxApi.instance.onUIReady();
    });
  }

  @override
  void onProtocolUrlReceived(String url) {
    Logger.instance.info(const UuidV4().generate(), "deep link received: $url");
    WoxApi.instance.onProtocolUrlReceived(url);
  }

  @override
  void dispose() {
    protocolHandler.removeListener(this);
    windowManager.removeListener(this);
    super.dispose();
  }

  @override
  void onWindowBlur() async {
    // if windows is already hidden, return
    // in Windows, when the window is hidden, the onWindowBlur event will be triggered which will cause
    // resize function to be called, and then the focus will be got again.
    // User will not be able to input anything because the focus is lost.
    if (!(await windowManager.isVisible())) {
      return;
    }

    // if in setting view, return
    if (launcherController.isInSettingView.value) {
      return;
    }

    WoxApi.instance.onFocusLost();
  }

  @override
  Widget build(BuildContext context) {
    return WoxBorderDragMoveArea(
      borderWidth: WoxThemeUtil.instance.currentTheme.value.appPaddingTop.toDouble(),
      onDragEnd: () {
        if (launcherController.isInSettingView.value) {
          return;
        }

        launcherController.focusQueryBox();
        launcherController.saveWindowPositionIfNeeded();
      },
      child: Obx(() => launcherController.isInSettingView.value ? const WoxSettingView() : const WoxLauncherView()),
    );
  }
}
