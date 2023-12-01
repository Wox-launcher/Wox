import 'dart:core';

import 'package:wox/entity/wox_theme.dart';
import 'package:wox/utils/wox_http_util.dart';

class WoxApi {
  WoxApi._privateConstructor();

  static final WoxApi _instance = WoxApi._privateConstructor();

  static WoxApi get instance => _instance;

  Future<WoxTheme> loadTheme() async {
    return await WoxHttpUtil.instance.postData<WoxTheme>("/theme", null);
  }
}
