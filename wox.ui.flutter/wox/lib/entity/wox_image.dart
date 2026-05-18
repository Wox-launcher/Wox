import 'dart:convert';
import 'dart:io';

import 'package:wox/enums/wox_image_type_enum.dart';

class WoxImage {
  late WoxImageType imageType;
  late String imageData;

  // Cached File object for absolute path images
  File? cachedFile;

  WoxImage({required this.imageType, required this.imageData}) {
    _cacheFileIfNeeded();
  }

  void _cacheFileIfNeeded() {
    if (imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_ABSOLUTE_PATH.code && imageData.isNotEmpty) {
      cachedFile = File(imageData);
    }
  }

  bool get isLazyLoad => imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_LAZY_LOAD_IMAGE.code;

  WoxLazyLoadImagePayload? lazyLoadPayload() {
    if (!isLazyLoad || imageData.isEmpty) {
      return null;
    }
    try {
      return WoxLazyLoadImagePayload.fromJson(Map<String, dynamic>.from(jsonDecode(imageData)));
    } catch (_) {
      return null;
    }
  }

  WoxImage.fromJson(Map<String, dynamic> json) {
    imageType = json['ImageType'];
    imageData = json['ImageData'];
    _cacheFileIfNeeded();
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['ImageType'] = imageType;
    data['ImageData'] = imageData;
    return data;
  }

  @override
  int get hashCode => imageType.hashCode ^ imageData.hashCode;

  @override
  bool operator ==(Object other) {
    if (identical(this, other)) return true;
    return other is WoxImage && other.imageType == imageType && other.imageData == imageData;
  }

  static WoxImage? parse(String imageData) {
    //split image data with : to get image type, only get first part
    final List<String> imageDataList = imageData.split(':');
    if (imageDataList.length < 2) return null;

    final imageType = imageDataList[0];
    // the rest of the string is the image data
    final imageDataString = imageDataList.sublist(1).join(':');

    if (imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_BASE64.code) {
      return WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_BASE64.code, imageData: imageDataString);
    } else if (imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_SVG.code) {
      return WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_SVG.code, imageData: imageDataString);
    } else if (imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_URL.code) {
      return WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_URL.code, imageData: imageDataString);
    } else if (imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_ABSOLUTE_PATH.code) {
      return WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_ABSOLUTE_PATH.code, imageData: imageDataString);
    } else if (imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_RELATIVE_PATH.code) {
      return WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_RELATIVE_PATH.code, imageData: imageDataString);
    } else if (imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code) {
      return WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_EMOJI.code, imageData: imageDataString);
    } else if (imageType == WoxImageTypeEnum.WOX_IMAGE_TYPE_LAZY_LOAD_IMAGE.code) {
      return WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_LAZY_LOAD_IMAGE.code, imageData: imageDataString);
    } else {
      return null;
    }
  }

  static WoxImage empty() {
    return WoxImage(imageType: "", imageData: "");
  }

  static WoxImage newBase64(String imageData) {
    return WoxImage(imageType: WoxImageTypeEnum.WOX_IMAGE_TYPE_BASE64.code, imageData: imageData);
  }
}

class WoxLazyLoadImagePayload {
  final String token;
  final WoxImage placeholder;
  final int targetSize;

  WoxLazyLoadImagePayload({required this.token, required this.placeholder, required this.targetSize});

  factory WoxLazyLoadImagePayload.fromJson(Map<String, dynamic> json) {
    // Core creates lazy image payloads after plugin results are polished. The UI
    // parses only the token and placeholder here so plugin-facing WoxImage stays
    // unchanged while large thumbnails can be loaded after the widget is built.
    return WoxLazyLoadImagePayload(
      token: json['token']?.toString() ?? "",
      placeholder: json['placeholder'] is Map ? WoxImage.fromJson(Map<String, dynamic>.from(json['placeholder'])) : WoxImage.empty(),
      targetSize: json['targetSize'] is int ? json['targetSize'] : int.tryParse(json['targetSize']?.toString() ?? "") ?? 0,
    );
  }
}
