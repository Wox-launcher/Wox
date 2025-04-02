import 'package:wox/entity/wox_ai.dart';
import 'package:wox/entity/wox_backup.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_lang.dart';
import 'package:wox/entity/wox_plugin.dart';
import 'package:wox/entity/wox_preview.dart';
import 'package:wox/entity/wox_setting.dart';
import 'package:wox/entity/wox_theme.dart';

class EntityFactory {
  static T generateOBJ<T>(dynamic json) {
    // Logger.instance.debug(const UuidV4().generate(), "try to unmarshal post data, datatype=${T.toString()}");
    if (T.toString() == "WoxTheme") {
      return WoxTheme.fromJson(json) as T;
    } else if (T.toString() == "WoxSetting") {
      return WoxSetting.fromJson(json) as T;
    } else if (T.toString() == "WoxPreview") {
      return WoxPreview.fromJson(json) as T;
    } else if (T.toString() == "WoxImage") {
      return WoxImage.fromJson(json) as T;
    } else if (T.toString() == "WoxLang") {
      return WoxLang.fromJson(json) as T;
    } else if (T.toString() == "PluginDetail") {
      return PluginDetail.fromJson(json) as T;
    } else if (T.toString() == "List<PluginDetail>") {
      return (json as List).map((e) => PluginDetail.fromJson(e)).toList() as T;
    } else if (T.toString() == "List<WoxTheme>") {
      return (json as List).map((e) => WoxTheme.fromJson(e)).toList() as T;
    } else if (T.toString() == "List<AIModel>") {
      return (json as List).map((e) => AIModel.fromJson(e)).toList() as T;
    } else if (T.toString() == "List<WoxLang>") {
      return (json as List).map((e) => WoxLang.fromJson(e)).toList() as T;
    } else if (T.toString() == "List<WoxBackup>") {
      return (json as List).map((e) => WoxBackup.fromJson(e)).toList() as T;
    } else if (T.toString() == "List<AIMCPTool>") {
      return (json as List).map((e) => AIMCPTool.fromJson(e)).toList() as T;
    } else {
      return json as T;
    }
  }
}
