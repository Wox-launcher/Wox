import 'package:wox/entity/wox_plugin.dart';
import 'package:wox/entity/wox_preview.dart';
import 'package:wox/entity/wox_setting.dart';
import 'package:wox/entity/wox_theme.dart';

class EntityFactory {
  static T generateOBJ<T>(dynamic json) {
    if (T.toString() == "WoxTheme") {
      return WoxTheme.fromJson(json) as T;
    } else if (T.toString() == "WoxSetting") {
      return WoxSetting.fromJson(json) as T;
    } else if (T.toString() == "WoxPreview") {
      return WoxPreview.fromJson(json) as T;
    } else if (T.toString() == "List<StorePlugin>") {
      return (json as List).map((e) => StorePlugin.fromJson(e)).toList() as T;
    } else {
      return json as T;
    }
  }
}
