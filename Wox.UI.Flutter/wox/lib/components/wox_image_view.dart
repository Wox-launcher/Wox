import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_svg/svg.dart';
import 'package:wox/entity/wox_query_result.dart';
import 'package:wox/enums/wox_image_type_enum.dart';

class WoxImageView extends StatelessWidget {
  final WoxImage woxImage;
  final double width;
  final double height;

  const WoxImageView({super.key, required this.woxImage, this.width = 24, this.height = 24});

  @override
  Widget build(BuildContext context) {
    if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_URL.code) {
      return Image.network(woxImage.imageData, width: width, height: height);
    } else if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_SVG.code) {
      SvgPicture.string(woxImage.imageData, width: width, height: height);
    } else if (woxImage.imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_BASE64.code) {
      return Image.memory(base64Decode(woxImage.imageData), width: width, height: height);
    }
    return const SizedBox(width: 24, height: 24);
  }
}
