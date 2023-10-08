import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_acrylic/flutter_acrylic.dart';
import 'package:web_socket_channel/web_socket_channel.dart';
import 'package:window_manager/window_manager.dart';

void main() async {
  WidgetsFlutterBinding.ensureInitialized();
  await windowManager.ensureInitialized();
  await Window.initialize();

  await Window.setEffect(
    effect: WindowEffect.mica,
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
  await windowManager.setVisibleOnAllWorkspaces(true, visibleOnFullScreen: true);
  await windowManager.setAsFrameless();
  windowManager.waitUntilReadyToShow(windowOptions, () async {
    await windowManager.show();
    await windowManager.focus();
  });

  runApp(const MyApp());
}

class MyApp extends StatelessWidget {
  const MyApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'Flutter Demo',
      theme: ThemeData(
        colorScheme: ColorScheme.fromSeed(seedColor: Colors.deepPurple),
        useMaterial3: true,
      ),
      home: MyHomePage(),
    );
  }
}

class MyHomePage extends StatefulWidget {
  MyHomePage({super.key});

  @override
  State<MyHomePage> createState() => _MyHomePageState();
}

class _MyHomePageState extends State<MyHomePage> {
  final _controller = TextEditingController();
  WebSocketChannel? channel;
  List<dynamic> results = [];

  @override
  void initState() {
    super.initState();
    connect();
  }

  connect() {
    final wsUrl = Uri.parse("ws://localhost:34987/ws");
    channel = WebSocketChannel.connect(wsUrl);
    channel?.stream.listen((message) async {
      print(message);

      var msg = jsonDecode(message);
      switch (msg["Method"]) {
        case "query":
          setState(() {
            results = msg["Data"];
          });
          break;
        case "ToggleApp":
          var isVisible = await windowManager.isVisible();
          if (isVisible) {
            await windowManager.hide();
          } else {
            await windowManager.show();
            await windowManager.focus();
            _controller.selection = TextSelection(baseOffset: 0, extentOffset: _controller.text.length);
          }
        case "result":
          print("result: ${msg["Params"]["result"]}");
          break;
        default:
          print("unknown method: ${msg["Method"]}");
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Column(
        children: <Widget>[
          RawKeyboardListener(
            focusNode: FocusNode(),
            onKey: (event) {
              if (event.logicalKey == LogicalKeyboardKey.escape) {
                windowManager.hide();
              }
            },
            child: TextField(
              controller: _controller,
              onChanged: (value) {
                setState(() {
                  results = [];
                });
                channel?.sink.add(jsonEncode({
                  "Id": "dfd",
                  "Method": "query",
                  "Params": {
                    "query": value,
                  },
                }));
              },
            ),
          ),
          //a alfred like result list
          Expanded(
            child: ListView.builder(
              itemCount: results.length,
              itemBuilder: (context, index) {
                return ListTile(
                  title: Text(results[index]["Title"]),
                  subtitle: Text(results[index]["SubTitle"]),
                );
              },
            ),
          ),
        ],
      ),
    );
  }
}
