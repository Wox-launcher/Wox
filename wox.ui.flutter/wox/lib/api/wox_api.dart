import 'dart:convert';
import 'dart:core';

import 'package:wox/entity/wox_ai.dart';
import 'package:wox/entity/wox_backup.dart';
import 'package:wox/entity/wox_lang.dart';
import 'package:wox/entity/wox_plugin.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/entity/wox_runtime_status.dart';
import 'package:wox/entity/wox_setting.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/entity/wox_usage_stats.dart';
import 'package:wox/models/doctor_check_result.dart';
import 'package:wox/utils/wox_http_util.dart';

class WoxApi {
  WoxApi._privateConstructor();

  static final WoxApi _instance = WoxApi._privateConstructor();

  static WoxApi get instance => _instance;

  Future<WoxTheme> loadTheme(String traceId) async {
    return await WoxHttpUtil.instance.postData<WoxTheme>(
      traceId,
      "/theme",
      null,
    );
  }

  Future<WoxSetting> loadSetting(String traceId) async {
    return await WoxHttpUtil.instance.postData<WoxSetting>(
      traceId,
      "/setting/wox",
      null,
    );
  }

  Future<void> updateSetting(String traceId, String key, String value) async {
    await WoxHttpUtil.instance.postData(traceId, "/setting/wox/update", {
      "Key": key,
      "Value": value,
    });
  }

  Future<List<String>> getSystemFontFamilies(String traceId) async {
    return await WoxHttpUtil.instance.postData<List<String>>(
      traceId,
      "/setting/ui/fonts",
      null,
    );
  }

  Future<List<WoxRuntimeStatus>> getRuntimeStatuses(String traceId) async {
    return await WoxHttpUtil.instance.postData(
      traceId,
      "/runtime/status",
      null,
    );
  }

  Future<void> updatePluginSetting(
    String traceId,
    String pluginId,
    String key,
    String value,
  ) async {
    await WoxHttpUtil.instance.postData(traceId, "/setting/plugin/update", {
      "PluginId": pluginId,
      "Key": key,
      "Value": value,
    });
  }

  Future<List<PluginDetail>> findStorePlugins(String traceId) async {
    return await WoxHttpUtil.instance.postData(traceId, "/plugin/store", null);
  }

  Future<List<PluginDetail>> findInstalledPlugins(String traceId) async {
    return await WoxHttpUtil.instance.postData(
      traceId,
      "/plugin/installed",
      null,
    );
  }

  Future<PluginDetail> getPluginDetail(String traceId, String pluginId) async {
    return await WoxHttpUtil.instance.postData(traceId, "/plugin/detail", {
      "id": pluginId,
    });
  }

  Future<void> installPlugin(String traceId, String id) async {
    await WoxHttpUtil.instance.postData(traceId, "/plugin/install", {"id": id});
  }

  Future<void> uninstallPlugin(String traceId, String id) async {
    await WoxHttpUtil.instance.postData(traceId, "/plugin/uninstall", {
      "id": id,
    });
  }

  Future<void> disablePlugin(String traceId, String id) async {
    await WoxHttpUtil.instance.postData(traceId, "/plugin/disable", {"id": id});
  }

  Future<void> enablePlugin(String traceId, String id) async {
    await WoxHttpUtil.instance.postData(traceId, "/plugin/enable", {"id": id});
  }

  Future<List<WoxTheme>> findStoreThemes(String traceId) async {
    return await WoxHttpUtil.instance.postData(traceId, "/theme/store", null);
  }

  Future<List<WoxTheme>> findInstalledThemes(String traceId) async {
    return await WoxHttpUtil.instance.postData(
      traceId,
      "/theme/installed",
      null,
    );
  }

  Future<void> installTheme(String traceId, String id) async {
    await WoxHttpUtil.instance.postData(traceId, "/theme/install", {"id": id});
  }

  Future<void> uninstallTheme(String traceId, String id) async {
    await WoxHttpUtil.instance.postData(traceId, "/theme/uninstall", {
      "id": id,
    });
  }

  Future<void> applyTheme(String traceId, String id) async {
    await WoxHttpUtil.instance.postData(traceId, "/theme/apply", {"id": id});
  }

  Future<bool> isHotkeyAvailable(String traceId, String hotkey) async {
    return await WoxHttpUtil.instance.postData(traceId, "/hotkey/available", {
      "hotkey": hotkey,
    });
  }

  Future<void> onUIReady(String traceId) async {
    await WoxHttpUtil.instance.postData(traceId, "/on/ready", {});
  }

  Future<void> onFocusLost(String traceId) async {
    await WoxHttpUtil.instance.postData(traceId, "/on/focus/lost", {});
  }

  Future<void> onShow(String traceId) async {
    await WoxHttpUtil.instance.postData(traceId, "/on/show", {});
  }

  Future<void> onQueryBoxFocus(String traceId) async {
    await WoxHttpUtil.instance.postData(traceId, "/on/querybox/focus", {});
  }

  Future<void> onHide(String traceId) async {
    await WoxHttpUtil.instance.postData(traceId, "/on/hide", {});
  }

