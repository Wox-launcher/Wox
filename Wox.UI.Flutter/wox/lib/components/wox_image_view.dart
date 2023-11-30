import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_svg/svg.dart';

import '../entity.dart';

class WoxImageView extends StatelessWidget {
  final WoxImage woxImage;
  final double width;
  final double height;

  const WoxImageView({super.key, required this.woxImage, this.width = 24, this.height = 24});

  @override
  Widget build(BuildContext context) {
    if (woxImage.imageType == woxImageTypeUrl) {
      return Image.network(woxImage.imageData, width: width, height: height);
    } else if (woxImage.imageType == woxImageTypeSvg) {
      SvgPicture.string(woxImage.imageData, width: width, height: height);
    } else if (woxImage.imageType == woxImageTypeBase64) {
      return Image.memory(base64Decode(woxImage.imageData), width: width, height: height);
    }

    return const SizedBox(width: 24, height: 24);
  }
}
