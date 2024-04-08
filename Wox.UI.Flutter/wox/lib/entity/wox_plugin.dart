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
  late List<String> screenshotUrls;
  late List<String> supportedOS;
  late bool isSystem;
  late bool isDisable;

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
    if (json['ScreenshotUrls'] != null) {
      screenshotUrls = List<String>.from(json['ScreenshotUrls']);
    } else {
      screenshotUrls = <String>[];
    }
    if (json['Settings'] != null) {
      isDisable = json['Settings']["Disabled"];
    } else {
      isDisable = false;
    }
    json['Commands']?.forEach((v) {
      commands.add(MetadataCommand.fromJson(v));
    });
    supportedOS = List<String>.from(json['SupportedOS']);
    isSystem = json['IsSystem'];
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

class PluginDetail {
  late String id;
  late String name;
  late String description;
  late String author;
  late String version;
  late WoxImage icon;
  late String website;
  late String entry;
  late List<String> triggerKeywords;
  late List<MetadataCommand> commands;
  late List<String> supportedOS;
  late List<String> screenshotUrls;
  late bool isSystem;
  late bool isInstalled;
  late bool isDisable;

  PluginDetail.empty() {
    id = '';
    name = '';
    description = '';
    author = '';
    version = '';
    icon = WoxImage.empty();
    website = '';
    entry = '';
    triggerKeywords = <String>[];
    commands = <MetadataCommand>[];
    supportedOS = <String>[];
    screenshotUrls = <String>[];
    isSystem = false;
    isInstalled = false;
    isDisable = false;
  }

  PluginDetail.fromInstalledPlugin(InstalledPlugin plugin) {
    id = plugin.id;
    name = plugin.name;
    description = plugin.description;
    author = plugin.author;
    version = plugin.version;
    icon = plugin.icon;
    website = plugin.website;
    entry = plugin.entry;
    triggerKeywords = plugin.triggerKeywords;
    commands = plugin.commands;
    supportedOS = plugin.supportedOS;
    isSystem = plugin.isSystem;
    isInstalled = true;
    screenshotUrls = plugin.screenshotUrls;
    isDisable = plugin.isDisable;
  }

  PluginDetail.fromStorePlugin(StorePlugin plugin) {
    id = plugin.id;
    name = plugin.name;
    description = plugin.description;
    author = plugin.author;
    version = plugin.version;
    icon = plugin.icon;
    website = plugin.website;
    isInstalled = plugin.isInstalled;
    isSystem = false;
    screenshotUrls = plugin.screenshotUrls;
    isDisable = false;
  }
}
