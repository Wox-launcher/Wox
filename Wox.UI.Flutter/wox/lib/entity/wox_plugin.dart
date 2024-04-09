import 'wox_image.dart';
import 'wox_plugin_setting.dart';

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
  late List<PluginSettingDefinitionItem> settingDefinitions;
  late PluginSetting setting;

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
    settingDefinitions = <PluginSettingDefinitionItem>[];
    setting = PluginSetting.empty();
  }

  PluginDetail.fromJson(Map<String, dynamic> json) {
    id = json['Id'];
    name = json['Name'];
    description = json['Description'];
    author = json['Author'];
    version = json['Version'];
    icon = WoxImage.fromJson(json['Icon']);
    website = json['Website'];
    entry = json['Entry'];
    isSystem = json['IsSystem'];
    isInstalled = json['IsInstalled'];
    isDisable = json['IsDisable'];

    if (json['TriggerKeywords'] != null) {
      triggerKeywords = (json['TriggerKeywords'] as List).map((e) => e.toString()).toList();
    } else {
      triggerKeywords = <String>[];
    }

    if (json['Commands'] != null) {
      commands = <MetadataCommand>[];
      json['Commands'].forEach((v) {
        commands.add(MetadataCommand.fromJson(v));
      });
    } else {
      commands = <MetadataCommand>[];
    }

    if (json['SupportedOS'] != null) {
      supportedOS = (json['SupportedOS'] as List).map((e) => e.toString()).toList();
    } else {
      supportedOS = <String>[];
    }

    if (json['ScreenshotUrls'] != null) {
      screenshotUrls = (json['ScreenshotUrls'] as List).map((e) => e.toString()).toList();
    } else {
      screenshotUrls = <String>[];
    }

    if (json['SettingDefinitions'] != null) {
      settingDefinitions = <PluginSettingDefinitionItem>[];
      json['SettingDefinitions'].forEach((v) {
        settingDefinitions.add(PluginSettingDefinitionItem.fromJson(v));
      });
    } else {
      settingDefinitions = <PluginSettingDefinitionItem>[];
    }

    if (json['Setting'] != null) {
      setting = PluginSetting.fromJson(json['Setting']);
    } else {
      setting = PluginSetting.empty();
    }
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

class PluginSetting {
  late bool disabled;
  late List<String> triggerKeywords;
  late List<PluginQueryCommand> queryCommands;
  late Map<String, String> settings;

  PluginSetting.empty() {
    disabled = false;
    triggerKeywords = <String>[];
    queryCommands = <PluginQueryCommand>[];
    settings = <String, String>{};
  }

  PluginSetting.fromJson(Map<String, dynamic> json) {
    disabled = json['Disabled'];

    if (json['TriggerKeywords'] == null) {
      triggerKeywords = <String>[];
    } else {
      triggerKeywords = (json['TriggerKeywords'] as List).map((e) => e.toString()).toList();
    }

    if (json['QueryCommands'] == null) {
      queryCommands = <PluginQueryCommand>[];
    } else {
      queryCommands = <PluginQueryCommand>[];
      json['QueryCommands'].forEach((v) {
        queryCommands.add(PluginQueryCommand.fromJson(v));
      });
    }

    if (json['Settings'] == null) {
      settings = <String, String>{};
    } else {
      settings = json['Settings'].cast<String, String>();
    }
  }
}

class PluginQueryCommand {
  late String command;
  late String description;

  PluginQueryCommand.fromJson(Map<String, dynamic> json) {
    command = json['Command'];
    description = json['Description'];
  }
}
