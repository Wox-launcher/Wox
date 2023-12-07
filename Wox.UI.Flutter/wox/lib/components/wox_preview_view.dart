import 'package:flutter/material.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:wox/entity/wox_preview.dart';
import 'package:wox/enums/wox_preview_type_enum.dart';

class WoxPreviewView extends StatelessWidget {
  final WoxPreview woxPreview;

  const WoxPreviewView({super.key, required this.woxPreview});

  @override
  Widget build(BuildContext context) {
    if (woxPreview.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_MARKDOWN.code) {
      return Markdown(data: woxPreview.previewData);
    } else if (woxPreview.previewType == WoxPreviewTypeEnum.WOX_PREVIEW_TYPE_TEXT.code) {
      return Text(woxPreview.previewData);
    }
    return const SizedBox();
  }
}
