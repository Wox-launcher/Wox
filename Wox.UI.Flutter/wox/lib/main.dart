import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_acrylic/flutter_acrylic.dart';
import 'package:get/get.dart';
import 'package:logger/logger.dart';
import 'package:window_manager/window_manager.dart';
import 'package:wox/controller.dart';
import 'package:wox/modules/launcher/wox_launcher_controller.dart';
import 'package:wox/utils/env.dart';
import 'package:wox/utils/heartbeat_checker.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/view.dart';

void main(List<String> arguments) async {
  WidgetsFlutterBinding.ensureInitialized();

  await initArgs(arguments);
  HeartbeatChecker().startChecking();
  await loadSystemConfig();
  await initWindow();
  await initGetX();
  runApp(const MyApp());
}

Future<void> initArgs(List<String> arguments) async {
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

Future<void> loadSystemConfig() async {
  WoxThemeUtil.instance.loadTheme();
}

Future<void> initWindow() async {
  await windowManager.ensureInitialized();
  await Window.initialize();

  await Window.setEffect(
    effect: WindowEffect.popover,
    dark: true,
  );

  WindowOptions windowOptions = const WindowOptions(
    size: Size(800, 300),
    center: true,
    backgroundColor: Colors.transparent,
    skipTaskbar: true,
    alwaysOnTop: true,
    titleBarStyle: TitleBarStyle.hidden,
    windowButtonVisibility: false,
  );

  if (Platform.isMacOS) {
    await windowManager.setVisibleOnAllWorkspaces(true, visibleOnFullScreen: true);
  }
  await windowManager.setAsFrameless();
  await windowManager.setResizable(false);
  await windowManager.waitUntilReadyToShow(windowOptions);
}

Future<void> initGetX() async {
  Get.put(Logger(printer: SimplePrinter()));
  Get.put(WoxController());
  Get.put(WoxLauncherController());
}

class MyApp extends StatelessWidget {
  const MyApp({super.key});

  @override
  Widget build(BuildContext context) {
    return const MaterialApp(
      debugShowCheckedModeBanner: false,
      home: WoxApp(),
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
    return const Scaffold(
      backgroundColor: Colors.transparent,
      body: WoxView(),
    );
  }
}
