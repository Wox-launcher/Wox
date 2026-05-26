import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_svg/svg.dart';
import 'package:wox/components/file_preview/file_preview_renderer.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/enums/wox_image_type_enum.dart';

class ImageFilePreviewRenderer implements WoxFilePreviewRenderer {
  static const imageExtensions = {"png", "gif", "bmp", "webp", "jpeg", "jpg", "svg"};

  @override
  bool supports(String fileExtension) {
    return imageExtensions.contains(fileExtension);
  }

  @override
  WoxFilePreviewResult render(WoxFilePreviewContext context) {
    final file = File(context.filePath);
    if (!file.existsSync()) {
      return WoxFilePreviewResult(content: context.buildText(context.tr("ui_file_preview_image_not_found", {"path": context.filePath})));
    }

    final image = context.fileExtension == "svg" ? SvgPicture.file(file, fit: BoxFit.contain) : Image.file(file, fit: BoxFit.contain);
    return WoxFilePreviewResult(
      content: context.buildImageSurface(image, overlayImage: WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_ABSOLUTE_PATH.code, imageData: context.filePath)),
      contentHandlesScrolling: true,
    );
  }
}
