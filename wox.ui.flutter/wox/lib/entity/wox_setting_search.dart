import 'package:wox/entity/wox_image.dart';

enum WoxSettingSearchTargetType { builtInSetting, installedPlugin, pluginSetting }

class WoxSettingSearchResult {
  final WoxSettingSearchTargetType type;
  final String id;
  final String title;
  final String subtitle;
  final String navPath;
  final String pluginId;
  final String settingKey;
  final WoxImage? icon;
  final List<String> searchTexts;
  final int score;

  const WoxSettingSearchResult({
    required this.type,
    required this.id,
    required this.title,
    required this.subtitle,
    required this.navPath,
    required this.pluginId,
    required this.settingKey,
    this.icon,
    required this.searchTexts,
    required this.score,
  });

  String get resultKey {
    switch (type) {
      case WoxSettingSearchTargetType.builtInSetting:
        return 'settings-search-result-builtInSetting-$settingKey';
      case WoxSettingSearchTargetType.installedPlugin:
        return 'settings-search-result-installedPlugin-$pluginId';
      case WoxSettingSearchTargetType.pluginSetting:
        return 'settings-search-result-pluginSetting-$pluginId-$settingKey';
    }
  }

  String get highlightTargetId {
    switch (type) {
      case WoxSettingSearchTargetType.builtInSetting:
        return 'built-in-$settingKey';
      case WoxSettingSearchTargetType.installedPlugin:
        return 'plugin-$pluginId';
      case WoxSettingSearchTargetType.pluginSetting:
        return 'plugin-setting-$pluginId-$settingKey';
    }
  }
}