  Future<void> onSetting(String traceId, bool inSettingView) async {
    await WoxHttpUtil.instance.postData(traceId, "/on/setting", {
      "inSettingView": inSettingView,
    });
  }

  Future<WoxUsageStats> getUsageStats(String traceId) async {
    return await WoxHttpUtil.instance.postData<WoxUsageStats>(
      traceId,
      "/usage/stats",
      {},
    );
  }

  Future<QueryMetadata> getQueryMetadata(
    String traceId,
    PlainQuery query,
  ) async {
    return await WoxHttpUtil.instance.postData(traceId, "/query/metadata", {
      "query": query.toJson(),
    });
  }

  Future<List<WoxLang>> getAllLanguages(String traceId) async {
    return await WoxHttpUtil.instance.postData(traceId, "/lang/available", {});
  }

  Future<Map<String, String>> getLangJson(
    String traceId,
    String langCode,
  ) async {
    var langJsonStr = await WoxHttpUtil.instance.postData(
      traceId,
      "/lang/json",
      {"langCode": langCode},
    );

    //unmarshal json string to map
    var jsonMap = json.decode(langJsonStr);
    return jsonMap.cast<String, String>();
  }

  Future<void> onProtocolUrlReceived(String traceId, String deeplink) async {
    await WoxHttpUtil.instance.postData(traceId, "/deeplink", {
      "deeplink": deeplink,
    });
  }

  Future<List<AIModel>> findAIModels(String traceId) async {
    return await WoxHttpUtil.instance.postData(traceId, "/ai/models", null);
  }

  Future<List<AIProviderInfo>> findAIProviders(String traceId) async {
    return await WoxHttpUtil.instance.postData(traceId, "/ai/providers", null);
  }

  Future<String> pingAIModel(
    String traceId,
    String providerName,
    String apiKey,
    String host,
  ) async {
    return await WoxHttpUtil.instance.postData(traceId, "/ai/ping", {
      "name": providerName,
      "apiKey": apiKey,
      "host": host,
    });
  }

  Future<List<AIMCPTool>> findAIMCPServerTools(
    String traceId,
    dynamic data,
  ) async {
    return await WoxHttpUtil.instance.postData(traceId, "/ai/mcp/tools", data);
  }

  Future<List<AIMCPTool>> findAIMCPServerToolsAll(String traceId) async {
    return await WoxHttpUtil.instance.postData(
      traceId,
      "/ai/mcp/tools/all",
      null,
    );
  }

  Future<List<AIAgent>> findAIAgents(String traceId) async {
    return await WoxHttpUtil.instance.postData(traceId, "/ai/agents", null);
  }

  Future<AIModel> findDefaultAIModel(String traceId) async {
    return await WoxHttpUtil.instance.postData(
      traceId,
      "/ai/model/default",
      null,
    );
  }

  Future<void> sendChatRequest(String traceId, WoxAIChatData data) async {
    return await WoxHttpUtil.instance.postData(traceId, "/ai/chat", {
      "chatData": data.toJson(),
    });
  }

  Future<List<DoctorCheckResult>> doctorCheck(String traceId) async {
    return await WoxHttpUtil.instance.postData<List<DoctorCheckResult>>(
      traceId,
      "/doctor/check",
      null,
    );
  }

  Future<String> getUserDataLocation(String traceId) async {
    return await WoxHttpUtil.instance.postData(
      traceId,
      "/setting/userdata/location",
      null,
    );
  }

  Future<void> updateUserDataLocation(String traceId, String location) async {
    await WoxHttpUtil.instance.postData(
      traceId,
      "/setting/userdata/location/update",
      {"location": location},
    );
  }

  Future<void> backupNow(String traceId) async {
    await WoxHttpUtil.instance.postData(traceId, "/backup/now", null);
  }

  Future<List<WoxBackup>> getAllBackups(String traceId) async {
    return await WoxHttpUtil.instance.postData(traceId, "/backup/all", null);
  }

  Future<void> restoreBackup(String traceId, String id) async {
    await WoxHttpUtil.instance.postData(traceId, "/backup/restore", {"id": id});
  }

  Future<String> getBackupFolder(String traceId) async {
    return await WoxHttpUtil.instance.postData(traceId, "/backup/folder", null);
  }

  Future<void> clearLogs(String traceId) async {
    await WoxHttpUtil.instance.postData(traceId, "/log/clear", null);
  }

  Future<void> openLogFile(String traceId) async {
    await WoxHttpUtil.instance.postData(traceId, "/log/open", null);
  }

  Future<void> open(String traceId, String path) async {
    await WoxHttpUtil.instance.postData(traceId, "/open", {"path": path});
  }

  Future<String> getWoxVersion(String traceId) async {
    return await WoxHttpUtil.instance.postData(traceId, "/version", null);
  }

  Future<void> saveWindowPosition(String traceId, int x, int y) async {
    await WoxHttpUtil.instance.postData(traceId, "/setting/position", {
      "x": x,
      "y": y,
    });
  }

  Future<void> toolbarSnooze(
    String traceId,
    String text,
    String duration,
  ) async {
    await WoxHttpUtil.instance.postData(traceId, "/toolbar/snooze", {
      "text": text,
      "duration": duration,
    });
  }
}
