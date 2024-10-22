import 'dart:async';
import 'dart:io';

import 'package:chinese_font_library/chinese_font_library.dart';
import 'package:flutter/material.dart';
import 'package:flutter_acrylic/flutter_acrylic.dart';
import 'package:get/get.dart';
import 'package:protocol_handler/protocol_handler.dart';
import 'package:uuid/v4.dart';
import 'package:window_manager/window_manager.dart';
import 'package:wox/api/wox_api.dart';
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
  launcherController.startDoctorCheckSchedule();

  await WoxWebsocketMsgUtil.instance.initialize(Uri.parse("ws://localhost:${Env.serverPort}/ws"), onMessageReceived: launcherController.handleWebSocketMessage);
  HeartbeatChecker().startChecking();
  Get.put(launcherController);
  var woxSettingController = WoxSettingController();
  Get.put(woxSettingController);

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
        useMaterial3: true,
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

class _WoxAppState extends State<WoxApp> with WindowListener, ProtocolListener {
  @override
  void initState() {
    super.initState();

    protocolHandler.addListener(this);

    setAcrylicEffect();

    var launcherController = Get.find<WoxLauncherController>();
    launcherController.isInSettingView.listen((isShowSetting) async {
      if (isShowSetting) {
        await windowManager.setAlwaysOnTop(false);
        await WoxThemeUtil.instance.loadTheme();
        await WoxSettingUtil.instance.loadSetting();
        launcherController.positionBeforeOpenSetting = await windowManager.getPosition();

        // when switching to setting view by executing the query action, which will trigger the hiding action panel, which will causing the window size will be changed first
        // so we need to wait the resize to complete and then resize the window to the setting view size
        WidgetsBinding.instance.addPostFrameCallback((timeStamp) async {
          await windowManager.setSize(const Size(1200, 800));
          if (LoggerSwitch.enableSizeAndPositionLog) Logger.instance.info(const UuidV4().generate(), "Resize: window to 1200x800 for setting view");
          await windowManager.center();
        });

        Get.find<WoxSettingController>().activePaneIndex.value = 0;
      } else {
        await windowManager.setAlwaysOnTop(true);
        launcherController.resizeHeight();
        await windowManager.setPosition(launcherController.positionBeforeOpenSetting);
        await windowManager.focus();
        launcherController.queryBoxFocusNode.requestFocus();
      }
      setState(() {});
    });
    windowManager.addListener(this);

    // notify server that ui is ready
    WidgetsBinding.instance.addPostFrameCallback((timeStamp) async {
      launcherController.resizeHeight();
      await windowManager.focus();
      launcherController.queryBoxFocusNode.requestFocus();

      WoxApi.instance.onUIReady();
    });
  }

  Future<void> setAcrylicEffect() async {
    if (Platform.isMacOS) {
      await windowManager.setVisibleOnAllWorkspaces(true, visibleOnFullScreen: true);
      await Window.setBlurViewState(MacOSBlurViewState.active);
      await Window.setEffect(effect: WindowEffect.popover, dark: false);
    }
    if (Platform.isWindows) {
      await Window.setEffect(effect: WindowEffect.mica, color: const Color(0xFF222222), dark: false);
    }
  }

  @override
  void onProtocolUrlReceived(String url) {
    Logger.instance.info(const UuidV4().generate(), "deep link received: $url");
    //replace %20 with space in the url
    url = url.replaceAll("%20", " ");
    // split the command and argument
    // wox://command?argument=value&argument2=value2
    var command = url.split("?")[0].split("//")[1];
    var arguments = url.split("?")[1].split("&");
    var argumentMap = <String, String>{};
    for (var argument in arguments) {
      var key = argument.split("=")[0];
      var value = argument.split("=")[1];
      // url decode value
      argumentMap[key] = Uri.decodeComponent(value);
    }

    WoxApi.instance.onProtocolUrlReceived(command, argumentMap);
  }

  @override
  void dispose() {
    protocolHandler.removeListener(this);
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
  void onWindowBlur() {
    WoxApi.instance.onFocusLost();
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
