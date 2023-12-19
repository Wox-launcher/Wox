import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_svg/svg.dart';
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
      return Image.network(woxImage.imageData, width: width, height: height);
    } else if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_SVG.code) {
      return SvgPicture.string(woxImage.imageData, width: width, height: height);
    } else if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_BASE64.code) {
      if (!woxImage.imageData.contains(";base64,")) {
        return Text("Invalid image data: ${woxImage.imageData}", style: const TextStyle(color: Colors.red));
      }
      final imageData = woxImage.imageData.split(";base64,")[1];
      return Image.memory(base64Decode(imageData), width: width, height: height, fit: BoxFit.fill);
    }
    return const SizedBox(width: 24, height: 24);
  }
}
