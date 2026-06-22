import 'package:uuid/v4.dart';
import 'package:wox/entity/wox_image.dart';
import 'package:wox/utils/log.dart';
import 'package:wox/utils/wox_http_util.dart';

// Loads the OS-resolved file icon through core so preview icons match launcher result icons.
Future<WoxImage?> loadWoxFilePreviewIcon(String filePath, {int size = 96}) async {
  final normalizedPath = filePath.trim();
  if (normalizedPath.isEmpty) {
    return null;
  }

  final traceId = const UuidV4().generate();
  try {
    final icon = await WoxHttpUtil.instance.postData<WoxImage>(traceId, "/image/file/icon", {"path": normalizedPath, "size": size});
    return icon.imageData.trim().isEmpty ? null : icon;
  } catch (error) {
    Logger.instance.warn(traceId, "Failed to load file preview icon for $normalizedPath: $error");
    return null;
  }
}
