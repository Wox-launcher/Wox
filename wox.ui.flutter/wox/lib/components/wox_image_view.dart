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

// Image cache to prevent flickering during refreshes
class _ImageCache {
  static final Map<String, Widget> _cache = {};

  static Widget? get(String key) => _cache[key];

  static void put(String key, Widget widget) {
    if (_cache.length > 100) {
      // Limit cache size
      _cache.clear();
    }
    _cache[key] = widget;
  }
}

class WoxImageView extends StatelessWidget {
  final WoxImage woxImage;
  final double? width;
  final double? height;

  const WoxImageView({super.key, required this.woxImage, this.width, this.height});

  @override
  Widget build(BuildContext context) {
    // Create cache key based on image data and dimensions
    final cacheKey = '${woxImage.imageType}_${woxImage.imageData}_${width}_$height';

    // Check cache first to prevent flickering
    final cachedWidget = _ImageCache.get(cacheKey);
    if (cachedWidget != null) {
      return cachedWidget;
    }

    Widget imageWidget;

    if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_URL.code) {
      imageWidget = Image.network(
        woxImage.imageData,
        width: width,
        height: height,
        fit: BoxFit.contain,
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
      // check if file exists
      if (!File(woxImage.imageData).existsSync()) {
        imageWidget = const SizedBox(width: 24, height: 24);
      } else {
        imageWidget = Image.file(File(woxImage.imageData), width: width, height: height);
      }
    } else if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_SVG.code) {
      imageWidget = SvgPicture.string(woxImage.imageData, width: width, height: height);
    } else if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code) {
      imageWidget = Padding(
        padding: const EdgeInsets.only(left: 2, right: 2),
        child: Text(woxImage.imageData, style: TextStyle(fontSize: width)),
      );
    } else if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_LOTTIE.code) {
      final bytes = utf8.encode(woxImage.imageData);
      imageWidget = Lottie.memory(bytes, width: width, height: height);
    } else if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_THEME.code) {
      imageWidget = WoxThemeIconView(theme: WoxTheme.fromJson(jsonDecode(woxImage.imageData)), width: width, height: height);
    } else if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_BASE64.code) {
      if (!woxImage.imageData.contains(";base64,")) {
        imageWidget = Text("Invalid image data: ${woxImage.imageData}", style: const TextStyle(color: Colors.red));
      } else {
        final imageData = woxImage.imageData.split(";base64,")[1];
        imageWidget = Image.memory(base64Decode(imageData), width: width, height: height, fit: BoxFit.contain);
      }
    } else {
      imageWidget = const SizedBox(width: 24, height: 24);
    }

    // Cache the widget to prevent future rebuilds
    _ImageCache.put(cacheKey, imageWidget);

    return imageWidget;
  }
}
