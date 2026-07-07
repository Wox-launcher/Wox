import '../wox_plugin_setting.dart';

enum DictationModelStatus {
  notDownloaded,
  downloading,
  extracting,
  downloaded,
  failed;

  static DictationModelStatus fromString(String s) {
    switch (s) {
      case 'not_downloaded':
        return notDownloaded;
      case 'downloading':
        return downloading;
      case 'extracting':
        return extracting;
      case 'downloaded':
        return downloaded;
      case 'failed':
        return failed;
      default:
        return notDownloaded;
    }
  }
}

class DictationModelOption {
  late String id;
  late String displayName;
  late String description;
  late String languages;
  late bool recommended;
  late DictationModelStatus status;
  late int downloadProgress;
  late int sizeMB;
  late String error;

  DictationModelOption.fromJson(Map<String, dynamic> json) {
    id = json['ID'];
    displayName = json['DisplayName'];
    description = json['Description'] ?? '';
    languages = json['Languages'] ?? '';
    recommended = json['Recommended'] ?? false;
    status = DictationModelStatus.fromString(json['Status']);
    downloadProgress = json['DownloadProgress'] ?? 0;
    sizeMB = json['SizeMB'] ?? 0;
    error = json['Error'] ?? '';
  }
}

class PluginSettingValueDictationModel {
  late String key;
  late String label;
  late String tooltip;
  late String defaultValue;
  late List<DictationModelOption> options;
  late PluginSettingValueStyle style;

  PluginSettingValueDictationModel.fromJson(Map<String, dynamic> json) {
    key = json['Key'];
    label = json['Label'];
    tooltip = json['Tooltip'];
    defaultValue = json['DefaultValue'];
    if (json['Options'] != null) {
      options = (json['Options'] as List).map((e) => DictationModelOption.fromJson(e)).toList();
    } else {
      options = [];
    }
    // Style is deprecated in plugin SDKs; ignore plugin JSON and let the UI layout own spacing and width.
    style = PluginSettingValueStyle.defaults();
  }
}
