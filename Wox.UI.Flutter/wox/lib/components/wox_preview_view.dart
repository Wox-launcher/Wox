import 'package:flutter/material.dart';
import 'package:flutter_markdown/flutter_markdown.dart';
import 'package:wox/entity.dart';

class WoxPreviewView extends StatelessWidget {
  final WoxPreview woxPreview;

  const WoxPreviewView({super.key, required this.woxPreview});

  @override
  Widget build(BuildContext context) {
    if (woxPreview.previewType == woxPreviewTypeMarkdown) {
      return Markdown(data: woxPreview.previewData);
    } else if (woxPreview.previewType == woxPreviewTypeText) {
      return Text(woxPreview.previewData);
    }

    return const SizedBox();
  }
}
