import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_svg/svg.dart';
import 'package:lottie/lottie.dart';
import 'package:uuid/v4.dart';
import 'package:wox/components/wox_theme_icon_view.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/entity/wox_theme.dart';
import 'package:wox/enums/wox_image_type_enum.dart';
import 'package:wox/utils/log.dart';

class WoxImageView extends StatelessWidget {
  final WoxImage woxImage;
  final double? width;
  final double? height;

  const WoxImageView({super.key, required this.woxImage, this.width, this.height});

  @override
  Widget build(BuildContext context) {
    if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_URL.code) {
      return Image.network(
        woxImage.imageData,
        width: width,
        height: height,
        fit: BoxFit.contain,
        gaplessPlayback: true,
        errorBuilder: (context, error, stackTrace) {
          var traceId = const UuidV4().generate();
          Logger.instance.error(traceId, "Failed to load wox url image: $error");
          Logger.instance.error(traceId, "Image URL: ${woxImage.imageData}");
          Logger.instance.error(traceId, "Stack trace: $stackTrace");
          return SizedBox(width: width, height: height);
        },
        loadingBuilder: (context, child, loadingProgress) {
          if (loadingProgress == null) return child;
          return SizedBox(
            width: width,
            height: height,
            child: Center(
              child: CircularProgressIndicator(
                value: loadingProgress.expectedTotalBytes != null ? loadingProgress.cumulativeBytesLoaded / loadingProgress.expectedTotalBytes! : null,
              ),
            ),
          );
        },
      );
    }
    if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_ABSOLUTE_PATH.code) {
      if (!File(woxImage.imageData).existsSync()) {
        return const SizedBox(width: 24, height: 24);
      }
      return Image.file(
        File(woxImage.imageData),
        width: width,
        height: height,
        fit: BoxFit.contain,
        gaplessPlayback: true,
      );
    }
    if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_SVG.code) {
      return SizedBox(
        width: width,
        height: height,
        child: SvgPicture.string(woxImage.imageData),
      );
    }
    if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code) {
      // on windows, the emoji has default padding, so we need to offset it a bit
      var offset = const Offset(0, 0);
      if (Platform.isWindows) {
        offset = const Offset(-6, -2);
      }

      return SizedBox(
        width: width,
        height: height,
        child: Transform.translate(
          offset: offset,
          child: Transform.scale(
            scale: 1.03,
            child: Text(woxImage.imageData, style: TextStyle(fontSize: width, height: 1.0)),
          ),
        ),
      );
    }
    if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_LOTTIE.code) {
      final bytes = utf8.encode(woxImage.imageData);
      return Lottie.memory(bytes, width: width, height: height);
    }
    if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_THEME.code) {
      return WoxThemeIconView(theme: WoxTheme.fromJson(jsonDecode(woxImage.imageData)), width: width, height: height);
    }
    if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_BASE64.code) {
      if (!woxImage.imageData.contains(";base64,")) {
        return Text("Invalid image data: ${woxImage.imageData}", style: const TextStyle(color: Colors.red));
      }
      final imageData = woxImage.imageData.split(";base64,")[1];
      return Image.memory(
        base64Decode(imageData),
        width: width,
        height: height,
        fit: BoxFit.contain,
        gaplessPlayback: true,
      );
    }

    return const SizedBox(width: 24, height: 24);
  }
}
