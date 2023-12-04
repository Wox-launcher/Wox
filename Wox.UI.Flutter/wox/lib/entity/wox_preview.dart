import 'dart:convert';

class WoxPreview {
  late String previewType;
  late String previewData;
  late Map<String, String> previewProperties;

  WoxPreview({required this.previewType, required this.previewData, required this.previewProperties});

  WoxPreview.fromJson(Map<String, dynamic> json) {
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

  static WoxPreview empty() {
    return WoxPreview(previewType: "", previewData: "", previewProperties: {});
  }
}
