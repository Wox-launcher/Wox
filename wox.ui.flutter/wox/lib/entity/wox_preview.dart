import 'package:wox/enums/wox_preview_scroll_position_enum.dart';
import 'package:wox/enums/wox_preview_type_enum.dart';
import 'package:wox/utils/wox_http_util.dart';

class WoxPreviewTag {
  late String label;
  late String tooltip;

  WoxPreviewTag({required this.label, this.tooltip = ""});

  WoxPreviewTag.fromJson(Map<String, dynamic> json) {
    label = json['Label']?.toString() ?? "";
    tooltip = json['Tooltip']?.toString() ?? "";
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['Label'] = label;
    data['Tooltip'] = tooltip;
    return data;
  }
}

class WoxPreview {
  late WoxPreviewType previewType;
  late String previewData;
  late String previewOverlayData;
  // Flutter only reads the normalized tag list that core sends, keeping the UI
  // model aligned with the tag-based footer instead of carrying legacy metadata
  // shapes into the rendering layer.
  late List<WoxPreviewTag> previewTags;
  late WoxPreviewScrollPosition scrollPosition;

  WoxPreview({required this.previewType, required this.previewData, this.previewOverlayData = "", this.previewTags = const [], required this.scrollPosition});

  @override
  int get hashCode => previewType.hashCode ^ previewData.hashCode ^ previewOverlayData.hashCode ^ previewTags.hashCode;

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;

    return other is WoxPreview &&
        other.previewType == previewType &&
        other.previewData == previewData &&
        other.previewOverlayData == previewOverlayData &&
        other.previewTags == previewTags;
  }

  WoxPreview.fromJson(Map<String, dynamic> json) {
    previewType = json['PreviewType'];
    previewData = json['PreviewData'];
    previewOverlayData = json['PreviewOverlayData'] ?? "";
    final rawPreviewTags = json['PreviewTags'];
    previewTags = rawPreviewTags is List ? rawPreviewTags.whereType<Map<String, dynamic>>().map(WoxPreviewTag.fromJson).toList() : const [];
    scrollPosition = json['ScrollPosition'];
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['PreviewType'] = previewType;
    data['PreviewData'] = previewData;
    data['PreviewOverlayData'] = previewOverlayData;
    data['PreviewTags'] = previewTags.map((tag) => tag.toJson()).toList();
    data['ScrollPosition'] = scrollPosition;
    return data;
  }

  static WoxPreview empty() {
    return WoxPreview(previewType: "", previewData: "", scrollPosition: "");
  }

  // unwrap the remote preview
  Future<WoxPreview> unWrap(String traceId) async {
    if (previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_REMOTE.code) {
      return await WoxHttpUtil.instance.getData<WoxPreview>(traceId, previewData);
    }

    return this;
  }
}
