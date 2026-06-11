import 'dart:async';

import 'package:flutter/material.dart';
import 'package:get/get.dart';
import 'package:uuid/v4.dart';
import 'package:wox/api/wox_api.dart';
import 'package:wox/components/wox_border_drag_move_view.dart';
import 'package:wox/controllers/wox_ai_chat_controller.dart';
import 'package:wox/controllers/wox_launcher_controller.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/entity/wox_websocket_msg.dart';
import 'package:wox/enums/wox_msg_type_enum.dart';
import 'package:wox/modules/launcher/views/wox_launcher_view.dart';
import 'package:wox/runtime/wox_window_driver.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/multiplewindow/wox_multiple_window.dart';
import 'package:wox/utils/wox_setting_util.dart';
import 'package:wox/utils/wox_theme_util.dart';
import 'package:wox/utils/wox_websocket_msg_util.dart';

class WoxAppRuntime {
  WoxAppRuntime._({required this.primaryInstance}) : registry = WoxInstanceRegistry();

  static late WoxAppRuntime instance;

  final WoxAppInstance primaryInstance;
  final WoxInstanceRegistry registry;

  static WoxAppRuntime initializePrimary({required String sessionId}) {
    final windowDriver = WoxPrimaryWindowDriver();
    final launcherController = WoxLauncherController(sessionId: sessionId, windowDriver: windowDriver, isPrimaryInstance: true);
    final aiChatController = WoxAIChatController(launcherController: launcherController);
    launcherController.aiChatController = aiChatController;

    final primary = WoxAppInstance(
      sessionId: sessionId,
      role: WoxAppInstanceRole.primary,
      instanceName: "",
      windowDriver: windowDriver,
      launcherController: launcherController,
      aiChatController: aiChatController,
      isRegisteredWithGet: false,
    );
    instance = WoxAppRuntime._(primaryInstance: primary);
    instance.registry.registerPrimary(primary);
    return instance;
  }

  Future<void> handleCoreWebSocketMessage(WoxWebsocketMsg msg) async {
    if (msg.type == WoxMsgTypeEnum.WOX_MSG_TYPE_REQUEST.code && msg.method == "OpenWoxInstance") {
      try {
        await registry.openFromRequest(msg);
        await respond(msg, true, null);
      } catch (e, stackTrace) {
        Logger.instance.error(msg.traceId, "OpenWoxInstance failed: $e $stackTrace");
        await respond(msg, false, e.toString());
      }
      return;
    }

    // Messages without an instance session are core/global UI commands. Keep
    // those on the primary launcher so older backend calls retain their
    // process-level behavior while secondary sessions stay explicitly routed.
    await primaryInstance.launcherController.handleWebSocketMessage(msg);
  }

  Future<void> respond(WoxWebsocketMsg request, bool success, dynamic data) async {
    await WoxWebsocketMsgUtil.instance.sendMessage(
      WoxWebsocketMsg(
        requestId: request.requestId,
        traceId: request.traceId,
        sessionId: request.sessionId,
        type: WoxMsgTypeEnum.WOX_MSG_TYPE_RESPONSE.code,
        method: request.method,
        data: data,
        success: success,
      ),
    );
  }
}

enum WoxAppInstanceRole {
  primary("primary"),
  secondary("secondary");

  const WoxAppInstanceRole(this.code);

  final String code;
}

class OpenWoxInstanceRequest {
  OpenWoxInstanceRequest({required this.role, required this.instanceName, required this.query, required this.showAppParams});

  final String role;
  final String instanceName;
  final PlainQuery query;
  final ShowAppParams showAppParams;

  factory OpenWoxInstanceRequest.fromJson(Map<String, dynamic> json) {
    final queryData = json["Query"] as Map<String, dynamic>? ?? <String, dynamic>{};
    final showAppData = json["ShowApp"] as Map<String, dynamic>? ?? <String, dynamic>{};
    return OpenWoxInstanceRequest(
      role: json["Role"]?.toString() ?? WoxAppInstanceRole.secondary.code,
      instanceName: json["InstanceName"]?.toString() ?? "",
      query: PlainQuery.fromJson(queryData),
      showAppParams: ShowAppParams.fromJson(showAppData),
    );
  }
}

class WoxAppInstance {
  WoxAppInstance({
    required this.sessionId,
    required this.role,
    required this.instanceName,
    required this.windowDriver,
    required this.launcherController,
    required this.aiChatController,
    required this.isRegisteredWithGet,
  });

  final String sessionId;
  final WoxAppInstanceRole role;
  final String instanceName;
  final WoxWindowDriver windowDriver;
  final WoxLauncherController launcherController;
  final WoxAIChatController aiChatController;
  final bool isRegisteredWithGet;
  bool isDestroyed = false;

  Future<void> open({required PlainQuery query, required ShowAppParams showAppParams}) async {
    final traceId = const UuidV4().generate();
    final nextQuery =
        query.queryId.isEmpty
            ? PlainQuery(
              queryId: const UuidV4().generate(),
              queryType: query.queryType,
              queryText: query.queryText,
              querySelection: query.querySelection,
              queryRefinements: query.queryRefinements,
            )
            : query;
    await launcherController.onQueryChanged(traceId, nextQuery, "open wox instance", moveCursorToEnd: true);
    await launcherController.showApp(traceId, showAppParams);
  }

