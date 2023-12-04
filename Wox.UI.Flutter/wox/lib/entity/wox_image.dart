import 'package:wox/enums/wox_image_type_enum.dart';

class WoxImage {
  late WoxImageType imageType;
  late String imageData;

  WoxImage({required this.imageType, required this.imageData});

  WoxImage.fromJson(Map<String, dynamic> json) {
    imageType = json['ImageType'];
    imageData = json['ImageData'];
  }

  Map<String, dynamic> toJson() {
    final Map<String, dynamic> data = <String, dynamic>{};
    data['ImageType'] = imageType;
    data['ImageData'] = imageData;
    return data;
  }
}
