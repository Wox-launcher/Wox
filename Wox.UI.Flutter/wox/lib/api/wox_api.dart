import 'dart:core';

import 'package:flutter/material.dart';
import 'package:wox/entity/wox_plugin.dart';
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

  Future<List<StorePlugin>> findStorePlugins() async {
    return await WoxHttpUtil.instance.postData("/plugin/store", null);
  }

  Future<List<InstalledPlugin>> findInstalledPlugins() async {
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
}