  void destroy() {
    if (isDestroyed) {
      return;
    }
    isDestroyed = true;
    WoxWebsocketMsgUtil.instance.unregisterSession(sessionId);
    unawaited(WoxApi.instance.onInstanceDestroyed(const UuidV4().generate(), sessionId: sessionId));
    if (isRegisteredWithGet) {
      Get.delete<WoxAIChatController>(tag: sessionId, force: true);
      Get.delete<WoxLauncherController>(tag: sessionId, force: true);
    }
  }
}

class WoxInstanceRegistry {
  final Map<String, WoxAppInstance> _instancesBySessionId = {};
  final Map<String, String> _sessionIdByInstanceName = {};
  WoxAppInstance? _primaryInstance;

  void registerPrimary(WoxAppInstance instance) {
    _primaryInstance = instance;
    _instancesBySessionId[instance.sessionId] = instance;
    WoxWebsocketMsgUtil.instance.registerSession(instance.sessionId, instance.launcherController.handleWebSocketMessage);
  }

  Future<void> openFromRequest(WoxWebsocketMsg msg) async {
    final request = OpenWoxInstanceRequest.fromJson(Map<String, dynamic>.from(msg.data as Map));
    if (request.role != WoxAppInstanceRole.secondary.code) {
      await _primaryInstance?.open(query: request.query, showAppParams: request.showAppParams);
      return;
    }

    await openSecondary(instanceName: request.instanceName, initialQuery: request.query, showAppParams: request.showAppParams);
  }

  Future<WoxAppInstance> openSecondary({required String instanceName, required PlainQuery initialQuery, required ShowAppParams showAppParams}) async {
    final existingSessionId = instanceName.isEmpty ? null : _sessionIdByInstanceName[instanceName];
    final existingInstance = existingSessionId == null ? null : _instancesBySessionId[existingSessionId];
    if (existingInstance != null && !existingInstance.isDestroyed) {
      await existingInstance.open(query: initialQuery, showAppParams: showAppParams);
      return existingInstance;
    }

    if (instanceName.isNotEmpty) {
      _sessionIdByInstanceName.remove(instanceName);
    }

    final sessionId = "secondary-${const UuidV4().generate()}";
    final initialWidth = showAppParams.windowWidth > 0 ? showAppParams.windowWidth.toDouble() : WoxSettingUtil.instance.currentSetting.appWidth.toDouble();
    final initialSize = Size(initialWidth, 120);
    final windowDriver = WoxSecondaryWindowDriver(initialSize: initialSize);
    final launcherController = WoxLauncherController(sessionId: sessionId, windowDriver: windowDriver, isPrimaryInstance: false);
    windowDriver.setOnBlur(() => launcherController.handleWindowBlur(const UuidV4().generate()));
    final aiChatController = WoxAIChatController(launcherController: launcherController);
    launcherController.aiChatController = aiChatController;
    final instance = WoxAppInstance(
      sessionId: sessionId,
      role: WoxAppInstanceRole.secondary,
      instanceName: instanceName,
      windowDriver: windowDriver,
      launcherController: launcherController,
      aiChatController: aiChatController,
      isRegisteredWithGet: true,
    );

    _instancesBySessionId[sessionId] = instance;
    if (instanceName.isNotEmpty) {
      _sessionIdByInstanceName[instanceName] = sessionId;
    }

    Get.put(launcherController, tag: sessionId);
    Get.put(aiChatController, tag: sessionId);
    WoxWebsocketMsgUtil.instance.registerSession(sessionId, launcherController.handleWebSocketMessage);

    final windowId = "wox.instance.$sessionId";
    final handle = await WoxMultipleWindow.createWindow(
      id: windowId,
      title: "Wox",
      preferredSize: initialSize,
      preferredConstraints: BoxConstraints.tight(initialSize),
      showTitleBar: false,
      mica: true,
      focusIfExists: true,
      resizable: true,
      minimizable: false,
      closeOnRequest: true,
      centerOnCreate: false,
      roundedCorners: true,
      onDestroyed: () => _destroySecondary(instance),
      builder:
          (_) => WoxBorderDragMoveArea(
            borderWidth: WoxThemeUtil.instance.currentTheme.value.appPaddingTop.toDouble(),
            onDragStart: windowDriver.startDragging,
            onDragEnd: () {
              launcherController.focusQueryBox();
            },
            child: WoxLauncherView(controller: launcherController),
          ),
    );
    windowDriver.attachHandle(handle);
    await instance.open(query: initialQuery, showAppParams: showAppParams);
    return instance;
  }

  void _destroySecondary(WoxAppInstance instance) {
    _instancesBySessionId.remove(instance.sessionId);
    if (instance.instanceName.isNotEmpty && _sessionIdByInstanceName[instance.instanceName] == instance.sessionId) {
      _sessionIdByInstanceName.remove(instance.instanceName);
    }
    instance.destroy();
  }
}
