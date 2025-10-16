import 'dart:convert';
import 'dart:core';

import 'package:wox/entity/wox_ai.dart';
import 'package:wox/entity/wox_backup.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_lang.dart';
import 'package:wox/entity/wox_plugin.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/entity/wox_runtime_status.dart';
import 'package:wox/entity/wox_setting.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/models/doctor_check_result.dart';
import 'package:wox/utils/wox_http_util.dart';

class WoxApi {
  WoxApi._privateConstructor();

  static final WoxApi _instance = WoxApi._privateConstructor();

  static WoxApi get instance => _instance;

  Future<WoxTheme> loadTheme() async {
    return await WoxHttpUtil.instance.postData<WoxTheme>("/theme", null);
  }

  Future<WoxSetting> loadSetting() async {
    return await WoxHttpUtil.instance
        .postData<WoxSetting>("/setting/wox", null);
  }

  Future<void> updateSetting(String key, String value) async {
    await WoxHttpUtil.instance
        .postData("/setting/wox/update", {"Key": key, "Value": value});
  }

  Future<List<WoxRuntimeStatus>> getRuntimeStatuses() async {
    return await WoxHttpUtil.instance.postData("/runtime/status", null);
  }

  Future<void> updatePluginSetting(
      String pluginId, String key, String value) async {
    await WoxHttpUtil.instance.postData("/setting/plugin/update",
        {"PluginId": pluginId, "Key": key, "Value": value});
  }

  Future<List<PluginDetail>> findStorePlugins() async {
    return await WoxHttpUtil.instance.postData("/plugin/store", null);
  }

  Future<List<PluginDetail>> findInstalledPlugins() async {
    return await WoxHttpUtil.instance.postData("/plugin/installed", null);
  }

  Future<PluginDetail> getPluginDetail(String pluginId) async {
    return await WoxHttpUtil.instance
        .postData("/plugin/detail", {"id": pluginId});
  }

  Future<void> installPlugin(String id) async {
    await WoxHttpUtil.instance.postData("/plugin/install", {"id": id});
  }

  Future<void> uninstallPlugin(String id) async {
    await WoxHttpUtil.instance.postData("/plugin/uninstall", {"id": id});
  }

  Future<void> disablePlugin(String id) async {
    await WoxHttpUtil.instance.postData("/plugin/disable", {"id": id});
  }

  Future<void> enablePlugin(String id) async {
    await WoxHttpUtil.instance.postData("/plugin/enable", {"id": id});
  }

  Future<List<WoxTheme>> findStoreThemes() async {
    return await WoxHttpUtil.instance.postData("/theme/store", null);
  }

  Future<List<WoxTheme>> findInstalledThemes() async {
    return await WoxHttpUtil.instance.postData("/theme/installed", null);
  }

  Future<void> installTheme(String id) async {
    await WoxHttpUtil.instance.postData("/theme/install", {"id": id});
  }

  Future<void> uninstallTheme(String id) async {
    await WoxHttpUtil.instance.postData("/theme/uninstall", {"id": id});
  }

  Future<void> applyTheme(String id) async {
    await WoxHttpUtil.instance.postData("/theme/apply", {"id": id});
  }

  Future<bool> isHotkeyAvailable(String hotkey) async {
    return await WoxHttpUtil.instance
        .postData("/hotkey/available", {"hotkey": hotkey});
  }

  Future<void> onUIReady() async {
    await WoxHttpUtil.instance.postData("/on/ready", {});
  }

  Future<void> onFocusLost() async {
    await WoxHttpUtil.instance.postData("/on/focus/lost", {});
  }

  Future<void> onShow() async {
    await WoxHttpUtil.instance.postData("/on/show", {});
  }

  Future<void> onQueryBoxFocus() async {
    await WoxHttpUtil.instance.postData("/on/querybox/focus", {});
  }

  Future<void> onHide(PlainQuery query) async {
    await WoxHttpUtil.instance.postData("/on/hide", {
      "query": query.toJson(),
    });
  }

  Future<WoxImage> getQueryIcon(PlainQuery query) async {
    return await WoxHttpUtil.instance.postData("/query/icon", {
      "query": query.toJson(),
    });
  }

