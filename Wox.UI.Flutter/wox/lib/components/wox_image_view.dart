import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_svg/svg.dart';
import 'package:lottie/lottie.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/enums/wox_image_type_enum.dart';

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
        errorBuilder: (context, error, stackTrace) {
          return SizedBox(width: width, height: height);
        },
      );
    } else if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_ABSOLUTE_PATH.code) {
      return Image.file(File(woxImage.imageData), width: width, height: height);
    } else if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_SVG.code) {
      return SvgPicture.string(woxImage.imageData, width: width, height: height);
    } else if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code) {
      return Text(woxImage.imageData, style: TextStyle(fontSize: width));
    } else if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_LOTTIE.code) {
      final bytes = utf8.encode(woxImage.imageData);
      return Lottie.memory(bytes, width: width, height: height);
    } else if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_BASE64.code) {
      if (!woxImage.imageData.contains(";base64,")) {
        return Text("Invalid image data: ${woxImage.imageData}", style: const TextStyle(color: Colors.red));
      }
      final imageData = woxImage.imageData.split(";base64,")[1];
      return Image.memory(base64Decode(imageData), width: width, height: height, fit: BoxFit.contain);
    }
    return const SizedBox(width: 24, height: 24);
  }
}
