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
    final Stopwatch? buildStopwatch = LoggerSwitch.enableBuildTimeLog ? (Stopwatch()..start()) : null;
    late final Widget content;

    if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_URL.code) {
      content = Image.network(
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
    } else if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_ABSOLUTE_PATH.code) {
      // Use cached File and existence check from WoxImage
      if (woxImage.cachedFileExists != true || woxImage.cachedFile == null) {
        content = const SizedBox(width: 24, height: 24);
      } else {
        content = Image.file(woxImage.cachedFile!, width: width, height: height, fit: BoxFit.contain, gaplessPlayback: true);
      }
    } else if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_SVG.code) {
      content = SizedBox(width: width, height: height, child: SvgPicture.string(woxImage.imageData));
    } else if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code) {
      // on windows, the emoji has default padding, so we need to offset it a bit
      var offset = const Offset(0, 0);
      if (Platform.isWindows) {
        offset = const Offset(-6, -2);
      }

      content = SizedBox(
        width: width,
        height: height,
        child: Transform.translate(offset: offset, child: Transform.scale(scale: 1.03, child: Text(woxImage.imageData, style: TextStyle(fontSize: width, height: 1.0)))),
      );
    } else if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_LOTTIE.code) {
      final bytes = utf8.encode(woxImage.imageData);
      content = Lottie.memory(bytes, width: width, height: height);
    } else if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_THEME.code) {
      content = WoxThemeIconView(theme: WoxTheme.fromJson(jsonDecode(woxImage.imageData)), width: width, height: height);
    } else if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_BASE64.code) {
      if (!woxImage.imageData.contains(";base64,")) {
        content = Text("Invalid image data: ${woxImage.imageData}", style: const TextStyle(color: Colors.red));
      } else {
        final imageData = woxImage.imageData.split(";base64,")[1];
        content = Image.memory(base64Decode(imageData), width: width, height: height, fit: BoxFit.contain, gaplessPlayback: true);
      }
    } else {
      content = const SizedBox(width: 24, height: 24);
    }

    if (buildStopwatch != null) {
      buildStopwatch.stop();
      Logger.instance.debug(const UuidV4().generate(), "flutter build metric: image view ${woxImage.imageType} - ${buildStopwatch.elapsedMicroseconds}μs");
    }

    return content;
  }
}
