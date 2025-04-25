import 'package:wox/enums/wox_preview_scroll_position_enum.dart';
import 'package:wox/enums/wox_preview_type_enum.dart';
import 'package:wox/utils/wox_http_util.dart';

class WoxPreview {
  late WoxPreviewType previewType;
  late String previewData;
  late Map<String, String> previewProperties;
  late WoxPreviewScrollPosition scrollPosition;

  WoxPreview({required this.previewType, required this.previewData, required this.previewProperties, required this.scrollPosition});

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
    scrollPosition = json['ScrollPosition'];
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['PreviewType'] = previewType;
    data['PreviewData'] = previewData;
    data['PreviewProperties'] = previewProperties;
    data['ScrollPosition'] = scrollPosition;
    return data;
  }

  static WoxPreview empty() {
    return WoxPreview(previewType: "", previewData: "", previewProperties: {}, scrollPosition: "");
  }

  // unwrap the remote preview
  Future<WoxPreview> unWrap() async {
    if (previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_REMOTE.code) {
      return await WoxHttpUtil.instance.getData<WoxPreview>(previewData);
    }

    return this;
  }
}
