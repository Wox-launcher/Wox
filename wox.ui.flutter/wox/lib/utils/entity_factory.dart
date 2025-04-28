import 'package:wox/entity/wox_ai.dart';
import 'package:wox/entity/wox_backup.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_lang.dart';
import 'package:wox/entity/wox_plugin.dart';
import 'package:wox/entity/wox_preview.dart';
import 'package:wox/entity/wox_setting.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/models/doctor_check_result.dart';

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
    } else if (T.toString() == "AIModel") {
      return AIModel.fromJson(json) as T;
    } else if (T.toString() == "DoctorCheckResult") {
      return DoctorCheckResult.fromJson(json) as T;
    } else if (T.toString() == "List<PluginDetail>") {
      if (json == null) {
        return <PluginDetail>[] as T;
      }
      return (json as List).map((e) => PluginDetail.fromJson(e)).toList() as T;
    } else if (T.toString() == "List<WoxTheme>") {
      if (json == null) {
        return <WoxTheme>[] as T;
      }
      return (json as List).map((e) => WoxTheme.fromJson(e)).toList() as T;
    } else if (T.toString() == "List<AIModel>") {
      if (json == null) {
        return <AIModel>[] as T;
      }
      return (json as List).map((e) => AIModel.fromJson(e)).toList() as T;
    } else if (T.toString() == "List<WoxLang>") {
      if (json == null) {
        return <WoxLang>[] as T;
      }
      return (json as List).map((e) => WoxLang.fromJson(e)).toList() as T;
    } else if (T.toString() == "List<WoxBackup>") {
      if (json == null) {
        return <WoxBackup>[] as T;
      }
      return (json as List).map((e) => WoxBackup.fromJson(e)).toList() as T;
    } else if (T.toString() == "List<AIMCPTool>") {
      if (json == null) {
        return <AIMCPTool>[] as T;
      }
      return (json as List).map((e) => AIMCPTool.fromJson(e)).toList() as T;
    } else if (T.toString() == "List<AIProviderInfo>") {
      if (json == null) {
        return <AIProviderInfo>[] as T;
      }
      return (json as List).map((e) => AIProviderInfo.fromJson(e)).toList() as T;
    } else if (T.toString() == "List<AIAgent>") {
      if (json == null) {
        return <AIAgent>[] as T;
      }
      return (json as List).map((e) => AIAgent.fromJson(e)).toList() as T;
    } else if (T.toString() == "List<DoctorCheckResult>") {
      if (json == null) {
        return <DoctorCheckResult>[] as T;
      }
      return (json as List).map((e) => DoctorCheckResult.fromJson(e)).toList() as T;
    } else {
      return json as T;
    }
  }
}
