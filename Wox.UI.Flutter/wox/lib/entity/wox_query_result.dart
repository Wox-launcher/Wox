import 'dart:convert';

import 'package:wox/enums/wox_image_type_enum.dart';

class WoxQueryResult {
  late String queryId;
  late String id;
  late String title;
  late String subTitle;
  late WoxImage icon;
  late Preview preview;
  late int score;
  late String contextData;
  late List<Actions> actions;
  late int refreshInterval;

  WoxQueryResult(
      {required this.queryId,
      required this.id,
      required this.title,
      required this.subTitle,
      required this.icon,
      required this.preview,
      required this.score,
      required this.contextData,
      required this.actions,
      required this.refreshInterval});

  WoxQueryResult.fromJson(Map<String, dynamic> json) {
    queryId = json['QueryId'];
    id = json['Id'];
    title = json['Title'];
    subTitle = json['SubTitle'];
    icon = (json['Icon'] != null ? WoxImage.fromJson(json['Icon']) : null)!;
    preview = (json['Preview'] != null ? Preview.fromJson(json['Preview']) : null)!;
    score = json['Score'];
    contextData = json['ContextData'];
    if (json['Actions'] != null) {
      actions = <Actions>[];
      json['Actions'].forEach((v) {
        actions.add(Actions.fromJson(v));
      });
    }
    refreshInterval = json['RefreshInterval'];
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['QueryId'] = queryId;
    data['Id'] = id;
    data['Title'] = title;
    data['SubTitle'] = subTitle;
    data['Icon'] = icon.toJson();
    data['Preview'] = preview.toJson();
    data['Score'] = score;
    data['ContextData'] = contextData;
    data['Actions'] = actions.map((v) => v.toJson()).toList();
    data['RefreshInterval'] = refreshInterval;
    return data;
  }
}

class WoxImage {
  late WoxImageType imageType;
  late String imageData;

  WoxImage({required this.imageType, required this.imageData});

  WoxImage.fromJson(Map<String, dynamic> json) {
    imageType = json['ImageType'];
    imageData = json['ImageData'];
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['ImageType'] = imageType;
    data['ImageData'] = imageData;
    return data;
  }
}

class Preview {
  late String previewType;
  late String previewData;
  late Map<String, String> previewProperties;

  Preview({required this.previewType, required this.previewData, required this.previewProperties});

  Preview.fromJson(Map<String, dynamic> json) {
    previewType = json['PreviewType'];
    previewData = json['PreviewData'];
    previewProperties = Map<String, String>.from(json['PreviewProperties'] ?? {});
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['PreviewType'] = previewType;
    data['PreviewData'] = previewData;
    data['PreviewProperties'] = const JsonEncoder().convert(previewProperties);
    return data;
  }
}

class Actions {
  late String id;
  late String name;
  late WoxImage icon;
  late bool isDefault;
  late bool preventHideAfterAction;

  Actions({required this.id, required this.name, required this.icon, required this.isDefault, required this.preventHideAfterAction});

  Actions.fromJson(Map<String, dynamic> json) {
    id = json['Id'];
    name = json['Name'];
    icon = (json['Icon'] != null ? WoxImage.fromJson(json['Icon']) : null)!;
    isDefault = json['IsDefault'];
    preventHideAfterAction = json['PreventHideAfterAction'];
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['Id'] = id;
    data['Name'] = name;
    data['Icon'] = icon.toJson();
    data['IsDefault'] = isDefault;
    data['PreventHideAfterAction'] = preventHideAfterAction;
    return data;
  }
}
