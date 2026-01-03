import 'package:wox/api/wox_api.dart';
import 'package:wox/entity/wox_setting.dart';

class WoxSettingUtil {
  late WoxSetting _currentSetting;

  WoxSettingUtil._privateConstructor();

  static final WoxSettingUtil _instance = WoxSettingUtil._privateConstructor();

  static WoxSettingUtil get instance => _instance;

  Future<void> loadSetting(String traceId) async {
    _currentSetting = await WoxApi.instance.loadSetting(traceId);
  }

  WoxSetting get currentSetting => _currentSetting;
}