  Future<double> getResultPreviewWidthRatio(PlainQuery query) async {
    return await WoxHttpUtil.instance.postData("/query/ratio", {
      "query": query.toJson(),
    });
  }

  Future<List<WoxLang>> getAllLanguages() async {
    return await WoxHttpUtil.instance.postData("/lang/available", {});
  }

  Future<Map<String, String>> getLangJson(String langCode) async {
    var langJsonStr = await WoxHttpUtil.instance.postData("/lang/json", {
      "langCode": langCode,
    });

    //unmarshal json string to map
    var jsonMap = json.decode(langJsonStr);
    return jsonMap.cast<String, String>();
  }

  Future<void> onProtocolUrlReceived(String deeplink) async {
    await WoxHttpUtil.instance.postData("/deeplink", {
      "deeplink": deeplink,
    });
  }

  Future<List<AIModel>> findAIModels() async {
    return await WoxHttpUtil.instance.postData("/ai/models", null);
  }

  Future<List<AIProviderInfo>> findAIProviders() async {
    return await WoxHttpUtil.instance.postData("/ai/providers", null);
  }

  Future<String> pingAIModel(
      String providerName, String apiKey, String host) async {
    return await WoxHttpUtil.instance.postData("/ai/ping", {
      "name": providerName,
      "apiKey": apiKey,
      "host": host,
    });
  }

  Future<List<AIMCPTool>> findAIMCPServerTools(dynamic data) async {
    return await WoxHttpUtil.instance.postData("/ai/mcp/tools", data);
  }

  Future<List<AIMCPTool>> findAIMCPServerToolsAll() async {
    return await WoxHttpUtil.instance.postData("/ai/mcp/tools/all", null);
  }

  Future<List<AIAgent>> findAIAgents() async {
    return await WoxHttpUtil.instance.postData("/ai/agents", null);
  }

  Future<AIModel> findDefaultAIModel() async {
    return await WoxHttpUtil.instance.postData("/ai/model/default", null);
  }

  Future<void> sendChatRequest(WoxAIChatData data) async {
    return await WoxHttpUtil.instance.postData("/ai/chat", {
      "chatData": data.toJson(),
    });
  }

  Future<List<DoctorCheckResult>> doctorCheck() async {
    return await WoxHttpUtil.instance
        .postData<List<DoctorCheckResult>>("/doctor/check", null);
  }

  Future<List<WoxQueryResult>> queryMRU(String traceId) async {
    final response =
        await WoxHttpUtil.instance.postData("/query/mru", {"traceId": traceId});
    if (response is List) {
      return response.map((item) => WoxQueryResult.fromJson(item)).toList();
    }
    return [];
  }

  Future<String> getUserDataLocation() async {
    return await WoxHttpUtil.instance
        .postData("/setting/userdata/location", null);
  }

  Future<void> updateUserDataLocation(String location) async {
    await WoxHttpUtil.instance
        .postData("/setting/userdata/location/update", {"location": location});
  }

  Future<void> backupNow() async {
    await WoxHttpUtil.instance.postData("/backup/now", null);
  }

  Future<List<WoxBackup>> getAllBackups() async {
    return await WoxHttpUtil.instance.postData("/backup/all", null);
  }

  Future<void> restoreBackup(String id) async {
    await WoxHttpUtil.instance.postData("/backup/restore", {"id": id});
  }

  Future<String> getBackupFolder() async {
    return await WoxHttpUtil.instance.postData("/backup/folder", null);
  }

  Future<void> open(String path) async {
    await WoxHttpUtil.instance.postData("/open", {"path": path});
  }

  Future<String> getWoxVersion() async {
    return await WoxHttpUtil.instance.postData("/version", null);
  }

  Future<void> saveWindowPosition(int x, int y) async {
    await WoxHttpUtil.instance.postData("/setting/position", {"x": x, "y": y});
  }

  Future<void> toolbarSnooze(String text, String duration) async {
    await WoxHttpUtil.instance
        .postData("/toolbar/snooze", {"text": text, "duration": duration});
  }
}
