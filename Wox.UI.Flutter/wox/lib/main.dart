import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_acrylic/flutter_acrylic.dart';
import 'package:get/get.dart';
import 'package:logger/logger.dart';
import 'package:window_manager/window_manager.dart';
import 'package:wox/controller.dart';
import 'package:wox/modules/launcher/views/wox_launcher_view.dart';
import 'package:wox/modules/launcher/wox_launcher_controller.dart';
import 'package:wox/view.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();
  initGetX();
  initWindow();
  runApp(const MyApp());
}

void initGetX() {
  Get.put(Logger(printer: SimplePrinter()));
  Get.put(WoxController());
  Get.put(WoxLauncherController());
}

void initWindow() async {
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
  await windowManager.waitUntilReadyToShow(windowOptions, () async {
    await windowManager.show();
    await windowManager.focus();
  });
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

class _WoxAppState extends State<WoxApp> {
  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.transparent,
      body: WoxView(),
    );
  }
}
