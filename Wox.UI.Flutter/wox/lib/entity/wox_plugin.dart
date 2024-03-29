import 'wox_image.dart';

class StorePlugin {
  late String id;
  late String name;
  late String author;
  late String version;
  late String minWoxVersion;
  late String runtime;
  late String description;
  late WoxImage icon;
  late String website;
  late String downloadUrl;
  late List<String> screenshotUrls;
  late String dateCreated;
  late String dateUpdated;
  late bool isInstalled;
  bool isInstalling = false;

  StorePlugin.fromJson(Map<String, dynamic> json) {
    id = json['Id'];
    name = json['Name'];
    author = json['Author'];
    version = json['Version'];
    minWoxVersion = json['MinWoxVersion'];
    runtime = json['Runtime'];
    description = json['Description'];
    icon = WoxImage.fromJson(json['Icon']);
    website = json['Website'];
    downloadUrl = json['DownloadUrl'];
    screenshotUrls = List<String>.from(json['ScreenshotUrls']);
    dateCreated = json['DateCreated'];
    dateUpdated = json['DateUpdated'];
    isInstalled = json['IsInstalled'];
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['Id'] = id;
    data['Name'] = name;
    data['Author'] = author;
    data['Version'] = version;
    data['MinWoxVersion'] = minWoxVersion;
    data['Runtime'] = runtime;
    data['Description'] = description;
    data['Icon'] = icon.toJson();
    data['Website'] = website;
    data['DownloadUrl'] = downloadUrl;
    data['ScreenshotUrls'] = screenshotUrls;
    data['DateCreated'] = dateCreated;
    data['DateUpdated'] = dateUpdated;
    data['IsInstalled'] = isInstalled;
    return data;
  }
}

class InstalledPlugin {
  late String id;
  late String name;
  late String author;
  late String version;
  late String minWoxVersion;
  late String runtime;
  late String description;
  late WoxImage icon;
  late String website;
  late String entry;
  late List<String> triggerKeywords;
  late List<MetadataCommand> commands;
  late List<String> supportedOS;
  late bool isSystem;

  InstalledPlugin.fromJson(Map<String, dynamic> json) {
    id = json['Id'];
    name = json['Name'];
    author = json['Author'];
    version = json['Version'];
    minWoxVersion = json['MinWoxVersion'];
    runtime = json['Runtime'];
    description = json['Description'];
    icon = WoxImage.fromJson(json['Icon']);
    website = json['Website'];
    entry = json['Entry'];
    triggerKeywords = List<String>.from(json['TriggerKeywords']);
    commands = <MetadataCommand>[];
    json['Commands']?.forEach((v) {
      commands.add(MetadataCommand.fromJson(v));
    });
    supportedOS = List<String>.from(json['SupportedOS']);
    isSystem = json['IsSystem'];
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['Id'] = id;
    data['Name'] = name;
    data['Author'] = author;
    data['Version'] = version;
    data['MinWoxVersion'] = minWoxVersion;
    data['Runtime'] = runtime;
    data['Description'] = description;
    data['Icon'] = icon.toJson();
    data['Website'] = website;
    data['Entry'] = entry;
    data['TriggerKeywords'] = triggerKeywords;
    data['Commands'] = commands.map((v) => v.toJson()).toList();
    data['SupportedOS'] = supportedOS;
    data['IsSystem'] = isSystem;
    return data;
  }
}

class MetadataCommand {
  late String command;
  late String description;

  MetadataCommand.fromJson(Map<String, dynamic> json) {
    command = json['Command'];
    description = json['Description'];
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['Command'] = command;
    data['Description'] = description;
    return data;
  }
}
