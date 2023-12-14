import 'dart:convert';

import 'package:wox/enums/wox_preview_type_enum.dart';

class WoxPreview {
  late WoxPreviewType previewType;
  late String previewData;
  late Map<String, String> previewProperties;

  WoxPreview({required this.previewType, required this.previewData, required this.previewProperties});

  @override
  int get hashCode => previewType.hashCode ^ previewData.hashCode ^ previewProperties.hashCode;

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;

    return other is WoxPreview && other.previewType == previewType && other.previewData == previewData && other.previewProperties == previewProperties;
  }

  WoxPreview.fromJson(Map<String, dynamic> json) {
    previewType = json['PreviewType'];
    previewData = json['PreviewData'];
    previewProperties = Map<String, String>.from(json['PreviewProperties'] ?? {});
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['PreviewType'] = previewType;
    data['PreviewData'] = previewData;
    data['PreviewProperties'] = previewProperties;
    return data;
  }

  static WoxPreview empty() {
    return WoxPreview(previewType: "", previewData: "", previewProperties: {});
  }
}
