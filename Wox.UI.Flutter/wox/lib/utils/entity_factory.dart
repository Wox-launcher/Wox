import 'package:uuid/v4.dart';
import 'package:wox/entity/wox_plugin.dart';
import 'package:wox/entity/wox_preview.dart';
import 'package:wox/entity/wox_setting.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/utils/log.dart';

class EntityFactory {
  static T generateOBJ<T>(dynamic json) {
    Logger.instance.debug(const UuidV4().generate(), "try to unmarshal post data, datatype=${T.toString()}");
    if (T.toString() == "WoxTheme") {
      return WoxTheme.fromJson(json) as T;
    } else if (T.toString() == "WoxSetting") {
      return WoxSetting.fromJson(json) as T;
    } else if (T.toString() == "WoxPreview") {
      return WoxPreview.fromJson(json) as T;
    } else if (T.toString() == "List<PluginDetail>") {
      return (json as List).map((e) => PluginDetail.fromJson(e)).toList() as T;
    } else if (T.toString() == "List<WoxSettingTheme>") {
      return (json as List).map((e) => WoxSettingTheme.fromJson(e)).toList() as T;
    } else {
      return json as T;
    }
  }
}
