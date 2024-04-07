import 'dart:async';
import 'dart:io';

import 'package:chinese_font_library/chinese_font_library.dart';
import 'package:flutter/material.dart';
import 'package:flutter_acrylic/flutter_acrylic.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:window_manager/window_manager.dart';
import 'package:wox/modules/launcher/views/wox_launcher_view.dart';
import 'package:wox/modules/launcher/wox_launcher_controller.dart';
import 'package:wox/modules/setting/views/wox_setting_view.dart';
import 'package:wox/modules/setting/wox_setting_controller.dart';
import 'package:wox/utils/env.dart';
import 'package:wox/utils/heartbeat_checker.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/wox_setting_util.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/utils/wox_websocket_msg_util.dart';

void main(List<String> arguments) async {
  await initialServices(arguments);
  await initWindow();
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
  var launcherController = WoxLauncherController()..startRefreshSchedule();
  await WoxWebsocketMsgUtil.instance.initialize(Uri.parse("ws://localhost:${Env.serverPort}/ws"), onMessageReceived: launcherController.handleWebSocketMessage);
  HeartbeatChecker().startChecking();
  Get.put(launcherController);
  Get.put(WoxSettingController());
}

Future<void> initWindow() async {
  await windowManager.ensureInitialized();
  await Window.initialize();

  WindowOptions windowOptions = WindowOptions(
    size: Size(WoxSettingUtil.instance.currentSetting.appWidth.toDouble(), WoxThemeUtil.instance.getQueryBoxHeight()),
    center: true,
    skipTaskbar: true,
    alwaysOnTop: true,
    titleBarStyle: TitleBarStyle.hidden,
    windowButtonVisibility: false,
  );

  if (Platform.isMacOS) {
    await windowManager.setVisibleOnAllWorkspaces(true, visibleOnFullScreen: true);
    await Window.setBlurViewState(MacOSBlurViewState.active);
    await Window.setEffect(effect: WindowEffect.popover, dark: false);
  }
  if (Platform.isWindows) {
    await Window.setEffect(effect: WindowEffect.mica);
  }
  await windowManager.setResizable(false);
  await windowManager.setMaximizable(false);
  await windowManager.setMinimizable(false);
  await windowManager.waitUntilReadyToShow(windowOptions);
}

class MyApp extends StatelessWidget {
  const MyApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      theme: ThemeData(
        textTheme: SystemChineseFont.textTheme(Brightness.light),
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

class _WoxAppState extends State<WoxApp> with WindowListener {
  @override
  void initState() {
    super.initState();
    var launcherController = Get.find<WoxLauncherController>();
    launcherController.isInSettingView.stream.listen((isShowSetting) async {
      if (isShowSetting) {
        launcherController.positionBeforeOpenSetting = await windowManager.getPosition();
        windowManager.setSize(const Size(1200, 800));
        windowManager.center();
        Get.find<WoxSettingController>().activePaneIndex.value = 0;
      } else {
        launcherController.resizeHeight();
        windowManager.setPosition(launcherController.positionBeforeOpenSetting);
      }
      setState(() {});
    });
    windowManager.addListener(this);
  }

  @override
  void dispose() {
    windowManager.removeListener(this);
    super.dispose();
  }

  @override
  void onWindowFocus() {
    // https://pub.dev/packages/window_manager#hidden-at-launch
    if (Platform.isWindows) {
      setState(() {});
    }
  }

  @override
  Widget build(BuildContext context) {
    final launcherController = Get.find<WoxLauncherController>();

    if (launcherController.isInSettingView.value) {
      return const WoxSettingView();
    }

    return const WoxLauncherView();
  }
}
