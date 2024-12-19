import 'dart:convert';
import 'dart:core';

import 'package:wox/entity/wox_ai.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_lang.dart';
import 'package:wox/entity/wox_plugin.dart';
import 'package:wox/entity/wox_query.dart';
import 'package:wox/entity/wox_setting.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/utils/wox_http_util.dart';

class WoxApi {
  WoxApi._privateConstructor();

  static final WoxApi _instance = WoxApi._privateConstructor();

  static WoxApi get instance => _instance;

  Future<WoxTheme> loadTheme() async {
    return await WoxHttpUtil.instance.postData<WoxTheme>("/theme", null);
  }

  Future<WoxSetting> loadSetting() async {
    return await WoxHttpUtil.instance.postData<WoxSetting>("/setting/wox", null);
  }

  Future<void> updateSetting(String key, String value) async {
    await WoxHttpUtil.instance.postData("/setting/wox/update", {"Key": key, "Value": value});
  }

  Future<void> updatePluginSetting(String pluginId, String key, String value) async {
    await WoxHttpUtil.instance.postData("/setting/plugin/update", {"PluginId": pluginId, "Key": key, "Value": value});
  }

  Future<List<PluginDetail>> findStorePlugins() async {
    return await WoxHttpUtil.instance.postData("/plugin/store", null);
  }

  Future<List<PluginDetail>> findInstalledPlugins() async {
    return await WoxHttpUtil.instance.postData("/plugin/installed", null);
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

  Future<bool> isHotkeyAvailable(String hotkey) async {
    return await WoxHttpUtil.instance.postData("/hotkey/available", {"hotkey": hotkey});
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

  Future<void> onProtocolUrlReceived(String command, Map<String, String> arguments) async {
    await WoxHttpUtil.instance.postData("/deeplink", {
      "command": command,
      "arguments": arguments,
    });
  }

  Future<List<AIModel>> findAIModels() async {
    return await WoxHttpUtil.instance.postData("/ai/models", null);
  }

  Future<bool> doctorCheck() async {
    return await WoxHttpUtil.instance.postData("/doctor/check", null);
  }
}
