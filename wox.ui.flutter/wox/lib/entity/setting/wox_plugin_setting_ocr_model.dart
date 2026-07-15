import '../wox_plugin_setting.dart';

enum OCRModelStatus {
  notDownloaded,
  downloading,
  finalizing,
  downloaded,
  failed;

  static OCRModelStatus fromString(String? value) {
    switch (value) {
      case 'downloading':
        return downloading;
      case 'finalizing':
        return finalizing;
      case 'downloaded':
        return downloaded;
      case 'failed':
        return failed;
      default:
        return notDownloaded;
    }
  }
}

class OCRModelOption {
  late String id;
  late String displayName;
  late String description;
  late String languages;
  late bool recommended;
  late bool available;
  late OCRModelStatus status;
  late int downloadProgress;
  late int sizeMB;
  late String error;

  OCRModelOption.fromJson(Map<String, dynamic> json) {
    id = json['ID'];
    displayName = json['DisplayName'];
    description = json['Description'] ?? '';
    languages = json['Languages'] ?? '';
    recommended = json['Recommended'] ?? false;
    available = json['Available'] ?? true;
    status = OCRModelStatus.fromString(json['Status']);
    downloadProgress = json['DownloadProgress'] ?? 0;
    sizeMB = json['SizeMB'] ?? 0;
    error = json['Error'] ?? '';
  }
}

class PluginSettingValueOCRModel {
  late String key;
  late String label;
  late String tooltip;
  late String defaultValue;
  late List<OCRModelOption> options;
  late PluginSettingValueStyle style;

  PluginSettingValueOCRModel.fromJson(Map<String, dynamic> json) {
    key = json['Key'];
    label = json['Label'];
    tooltip = json['Tooltip'];
    defaultValue = json['DefaultValue'];
    options = (json['Options'] as List<dynamic>? ?? []).map((option) => OCRModelOption.fromJson(Map<String, dynamic>.from(option))).toList();
    style = PluginSettingValueStyle.defaults();
  }
}
