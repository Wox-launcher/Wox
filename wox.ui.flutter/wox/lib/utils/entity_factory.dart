import 'package:uuid/v4.dart';
import 'package:wox/entity/wox_ai.dart';
import 'package:wox/entity/wox_backup.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_lang.dart';
import 'package:wox/entity/wox_plugin.dart';
import 'package:wox/entity/wox_preview.dart';
import 'package:wox/entity/wox_runtime_status.dart';
import 'package:wox/entity/wox_setting.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/models/doctor_check_result.dart';
import 'package:wox/utils/log.dart';

/// Factory function type for creating objects from JSON
typedef JsonFactory = dynamic Function(dynamic json);

class EntityFactory {
  // Single object factories
  static final Map<String, JsonFactory> _singleFactories = {
    'WoxTheme': (json) => WoxTheme.fromJson(json),
    'WoxSetting': (json) => WoxSetting.fromJson(json),
    'WoxPreview': (json) => WoxPreview.fromJson(json),
    'WoxImage': (json) => WoxImage.fromJson(json),
    'WoxLang': (json) => WoxLang.fromJson(json),
    'PluginDetail': (json) => PluginDetail.fromJson(json),
    'AIModel': (json) => AIModel.fromJson(json),
    'DoctorCheckResult': (json) => DoctorCheckResult.fromJson(json),
  };

  // List factories
  static final Map<String, JsonFactory> _listFactories = {
    'List<PluginDetail>': (json) =>
        _createList<PluginDetail>(json, (e) => PluginDetail.fromJson(e)),
    'List<WoxTheme>': (json) =>
        _createList<WoxTheme>(json, (e) => WoxTheme.fromJson(e)),
    'List<AIModel>': (json) =>
        _createList<AIModel>(json, (e) => AIModel.fromJson(e)),
    'List<WoxLang>': (json) =>
        _createList<WoxLang>(json, (e) => WoxLang.fromJson(e)),
    'List<WoxBackup>': (json) =>
        _createList<WoxBackup>(json, (e) => WoxBackup.fromJson(e)),
    'List<AIMCPTool>': (json) =>
        _createList<AIMCPTool>(json, (e) => AIMCPTool.fromJson(e)),
    'List<AIProviderInfo>': (json) =>
        _createList<AIProviderInfo>(json, (e) => AIProviderInfo.fromJson(e)),
    'List<AIAgent>': (json) =>
        _createList<AIAgent>(json, (e) => AIAgent.fromJson(e)),
    'List<DoctorCheckResult>': (json) => _createList<DoctorCheckResult>(
        json, (e) => DoctorCheckResult.fromJson(e)),
    'List<WoxRuntimeStatus>': (json) => _createList<WoxRuntimeStatus>(
        json, (e) => WoxRuntimeStatus.fromJson(e)),
  };

  /// Helper method to create typed lists from JSON with robust error handling
  static List<T> _createList<T>(dynamic json, T Function(dynamic) fromJson) {
    if (json == null) return <T>[];

    // Ensure json is actually a List
    if (json is! List) {
      final traceId = const UuidV4().generate();
      Logger.instance.warn(traceId,
          'EntityFactory: Expected List but got ${json.runtimeType}, returning empty list');
      return <T>[];
    }

    final List<T> result = <T>[];

    for (int i = 0; i < json.length; i++) {
      try {
        final item = fromJson(json[i]);
        result.add(item);
      } catch (e) {
        // Log the error but continue processing other items
        final traceId = const UuidV4().generate();
        Logger.instance.warn(
            traceId, 'EntityFactory: Failed to parse item at index $i: $e');
        // Skip this item and continue with the rest
      }
    }

    return result;
  }

  /// Generate object from JSON based on type T with robust error handling
  static T generateOBJ<T>(dynamic json) {
    final typeName = T.toString();

    try {
      // Try single object factories first
      final singleFactory = _singleFactories[typeName];
      if (singleFactory != null) {
        return singleFactory(json) as T;
      }

      // Try list factories
      final listFactory = _listFactories[typeName];
      if (listFactory != null) {
        return listFactory(json) as T;
      }

      // Fallback to direct casting for primitive types
      return json as T;
    } catch (e) {
      final traceId = const UuidV4().generate();
      Logger.instance.error(traceId,
          'EntityFactory: Failed to generate object of type $typeName: $e');

      // Return safe default values based on type
      return _getSafeDefault<T>();
    }
  }

  /// Get safe default value for type T
  static T _getSafeDefault<T>() {
    final typeName = T.toString();

    // Handle list types
    if (typeName.startsWith('List<')) {
      return <dynamic>[] as T;
    }

    // Handle common primitive types
    switch (typeName) {
      case 'String':
        return '' as T;
      case 'int':
        return 0 as T;
      case 'double':
        return 0.0 as T;
      case 'bool':
        return false as T;
      case 'Map<String, dynamic>':
        return <String, dynamic>{} as T;
      default:
        // For complex objects, return null and let the caller handle it
        return null as T;
    }
  }
}
